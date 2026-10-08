package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type harness struct {
	hub     *Hub
	tickets *TicketStore
	srv     *httptest.Server
	cancel  context.CancelFunc
	enabled atomic.Bool
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	h := &harness{hub: NewHub(testLogger(), rdb), tickets: NewTicketStore(rdb, TicketTTL)}
	h.enabled.Store(true)
	h.hub.SetModuleGate(func(key string) bool { return key != "chat" || h.enabled.Load() })
	h.hub.RegisterModule("chat", func(_ context.Context, c ClientInfo, topic string) error {
		if strings.HasSuffix(topic, ":secreta") {
			return ErrTopicForbidden
		}
		return nil
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { _ = h.hub.Run(ctx) }()
	h.srv = httptest.NewServer(UpgradeHandler(h.hub, h.tickets, "http://nexus.test", testLogger()))
	t.Cleanup(func() { h.srv.Close(); cancel() })
	time.Sleep(50 * time.Millisecond) // backplane assinado
	return h
}

func (h *harness) dial(t *testing.T, userID string) *websocket.Conn {
	t.Helper()
	tk, err := h.tickets.Issue(context.Background(), ClientInfo{UserID: userID})
	if err != nil {
		t.Fatal(err)
	}
	url := "ws" + strings.TrimPrefix(h.srv.URL, "http") + "/ws?ticket=" + tk.Value
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func readFrame(t *testing.T, conn *websocket.Conn) Frame {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var f Frame
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func send(t *testing.T, conn *websocket.Conn, f Frame) {
	t.Helper()
	b, _ := json.Marshal(f)
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatal(err)
	}
}

func TestTicketIsSingleUse(t *testing.T) {
	h := newHarness(t)
	tk, _ := h.tickets.Issue(context.Background(), ClientInfo{UserID: "u1"})
	if _, err := h.tickets.Redeem(context.Background(), tk.Value); err != nil {
		t.Fatal(err)
	}
	if _, err := h.tickets.Redeem(context.Background(), tk.Value); err != ErrInvalidTicket {
		t.Fatalf("segundo resgate deveria falhar, veio %v", err)
	}
}

func TestSubscribePublishViaBackplane(t *testing.T) {
	h := newHarness(t)
	conn := h.dial(t, "u1")

	send(t, conn, Frame{Type: "subscribe", Topic: "chat:room:1"})
	if f := readFrame(t, conn); f.Type != "subscribed" {
		t.Fatalf("esperado subscribed, veio %+v", f)
	}
	if err := h.hub.Publish(context.Background(), "chat:room:1", "chat.message", map[string]string{"body": "olá"}); err != nil {
		t.Fatal(err)
	}
	f := readFrame(t, conn)
	if f.Type != "chat.message" || !strings.Contains(string(f.Data), "olá") {
		t.Fatalf("mensagem não entregue: %+v", f)
	}

	// Tópico pessoal é automático.
	_ = h.hub.Publish(context.Background(), UserTopic("u1"), "notification", map[string]string{"x": "y"})
	if f := readFrame(t, conn); f.Type != "notification" {
		t.Fatalf("tópico pessoal não entregue: %+v", f)
	}
}

func TestSubscribeDeniedByAuthorizerOrDisabledModule(t *testing.T) {
	h := newHarness(t)
	conn := h.dial(t, "u1")

	send(t, conn, Frame{Type: "subscribe", Topic: "chat:room:secreta"})
	if f := readFrame(t, conn); f.Type != "error" {
		t.Fatalf("autorizador deveria negar, veio %+v", f)
	}
	send(t, conn, Frame{Type: "subscribe", Topic: "desconhecido:x"})
	if f := readFrame(t, conn); f.Type != "error" {
		t.Fatalf("módulo sem autorizador deveria negar, veio %+v", f)
	}
	h.enabled.Store(false)
	send(t, conn, Frame{Type: "subscribe", Topic: "chat:room:1"})
	if f := readFrame(t, conn); f.Type != "error" {
		t.Fatalf("módulo desativado deveria negar, veio %+v", f)
	}
}

func TestDropModuleRemovesSubscriptions(t *testing.T) {
	h := newHarness(t)
	conn := h.dial(t, "u1")
	send(t, conn, Frame{Type: "subscribe", Topic: "chat:room:1"})
	_ = readFrame(t, conn)

	h.hub.DropModule("chat")
	if f := readFrame(t, conn); f.Type != "module.disabled" {
		t.Fatalf("esperado aviso module.disabled, veio %+v", f)
	}
	_ = h.hub.Publish(context.Background(), "chat:room:1", "chat.message", "depois")
	_ = h.hub.Publish(context.Background(), TopicBroadcast, "ping", nil)
	if f := readFrame(t, conn); f.Type != "ping" {
		t.Fatalf("mensagem do módulo desativado não deveria chegar; veio %+v", f)
	}
}

func TestOriginCheck(t *testing.T) {
	h := newHarness(t)
	tk, _ := h.tickets.Issue(context.Background(), ClientInfo{UserID: "u1"})
	url := "ws" + strings.TrimPrefix(h.srv.URL, "http") + "/ws?ticket=" + tk.Value
	hdr := map[string][]string{"Origin": {"https://evil.example"}}
	if _, _, err := websocket.DefaultDialer.Dial(url, hdr); err == nil {
		t.Fatal("origem não permitida deveria ser recusada")
	}
}
