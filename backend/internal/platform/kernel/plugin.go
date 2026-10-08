// Package kernel é o Core Kernel do Microkernel do Nexus.
//
// Todo módulo de negócio é um Plugin registrado por RegisterModule. O
// Kernel é dono do ciclo de vida: ativa e desativa plugins em tempo de
// execução (sem recompilar nem reiniciar), a partir da tabela
// system_modules. Um plugin inativo:
//   - responde 404 em toda a sua superfície HTTP (o handler nunca roda);
//   - tem seus workers e consumidores de fila parados (as mensagens ficam
//     retidas na fila durável até a reativação);
//   - perde as assinaturas de WebSocket dos seus tópicos na hora;
//   - some da Busca Global.
//
// Plugins nunca importam uns aos outros: dependências entre eles são
// declaradas no Manifest (DependsOn) e a colaboração passa por portas
// definidas pelo consumidor e conectadas em internal/app.
package kernel

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// PermissionInfo documenta uma permissão declarada pelo plugin (exibida
// na tela de Perfis para montar perfis sem decorar strings).
type PermissionInfo struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// Manifest descreve um plugin.
type Manifest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Core: módulo do núcleo imutável (IAM, Auditoria) — nunca desativável.
	Core bool `json:"core"`
	// DefaultEnabled: estado inicial na primeira vez que o módulo aparece.
	DefaultEnabled bool             `json:"default_enabled"`
	DependsOn      []string         `json:"depends_on"`
	Permissions    []PermissionInfo `json:"permissions"`
	// Public: o plugin expõe rotas anônimas (site público) — o estado dele
	// é informado em GET /api/v1/system/public-modules.
	Public bool `json:"public"`
	// Icon: dica de ícone para o frontend (nome lucide).
	Icon string `json:"icon"`
	// Route: rota principal no frontend (ex.: "/mercurio").
	Route string `json:"route"`
}

// Plugin é o contrato mínimo de um módulo.
type Plugin interface {
	Manifest() Manifest
}

// Routes são os pontos de montagem HTTP entregues a um plugin, ambos
// relativos a /api/v1 e já protegidos pelo guard de ativação do módulo.
type Routes struct {
	// Public: sem autenticação, com rate limit por IP.
	Public chi.Router
	// Authed: exige token válido; identidade e permissões já resolvidas.
	Authed chi.Router
}

// RouteProvider é implementado por plugins com API HTTP.
type RouteProvider interface {
	RegisterRoutes(r Routes)
}

// Process identifica em qual processo um worker roda.
type Process string

const (
	ProcessAPI    Process = "api"
	ProcessWorker Process = "worker"
)

// Worker é uma goroutine de segundo plano do plugin, controlada
// estritamente por context.Context: Run precisa retornar assim que ctx for
// cancelado (desativação do módulo ou shutdown).
type Worker struct {
	Name    string
	Process Process
	Run     func(ctx context.Context) error
}

// WorkerProvider é implementado por plugins com processamento assíncrono.
type WorkerProvider interface {
	Workers() []Worker
}

// Consumer liga uma fila do RabbitMQ a um handler. Todo consumidor roda
// no processo worker, com deduplicação por event_id (processed_events).
type Consumer struct {
	Queue   messaging.QueueSpec
	Handler events.MessageHandler
}

// ConsumerProvider é implementado por plugins que consomem eventos.
type ConsumerProvider interface {
	Consumers() []Consumer
}

// SearchProvider é implementado por plugins com conteúdo pesquisável.
type SearchProvider interface {
	SearchProviders() []search.Provider
}

// WSProvider é implementado por plugins com tópicos de WebSocket
// ("<chave>:...") e/ou frames de entrada ("<chave>.<tipo>").
type WSProvider interface {
	WSAuthorizer() ws.Authorizer
	WSInbound() ws.InboundHandler
}

// Lifecycle recebe os eventos de ativação/desativação em runtime (em
// todas as réplicas, via NOTIFY do Postgres).
type Lifecycle interface {
	OnEnable(ctx context.Context) error
	OnDisable(ctx context.Context) error
}
