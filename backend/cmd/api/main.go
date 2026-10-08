// Command api roda a API HTTP do Projeto Nexus: endpoints REST, notificações
// via WebSocket, health/readiness/metrics. O processamento assíncrono de
// jobs fica em cmd/worker, não aqui — os dois processos são deployados e
// escalados separadamente (§7/§20).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/yurythx/projeto-nexus/internal/app"
)

// shutdownTimeout limita quanto tempo a API espera as requisições em
// andamento terminarem durante o graceful shutdown antes de forçar a saída.
const shutdownTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		// O caminho de falha do bootstrap de dependências pode rodar antes
		// de existir um logger configurado, então este é o único lugar
		// autorizado a usar o logger padrão da stdlib diretamente.
		fmt.Fprintln(os.Stderr, "api: fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	// signal.NotifyContext cancela ctx assim que o processo recebe
	// SIGINT/SIGTERM — é o gatilho que inicia o graceful shutdown abaixo
	// (§55), em vez de o processo morrer abruptamente e derrubar
	// requisições/conexões WebSocket em andamento.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	deps, err := app.NewDependencies(ctx, "api")
	if err != nil {
		return fmt.Errorf("bootstrap dependencies: %w", err)
	}
	defer deps.Close()

	// Backplane do WebSocket, consumidor de notificações, invalidação do
	// cache do IAM, watcher de módulos e workers de plugin do processo
	// "api" — todos encerram no graceful shutdown abaixo.
	var background sync.WaitGroup
	background.Add(1)
	go func() {
		defer background.Done()
		app.RunAPIBackground(ctx, deps)
	}()

	router := app.NewRouter(deps)
	// otelhttp envolve cada requisição em um span de servidor (vira no-op
	// se a telemetria não estiver configurada — §51) e propaga o contexto
	// de trace extraído dos headers de entrada, de modo que uma requisição
	// que acaba publicando um evento carrega o mesmo trace até o worker.
	instrumentedRouter := otelhttp.NewHandler(router, "http.server")

	server := &http.Server{
		Addr:    deps.Config.HTTP.Addr(),
		Handler: instrumentedRouter,
		// Timeouts anti-DoS/Slowloris (skill §4): 2s / 5s / 10s / 120s por
		// padrão, ajustáveis por HTTP_*_TIMEOUT.
		ReadHeaderTimeout: deps.Config.HTTP.ReadHeaderTimeout,
		ReadTimeout:       deps.Config.HTTP.ReadTimeout,
		WriteTimeout:      deps.Config.HTTP.WriteTimeout,
		IdleTimeout:       deps.Config.HTTP.IdleTimeout,
	}

	serveErrCh := make(chan error, 1)
	go func() {
		deps.Logger.Info("api starting", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
			return
		}
		serveErrCh <- nil
	}()

	select {
	case <-ctx.Done():
		// Sinal de shutdown recebido: para de aceitar conexões novas e
		// deixa as requisições em andamento terminarem (drenagem) dentro
		// do shutdownTimeout, em vez de cortá-las na marra.
		deps.Logger.Info("shutdown signal received, draining in-flight requests")
	case err := <-serveErrCh:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		// Estourou o prazo de graceful shutdown — força o fechamento em
		// vez de travar o processo indefinidamente esperando requisições
		// que nunca vão terminar.
		deps.Logger.Error("graceful shutdown failed, forcing close", slog.Any("error", err))
		_ = server.Close()
	}

	// Espera o Hub de WebSocket e o consumer de notificações encerrarem
	// suas goroutines antes de considerar o processo realmente parado.
	background.Wait()
	deps.Logger.Info("api stopped")
	return nil
}
