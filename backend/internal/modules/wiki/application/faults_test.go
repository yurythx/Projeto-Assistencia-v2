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

	"github.com/yurythx/projeto-nexus/internal/modules/wiki"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki/application"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	ana  auth.Identity
}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo)
}

func (e *env) page(parent *uuid.UUID) domain.Page {
	e.t.Helper()
	p, err := e.svc(infrastructure.NewRepository()).Create(context.Background(), e.ana, application.Input{ParentID: parent, Title: "Página " + uuid.NewString()[:8], Body: "texto"})
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	pool := dbtest.Pool(t)
	e := &env{t: t, pool: pool, ana: auth.Identity{UserID: dbtest.User(t, pool)}}
	ctx := context.Background()
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"Tree": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Tree(ctx, e.ana); return err }
		},
		"GetByID": func() func(*application.Service) error {
			p := e.page(nil)
			return func(s *application.Service) error { _, err := s.Get(ctx, e.ana, p.ID.String()); return err }
		},
		"GetBySlug": func() func(*application.Service) error {
			p := e.page(nil)
			return func(s *application.Service) error { _, err := s.Get(ctx, e.ana, p.Slug); return err }
		},
		"Create": func() func(*application.Service) error {
			parent := e.page(nil)
			return func(s *application.Service) error {
				_, err := s.Create(ctx, e.ana, application.Input{ParentID: &parent.ID, Title: "Nova " + uuid.NewString()[:8]})
				return err
			}
		},
		"Update": func() func(*application.Service) error {
			p, parent := e.page(nil), e.page(nil)
			return func(s *application.Service) error {
				_, err := s.Update(ctx, e.ana, p.ID, p.Version, application.Input{ParentID: &parent.ID, Title: "Editada", Body: "novo"})
				return err
			}
		},
		"Restore": func() func(*application.Service) error {
			p := e.page(nil)
			return func(s *application.Service) error { _, err := s.Restore(ctx, e.ana, p.ID, 1); return err }
		},
		"Delete": func() func(*application.Service) error {
			p := e.page(nil)
			return func(s *application.Service) error { return s.Delete(ctx, gestor, p.ID) }
		},
		"Revisions": func() func(*application.Service) error {
			p := e.page(nil)
			return func(s *application.Service) error { _, err := s.Revisions(ctx, e.ana, p.ID); return err }
		},
		"Revision": func() func(*application.Service) error {
			p := e.page(nil)
			return func(s *application.Service) error { _, err := s.Revision(ctx, e.ana, p.ID, 1); return err }
		},
		"Search": func() func(*application.Service) error {
			return func(s *application.Service) error { _, _, err := s.Search(ctx, e.ana, "página", 5); return err }
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

func TestModuleSearchAndHandlers(t *testing.T) {
	pool := dbtest.Pool(t)
	e := &env{t: t, pool: pool, ana: auth.Identity{UserID: dbtest.User(t, pool)}}
	p := e.page(nil)
	m := wiki.New(modkit.Deps{Pool: pool, Config: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	res, err := m.SearchProviders()[0].Search(context.Background(), e.ana, p.Title, 5)
	if err != nil || len(res) == 0 || m.SearchProviders()[0].Module() != wiki.Key || m.Manifest().Key != wiki.Key {
		t.Fatalf("busca global: %+v %v", res, err)
	}
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.SearchProviders()[0].Search(cctx, e.ana, "x", 5); err == nil {
		t.Fatal("busca com o banco indisponível falha")
	}
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), e.ana)))
		})
	})
	h.RegisterRoutes(r)
	for _, path := range []string{"/wiki/tree", "/wiki/pages/" + p.ID.String() + "/revisions"} {
		down.calls = 0
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("GET %s com o banco fora: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}
