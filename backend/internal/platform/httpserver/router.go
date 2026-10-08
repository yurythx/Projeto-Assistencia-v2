// Package httpserver fornece o router chi base, os middlewares
// compartilhados (request id, recovery, CORS, security headers, limite de
// tamanho, rate limiting) e os endpoints /health, /ready, /metrics. As
// rotas de negócio são montadas sobre o router retornado por New pelo
// internal/app/router.go — este pacote não carrega nenhuma regra de
// negócio, só a infraestrutura HTTP comum a toda a aplicação.
package httpserver

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Options configura o router base.
type Options struct {
	Logger         *slog.Logger
	AllowedOrigins []string
	RequestTimeout time.Duration
	// MetricsToken, quando não vazio, exige "Authorization: Bearer
	// <token>" no /metrics (gap G-02). Vazio = endpoint aberto.
	MetricsToken string
	// TrustedProxies: CIDRs de proxies reversos confiáveis (ver
	// TrustedRealIP).
	TrustedProxies []*net.IPNet
}

// New constrói um chi.Router com a pilha padrão de middlewares da
// plataforma já montada, mais /health e /metrics. Não inclui nenhuma rota
// de autenticação ou de negócio — essas são adicionadas por cima
// (internal/app/router.go), incluindo o /ready, que depende dos Checks de
// dependência específicos daquele processo (API ou worker).
func New(opts Options) chi.Router {
	r := chi.NewRouter()

	// A ordem dos middlewares importa: RequestID precisa vir primeiro para
	// que AccessLog e Recoverer já consigam incluir o request id nos logs
	// que emitem; Recoverer precisa vir antes de qualquer coisa que possa
	// dar panic, para transformar isso em 500 em vez de derrubar o
	// processo; SecurityHeaders/CORS valem tanto para respostas normais
	// quanto para as de erro capturadas pelo Recoverer.
	for _, origin := range opts.AllowedOrigins {
		if origin == "*" {
			panic("httpserver: AllowCredentials cannot be used with AllowedOrigins containing '*' wildcard")
		}
	}

	r.Use(TrustedRealIP(opts.TrustedProxies))
	r.Use(RequestID)
	r.Use(AccessLog(opts.Logger))
	r.Use(Recoverer(opts.Logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   opts.AllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Idempotency-Key", "Idempotency-Key", "traceparent", "tracestate"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(SecurityHeaders)
	r.Use(timeoutExceptWebSocket(opts.RequestTimeout))
	r.Use(Metrics)

	// GET e HEAD: healthcheck do Docker (wget --spider, ver
	// Dockerfile.api/docker-compose.yml) manda HEAD — chi não promove
	// GET pra HEAD automaticamente, então sem o registro explícito o
	// container nunca fica "healthy" (405 Method Not Allowed).
	//
	// /livez e /healthz são aliases dos nomes canônicos de sonda
	// (Kubernetes) — gap G-14: facilita padronizar manifests entre órgãos
	// sem mudar o comportamento das rotas existentes.
	liveness := HealthHandler()
	r.Get("/health", liveness)
	r.Head("/health", liveness)
	r.Get("/livez", liveness)
	r.Head("/livez", liveness)
	r.Get("/healthz", liveness)
	r.Head("/healthz", liveness)

	// /metrics: protegido por bearer de scrape quando MetricsToken está
	// configurado (gap G-02); aberto caso contrário.
	if opts.MetricsToken == "" {
		r.Handle("/metrics", promhttp.Handler())
	} else {
		r.With(requireMetricsToken(opts.MetricsToken)).Handle("/metrics", promhttp.Handler())
	}

	return r
}
