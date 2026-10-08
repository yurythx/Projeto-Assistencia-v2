// Package example é o módulo-modelo (blueprint) de um plugin do Nexus:
// Clean Architecture (domain/application/infrastructure/transport),
// Transactional Outbox, auditoria na mesma transação e autorização por
// escopo. Use `make new-module NAME=...` para gerar um novo a partir dele.
package example

import (
	"github.com/yurythx/projeto-nexus/internal/modules/example/application"
	"github.com/yurythx/projeto-nexus/internal/modules/example/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/example/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

// Key é a chave do módulo no Kernel.
const Key = "example"

// Module é o plugin-modelo.
type Module struct {
	deps     modkit.Deps
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewPostgresRepository(), deps.Outbox, deps.Logger)
	return &Module{deps: deps, handlers: transport.NewHandlers(svc, deps.Logger)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Módulo Modelo",
		Description:    "Blueprint de referência para novos plugins (desativado por padrão).",
		DefaultEnabled: false,
		Icon:           "box",
		Route:          "/exemplos",
		Permissions:    []kernel.PermissionInfo{{Key: "example:manage", Description: "Criar itens do módulo-modelo"}},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) {
	transport.RegisterRoutes(r.Authed, m.handlers, m.deps.Logger)
}
