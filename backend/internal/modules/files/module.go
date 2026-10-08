// Package files é o plugin Arquivos: pastas e arquivos sobre o MinIO com
// permissões por grupo do AD, perfil, lotação ou usuário.
package files

import (
	"context"

	"github.com/yurythx/projeto-nexus/internal/modules/files/application"
	"github.com/yurythx/projeto-nexus/internal/modules/files/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/files/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// Key é a chave do módulo.
const Key = "files"

// Module é o plugin.
type Module struct {
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), deps.Outbox, deps.Storage,
		deps.Config.MinIO.Bucket, deps.Config.Upload.MaxFileBytes, deps.Config.Upload.URLExpiry, deps.Logger)
	return &Module{svc: svc, handlers: transport.NewHandlers(svc, deps.Logger)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Arquivos",
		Description:    "Gestão de pastas e arquivos sobre MinIO com permissões por grupo/perfil.",
		DefaultEnabled: true,
		Icon:           "folder",
		Route:          "/arquivos",
		Permissions:    []kernel.PermissionInfo{{Key: "files:manage", Description: "Acesso total a todas as pastas (moderação)"}},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) { m.handlers.RegisterRoutes(r.Authed) }

// Workers implementa kernel.WorkerProvider.
func (m *Module) Workers() []kernel.Worker {
	return []kernel.Worker{{Name: "stale-upload-gc", Process: kernel.ProcessWorker, Run: m.svc.CollectStaleUploads}}
}

// SearchProviders implementa kernel.SearchProvider.
func (m *Module) SearchProviders() []search.Provider { return []search.Provider{provider{m.svc}} }

type provider struct{ svc *application.Service }

func (provider) Module() string { return Key }

func (p provider) Search(ctx context.Context, identity auth.Identity, q string, limit int) ([]search.Result, error) {
	files, err := p.svc.Search(ctx, identity, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]search.Result, 0, len(files))
	for _, f := range files {
		updated := f.UpdatedAt
		out = append(out, search.Result{
			Module: Key, Type: "arquivo", ID: f.ID.String(), Title: f.Name, Snippet: f.ContentType,
			URL: "/arquivos?pasta=" + f.FolderID.String(), Score: 0.4, UpdatedAt: &updated,
		})
	}
	return out, nil
}
