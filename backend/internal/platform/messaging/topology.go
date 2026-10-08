package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ExchangeEvents é o único exchange topic do barramento de eventos do
// Nexus. O Relay do Outbox publica aqui com routing key = tipo do evento.
const ExchangeEvents = "nexus.events"

// QueueSpec descreve uma fila durável com sua DLQ e seus bindings.
type QueueSpec struct {
	Name        string
	DLQName     string
	RoutingKeys []string
}

// RetryHeader carrega o número da tentativa numa republicação com atraso.
const RetryHeader = "x-nexus-attempt"

// RoutingKeyHeader preserva a routing key original (o tipo do evento) na
// cópia de retry, que volta direto para a fila de origem pelo exchange
// padrão (routing key = nome da fila).
const RoutingKeyHeader = "x-nexus-routing-key"

// QueueNotificationWebsocket alimenta o Hub de WebSocket (processo API)
// com eventos SEGUROS para difusão geral — nunca eventos que carreguem
// metadados sigilosos (ex.: Trâmite restrito/sigiloso).
var QueueNotificationWebsocket = QueueSpec{
	Name:    "nexus.notification.websocket",
	DLQName: "nexus.notification.dlq",
	RoutingKeys: []string{
		"notification.created",
		"blog.post.published",
		"catalog.service.published",
		"calendar.event.created",
		"example.item.created",
	},
}

// PlatformQueues são as filas do próprio núcleo (as dos plugins vêm do
// Kernel — kernel.Queues()).
func PlatformQueues() []QueueSpec {
	return []QueueSpec{QueueNotificationWebsocket}
}

// DeclareTopology declara exchange, filas, DLQs e bindings (idempotente).
func DeclareTopology(ch *amqp.Channel, specs []QueueSpec) error {
	if err := ch.ExchangeDeclare(ExchangeEvents, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("messaging: declare exchange %s: %w", ExchangeEvents, err)
	}
	for _, spec := range specs {
		if err := declareQueue(ch, spec); err != nil {
			return err
		}
	}
	return nil
}

func declareQueue(ch *amqp.Channel, spec QueueSpec) error {
	if _, err := ch.QueueDeclare(spec.DLQName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("messaging: declare DLQ %s: %w", spec.DLQName, err)
	}
	mainArgs := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": spec.DLQName,
	}
	if _, err := ch.QueueDeclare(spec.Name, true, false, false, false, mainArgs); err != nil {
		return fmt.Errorf("messaging: declare queue %s: %w", spec.Name, err)
	}
	for _, key := range spec.RoutingKeys {
		if err := ch.QueueBind(spec.Name, key, ExchangeEvents, false, nil); err != nil {
			return fmt.Errorf("messaging: bind queue %s to key %s: %w", spec.Name, key, err)
		}
	}
	return nil
}
