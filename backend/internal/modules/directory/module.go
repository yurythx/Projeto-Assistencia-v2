// Package directory é o plugin Diretório: consulta interna de pessoas e
// pública de setores, a partir dos dados sincronizados do AD.
package directory

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/application"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// Key é a chave do módulo.
const Key = "directory"

// Module é o plugin.
type Module struct {
	deps     modkit.Deps
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository())
	return &Module{deps: deps, svc: svc, handlers: transport.NewHandlers(svc, deps.Logger, deps.Config.MaxPageSize)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Diretório",
		Description:    "Consulta de pessoas e setores com base nos dados sincronizados do AD.",
		DefaultEnabled: true,
		Public:         true,
		Icon:           "contact",
		Route:          "/diretorio",
		Permissions:    []kernel.PermissionInfo{{Key: "directory:manage", Description: "Editar o perfil e a lotação exibida de qualquer pessoa"}},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) {
	r.Public.Group(func(pub chi.Router) {
		pub.Use(httpserver.RateLimit(m.deps.Logger, m.deps.PublicLimiter, httpserver.ClientIPKey))
		m.handlers.RegisterPublicRoutes(pub)
	})
	m.handlers.RegisterAuthedRoutes(r.Authed)
}

// SearchProviders implementa kernel.SearchProvider.
func (m *Module) SearchProviders() []search.Provider { return []search.Provider{provider{m.svc}} }

type provider struct{ svc *application.Service }

func (provider) Module() string { return Key }

func (p provider) Search(ctx context.Context, identity auth.Identity, q string, limit int) ([]search.Result, error) {
	people, _, err := p.svc.List(ctx, identity, domain.Filter{Query: q}, pagination.New(1, limit, limit))
	if err != nil {
		return nil, err
	}
	out := make([]search.Result, 0, len(people))
	for _, person := range people {
		snippet := person.JobTitle
		if person.Departamento != "" {
			snippet += " · " + person.Departamento
		}
		out = append(out, search.Result{
			Module: Key, Type: "pessoa", ID: person.UserID.String(), Title: person.Name,
			Snippet: snippet, URL: "/diretorio/" + person.UserID.String(), Score: 0.5,
		})
	}
	return out, nil
}

// ExportPersonalData implementa lgpd.PersonalData.
func (m *Module) ExportPersonalData(ctx context.Context, db database.DBTX, userID uuid.UUID) (any, error) {
	return m.svc.ExportPersonalData(ctx, db, userID)
}

// ErasePersonalData implementa lgpd.PersonalData.
func (m *Module) ErasePersonalData(ctx context.Context, tx database.DBTX, userID uuid.UUID) error {
	return m.svc.ErasePersonalData(ctx, tx, userID)
}
