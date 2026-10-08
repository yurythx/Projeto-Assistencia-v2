package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/metrics"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMessage = 8192
	sendBuffer = 64
)

// ClientInfo é a identidade do dono da conexão (resgatada do ticket).
type ClientInfo struct {
	UserID      string       `json:"user_id"`
	Subject     string       `json:"subject"`
	Username    string       `json:"username"`
	Roles       []string     `json:"roles"`
	Groups      []string     `json:"groups"`
	Permissions []string     `json:"permissions"`
	Scopes      []auth.Scope `json:"scopes"`
}

// Client é uma conexão WebSocket local.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	info   ClientInfo
	logger *slog.Logger

	topics    map[string]struct{} // protegido por hub.mu
	closeOnce sync.Once
	done      chan struct{}
}

func newClient(hub *Hub, conn *websocket.Conn, info ClientInfo, logger *slog.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, sendBuffer),
		info:   info,
		logger: logger,
		topics: make(map[string]struct{}),
		done:   make(chan struct{}),
	}
}

// enqueue entrega frame sem bloquear; cliente lento é desconectado.
func (c *Client) enqueue(frame []byte) {
	select {
	case <-c.done:
	case c.send <- frame:
	default:
		c.logger.Warn("ws: cliente lento desconectado", slog.String("user_id", c.info.UserID))
		metrics.WebSocketErrorsTotal.Inc()
		go c.hub.unregister(c)
	}
}

func (c *Client) close() {
	c.closeOnce.Do(func() { close(c.done) })
}

func (c *Client) run(ctx context.Context) {
	c.hub.register(c)
	go c.writePump()
	c.readPump(ctx)
}

func (c *Client) readPump(ctx context.Context) {
	defer c.hub.unregister(c)
	c.conn.SetReadLimit(maxMessage)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.hub.pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.hub.pongWait))
	})
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var f Frame
		if err := json.Unmarshal(raw, &f); err != nil {
			c.reply("error", "", map[string]string{"message": "frame inválido"})
			continue
		}
		switch f.Type {
		case "ping":
			c.reply("pong", "", nil)
		case "subscribe":
			if err := c.hub.Subscribe(ctx, c, f.Topic); err != nil {
				c.reply("error", f.Topic, map[string]string{"message": "assinatura negada"})
				continue
			}
			c.reply("subscribed", f.Topic, nil)
		case "unsubscribe":
			c.hub.Unsubscribe(c, f.Topic)
			c.reply("unsubscribed", f.Topic, nil)
		default:
			if err := c.hub.dispatchInbound(ctx, c, f); err != nil {
				c.reply("error", f.Topic, map[string]string{"message": err.Error()})
			}
		}
	}
}

func (c *Client) reply(frameType, topic string, data any) {
	var raw json.RawMessage
	if data != nil {
		raw, _ = json.Marshal(data)
	}
	b, _ := json.Marshal(Frame{Type: frameType, Topic: topic, Data: raw})
	c.enqueue(b)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(c.hub.pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case <-c.done:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, ""))
			return
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.hub.unregister(c)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.hub.unregister(c)
				return
			}
		}
	}
}

// Identity reconstrói a identidade efetiva do dono da conexão, para que
// os autorizadores de tópico dos plugins reutilizem as mesmas regras de
// acesso da API HTTP.
func (c ClientInfo) Identity() auth.Identity {
	uid, _ := uuid.Parse(c.UserID)
	return auth.Identity{
		Subject: c.Subject, Username: c.Username, Roles: c.Roles, Groups: c.Groups,
		Permissions: c.Permissions, Scopes: c.Scopes, UserID: uid,
	}
}
