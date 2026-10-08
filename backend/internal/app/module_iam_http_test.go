package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type idResp struct {
	ID string `json:"id"`
}

func TestIAMOrganizationalStructureHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, plain := h.user("nexus-user")
	sfx := uuid.NewString()[:6]

	// Leitura liberada a qualquer autenticado; escrita exige iam:manage.
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/org-tree", plain, "")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/iam/entidades", plain, `{"nome":"X"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":""}`)

	ent := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin,
		`{"nome":"Secretaria `+sfx+`","sigla":"S`+sfx+`"}`))
	un := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin,
		`{"entidade_id":"`+ent.ID+`","nome":"Unidade `+sfx+`","ad_group":"GRP_UN_`+sfx+`","email":"un@org.gov.br"}`))
	dep := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", admin,
		`{"unidade_id":"`+un.ID+`","nome":"Depto `+sfx+`"}`))
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/iam/unidades", admin,
		`{"entidade_id":"`+ent.ID+`","nome":"X","email":"não-é-email"}`)

	tree := h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/org-tree", plain, "").Body.String()
	for _, want := range []string{ent.ID, un.ID, dep.ID} {
		if !strings.Contains(tree, want) {
			t.Fatalf("org-tree deveria conter %s", want)
		}
	}
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/departamentos/"+dep.ID, admin,
		`{"unidade_id":"`+un.ID+`","nome":"Depto renomeado `+sfx+`"}`)

	// Não apaga a entidade com unidades embaixo; apaga de baixo para cima.
	h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/iam/entidades/"+ent.ID, admin, "")
	h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/iam/unidades/"+un.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/departamentos/"+dep.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/unidades/"+un.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/entidades/"+ent.ID, admin, "")
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/iam/entidades/"+ent.ID, admin, "")
}

// Lotação concede permissão em tempo real (cache do IAM invalidado) e a
// remoção retira — sem novo login.
func TestIAMProfilesAndLotacoesGrantAtRuntime(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	userID, tok := h.user("nexus-user")
	sfx := uuid.NewString()[:6]

	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/iam/perfis", admin, `{"nome":"Ruim","permissoes":["sem formato"]}`)
	perfil := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
		`{"nome":"Auditor `+sfx+`","permissoes":["audit:read","AUDIT:read"]}`))
	perms := h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/perfis", admin, "").Body.String()
	if !strings.Contains(perms, `"permissoes":["audit:read"]`) {
		t.Fatalf("permissões deveriam ser normalizadas e deduplicadas: %s", perms)
	}
	catalog := h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/permissions", tok, "").Body.String()
	if !strings.Contains(catalog, `"audit:read"`) || !strings.Contains(catalog, `"tramite:create"`) {
		t.Fatalf("catálogo deveria agregar as permissões dos manifests: %s", catalog)
	}

	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/audit/logs", tok, "")
	lot := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+userID.String()+"/lotacoes", admin,
		`{"perfil_id":"`+perfil.ID+`","principal":true}`))
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/audit/logs", tok, "")
	me := h.expect(http.StatusOK, http.MethodGet, "/api/v1/me", tok, "").Body.String()
	if !strings.Contains(me, `"audit:read"`) || !strings.Contains(me, `"origem":"manual"`) {
		t.Fatalf("/me deveria refletir a lotação: %s", me)
	}

	// Perfil em uso não é excluído; perfil de sistema nunca.
	h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/iam/perfis/"+perfil.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/users/"+userID.String()+"/lotacoes/"+lot.ID, admin, "")
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/audit/logs", tok, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/perfis/"+perfil.ID, admin, "")

	var sysID string
	if err := h.d.DB.QueryRow(context.Background(), `SELECT id FROM perfis WHERE sistema LIMIT 1`).Scan(&sysID); err == nil {
		h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/iam/perfis/"+sysID, admin, "")
	}
}

func TestIAMADMappingsHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	perfil := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
		`{"nome":"Leitor `+sfx+`","permissoes":["users:read"]}`))

	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/iam/ad-mappings", admin, `{"perfil_id":"`+perfil.ID+`"}`)
	m := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/ad-mappings", admin,
		`{"ad_group":"GRP_LEITORES_`+sfx+`","perfil_id":"`+perfil.ID+`","descricao":"teste"}`))
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/iam/ad-mappings", admin,
		`{"ad_group":"GRP_LEITORES_`+sfx+`","perfil_id":"`+perfil.ID+`"}`)
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/iam/ad-mappings", admin, "").Body.String(); !strings.Contains(body, m.ID) {
		t.Fatal("mapeamento criado deveria ser listado")
	}
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/iam/ad-mappings/"+m.ID, admin, "")
}

func TestIAMUserManagementHTTP(t *testing.T) {
	h := newHarness(t)
	adminID, admin := h.user("nexus-admin")
	_, plain := h.user("nexus-user")
	sfx := strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/users", plain, "")
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/users", admin,
		`{"username":"fraca_`+sfx+`","email":"f`+sfx+`@org.gov.br","password":"curta"}`)
	created := data[struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users", admin,
		`{"username":"novo_`+sfx+`","email":"Novo`+sfx+`@Org.gov.br","display_name":"Novo","password":"Senha-Muito-Forte-1"}`))

	list := h.expect(http.StatusOK, http.MethodGet, "/api/v1/users?q=novo_"+sfx, admin, "").Body.String()
	if !strings.Contains(list, created.ID) || !strings.Contains(list, `"total_items":1`) {
		t.Fatalf("busca de usuários: %s", list)
	}
	// E-mail normalizado para minúsculas.
	if u := h.expect(http.StatusOK, http.MethodGet, "/api/v1/users/"+created.ID, admin, "").Body.String(); !strings.Contains(u, `"novo`+sfx+`@org.gov.br"`) {
		t.Fatalf("e-mail deveria ser normalizado: %s", u)
	}

	// Login com a senha inicial, redefinição e login com a nova.
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/auth/login", "", `{"username":"novo_`+sfx+`","password":"Senha-Muito-Forte-1"}`)
	h.expect(http.StatusNoContent, http.MethodPost, "/api/v1/users/"+created.ID+"/password", admin, `{"password":"Outra-Senha-Forte-2"}`)
	h.expect(http.StatusUnauthorized, http.MethodPost, "/api/v1/auth/login", "", `{"username":"novo_`+sfx+`","password":"Senha-Muito-Forte-1"}`)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/auth/login", "", `{"username":"novo_`+sfx+`","password":"Outra-Senha-Forte-2"}`)

	// Desativar bloqueia o acesso na hora, mesmo com token válido.
	login := data[struct {
		AccessToken string `json:"access_token"`
	}](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/auth/login", "", `{"username":"novo_`+sfx+`","password":"Outra-Senha-Forte-2"}`))
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/me", login.AccessToken, "")
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/users/"+created.ID, admin, `{"display_name":"Novo","active":false,"roles":[]}`)
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/me", login.AccessToken, "")

	// Ninguém desativa a própria conta.
	h.expect(http.StatusConflict, http.MethodPatch, "/api/v1/users/"+adminID.String(), admin, `{"display_name":"Eu","active":false,"roles":["nexus-admin"]}`)
}

// A01 — escalada de privilégio: um gestor de usuários/IAM sem acesso total
// não concede o papel nexus-admin nem permissões que não possui.
func TestIAMPreventsPrivilegeEscalation(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := uuid.NewString()[:6]
	gestor := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
		`{"nome":"Gestor `+sfx+`","permissoes":["users:read","users:manage","iam:manage"]}`))
	managerID, manager := h.user("nexus-user")
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+managerID.String()+"/lotacoes", admin, `{"perfil_id":"`+gestor.ID+`"}`)

	// Promover a si mesmo a nexus-admin: 403.
	h.expect(http.StatusForbidden, http.MethodPatch, "/api/v1/users/"+managerID.String(), manager,
		`{"display_name":"Eu","active":true,"roles":["nexus-admin"]}`)
	// Criar conta já administradora: 403.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/users", manager,
		`{"username":"esc_`+sfx+`","email":"esc`+sfx+`@org.gov.br","password":"Senha-Muito-Forte-1","roles":["nexus-admin"]}`)
	// Criar perfil com permissão que não tem ("*" ou audit:read): 403.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/iam/perfis", manager, `{"nome":"Tudo `+sfx+`","permissoes":["*"]}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/iam/perfis", manager, `{"nome":"Aud `+sfx+`","permissoes":["audit:read"]}`)
	// Lotar-se num perfil mais poderoso criado pelo admin: 403.
	root := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin, `{"nome":"Root `+sfx+`","permissoes":["*"]}`))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/users/"+managerID.String()+"/lotacoes", manager, `{"perfil_id":"`+root.ID+`"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/iam/ad-mappings", manager, `{"ad_group":"GRP_ROOT_`+sfx+`","perfil_id":"`+root.ID+`"}`)
	// Dentro do que possui, pode.
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", manager, `{"nome":"Leitor `+sfx+`","permissoes":["users:read"]}`)
	// O admin (acesso total) pode tudo isso.
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/users/"+managerID.String(), admin, `{"display_name":"Gestor","active":true,"roles":["nexus-admin"]}`)
}

// Desbloqueio administrativo libera o login também no lockout distribuído.
func TestIAMUnlockClearsDistributedLockout(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	u := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users", admin,
		`{"username":"lock_`+sfx+`","email":"lock`+sfx+`@org.gov.br","password":"Senha-Muito-Forte-1"}`))

	var locked bool
	for range 12 {
		rec := h.do(http.MethodPost, "/api/v1/auth/login", "", `{"username":"lock_`+sfx+`","password":"errada-errada-1"}`)
		if rec.Code == http.StatusTooManyRequests {
			locked = true
			break
		}
	}
	if !locked {
		t.Fatal("falhas seguidas deveriam acionar o lockout progressivo (429)")
	}
	// Outro IP: o bloqueio é da CONTA (usuário), não só do endereço.
	good := `{"username":"lock_` + sfx + `","password":"Senha-Muito-Forte-1"}`
	if rec := h.doFrom("198.51.100.7:1", http.MethodPost, "/api/v1/auth/login", "", good); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("conta bloqueada deveria recusar mesmo de outro IP: %d", rec.Code)
	}
	h.expect(http.StatusNoContent, http.MethodPost, "/api/v1/users/"+u.ID+"/unlock", admin, "")
	if rec := h.doFrom("198.51.100.8:1", http.MethodPost, "/api/v1/auth/login", "", good); rec.Code != http.StatusOK {
		t.Fatalf("após o desbloqueio o login deveria funcionar: %d %s", rec.Code, rec.Body.String())
	}
}
