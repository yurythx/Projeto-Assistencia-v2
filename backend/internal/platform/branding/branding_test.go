package branding

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func valid() Settings {
	return Settings{AppName: "Nexus", OrgName: "Prefeitura", Tokens: map[string]string{}}
}

func TestValidateRules(t *testing.T) {
	for name, mut := range map[string]func(*Settings){
		"campo obrigatório":          func(s *Settings) { s.AppName = "" },
		"token desconhecido":         func(s *Settings) { s.Tokens["fundo-magico"] = "#fff" },
		"cor não hexadecimal":        func(s *Settings) { s.Tokens["primary"] = "azul" },
		"contraste baixo (primário)": func(s *Settings) { s.Tokens["primary"], s.Tokens["primary-foreground"] = "#777777", "#888888" },
		"contraste baixo (rodapé)":   func(s *Settings) { s.Tokens["footer-bg"], s.Tokens["footer-foreground"] = "#ffffff", "#ffff00" },
		"logo javascript:":           func(s *Settings) { s.LogoURL = "javascript:alert(1)" },
		"logo http":                  func(s *Settings) { s.LogoURL = "http://cdn.gov.br/logo.png" },
		"logo relativo ao protocolo": func(s *Settings) { s.LogoURL = "//site-externo.com/logo.png" },
		"logo com barra invertida":   func(s *Settings) { s.LogoURL = `/\site-externo.com/logo.png` },
		"favicon com aspas":          func(s *Settings) { s.FaviconURL = `/fav.ico" onerror="x` },
	} {
		s := valid()
		mut(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s deveria ser recusado", name)
		}
	}
	ok := valid()
	ok.Tokens = map[string]string{"primary": "#1351B4", "primary-foreground": "#FFF", "footer-bg": "#071D41", "footer-foreground": "#ffffff", "accent": "#abc"}
	ok.LogoURL, ok.FaviconURL = "https://cdn.orgao.gov.br/logo.svg", "/favicon.ico"
	if err := ok.Validate(); err != nil {
		t.Fatalf("paleta DSGov (contraste AA) e URLs seguras: %v", err)
	}
	if r := contrast("#000", "#fff"); r < 20.9 || r > 21.1 {
		t.Fatalf("preto sobre branco = 21:1, veio %.2f", r)
	}
}

func restore(t *testing.T, pool *pgxpool.Pool) {
	s := NewStore(pool)
	orig, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.Save(context.Background(), orig, "teste") })
}

func router(h *Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Username: "admin", Permissions: []string{"*"}})))
		})
	})
	h.RegisterPublicRoutes(r)
	h.RegisterAdminRoutes(r)
	return r
}

func do(h http.Handler, method, body string) *httptest.ResponseRecorder {
	path := "/branding"
	if method == http.MethodPut {
		path = "/admin/branding"
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBrandingHTTPAndAudit(t *testing.T) {
	pool := dbtest.Pool(t)
	restore(t, pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := router(NewHandlers(NewStore(pool), logger))
	rec := do(h, http.MethodPut, `{"app_name":"Portal","org_name":"Câmara","logo_url":"/logo.svg"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tokens":{}`) {
		t.Fatalf("salvar sem tokens grava {}: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodGet, ""); !strings.Contains(rec.Body.String(), `"app_name":"Portal"`) || rec.Header().Get("Cache-Control") == "" {
		t.Fatalf("leitura pública: %s", rec.Body.String())
	}
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = 'branding.updated' AND diff_after->>'app_name' = 'Portal'`).Scan(&n)
	if n < 1 {
		t.Fatal("alteração auditada com diff")
	}
	for body, want := range map[string]int{"{": 400, `{"app_name":"x"}`: 422, `{"app_name":"x","org_name":"y","tokens":{"primary":"#777","primary-foreground":"#888"}}`: 422} {
		if rec := do(h, http.MethodPut, body); rec.Code != want {
			t.Errorf("%s: %d", body, rec.Code)
		}
	}

	down := router(NewHandlers(&Store{pool: pool, db: dbtest.Fail{}}, logger))
	if rec := do(down, http.MethodGet, ""); rec.Code != 500 {
		t.Errorf("leitura com o banco fora: %d", rec.Code)
	}
	closed, _ := pgxpool.New(context.Background(), pool.Config().ConnString())
	closed.Close()
	if rec := do(router(NewHandlers(NewStore(closed), logger)), http.MethodPut, `{"app_name":"x","org_name":"y"}`); rec.Code != 500 {
		t.Errorf("gravação com o banco fora: %d", rec.Code)
	}
}

// hook envolve o banco real falhando o Exec ou a n-ésima leitura.
type hook struct {
	database.DBTX
	failExec bool
	failRow  int
	rows     int
}

func (h *hook) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if h.failExec {
		return pgconn.CommandTag{}, dbtest.ErrInjected
	}
	return h.DBTX.Exec(ctx, sql, args...)
}

func (h *hook) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	h.rows++
	if h.rows == h.failRow {
		return dbtest.Fail{}.QueryRow(ctx, sql)
	}
	return h.DBTX.QueryRow(ctx, sql, args...)
}

func TestSaveStepsFail(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	s := NewStore(pool)
	for name, h := range map[string]*hook{"leitura inicial": {failRow: 1}, "gravação": {failExec: true}, "releitura": {failRow: 2}} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		h.DBTX = tx
		if _, err := s.save(ctx, h, valid(), "t"); err == nil {
			t.Errorf("%s: falha propagada", name)
		}
		_ = tx.Rollback(ctx)
	}
}
