package kernel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// ConsumeFunc conecta um handler a uma fila e bloqueia até ctx acabar
// (implementado por messaging.Consumer — injetado para manter o Kernel
// testável sem RabbitMQ).
type ConsumeFunc func(ctx context.Context, queue string, handler events.MessageHandler) error

// unit é uma goroutine supervisionada pertencente a um módulo.
type unit struct {
	module string
	name   string
	run    func(ctx context.Context) error
}

// Supervise roda os workers de process (e, no processo worker, os
// consumidores de fila) de todos os plugins. Cada unidade só executa
// enquanto o módulo está ativo: desativar cancela o context da unidade;
// reativar a inicia de novo. Uma unidade que falha é reiniciada com
// backoff exponencial. Bloqueia até ctx acabar e todas as unidades
// terminarem.
func (k *Kernel) Supervise(ctx context.Context, process Process, consume ConsumeFunc, dedup *pgxpool.Pool) error {
	var units []unit
	for _, p := range k.Plugins() {
		key := p.Manifest().Key
		if wp, ok := p.(WorkerProvider); ok {
			for _, w := range wp.Workers() {
				if w.Process == process {
					units = append(units, unit{module: key, name: w.Name, run: w.Run})
				}
			}
		}
		if process == ProcessWorker && consume != nil {
			if cp, ok := p.(ConsumerProvider); ok {
				for _, c := range cp.Consumers() {
					c := c
					handler := c.Handler
					if dedup != nil {
						handler = Idempotent(dedup, c.Queue.Name, c.Handler)
					}
					units = append(units, unit{
						module: key,
						name:   "consumer:" + c.Queue.Name,
						run: func(ctx context.Context) error {
							return consume(ctx, c.Queue.Name, handler)
						},
					})
				}
			}
		}
	}

	var wg sync.WaitGroup
	for _, u := range units {
		wg.Add(1)
		go func(u unit) {
			defer wg.Done()
			k.superviseUnit(ctx, u)
		}(u)
	}
	k.logger.Info("kernel: supervisor iniciado", slog.String("process", string(process)), slog.Int("units", len(units)))
	// Sem unidades (ex.: nenhum plugin tem worker no processo da API),
	// wg.Wait() voltaria na hora — e o wrapper supervised() reiniciava o
	// supervisor com WARN a cada backoff, para sempre. Cada unidade só
	// termina com ctx, então esperar ctx primeiro não muda o caso normal.
	<-ctx.Done()
	wg.Wait()
	return nil
}

func (k *Kernel) superviseUnit(ctx context.Context, u unit) {
	changes := k.subscribe()
	defer k.unsubscribe(changes)
	log := k.logger.With(slog.String("module", u.module), slog.String("unit", u.name))
	backoff := k.minBackoff

	for {
		if ctx.Err() != nil {
			return
		}
		if !k.Enabled(u.module) {
			select {
			case <-ctx.Done():
				return
			case <-changes:
				continue
			}
		}

		runCtx, cancel := context.WithCancel(ctx)
		started := time.Now()
		done := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("panic: %v", r)
				}
			}()
			done <- u.run(runCtx)
		}()
		log.Info("kernel: unidade iniciada")

		stoppedByToggle := false
	watch:
		for {
			select {
			case <-ctx.Done():
				cancel()
				<-done
				return
			case <-changes:
				if !k.Enabled(u.module) {
					cancel()
					<-done
					stoppedByToggle = true
					log.Info("kernel: unidade parada — módulo desativado")
					break watch
				}
			case err := <-done:
				cancel()
				if ctx.Err() != nil {
					return
				}
				// Uma unidade que rodou bem por mais que o backoff máximo
				// antes de cair recomeça do backoff mínimo.
				if time.Since(started) > k.maxBackoff {
					backoff = k.minBackoff
				}
				log.Error("kernel: unidade terminou inesperadamente, reiniciando", slog.Any("error", err), slog.Duration("retry_in", backoff))
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				if backoff < k.maxBackoff {
					backoff = min(backoff*2, k.maxBackoff)
				}
				break watch
			}
		}
		if stoppedByToggle {
			backoff = k.minBackoff
		}
	}
}

// Idempotent embrulha handler com deduplicação por (consumidor, event_id)
// em processed_events (A08): uma reentrega do RabbitMQ de um evento já
// processado com sucesso é confirmada sem reexecutar o handler. O marcador
// só é gravado (commit) quando o handler termina sem erro.
func Idempotent(pool *pgxpool.Pool, consumer string, handler events.MessageHandler) events.MessageHandler {
	return func(ctx context.Context, event events.Event) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("kernel: dedup begin: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		var inserted bool
		err = tx.QueryRow(ctx, `
			INSERT INTO processed_events (consumer, event_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING RETURNING true`, consumer, event.ID).Scan(&inserted)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // já processado
		}
		if err != nil {
			return fmt.Errorf("kernel: dedup insert: %w", err)
		}
		if err := handler(ctx, event); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}

// CleanupProcessedEvents apaga marcadores de deduplicação antigos (a
// janela de reentrega do RabbitMQ é de minutos; 14 dias é folga ampla).
func CleanupProcessedEvents(pool *pgxpool.Pool, logger *slog.Logger) func(ctx context.Context) error {
	return cleanupProcessedEvents(pool, logger, time.Hour)
}

func cleanupProcessedEvents(db database.DBTX, logger *slog.Logger, every time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			if _, err := db.Exec(ctx, `DELETE FROM processed_events WHERE processed_at < now() - interval '14 days'`); err != nil && ctx.Err() == nil {
				logger.Warn("kernel: limpeza de processed_events falhou", slog.Any("error", err))
			}
			select {
			case <-ctx.Done():
				return nil
			case <-t.C:
			}
		}
	}
}
