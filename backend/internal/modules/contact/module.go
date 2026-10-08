// Package contact é o plugin Contato: formulário público institucional
// com rate limit dedicado e evento contact.message.submitted.
package contact

import (
	"github.com/yurythx/projeto-nexus/internal/modules/contact/application"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

// Key é a chave do módulo.
const Key = "contact"

// Module é o plugin.
type Module struct {
	deps     modkit.Deps
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), deps.Outbox)
	return &Module{deps: deps, handlers: transport.NewHandlers(svc, deps.Logger, deps.Config.MaxPageSize)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Contato",
		Description:    "Formulário público institucional com rate limit dedicado.",
		DefaultEnabled: true,
		Public:         true,
		Icon:           "mail",
		Route:          "/gestao/contato",
		Permissions: []kernel.PermissionInfo{
			{Key: "contact:read", Description: "Ler mensagens recebidas (contém dados pessoais)"},
			{Key: "contact:manage", Description: "Triar e responder mensagens"},
		},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) {
	m.handlers.RegisterPublicRoutes(r.Public, m.deps.ContactLimiter)
	m.handlers.RegisterAdminRoutes(r.Authed)
}
