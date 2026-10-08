package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/idempotency"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// Worker roda os processadores de segundo plano: os do núcleo (Relay do
// Outbox, limpezas, LGPD, cópia WORM da auditoria) e — supervisionados
// pelo Kernel — os workers e consumidores de fila dos plugins ATIVOS.
type Worker struct {
	deps       *Dependencies
	processors []processor
}

type processor func(ctx context.Context) error

// NewWorker constrói o runner do processo worker.
func NewWorker(d *Dependencies) *Worker {
	relay := outbox.NewPublisher(d.DB, d.Publisher, d.Logger)
	return &Worker{
		deps: d,
		processors: []processor{
			// Relay Worker do Transactional Outbox -> RabbitMQ.
			supervised("outbox_relay", d.Logger, relay.Run),
			supervised("idempotency_cleanup", d.Logger, idempotency.Cleanup(d.DB)),
			supervised("processed_events_cleanup", d.Logger, kernel.CleanupProcessedEvents(d.DB, d.Logger)),
			// LGPD art. 18, VI — anonimização a pedido do titular.
			supervised("lgpd_erasure", d.Logger, d.LGPD.ErasureProcessor()),
			// Cópia WORM diária da trilha (com os hashes da cadeia).
			supervised("audit_worm_export", d.Logger, audit.WORMExporter(d.DB, d.Storage, d.Config.AuditWORM.Bucket, d.Config.AuditWORM.RetentionDays, d.Logger)),
			// Estado dos módulos: LISTEN/NOTIFY + polling.
			supervised("kernel_watch", d.Logger, d.Kernel.Watch),
			// Workers e consumidores dos plugins (param ao desativar).
			supervised("kernel_supervisor", d.Logger, func(ctx context.Context) error {
				return d.Kernel.Supervise(ctx, kernel.ProcessWorker, d.consume, d.DB)
			}),
		},
	}
}

// consume liga uma fila a um handler (usado pelo supervisor do Kernel).
func (d *Dependencies) consume(ctx context.Context, queue string, handler events.MessageHandler) error {
	c := messaging.NewConsumer(d.Messaging, queue, d.Config.RabbitMQ.PrefetchCount, d.Config.RabbitMQ.MaxRetries, d.Logger)
	return c.Consume(ctx, handler)
}

// Backoff do supervised (variáveis só para os testes encurtarem; lidas na
// criação do processor).
var (
	supervisedMinBackoff = time.Second
	supervisedMaxBackoff = 30 * time.Second
)

// supervised reinicia fn com backoff exponencial até ctx acabar.
func supervised(name string, logger *slog.Logger, fn processor) processor {
	minBackoff, maxBackoff := supervisedMinBackoff, supervisedMaxBackoff
	return func(ctx context.Context) error {
		backoff := minBackoff
		for {
			err := fn(ctx)
			if ctx.Err() != nil {
				return nil
			}
			if err != nil {
				logger.Error("processor exited unexpectedly, restarting", slog.String("processor", name), slog.Any("error", err), slog.Duration("retry_in", backoff))
			} else {
				logger.Warn("processor returned before shutdown, restarting", slog.String("processor", name))
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
		}
	}
}

// Run executa todos os processadores até ctx ser cancelado.
func (w *Worker) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, p := range w.processors {
		wg.Add(1)
		go func(p processor) {
			defer wg.Done()
			_ = p(ctx)
		}(p)
	}
	<-ctx.Done()
	wg.Wait()
	return nil
}

// RunMetricsServer expõe /health e /metrics do worker.
func (w *Worker) RunMetricsServer(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/health", httpserver.HealthHandler())
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:              w.deps.Config.Worker.MetricsAddr(),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		w.deps.Logger.Info("worker metrics listener starting", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := server.Shutdown(shutdownCtx)
	<-errCh // Shutdown fecha o listener: o ListenAndServe já voltou
	return err
}
