// Package catalog é o plugin Catálogo: central pública de serviços
// institucionais (leitura anônima, edição com catalog:manage).
package catalog

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/catalog/application"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// Key é a chave do módulo.
const Key = "catalog"

// Module é o plugin.
type Module struct {
	deps     modkit.Deps
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), deps.Outbox)
	return &Module{deps: deps, svc: svc, handlers: transport.NewHandlers(svc, deps.Logger, deps.Config.MaxPageSize)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Catálogo de Serviços",
		Description:    "Central pública de serviços institucionais.",
		DefaultEnabled: true,
		Public:         true,
		Icon:           "layout-grid",
		Route:          "/gestao/servicos",
		Permissions:    []kernel.PermissionInfo{{Key: "catalog:manage", Description: "Criar, editar e publicar serviços do catálogo"}},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) {
	r.Public.Group(func(pub chi.Router) {
		pub.Use(httpserver.RateLimit(m.deps.Logger, m.deps.PublicLimiter, httpserver.ClientIPKey))
		m.handlers.RegisterPublicRoutes(pub)
	})
	m.handlers.RegisterAdminRoutes(r.Authed)
}

// SearchProviders implementa kernel.SearchProvider.
func (m *Module) SearchProviders() []search.Provider { return []search.Provider{provider{m.svc}} }

type provider struct{ svc *application.Service }

func (provider) Module() string { return Key }

func (p provider) Search(ctx context.Context, _ auth.Identity, q string, limit int) ([]search.Result, error) {
	items, ranks, err := p.svc.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]search.Result, 0, len(items))
	for i, s := range items {
		updated := s.UpdatedAt
		out = append(out, search.Result{
			Module: Key, Type: "servico", ID: s.ID.String(), Title: s.Title,
			Snippet: modkit.Snippet(s.Summary, 180), URL: "/servicos/" + s.Slug, Score: ranks[i], UpdatedAt: &updated,
		})
	}
	return out, nil
}
