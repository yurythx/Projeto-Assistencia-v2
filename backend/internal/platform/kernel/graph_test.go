package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// chain monta a -> b -> c (a depende de b, b depende de c) + um núcleo.
func chain(t *testing.T) (*Kernel, *memStore) {
	t.Helper()
	return newKernel(t,
		&fakePlugin{m: Manifest{Key: "core", Core: true}},
		&fakePlugin{m: Manifest{Key: "cc", DefaultEnabled: true}},
		&fakePlugin{m: Manifest{Key: "bb", DefaultEnabled: true, DependsOn: []string{"cc"}}},
		&fakePlugin{m: Manifest{Key: "aa", DefaultEnabled: true, DependsOn: []string{"bb"}}},
	)
}

func statusOf(k *Kernel, key string) ModuleStatus {
	for _, s := range k.Status() {
		if s.Key == key {
			return s
		}
	}
	return ModuleStatus{}
}

func TestStatusExposesDependencyGraphBothWays(t *testing.T) {
	k, _ := chain(t)
	cc := statusOf(k, "cc")
	if len(cc.Dependents) != 1 || cc.Dependents[0] != "bb" {
		t.Fatalf("cc.Dependents = %v, quero [bb]", cc.Dependents)
	}
	aa := statusOf(k, "aa")
	if len(aa.DependsOn) != 1 || aa.DependsOn[0] != "bb" || len(aa.Dependents) != 0 {
		t.Fatalf("aa: depends_on=%v dependents=%v", aa.DependsOn, aa.Dependents)
	}
	for _, s := range k.Status() {
		if !s.Enabled || !s.Configured || len(s.BlockedBy) != 0 {
			t.Fatalf("%s deveria estar ativo e desbloqueado: %+v", s.Key, s)
		}
		if s.DependsOn == nil || s.Dependents == nil || s.BlockedBy == nil || s.Permissions == nil {
			t.Fatalf("%s: listas nunca podem ser null no JSON", s.Key)
		}
	}
}

func TestDisableOrderMustFollowGraph(t *testing.T) {
	k, _ := chain(t)
	ctx := context.Background()

	// cc tem bb ativo por cima: recusa e nomeia o bloqueador.
	err := k.SetEnabled(ctx, "cc", false, "t")
	if !errors.Is(err, ErrDependentActive) || !strings.Contains(err.Error(), `"bb"`) {
		t.Fatalf("esperado ErrDependentActive citando bb, veio %v", err)
	}
	// Desligando de cima para baixo funciona.
	for _, key := range []string{"aa", "bb", "cc"} {
		if err := k.SetEnabled(ctx, key, false, "t"); err != nil {
			t.Fatalf("desativar %s: %v", key, err)
		}
	}
	// Religar fora de ordem é recusado citando a dependência que falta.
	err = k.SetEnabled(ctx, "aa", true, "t")
	if !errors.Is(err, ErrDependencyMissing) || !strings.Contains(err.Error(), `"bb"`) {
		t.Fatalf("esperado ErrDependencyMissing citando bb, veio %v", err)
	}
	for _, key := range []string{"cc", "bb", "aa"} {
		if err := k.SetEnabled(ctx, key, true, "t"); err != nil {
			t.Fatalf("reativar %s: %v", key, err)
		}
	}
	if !k.Enabled("aa") {
		t.Fatal("aa deveria voltar ativo")
	}
}

func TestDisableErrorListsEveryActiveDependent(t *testing.T) {
	k, _ := newKernel(t,
		&fakePlugin{m: Manifest{Key: "base", DefaultEnabled: true}},
		&fakePlugin{m: Manifest{Key: "uno", DefaultEnabled: true, DependsOn: []string{"base"}}},
		&fakePlugin{m: Manifest{Key: "dos", DefaultEnabled: true, DependsOn: []string{"base"}}},
	)
	err := k.SetEnabled(context.Background(), "base", false, "t")
	if err == nil || !strings.Contains(err.Error(), `"uno", "dos"`) {
		t.Fatalf("erro deveria listar uno e dos em ordem de registro: %v", err)
	}
}

// Estado gravado por outra réplica (ou direto no banco) pode deixar um
// módulo configurado como ativo com a dependência desligada: o estado
// efetivo é falso, o Guard responde 404 e o status explica o motivo.
func TestTransitiveDisableFromAnotherReplica(t *testing.T) {
	k, st := chain(t)
	var mu sync.Mutex
	changed := map[string]bool{}
	k.OnChange(func(_ context.Context, key string, enabled bool) {
		mu.Lock()
		changed[key] = enabled
		mu.Unlock()
	})

	st.mu.Lock()
	st.state["cc"] = false
	st.mu.Unlock()
	if err := k.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"aa", "bb", "cc"} {
		if k.Enabled(key) {
			t.Fatalf("%s deveria estar efetivamente inativo", key)
		}
		if v, ok := changed[key]; !ok || v {
			t.Fatalf("hook de %s deveria ter disparado com enabled=false (%v)", key, changed)
		}
	}
	aa := statusOf(k, "aa")
	if !aa.Configured || aa.Enabled || len(aa.BlockedBy) != 1 || aa.BlockedBy[0] != "bb" {
		t.Fatalf("aa deveria estar configurado, inativo e bloqueado por bb: %+v", aa)
	}
	if !k.Enabled("core") {
		t.Fatal("núcleo nunca desliga")
	}
}

func TestWatchAppliesNotifyFromOtherReplicas(t *testing.T) {
	k, st := chain(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = k.Watch(ctx) }()

	st.mu.Lock()
	st.state["aa"] = false
	st.mu.Unlock()
	st.notify <- struct{}{}

	deadline := time.Now().Add(2 * time.Second)
	for k.Enabled("aa") {
		if time.Now().After(deadline) {
			t.Fatal("NOTIFY não aplicou a desativação")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type searchPlugin struct {
	fakePlugin
	name string
}

type namedProvider string

func (n namedProvider) Module() string { return string(n) }
func (n namedProvider) Search(context.Context, auth.Identity, string, int) ([]search.Result, error) {
	return nil, nil
}

func (p *searchPlugin) SearchProviders() []search.Provider {
	return []search.Provider{namedProvider(p.name)}
}

func TestSearchProvidersFollowActivation(t *testing.T) {
	k, _ := newKernel(t,
		&searchPlugin{fakePlugin: fakePlugin{m: Manifest{Key: "wiki", DefaultEnabled: true}}, name: "wiki"},
		&searchPlugin{fakePlugin: fakePlugin{m: Manifest{Key: "blog", DefaultEnabled: true}}, name: "blog"},
	)
	if n := len(k.SearchProviders()); n != 2 {
		t.Fatalf("2 providers ativos esperados, veio %d", n)
	}
	if err := k.SetEnabled(context.Background(), "wiki", false, "t"); err != nil {
		t.Fatal(err)
	}
	ps := k.SearchProviders()
	if len(ps) != 1 || ps[0].Module() != "blog" {
		t.Fatalf("só o blog deveria sobrar na busca: %v", ps)
	}
}

func TestDefaultEnabledOnlyAppliesOnFirstBoot(t *testing.T) {
	st := newMemStore()
	st.state["off_by_admin"] = false
	k := New(st, quiet())
	k.MustRegister(
		&fakePlugin{m: Manifest{Key: "off_by_admin", DefaultEnabled: true}},
		&fakePlugin{m: Manifest{Key: "brand_new", DefaultEnabled: false}},
	)
	if err := k.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if k.Enabled("off_by_admin") {
		t.Fatal("estado gravado pelo administrador deve prevalecer sobre o default")
	}
	if k.Enabled("brand_new") {
		t.Fatal("módulo novo com DefaultEnabled=false nasce desativado")
	}
	if err := k.RegisterModule(&fakePlugin{m: Manifest{Key: "late"}}); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("registro após Start deveria falhar: %v", err)
	}
}

// Handlers HTTP do Kernel: lista, público e toggle com os códigos certos.
func TestModuleHandlers(t *testing.T) {
	k, _ := newKernel(t,
		&fakePlugin{m: Manifest{Key: "core", Core: true}},
		&fakePlugin{m: Manifest{Key: "site", DefaultEnabled: true, Public: true}},
		&fakePlugin{m: Manifest{Key: "base", DefaultEnabled: true}},
		&fakePlugin{m: Manifest{Key: "top", DefaultEnabled: true, DependsOn: []string{"base"}}},
	)
	h := NewHandlers(k, quiet())
	withPerm := func(perms ...string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := auth.Identity{Username: "tester", Permissions: perms}
				next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), id)))
			})
		}
	}
	serve := func(perms []string, method, path, body string) *httptest.ResponseRecorder {
		r := chi.NewRouter()
		r.Use(withPerm(perms...))
		h.RegisterAnonymousRoutes(r)
		h.RegisterRoutes(r)
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	admin := []string{"modules:manage"}

	rec := serve(nil, http.MethodGet, "/system/public-modules", "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"base"`) || !strings.Contains(rec.Body.String(), `"site"`) {
		t.Fatalf("public-modules deveria listar só módulos públicos: %d %s", rec.Code, rec.Body.String())
	}

	rec = serve(nil, http.MethodGet, "/system/modules", "")
	var list struct {
		Data []ModuleStatus `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != http.StatusOK || len(list.Data) != 4 {
		t.Fatalf("lista de módulos: %d %s", rec.Code, rec.Body.String())
	}

	cases := []struct {
		name  string
		perms []string
		path  string
		body  string
		want  int
	}{
		{"sem permissão", nil, "/admin/modules/site", `{"enabled":false}`, http.StatusForbidden},
		{"corpo inválido", admin, "/admin/modules/site", `{}`, http.StatusUnprocessableEntity},
		{"desconhecido", admin, "/admin/modules/nope", `{"enabled":false}`, http.StatusNotFound},
		{"núcleo", admin, "/admin/modules/core", `{"enabled":false}`, http.StatusConflict},
		{"dependente ativo", admin, "/admin/modules/base", `{"enabled":false}`, http.StatusConflict},
		{"ok", admin, "/admin/modules/top", `{"enabled":false}`, http.StatusOK},
		{"agora a base desliga", admin, "/admin/modules/base", `{"enabled":false}`, http.StatusOK},
		{"sem a base não religa", admin, "/admin/modules/top", `{"enabled":true}`, http.StatusConflict},
	}
	for _, c := range cases {
		if rec := serve(c.perms, http.MethodPatch, c.path, c.body); rec.Code != c.want {
			t.Errorf("%s: esperado %d, veio %d %s", c.name, c.want, rec.Code, rec.Body.String())
		}
	}
}
