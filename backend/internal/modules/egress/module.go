// Package egress é o plugin Egress: barramento de eventos assíncronos e
// webhooks de saída com proteção anti-SSRF (n8n, Zabbix, Grafana).
package egress

import (
	"github.com/yurythx/projeto-nexus/internal/modules/egress/application"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/netguard"
)

// Key é a chave do módulo.
const Key = "egress"

// QueueDispatcher recebe TODO evento de domínio do barramento.
var QueueDispatcher = messaging.QueueSpec{
	Name:        "nexus.egress.dispatcher",
	DLQName:     "nexus.egress.dispatcher.dlq",
	RoutingKeys: []string{"#"},
}

// Module é o plugin.
type Module struct {
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo.
func New(deps modkit.Deps) *Module {
	cfg := deps.Config.Egress
	policy := netguard.Policy{AllowPrivate: cfg.AllowPrivateNetworks, AllowHTTP: cfg.AllowPrivateNetworks}
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), infrastructure.NewDeliverer(cfg.Timeout, policy),
		deps.Cipher, application.Config{MaxAttempts: cfg.MaxAttempts, BatchSize: cfg.BatchSize, PollInterval: cfg.PollInterval}, deps.Logger)
	return &Module{svc: svc, handlers: transport.NewHandlers(svc, deps.Logger, deps.Config.MaxPageSize)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Egress",
		Description:    "Webhooks de saída assíncronos e assinados, com proteção anti-SSRF (n8n, Zabbix, Grafana).",
		DefaultEnabled: true,
		Icon:           "send",
		Route:          "/configuracao/egress",
		Permissions:    []kernel.PermissionInfo{{Key: "egress:manage", Description: "Gerenciar destinos de webhook e entregas"}},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) { m.handlers.RegisterRoutes(r.Authed) }

// Consumers implementa kernel.ConsumerProvider.
func (m *Module) Consumers() []kernel.Consumer {
	return []kernel.Consumer{{Queue: QueueDispatcher, Handler: m.svc.Dispatch}}
}

// Workers implementa kernel.WorkerProvider.
func (m *Module) Workers() []kernel.Worker {
	return []kernel.Worker{{Name: "delivery", Process: kernel.ProcessWorker, Run: m.svc.RunDeliveries}}
}
