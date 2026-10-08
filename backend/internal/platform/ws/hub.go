// Package ws implementa o servidor WebSocket do Nexus.
//
// Modelo de tópicos: cada cliente assina tópicos ("mercurio:room:<id>",
// "user:<id>", "broadcast"). Toda publicação passa pelo backplane Redis
// (PUBLISH em "nexus:ws"), e cada réplica da API entrega aos SEUS clientes
// locais assinantes daquele tópico — o chat e as notificações funcionam
// com N réplicas atrás do balanceador.
//
// Tópicos de plugin ("<modulo>:...") só podem ser assinados se o módulo
// estiver ativo e o autorizador que ele registrou permitir; quando o
// Kernel desativa o módulo, DropModule derruba todas as assinaturas dele
// na hora (o cliente recebe {"type":"module.disabled"}).
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/yurythx/projeto-nexus/internal/platform/metrics"
	"github.com/yurythx/projeto-nexus/internal/platform/redisx"
)

// TopicBroadcast é assinado automaticamente por todo cliente.
const TopicBroadcast = "broadcast"

// UserTopic é o tópico pessoal de um usuário (assinado automaticamente).
func UserTopic(userID string) string { return "user:" + userID }

// ErrTopicForbidden é devolvido quando o autorizador recusa a assinatura.
var ErrTopicForbidden = errors.New("ws: assinatura de tópico não autorizada")

// Authorizer decide se o cliente pode assinar topic. Registrado por
// plugin (prefixo = chave do módulo).
type Authorizer func(ctx context.Context, client ClientInfo, topic string) error

// InboundHandler trata um frame de cliente cujo "type" começa com
// "<modulo>." (ex.: "mercurio.typing").
type InboundHandler func(ctx context.Context, client ClientInfo, frame Frame) error

// Frame é a mensagem trocada com o navegador.
type Frame struct {
	Type  string          `json:"type"`
	Topic string          `json:"topic,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

type backplaneMsg struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
}

// ModuleGate informa se um módulo está ativo (implementado pelo Kernel).
type ModuleGate func(key string) bool

// Hub mantém os clientes locais e seus tópicos.
type Hub struct {
	logger *slog.Logger
	redis  *redis.Client
	gate   ModuleGate

	mu      sync.RWMutex
	clients map[*Client]struct{}
	topics  map[string]map[*Client]struct{}

	hmu         sync.RWMutex
	authorizers map[string]Authorizer
	inbound     map[string]InboundHandler

	// subscribe abre a assinatura do backplane (substituível em teste).
	subscribe func(ctx context.Context) (<-chan *redis.Message, func() error)
	// pongWait: sem pong nesse prazo a conexão é dada como morta;
	// pingPeriod: intervalo dos pings (sempre menor que pongWait).
	pongWait, pingPeriod time.Duration
}

// NewHub cria o Hub. rdb nil = entrega só local (uma réplica).
func NewHub(logger *slog.Logger, rdb *redis.Client) *Hub {
	h := &Hub{
		logger:      logger,
		redis:       rdb,
		gate:        func(string) bool { return true },
		clients:     make(map[*Client]struct{}),
		topics:      make(map[string]map[*Client]struct{}),
		authorizers: make(map[string]Authorizer),
		inbound:     make(map[string]InboundHandler),
		pongWait:    pongWait,
		pingPeriod:  pingPeriod,
	}
	h.subscribe = func(ctx context.Context) (<-chan *redis.Message, func() error) {
		sub := h.redis.Subscribe(ctx, redisx.Key("ws"))
		return sub.Channel(), sub.Close
	}
	return h
}

// SetModuleGate conecta o Hub ao estado de ativação do Kernel.
func (h *Hub) SetModuleGate(g ModuleGate) {
	h.hmu.Lock()
	h.gate = g
	h.hmu.Unlock()
}

// RegisterModule registra o autorizador de tópicos e o handler de frames
// de entrada de um plugin (qualquer um dos dois pode ser nil).
func (h *Hub) RegisterModule(key string, authz Authorizer, inbound InboundHandler) {
	h.hmu.Lock()
	defer h.hmu.Unlock()
	if authz != nil {
		h.authorizers[key] = authz
	}
	if inbound != nil {
		h.inbound[key] = inbound
	}
}

func topicModule(topic string) string {
	mod, _, _ := strings.Cut(topic, ":")
	return mod
}

// Run consome o backplane Redis até ctx ser cancelado e então desconecta
// todos os clientes locais.
func (h *Hub) Run(ctx context.Context) error {
	defer h.closeAll()
	if h.redis == nil {
		<-ctx.Done()
		return nil
	}
	ch, closeSub := h.subscribe(ctx)
	defer func() { _ = closeSub() }()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return fmt.Errorf("ws: backplane encerrado")
			}
			var bm backplaneMsg
			if err := json.Unmarshal([]byte(msg.Payload), &bm); err != nil {
				h.logger.Warn("ws: mensagem inválida no backplane", slog.Any("error", err))
				continue
			}
			h.deliverLocal(bm.Topic, bm.Payload)
		}
	}
}

// Publish entrega frame a todos os assinantes de topic, em todas as
// réplicas. Sem Redis (ou com falha nele), entrega só aos clientes locais.
func (h *Hub) Publish(ctx context.Context, topic, frameType string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("ws: marshal data: %w", err)
	}
	frame, _ := json.Marshal(Frame{Type: frameType, Topic: topic, Data: raw}) // Data já é JSON válido
	return h.PublishRaw(ctx, topic, frame)
}

// PublishRaw publica um frame já serializado.
func (h *Hub) PublishRaw(ctx context.Context, topic string, frame []byte) error {
	if h.redis != nil {
		msg, _ := json.Marshal(backplaneMsg{Topic: topic, Payload: frame})
		if err := h.redis.Publish(ctx, redisx.Key("ws"), msg).Err(); err == nil {
			return nil
		} else {
			h.logger.Warn("ws: backplane indisponível, entregando só localmente", slog.Any("error", err))
		}
	}
	h.deliverLocal(topic, frame)
	return nil
}

func (h *Hub) deliverLocal(topic string, frame []byte) {
	h.mu.RLock()
	subs := make([]*Client, 0, len(h.topics[topic]))
	for c := range h.topics[topic] {
		subs = append(subs, c)
	}
	h.mu.RUnlock()
	for _, c := range subs {
		c.enqueue(frame)
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	count := len(h.clients)
	h.mu.Unlock()
	metrics.WebSocketConnections.Set(float64(count))
	h.subscribeUnchecked(c, TopicBroadcast)
	if c.info.UserID != "" {
		h.subscribeUnchecked(c, UserTopic(c.info.UserID))
	}
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; !ok {
		h.mu.Unlock()
		return
	}
	delete(h.clients, c)
	for t := range c.topics {
		if subs := h.topics[t]; subs != nil {
			delete(subs, c)
			if len(subs) == 0 {
				delete(h.topics, t)
			}
		}
	}
	count := len(h.clients)
	h.mu.Unlock()
	c.close()
	metrics.WebSocketConnections.Set(float64(count))
}

func (h *Hub) subscribeUnchecked(c *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.topics[topic] == nil {
		h.topics[topic] = make(map[*Client]struct{})
	}
	h.topics[topic][c] = struct{}{}
	c.topics[topic] = struct{}{}
}

// Subscribe assina topic para c depois de conferir módulo e autorizador.
func (h *Hub) Subscribe(ctx context.Context, c *Client, topic string) error {
	if topic == "" || len(topic) > 200 {
		return ErrTopicForbidden
	}
	mod := topicModule(topic)
	if mod == "user" || mod == TopicBroadcast {
		return ErrTopicForbidden // pessoais/globais são automáticos
	}
	h.hmu.RLock()
	authz, ok := h.authorizers[mod]
	gate := h.gate
	h.hmu.RUnlock()
	if !ok || !gate(mod) {
		return ErrTopicForbidden
	}
	if err := authz(ctx, c.info, topic); err != nil {
		return ErrTopicForbidden
	}
	h.subscribeUnchecked(c, topic)
	return nil
}

// Unsubscribe remove a assinatura de topic.
func (h *Hub) Unsubscribe(c *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs := h.topics[topic]; subs != nil {
		delete(subs, c)
		if len(subs) == 0 {
			delete(h.topics, topic)
		}
	}
	delete(c.topics, topic)
}

// DropModule remove de todos os clientes locais as assinaturas dos
// tópicos do módulo e os avisa. Chamado pelo Kernel ao desativar o plugin
// (em cada réplica, via notificação de mudança de estado).
func (h *Hub) DropModule(key string) {
	prefix := key + ":"
	notice, _ := json.Marshal(Frame{Type: "module.disabled", Data: json.RawMessage(fmt.Sprintf("%q", key))})
	affected := map[*Client]struct{}{}
	h.mu.Lock()
	for topic, subs := range h.topics {
		if !strings.HasPrefix(topic, prefix) {
			continue
		}
		for c := range subs {
			delete(c.topics, topic)
			affected[c] = struct{}{}
		}
		delete(h.topics, topic)
	}
	h.mu.Unlock()
	for c := range affected {
		c.enqueue(notice)
	}
}

func (h *Hub) dispatchInbound(ctx context.Context, c *Client, f Frame) error {
	mod, _, found := strings.Cut(f.Type, ".")
	if !found {
		return fmt.Errorf("ws: tipo de frame desconhecido %q", f.Type)
	}
	h.hmu.RLock()
	handler, ok := h.inbound[mod]
	gate := h.gate
	h.hmu.RUnlock()
	if !ok || !gate(mod) {
		return fmt.Errorf("ws: módulo %q indisponível", mod)
	}
	return handler(ctx, c.info, f)
}

// ClientCount devolve o número de conexões locais.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) closeAll() {
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.clients = make(map[*Client]struct{})
	h.topics = make(map[string]map[*Client]struct{})
	h.mu.Unlock()
	for _, c := range clients {
		c.close()
	}
}

// DropTopic remove todos os assinantes locais de um tópico (ex.: sala
// arquivada) e os avisa com {"type":"topic.dropped"} — antes a assinatura
// sumia em silêncio e a tela simplesmente parava de receber. Outras
// réplicas convergem na próxima tentativa de assinatura, que o autorizador
// passa a recusar.
func (h *Hub) DropTopic(topic string) {
	notice, _ := json.Marshal(Frame{Type: "topic.dropped", Topic: topic})
	h.mu.Lock()
	affected := make([]*Client, 0, len(h.topics[topic]))
	for c := range h.topics[topic] {
		delete(c.topics, topic)
		affected = append(affected, c)
	}
	delete(h.topics, topic)
	h.mu.Unlock()
	for _, c := range affected {
		c.enqueue(notice)
	}
}
