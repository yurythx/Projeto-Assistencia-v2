package app

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
)

type route struct{ method, path string }

// pluginRoutes descobre TODAS as rotas que um plugin registra (públicas e
// autenticadas), montando-o isoladamente — nenhuma lista mantida à mão.
func pluginRoutes(t *testing.T, p kernel.Plugin) []route {
	t.Helper()
	rp, ok := p.(kernel.RouteProvider)
	if !ok {
		return nil
	}
	pub, authed := chi.NewRouter(), chi.NewRouter()
	rp.RegisterRoutes(kernel.Routes{Public: pub, Authed: authed})
	var out []route
	walk := func(r chi.Router) {
		_ = chi.Walk(r, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			out = append(out, route{method, "/api/v1" + strings.TrimSuffix(path, "/")})
			return nil
		})
	}
	walk(pub)
	walk(authed)
	sort.Slice(out, func(i, j int) bool { return out[i].method+out[i].path < out[j].method+out[j].path })
	return out
}

var urlParam = regexp.MustCompile(`\{[^}]+\}`)

func concrete(path string) string {
	return urlParam.ReplaceAllStringFunc(path, func(p string) string {
		if p == "{version}" {
			return "1"
		}
		return "00000000-0000-4000-8000-000000000001"
	})
}

// Para cada plugin desativável: desligar pela API faz TODA a sua superfície
// HTTP responder 404 MODULE_DISABLED na hora (sem restart); religar
// devolve a superfície. O núcleo nunca desliga.
func TestEveryPluginSurfaceFollowsActivation(t *testing.T) {
	h := newHarness(t)
	token := h.admin()

	for _, p := range h.d.Kernel.Plugins() {
		m := p.Manifest()
		routes := pluginRoutes(t, p)
		t.Run(m.Key, func(t *testing.T) {
			sub := &apiHarness{t: t, d: h.d, router: h.router}
			if m.Core {
				rec := sub.do(http.MethodPatch, "/api/v1/admin/modules/"+m.Key, token, `{"enabled":false}`)
				if rec.Code != http.StatusConflict {
					t.Fatalf("núcleo %s: desativar deveria dar 409, veio %d", m.Key, rec.Code)
				}
				return
			}
			if len(routes) == 0 {
				t.Fatalf("plugin %s sem rotas HTTP", m.Key)
			}

			sub.setModule(token, m.Key, false)
			for _, r := range routes {
				rec := sub.do(r.method, concrete(r.path), token, `{}`)
				if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "MODULE_DISABLED") {
					t.Errorf("%s desativado: %s %s deveria responder 404 MODULE_DISABLED, veio %d %s",
						m.Key, r.method, r.path, rec.Code, rec.Body.String())
				}
			}
			// O estado público acompanha (site institucional esconde a seção).
			if m.Public {
				body := sub.expect(http.StatusOK, http.MethodGet, "/api/v1/system/public-modules", "", "").Body.String()
				if !strings.Contains(body, `{"key":"`+m.Key+`","enabled":false}`) {
					t.Errorf("public-modules deveria marcar %s como desativado: %s", m.Key, body)
				}
			}

			sub.setModule(token, m.Key, true)
			for _, r := range routes {
				rec := sub.do(r.method, concrete(r.path), token, `{}`)
				if strings.Contains(rec.Body.String(), "MODULE_DISABLED") {
					t.Errorf("%s reativado: %s %s ainda responde MODULE_DISABLED", m.Key, r.method, r.path)
				}
			}
		})
	}
}

// O grafo de dependências é exposto pela API e aplicado nos dois sentidos.
func TestModuleDependenciesViaAPI(t *testing.T) {
	h := newHarness(t)
	token := h.admin()

	type status struct {
		Key        string   `json:"key"`
		DependsOn  []string `json:"depends_on"`
		Dependents []string `json:"dependents"`
		BlockedBy  []string `json:"blocked_by"`
		Enabled    bool     `json:"enabled"`
		Configured bool     `json:"configured"`
	}
	get := func() map[string]status {
		out := map[string]status{}
		for _, s := range data[[]status](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/system/modules", token, "")) {
			out[s.Key] = s
		}
		return out
	}

	st := get()
	if got := st["tramite"].DependsOn; len(got) != 1 || got[0] != "signum" {
		t.Fatalf("tramite deveria depender de signum: %v", got)
	}
	if got := st["signum"].Dependents; len(got) != 1 || got[0] != "tramite" {
		t.Fatalf("signum deveria listar tramite como dependente: %v", got)
	}

	// Com o Trâmite ativo, o Signum não desliga — e o erro diz por quê.
	h.setModule(token, "tramite", true)
	rec := h.expect(http.StatusConflict, http.MethodPatch, "/api/v1/admin/modules/signum", token, `{"enabled":false}`)
	if !strings.Contains(rec.Body.String(), "tramite") {
		t.Fatalf("409 deveria citar o dependente: %s", rec.Body.String())
	}

	// Desligando na ordem certa, o Trâmite não religa sem o Signum.
	h.setModule(token, "signum", false)
	st = get()
	if st["tramite"].Enabled || st["signum"].Enabled {
		t.Fatal("tramite e signum deveriam estar desativados")
	}
	rec = h.expect(http.StatusConflict, http.MethodPatch, "/api/v1/admin/modules/tramite", token, `{"enabled":true}`)
	if !strings.Contains(rec.Body.String(), "signum") {
		t.Fatalf("409 deveria citar a dependência: %s", rec.Body.String())
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/processos", token, "")

	h.setModule(token, "tramite", true) // liga signum antes, automaticamente
	st = get()
	if !st["signum"].Enabled || !st["tramite"].Enabled || len(st["tramite"].BlockedBy) != 0 {
		t.Fatalf("religados na ordem: %+v %+v", st["signum"], st["tramite"])
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos", token, "")
}

// Toda troca de estado vira evento na trilha de auditoria imutável.
func TestModuleToggleIsAudited(t *testing.T) {
	h := newHarness(t)
	token := h.admin()
	h.setModule(token, "wiki", false)
	var n int
	if err := h.d.DB.QueryRow(t.Context(),
		`SELECT count(*) FROM audit_logs WHERE action = $1 AND resource_id = 'wiki' AND created_at > now() - interval '1 minute'`, audit.ActionModuleToggled).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("desativar um módulo deveria gravar auditoria kernel.module.toggled")
	}
}
