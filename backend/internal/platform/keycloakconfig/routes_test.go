package keycloakconfig

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
)

// TestRegisterRoutes_MountsRelativeToAPIV1Group cobre o mesmo achado de
// auditoria já coberto em internal/platform/lgpd/routes_test.go e
// internal/platform/audit/routes_test.go: RegisterRoutes precisa usar
// caminhos RELATIVOS ao grupo em que internal/app/router.go o monta
// ("/api/v1"), nunca absolutos — um "/api/v1/..." aqui dentro duplicaria
// o prefixo e deixaria o endpoint inalcançável no caminho real
// (/api/v1/api/v1/admin/keycloak).
func TestRegisterRoutes_MountsRelativeToAPIV1Group(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandlers(nil, nil, config.KeycloakConfig{}, nil, logger)

	root := chi.NewRouter()
	root.Route("/api/v1", func(api chi.Router) {
		RegisterRoutes(api, h, logger)
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

	want := map[string]bool{
		"/api/v1/admin/keycloak/":     false,
		"/api/v1/admin/keycloak/test": false,
	}
	for _, p := range patterns {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("rota esperada %q não foi registrada (rotas de fato registradas: %v)", route, patterns)
		}
	}
}
