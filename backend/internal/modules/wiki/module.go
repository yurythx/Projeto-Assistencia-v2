// Package wiki é o plugin Wiki: base de conhecimento em árvore Markdown
// colaborativa (wiki:manage para exclusões).
package wiki

import (
	"context"

	"github.com/yurythx/projeto-nexus/internal/modules/wiki/application"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// Key é a chave do módulo.
const Key = "wiki"

// Module é o plugin.
type Module struct {
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository())
	return &Module{svc: svc, handlers: transport.NewHandlers(svc, deps.Logger)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Wiki",
		Description:    "Base de conhecimento em árvore Markdown colaborativa com histórico de revisões.",
		DefaultEnabled: true,
		Icon:           "book-open",
		Route:          "/wiki",
		Permissions:    []kernel.PermissionInfo{{Key: "wiki:manage", Description: "Excluir páginas da wiki"}},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) { m.handlers.RegisterRoutes(r.Authed) }

// SearchProviders implementa kernel.SearchProvider.
func (m *Module) SearchProviders() []search.Provider { return []search.Provider{provider{m.svc}} }

type provider struct{ svc *application.Service }

func (provider) Module() string { return Key }

func (p provider) Search(ctx context.Context, identity auth.Identity, q string, limit int) ([]search.Result, error) {
	pages, ranks, err := p.svc.Search(ctx, identity, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]search.Result, 0, len(pages))
	for i, page := range pages {
		updated := page.UpdatedAt
		out = append(out, search.Result{
			Module: Key, Type: "pagina", ID: page.ID.String(), Title: page.Title,
			Snippet: modkit.Snippet(page.Body, 200), URL: "/wiki/" + page.Slug, Score: ranks[i], UpdatedAt: &updated,
		})
	}
	return out, nil
}
