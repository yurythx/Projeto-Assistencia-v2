package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// A01 — só administra uma conta quem cobre as permissões efetivas dela
// (lotações, grupos do AD e o papel nexus-admin). Sem isso, quem tem só
// users:manage redefiniria a senha de um administrador e entraria como ele.
func TestIAMAdministersOnlyCoveredAccounts(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	ctx := context.Background()
	sfx := uuid.NewString()[:6]
	perfil := func(slug string) string {
		var id string
		if err := h.d.DB.QueryRow(ctx, `SELECT id FROM perfis WHERE slug = $1`, slug).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	managerID, manager := h.user("nexus-user")
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+managerID.String()+"/lotacoes", admin, `{"perfil_id":"`+perfil("gestor-iam")+`"}`)

	adminID, _ := h.user("nexus-admin")
	auditorID, _ := h.user("nexus-user")
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+auditorID.String()+"/lotacoes", admin, `{"perfil_id":"`+perfil("auditor")+`"}`)
	comumID, _ := h.user("nexus-user")
	// Permissão vinda de grupo do AD também conta.
	grupoID, _ := h.user("nexus-user")
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/ad-mappings", admin, `{"ad_group":"GRP_AUD_`+sfx+`","perfil_id":"`+perfil("auditor")+`"}`)
	if _, err := h.d.DB.Exec(ctx, `UPDATE users SET groups = ARRAY['grp_aud_`+sfx+`'] WHERE id = $1`, grupoID); err != nil {
		t.Fatal(err)
	}

	reset := `{"password":"Outra-Senha-Forte-9"}`
	for name, id := range map[string]uuid.UUID{"administrador": adminID, "auditor (lotação)": auditorID, "auditor (grupo do AD)": grupoID} {
		h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/users/"+id.String()+"/password", manager, reset)
		h.expect(http.StatusForbidden, http.MethodPatch, "/api/v1/users/"+id.String(), manager, `{"display_name":"x","active":false}`)
		_ = name
	}
	h.expect(http.StatusNoContent, http.MethodPost, "/api/v1/users/"+comumID.String()+"/password", manager, reset)
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/users/"+comumID.String(), manager, `{"display_name":"Comum","active":true}`)
	h.expect(http.StatusNoContent, http.MethodPost, "/api/v1/users/"+auditorID.String()+"/password", admin, reset)
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/users/"+uuid.NewString()+"/password", admin, reset)

	// Conta federada não ganha senha local (nem pelo administrador).
	var fedID uuid.UUID
	if err := h.d.DB.QueryRow(ctx, `INSERT INTO users (username, email, keycloak_subject) VALUES ($1, $1 || '@ad.test', $1) RETURNING id`,
		"fed_"+sfx).Scan(&fedID); err != nil {
		t.Fatal(err)
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/users/"+fedID.String()+"/password", admin, reset)

	// Retirar também exige cobrir: lotação e mapeamento de perfil mais forte.
	lot := h.expect(http.StatusOK, http.MethodGet, "/api/v1/users/"+auditorID.String()+"/lotacoes", admin, "").Body.String()
	lotID := lot[strings.Index(lot, `"id":"`)+6:][:36]
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/users/"+auditorID.String()+"/lotacoes/"+lotID, manager, "")
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/users/"+comumID.String()+"/lotacoes/"+lotID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/users/"+auditorID.String()+"/lotacoes/"+lotID, admin, "")
	var meta string
	if err := h.d.DB.QueryRow(ctx, `SELECT COALESCE(diff_before::text, '') FROM audit_logs WHERE action = 'iam.lotacao.deleted' AND resource_id = $1`,
		auditorID.String()).Scan(&meta); err != nil || !strings.Contains(meta, perfil("auditor")) {
		t.Fatalf("a auditoria guarda o perfil retirado: %q %v", meta, err)
	}
	maps := h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/ad-mappings", admin, "").Body.String()
	i := strings.Index(maps, "GRP_AUD_"+sfx)
	mapID := maps[strings.LastIndex(maps[:i], `"id":"`)+6:][:36]
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/iam/ad-mappings/"+mapID, manager, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/ad-mappings/"+mapID, admin, "")
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/iam/ad-mappings/"+mapID, admin, "")

	// Perfil: retirar permissão ou desativar exige cobrir o que muda.
	forte := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
		`{"nome":"Forte `+sfx+`","permissoes":["audit:read","users:read"]}`))
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/iam/perfis/"+forte.ID, manager, `{"nome":"Forte","permissoes":["users:read"]}`)
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/iam/perfis/"+forte.ID, manager,
		`{"nome":"Forte","permissoes":["audit:read","users:read"],"ativo":false}`)
	fraco := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", manager, `{"nome":"Fraco `+sfx+`","permissoes":["users:read"],"ativo":true}`))
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/perfis/"+fraco.ID, manager, `{"nome":"Fraco `+sfx+` inativo","permissoes":[],"ativo":false}`)

	// Lotação para conta inexistente: 404 (não "escopo inválido").
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/users/"+uuid.NewString()+"/lotacoes", admin, `{"perfil_id":"`+perfil("servidor")+`"}`)
}

// Hierarquia organizacional consistente: unidade não troca de entidade,
// mãe precisa existir e ser da mesma entidade, sem ciclos; departamento
// não troca de unidade.
func TestIAMHierarchyRules(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	e1 := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"E1 `+sfx+`"}`))
	e2 := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"E2 `+sfx+`"}`))
	un := func(ent, nome, parent string) idResp {
		body := `{"entidade_id":"` + ent + `","nome":"` + nome + " " + sfx + `"`
		if parent != "" {
			body += `,"parent_id":"` + parent + `"`
		}
		return data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin, body+`}`))
	}
	raiz := un(e1.ID, "Raiz", "")
	filha := un(e1.ID, "Filha", raiz.ID)
	neta := un(e1.ID, "Neta", filha.ID)
	outra := un(e2.ID, "Outra", "")

	put := func(want int, id, body string) {
		h.expect(want, http.MethodPut, "/api/v1/iam/unidades/"+id, admin, body)
	}
	put(http.StatusUnprocessableEntity, raiz.ID, `{"entidade_id":"`+e2.ID+`","nome":"Raiz"}`)
	put(http.StatusUnprocessableEntity, raiz.ID, `{"entidade_id":"`+e1.ID+`","nome":"Raiz","parent_id":"`+neta.ID+`"}`)
	put(http.StatusUnprocessableEntity, raiz.ID, `{"entidade_id":"`+e1.ID+`","nome":"Raiz","parent_id":"`+raiz.ID+`"}`)
	put(http.StatusUnprocessableEntity, raiz.ID, `{"entidade_id":"`+e1.ID+`","nome":"Raiz","parent_id":"`+outra.ID+`"}`)
	put(http.StatusUnprocessableEntity, raiz.ID, `{"entidade_id":"`+e1.ID+`","nome":"Raiz","parent_id":"`+uuid.NewString()+`"}`)
	put(http.StatusNotFound, uuid.NewString(), `{"entidade_id":"`+e1.ID+`","nome":"Fantasma"}`)
	put(http.StatusOK, neta.ID, `{"entidade_id":"`+e1.ID+`","nome":"Neta promovida","parent_id":"`+raiz.ID+`"}`)

	dep := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", admin, `{"unidade_id":"`+raiz.ID+`","nome":"Dep `+sfx+`"}`))
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/iam/departamentos/"+dep.ID, admin, `{"unidade_id":"`+filha.ID+`","nome":"Dep"}`)
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/departamentos/"+dep.ID, admin, `{"unidade_id":"`+raiz.ID+`","nome":"Dep renomeado"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/iam/departamentos", admin, `{"unidade_id":"`+uuid.NewString()+`","nome":"Órfão"}`)
}

// Toda rota do IAM recusa identificador malformado (400) e corpo inválido
// (400/422) antes de tocar no banco.
func TestIAMRejectsMalformedInput(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	id := uuid.NewString()
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/iam/entidades/x", `{"nome":"a"}`},
		{http.MethodDelete, "/api/v1/iam/entidades/x", ""},
		{http.MethodPut, "/api/v1/iam/unidades/x", `{"nome":"a","entidade_id":"` + id + `"}`},
		{http.MethodDelete, "/api/v1/iam/unidades/x", ""},
		{http.MethodPut, "/api/v1/iam/departamentos/x", `{"nome":"a","unidade_id":"` + id + `"}`},
		{http.MethodDelete, "/api/v1/iam/departamentos/x", ""},
		{http.MethodPut, "/api/v1/iam/perfis/x", `{"nome":"a"}`},
		{http.MethodDelete, "/api/v1/iam/perfis/x", ""},
		{http.MethodDelete, "/api/v1/iam/ad-mappings/x", ""},
		{http.MethodGet, "/api/v1/users/x", ""},
		{http.MethodPatch, "/api/v1/users/x", `{"active":true}`},
		{http.MethodPost, "/api/v1/users/x/password", `{"password":"Senha-Forte-123!"}`},
		{http.MethodPost, "/api/v1/users/x/unlock", ""},
		{http.MethodGet, "/api/v1/users/x/lotacoes", ""},
		{http.MethodPost, "/api/v1/users/x/lotacoes", `{"perfil_id":"` + id + `"}`},
		{http.MethodDelete, "/api/v1/users/x/lotacoes/" + id, ""},
		{http.MethodDelete, "/api/v1/users/" + id + "/lotacoes/x", ""},
		{http.MethodGet, "/api/v1/iam/unidades?entidade_id=x", ""},
		{http.MethodGet, "/api/v1/iam/departamentos?unidade_id=x", ""},
		{http.MethodGet, "/api/v1/users?active=talvez", ""},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, admin, c.body)
	}
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/iam/entidades"}, {http.MethodPut, "/api/v1/iam/entidades/" + id},
		{http.MethodPost, "/api/v1/iam/unidades"}, {http.MethodPut, "/api/v1/iam/unidades/" + id},
		{http.MethodPost, "/api/v1/iam/departamentos"}, {http.MethodPut, "/api/v1/iam/departamentos/" + id},
		{http.MethodPost, "/api/v1/iam/perfis"}, {http.MethodPut, "/api/v1/iam/perfis/" + id},
		{http.MethodPost, "/api/v1/iam/ad-mappings"}, {http.MethodPost, "/api/v1/users"},
		{http.MethodPatch, "/api/v1/users/" + id}, {http.MethodPost, "/api/v1/users/" + id + "/password"},
		{http.MethodPost, "/api/v1/users/" + id + "/lotacoes"},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, admin, `{`)
	}
	// Filtros válidos e registros inexistentes.
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/users?active=false&q=ninguem", admin, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/unidades?entidade_id="+id, admin, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/departamentos?unidade_id="+id, admin, "")
	for _, p := range []string{"/api/v1/iam/entidades/", "/api/v1/iam/unidades/", "/api/v1/iam/departamentos/", "/api/v1/iam/perfis/"} {
		h.expect(http.StatusNotFound, http.MethodDelete, p+id, admin, "")
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/users/"+id, admin, "")
	h.expect(http.StatusNotFound, http.MethodPatch, "/api/v1/users/"+id, admin, `{"display_name":"x","active":true}`)
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/iam/perfis/"+id, admin, `{"nome":"Fantasma"}`)
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/iam/entidades/"+id, admin, `{"nome":"Fantasma"}`)
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/iam/departamentos/"+id, admin, `{"nome":"Fantasma","unidade_id":"`+id+`"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/iam/perfis", admin, `{"nome":"Ruim","permissoes":["Nao Vale"]}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/users", admin, `{"username":"a b","email":"a@b.gov.br","password":"Senha-Forte-123!"}`)
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/permissions", admin, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/entidades", admin, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/me", admin, "")
}
