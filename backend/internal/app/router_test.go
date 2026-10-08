package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/branding"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/iam"
	"github.com/yurythx/projeto-nexus/internal/platform/idempotency"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/keycloakconfig"
	"github.com/yurythx/projeto-nexus/internal/platform/lgpd"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/passwords"
	"github.com/yurythx/projeto-nexus/internal/platform/ratelimit"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
	"github.com/yurythx/projeto-nexus/internal/platform/storage/storagetest"
	"github.com/yurythx/projeto-nexus/internal/platform/transparency"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// testDeps monta um Dependencies real (Postgres de teste + Redis em
// memória, storage em memória) sem RabbitMQ/MinIO/Keycloak.
func testDeps(t *testing.T) *Dependencies {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido — pulando teste de roteamento")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	cfg := &config.Config{
		App:         config.AppConfig{Env: "test", Name: "nexus-test"},
		LocalAuth:   config.LocalAuthConfig{Enabled: true, PrivateKeyPEM: pemKey, TokenTTL: time.Hour},
		MinIO:       config.MinIOConfig{Bucket: "nexus-test"},
		Upload:      config.UploadConfig{MaxFileBytes: 10 << 20, URLExpiry: time.Minute},
		Signum:      config.SignumConfig{ChallengeTTL: time.Minute},
		Egress:      config.EgressConfig{Timeout: time.Second, MaxAttempts: 3, BatchSize: 10, PollInterval: time.Second},
		Security:    config.SecurityConfig{ConfigEncryptionKey: "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA="},
		FrontendURL: "http://localhost:3000",
		MaxPageSize: 100,
	}
	signer, err := auth.NewLocalSigner(cfg.LocalAuth)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := auth.NewVerifier(context.Background(), cfg.Keycloak, signer)
	if err != nil {
		t.Fatal(err)
	}
	cipher, _ := secretcrypto.NewFromBase64Key(cfg.Security.ConfigEncryptionKey)

	d := &Dependencies{
		Config: cfg, Logger: logger, DB: pool, Redis: rdb, Verifier: verifier, LocalSigner: signer,
		IAM: iam.NewResolver(pool, rdb, logger), Outbox: outbox.NewWriter("nexus.test"), OutboxStats: outbox.NewStats(pool),
		Storage: storagetest.New(), Hub: ws.NewHub(logger, rdb), Tickets: ws.NewTicketStore(rdb, ws.TicketTTL), Cipher: cipher,
		Idempotency: idempotency.NewPostgresStore(pool), KeycloakCfg: keycloakconfig.NewPostgresStore(pool, cipher),
		Branding: branding.NewStore(pool), LGPD: lgpd.NewService(pool, logger, nil),
		RateLimiters: &RateLimiters{
			APIGlobal: ratelimit.NewRedisLimiter(rdb, 60, 1000, "api"), Login: ratelimit.NewRedisLimiter(rdb, 60, 100, "login"),
			WSTicket: ratelimit.NewRedisLimiter(rdb, 5, 5, "ws"), Public: ratelimit.NewRedisLimiter(rdb, 60, 1000, "public"),
			Contact: ratelimit.NewRedisLimiter(rdb, 60, 100, "contact"), AtlasChat: ratelimit.NewRedisLimiter(rdb, 60, 5, "atlas_chat"),
			Lockout: ratelimit.NewLockout(rdb, 5, time.Minute, time.Hour),
		},
	}
	d.Kernel = kernel.New(kernel.NewPostgresStore(pool, logger), logger)
	registerPlugins(d)
	if err := d.Kernel.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	d.Transparency = transparency.NewService(pool, logger, d.moduleInfo)
	wireWebSocket(d)
	return d
}

func TestRouterMountsEveryPluginAndEnforcesAuthAndGuard(t *testing.T) {
	d := testDeps(t)
	router := NewRouter(d)

	// 1) A tabela de rotas monta sem conflito e contém a superfície dos plugins.
	routes := map[string]bool{}
	_ = chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = true
		return nil
	})
	for _, want := range []string{
		"GET /api/v1/me", "GET /api/v1/iam/org-tree", "GET /api/v1/audit/verify", "PATCH /api/v1/admin/modules/{key}",
		"GET /api/v1/mercurio/rooms", "GET /api/v1/egress/targets", "GET /api/v1/blog/posts", "GET /api/v1/catalog/services",
		"POST /api/v1/contact/messages", "GET /api/v1/directory/people", "GET /api/v1/calendar/events", "GET /api/v1/files/browse",
		"GET /api/v1/wiki/tree", "GET /api/v1/search", "POST /api/v1/signum/envelopes/{id}/sign", "GET /api/v1/signum/verify/{id}",
		"POST /api/v1/tramite/processos", "POST /api/v1/auth/login", "GET /api/v1/system/public-modules", "GET /api/v1/branding",
		"GET /api/v1/atlas/workflows", "POST /api/v1/atlas/chat", "POST /api/v1/atlas/admin/workflows",
	} {
		if !routes[want] {
			t.Errorf("rota ausente: %s", want)
		}
	}

	// 2) Conta local com senha Argon2id -> login -> token RS256.
	username := "rt_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	hash, _ := passwords.Hash("Senha-Forte-123")
	if _, err := d.DB.Exec(context.Background(), `INSERT INTO users (username, email, password_hash, roles) VALUES ($1, $1 || '@nexus.test', $2, '{nexus-admin}')`,
		username, hash); err != nil {
		t.Fatal(err)
	}
	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	rec := do(http.MethodPost, "/api/v1/auth/login", "", `{"username":"`+username+`","password":"Senha-Forte-123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login local: %d %s", rec.Code, rec.Body.String())
	}
	var login struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &login)
	token := login.Data.AccessToken

	// 3) Headers de segurança da skill em toda resposta.
	if got := rec.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "preload") {
		t.Errorf("HSTS sem preload: %q", got)
	}

	// 4) Autenticação obrigatória e IAM resolvendo o admin.
	if rec := do(http.MethodGet, "/api/v1/blog/posts", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem token deveria ser 401, veio %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/api/v1/me", token, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"*"`) {
		t.Fatalf("/me deveria trazer a permissão curinga do nexus-admin: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodGet, "/api/v1/blog/posts", token, ""); rec.Code != http.StatusOK {
		t.Fatalf("blog ativo deveria responder 200, veio %d %s", rec.Code, rec.Body.String())
	}

	// 5) Desativação em runtime: 404 MODULE_DISABLED na hora, sem restart.
	if rec := do(http.MethodPatch, "/api/v1/admin/modules/blog", token, `{"enabled":false}`); rec.Code != http.StatusOK {
		t.Fatalf("desativar blog: %d %s", rec.Code, rec.Body.String())
	}
	t.Cleanup(func() { _ = d.Kernel.SetEnabled(context.Background(), "blog", true, "test") })
	rec = do(http.MethodGet, "/api/v1/blog/posts", token, "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "MODULE_DISABLED") {
		t.Fatalf("blog desativado deveria responder 404 MODULE_DISABLED, veio %d %s", rec.Code, rec.Body.String())
	}
	// Núcleo não desativa.
	if rec := do(http.MethodPatch, "/api/v1/admin/modules/audit", token, `{"enabled":false}`); rec.Code != http.StatusConflict {
		t.Fatalf("desativar o núcleo deveria dar 409, veio %d", rec.Code)
	}
	// Dependência: Signum não desativa com Trâmite ativo.
	if rec := do(http.MethodPatch, "/api/v1/admin/modules/signum", token, `{"enabled":false}`); rec.Code != http.StatusConflict {
		t.Fatalf("desativar dependência de módulo ativo deveria dar 409, veio %d", rec.Code)
	}

	// 6) Rota pública anônima.
	if rec := do(http.MethodGet, "/api/v1/system/public-modules", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("public-modules: %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/api/v1/catalog/services", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("catálogo público: %d %s", rec.Code, rec.Body.String())
	}

	// 7) Autorização por escopo: usuário sem perfil não acessa a auditoria.
	plain := "rt_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if _, err := d.DB.Exec(context.Background(), `INSERT INTO users (username, email, password_hash, roles) VALUES ($1, $1 || '@nexus.test', $2, '{nexus-user}')`,
		plain, hash); err != nil {
		t.Fatal(err)
	}
	rec = do(http.MethodPost, "/api/v1/auth/login", "", `{"username":"`+plain+`","password":"Senha-Forte-123"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &login)
	if rec := do(http.MethodGet, "/api/v1/audit/logs", login.Data.AccessToken, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("sem audit:read deveria ser 403, veio %d", rec.Code)
	}

	// 8) Idempotência (X-Idempotency-Key): a mesma mutação não executa duas vezes.
	if rec := do(http.MethodPatch, "/api/v1/admin/modules/blog", token, `{"enabled":true}`); rec.Code != http.StatusOK {
		t.Fatalf("reativar blog: %d", rec.Code)
	}
	body := `{"title":"Idempotente ` + uuid.NewString()[:6] + `","kind":"noticia"}`
	req := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/blog/posts", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Idempotency-Key", uuid.NewString()[:0]+"chave-"+username)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	first, second := req(), req()
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated || first.Body.String() != second.Body.String() {
		t.Fatalf("replay idempotente esperado: %d / %d", first.Code, second.Code)
	}
}
