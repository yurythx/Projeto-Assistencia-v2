package keycloakconfig

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// kc é um Keycloak falso que registra o client_secret recebido no token_endpoint.
type kc struct {
	srv       *httptest.Server
	mu        sync.Mutex
	secrets   []string
	failAfter atomic.Int32 // >0: discovery responde 500 a partir da n-ésima chamada
	calls     atomic.Int32
}

func newKC(t *testing.T) *kc {
	k := &kc{}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		n := k.calls.Add(1)
		if f := k.failAfter.Load(); f > 0 && n >= f {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": k.srv.URL, "authorization_endpoint": k.srv.URL + "/auth",
			"token_endpoint": k.srv.URL + "/token", "jwks_uri": k.srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		k.mu.Lock()
		k.secrets = append(k.secrets, r.PostForm.Get("client_secret"))
		k.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	k.srv = httptest.NewServer(mux)
	t.Cleanup(k.srv.Close)
	return k
}

func (k *kc) received() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]string(nil), k.secrets...)
}

// memStore é um Store em memória com falhas injetáveis.
type memStore struct {
	s              Settings
	getErr, setErr error
}

func (m *memStore) Get(context.Context) (Settings, error) { return m.s, m.getErr }
func (m *memStore) Set(_ context.Context, in Settings, by string) (Settings, error) {
	if m.setErr != nil {
		return Settings{}, m.setErr
	}
	if in.ClientSecret == "" {
		in.ClientSecret = m.s.ClientSecret
	}
	in.Configured, in.UpdatedBy, in.UpdatedAt = true, by, time.Now()
	m.s = in
	return in, nil
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func handlers(t *testing.T, st Store, env config.KeycloakConfig, k *kc) http.Handler {
	v, err := auth.NewVerifier(context.Background(), config.KeycloakConfig{IssuerURL: k.srv.URL, ClientID: "nexus"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(st, v, env, audit.NewWriter(dbtest.Fail{}), quiet())
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Subject: "admin", Permissions: []string{"*"}})))
		})
	})
	RegisterRoutes(r, h, quiet())
	return r
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestStoredSecretIsNeverSentToAnotherIssuer(t *testing.T) {
	real, evil := newKC(t), newKC(t)
	st := &memStore{s: Settings{Configured: true, IssuerURL: real.srv.URL, ClientID: "nexus", ClientSecret: "segredo-real", Audience: "nexus"}}
	h := handlers(t, st, config.KeycloakConfig{}, real)

	// Mesmo issuer e client: reaproveita o segredo salvo.
	call(h, http.MethodPost, "/admin/keycloak/test", `{"issuer_url":"`+real.srv.URL+`","client_id":"nexus"}`)
	if !sameIssuer(real.srv.URL+"/ ", real.srv.URL) {
		t.Fatal("barra final e espaços não mudam o issuer")
	}
	if got := real.received(); len(got) != 1 || got[0] != "segredo-real" {
		t.Fatalf("mesmo issuer: %v", got)
	}
	// Outro issuer: o segredo salvo NÃO vai junto.
	rec := call(h, http.MethodPost, "/admin/keycloak/test", `{"issuer_url":"`+evil.srv.URL+`","client_id":"nexus"}`)
	if len(evil.received()) != 0 || !strings.Contains(rec.Body.String(), `"credentials_checked":false`) {
		t.Fatalf("segredo vazou para outro issuer: %v %s", evil.received(), rec.Body.String())
	}
	// Salvar trocando o issuer sem redigitar o segredo: recusado, e nada vai ao outro issuer.
	rec = call(h, http.MethodPut, "/admin/keycloak/", `{"issuer_url":"`+evil.srv.URL+`","realm":"r","client_id":"nexus","audience":"nexus"}`)
	if rec.Code != http.StatusUnprocessableEntity || len(evil.received()) != 0 || st.s.IssuerURL != real.srv.URL {
		t.Fatalf("troca de issuer sem segredo: %d %v", rec.Code, evil.received())
	}
	// Com o segredo informado de novo, a troca é aceita.
	rec = call(h, http.MethodPut, "/admin/keycloak/", `{"issuer_url":"`+evil.srv.URL+`","realm":"r","client_id":"nexus","client_secret":"novo","audience":"nexus"}`)
	if rec.Code != http.StatusOK || st.s.ClientSecret != "novo" {
		t.Fatalf("troca com segredo novo: %d %s", rec.Code, rec.Body.String())
	}
}

func TestFirstSaveAdoptsEnvSecretAndStatusSources(t *testing.T) {
	k := newKC(t)
	env := config.KeycloakConfig{IssuerURL: k.srv.URL, Realm: "r", ClientID: "nexus", ClientSecret: "segredo-env", Audience: "nexus"}
	st := &memStore{}
	h := handlers(t, st, env, k)
	if rec := call(h, http.MethodGet, "/admin/keycloak/", ""); !strings.Contains(rec.Body.String(), `"source":"environment"`) {
		t.Fatalf("fonte: .env: %s", rec.Body.String())
	}
	// Teste com o issuer do .env reaproveita o segredo do .env.
	call(h, http.MethodPost, "/admin/keycloak/test", `{"issuer_url":"`+k.srv.URL+`","client_id":"nexus"}`)
	if rec := call(h, http.MethodPut, "/admin/keycloak/", `{"issuer_url":"`+k.srv.URL+`","realm":"r","client_id":"nexus","audience":"nexus"}`); rec.Code != http.StatusOK {
		t.Fatalf("adotar .env: %d %s", rec.Code, rec.Body.String())
	}
	if st.s.ClientSecret != "segredo-env" {
		t.Fatalf("o segredo testado precisa ser gravado ao adotar o .env: %q", st.s.ClientSecret)
	}
	if rec := call(h, http.MethodGet, "/admin/keycloak/", ""); !strings.Contains(rec.Body.String(), `"source":"database"`) || strings.Contains(rec.Body.String(), "segredo-env") {
		t.Fatalf("fonte: banco, sem expor o segredo: %s", rec.Body.String())
	}
	unset := handlers(t, &memStore{}, config.KeycloakConfig{}, k)
	if rec := call(unset, http.MethodGet, "/admin/keycloak/", ""); !strings.Contains(rec.Body.String(), `"source":"unset"`) {
		t.Fatalf("fonte: nenhuma: %s", rec.Body.String())
	}
}

func TestHandlerFailures(t *testing.T) {
	k := newKC(t)
	boom := errors.New("banco fora")
	body := `{"issuer_url":"` + k.srv.URL + `","realm":"r","client_id":"nexus","client_secret":"s","audience":"nexus"}`
	for name, c := range map[string]struct {
		st           *memStore
		method, path string
		body         string
		want         int
	}{
		"status com o banco fora":   {&memStore{getErr: boom}, http.MethodGet, "/admin/keycloak/", "", 500},
		"teste com JSON malformado": {&memStore{}, http.MethodPost, "/admin/keycloak/test", "{", 400},
		"teste sem issuer":          {&memStore{}, http.MethodPost, "/admin/keycloak/test", `{}`, 422},
		"salvar JSON malformado":    {&memStore{}, http.MethodPut, "/admin/keycloak/", "{", 400},
		"salvar sem campos":         {&memStore{}, http.MethodPut, "/admin/keycloak/", `{}`, 422},
		"salvar com o banco fora":   {&memStore{getErr: boom}, http.MethodPut, "/admin/keycloak/", body, 500},
		"gravação falha":            {&memStore{setErr: boom}, http.MethodPut, "/admin/keycloak/", body, 500},
		"issuer inalcançável":       {&memStore{}, http.MethodPut, "/admin/keycloak/", strings.Replace(body, k.srv.URL, "http://127.0.0.1:1", 1), 422},
	} {
		if rec := call(handlers(t, c.st, config.KeycloakConfig{}, k), c.method, c.path, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, esperado %d (%s)", name, rec.Code, c.want, rec.Body.String())
		}
	}
	// Salvo, mas o reload do verificador falha: responde 200 e registra.
	k.failAfter.Store(k.calls.Load() + 3) // handlers() + teste de conexão passam; o Reload falha
	st := &memStore{}
	h := handlers(t, st, config.KeycloakConfig{}, k)
	if rec := call(h, http.MethodPut, "/admin/keycloak/", body); rec.Code != http.StatusOK || !st.s.Configured {
		t.Fatalf("reload falhou depois de salvar: %d %s", rec.Code, rec.Body.String())
	}
}
