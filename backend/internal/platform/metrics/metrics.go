// Package metrics centraliza toda métrica Prometheus que a plataforma
// expõe em /metrics (§53). Defini-las todas aqui — em vez de espalhar
// chamadas a promauto por todo pacote — permite enxergar a superfície
// completa de métricas da plataforma num único arquivo e evita erros de
// registro duplicado; cada var é registrada no registry padrão exatamente
// uma vez, na inicialização do pacote.
//
// Os textos de "Help" de cada métrica ficam em inglês deliberadamente —
// são o contrato consumido por Prometheus/Grafana/alertas, não texto de
// UI, e seguem a convenção do ecossistema (o mesmo padrão usado pelas
// métricas nativas do próprio Prometheus e de toda métrica de terceiros
// que este projeto pode vir a agregar num dashboard).
package metrics

import (
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// --- HTTP ---
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_http_requests_total",
		Help: "Total HTTP requests handled, by method, route and status code.",
	}, []string{"method", "route", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nexus_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds, by method and route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	// --- RabbitMQ ---
	RabbitMQPublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_rabbitmq_published_total",
		Help: "Messages successfully published (broker-confirmed), by routing key.",
	}, []string{"routing_key"})

	RabbitMQConsumedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_rabbitmq_consumed_total",
		Help: "Messages successfully consumed (acked), by queue.",
	}, []string{"queue"})

	RabbitMQFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_rabbitmq_failed_total",
		Help: "Message handler failures, by queue.",
	}, []string{"queue"})

	RabbitMQRetryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_rabbitmq_retry_total",
		Help: "Message redelivery attempts scheduled, by queue.",
	}, []string{"queue"})

	RabbitMQDLQTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_rabbitmq_dlq_total",
		Help: "Messages routed to a dead-letter queue after exhausting retries, by queue.",
	}, []string{"queue"})

	// --- WebSocket ---
	WebSocketConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "nexus_websocket_connections",
		Help: "Currently connected WebSocket clients.",
	})

	WebSocketErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "nexus_websocket_errors_total",
		Help: "WebSocket upgrade failures and dropped (slow/stuck) clients.",
	})

	// --- Idempotency (internal/platform/idempotency) ---
	IdempotencyOutcomesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nexus_idempotency_outcomes_total",
		Help: "Outcomes of idempotency-key-guarded requests, by outcome (new/replayed/conflict/reused_key).",
	}, []string{"outcome"})
)

// poolStats é o pool observado pelos gauges nexus_postgres_connections.
var (
	poolMu       sync.RWMutex
	observedPool *pgxpool.Pool
	registerOnce sync.Once
)

// RegisterPostgresPoolMetrics expõe os gauges
// nexus_postgres_connections{state=acquired|idle|max} com as estatísticas
// ao vivo do pool (§53). Os valores são calculados no scrape (GaugeFunc),
// sem goroutine própria. Os gauges são registrados uma única vez; chamar
// de novo (ex.: um segundo NewDependencies no mesmo processo, nos testes)
// só troca o pool observado, em vez de entrar em pânico com o registro
// duplicado.
func RegisterPostgresPoolMetrics(pool *pgxpool.Pool) {
	poolMu.Lock()
	observedPool = pool
	poolMu.Unlock()
	registerOnce.Do(func() {
		for state, value := range map[string]func(*pgxpool.Stat) float64{
			"acquired": func(s *pgxpool.Stat) float64 { return float64(s.AcquiredConns()) },
			"idle":     func(s *pgxpool.Stat) float64 { return float64(s.IdleConns()) },
			"max":      func(s *pgxpool.Stat) float64 { return float64(s.MaxConns()) },
		} {
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Name:        "nexus_postgres_connections",
				Help:        "PostgreSQL pool connections, by state (acquired/idle/max).",
				ConstLabels: prometheus.Labels{"state": state},
			}, func() float64 {
				poolMu.RLock()
				defer poolMu.RUnlock()
				return value(observedPool.Stat())
			})
		}
	})
}
