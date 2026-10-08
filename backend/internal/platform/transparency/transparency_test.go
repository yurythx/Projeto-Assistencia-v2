package transparency

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/lgpd"
)

type allow struct{}

func (allow) Allow(context.Context, string) (bool, error) { return true, nil }

func router(s *Service) http.Handler {
	r := chi.NewRouter()
	RegisterRoutes(r, s, allow{})
	return r
}

func get(h http.Handler, path, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func modules() []ModuleInfo {
	return []ModuleInfo{
		{Key: "directory", Name: "Pessoas & Setores", Description: "Consulta <pública>", Enabled: true},
		{Key: "wiki", Name: "Wiki", Enabled: false},
	}
}

func TestDatasetsInEveryFormat(t *testing.T) {
	pool := dbtest.Pool(t)
	h := router(NewService(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), modules))

	var plat struct {
		Registros []map[string]any `json:"registros"`
	}
	rec := get(h, "/transparencia/plataforma", "")
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &plat) != nil || plat.Registros[0]["modulos_ativos"] != float64(1) ||
		plat.Registros[0]["termos_lgpd_vigentes"] != lgpd.CurrentTermVersion {
		t.Fatalf("plataforma: %d %s", rec.Code, rec.Body.String())
	}
	// XML bem formado mesmo com & e < nos valores.
	x := get(h, "/transparencia/modulos?format=xml", "")
	dec := xml.NewDecoder(strings.NewReader(x.Body.String()))
	for {
		if _, err := dec.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("XML inválido: %v\n%s", err, x.Body.String())
		}
	}
	if !strings.Contains(x.Body.String(), "Pessoas &amp; Setores") {
		t.Fatalf("valores escapados: %s", x.Body.String())
	}
	if c := get(h, "/transparencia/modulos", "text/csv"); !strings.HasPrefix(c.Body.String(), "ativo,chave,descricao,nome") {
		t.Fatalf("CSV com cabeçalho ordenado: %s", c.Body.String())
	}
	if c := get(h, "/transparencia/modulos", "text/xml"); !strings.Contains(c.Header().Get("Content-Type"), "xml") {
		t.Fatal("Accept: text/xml")
	}
	if c := get(h, "/transparencia/datasets?format=json", ""); !strings.Contains(c.Body.String(), `"nota"`) {
		t.Fatal("catálogo com nota")
	}
	empty := router(NewService(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), func() []ModuleInfo { return nil }))
	if c := get(empty, "/transparencia/modulos?format=csv", ""); c.Body.String() != "" {
		t.Fatalf("CSV sem registros fica vazio: %q", c.Body.String())
	}
	if a := get(h, "/transparencia/auditoria/acoes?dias=7", ""); a.Code != 200 || !strings.Contains(a.Body.String(), `"periodo_dias":7`) {
		t.Fatalf("ações auditadas: %d %s", a.Code, a.Body.String())
	}
	for _, bad := range []string{"0", "366", "abc"} {
		if a := get(h, "/transparencia/auditoria/acoes?dias="+bad, ""); a.Code != http.StatusBadRequest {
			t.Errorf("dias=%s: %d", bad, a.Code)
		}
	}
}

func TestPublicDataNeverPublishedWrong(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	down := router(&Service{db: dbtest.Fail{}, logger: logger, modules: modules})
	for _, p := range []string{"/transparencia/plataforma", "/transparencia/auditoria/acoes"} {
		if rec := get(down, p, ""); rec.Code != http.StatusInternalServerError {
			t.Errorf("%s com o banco fora: %d (nunca um número falso)", p, rec.Code)
		}
	}
	bad := router(&Service{db: &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.BadRows()}}}, logger: logger, modules: modules})
	if rec := get(bad, "/transparencia/auditoria/acoes", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("linha ilegível: %d", rec.Code)
	}
}
