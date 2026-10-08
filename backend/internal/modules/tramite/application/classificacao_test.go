package application_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/tramite/application"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// Classificação pelo Atlas (ADR 026): na abertura ou depois, inclusive com
// o processo encerrado; o código da série é conferido.
func TestClassificacaoPeloAtlas(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.real()
	proc := uuid.New()

	p, err := s.Abrir(ctx, e.author, application.AbrirInput{TipoID: e.tipo, Assunto: "Pregão", Sigilo: "publico",
		UnidadeOrigemID: e.unidade, AtlasProcedimentoID: &proc, CodigoTTDD: " 2.0.02.00.07 "})
	if err != nil || p.AtlasProcedimentoID == nil || *p.AtlasProcedimentoID != proc || p.CodigoTTDD != "2.0.02.00.07" {
		t.Fatalf("abrir classificado: %+v %v", p, err)
	}
	if _, err := s.Abrir(ctx, e.author, application.AbrirInput{TipoID: e.tipo, Assunto: "x", Sigilo: "publico",
		UnidadeOrigemID: e.unidade, CodigoTTDD: "2.0.02"}); status(err) != http.StatusUnprocessableEntity {
		t.Fatalf("série inválida na abertura: %v", err)
	}

	// Sem classificação; classificar depois de concluir (a guarda vale para o encerrado).
	q := e.processo(domain.SigiloPublico)
	if q.AtlasProcedimentoID != nil || q.CodigoTTDD != "" {
		t.Fatalf("aberto sem classificação: %+v", q)
	}
	if _, err := s.Concluir(ctx, e.author, q.ID, "Concluído"); err != nil {
		t.Fatal(err)
	}
	c, err := s.Classificar(ctx, e.author, q.ID, &proc, "2.0.02.00.07")
	if err != nil || c.CodigoTTDD != "2.0.02.00.07" || c.AtlasProcedimentoID == nil || c.Status != domain.StatusConcluido {
		t.Fatalf("classificar: %+v %v", c, err)
	}
	if c, err = s.Classificar(ctx, e.author, q.ID, nil, ""); err != nil || c.CodigoTTDD != "" || c.AtlasProcedimentoID != nil {
		t.Fatalf("retirar a classificação: %+v %v", c, err)
	}
	if _, err := s.Classificar(ctx, e.author, q.ID, nil, "x"); status(err) != http.StatusUnprocessableEntity {
		t.Fatalf("série inválida: %v", err)
	}
	if _, err := s.Classificar(ctx, e.author, uuid.New(), nil, ""); status(err) != http.StatusNotFound {
		t.Fatalf("processo inexistente: %v", err)
	}
	// Quem não pode movimentar o processo não classifica.
	alheio := auth.Identity{UserID: dbtest.User(t, e.pool)}
	if _, err := s.Classificar(ctx, alheio, p.ID, nil, ""); status(err) != http.StatusForbidden {
		t.Fatalf("sem poder movimentar: %v", err)
	}
}

func TestClassificacaoFalhas(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	p := e.processo(domain.SigiloPublico)
	probe := &faultRepo{Repository: infrastructure.NewRepository()}
	if _, err := e.svc(probe, fakeSign{available: true}).Classificar(ctx, e.author, p.ID, nil, "2.0.02.00.07"); err != nil {
		t.Fatal(err)
	}
	for i := range probe.trace {
		for _, poison := range []bool{false, true} {
			r := &faultRepo{Repository: infrastructure.NewRepository()}
			if poison {
				r.poisonAt = i + 1
			} else {
				r.failAt = i + 1
			}
			if _, err := e.svc(r, fakeSign{available: true}).Classificar(ctx, e.author, p.ID, nil, "2.0.02.00.07"); err == nil && !r.noTx {
				t.Errorf("falha (veneno=%v) na chamada %d (%s) engolida", poison, i+1, probe.trace[i])
			}
		}
	}
}

func TestClassificacaoHTTP(t *testing.T) {
	e := newEnv(t)
	p := e.processo(domain.SigiloPublico)
	router := func(svc *application.Service) *chi.Mux {
		h := transport.NewHandlers(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), e.author)))
			})
		})
		h.RegisterRoutes(r)
		return r
	}
	r := router(e.real())
	do := func(r *chi.Mux, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}
	base := "/tramite/processos/" + p.ID.String() + "/classificacao"
	for body, want := range map[string]int{
		`{"atlas_procedimento_id":"` + uuid.NewString() + `","codigo_ttdd":"2.0.02.00.07"}`: http.StatusOK,
		`{"codigo_ttdd":"abc"}`: http.StatusUnprocessableEntity,
		`{`:                     http.StatusBadRequest,
	} {
		if rec := do(r, base, body); rec.Code != want {
			t.Errorf("PUT %s: %d, quero %d — %s", body, rec.Code, want, rec.Body.String())
		}
	}
	if rec := do(r, "/tramite/processos/x/classificacao", `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("id inválido: %d", rec.Code)
	}
	down := router(e.svc(&faultRepo{Repository: infrastructure.NewRepository(), failAt: 1}, fakeSign{available: true}))
	if rec := do(down, base, `{}`); rec.Code != http.StatusInternalServerError {
		t.Errorf("banco fora: %d", rec.Code)
	}
}
