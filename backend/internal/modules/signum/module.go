// Package signum é o plugin Signum: motor de assinatura eletrônica com
// cerimônia de reautenticação e hash de integridade SHA-256.
package signum

import (
	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/signum/application"
	"github.com/yurythx/projeto-nexus/internal/modules/signum/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/signum/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

// Key é a chave do módulo.
const Key = "signum"

// Module é o plugin.
type Module struct {
	deps     modkit.Deps
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps, creds infrastructure.KeycloakCredentials, throttle application.Throttle) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(),
		infrastructure.NewReauthenticator(deps.Pool, creds), throttle, deps.Outbox,
		deps.Config.Security.ConfigEncryptionKey, deps.Config.Signum.ChallengeTTL, deps.Logger)
	return &Module{deps: deps, svc: svc, handlers: transport.NewHandlers(svc, deps.Logger)}
}

// Service expõe o serviço para as portas de outros plugins (ligadas em
// internal/app — nenhum plugin importa este pacote).
func (m *Module) Service() *application.Service { return m.svc }

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Signum",
		Description:    "Assinatura eletrônica com cerimônia de reautenticação e hash de integridade SHA-256.",
		DefaultEnabled: true,
		Public:         true,
		Icon:           "pen-tool",
		Route:          "/signum",
		Permissions:    []kernel.PermissionInfo{{Key: "signum:manage", Description: "Ver e cancelar qualquer envelope"}},
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
