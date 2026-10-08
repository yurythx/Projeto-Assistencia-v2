package messaging

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// waiter é a confirmação de uma publicação (*amqp.DeferredConfirmation).
type waiter interface {
	WaitContext(ctx context.Context) (bool, error)
}

// confirmer é o canal usado para publicar com publisher confirms.
type confirmer interface {
	Confirm(noWait bool) error
	publish(ctx context.Context, exchange, key string, msg amqp.Publishing) (waiter, error)
}

// amqpConfirmer adapta *amqp.Channel a confirmer.
type amqpConfirmer struct{ *amqp.Channel }

func (c amqpConfirmer) publish(ctx context.Context, exchange, key string, msg amqp.Publishing) (waiter, error) {
	d, err := c.PublishWithDeferredConfirmWithContext(ctx, exchange, key, false, false, msg)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// publishConfirmed liga as confirmações do canal, publica e espera o broker
// confirmar — o único caminho de publicação do pacote (Publisher e o retry
// do Consumer). Um nack do broker é erro: a mensagem não foi aceita.
func publishConfirmed(ctx context.Context, ch confirmer, exchange, key string, msg amqp.Publishing) error {
	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("enable publisher confirms: %w", err)
	}
	confirmation, err := ch.publish(ctx, exchange, key, msg)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	ok, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for publisher confirm: %w", err)
	}
	if !ok {
		return fmt.Errorf("broker nacked the message")
	}
	return nil
}
