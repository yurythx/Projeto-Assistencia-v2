package app

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// relogin entra de novo com a conta local (o token do login local leva os
// grupos da conta, como o do Keycloak leva os do AD).
func (h *apiHarness) relogin(id uuid.UUID) string {
	h.t.Helper()
	var username string
	if err := h.d.DB.QueryRow(context.Background(), `SELECT username FROM users WHERE id = $1`, id).Scan(&username); err != nil {
		h.t.Fatal(err)
	}
	rec := h.do(http.MethodPost, "/api/v1/auth/login", "", `{"username":"`+username+`","password":"Senha-Forte-123!"}`)
	if rec.Code != http.StatusOK {
		h.t.Fatalf("login %s: %d %s", username, rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Data.AccessToken
}

// Estrutura desativada deixa de conceder (migration 000126,
// docs/REGRAS_DE_NEGOCIO.md §2): lotações e mapeamentos de grupo num
// escopo com entidade, unidade ou departamento desativado não valem — e a
// mudança é imediata (salvar a estrutura invalida o cache do IAM).
func TestIAMEscopoDesativadoNaoConcede(t *testing.T) {
	h := newHarness(t)
	// blog:manage vale com escopo (ADR 013): aparece em /me enquanto o
	// escopo da concessão estiver ativo.
	quer := func(tok string, want bool) {
		t.Helper()
		var me struct {
			Data struct {
				Permissions []string `json:"permissions"`
			} `json:"data"`
		}
		_ = json.Unmarshal(h.expect(http.StatusOK, http.MethodGet, "/api/v1/me", tok, "").Body.Bytes(), &me)
		if got := slices.Contains(me.Data.Permissions, "blog:manage"); got != want {
			t.Fatalf("blog:manage em /me = %v, esperado %v (%v)", got, want, me.Data.Permissions)
		}
	}
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	ent := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"Órgão `+sfx+`"}`))
	un := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin,
		`{"entidade_id":"`+ent.ID+`","nome":"Unidade `+sfx+`"}`))
	dep := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", admin,
		`{"unidade_id":"`+un.ID+`","nome":"Depto `+sfx+`"}`))
	perfil := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
		`{"nome":"Editor `+sfx+`","permissoes":["blog:manage"]}`))
	setAtivo := func(kind, id, body string) {
		t.Helper()
		h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/"+kind+"/"+id, admin, body)
	}

	// Lotação manual no departamento (o escopo é completado até a entidade).
	userID, tok := h.user("nexus-user")
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+userID.String()+"/lotacoes", admin,
		`{"perfil_id":"`+perfil.ID+`","departamento_id":"`+dep.ID+`"}`)
	quer(tok, true)

	setAtivo("departamentos", dep.ID, `{"unidade_id":"`+un.ID+`","nome":"Depto `+sfx+`","ativo":false}`)
	quer(tok, false)
	setAtivo("departamentos", dep.ID, `{"unidade_id":"`+un.ID+`","nome":"Depto `+sfx+`","ativo":true}`)
	quer(tok, true)

	// Entidade desativada derruba tudo o que está abaixo dela.
	setAtivo("entidades", ent.ID, `{"nome":"Órgão `+sfx+`","ativo":false}`)
	quer(tok, false)
	setAtivo("entidades", ent.ID, `{"nome":"Órgão `+sfx+`","ativo":true}`)
	quer(tok, true)

	// Mapeamento de grupo com escopo na unidade: mesma regra.
	grupo := "GRP_ESCOPO_" + sfx
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/ad-mappings", admin,
		`{"ad_group":"`+grupo+`","perfil_id":"`+perfil.ID+`","unidade_id":"`+un.ID+`"}`)
	membroID, _ := h.user("nexus-user")
	if _, err := h.d.DB.Exec(context.Background(), `UPDATE users SET groups = $2 WHERE id = $1`, membroID, []string{grupo}); err != nil {
		t.Fatal(err)
	}
	membro := h.relogin(membroID)
	quer(membro, true)
	setAtivo("unidades", un.ID, `{"entidade_id":"`+ent.ID+`","nome":"Unidade `+sfx+`","ativo":false}`)
	quer(membro, false)
	quer(tok, false)
	setAtivo("unidades", un.ID, `{"entidade_id":"`+ent.ID+`","nome":"Unidade `+sfx+`","ativo":true}`)
	quer(membro, true)
}

// Permissão de plataforma só vale com concessão global (ADR 013): lotar
// alguém como auditor NUMA unidade não abre a auditoria da plataforma.
func TestIAMPermissaoDePlataformaSoGlobal(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	ent := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"Órgão `+sfx+`"}`))
	un := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin,
		`{"entidade_id":"`+ent.ID+`","nome":"Unidade `+sfx+`"}`))
	perfil := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
		`{"nome":"Monitor `+sfx+`","permissoes":["monitoring:read"]}`))

	userID, tok := h.user("nexus-user")
	lot := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+userID.String()+"/lotacoes", admin,
		`{"perfil_id":"`+perfil.ID+`","unidade_id":"`+un.ID+`"}`))
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/monitoring/outbox-stats", tok, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/users/"+userID.String()+"/lotacoes/"+lot.ID, admin, "")
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+userID.String()+"/lotacoes", admin, `{"perfil_id":"`+perfil.ID+`"}`)
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/monitoring/outbox-stats", tok, "")
}

// Auditoria com escopo (ADR 013, fase 3): audit:read numa unidade vê as
// ações de quem estava lotado na área (ela e as subunidades) quando agiu —
// na consulta, no detalhe e na exportação. Verificar a cadeia segue global.
func TestAuditoriaComEscopo(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	o := h.orgEscopo(t)
	auditorA := h.gestorEm(t, o.a, "audit:read", "audit:verify")
	naSub := h.gestorEm(t, o.sub, "blog:manage")
	emB := h.gestorEm(t, o.b, "blog:manage")
	post := func(tok, unidade string) string {
		return data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", tok,
			`{"title":"Post `+uuid.NewString()[:8]+`","summary":"s","body":"b","unidade_id":"`+unidade+`"}`)).ID
	}
	daSub, deB := post(naSub, o.sub), post(emB, o.b)
	type logResp struct {
		ID string `json:"id"`
	}
	logs := func(tok, resource string) []logResp {
		return data[[]logResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/audit/logs?action=blog.post.created&resource_id="+resource, tok, ""))
	}
	if len(logs(auditorA, daSub)) != 1 || len(logs(auditorA, deB)) != 0 {
		t.Fatal("auditor de A vê as ações de quem é da subunidade, não as de B")
	}
	registroB := logs(admin, deB)
	if len(registroB) != 1 {
		t.Fatal("a gestão global vê tudo")
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/audit/logs/"+registroB[0].ID, auditorA, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/audit/logs/"+logs(auditorA, daSub)[0].ID, auditorA, "")
	hoje := time.Now().UTC().Format("2006-01-02")
	amanha := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	export := h.expect(http.StatusOK, http.MethodGet, "/api/v1/audit/export?format=json&action=blog.post.created&from="+hoje+"&to="+amanha, auditorA, "").Body.String()
	if !strings.Contains(export, daSub) || strings.Contains(export, deB) {
		t.Fatalf("exportação (LAI) restrita à área: %s", export)
	}
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/audit/verify", auditorA, "")
}

// Excluir a unidade-mãe com subunidades é recusado (antes as subunidades
// perdiam a mãe em silêncio).
func TestIAMUnidadeMaeComSubunidadesNaoEExcluida(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	ent := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"Rede `+sfx+`"}`))
	mae := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin,
		`{"entidade_id":"`+ent.ID+`","nome":"Sede `+sfx+`"}`))
	filha := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin,
		`{"entidade_id":"`+ent.ID+`","parent_id":"`+mae.ID+`","nome":"Filial `+sfx+`"}`))

	h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/iam/unidades/"+mae.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/unidades/"+filha.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/unidades/"+mae.ID, admin, "")
}

// orgEscopo monta, para os testes de permissão com escopo (ADR 013), uma
// entidade com as unidades A, B e uma subunidade de A.
type orgEscopo struct{ ent, a, sub, b string }

func (h *apiHarness) orgEscopo(t *testing.T) orgEscopo {
	t.Helper()
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	var o orgEscopo
	o.ent = data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"Órgão `+sfx+`"}`)).ID
	unidade := func(nome, parent string) string {
		body := `{"entidade_id":"` + o.ent + `","nome":"` + nome + ` ` + sfx + `"`
		if parent != "" {
			body += `,"parent_id":"` + parent + `"`
		}
		return data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin, body+`}`)).ID
	}
	o.a = unidade("Unidade A", "")
	o.b = unidade("Unidade B", "")
	o.sub = unidade("Subunidade de A", o.a)
	return o
}

// gestorEm cria um usuário lotado, na unidade dada, com um perfil que tem
// as permissões dadas; devolve o token.
func (h *apiHarness) gestorEm(t *testing.T, unidade string, permissoes ...string) string {
	t.Helper()
	admin := h.admin()
	perms, _ := json.Marshal(permissoes)
	perfil := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
		`{"nome":"Gestor `+uuid.NewString()[:8]+`","permissoes":`+string(perms)+`}`)).ID
	id, tok := h.user("nexus-user")
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+id.String()+"/lotacoes", admin,
		`{"perfil_id":"`+perfil+`","unidade_id":"`+unidade+`"}`)
	return tok
}

// IAM delegado (ADR 013, fase 3): iam:manage / users:* numa unidade
// administram a estrutura abaixo dela, as lotações e as contas da área —
// sem entidades, perfis ou mapeamentos do AD, e sem conceder o que o
// delegado não tem ali.
func TestIAMDelegado(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	o := h.orgEscopo(t)
	sfx := uuid.NewString()[:6]
	delegado := h.gestorEm(t, o.a, "iam:manage", "users:read", "users:manage", "blog:manage")
	perfil := func(perms string) string {
		return data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
			`{"nome":"P `+uuid.NewString()[:8]+`","permissoes":`+perms+`}`)).ID
	}
	perfilBlog, perfilPlataforma := perfil(`["blog:manage"]`), perfil(`["modules:manage"]`)
	unidade := func(want int, parent, nome string) string {
		body := `{"entidade_id":"` + o.ent + `","nome":"` + nome + ` ` + sfx + `"`
		if parent != "" {
			body += `,"parent_id":"` + parent + `"`
		}
		rec := h.expect(want, http.MethodPost, "/api/v1/iam/unidades", delegado, body+`}`)
		if want != http.StatusCreated {
			return ""
		}
		return data[idResp](t, rec).ID
	}

	// Estrutura: abaixo de A, sim; no nível da entidade e em B, não.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/iam/entidades", delegado, `{"nome":"Outra `+sfx+`"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/iam/perfis", delegado, `{"nome":"X `+sfx+`","permissoes":["blog:manage"]}`)
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/iam/ad-mappings", delegado, "")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/iam/ad-mappings", delegado, `{"ad_group":"G-`+sfx+`","perfil_id":"`+perfilBlog+`","unidade_id":"`+o.a+`"}`)
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/iam/ad-mappings/"+uuid.NewString(), delegado, "")
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/iam/perfis/"+perfilBlog, delegado, "")
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/iam/entidades/"+o.ent, delegado, "")
	nova := unidade(http.StatusCreated, o.a, "Nova")
	unidade(http.StatusForbidden, "", "Topo")
	unidade(http.StatusForbidden, o.b, "Em B")
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/unidades/"+o.sub, delegado, `{"entidade_id":"`+o.ent+`","parent_id":"`+o.a+`","nome":"Sub renomeada `+sfx+`"}`)
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/iam/unidades/"+o.sub, delegado, `{"entidade_id":"`+o.ent+`","parent_id":"`+o.b+`","nome":"Sub movida `+sfx+`"}`)
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/iam/unidades/"+o.b, delegado, `{"entidade_id":"`+o.ent+`","nome":"B tomada `+sfx+`"}`)
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/iam/unidades/"+o.a, delegado, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/unidades/"+nova, delegado, "")
	dep := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", delegado, `{"unidade_id":"`+o.sub+`","nome":"Protocolo `+sfx+`"}`)).ID
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/iam/departamentos", delegado, `{"unidade_id":"`+o.b+`","nome":"Protocolo `+sfx+`"}`)
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/departamentos/"+dep, delegado, "")
	depB := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", admin, `{"unidade_id":"`+o.b+`","nome":"Protocolo `+sfx+`"}`)).ID
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/iam/departamentos/"+depB, delegado, "")

	// Contas: as sem lotação e as da área aparecem; as de fora, não.
	novoID, _ := h.user("nexus-user")
	deBID, _ := h.user("nexus-user")
	adminID, _ := h.user("nexus-admin")
	lotar := func(want int, tok string, user uuid.UUID, perfilID, unidadeID string) string {
		body := `{"perfil_id":"` + perfilID + `"`
		if unidadeID != "" {
			body += `,"unidade_id":"` + unidadeID + `"`
		}
		rec := h.expect(want, http.MethodPost, "/api/v1/users/"+user.String()+"/lotacoes", tok, body+`}`)
		if want != http.StatusCreated {
			return ""
		}
		return data[idResp](t, rec).ID
	}
	emB := lotar(http.StatusCreated, admin, deBID, perfilBlog, o.b)
	lotar(http.StatusCreated, admin, deBID, perfilBlog, o.sub)
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/users/"+novoID.String(), delegado, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/users/"+deBID.String(), delegado, "") // tem uma lotação na área
	if lista := h.expect(http.StatusOK, http.MethodGet, "/api/v1/users?page_size=100&q=t_", delegado, "").Body.String(); !strings.Contains(lista, novoID.String()) {
		t.Fatalf("conta sem lotação aparece para o delegado: %s", lista)
	}
	if lotacoes := data[[]idResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/users/"+deBID.String()+"/lotacoes", delegado, "")); len(lotacoes) != 1 {
		t.Fatalf("o delegado vê só as lotações da área: %v", lotacoes)
	}
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/users/"+deBID.String()+"/lotacoes/"+emB, delegado, "")
	h.expect(http.StatusForbidden, http.MethodPatch, "/api/v1/users/"+deBID.String(), delegado, `{"display_name":"B","active":true}`)
	soB, _ := h.user("nexus-user")
	lotar(http.StatusCreated, admin, soB, perfilBlog, o.b)
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/users/"+soB.String(), delegado, "")

	// Lotar: na área, com perfil que o delegado tem ali.
	lotar(http.StatusForbidden, delegado, novoID, perfilBlog, "")
	lotar(http.StatusForbidden, delegado, novoID, perfilBlog, o.b)
	lotar(http.StatusForbidden, delegado, novoID, perfilPlataforma, o.sub)
	lot := lotar(http.StatusCreated, delegado, novoID, perfilBlog, o.sub)

	// Administrar a conta: só a que está inteiramente na área.
	h.expect(http.StatusNoContent, http.MethodPost, "/api/v1/users/"+novoID.String()+"/password", delegado, `{"password":"Outra-Senha-Forte-9"}`)
	h.expect(http.StatusNoContent, http.MethodPost, "/api/v1/users/"+novoID.String()+"/unlock", delegado, "")
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/users/"+novoID.String(), delegado, `{"display_name":"Nova pessoa","active":true,"roles":["nexus-user"]}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/users/"+deBID.String()+"/unlock", delegado, "")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/users/"+adminID.String()+"/password", delegado, `{"password":"Outra-Senha-Forte-9"}`)
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/users/"+novoID.String()+"/lotacoes/"+lot, delegado, "")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/users/"+novoID.String()+"/unlock", delegado, "") // sem lotação: fora
}
