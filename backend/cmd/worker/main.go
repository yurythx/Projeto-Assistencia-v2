// Command worker roda os processadores assíncronos do Projeto Nexus:
// consumers das filas do RabbitMQ, publisher do outbox e rotinas de manutenção.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/yurythx/projeto-nexus/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "worker: fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	deps, err := app.NewDependencies(ctx, "worker")
	if err != nil {
		return fmt.Errorf("bootstrap dependencies: %w", err)
	}
	defer deps.Close()

	runner := app.NewWorker(deps)

	deps.Logger.Info("projeto nexus worker starting")

	errCh := make(chan error, 2)
	go func() { errCh <- runner.Run(ctx) }()
	go func() { errCh <- runner.RunMetricsServer(ctx) }()

	var firstErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return fmt.Errorf("worker run: %w", firstErr)
	}

	deps.Logger.Info("worker stopped", slog.String("reason", "graceful shutdown complete"))
	return nil
}
