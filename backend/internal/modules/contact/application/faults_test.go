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
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/application"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo, outbox.NewWriter("test"))
}

func (e *env) submit() domain.Message {
	e.t.Helper()
	m, err := e.svc(infrastructure.NewRepository()).Submit(context.Background(), application.SubmitInput{
		Name: "Maria", Email: "maria@example.com", Subject: "Assunto", Message: "Mensagem longa o bastante"})
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	assignee := dbtest.User(t, e.pool)
	setor := dbtest.Unidade(t, e.pool)
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"Submit": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.Submit(ctx, application.SubmitInput{Name: "João", Email: "j@x.org", Subject: "S", Message: "Mensagem de teste"})
				return err
			}
		},
		"List": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, _, err := s.List(ctx, gestor, domain.Filter{}, pagination.New(1, 5, 5))
				return err
			}
		},
		"Get": func() func(*application.Service) error {
			m := e.submit()
			return func(s *application.Service) error { _, err := s.Get(ctx, gestor, m.ID); return err }
		},
		"Triage": func() func(*application.Service) error {
			m := e.submit()
			return func(s *application.Service) error {
				_, err := s.Triage(ctx, gestor, m.ID, domain.Triage{Status: "in_progress", Notes: "nota", AssignedTo: &assignee, UnidadeID: &setor})
				return err
			}
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
	if m := e.submit(); m.Category != "duvida" || !strings.HasPrefix(m.Protocol, "CT-") {
		t.Fatalf("categoria padrão e protocolo: %+v", m)
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Error("erro desconhecido passa adiante")
	}
}

func TestHandlersReportServiceFailures(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Permissions: []string{"*"}})))
		})
	})
	h.RegisterPublicRoutes(r, allowAll{})
	h.RegisterAdminRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/contact/messages", nil))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
		t.Errorf("caixa com o banco fora: %d %s", rec.Code, rec.Body.String())
	}
	down.calls = 0
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/contact/messages", strings.NewReader(
		`{"name":"Maria","email":"m@x.org","subject":"Assunto","message":"Mensagem de teste","consent":true}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("envio com o banco fora: %d %s", rec.Code, rec.Body.String())
	}
}

type allowAll struct{}

func (allowAll) Allow(context.Context, string) (bool, error) { return true, nil }
