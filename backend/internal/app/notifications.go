package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// RunAPIBackground roda, no processo da API, tudo que vive ao lado do
// servidor HTTP: o backplane do Hub (Redis), o consumidor de notificações
// (RabbitMQ -> Hub), a invalidação do cache do IAM, o watcher de estado
// dos módulos e os workers de plugin marcados para o processo "api".
// Bloqueia até ctx acabar.
func RunAPIBackground(ctx context.Context, d *Dependencies) {
	var wg sync.WaitGroup
	run := func(name string, fn processor) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = supervised(name, d.Logger, fn)(ctx)
		}()
	}
	run("ws_hub", d.Hub.Run)
	run("iam_invalidation", d.IAM.RunInvalidationListener)
	run("kernel_watch", d.Kernel.Watch)
	run("kernel_supervisor", func(ctx context.Context) error {
		return d.Kernel.Supervise(ctx, kernel.ProcessAPI, nil, nil)
	})
	run("notification_consumer", func(ctx context.Context) error {
		c := messaging.NewConsumer(d.Messaging, messaging.QueueNotificationWebsocket.Name,
			d.Config.RabbitMQ.PrefetchCount, d.Config.RabbitMQ.MaxRetries, d.Logger)
		return c.Consume(ctx, NotificationHandler(d.Hub, d.Logger))
	})
	wg.Wait()
}

// NotificationHandler encaminha ao navegador os eventos de difusão geral
// (fila nexus.notification.websocket só recebe eventos seguros — ver
// messaging.QueueNotificationWebsocket). Nunca são difundidos: eventos de
// agenda privados e conteúdo com público-alvo ("restrito", ADR 014) — a
// difusão chega a todos, e o título não pode.
func NotificationHandler(hub *ws.Hub, logger *slog.Logger) events.MessageHandler {
	return func(ctx context.Context, event events.Event) error {
		var p struct {
			Visibility string `json:"visibility"`
			Restrito   bool   `json:"restrito"`
		}
		if err := json.Unmarshal(event.Payload, &p); err == nil && (p.Visibility == "private" || p.Restrito) {
			return nil
		}
		if err := hub.Publish(ctx, ws.TopicBroadcast, "event", event); err != nil {
			logger.Error("notification: falha ao difundir evento", slog.Any("error", err))
			return err
		}
		return nil
	}
}
