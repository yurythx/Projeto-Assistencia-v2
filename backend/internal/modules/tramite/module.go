// Package tramite é o plugin Trâmite: processos administrativos numerados
// com controle de sigilo, tramitação entre unidades e acionamento do
// Signum para assinatura dos documentos.
package tramite

import (
	"context"

	"github.com/yurythx/projeto-nexus/internal/modules/tramite/application"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// Key é a chave do módulo.
const Key = "tramite"

// QueueSignum recebe os eventos de conclusão/recusa do Signum.
var QueueSignum = messaging.QueueSpec{
	Name:    "nexus.tramite.signum",
	DLQName: "nexus.tramite.signum.dlq",
	RoutingKeys: []string{
		"signum.envelope.completed", "signum.envelope.refused", "signum.envelope.cancelled",
	},
}

// QueueAtlas recebe a desativação de versões de procedimento do Atlas, para
// avisar os processos em andamento que seguem a versão antiga (ADR 027).
var QueueAtlas = messaging.QueueSpec{
	Name:        "nexus.tramite.atlas",
	DLQName:     "nexus.tramite.atlas.dlq",
	RoutingKeys: []string{"atlas.workflow.deactivated"},
}

// Module é o plugin.
type Module struct {
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo; sign é a porta para o Signum (ligada em app).
func New(deps modkit.Deps, sign domain.SignaturePort) *Module {
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), sign, deps.Outbox, deps.Storage,
		deps.Config.MinIO.Bucket, deps.Config.Upload.MaxFileBytes, deps.Config.Upload.URLExpiry, deps.Logger).
		WithNotificacoes(deps.Notificacoes)
	return &Module{svc: svc, handlers: transport.NewHandlers(svc, deps.Logger, deps.Config.MaxPageSize)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Trâmite",
		Description:    "Processos administrativos numerados com controle de sigilo e assinatura via Signum.",
		DefaultEnabled: true,
		DependsOn:      []string{"signum"},
		Icon:           "folder-kanban",
		Route:          "/tramite",
		Permissions: []kernel.PermissionInfo{
			{Key: "tramite:create", Description: "Abrir processos"},
			{Key: "tramite:route", Description: "Tramitar processos entre unidades"},
			{Key: "tramite:manage", Description: "Reabrir processos e atuar em processos restritos de qualquer unidade"},
		},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) { m.handlers.RegisterRoutes(r.Authed) }

// Consumers implementa kernel.ConsumerProvider.
func (m *Module) Consumers() []kernel.Consumer {
	return []kernel.Consumer{
		{Queue: QueueSignum, Handler: m.svc.HandleSignatureEvent},
		{Queue: QueueAtlas, Handler: m.svc.HandleAtlasEvent},
	}
}

// SearchProviders implementa kernel.SearchProvider.
func (m *Module) SearchProviders() []search.Provider { return []search.Provider{provider{m.svc}} }

type provider struct{ svc *application.Service }

func (provider) Module() string { return Key }

func (p provider) Search(ctx context.Context, identity auth.Identity, q string, limit int) ([]search.Result, error) {
	items, err := p.svc.Search(ctx, identity, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]search.Result, 0, len(items))
	for _, proc := range items {
		updated := proc.UpdatedAt
		title := proc.Numero
		if proc.Sigilo == domain.SigiloPublico {
			title += " — " + proc.Assunto
		}
		out = append(out, search.Result{
			Module: Key, Type: "processo", ID: proc.ID.String(), Title: title,
			Snippet: proc.Tipo + " · " + proc.UnidadeAtual, URL: "/tramite/" + proc.ID.String(), Score: 0.6, UpdatedAt: &updated,
		})
	}
	return out, nil
}
