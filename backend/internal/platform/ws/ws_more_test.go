package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

func TestReadPumpHandlesEveryFrameKind(t *testing.T) {
	h := newHarness(t)
	h.hub.RegisterModule("eco", func(context.Context, ClientInfo, string) error { return nil },
		func(_ context.Context, _ ClientInfo, f Frame) error {
			if f.Type == "eco.falha" {
				return errors.New("recusado pelo módulo")
			}
			return nil
		})
	conn := h.dial(t, "u1")
	if err := conn.WriteMessage(websocket.TextMessage, []byte("{nao-json")); err != nil {
		t.Fatal(err)
	}
	if f := readFrame(t, conn); f.Type != "error" {
		t.Fatalf("frame inválido: %+v", f)
	}
	for _, c := range []struct {
		in   Frame
		want string
	}{
		{Frame{Type: "ping"}, "pong"},
		{Frame{Type: "subscribe", Topic: "chat:sala"}, "subscribed"},
		{Frame{Type: "unsubscribe", Topic: "chat:sala"}, "unsubscribed"},
		{Frame{Type: "subscribe", Topic: "chat:secreta"}, "error"},
		{Frame{Type: "desconhecido"}, "error"},   // sem "<modulo>."
		{Frame{Type: "chat.digitando"}, "error"}, // módulo sem handler de entrada
		{Frame{Type: "eco.falha"}, "error"},      // handler recusa
	} {
		send(t, conn, c.in)
		if f := readFrame(t, conn); f.Type != c.want {
			t.Fatalf("%s: esperado %s, veio %+v", c.in.Type, c.want, f)
		}
	}
	send(t, conn, Frame{Type: "eco.ok"}) // aceito: sem resposta
	send(t, conn, Frame{Type: "ping"})
	if f := readFrame(t, conn); f.Type != "pong" {
		t.Fatalf("frame aceito não responde nada: %+v", f)
	}
}

func TestDropTopicNotifiesSubscribers(t *testing.T) {
	h := newHarness(t)
	conn := h.dial(t, "u1")
	send(t, conn, Frame{Type: "subscribe", Topic: "chat:sala"})
	readFrame(t, conn)
	h.hub.DropTopic("chat:sala")
	if f := readFrame(t, conn); f.Type != "topic.dropped" || f.Topic != "chat:sala" {
		t.Fatalf("assinante avisado da queda: %+v", f)
	}
	if err := h.hub.Publish(context.Background(), "chat:sala", "chat.msg", "x"); err != nil {
		t.Fatal(err)
	}
	send(t, conn, Frame{Type: "ping"})
	if f := readFrame(t, conn); f.Type != "pong" {
		t.Fatalf("depois da queda o tópico não entrega mais: %+v", f)
	}
}

func TestSlowClientIsDisconnected(t *testing.T) {
	hub := NewHub(testLogger(), nil)
	c := newClient(hub, nil, ClientInfo{UserID: "lento"}, testLogger())
	hub.register(c)
	for i := 0; i < sendBuffer; i++ {
		c.enqueue([]byte("x")) // ninguém lê: o buffer enche
	}
	c.enqueue([]byte("transbordou"))
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		t.Fatal("cliente lento deveria ser desconectado")
	}
	waitUntil(t, func() bool { return hub.ClientCount() == 0 })
	c.enqueue([]byte("depois de fechado")) // não bloqueia
}

// writeOnly abre uma conexão real e roda SÓ o writePump do lado do
// servidor, com a conexão TCP já derrubada: toda escrita falha.
func writeOnly(t *testing.T, hub *Hub, prime func(c *Client)) {
	t.Helper()
	done := make(chan struct{})
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		c := newClient(hub, conn, ClientInfo{UserID: uuid.NewString()}, testLogger())
		hub.register(c)
		_ = conn.UnderlyingConn().Close()
		prime(c)
		c.writePump()
		close(done)
	}))
	defer srv.Close()
	cli, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cli.Close() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("writePump deveria encerrar quando a escrita falha")
	}
}

func TestWritePumpStopsOnWriteFailures(t *testing.T) {
	hub := NewHub(testLogger(), nil)
	writeOnly(t, hub, func(c *Client) { c.send <- []byte(`{"type":"x"}`) }) // mensagem
	hub.pingPeriod = 5 * time.Millisecond
	writeOnly(t, hub, func(*Client) {}) // ping
	if hub.ClientCount() != 0 {
		t.Fatal("conexões com falha de escrita são removidas")
	}
}

func TestPingKeepsHealthyConnectionsAlive(t *testing.T) {
	h := newHarness(t)
	h.hub.pingPeriod, h.hub.pongWait = 10*time.Millisecond, 200*time.Millisecond
	conn := h.dial(t, "u1")
	pings := make(chan struct{}, 8)
	conn.SetPingHandler(func(string) error {
		select {
		case pings <- struct{}{}:
		default: // o teste só precisa das primeiras; nunca bloquear o leitor
		}
		return conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second))
	})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-pings:
		case <-time.After(2 * time.Second):
			t.Fatal("servidor deveria mandar pings periódicos")
		}
	}
	time.Sleep(300 * time.Millisecond) // além do pongWait: os pongs renovam o prazo
	if h.hub.ClientCount() != 1 {
		t.Fatalf("conexão saudável (respondendo pong) não pode cair: %d conexões", h.hub.ClientCount())
	}
}

func TestTicketHandler(t *testing.T) {
	mr := miniredis.RunT(t)
	store := NewTicketStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}), TicketTTL)
	h := TicketHandler(store, testLogger())
	call := func(id *auth.Identity) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/ws/ticket", nil)
		if id != nil {
			req = req.WithContext(auth.WithIdentity(req.Context(), *id))
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}
	if rec := call(nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem identidade: %d", rec.Code)
	}
	uid := uuid.New()
	rec := call(&auth.Identity{Subject: "s", UserID: uid, Permissions: []string{"blog:manage"}})
	var body struct {
		Data TicketResponse `json:"data"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
		t.Fatalf("emissão: %d %s", rec.Code, rec.Body.String())
	}
	info, err := store.Redeem(context.Background(), body.Data.Ticket)
	if err != nil || info.UserID != uid.String() || info.Identity().UserID != uid || info.Identity().Permissions[0] != "blog:manage" {
		t.Fatalf("ticket carrega a identidade: %+v %v", info, err)
	}
	if rec := call(&auth.Identity{Subject: "sem-id"}); rec.Code != http.StatusCreated {
		t.Fatalf("identidade sem id interno: %d", rec.Code)
	}
	// Ticket gravado corrompido: inválido.
	bad := strings.Repeat("a", 64)
	mr.Set("nexus:ws:ticket:"+bad, "{nao-json")
	if _, err := store.Redeem(context.Background(), bad); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("ticket corrompido: %v", err)
	}
	// Redis fora: emissão 503 e resgate com erro (não "inválido").
	mr.Close()
	if rec := call(&auth.Identity{Subject: "s"}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Redis fora: %d", rec.Code)
	}
	if _, err := store.Redeem(context.Background(), bad); err == nil || errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("resgate com o Redis fora: %v", err)
	}
}

func TestUpgradeRejectsNonWebSocketRequests(t *testing.T) {
	h := newHarness(t)
	tk, _ := h.tickets.Issue(context.Background(), ClientInfo{UserID: "u1"})
	res, err := http.Get(h.srv.URL + "/ws?ticket=" + tk.Value) // ticket válido, mas sem upgrade
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("requisição comum no endpoint de WebSocket: %d", res.StatusCode)
	}
}

func TestHubBackplaneEdgeCases(t *testing.T) {
	ctx := context.Background()
	// Sem Redis: Run só espera o shutdown e a publicação é local.
	local := NewHub(testLogger(), nil)
	c, cancel := context.WithCancel(ctx)
	cancel()
	if err := local.Run(c); err != nil {
		t.Fatal(err)
	}
	if err := local.Publish(ctx, "t", "x", make(chan int)); err == nil {
		t.Fatal("dado não serializável")
	}

	// Mensagem inválida no backplane é ignorada; canal fechado é erro.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	hub := NewHub(testLogger(), rdb)
	msgs := make(chan *redis.Message, 1)
	hub.subscribe = func(context.Context) (<-chan *redis.Message, func() error) { return msgs, func() error { return nil } }
	msgs <- &redis.Message{Payload: "{nao-json"}
	close(msgs)
	if err := hub.Run(ctx); err == nil {
		t.Fatal("backplane encerrado deveria ser erro")
	}

	// Redis fora: entrega só local.
	cl := newClient(hub, nil, ClientInfo{}, testLogger())
	hub.register(cl)
	hub.subscribeUnchecked(cl, "chat:x")
	mr.Close()
	if err := hub.Publish(ctx, "chat:x", "chat.msg", "oi"); err != nil {
		t.Fatal(err)
	}
	select {
	case f := <-cl.send:
		if !strings.Contains(string(f), "chat.msg") {
			t.Fatalf("frame local: %s", f)
		}
	case <-time.After(time.Second):
		t.Fatal("sem backplane, a entrega local continua")
	}
}

func TestSubscribeRules(t *testing.T) {
	hub := NewHub(testLogger(), nil)
	hub.RegisterModule("chat", func(context.Context, ClientInfo, string) error { return nil }, nil)
	c := newClient(hub, nil, ClientInfo{}, testLogger())
	for _, topic := range []string{"", strings.Repeat("x", 201), "user:outro", "broadcast", "sem_autorizador:x"} {
		if err := hub.Subscribe(context.Background(), c, topic); !errors.Is(err, ErrTopicForbidden) {
			t.Errorf("tópico %q deveria ser recusado", topic[:min(len(topic), 20)])
		}
	}
	if err := hub.Subscribe(context.Background(), c, "chat:sala"); err != nil {
		t.Fatal(err)
	}
	hub.Unsubscribe(c, "chat:inexistente") // sem efeito
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condição não atingida a tempo")
}

func TestShutdownAndInvalidTickets(t *testing.T) {
	hub := NewHub(testLogger(), nil)
	c := newClient(hub, nil, ClientInfo{UserID: "u"}, testLogger())
	hub.register(c)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = hub.Run(ctx) // shutdown: desconecta os clientes locais
	select {
	case <-c.done:
	default:
		t.Fatal("shutdown fecha as conexões locais")
	}
	hub.unregister(c) // já removido: sem efeito
	hub.unregister(c)

	h := newHarness(t)
	for _, tk := range []string{"curto", strings.Repeat("b", 64)} {
		_, res, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(h.srv.URL, "http")+"/ws?ticket="+tk, nil)
		if err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("ticket %q deveria ser recusado (401)", tk[:5])
		}
	}
}
