package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Autoatendimento nunca define a própria lotação (nem no primeiro
// salvamento); a gestão define, com departamento coerente com a unidade;
// perfil oculto; busca e filtros.
func TestDirectoryProfileRules(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	ent := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"Órgão `+sfx+`"}`))
	un1 := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin, `{"entidade_id":"`+ent.ID+`","nome":"Obras `+sfx+`"}`))
	un2 := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin, `{"entidade_id":"`+ent.ID+`","nome":"Saúde `+sfx+`"}`))
	dep := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", admin, `{"unidade_id":"`+un1.ID+`","nome":"Projetos `+sfx+`"}`))
	anaID, ana := h.user("nexus-user")
	_, beto := h.user("nexus-user")

	// Primeiro salvamento da própria ficha: a lotação enviada é ignorada.
	me := h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/me", ana,
		`{"job_title":"Engenheira `+sfx+`","phone":"61 3333-0000","unidade_id":"`+un2.ID+`","visible":true}`).Body.String()
	if strings.Contains(me, un2.ID) {
		t.Fatalf("autoatendimento não escolhe a própria lotação: %s", me)
	}
	// A gestão define — o departamento precisa ser da unidade.
	put := func(want int, body string) string {
		return h.expect(want, http.MethodPut, "/api/v1/directory/people/"+anaID.String(), admin, body).Body.String()
	}
	put(http.StatusUnprocessableEntity, `{"job_title":"Engenheira","unidade_id":"`+un2.ID+`","departamento_id":"`+dep.ID+`"}`)
	put(http.StatusUnprocessableEntity, `{"job_title":"Engenheira","departamento_id":"`+uuid.NewString()+`"}`)
	put(http.StatusUnprocessableEntity, `{"job_title":"Engenheira","unidade_id":"`+uuid.NewString()+`"}`)
	if got := put(http.StatusOK, `{"job_title":"Engenheira `+sfx+`","departamento_id":"`+dep.ID+`"}`); !strings.Contains(got, un1.ID) {
		t.Fatalf("o departamento completa a unidade: %s", got)
	}
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/directory/people/"+anaID.String(), beto, `{"job_title":"x"}`)
	// Depois, a própria pessoa edita a ficha sem perder a lotação da gestão.
	if got := h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/me", ana, `{"job_title":"Engenheira sênior `+sfx+`","unidade_id":"`+un2.ID+`"}`).Body.String(); !strings.Contains(got, un1.ID) {
		t.Fatalf("autoatendimento preserva a lotação definida pela gestão: %s", got)
	}

	// Busca por cargo, filtros e curingas do LIKE tratados como texto.
	if l := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people?q=s%C3%AAnior+"+sfx, beto, "").Body.String(); !strings.Contains(l, anaID.String()) {
		t.Fatalf("busca por cargo: %s", l)
	}
	if l := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people?unidade_id="+un1.ID+"&departamento_id="+dep.ID, beto, "").Body.String(); !strings.Contains(l, anaID.String()) {
		t.Fatalf("filtros por unidade e departamento: %s", l)
	}
	if l := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people?q=%25%25%25", beto, "").Body.String(); strings.Contains(l, anaID.String()) {
		t.Fatal("% digitado na busca não vira curinga")
	}

	// Ficha oculta: some da busca e da consulta de terceiros.
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/me", ana, `{"job_title":"Engenheira","visible":false}`)
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/directory/people/"+anaID.String(), beto, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people/"+anaID.String(), ana, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people/"+anaID.String(), admin, "")
	if l := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people?q="+sfx, beto, "").Body.String(); strings.Contains(l, anaID.String()) {
		t.Fatal("ficha oculta fora da busca")
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/me", ana, "")
	if s := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/public/sectors?q=Obras+"+sfx, "", "").Body.String(); !strings.Contains(s, un1.ID) {
		t.Fatalf("setores públicos: %s", s)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/sectors?q="+sfx, beto, "")
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/directory/people/x", ""},
		{http.MethodPut, "/api/v1/directory/people/x", `{}`},
		{http.MethodGet, "/api/v1/directory/people?unidade_id=x", ""},
		{http.MethodGet, "/api/v1/directory/people?departamento_id=x", ""},
		{http.MethodPut, "/api/v1/directory/me", `{`},
		{http.MethodPut, "/api/v1/directory/people/" + anaID.String(), `{`},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, admin, c.body)
	}
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/directory/people/"+uuid.NewString(), admin, `{"job_title":"Fantasma"}`)
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/directory/people/"+uuid.NewString(), admin, "")
}

// LGPD art. 18 com dados de plugin: o perfil do Diretório entra no pacote
// do titular e é eliminado junto com a anonimização da conta — mesmo com o
// módulo desativado (desativar não suspende os direitos do titular).
func TestDirectoryPersonalDataFollowsLGPDRequests(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	anaID, ana := h.user("nexus-user")
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/me", ana, `{"job_title":"Analista","phone":"61 99999-0000","bio":"sobre mim"}`)

	export := h.expect(http.StatusOK, http.MethodGet, "/api/v1/lgpd/meus-dados", ana, "").Body.String()
	if !strings.Contains(export, `"directory"`) || !strings.Contains(export, "61 99999-0000") {
		t.Fatalf("o perfil do Diretório entra no pacote do titular: %s", export)
	}

	h.setModule(admin, "directory", false)
	defer h.setModule(admin, "directory", true)
	h.expect(http.StatusAccepted, http.MethodPost, "/api/v1/lgpd/solicitar-exclusao", ana, "")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = h.d.LGPD.ErasureProcessor()(ctx); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		_ = h.d.DB.QueryRow(context.Background(), `SELECT status FROM data_subject_requests WHERE user_id = $1`, anaID).Scan(&status)
		if status == "completed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	var profiles int
	_ = h.d.DB.QueryRow(context.Background(), `SELECT count(*) FROM directory_profiles WHERE user_id = $1`, anaID).Scan(&profiles)
	if status != "completed" || profiles != 0 {
		t.Fatalf("eliminação concluída e perfil apagado: %s, %d perfil(is)", status, profiles)
	}
}

// directory:manage com escopo (ADR 013): a gestão da unidade A edita a
// ficha e a lotação exibida das pessoas de A (e subunidades) — não as de
// B, nem quem ainda não tem lotação exibida (institucional); vê os perfis
// ocultos só no escopo.
func TestDirectoryGestaoComEscopo(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	o := h.orgEscopo(t)
	gestorA := h.gestorEm(t, o.a, "directory:manage")
	pessoa := func(unidade string, visivel bool) string {
		id, _ := h.user("nexus-user")
		if unidade != "" {
			h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/people/"+id.String(), admin,
				`{"job_title":"Servidor","unidade_id":"`+unidade+`","visible":`+map[bool]string{true: "true", false: "false"}[visivel]+`}`)
		}
		return id.String()
	}
	deA, deSub, deB, semLotacao := pessoa(o.a, false), pessoa(o.sub, true), pessoa(o.b, false), pessoa("", true)
	ficha := func(unidade string) string {
		return `{"job_title":"Chefe","unidade_id":"` + unidade + `","visible":false}`
	}
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/people/"+deA, gestorA, ficha(o.a))
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/people/"+deSub, gestorA, ficha(o.sub))    // herança
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/directory/people/"+deA, gestorA, ficha(o.b)) // não move para B
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/directory/people/"+deB, gestorA, ficha(o.a)) // não "puxa" de B
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/directory/people/"+semLotacao, gestorA, ficha(o.a))
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/people/"+semLotacao, admin, ficha(o.a)) // a global define a 1ª lotação

	// Perfis ocultos: visíveis só para a gestão que cobre a pessoa.
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people/"+deA, gestorA, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/directory/people/"+deB, gestorA, "")
	lista := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people?unidade_id="+o.a, gestorA, "").Body.String()
	if !strings.Contains(lista, deA) {
		t.Fatalf("a gestão de A vê o oculto de A na busca: %s", lista)
	}
	if outra := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people?unidade_id="+o.b, gestorA, "").Body.String(); strings.Contains(outra, deB) {
		t.Fatal("o oculto de B não aparece para a gestão de A")
	}
}
