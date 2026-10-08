package lgpd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestRegisterRoutes_MountsRelativeToAPIV1Group cobre um bug real
// encontrado em auditoria: RegisterRoutes registrava caminhos ABSOLUTOS
// ("/api/v1/lgpd/status") dentro de um router que internal/app/router.go
// já monta em "/api/v1" (via r.Route("/api/v1", func(api chi.Router)
// {...})) — o endpoint só existia de fato em
// "/api/v1/api/v1/lgpd/status", 404 no caminho real que o frontend chama.
// Este teste reproduz a mesma topologia de montagem (RegisterRoutes
// dentro de um grupo "/api/v1") e usa chi.Walk pra garantir que nenhuma
// rota registrada acaba duplicando o prefixo.
func TestRegisterRoutes_MountsRelativeToAPIV1Group(t *testing.T) {
	svc := &Service{}

	root := chi.NewRouter()
	root.Route("/api/v1", func(api chi.Router) {
		svc.RegisterRoutes(api)
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

	want := map[string]bool{"/api/v1/lgpd/status": false, "/api/v1/lgpd/accept": false}
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

// TestRegisterDSRRoutes_MountsRelativeToAPIV1Group — mesmo cuidado para os
// endpoints de direitos do titular (LGPD art. 18 / F3.2).
func TestRegisterDSRRoutes_MountsRelativeToAPIV1Group(t *testing.T) {
	svc := &Service{}
	root := chi.NewRouter()
	root.Route("/api/v1", func(api chi.Router) { svc.RegisterDSRRoutes(api) })

	var patterns []string
	if err := chi.Walk(root, func(_, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		patterns = append(patterns, route)
		return nil
	}); err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}

	want := map[string]bool{
		"/api/v1/lgpd/meus-dados":          false,
		"/api/v1/lgpd/solicitar-exclusao":  false,
		"/api/v1/lgpd/minhas-solicitacoes": false,
	}
	for _, p := range patterns {
		if strings.Count(p, "/api/v1") > 1 {
			t.Errorf("rota %q duplica o prefixo /api/v1", p)
		}
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("rota esperada %q não registrada (registradas: %v)", route, patterns)
		}
	}
}
