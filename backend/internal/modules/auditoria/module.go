// Package auditoria é o módulo núcleo de Auditoria (Core imutável — nunca
// desativável): consulta da trilha append-only, verificação da cadeia de
// integridade SHA-256 e exportação (LAI) em CSV/JSON/XML. A escrita da
// trilha é feita por toda a plataforma via internal/platform/audit.
package auditoria

import (
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

// Key é a chave do módulo no Kernel.
const Key = "audit"

// Module é o plugin de Auditoria.
type Module struct {
	handlers *audit.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	return &Module{handlers: audit.NewHandlers(
		audit.NewReader(deps.Pool),
		audit.NewExporter(deps.Pool, deps.Logger),
		deps.Logger,
		deps.Config.MaxPageSize,
	)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:         Key,
		Name:        "Auditoria",
		Description: "Registro transacional e imutável de operações com encadeamento de hash SHA-256.",
		Core:        true,
		Icon:        "shield-check",
		Route:       "/auditoria",
		Permissions: []kernel.PermissionInfo{
			{Key: "audit:read", Description: "Consultar e exportar a trilha de auditoria"},
			{Key: "audit:verify", Description: "Verificar a integridade da cadeia SHA-256"},
		},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) {
	audit.RegisterRoutes(r.Authed, m.handlers)
}
