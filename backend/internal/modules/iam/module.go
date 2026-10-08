// Package iam é o módulo núcleo "IAM & Usuários" (Core imutável — nunca
// desativável): gestão de identidades, estrutura organizacional
// multi-escopo, Perfis, lotações e o mapeamento de grupos do Active
// Directory para Perfis + escopos.
package iam

import (
	"github.com/yurythx/projeto-nexus/internal/modules/iam/application"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

// Key é a chave do módulo no Kernel.
const Key = "iam"

// Module é o plugin do IAM.
type Module struct {
	handlers *transport.Handlers
}

// New constrói o módulo. catalog lista as permissões de todos os plugins
// (fornecido pelo Kernel, que conhece os manifestos).
func New(deps modkit.Deps, catalog transport.PermissionCatalog) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), deps.InvalidatePermissions).
		WithLoginLockoutReset(deps.ResetLoginLockout)
	return &Module{handlers: transport.NewHandlers(svc, catalog, deps.Logger, deps.Config.MaxPageSize)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:         Key,
		Name:        "IAM & Usuários",
		Description: "Identidades, sessões, RBAC multi-escopo (Entidades, Unidades, Departamentos) e mapeamento de grupos do AD.",
		Core:        true,
		Icon:        "users",
		Route:       "/configuracao/usuarios",
		Permissions: []kernel.PermissionInfo{
			{Key: "users:read", Description: "Consultar usuários e suas lotações"},
			{Key: "users:manage", Description: "Criar contas locais, ativar/desativar, redefinir senha"},
			{Key: "iam:manage", Description: "Estrutura organizacional, perfis, lotações e mapeamento de grupos do AD"},
			{Key: "modules:manage", Description: "Ativar e desativar módulos"},
			{Key: "keycloak:manage", Description: "Configurar o provedor de identidade (Keycloak)"},
			{Key: "ia:manage", Description: "Configurar as conexões de inteligência artificial (fornecedor, modelo, chave)"},
			{Key: "branding:manage", Description: "Editar identidade visual (white-label)"},
			{Key: "monitoring:read", Description: "Painel de monitoramento da plataforma"},
			{Key: "monitoring:manage", Description: "Reprocessar eventos do outbox que falharam"},
		},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) {
	m.handlers.RegisterRoutes(r.Authed)
}
