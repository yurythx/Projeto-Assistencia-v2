// Package mercurio é o plugin Mercúrio: chat em tempo real via WebSocket
// com backplane Redis (canais globais, salas departamentais mapeadas via
// AD e mensagens diretas).
package mercurio

import (
	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/application"
	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// Key é a chave do módulo (também o prefixo dos tópicos WebSocket).
const Key = "mercurio"

// Module é o plugin.
type Module struct {
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), deps.Hub, deps.Outbox, deps.Logger)
	return &Module{svc: svc, handlers: transport.NewHandlers(svc, deps.Logger)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Mercúrio",
		Description:    "Chat em tempo real: canais globais, salas departamentais mapeadas via AD e mensagens diretas.",
		DefaultEnabled: true,
		Icon:           "messages-square",
		Route:          "/mercurio",
		Permissions:    []kernel.PermissionInfo{{Key: "mercurio:manage", Description: "Criar/arquivar salas e moderar mensagens"}},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) { m.handlers.RegisterRoutes(r.Authed) }

// WSAuthorizer implementa kernel.WSProvider.
func (m *Module) WSAuthorizer() ws.Authorizer { return m.svc.Authorize }

// WSInbound implementa kernel.WSProvider.
func (m *Module) WSInbound() ws.InboundHandler { return m.svc.Inbound }
