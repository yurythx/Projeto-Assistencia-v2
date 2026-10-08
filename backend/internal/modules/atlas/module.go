// Package atlas é o plugin Atlas: catálogo de procedimentos canônicos de
// processo administrativo eletrônico (padrão SEI), Tabela de
// Temporalidade e Destinação de Documentos (TTDD/CCPAD) com vigência e
// histórico, e assistente sobre fluxos e TTDD com grounding estrito (consulta
// pública; assistente com atlas:read; gestão com atlas:manage).
package atlas

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/application"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/iaconfig"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// Key é a chave do módulo.
const Key = "atlas"

// Module é o plugin.
type Module struct {
	deps     modkit.Deps
	svc      *application.Service
	handlers *transport.Handlers
}

// New constrói o módulo. A IA do assistente vem das conexões configuradas
// em Configurações > Inteligência artificial (ou, enquanto nada foi
// configurado lá, de ATLAS_AI_*); sem nenhuma, o assistente responde com a
// síntese canônica dos procedimentos (sem modelo de linguagem).
func New(deps modkit.Deps) *Module {
	c := deps.Config.Atlas
	roteador := iaconfig.NovoRoteador(iaconfig.NewPostgresStore(deps.Pool, deps.Cipher), iaconfig.NovoCliente(),
		iaconfig.ConexaoDoAmbiente(c.AIEndpoint, c.AIAPIKey, c.AIModel, int(c.AITimeout.Seconds())), deps.Logger)
	var assistente domain.Assistente = infrastructure.NewAssistenteIA(roteador)
	svc := application.NewService(deps.Pool, infrastructure.NewRepository(), deps.Outbox, assistente, deps.Logger).
		WithStorage(deps.Storage, deps.Config.MinIO.Bucket).WithNotificacoes(deps.Notificacoes)
	return &Module{deps: deps, svc: svc, handlers: transport.NewHandlers(svc, deps.Logger, deps.Config.MaxPageSize)}
}

// Manifest implementa kernel.Plugin.
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            Key,
		Name:           "Atlas — Procedimentos e Temporalidade",
		Description:    "Procedimentos canônicos de processo (padrão SEI), Tabela de Temporalidade (TTDD) oficial e assistente sobre fluxos e TTDD.",
		DefaultEnabled: true,
		Public:         true,
		Icon:           "clipboard-list",
		Route:          "/atlas",
		Permissions: []kernel.PermissionInfo{
			{Key: string(auth.PermAtlasRead), Description: "Consultar o assistente do Atlas (fluxos e TTDD)"},
			{Key: string(auth.PermAtlasManage), Description: "Gerir os procedimentos do Atlas (cadastro, novas versões, ativação) e atualizar a TTDD"},
		},
	}
}

// RegisterRoutes implementa kernel.RouteProvider.
func (m *Module) RegisterRoutes(r kernel.Routes) {
	r.Public.Group(func(pub chi.Router) {
		pub.Use(httpserver.RateLimit(m.deps.Logger, m.deps.PublicLimiter, httpserver.ClientIPKey))
		m.handlers.RegisterPublicRoutes(pub)
	})
	m.handlers.RegisterRoutes(r.Authed, m.deps.AtlasChatLimiter)
}

// SearchProviders implementa kernel.SearchProvider.
func (m *Module) SearchProviders() []search.Provider { return []search.Provider{provider{m.svc}} }

type provider struct{ svc *application.Service }

func (provider) Module() string { return Key }

func (p provider) Search(ctx context.Context, _ auth.Identity, q string, limit int) ([]search.Result, error) {
	return p.svc.BuscaGlobal(ctx, Key, q, limit)
}
