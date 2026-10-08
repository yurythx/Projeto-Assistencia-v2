package application_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// serieVigente é a série usada pelos procedimentos de teste.
const serieVigente = "2.0.02.00.07"

// serieRevogada cria (uma vez) uma série revogada para os testes.
func (e *env) serieRevogada() string {
	e.t.Helper()
	const codigo = "2.0.02.00.95"
	if _, err := e.pool.Exec(context.Background(), `INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor,
		fase_corrente_anos, fase_interm_anos, destinacao_final, revogada_em, revogada_edicao)
		VALUES ($1, '2.0.02.00', 'Série revogada de teste dos modelos', 1, 2, 'ELIMINACAO', '2027-03-01', '6.400')
		ON CONFLICT (codigo) DO NOTHING`, codigo); err != nil {
		e.t.Fatal(err)
	}
	return codigo
}

func TestModelosDaSerie(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()
	m, outro := e.modelo(), e.modelo()

	got, err := s.LigarModeloSerie(ctx, gestor, serieVigente, m.ID)
	if err != nil || !contem(got, m.ID) {
		t.Fatalf("ligar: %+v %v", got, err)
	}
	// Repetir não duplica.
	if got, err = s.LigarModeloSerie(ctx, gestor, serieVigente, m.ID); err != nil || contar(got, m.ID) != 1 {
		t.Fatalf("ligar de novo: %+v %v", got, err)
	}
	if _, err := s.LigarModeloSerie(ctx, gestor, serieVigente, outro.ID); err != nil {
		t.Fatal(err)
	}
	// Desativado continua listado na série (segue baixável).
	if _, err := s.AlterarModelo(ctx, gestor, domain.Modelo{ID: outro.ID, Nome: outro.Nome, Ativo: false}); err != nil {
		t.Fatal(err)
	}
	if lista, err := s.ModelosDaSerie(ctx, serieVigente); err != nil || !contem(lista, outro.ID) || !contem(lista, m.ID) {
		t.Fatalf("listar: %v", err)
	}
	if got, err = s.DesligarModeloSerie(ctx, serieVigente, m.ID); err != nil || contem(got, m.ID) {
		t.Fatalf("desligar: %+v %v", got, err)
	}

	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"modelo desativado": {func() error { _, err := s.LigarModeloSerie(ctx, gestor, serieVigente, outro.ID); return err }(), "VALIDATION_ERROR"},
		"modelo inexistente": {func() error {
			_, err := s.LigarModeloSerie(ctx, gestor, serieVigente, uuid.New())
			return err
		}(), "VALIDATION_ERROR"},
		"série revogada":    {func() error { _, err := s.LigarModeloSerie(ctx, gestor, e.serieRevogada(), m.ID); return err }(), "VALIDATION_ERROR"},
		"série inexistente": {func() error { _, err := s.LigarModeloSerie(ctx, gestor, "99.0.01.00.01", m.ID); return err }(), "NOT_FOUND"},
		"desligar o que não está ligado": {func() error {
			_, err := s.DesligarModeloSerie(ctx, serieVigente, m.ID)
			return err
		}(), "NOT_FOUND"},
		"listar série inexistente": {func() error { _, err := s.ModelosDaSerie(ctx, "99.0.01.00.01"); return err }(), "NOT_FOUND"},
	} {
		if codigo(c.err) != c.want {
			t.Errorf("%s: %v, quero %s", name, c.err, c.want)
		}
	}
}

func contar(ms []domain.Modelo, id uuid.UUID) int {
	n := 0
	for _, m := range ms {
		if m.ID == id {
			n++
		}
	}
	return n
}

func TestModelosDaSerieHTTP(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	m := e.modelo()
	router := func(down bool) *chi.Mux {
		svc := e.real()
		if down {
			svc = e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 1})
		}
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), gestor)))
			})
		})
		h := transport.NewHandlers(svc, logger, 100)
		h.RegisterPublicRoutes(r)
		h.RegisterRoutes(r, unlimited{})
		return r
	}
	do := func(r *chi.Mux, method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}
	r := router(false)
	admin := "/atlas/admin/ttdd/" + serieVigente + "/modelos"
	for _, c := range []struct {
		method, path, body string
		want               int
	}{
		{http.MethodPost, admin, `{"modelo_id":"` + m.ID.String() + `"}`, http.StatusOK},
		{http.MethodGet, "/atlas/ttdd/" + serieVigente + "/modelos", "", http.StatusOK},
		{http.MethodDelete, admin + "/" + m.ID.String(), "", http.StatusOK},
		{http.MethodPost, admin, `{}`, http.StatusUnprocessableEntity},
		{http.MethodPost, "/atlas/admin/ttdd/x/modelos", `{}`, http.StatusBadRequest},
		{http.MethodDelete, "/atlas/admin/ttdd/x/modelos/" + m.ID.String(), "", http.StatusBadRequest},
		{http.MethodDelete, admin + "/x", "", http.StatusBadRequest},
		{http.MethodGet, "/atlas/ttdd/x/modelos", "", http.StatusBadRequest},
	} {
		if rec := do(r, c.method, c.path, c.body); rec.Code != c.want {
			t.Errorf("%s %s: %d, quero %d — %s", c.method, c.path, rec.Code, c.want, rec.Body.String())
		}
	}
	// Lista pública sem autoria.
	do(r, http.MethodPost, admin, `{"modelo_id":"`+m.ID.String()+`"}`)
	if body := do(r, http.MethodGet, "/atlas/ttdd/"+serieVigente+"/modelos", "").Body.String(); !strings.Contains(body, m.Nome) ||
		strings.Contains(body, `"updated_by":"gestor"`) {
		t.Fatalf("lista pública: %s", body)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/atlas/ttdd/" + serieVigente + "/modelos", ""},
		{http.MethodPost, admin, `{"modelo_id":"` + m.ID.String() + `"}`},
		{http.MethodDelete, admin + "/" + m.ID.String(), ""},
	} {
		if rec := do(router(true), c.method, c.path, c.body); rec.Code != http.StatusInternalServerError {
			t.Errorf("%s %s com o banco fora: %d", c.method, c.path, rec.Code)
		}
	}
}

// Busca Global: procedimentos, séries da TTDD e modelos; o assistente cita os
// modelos ativos da série das fontes.
func TestBuscaGlobalEAssistenteComModelos(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()
	unico := "Zelofax" + strings.ToUpper(uuid.NewString()[:6])
	m, err := s.CriarModelo(ctx, gestor, domain.Modelo{Nome: "Modelo " + unico, Descricao: "Edital de pregão"}, docx())
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.BuscaGlobal(ctx, "atlas", unico, 5)
	if err != nil || len(res) != 1 || res[0].Type != "modelo" || res[0].ID != m.ID.String() ||
		res[0].URL != "/atlas/modelos?q=Modelo+"+unico || res[0].Module != "atlas" {
		t.Fatalf("modelo na busca: %+v %v", res, err)
	}
	res, err = s.BuscaGlobal(ctx, "atlas", serieVigente, 5)
	exata := false // código exato: a série com a maior pontuação
	for _, r := range res {
		exata = exata || (r.Type == "serie_ttdd" && r.URL == "/atlas/ttdd/"+serieVigente && r.Score == 1)
	}
	if err != nil || !exata {
		t.Fatalf("série na busca: %+v %v", res, err)
	}

	if _, err := s.LigarModeloSerie(ctx, gestor, serieVigente, m.ID); err != nil {
		t.Fatal(err)
	}
	r, err := s.Perguntar(ctx, "qual o prazo de guarda dos processos de pregão eletrônico?")
	if err != nil || !strings.Contains(r.Answer, "Modelos de documento para baixar (biblioteca do Atlas, série "+serieVigente+")") ||
		!strings.Contains(r.Answer, "Modelo "+unico) {
		t.Fatalf("assistente sem os modelos da série: %+v %v", r, err)
	}
	if _, err := s.DesligarModeloSerie(ctx, serieVigente, m.ID); err != nil {
		t.Fatal(err)
	}
}
