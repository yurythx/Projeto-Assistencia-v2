package application_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/example/application"
	"github.com/yurythx/projeto-nexus/internal/modules/example/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/example/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// Matriz de falhas: cada chamada ao repositório falha (ou envenena a
// transação logo depois) e o caso de uso tem de propagar o erro — nunca
// devolver sucesso com item gravado sem evento/auditoria.
func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	svc := func(r *faultRepo) *application.Service {
		return application.NewService(pool, r, outbox.NewWriter("nexus.example"), testLogger())
	}
	created, err := svc(&faultRepo{inner: infrastructure.NewPostgresRepository()}).CreateItem(ctx, "Base", "")
	if err != nil {
		t.Fatal(err)
	}
	ops := map[string]func(s *application.Service) error{
		"CreateItem": func(s *application.Service) error { _, err := s.CreateItem(ctx, "Falha", ""); return err },
		"GetItem":    func(s *application.Service) error { _, err := s.GetItem(ctx, created.ID); return err },
		"ListItems": func(s *application.Service) error {
			_, _, err := s.ListItems(ctx, pagination.New(1, 5, 100))
			return err
		},
	}
	for name, op := range ops {
		probe := &faultRepo{inner: infrastructure.NewPostgresRepository()}
		if err := op(svc(probe)); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			for _, poison := range []bool{false, true} {
				r := &faultRepo{inner: infrastructure.NewPostgresRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := op(svc(r)); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Error("erro desconhecido passa adiante")
	}
}

func TestHandlersReportServiceFailures(t *testing.T) {
	pool := testPool(t)
	down := &faultRepo{inner: infrastructure.NewPostgresRepository(), failAt: 1}
	h := transport.NewHandlers(application.NewService(pool, down, outbox.NewWriter("nexus.example"), testLogger()), testLogger())
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Permissions: []string{"*"}})))
		})
	})
	transport.RegisterRoutes(r, h, testLogger())
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/examples", ""},
		{http.MethodPost, "/examples", `{"title":"x"}`},
	} {
		down.calls = 0
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("%s %s com o banco fora: %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
