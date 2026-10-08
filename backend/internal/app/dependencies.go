// Package app conecta toda dependência de plataforma e registra os
// plugins no Kernel. É o ÚNICO lugar que conhece todos os módulos — nenhum
// plugin importa outro; as colaborações entre plugins (ex.: Trâmite ->
// Signum) passam por portas ligadas aqui.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/branding"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/iam"
	"github.com/yurythx/projeto-nexus/internal/platform/idempotency"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/keycloakconfig"
	"github.com/yurythx/projeto-nexus/internal/platform/lgpd"
	"github.com/yurythx/projeto-nexus/internal/platform/logging"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
	"github.com/yurythx/projeto-nexus/internal/platform/metrics"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/ratelimit"
	"github.com/yurythx/projeto-nexus/internal/platform/redisx"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
	"github.com/yurythx/projeto-nexus/internal/platform/storage"
	"github.com/yurythx/projeto-nexus/internal/platform/telemetry"
	"github.com/yurythx/projeto-nexus/internal/platform/transparency"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// OutboxSource identifica este backend no campo Source de todo evento.
const OutboxSource = "projeto-nexus.platform"

// RateLimiters agrupa os limitadores distribuídos (Redis).
type RateLimiters struct {
	APIGlobal *ratelimit.RedisLimiter // grupo autenticado, por identidade
	Login     *ratelimit.RedisLimiter // login local, por IP (adaptativo)
	WSTicket  *ratelimit.RedisLimiter
	Public    *ratelimit.RedisLimiter // rotas anônimas, por IP
	Contact   *ratelimit.RedisLimiter // formulário de contato, por IP
	AtlasChat *ratelimit.RedisLimiter // assistente do Atlas, por identidade
	Lockout   *ratelimit.Lockout      // lockout progressivo de credenciais
}

// Dependencies guarda todo recurso de plataforma compartilhado.
type Dependencies struct {
	Config         *config.Config
	Logger         *slog.Logger
	DB             *pgxpool.Pool
	Redis          *redis.Client
	Verifier       *auth.Verifier
	LocalSigner    *auth.LocalSigner
	IAM            *iam.Resolver
	Messaging      *messaging.Connection
	Publisher      events.EventPublisher
	Outbox         *outbox.Writer
	OutboxStats    *outbox.Stats
	Storage        storage.Provider
	Hub            *ws.Hub
	Notificacoes   *notificacoes.Service
	Tickets        *ws.TicketStore
	Cipher         *secretcrypto.Cipher
	RateLimiters   *RateLimiters
	Idempotency    idempotency.Store
	KeycloakCfg    keycloakconfig.Store
	Branding       *branding.Store
	LGPD           *lgpd.Service
	Transparency   *transparency.Service
	TrustedProxies []*net.IPNet
	Kernel         *kernel.Kernel

	telemetryShutdown telemetry.Shutdown
	closers           []func()
}

// NewDependencies constrói e valida toda dependência de um processo
// ("api" ou "worker") e registra os plugins no Kernel. Falha rápido se
// qualquer dependência obrigatória (Postgres, Redis, RabbitMQ, discovery
// OIDC) estiver inalcançável.
func NewDependencies(ctx context.Context, component string) (*Dependencies, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("app: load config: %w", err)
	}
	return build(ctx, cfg, component)
}

// build monta as dependências a partir de uma configuração já carregada.
func build(ctx context.Context, cfg *config.Config, component string) (*Dependencies, error) {
	var err error
	serviceName := cfg.App.Name + "-" + component
	logger := logging.New(logging.Options{Level: cfg.App.LogLevel, Format: cfg.App.LogFormat, Service: serviceName, Environment: cfg.App.Env})

	d := &Dependencies{Config: cfg, Logger: logger}
	fail := func(step string, err error) (*Dependencies, error) {
		d.Close()
		return nil, fmt.Errorf("app: %s: %w", step, err)
	}

	d.telemetryShutdown = telemetry.Setup(ctx, serviceName, cfg.App.Env, cfg.OTELExporterOTLPURL, logger)
	if d.DB, err = database.New(ctx, cfg.Database); err != nil {
		return fail("connect database", err)
	}
	d.closers = append(d.closers, d.DB.Close)
	metrics.RegisterPostgresPoolMetrics(d.DB)

	if d.Redis, err = redisx.Connect(ctx, cfg.Redis.URL); err != nil {
		return fail("connect redis", err)
	}
	d.closers = append(d.closers, func() { _ = d.Redis.Close() })

	if d.LocalSigner, err = auth.NewLocalSigner(cfg.LocalAuth); err != nil {
		return fail("local auth signer", err)
	}
	if d.Verifier, err = auth.NewVerifier(ctx, cfg.Keycloak, d.LocalSigner); err != nil {
		return fail("OIDC verifier", err)
	}
	if d.Cipher, err = secretcrypto.NewFromBase64Key(cfg.Security.ConfigEncryptionKey); err != nil {
		return fail("config encryption cipher", err)
	}
	d.KeycloakCfg = keycloakconfig.NewPostgresStore(d.DB, d.Cipher)
	// A configuração do Keycloak salva pela tela de administração vale já
	// no boot (sem isso, um restart voltaria só às variáveis de ambiente).
	if saved, err := d.KeycloakCfg.Get(ctx); err != nil {
		logger.Error("app: falha ao ler configuração persistida do Keycloak — seguindo com o .env", slog.Any("error", err))
	} else if saved.Configured {
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		if err := d.Verifier.Reload(rctx, saved.ToKeycloakConfig()); err != nil {
			logger.Error("app: configuração persistida do Keycloak falhou no discovery — seguindo com o .env", slog.Any("error", err))
		}
		cancel()
	}

	if d.TrustedProxies, err = httpserver.ParseTrustedProxies(cfg.Security.TrustedProxies); err != nil {
		return fail("parse TRUSTED_PROXIES", err)
	}

	if d.Messaging, err = messaging.Connect(ctx, cfg.RabbitMQ.URL, logger); err != nil {
		return fail("connect rabbitmq", err)
	}
	d.closers = append(d.closers, func() { _ = d.Messaging.Close() })
	d.Publisher = messaging.NewPublisher(d.Messaging)

	minio, err := storage.NewMinioProviderWithPresign(cfg.MinIO.Endpoint, cfg.MinIO.PublicEndpoint,
		cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, cfg.MinIO.UseSSL, cfg.MinIO.PublicUseSSL)
	if err != nil {
		return fail("connect minio", err)
	}
	if err := minio.EnsureBucket(ctx, cfg.MinIO.Bucket); err != nil {
		logger.Warn("app: não foi possível garantir o bucket do MinIO no boot", slog.Any("error", err))
	}
	d.Storage = minio

	d.Outbox = outbox.NewWriter(OutboxSource)
	d.OutboxStats = outbox.NewStats(d.DB)
	d.Hub = ws.NewHub(logger, d.Redis)
	d.Notificacoes = notificacoes.NewService(d.DB, d.Hub, logger)
	d.Tickets = ws.NewTicketStore(d.Redis, ws.TicketTTL)
	d.IAM = iam.NewResolver(d.DB, d.Redis, logger)
	d.Idempotency = idempotency.NewPostgresStore(d.DB)
	d.Branding = branding.NewStore(d.DB)
	d.LGPD = lgpd.NewService(d.DB, logger, d.TrustedProxies)
	d.RateLimiters = &RateLimiters{
		APIGlobal: ratelimit.NewRedisLimiter(d.Redis, cfg.APIRateLimit.WindowSeconds, cfg.APIRateLimit.MaxRequests, "api_global"),
		Login:     ratelimit.NewRedisLimiter(d.Redis, cfg.LoginRateLimit.WindowSeconds, cfg.LoginRateLimit.MaxRequests, "login"),
		WSTicket:  ratelimit.NewRedisLimiter(d.Redis, 5, 5, "ws_ticket"),
		Public:    ratelimit.NewRedisLimiter(d.Redis, cfg.PublicRateLimit.WindowSeconds, cfg.PublicRateLimit.MaxRequests, "public"),
		Contact:   ratelimit.NewRedisLimiter(d.Redis, cfg.ContactRateLimit.WindowSeconds, cfg.ContactRateLimit.MaxRequests, "contact"),
		AtlasChat: ratelimit.NewRedisLimiter(d.Redis, cfg.Atlas.ChatRateLimit.WindowSeconds, cfg.Atlas.ChatRateLimit.MaxRequests, "atlas_chat"),
		// Conta: 5 falhas liberadas; depois 1min, 2min, 4min... até 24h.
		// IP: teto bem mais alto, janela de 1h e bloqueio máximo de 1h —
		// atrás de NAT o IP é o prédio inteiro (ver Lockout.WithPolicy).
		Lockout: ratelimit.NewLockout(d.Redis, 5, time.Minute, 24*time.Hour).
			WithPolicy("ip:", ratelimit.LockoutPolicy{Threshold: cfg.LoginLockoutIPThreshold, Base: time.Minute, Max: time.Hour, Window: time.Hour}),
	}

	// ---- Kernel: registro dos plugins, estado e topologia ----
	d.Kernel = kernel.New(kernel.NewPostgresStore(d.DB, logger), logger)
	registerPlugins(d)
	if err := d.Kernel.Start(ctx); err != nil {
		return fail("kernel start", err)
	}
	d.Transparency = transparency.NewService(d.DB, logger, d.moduleInfo)
	wireWebSocket(d)

	if err := declareTopology(d.Messaging, append(messaging.PlatformQueues(), d.Kernel.Queues()...)); err != nil {
		return fail("declare rabbitmq topology", err)
	}
	return d, nil
}

// declareTopology declara exchanges e filas (núcleo + plugins) num canal
// próprio, fechado ao fim.
func declareTopology(conn *messaging.Connection, queues []messaging.QueueSpec) error {
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer func() { _ = ch.Close() }()
	return messaging.DeclareTopology(ch, queues)
}

func (d *Dependencies) moduleInfo() []transparency.ModuleInfo {
	out := []transparency.ModuleInfo{}
	for _, s := range d.Kernel.Status() {
		out = append(out, transparency.ModuleInfo{Key: s.Key, Name: s.Name, Description: s.Description, Enabled: s.Enabled})
	}
	return out
}

// wireWebSocket liga o Hub ao estado do Kernel: tópicos de um plugin só
// podem ser assinados enquanto ele está ativo, e desativá-lo derruba as
// assinaturas na hora (em todas as réplicas, via NOTIFY -> Reload).
func wireWebSocket(d *Dependencies) {
	d.Hub.SetModuleGate(d.Kernel.Enabled)
	for _, p := range d.Kernel.Plugins() {
		if wp, ok := p.(kernel.WSProvider); ok {
			d.Hub.RegisterModule(p.Manifest().Key, wp.WSAuthorizer(), wp.WSInbound())
		}
	}
	d.Kernel.OnChange(func(_ context.Context, key string, enabled bool) {
		if !enabled {
			d.Hub.DropModule(key)
		}
	})
}

// Close libera tudo que NewDependencies abriu (ordem inversa).
func (d *Dependencies) Close() {
	for i := len(d.closers) - 1; i >= 0; i-- {
		d.closers[i]()
	}
	d.closers = nil
	if d.telemetryShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.telemetryShutdown(ctx)
		d.telemetryShutdown = nil
	}
}
