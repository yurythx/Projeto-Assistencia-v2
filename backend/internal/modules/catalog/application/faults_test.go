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
	"github.com/yurythx/projeto-nexus/internal/modules/catalog"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/application"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// gestor: gestão global do catálogo (ADR 013 — concessão sem escopo).
var gestor = auth.Identity{Permissions: []string{"*"}, Scopes: []auth.Scope{{Perfil: "administrador", Permissions: []string{"*"}}}}

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo, outbox.NewWriter("test"))
}

func (e *env) real() *application.Service { return e.svc(infrastructure.NewRepository()) }

func (e *env) service(published bool) domain.Service {
	e.t.Helper()
	s, err := e.real().Save(context.Background(), gestor, uuid.Nil, domain.Service{Title: "Serviço " + uuid.NewString()[:8], Summary: "Resumo",
		Channels: []domain.Channel{{Type: "online", Label: "Portal", Value: "https://x"}}})
	if err != nil {
		e.t.Fatal(err)
	}
	if published {
		if s, err = e.real().SetStatus(context.Background(), gestor, s.ID, domain.StatusPublished); err != nil {
			e.t.Fatal(err)
		}
	}
	return s
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	p := pagination.New(1, 5, 5)
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"ListPublic": func() func(*application.Service) error {
			return func(s *application.Service) error { _, _, err := s.ListPublic(ctx, "", "", p); return err }
		},
		"ListAll": func() func(*application.Service) error {
			return func(s *application.Service) error { _, _, err := s.ListAll(ctx, gestor, "", "", "", p); return err }
		},
		"GetPublic": func() func(*application.Service) error {
			svc := e.service(true)
			return func(s *application.Service) error { _, err := s.GetPublic(ctx, svc.Slug); return err }
		},
		"Get": func() func(*application.Service) error {
			svc := e.service(false)
			return func(s *application.Service) error { _, err := s.Get(ctx, gestor, svc.ID); return err }
		},
		"Categories": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Categories(ctx); return err }
		},
		"Save": func() func(*application.Service) error {
			svc := e.service(true)
			return func(s *application.Service) error { _, err := s.Save(ctx, gestor, svc.ID, svc); return err }
		},
		"SetStatus": func() func(*application.Service) error {
			svc := e.service(false)
			return func(s *application.Service) error {
				_, err := s.SetStatus(ctx, gestor, svc.ID, domain.StatusPublished)
				return err
			}
		},
		"Delete": func() func(*application.Service) error {
			svc := e.service(false)
			return func(s *application.Service) error { return s.Delete(ctx, gestor, svc.ID) }
		},
		"Search": func() func(*application.Service) error {
			return func(s *application.Service) error { _, _, err := s.Search(ctx, "serviço", 5); return err }
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

// Rascunho não é público; categoria padrão; provedor da busca global.
func TestPublicVisibilityAndSearchProvider(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	draft := e.service(false)
	if draft.Category != "Geral" || draft.Status != domain.StatusDraft {
		t.Fatalf("novo serviço nasce rascunho na categoria Geral: %+v", draft)
	}
	if _, err := e.real().GetPublic(ctx, draft.Slug); err == nil {
		t.Fatal("rascunho não aparece no público")
	}
	pub := e.service(true)
	m := catalog.New(modkit.Deps{Pool: e.pool, Outbox: outbox.NewWriter("test"), Config: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	res, err := m.SearchProviders()[0].Search(ctx, auth.Identity{}, pub.Title, 5)
	if err != nil || len(res) == 0 || res[0].URL != "/servicos/"+pub.Slug || m.SearchProviders()[0].Module() != catalog.Key {
		t.Fatalf("busca global aponta para a página pública: %+v %v", res, err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := m.SearchProviders()[0].Search(cctx, auth.Identity{}, "x", 5); err == nil {
		t.Fatal("busca com o banco indisponível falha")
	}
	if m.Manifest().Key != catalog.Key {
		t.Fatal("manifesto")
	}
}

func TestHandlersReportServiceFailures(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), gestor)))
		})
	})
	h.RegisterPublicRoutes(r)
	h.RegisterAdminRoutes(r)
	for _, path := range []string{"/catalog/services", "/catalog/categories", "/catalog/admin/services"} {
		down.calls = 0
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("GET %s com o banco fora: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}
