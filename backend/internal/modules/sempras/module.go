// Package sempras é o plugin SEMPRAS: atendimento socioassistencial das
// unidades da Secretaria de Promoção e Assistência Social (ADR 030) —
// busca e cadastro do munícipe, fila por unidade em tempo real,
// atendimento técnico/psicológico e prontuário com leitura auditada. As
// unidades são dados (Unidades do IAM), não módulos: o escopo de cada
// pessoa vem das lotações e dos grupos do AD (ADR 013).
package sempras

import (
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/application"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/transport"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/kernel"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/modkit"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/ws"
)

// Key é a chave do módulo (também o prefixo dos tópicos WebSocket).
const Key = "sempras"

// Module é o plugin.
type Module struct {
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), deps.Outbox, deps.Hub, deps.Logger)
	return &Module{svc: svc, handlers: transport.NewHandlers(svc, deps.Logger)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "SEMPRAS",
		Description:    "Atendimento socioassistencial: busca e cadastro do munícipe, fila por unidade em tempo real e prontuário.",
		DefaultEnabled: false,
		Icon:           "hand-heart",
		Route:          "/sempras",
		Permissions: []kernel.PermissionInfo{
			{Key: "sempras:recepcao", Description: "Buscar e cadastrar munícipes e incluí-los na fila da unidade"},
			{Key: "sempras:atendimento", Description: "Chamar da fila, atender e registrar no prontuário (atendimento social)"},
			{Key: "sempras:psicologia", Description: "Atendimento psicológico e registros sigilosos da psicologia"},
			{Key: "sempras:gestao", Description: "Fila e painel de todas as unidades cobertas; cancelar entradas"},
		},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) { m.handlers.RegisterRoutes(r.Authed) }

// WSAuthorizer implementa kernel.WSProvider.
func (m *Module) WSAuthorizer() ws.Authorizer { return m.svc.Authorize }

// WSInbound implementa kernel.WSProvider.
func (m *Module) WSInbound() ws.InboundHandler { return m.svc.Inbound }
