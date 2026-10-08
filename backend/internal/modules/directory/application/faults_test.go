package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/directory"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/application"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
}

// gestao: directory:manage global (ADR 013 — concessão sem escopo).
var gestao = auth.Identity{Roles: []string{auth.RoleAdmin}}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo)
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	user := dbtest.User(t, e.pool)
	un := dbtest.Unidade(t, e.pool)
	var dep uuid.UUID
	if err := e.pool.QueryRow(ctx, `INSERT INTO departamentos (unidade_id, nome, slug) VALUES ($1, 'D', $2) RETURNING id`, un, uuid.NewString()).Scan(&dep); err != nil {
		t.Fatal(err)
	}
	viewer := auth.Identity{UserID: dbtest.User(t, e.pool)}
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"List": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, _, err := s.List(ctx, viewer, domain.Filter{}, pagination.New(1, 5, 5))
				return err
			}
		},
		"Get": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Get(ctx, viewer, user); return err }
		},
		"SaveProfile": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.SaveProfile(ctx, gestao, user, domain.ProfileInput{JobTitle: "Cargo", Visible: true, DepartamentoID: &dep}, false)
				return err
			}
		},
		"Sectors": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Sectors(ctx, ""); return err }
		},
	}
	for name, o := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := o()(e.svc(probe)); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			for _, poison := range []bool{false, true} {
				r := &faultRepo{inner: infrastructure.NewRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := o()(e.svc(r)); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Error("erro desconhecido passa adiante")
	}
}

func TestModuleAndHandlers(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	m := directory.New(modkit.Deps{Pool: e.pool, Config: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if m.Manifest().Key != directory.Key {
		t.Fatal("manifesto")
	}
	ctx := context.Background()
	user := dbtest.User(t, e.pool)
	un := dbtest.Unidade(t, e.pool)
	var dep uuid.UUID
	if err := e.pool.QueryRow(ctx, `INSERT INTO departamentos (unidade_id, nome, slug) VALUES ($1, 'Protocolo', $2) RETURNING id`, un, uuid.NewString()).Scan(&dep); err != nil {
		t.Fatal(err)
	}
	marker := "Cargo" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if _, err := e.svc(infrastructure.NewRepository()).SaveProfile(ctx, gestao, user, domain.ProfileInput{JobTitle: marker, Visible: true, DepartamentoID: &dep}, false); err != nil {
		t.Fatal(err)
	}
	other := dbtest.User(t, e.pool)
	if _, err := e.svc(infrastructure.NewRepository()).SaveProfile(ctx, gestao, other, domain.ProfileInput{JobTitle: marker + " sem setor", Visible: true}, false); err != nil {
		t.Fatal(err)
	}
	sp := m.SearchProviders()[0]
	res, err := sp.Search(ctx, auth.Identity{UserID: uuid.New()}, marker, 5)
	if err != nil || len(res) != 2 || sp.Module() != directory.Key {
		t.Fatalf("busca global de pessoas: %+v %v", res, err)
	}
	joined := res[0].Snippet + "|" + res[1].Snippet
	if !strings.Contains(joined, marker+" · Protocolo") || !strings.Contains(joined, marker+" sem setor") {
		t.Fatalf("resumo com cargo e departamento: %q", joined)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := sp.Search(cctx, auth.Identity{}, marker, 5); err == nil {
		t.Fatal("busca com o banco indisponível falha")
	}
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{UserID: uuid.New()})))
		})
	})
	h.RegisterPublicRoutes(r)
	h.RegisterAuthedRoutes(r)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/directory/people"}, {http.MethodGet, "/directory/public/sectors"}, {http.MethodGet, "/directory/me"},
		{http.MethodPut, "/directory/me"},
	} {
		down.calls = 0
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(`{"job_title":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("%s %s com o banco fora: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
}

func TestPersonalDataExportAndErase(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	svc := application.NewService(pool, infrastructure.NewRepository())
	if data, err := svc.ExportPersonalData(ctx, pool, uuid.New()); err != nil || data != nil {
		t.Fatalf("titular sem conta: nada a exportar (%v, %v)", data, err)
	}
	down := application.NewService(pool, &faultRepo{inner: infrastructure.NewRepository(), failAt: 1})
	if _, err := down.ExportPersonalData(ctx, pool, uuid.New()); !errors.Is(err, errBoom) {
		t.Fatalf("falha do repositório na exportação: %v", err)
	}
	down = application.NewService(pool, &faultRepo{inner: infrastructure.NewRepository(), failAt: 1})
	if err := down.ErasePersonalData(ctx, pool, uuid.New()); !errors.Is(err, errBoom) {
		t.Fatalf("falha do repositório na eliminação: %v", err)
	}
}
