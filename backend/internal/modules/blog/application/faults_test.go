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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/blog"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/application"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/storage/storagetest"
)

const bucket = "nexus-test"

type env struct {
	t       *testing.T
	pool    *pgxpool.Pool
	store   *storagetest.Memory
	manager auth.Identity
	ctx     context.Context
}

// gestao: blog:manage global (ADR 013 — concessão sem escopo).
var gestao = auth.Identity{Roles: []string{auth.RoleAdmin}}

func newEnv(t *testing.T) *env {
	pool := dbtest.Pool(t)
	m := auth.Identity{UserID: dbtest.User(t, pool), Permissions: []string{string(auth.PermBlogManage)},
		Scopes: []auth.Scope{{Perfil: "gestor", Permissions: []string{string(auth.PermBlogManage)}}}}
	return &env{t: t, pool: pool, store: storagetest.New(), manager: m, ctx: auth.WithIdentity(context.Background(), m)}
}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo, outbox.NewWriter("test"), e.store, bucket, time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (e *env) real() *application.Service { return e.svc(infrastructure.NewRepository()) }

func (e *env) cover() string {
	e.t.Helper()
	key := "blog/covers/" + uuid.NewString() + ".png"
	if err := e.store.Put(context.Background(), bucket, key, strings.NewReader("\x89PNG"), 4, "image/png"); err != nil {
		e.t.Fatal(err)
	}
	return key
}

func (e *env) post(publish bool) domain.Post {
	e.t.Helper()
	p, err := e.real().Create(e.ctx, gestao, application.Input{Title: "Notícia " + uuid.NewString()[:8], Body: "Texto", CoverObjectKey: e.cover()})
	if err != nil {
		e.t.Fatal(err)
	}
	if publish {
		if p, err = e.real().Transition(e.ctx, gestao, p.ID, domain.StatusPublished); err != nil {
			e.t.Fatal(err)
		}
	}
	return p
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := newEnv(t)
	ctx := e.ctx
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"List": func() func(*application.Service) error {
			e.post(true)
			return func(s *application.Service) error {
				_, _, err := s.List(ctx, e.manager, domain.Filter{}, pagination.New(1, 5, 5))
				return err
			}
		},
		"GetByID": func() func(*application.Service) error {
			p := e.post(false)
			return func(s *application.Service) error { _, err := s.Get(ctx, e.manager, p.ID.String()); return err }
		},
		"GetBySlug": func() func(*application.Service) error {
			p := e.post(true)
			return func(s *application.Service) error { _, err := s.Get(ctx, e.manager, p.Slug); return err }
		},
		"Create": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.Create(ctx, gestao, application.Input{Title: "Nova " + uuid.NewString()[:8]})
				return err
			}
		},
		"Update": func() func(*application.Service) error {
			p := e.post(true)
			return func(s *application.Service) error {
				_, err := s.Update(ctx, gestao, p.ID, application.Input{Title: p.Title, Body: "novo", CoverObjectKey: e.cover()})
				return err
			}
		},
		"Transition": func() func(*application.Service) error {
			p := e.post(false)
			return func(s *application.Service) error {
				_, err := s.Transition(ctx, gestao, p.ID, domain.StatusPublished)
				return err
			}
		},
		"Delete": func() func(*application.Service) error {
			p := e.post(false)
			return func(s *application.Service) error { return s.Delete(ctx, gestao, p.ID) }
		},
		"Search": func() func(*application.Service) error {
			return func(s *application.Service) error { _, _, err := s.Search(ctx, gestao, "notícia", 5); return err }
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

// Armazenamento: capa sem URL não derruba a leitura; capa órfã é logada.
func TestCoverStorageFailures(t *testing.T) {
	e := newEnv(t)
	p := e.post(true)
	e.store.FailPresign = true
	got, err := e.real().Get(e.ctx, e.manager, p.ID.String())
	if err != nil || got.CoverURL != "" {
		t.Fatalf("sem URL temporária, a publicação sai sem capa: %+v %v", got, err)
	}
	e.store.FailPresign = false
	e.store.FailDelete = true
	if err := e.real().Delete(e.ctx, gestao, p.ID); err != nil {
		t.Fatalf("capa não removida não desfaz a exclusão: %v", err)
	}
	e.store.FailDelete = false
	if _, err := e.real().Get(e.ctx, auth.Identity{}, e.post(false).ID.String()); err == nil {
		t.Fatal("rascunho é invisível a quem não gerencia")
	}
	if _, _, err := e.real().List(e.ctx, auth.Identity{}, domain.Filter{Status: domain.StatusDraft}, pagination.New(1, 5, 5)); err == nil {
		t.Fatal("leitor não lista rascunhos")
	}
}

func TestSearchProviderAndModule(t *testing.T) {
	e := newEnv(t)
	p := e.post(true)
	m := blog.New(modkit.Deps{Pool: e.pool, Outbox: outbox.NewWriter("test"), Storage: e.store, Config: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	res, err := m.SearchProviders()[0].Search(context.Background(), auth.Identity{}, p.Title, 5)
	if err != nil || len(res) == 0 || m.SearchProviders()[0].Module() != blog.Key {
		t.Fatalf("busca global: %+v %v", res, err)
	}
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.SearchProviders()[0].Search(cctx, auth.Identity{}, "x", 5); err == nil {
		t.Fatal("busca com o banco indisponível falha")
	}
	if m.Manifest().Key != blog.Key {
		t.Fatal("manifesto")
	}
}

func TestHandlersReportServiceFailures(t *testing.T) {
	e := newEnv(t)
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), e.manager)))
		})
	})
	h.RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/blog/posts", nil))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
		t.Errorf("listagem com o banco fora: %d %s", rec.Code, rec.Body.String())
	}
}
