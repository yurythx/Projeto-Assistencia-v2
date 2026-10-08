package app

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Árvore: mãe inexistente, ciclo com neta, slug vazio, histórico de
// página inexistente e parâmetros malformados.
func TestWikiTreeRules(t *testing.T) {
	h := newHarness(t)
	_, ana := h.user("nexus-user")
	sfx := uuid.NewString()[:6]
	raiz := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", ana, `{"title":"Raiz `+sfx+`"}`))
	filha := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", ana, `{"title":"Filha `+sfx+`","parent_id":"`+raiz.ID+`"}`))
	neta := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", ana, `{"title":"Neta `+sfx+`","parent_id":"`+filha.ID+`"}`))

	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/wiki/pages", ana, `{"title":"Órfã","parent_id":"`+uuid.NewString()+`"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/wiki/pages", ana, `{"title":"###"}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/wiki/pages", ana, `{"title":"Outra","slug":"`+raiz.Slug+`"}`)
	h.expect(http.StatusConflict, http.MethodPut, "/api/v1/wiki/pages/"+raiz.ID, ana, `{"title":"Raiz","parent_id":"`+neta.ID+`","version":1}`)
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/wiki/pages/"+raiz.ID, ana, `{"title":"Raiz","parent_id":"`+uuid.NewString()+`","version":1}`)
	moved := data[wikiResp](t, h.expect(http.StatusOK, http.MethodPut, "/api/v1/wiki/pages/"+neta.ID, ana, `{"title":"Neta promovida","parent_id":"`+raiz.ID+`","version":1}`))
	if moved.Version != 2 {
		t.Fatalf("mover gera nova versão: %+v", moved)
	}
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/wiki/pages/"+neta.ID, ana, "").Body.String(); !strings.Contains(body, neta.Slug) {
		t.Fatalf("slug mantido quando não informado: %s", body)
	}
	if res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q=Raiz+"+sfx+"&module=wiki", ana, "").Body.String(); !strings.Contains(res, raiz.ID) {
		t.Fatalf("busca global na wiki: %s", res)
	}

	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/wiki/pages/"+uuid.NewString()+"/revisions", ana, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/wiki/pages/"+raiz.ID+"/revisions/99", ana, "")
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/wiki/pages/"+raiz.ID+"/revisions/99/restore", ana, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/wiki/pages/nao-existe-"+sfx, ana, "")
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/wiki/pages/"+uuid.NewString(), ana, `{"title":"x","version":1}`)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/wiki/pages/x", `{"title":"x","version":1}`},
		{http.MethodGet, "/api/v1/wiki/pages/x/revisions", ""},
		{http.MethodGet, "/api/v1/wiki/pages/x/revisions/1", ""},
		{http.MethodGet, "/api/v1/wiki/pages/" + raiz.ID + "/revisions/um", ""},
		{http.MethodPost, "/api/v1/wiki/pages/x/revisions/1/restore", ""},
		{http.MethodPost, "/api/v1/wiki/pages/" + raiz.ID + "/revisions/um/restore", ""},
		{http.MethodPost, "/api/v1/wiki/pages", `{`},
		{http.MethodPut, "/api/v1/wiki/pages/" + raiz.ID, `{`},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, ana, c.body)
	}
	admin := h.admin()
	h.expect(http.StatusBadRequest, http.MethodDelete, "/api/v1/wiki/pages/x", admin, "")
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/wiki/pages/"+uuid.NewString(), admin, "")
}

// Dono organizacional da página (ADR 013): marca como dona quem está
// lotado na unidade (ou tem wiki:manage cobrindo-a); subpágina herda a dona
// da mãe; excluir exige wiki:manage cobrindo a dona.
func TestWikiGestaoComEscopo(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	o := h.orgEscopo(t)
	gestorA := h.gestorEm(t, o.a, "wiki:manage")
	lotadoB := h.gestorEm(t, o.b, "contact:read") // lotado em B, sem wiki:manage
	_, estranho := h.user("nexus-user")
	pagina := func(unidade, parent string) string {
		body := `{"title":"Página ` + uuid.NewString()[:8] + `"`
		if unidade != "" {
			body += `,"unidade_id":"` + unidade + `"`
		}
		if parent != "" {
			body += `,"parent_id":"` + parent + `"`
		}
		return body + `}`
	}
	type paginaResp struct {
		ID        string  `json:"id"`
		UnidadeID *string `json:"unidade_id"`
	}

	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/wiki/pages", estranho, pagina(o.b, ""))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/wiki/pages", gestorA, pagina(o.b, ""))
	deB := data[paginaResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", lotadoB, pagina(o.b, "")))
	deSub := data[paginaResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", gestorA, pagina(o.sub, "")))
	filha := data[paginaResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", estranho, pagina("", deSub.ID)))
	if filha.UnidadeID == nil || *filha.UnidadeID != o.sub {
		t.Fatalf("subpágina herda a dona da mãe: %+v", filha)
	}

	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/wiki/pages/"+deB.ID, gestorA, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/wiki/pages/"+filha.ID, gestorA, "") // subunidade de A
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/wiki/pages/"+deSub.ID, gestorA, "")
	institucional := data[paginaResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", estranho, pagina("", "")))
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/wiki/pages/"+institucional.ID, gestorA, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/wiki/pages/"+institucional.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/wiki/pages/"+deB.ID, admin, "")
}

// Público-alvo na Wiki (ADR 014): a página para a unidade A e as
// subpáginas dela só são lidas por quem está em A ou abaixo (a subpágina
// restringe mais, nunca abre); fora do público somem da árvore, do
// detalhe, do histórico e da busca. Trocar o público: quem criou ou a
// gestão da dona — qualquer outro que edite não esconde a página.
func TestWikiPublicoAlvo(t *testing.T) {
	h := newHarness(t)
	o := h.orgEscopo(t)
	autorID, autor := h.user("nexus-user")
	_ = autorID
	naSub := h.gestorEm(t, o.sub, "contact:read")
	emB := h.gestorEm(t, o.b, "contact:read")
	sfx := uuid.NewString()[:8]
	manual := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", autor,
		`{"title":"Manual de A `+sfx+`","body":"procedimento `+sfx+`","publico":{"unidades":["`+o.a+`"]}}`))
	anexo := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", autor,
		`{"title":"Anexo `+sfx+`","parent_id":"`+manual.ID+`","body":"anexo `+sfx+`"}`))
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/wiki/pages", autor,
		`{"title":"Fantasma `+sfx+`","publico":{"entidades":["`+uuid.NewString()+`"]}}`)

	arvore := func(tok string) string {
		return h.expect(http.StatusOK, http.MethodGet, "/api/v1/wiki/tree", tok, "").Body.String()
	}
	if a := arvore(naSub); !strings.Contains(a, manual.ID) || !strings.Contains(a, anexo.ID) {
		t.Fatalf("lotado na subunidade de A lê o manual e o anexo: %s", a)
	}
	if a := arvore(emB); strings.Contains(a, manual.ID) || strings.Contains(a, anexo.ID) {
		t.Fatalf("lotado em B não vê o manual nem o anexo herdado: %s", a)
	}
	for _, path := range []string{"/api/v1/wiki/pages/" + anexo.Slug, "/api/v1/wiki/pages/" + manual.ID + "/revisions", "/api/v1/wiki/pages/" + manual.ID + "/revisions/1"} {
		h.expect(http.StatusNotFound, http.MethodGet, path, emB, "")
		h.expect(http.StatusOK, http.MethodGet, path, naSub, "")
	}
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/wiki/pages", emB, `{"title":"Filha intrusa","parent_id":"`+manual.ID+`"}`)
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/wiki/pages/"+anexo.ID, emB, `{"title":"x","parent_id":"`+manual.ID+`","version":1}`)
	if b := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+sfx, emB, "").Body.String(); strings.Contains(b, manual.ID) || strings.Contains(b, anexo.ID) {
		t.Fatalf("busca global respeita o público: %s", b)
	}

	// Editar mantendo o público: qualquer leitor. Trocar: só quem criou.
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/wiki/pages/"+manual.ID, naSub, `{"title":"Manual de A `+sfx+`","body":"revisto","version":1}`)
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/wiki/pages/"+manual.ID, naSub, `{"title":"Manual de A `+sfx+`","version":2,"publico":{"entidades":[],"unidades":[]}}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/wiki/pages/"+manual.ID, autor, `{"title":"Manual de A `+sfx+`","version":2,"publico":{"unidades":["`+uuid.NewString()+`"]}}`)
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/wiki/pages/"+manual.ID, autor, `{"title":"Manual de A `+sfx+`","version":2,"publico":{"entidades":[],"unidades":[]}}`)
	if a := arvore(emB); !strings.Contains(a, manual.ID) {
		t.Fatalf("sem público, todos leem: %s", a)
	}
	// Mover para baixo de uma página que não vê: não encontrada.
	fechada := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", autor,
		`{"title":"Fechada `+sfx+`","publico":{"unidades":["`+o.a+`"]}}`))
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/wiki/pages/"+manual.ID, emB, `{"title":"Manual de A `+sfx+`","version":3,"parent_id":"`+fechada.ID+`"}`)
}
