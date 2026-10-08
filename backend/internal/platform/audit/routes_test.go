package audit

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestExporterRegisterRoutes_MountsRelativeToAPIV1Group — mesmo achado de
// auditoria do pacote lgpd (ver internal/platform/lgpd/routes_test.go):
// RegisterRoutes registrava "/api/v1/audit/export" dentro de um router já
// montado em "/api/v1", duplicando o prefixo e deixando o endpoint
// inalcançável no caminho real.
func TestExporterRegisterRoutes_MountsRelativeToAPIV1Group(t *testing.T) {
	e := &Exporter{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	root := chi.NewRouter()
	root.Route("/api/v1", func(api chi.Router) {
		e.RegisterRoutes(api)
	})

	var patterns []string
	err := chi.Walk(root, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		patterns = append(patterns, route)
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	if len(patterns) == 0 {
		t.Fatal("RegisterRoutes não registrou nenhuma rota")
	}
	for _, p := range patterns {
		if strings.Count(p, "/api/v1") > 1 {
			t.Errorf("rota %q duplica o prefixo /api/v1 — RegisterRoutes deve usar caminhos relativos ao grupo em que é montado", p)
		}
	}

	const want = "/api/v1/audit/export"
	found := false
	for _, p := range patterns {
		if p == want {
			found = true
		}
	}
	if !found {
		t.Errorf("rota esperada %q não foi registrada (rotas de fato registradas: %v)", want, patterns)
	}
}
