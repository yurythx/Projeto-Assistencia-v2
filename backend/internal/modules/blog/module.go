// Package blog é o plugin Blog: publicações internas e comunicados com
// fluxo rascunho -> publicação; publicar emite blog.post.published.
package blog

import (
	"context"

	"github.com/yurythx/projeto-nexus/internal/modules/blog/application"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// Key é a chave do módulo.
const Key = "blog"

// Module é o plugin.
type Module struct {
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), deps.Outbox, deps.Storage,
		deps.Config.MinIO.Bucket, deps.Config.Upload.URLExpiry, deps.Logger)
	return &Module{svc: svc, handlers: transport.NewHandlers(svc, deps.Logger, deps.Config.MaxPageSize)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Blog",
		Description:    "Publicações internas e comunicados com rascunho e publicação.",
		DefaultEnabled: true,
		Icon:           "newspaper",
		Route:          "/blog",
		Permissions:    []kernel.PermissionInfo{{Key: "blog:manage", Description: "Criar, editar, publicar e arquivar publicações"}},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) { m.handlers.RegisterRoutes(r.Authed) }

// SearchProviders implementa kernel.SearchProvider.
func (m *Module) SearchProviders() []search.Provider { return []search.Provider{provider{m.svc}} }

type provider struct{ svc *application.Service }

func (provider) Module() string { return Key }

func (p provider) Search(ctx context.Context, identity auth.Identity, q string, limit int) ([]search.Result, error) {
	posts, ranks, err := p.svc.Search(ctx, identity, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]search.Result, 0, len(posts))
	for i, post := range posts {
		updated := post.UpdatedAt
		out = append(out, search.Result{
			Module: Key, Type: post.Kind, ID: post.ID.String(), Title: post.Title,
			Snippet: modkit.Snippet(post.Summary, 180), URL: "/blog/" + post.Slug, Score: ranks[i], UpdatedAt: &updated,
		})
	}
	return out, nil
}
