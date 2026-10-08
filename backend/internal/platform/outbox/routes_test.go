package outbox

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestRegisterStatsRoutes_MountsRelativeToAPIV1Group cobre o mesmo achado
// de auditoria já coberto em lgpd/audit/keycloakconfig: RegisterStatsRoutes
// precisa usar caminhos RELATIVOS ao grupo em que
// internal/app/router.go o monta ("/api/v1"), nunca absolutos.
func TestRegisterStatsRoutes_MountsRelativeToAPIV1Group(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewStatsHandlers(nil, logger)

	root := chi.NewRouter()
	root.Route("/api/v1", func(api chi.Router) {
		RegisterStatsRoutes(api, h, logger)
	})

	var patterns []string
	err := chi.Walk(root, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		patterns = append(patterns, route)
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}

	const want = "/api/v1/monitoring/outbox-stats"
	found := false
	for _, p := range patterns {
		if strings.Count(p, "/api/v1") > 1 {
			t.Errorf("rota %q duplica o prefixo /api/v1", p)
		}
		if p == want {
			found = true
		}
	}
	if !found {
		t.Errorf("rota esperada %q não foi registrada (rotas de fato registradas: %v)", want, patterns)
	}
}
