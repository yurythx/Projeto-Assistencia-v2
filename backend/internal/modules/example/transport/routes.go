package transport

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

// RegisterRoutes monta as rotas do módulo-modelo. Leitura: qualquer
// autenticado; criação: example:manage (blueprint de autorização por
// escopo "recurso:ação" validada em middleware — A01).
func RegisterRoutes(r chi.Router, h *Handlers, logger *slog.Logger) {
	r.Route("/examples", func(r chi.Router) {
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.With(auth.RequirePermission(logger, auth.PermExampleManage)).Post("/", h.Create)
	})
}
