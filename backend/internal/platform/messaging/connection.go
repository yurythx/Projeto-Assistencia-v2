// Package messaging implementa o adaptador RabbitMQ para as abstrações
// events.EventPublisher/EventConsumer da plataforma: um exchange topic
// (nexus.events), filas duráveis por módulo com roteamento de dead-letter,
// publisher confirms, ack/nack manual e retry baseado em backoff. Nada em
// internal/domain nem em internal/modules importa este pacote diretamente
// — eles dependem apenas das interfaces de events (§25), então trocar
// RabbitMQ por outro broker no futuro não tocaria a camada de domínio.
package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Connection possui uma única conexão AMQP e a redisca automaticamente com
// backoff caso ela caia, para que publishers/consumers construídos por
// cima não precisem cada um da sua própria lógica de reconexão — eles só
// chamam Channel() de novo.
type Connection struct {
	url    string
	logger *slog.Logger

	mu   sync.RWMutex
	conn *amqp.Connection

	done      chan struct{}
	closeOnce sync.Once

	// backoff da reconexão, fixado na criação (lido pelo supervisor).
	reconnectMin, reconnectMax time.Duration
}

// initialConnectBudget limita quanto tempo Connect insiste na primeira
// discagem antes de desistir. Cobre a corrida de boot comum (o broker
// sobe alguns segundos depois da API, ou o healthcheck do RabbitMQ passa
// no `ping` do nó Erlang antes do listener AMQP na 5672 estar aceitando)
// sem transformar uma RABBITMQ_URL de fato errada num processo que fica
// tentando para sempre — passado o orçamento, ainda falha o startup.
var initialConnectBudget = 45 * time.Second

// Intervalos de nova tentativa (variáveis só para os testes encurtarem).
var (
	dialBackoffMax      = 5 * time.Second
	reconnectBackoffMin = time.Second
	reconnectBackoffMax = 30 * time.Second
)

// Connect disca o RabbitMQ, tolerando um broker que ainda não terminou de
// subir (retry com backoff até initialConnectBudget), e então inicia um
// supervisor em background que redisca com backoff exponencial em
// qualquer desconexão subsequente. Uma URL de fato inválida ou um broker
// ausente após o orçamento ainda param o startup (erro retornado); uma URL
// malformada falha na hora.
func Connect(ctx context.Context, url string, logger *slog.Logger) (*Connection, error) {
	// URL malformada não melhora esperando: falha na hora.
	if _, err := amqp.ParseURI(url); err != nil {
		return nil, fmt.Errorf("messaging: RABBITMQ_URL inválida: %w", err)
	}
	dialCtx, cancel := context.WithTimeout(ctx, initialConnectBudget)
	defer cancel()

	backoff := min(time.Second, dialBackoffMax)
	maxBackoff := dialBackoffMax
	var conn *amqp.Connection
	var err error
	for attempt := 1; ; attempt++ {
		conn, err = amqp.DialConfig(url, amqp.Config{Heartbeat: 10 * time.Second})
		if err == nil {
			break
		}
		if dialCtx.Err() != nil {
			return nil, fmt.Errorf("messaging: initial connect failed after %s: %w", initialConnectBudget, err)
		}
		if logger != nil {
			logger.Warn("rabbitmq ainda indisponível na inicialização, tentando de novo",
				slog.Int("attempt", attempt), slog.Any("error", err), slog.Duration("retry_in", backoff))
		}
		select {
		case <-dialCtx.Done():
			return nil, fmt.Errorf("messaging: initial connect failed after %s: %w", initialConnectBudget, err)
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}

	c := &Connection{
		url:    url,
		logger: logger,
		conn:   conn,
		done:   make(chan struct{}),

		reconnectMin: reconnectBackoffMin,
		reconnectMax: reconnectBackoffMax,
	}

	go c.superviseReconnect()
	return c, nil
}

// superviseReconnect fica escutando o canal de fechamento da conexão AMQP
// atual; quando ela cai (crash do broker, rede instável, etc.), aciona
// reconnectWithBackoff e volta a escutar a nova conexão assim que ela for
// restabelecida. Roda para sempre até Close() fechar o canal done.
func (c *Connection) superviseReconnect() {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		// Close() também fecha a conexão, então basta esperar o fim dela.
		amqpErr := <-conn.NotifyClose(make(chan *amqp.Error, 1))
		// Sem erro pode ser o Close() proposital — ou uma conexão que caiu
		// antes do NotifyClose ser registrado (a biblioteca devolve o canal
		// já fechado). Só o done distingue os dois.
		if amqpErr == nil && c.closing() {
			return
		}
		c.logger.Error("rabbitmq connection lost, reconnecting", slog.Any("error", amqpErr))
		c.reconnectWithBackoff()
	}
}

// reconnectWithBackoff tenta redisial repetidamente, dobrando o intervalo
// de espera a cada falha (1s, 2s, 4s, ... até um teto de 30s) em vez de
// martelar o broker com tentativas imediatas enquanto ele está
// reiniciando ou a rede está instável.
func (c *Connection) reconnectWithBackoff() {
	backoff := c.reconnectMin
	maxBackoff := c.reconnectMax

	for {
		select {
		case <-c.done:
			return
		default:
		}

		conn, err := amqp.DialConfig(c.url, amqp.Config{Heartbeat: 10 * time.Second})
		if err != nil {
			c.logger.Warn("rabbitmq reconnect attempt failed", slog.Any("error", err), slog.Duration("retry_in", backoff))
			select {
			case <-c.done:
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()
		c.logger.Info("rabbitmq reconnected")
		return
	}
}

// Channel abre um novo canal AMQP na conexão atual. Quem chama deve abrir
// um canal por goroutine de publisher/consumer — canais não são seguros
// para uso concorrente por múltiplas goroutines ao mesmo tempo.
func (c *Connection) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil || conn.IsClosed() {
		return nil, fmt.Errorf("messaging: no active rabbitmq connection")
	}
	return conn.Channel()
}

// Ping é uma função de verificação (Check) de readiness, compatível com
// httpserver.Check — usada pelo endpoint /ready para reportar se o
// RabbitMQ está conectado.
func (c *Connection) Ping(ctx context.Context) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil || conn.IsClosed() {
		return fmt.Errorf("messaging: not connected")
	}
	return nil
}

// closing informa se Close já foi chamado.
func (c *Connection) closing() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Close para o supervisor de reconexão e fecha a conexão subjacente.
// Idempotente: chamadas repetidas não fazem nada.
func (c *Connection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)
		c.mu.Lock()
		defer c.mu.Unlock()
		err = c.conn.Close()
	})
	return err
}
