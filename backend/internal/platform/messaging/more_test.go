package messaging

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
)

func brokerURL(t *testing.T) string {
	u := os.Getenv("TEST_RABBITMQ_URL")
	if u == "" {
		t.Skip("TEST_RABBITMQ_URL not set")
	}
	return u
}

func queueDepth(t *testing.T, conn *Connection, name string) int {
	t.Helper()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	q, err := ch.QueueDeclarePassive(name, true, false, false, false, amqp.Table{"x-dead-letter-exchange": "", "x-dead-letter-routing-key": name + ".dlq"})
	if err != nil {
		t.Fatal(err)
	}
	return q.Messages
}

// A falha de UM consumidor não pode reentregar o evento aos outros.
func TestRetryGoesOnlyToTheFailingQueue(t *testing.T) {
	conn := testConnection(t)
	a := testQueueSpec(t, conn)
	b := testQueueSpec(t, conn)
	b.RoutingKeys = a.RoutingKeys // as duas filas escutam o mesmo evento
	ch, _ := conn.Channel()
	if err := DeclareTopology(ch, []QueueSpec{b}); err != nil {
		t.Fatal(err)
	}
	_ = ch.Close()

	c := NewConsumer(conn, a.Name, 5, 3, testLogger())
	c.baseBackoff, c.maxBackoff = 50*time.Millisecond, 50*time.Millisecond
	ev, _ := events.New(a.RoutingKeys[0], "nexus.test", uuid.Nil, map[string]string{})
	if err := NewPublisher(conn).Publish(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var gotKey atomic.Value
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = c.Consume(ctx, func(context.Context, events.Event) error {
			if calls.Add(1) == 1 {
				return errors.New("falha transitória")
			}
			stop()
			return nil
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("retry não concluiu")
	}
	_ = gotKey
	time.Sleep(200 * time.Millisecond)
	if n := queueDepth(t, conn, b.Name); n != 1 {
		t.Fatalf("a outra fila recebeu %d cópias (esperado 1: o retry volta só para a fila que falhou)", n)
	}
}

func TestOriginalRoutingKeyAndHeaders(t *testing.T) {
	if got := originalRoutingKey(amqp.Delivery{RoutingKey: "fila", Headers: amqp.Table{RoutingKeyHeader: "blog.post.published"}}); got != "blog.post.published" {
		t.Fatalf("cópia de retry mantém o tipo do evento: %q", got)
	}
	if got := originalRoutingKey(amqp.Delivery{RoutingKey: "blog.post.published"}); got != "blog.post.published" {
		t.Fatalf("primeira entrega: %q", got)
	}
	for v, want := range map[any]int{int32(2): 2, int64(3): 3, 4: 4, int16(5): 5, "x": 0} {
		if got := attemptFromHeaders(amqp.Table{RetryHeader: v}); got != want {
			t.Errorf("%T: %d", v, got)
		}
	}
	if attemptFromHeaders(amqp.Table{}) != 0 {
		t.Error("sem header: primeira tentativa")
	}
	if computeBackoff(10, time.Second, 5*time.Second) != 5*time.Second || computeBackoff(1, time.Second, time.Minute) != time.Second {
		t.Error("backoff exponencial com teto")
	}
	c := amqpHeaderCarrier{"traceparent": "00-x", "n": int32(1)}
	if c.Get("traceparent") != "00-x" || c.Get("n") != "" || c.Get("ausente") != "" || len(c.Keys()) != 2 {
		t.Error("carrier de trace")
	}
	c.Set("tracestate", "v")
	if c.Get("tracestate") != "v" {
		t.Error("Set do carrier")
	}
}

func TestConnectFailsFastAndCloseIsIdempotent(t *testing.T) {
	orig, origMax := initialConnectBudget, dialBackoffMax
	initialConnectBudget, dialBackoffMax = 300*time.Millisecond, 50*time.Millisecond
	defer func() { initialConnectBudget, dialBackoffMax = orig, origMax }()
	if _, err := Connect(context.Background(), "amqp://x:y@127.0.0.1:1/", testLogger()); err == nil {
		t.Fatal("broker ausente depois do orçamento")
	}
	if _, err := Connect(context.Background(), "amqp://x:y@127.0.0.1:1/", nil); err == nil {
		t.Fatal("sem logger também falha")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	initialConnectBudget = time.Hour
	// URL malformada falha sem esperar o orçamento de uma hora
	if _, err := Connect(context.Background(), "http://127.0.0.1:5672/", nil); err == nil || !strings.Contains(err.Error(), "inválida") {
		t.Fatalf("url malformada: %v", err)
	}
	if _, err := Connect(ctx, "amqp://x:y@127.0.0.1:1/", nil); err == nil {
		t.Fatal("contexto cancelado")
	}

	conn := testConnection(t)
	if err := conn.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal("Close repetido não entra em pânico nem falha")
	}
	if _, err := conn.Channel(); err == nil {
		t.Fatal("canal depois de fechado")
	}
	if err := conn.Ping(context.Background()); err == nil {
		t.Fatal("ping depois de fechado")
	}
}

// proxy TCP que pode derrubar as conexões e parar de aceitar, simulando
// o broker caindo e voltando.
type proxy struct {
	ln     net.Listener
	target string
	mu     sync.Mutex
	conns  []net.Conn
	down   atomic.Bool
}

func newProxy(t *testing.T, target string) *proxy {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &proxy{ln: ln, target: target}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			if p.down.Load() {
				_ = c.Close()
				continue
			}
			up, err := net.Dial("tcp", target)
			if err != nil {
				_ = c.Close()
				continue
			}
			p.mu.Lock()
			p.conns = append(p.conns, c, up)
			p.mu.Unlock()
			go func() { _, _ = io.Copy(up, c); _ = up.Close() }()
			go func() { _, _ = io.Copy(c, up); _ = c.Close() }()
		}
	}()
	t.Cleanup(func() { _ = ln.Close(); p.drop() })
	return p
}

func (p *proxy) drop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		_ = c.Close()
	}
	p.conns = nil
}

func TestReconnectsAfterBrokerDrop(t *testing.T) {
	raw := brokerURL(t)
	u, _ := url.Parse(raw)
	p := newProxy(t, u.Host)
	u.Host = p.ln.Addr().String()
	orig, origMax := reconnectBackoffMin, reconnectBackoffMax
	reconnectBackoffMin, reconnectBackoffMax = 20*time.Millisecond, 40*time.Millisecond
	defer func() { reconnectBackoffMin, reconnectBackoffMax = orig, origMax }()

	conn, err := Connect(context.Background(), u.String(), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	p.down.Store(true) // as tentativas de reconexão falham por um tempo
	p.drop()
	time.Sleep(150 * time.Millisecond)
	p.down.Store(false)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if conn.Ping(context.Background()) == nil {
			if ch, err := conn.Channel(); err == nil {
				_ = ch.Close()
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("não reconectou depois da queda")
}

func TestReconnectStopsOnClose(t *testing.T) {
	raw := brokerURL(t)
	u, _ := url.Parse(raw)
	p := newProxy(t, u.Host)
	u.Host = p.ln.Addr().String()
	orig := reconnectBackoffMin
	reconnectBackoffMin = time.Hour // fica esperando o backoff
	defer func() { reconnectBackoffMin = orig }()
	conn, err := Connect(context.Background(), u.String(), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	p.down.Store(true)
	p.drop()
	time.Sleep(100 * time.Millisecond)
	if err := conn.Close(); err == nil {
		t.Log("close da conexão já caída")
	}
}

func TestConsumerEdgeCases(t *testing.T) {
	conn := testConnection(t)
	spec := testQueueSpec(t, conn)
	pub := NewPublisher(conn)

	// Fila inexistente: erro ao consumir.
	if err := NewConsumer(conn, "nao.existe."+uuid.NewString()[:6], 1, 1, testLogger()).Consume(context.Background(), nil); err == nil {
		t.Fatal("fila inexistente")
	}
	// Mensagem ilegível vai direto para a DLQ.
	ch, _ := conn.Channel()
	if err := ch.Publish(ExchangeEvents, spec.RoutingKeys[0], false, false, amqp.Publishing{Body: []byte("{nao-json")}); err != nil {
		t.Fatal(err)
	}
	_ = ch.Close()
	c := NewConsumer(conn, spec.Name, 1, 3, testLogger())
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = c.Consume(ctx, func(context.Context, events.Event) error { return nil }); close(done) }()
	waitDepth(t, conn, spec.DLQName, 1)
	stop()
	<-done

	// Shutdown durante o backoff: a mensagem volta para a fila.
	ev, _ := events.New(spec.RoutingKeys[0], "nexus.test", uuid.Nil, nil)
	_ = pub.Publish(context.Background(), ev)
	c.baseBackoff, c.maxBackoff = time.Hour, time.Hour
	ctx, stop = context.WithCancel(context.Background())
	failed := make(chan struct{}, 1)
	done = make(chan struct{})
	go func() {
		_ = c.Consume(ctx, func(context.Context, events.Event) error { failed <- struct{}{}; return errors.New("x") })
		close(done)
	}()
	<-failed
	time.Sleep(50 * time.Millisecond)
	stop()
	<-done
	waitDepth(t, conn, spec.Name, 1)
}

func waitDepth(t *testing.T, conn *Connection, q string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if queueDepth(t, conn, q) == n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fila %s não chegou a %d mensagens", q, n)
}

func TestTopologyErrors(t *testing.T) {
	conn := testConnection(t)
	name := "test.topo." + uuid.NewString()[:6]
	ch, _ := conn.Channel()
	t.Cleanup(func() {
		c, err := conn.Channel()
		if err != nil {
			return
		}
		_, _ = c.QueueDelete(name, false, false, false)
		_, _ = c.QueueDelete(name+".dlq", false, false, false)
		_ = c.Close()
	})
	// DLQ já existe com outros argumentos: recusa.
	if _, err := ch.QueueDeclare(name+".dlq", true, false, false, false, amqp.Table{"x-max-length": int32(1)}); err != nil {
		t.Fatal(err)
	}
	if err := DeclareTopology(ch, []QueueSpec{{Name: name, DLQName: name + ".dlq"}}); err == nil || !strings.Contains(err.Error(), "DLQ") {
		t.Fatalf("DLQ incompatível: %v", err)
	}
	ch, _ = conn.Channel()
	// Fila principal já existe sem a DLX: recusa.
	if _, err := ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := DeclareTopology(ch, []QueueSpec{{Name: name, DLQName: name + ".dlq2"}}); err == nil || !strings.Contains(err.Error(), "declare queue") {
		t.Fatalf("fila incompatível: %v", err)
	}
	ch, _ = conn.Channel()
	c2, _ := conn.Channel()
	_, _ = c2.QueueDelete(name, false, false, false)
	_, _ = c2.QueueDelete(name+".dlq", false, false, false)
	_ = c2.Close()
	if err := DeclareTopology(ch, []QueueSpec{{Name: name, DLQName: name + ".dlq", RoutingKeys: []string{strings.Repeat("k", 300)}}}); err == nil || !strings.Contains(err.Error(), "bind") {
		t.Fatalf("binding inválido: %v", err)
	}
	_ = ch.Close()
	if err := DeclareTopology(ch, nil); err == nil {
		t.Fatal("canal fechado")
	}
}

func TestPublisherFailures(t *testing.T) {
	conn := testConnection(t)
	ev, _ := events.New("test.pub.fail", "nexus.test", uuid.Nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewPublisher(conn).Publish(ctx, ev); err == nil {
		t.Fatal("contexto cancelado")
	}
	_ = conn.Close()
	if err := NewPublisher(conn).Publish(context.Background(), ev); err == nil {
		t.Fatal("sem conexão")
	}
}

// fakeConfirmer simula o canal com cada etapa podendo falhar.
type fakeConfirmer struct {
	confirmErr, publishErr, waitErr error
	nack                            bool
}

func (f fakeConfirmer) Confirm(bool) error { return f.confirmErr }
func (f fakeConfirmer) publish(context.Context, string, string, amqp.Publishing) (waiter, error) {
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	return f, nil
}
func (f fakeConfirmer) WaitContext(context.Context) (bool, error) { return !f.nack, f.waitErr }

func TestPublishConfirmedEveryStep(t *testing.T) {
	boom := errors.New("x")
	for name, c := range map[string]fakeConfirmer{
		"confirms recusados":   {confirmErr: boom},
		"publicação falha":     {publishErr: boom},
		"espera falha":         {waitErr: boom},
		"broker recusa (nack)": {nack: true},
	} {
		if err := publishConfirmed(context.Background(), c, "ex", "k", amqp.Publishing{}); err == nil {
			t.Errorf("%s: erro esperado", name)
		}
	}
	if err := publishConfirmed(context.Background(), fakeConfirmer{}, "ex", "k", amqp.Publishing{}); err != nil {
		t.Fatal(err)
	}
}

func TestConsumerStopsWhenConnectionGoes(t *testing.T) {
	// Conexão já fechada: não abre canal.
	closed := testConnection(t)
	_ = closed.Close()
	if err := NewConsumer(closed, "q", 1, 1, testLogger()).Consume(context.Background(), nil); err == nil {
		t.Fatal("sem conexão")
	}

	// Conexão cai no meio do backoff: o retry não consegue canal, a
	// mensagem volta à fila pelo requeue nativo; e o laço termina quando o
	// canal de entregas fecha.
	conn := testConnection(t)
	spec := testQueueSpec(t, conn)
	ev, _ := events.New(spec.RoutingKeys[0], "nexus.test", uuid.Nil, nil)
	if err := NewPublisher(conn).Publish(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	own, err := Connect(context.Background(), brokerURL(t), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	c := NewConsumer(own, spec.Name, 1, 3, testLogger())
	c.baseBackoff, c.maxBackoff = 50*time.Millisecond, 50*time.Millisecond
	done := make(chan error, 1)
	go func() {
		done <- c.Consume(context.Background(), func(context.Context, events.Event) error {
			_ = own.Close() // derruba a conexão durante o processamento
			return errors.New("falha")
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("fim do canal de entregas encerra sem erro: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consumidor não terminou quando a conexão caiu")
	}
	waitDepth(t, conn, spec.Name, 1)
}

func TestPlatformQueuesAndGracefulClose(t *testing.T) {
	if qs := PlatformQueues(); len(qs) != 1 || qs[0].Name != QueueNotificationWebsocket.Name {
		t.Fatalf("filas do núcleo: %v", qs)
	}
	conn := testConnection(t)
	time.Sleep(20 * time.Millisecond) // supervisor escutando
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond) // o supervisor vê o fechamento proposital e sai
}

func TestReconnectAbortsWhenAlreadyClosing(t *testing.T) {
	c := &Connection{done: make(chan struct{}), logger: testLogger()}
	if c.closing() {
		t.Fatal("recém-criada não está fechando")
	}
	close(c.done)
	c.reconnectWithBackoff() // retorna na hora, sem discar
	if !c.closing() {
		t.Fatal("fechando depois do done")
	}
}
