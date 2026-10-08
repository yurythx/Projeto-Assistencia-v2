package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/branding"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/iaconfig"
	"github.com/yurythx/projeto-nexus/internal/platform/idempotency"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/keycloakconfig"
	"github.com/yurythx/projeto-nexus/internal/platform/lgpd"
	"github.com/yurythx/projeto-nexus/internal/platform/localauth"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/redisx"
	"github.com/yurythx/projeto-nexus/internal/platform/transparency"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// NewRouter monta o router HTTP da API: base de plataforma (request id,
// IP real, headers de segurança, CORS, métricas), readiness, WebSocket e
// a API versionada /api/v1 — rotas públicas e autenticadas do núcleo e,
// via Kernel, de cada plugin (protegidas pelo guard de ativação).
func NewRouter(d *Dependencies) chi.Router {
	r := httpserver.New(httpserver.Options{
		Logger:         d.Logger,
		AllowedOrigins: []string{d.Config.FrontendURL},
		// Abaixo do WriteTimeout (10s) do servidor: o handler recebe o
		// cancelamento antes de a conexão ser cortada.
		RequestTimeout: 9 * time.Second,
		MetricsToken:   d.Config.Security.MetricsToken,
		TrustedProxies: d.TrustedProxies,
	})

	readiness := httpserver.ReadyHandler([]httpserver.Check{
		{Name: "postgres", Fn: database.Ping(d.DB)},
		{Name: "redis", Fn: redisx.Ping(d.Redis)},
		{Name: "rabbitmq", Fn: d.Messaging.Ping},
		{Name: "minio", Fn: d.Storage.Ping},
	}, 3*time.Second)
	r.Get("/ready", readiness)
	r.Get("/readyz", readiness)

	// WebSocket autenticado por ticket de uso único (Redis).
	r.Get("/ws", ws.UpgradeHandler(d.Hub, d.Tickets, d.Config.FrontendURL, d.Logger))

	// Documentação OpenAPI (Swagger UI vendorado, sem CDN).
	r.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "docs/openapi.json") })
	r.Handle("/docs/assets/*", http.StripPrefix("/docs/assets/", http.FileServer(http.Dir("docs/swagger-ui"))))
	r.Get("/docs", swaggerUI)

	localAuth := localauth.NewHandlers(localauth.NewPostgresStore(d.DB), d.LocalSigner, audit.NewWriter(d.DB),
		d.RateLimiters.Lockout, d.RateLimiters.Login, d.Logger)
	kernelHandlers := kernel.NewHandlers(d.Kernel, d.Logger)
	brandingHandlers := branding.NewHandlers(d.Branding, d.Logger)
	keycloakHandlers := keycloakconfig.NewHandlers(d.KeycloakCfg, d.Verifier, d.Config.Keycloak, audit.NewWriter(d.DB), d.Logger)
	atlasIA := d.Config.Atlas
	iaHandlers := iaconfig.NewHandlers(iaconfig.NewPostgresStore(d.DB, d.Cipher), iaconfig.NovoCliente(),
		iaconfig.ConexaoDoAmbiente(atlasIA.AIEndpoint, atlasIA.AIAPIKey, atlasIA.AIModel, int(atlasIA.AITimeout.Seconds())),
		audit.NewWriter(d.DB), d.Logger)
	outboxStats := outbox.NewStatsHandlers(d.OutboxStats, d.Logger)

	r.Route("/api/v1", func(api chi.Router) {
		// IP real + user agent no context: toda auditoria gravada pelos
		// casos de uso (audit.Meta) carrega a proveniência completa.
		api.Use(audit.CaptureOrigin)

		// ---------------- grupo PÚBLICO (visitante anônimo) ----------------
		api.Group(func(pub chi.Router) {
			pub.Use(httpserver.RateLimit(d.Logger, d.RateLimiters.Public, httpserver.ClientIPKey))
			kernelHandlers.RegisterAnonymousRoutes(pub)
			brandingHandlers.RegisterPublicRoutes(pub)
			// O limiter do grupo já cobre estas rotas (evita contar duas vezes).
			lgpd.RegisterPublicRoutes(pub, d.LGPD, unlimited{})
			transparency.RegisterRoutes(pub, d.Transparency, unlimited{})
		})
		// Login local fora do limiter público: tem o próprio (adaptativo).
		localauth.RegisterRoutes(api, localAuth, d.Logger, d.RateLimiters.Login)

		// ---------------- grupo AUTENTICADO ----------------
		api.Group(func(authed chi.Router) {
			authed.Use(auth.RequireAuthentication(d.Verifier, d.IAM.Enrich, d.Logger))
			authed.Use(httpserver.RateLimit(d.Logger, d.RateLimiters.APIGlobal, identityRateLimitKey))
			authed.Use(idempotency.Middleware(d.Idempotency, d.Logger))

			authed.With(httpserver.RateLimit(d.Logger, d.RateLimiters.WSTicket, identityRateLimitKey)).
				Post("/ws/ticket", ws.TicketHandler(d.Tickets, d.Logger))
			localauth.RegisterAuthedRoutes(authed, localAuth)
			kernelHandlers.RegisterRoutes(authed)
			brandingHandlers.RegisterAdminRoutes(authed)
			keycloakconfig.RegisterRoutes(authed, keycloakHandlers, d.Logger)
			iaconfig.RegisterRoutes(authed, iaHandlers, d.Logger)
			notificacoes.RegisterRoutes(authed, notificacoes.NewHandlers(d.Notificacoes, d.Logger))
			outbox.RegisterStatsRoutes(authed, outboxStats, d.Logger)
			d.LGPD.RegisterRoutes(authed)
			d.LGPD.RegisterDSRRoutes(authed)

			// Plugins: cada um recebe um ponto de montagem público e um
			// autenticado, ambos atrás do guard de ativação do Kernel.
			api.Group(func(pluginPublic chi.Router) {
				d.Kernel.MountRoutes(pluginPublic, authed, d.Logger)
			})
		})
	})
	return r
}

// unlimited é um Limiter que nunca barra (rotas já limitadas pelo grupo).
type unlimited struct{}

func (unlimited) Allow(context.Context, string) (bool, error) { return true, nil }

// identityRateLimitKey limita por identidade autenticada (fallback: IP).
func identityRateLimitKey(r *http.Request) string {
	if identity, ok := auth.IdentityFromContext(r.Context()); ok && identity.Subject != "" {
		return "sub:" + identity.Subject
	}
	return "ip:" + httpserver.ClientIPKey(r)
}

func swaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'")
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Projeto Nexus — API (OpenAPI 3)</title>
  <link rel="stylesheet" href="/docs/assets/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="/docs/assets/swagger-ui-bundle.js"></script>
  <script src="/docs/assets/swagger-initializer.js"></script>
</body>
</html>`))
}
