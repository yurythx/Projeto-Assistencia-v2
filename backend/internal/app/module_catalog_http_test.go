package app

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Carta de Serviços: publicar exige resumo e canal (Lei 13.460/2017);
// mudar para o mesmo estado não regrava; publicado não se exclui; slug e
// unidade responsável validados; busca global só com publicados.
func TestCatalogPublicationRules(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	create := func(want int, body string) postResp {
		return data[postResp](t, h.expect(want, http.MethodPost, "/api/v1/catalog/admin/services", admin, body))
	}
	incompleto := create(http.StatusCreated, `{"title":"Passaporte `+sfx+`"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/catalog/admin/services/"+incompleto.ID+"/publish", admin, "")
	full := `{"title":"Passaporte ` + sfx + `","summary":"Emissão do passaporte","channels":[{"type":"presencial","label":"Posto","value":"Rua A, 1"}]}`
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/catalog/admin/services/"+incompleto.ID, admin, full)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/catalog/admin/services/"+incompleto.ID+"/publish", admin, "")
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/catalog/admin/services/"+incompleto.ID+"/publish", admin, "") // já publicado
	if outboxCount(t, h, "catalog.service.published", incompleto.ID) != 1 {
		t.Fatal("republicar não emite o evento de novo")
	}
	// Publicado continua precisando do mínimo: tirar o canal é recusado.
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/catalog/admin/services/"+incompleto.ID, admin,
		`{"title":"Passaporte `+sfx+`","summary":"Emissão do passaporte","channels":[]}`)
	h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/catalog/admin/services/"+incompleto.ID, admin, "")

	// Busca global (anônima) encontra o publicado.
	if res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q=Passaporte+"+sfx+"&module=catalog", admin, "").Body.String(); !strings.Contains(res, incompleto.ID) {
		t.Fatalf("serviço publicado na busca global: %s", res)
	}
	// Lista de gestão por estado e texto.
	if l := h.expect(http.StatusOK, http.MethodGet, "/api/v1/catalog/admin/services?status=published&q="+sfx, admin, "").Body.String(); !strings.Contains(l, incompleto.ID) {
		t.Fatalf("lista de gestão filtrada: %s", l)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/catalog/admin/services/"+incompleto.ID, admin, "")
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/catalog/admin/services/"+incompleto.ID+"/unpublish", admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/catalog/admin/services/"+incompleto.ID, admin, "")

	// Slug e unidade responsável.
	create(http.StatusUnprocessableEntity, `{"title":"!!!"}`)
	create(http.StatusUnprocessableEntity, `{"title":"Com unidade","responsible_unidade_id":"`+uuid.NewString()+`"}`)
	dup := create(http.StatusCreated, `{"title":"Duplicado `+sfx+`"}`)
	create(http.StatusConflict, `{"title":"Outro","slug":"`+dup.Slug+`"}`)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/catalog/admin/services/x", ""},
		{http.MethodPut, "/api/v1/catalog/admin/services/x", `{"title":"x"}`},
		{http.MethodPost, "/api/v1/catalog/admin/services/x/publish", ""},
		{http.MethodDelete, "/api/v1/catalog/admin/services/x", ""},
		{http.MethodPost, "/api/v1/catalog/admin/services", `{`},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, admin, c.body)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/catalog/admin/services/" + uuid.NewString(), ""},
		{http.MethodPut, "/api/v1/catalog/admin/services/" + uuid.NewString(), `{"title":"Fantasma"}`},
		{http.MethodPost, "/api/v1/catalog/admin/services/" + uuid.NewString() + "/archive", ""},
		{http.MethodDelete, "/api/v1/catalog/admin/services/" + uuid.NewString(), ""},
		{http.MethodGet, "/api/v1/catalog/services/nao-existe-" + sfx, ""},
	} {
		h.expect(http.StatusNotFound, c.method, c.path, admin, c.body)
	}
}

// catalog:manage com escopo (ADR 013): o gestor da unidade A gerencia os
// serviços de A e das subunidades; não os de B nem os institucionais.
func TestCatalogGestaoComEscopo(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	o := h.orgEscopo(t)
	gestorA := h.gestorEm(t, o.a, "catalog:manage")
	svc := func(nome, unidade string) string {
		body := `{"title":"` + nome + ` ` + uuid.NewString()[:6] + `"`
		if unidade != "" {
			body += `,"responsible_unidade_id":"` + unidade + `"`
		}
		return body + `}`
	}
	deA := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/catalog/admin/services", gestorA, svc("Serviço de A", o.a)))
	deSub := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/catalog/admin/services", gestorA, svc("Serviço da subunidade", o.sub)))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/catalog/admin/services", gestorA, svc("Serviço de B", o.b))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/catalog/admin/services", gestorA, svc("Institucional", ""))

	institucional := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/catalog/admin/services", admin, svc("Institucional", "")))
	deB := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/catalog/admin/services", admin, svc("Serviço de B", o.b)))
	for _, id := range []string{institucional.ID, deB.ID} {
		h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/catalog/admin/services/"+id, gestorA, "")
		h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/catalog/admin/services/"+id+"/archive", gestorA, "")
		h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/catalog/admin/services/"+id, gestorA, "")
		h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/catalog/admin/services/"+id, gestorA, svc("Tomado", o.a))
	}
	// Não "puxa" um serviço próprio para fora do escopo.
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/catalog/admin/services/"+deA.ID, gestorA, svc("Para B", o.b))
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/catalog/admin/services/"+deA.ID, gestorA, svc("Serviço de A editado", o.a))
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/catalog/admin/services/"+deSub.ID, gestorA, "")

	lista := h.expect(http.StatusOK, http.MethodGet, "/api/v1/catalog/admin/services", gestorA, "").Body.String()
	if !strings.Contains(lista, deA.ID) || strings.Contains(lista, deB.ID) || strings.Contains(lista, institucional.ID) {
		t.Fatalf("lista de gestão do gestor de A: só o que ele gerencia: %s", lista)
	}
	// A gestão global alcança tudo (a lista é paginada: confere direto).
	for _, id := range []string{deA.ID, deB.ID, institucional.ID} {
		h.expect(http.StatusOK, http.MethodGet, "/api/v1/catalog/admin/services/"+id, admin, "")
	}
}
