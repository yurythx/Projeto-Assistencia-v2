package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// flakyPublisher falha nas primeiras `fail` chamadas.
type flakyPublisher struct {
	fail  int32
	calls atomic.Int32
	onPub func()
}

func (f *flakyPublisher) Publish(context.Context, events.Event) error {
	n := f.calls.Add(1)
	if f.onPub != nil {
		f.onPub()
	}
	if n <= f.fail {
		return errors.New("broker fora do ar")
	}
	return nil
}

func writeEvents(t *testing.T, pool *pgxpool.Pool, n int) string {
	t.Helper()
	agg := uuid.NewString()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if err := NewWriter("nexus.test").Write(context.Background(), tx, "test.relay.created", "test", agg, uuid.Nil, map[string]int{"i": i}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return agg
}

func TestPublisherBacksOffPerEventAndStopsTheBatch(t *testing.T) {
	pool := testPool(t)
	truncateOutbox(t, pool)
	ctx := context.Background()
	agg := writeEvents(t, pool, 3)
	fp := &flakyPublisher{fail: 2}
	pub := NewPublisher(pool, fp, quiet())

	// Broker fora: o lote para na primeira falha — só UMA linha perde tentativa.
	if err := pub.publishPendingBatch(ctx); err != nil {
		t.Fatal(err)
	}
	var burned int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND attempts > 0`, agg).Scan(&burned)
	if fp.calls.Load() != 1 || burned != 1 {
		t.Fatalf("broker fora deveria gastar só uma tentativa por lote: calls=%d linhas=%d", fp.calls.Load(), burned)
	}
	var wait time.Duration
	_ = pool.QueryRow(ctx, `SELECT extract(epoch FROM next_attempt_at - now()) * interval '1 second' FROM outbox_events
		WHERE aggregate_id = $1 AND attempts = 1`, agg).Scan(&wait)
	if wait < 500*time.Millisecond || wait > 2*time.Second {
		t.Fatalf("primeira falha reagenda em ~1s, veio %v", wait)
	}

	// Próximo lote: a linha em espera é pulada; a seguinte é tentada.
	if err := pub.publishPendingBatch(ctx); err != nil {
		t.Fatal(err)
	}
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND attempts > 0`, agg).Scan(&burned)
	if fp.calls.Load() != 2 || burned != 2 {
		t.Fatalf("linha em backoff não é tentada antes da hora: calls=%d linhas=%d", fp.calls.Load(), burned)
	}

	// Broker de volta e horários vencidos: tudo sai no mesmo lote.
	if _, err := pool.Exec(ctx, `UPDATE outbox_events SET next_attempt_at = now() WHERE aggregate_id = $1`, agg); err != nil {
		t.Fatal(err)
	}
	if err := pub.publishPendingBatch(ctx); err != nil {
		t.Fatal(err)
	}
	var published int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND status = 'published'`, agg).Scan(&published)
	if published != 3 {
		t.Fatalf("com o broker de volta, todas publicadas: %d", published)
	}
}

func TestRetryDelayGrowsAndCaps(t *testing.T) {
	p := &Publisher{retryBase: time.Second, retryMax: time.Hour}
	for attempts, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 5: 16 * time.Second, 12: 2048 * time.Second, 13: time.Hour, 40: time.Hour} {
		if got := p.retryDelay(attempts); got != want {
			t.Errorf("tentativa %d: %v, esperado %v", attempts, got, want)
		}
	}
	// Com os padrões, 20 tentativas cobrem horas de broker fora (não minutos).
	d := NewPublisher(nil, nil, quiet())
	var total time.Duration
	for i := 1; i < d.maxAttempts; i++ {
		total += d.retryDelay(i)
	}
	if total < 6*time.Hour {
		t.Fatalf("janela antes de desistir: %v", total)
	}
}

func TestUndecodableEnvelopeFailsWithoutBlockingTheBatch(t *testing.T) {
	pool := testPool(t)
	truncateOutbox(t, pool)
	ctx := context.Background()
	bad := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload, created_at)
		VALUES ($1, 'test.relay.bad', 'test', 'x', '{"id": 5}', now() - interval '1 minute')`, bad); err != nil {
		t.Fatal(err)
	}
	agg := writeEvents(t, pool, 1)
	fp := &flakyPublisher{}
	if err := NewPublisher(pool, fp, quiet()).publishPendingBatch(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	_ = pool.QueryRow(ctx, `SELECT status FROM outbox_events WHERE id = $1`, bad).Scan(&status)
	var ok int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND status = 'published'`, agg).Scan(&ok)
	if status != "failed" || ok != 1 || fp.calls.Load() != 1 {
		t.Fatalf("envelope ilegível falha sozinho e o lote segue: %s %d %d", status, ok, fp.calls.Load())
	}
}

func TestPublishRowPropagatesStatusUpdateFailures(t *testing.T) {
	ctx := context.Background()
	ev, _ := events.New("test.relay.created", "nexus.test", uuid.Nil, nil)
	payload, _ := json.Marshal(ev)
	for name, c := range map[string]struct {
		pub      events.EventPublisher
		payload  []byte
		attempts int
	}{
		"publicado":     {pub: &flakyPublisher{}, payload: payload},
		"reagendado":    {pub: &flakyPublisher{fail: 1}, payload: payload},
		"esgotado":      {pub: &flakyPublisher{fail: 1}, payload: payload, attempts: 19},
		"envelope ruim": {pub: &flakyPublisher{}, payload: []byte(`{"id": 5}`)},
	} {
		p := NewPublisher(nil, c.pub, quiet())
		if _, err := p.publishRow(ctx, dbtest.Fail{}, outboxRow{id: uuid.New(), payload: c.payload, attempts: c.attempts}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s: falha ao gravar o status tem de abortar o lote (%v)", name, err)
		}
	}
}

// txHook envolve uma transação real trocando Query/Commit.
type txHook struct {
	pgx.Tx
	failQuery, failCommit bool
	rows                  pgx.Rows
}

func (h *txHook) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if h.failQuery {
		return nil, dbtest.ErrInjected
	}
	if h.rows != nil {
		return h.rows, nil
	}
	return h.Tx.Query(ctx, sql, args...)
}

func (h *txHook) Commit(ctx context.Context) error {
	if h.failCommit {
		return dbtest.ErrInjected
	}
	return h.Tx.Commit(ctx)
}

func TestPublishPendingBatchFailures(t *testing.T) {
	pool := testPool(t)
	truncateOutbox(t, pool)
	ctx := context.Background()
	writeEvents(t, pool, 1)
	for name, hook := range map[string]func(*txHook){
		"consulta":             func(h *txHook) { h.failQuery = true },
		"linha ilegível":       func(h *txHook) { h.rows = dbtest.BadRows() },
		"leitura interrompida": func(h *txHook) { h.rows = dbtest.ErrRows() },
		"commit":               func(h *txHook) { h.failCommit = true },
	} {
		p := NewPublisher(pool, &flakyPublisher{}, quiet())
		p.begin = func(ctx context.Context) (pgx.Tx, error) {
			tx, err := pool.Begin(ctx)
			h := &txHook{Tx: tx}
			hook(h)
			return h, err
		}
		if err := p.publishPendingBatch(ctx); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Status que não grava aborta o lote e o commit não acontece.
	cctx, cancel := context.WithCancel(ctx)
	p := NewPublisher(pool, &flakyPublisher{onPub: cancel}, quiet())
	if err := p.publishPendingBatch(cctx); err == nil {
		t.Error("status não gravado deveria abortar o lote")
	}
	var pending int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE status = 'pending'`).Scan(&pending)
	if pending != 1 {
		t.Fatalf("lote revertido: a linha continua pendente (%d)", pending)
	}
	closed, _ := pgxpool.New(ctx, pool.Config().ConnString())
	closed.Close()
	if err := NewPublisher(closed, &flakyPublisher{}, quiet()).publishPendingBatch(ctx); err == nil {
		t.Error("banco fora no begin")
	}
}

func TestRunDispatchesOnNotifyTickerAndReconnects(t *testing.T) {
	pool := testPool(t)
	truncateOutbox(t, pool)
	fp := &flakyPublisher{}
	pub := NewPublisher(pool, fp, quiet())
	// Sem polling: tudo o que sai depois vem do NOTIFY, o que prova que o
	// LISTEN está de pé (e, depois de derrubado, que reconectou).
	pub.pollInterval = time.Hour
	pub.listenMin = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- pub.Run(ctx) }()

	// pid da conexão deste banco em LISTEN (0 se ainda não há).
	// pg_stat_activity cobre o cluster todo, daí o filtro por datname.
	listener := func() int32 {
		var pid int32
		_ = pool.QueryRow(context.Background(), `SELECT pid FROM pg_stat_activity
			WHERE pid <> pg_backend_pid() AND datname = current_database() AND query = 'LISTEN '||$1
			ORDER BY backend_start DESC LIMIT 1`, Channel).Scan(&pid)
		return pid
	}
	var first int32
	waitUntil(t, func() bool { first = listener(); return first != 0 })

	writeEvents(t, pool, 1)
	waitUntil(t, func() bool { return fp.calls.Load() == 1 })

	// LISTEN derrubado depois de estabelecido: reconecta com o backoff
	// mínimo e segue despachando.
	var killed bool
	if err := pool.QueryRow(context.Background(), `SELECT pg_terminate_backend($1)`, first).Scan(&killed); err != nil || !killed {
		t.Fatalf("derrubar o LISTEN (pid %d): %v %v", first, killed, err)
	}
	waitUntil(t, func() bool { pid := listener(); return pid != 0 && pid != first })
	writeEvents(t, pool, 1)
	waitUntil(t, func() bool { return fp.calls.Load() == 2 })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunSurvivesDatabaseOutage(t *testing.T) {
	pool := testPool(t)
	closed, _ := pgxpool.New(context.Background(), pool.Config().ConnString())
	closed.Close()
	pub := NewPublisher(closed, &flakyPublisher{}, quiet())
	pub.pollInterval, pub.listenMin, pub.listenMax = 5*time.Millisecond, time.Millisecond, 2*time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if err := pub.Run(ctx); err != nil {
		t.Fatal(err)
	}

	// LISTEN recusado (canal inválido) e shutdown durante a espera de reconexão.
	bad := NewPublisher(pool, &flakyPublisher{}, quiet())
	bad.channel = "canal inválido"
	bad.listenMin = time.Hour
	c2, cancel2 := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel2()
	bad.listenLoop(c2, func() {})
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condição não atingida a tempo")
}

func TestRequeueReturnsFailedEventsAndAudits(t *testing.T) {
	pool := testPool(t)
	truncateOutbox(t, pool)
	ctx := context.Background()
	agg := writeEvents(t, pool, 2)
	if _, err := pool.Exec(ctx, `UPDATE outbox_events SET status = 'failed', attempts = 20, last_error = 'x',
		next_attempt_at = now() + interval '1 day' WHERE aggregate_id = $1`, agg); err != nil {
		t.Fatal(err)
	}
	s := NewStats(pool)
	h := NewStatsHandlers(s, quiet())
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Permissions: []string{"*"}})))
		})
	})
	RegisterStatsRoutes(r, h, quiet())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/monitoring/outbox/requeue", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"requeued":2`) {
		t.Fatalf("reprocessar: %d %s", rec.Code, rec.Body.String())
	}
	var due int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND status = 'pending'
		AND attempts = 0 AND last_error IS NULL AND next_attempt_at <= now()`, agg).Scan(&due)
	var audited int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = $1 AND diff_after->>'requeued' = '2'`, ActionRequeued).Scan(&audited)
	if due != 2 || audited < 1 {
		t.Fatalf("eventos elegíveis agora (%d) e reprocessamento auditado (%d)", due, audited)
	}

	if rec := httptest.NewRecorder(); true {
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/monitoring/outbox-stats", nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"pending":2`) {
			t.Fatalf("estatísticas: %d %s", rec.Code, rec.Body.String())
		}
	}
	// NOTIFY recusado (canal longo demais): nada volta à fila.
	s.channel = strings.Repeat("c", 100)
	if _, err := s.Requeue(ctx); err == nil {
		t.Error("NOTIFY recusado deveria falhar o reprocessamento")
	}

	// Banco fora: 500 nas duas rotas.
	closed, _ := pgxpool.New(ctx, pool.Config().ConnString())
	closed.Close()
	down := chi.NewRouter()
	down.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Permissions: []string{"*"}})))
		})
	})
	RegisterStatsRoutes(down, NewStatsHandlers(NewStats(closed), quiet()), quiet())
	for _, m := range []string{http.MethodPost + " /monitoring/outbox/requeue", http.MethodGet + " /monitoring/outbox-stats"} {
		parts := strings.SplitN(m, " ", 2)
		rec := httptest.NewRecorder()
		down.ServeHTTP(rec, httptest.NewRequest(parts[0], parts[1], nil))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s com o banco fora: %d", m, rec.Code)
		}
	}
}

func TestStatsPropagatesReadFailures(t *testing.T) {
	s := &Stats{}
	for name, db := range map[string]*dbtest.Seq{
		"linha ilegível":       {Queries: []dbtest.QueryResult{{Rows: dbtest.BadRows()}}},
		"leitura interrompida": {Queries: []dbtest.QueryResult{{Rows: dbtest.ErrRows()}}},
	} {
		s.db = db
		if _, err := s.Get(context.Background()); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// execTx falha no n-ésimo Exec.
type execTx struct {
	pgx.Tx
	failAt, n int
}

func (e *execTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	e.n++
	if e.n == e.failAt {
		return pgconn.CommandTag{}, dbtest.ErrInjected
	}
	return e.Tx.Exec(ctx, sql, args...)
}

func TestWriterRejectsBadEventsAndPropagatesFailures(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	w := NewWriter("nexus.test")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := w.Write(ctx, tx, "", "t", "a", uuid.Nil, nil); err == nil {
		t.Error("tipo vazio")
	}
	if err := w.Write(ctx, tx, "Fora.Da.Convencao!", "t", "a", uuid.Nil, nil); err == nil {
		t.Error("tipo fora da convenção <contexto>.<entidade>.<ação>")
	}
	for _, at := range []int{1, 2} {
		if err := w.Write(ctx, &execTx{Tx: tx, failAt: at}, "test.writer.created", "t", "a", uuid.Nil, nil); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("falha no Exec %d (insert/notify): %v", at, err)
		}
	}
}

func TestSchemaCompilationAndValidationFailures(t *testing.T) {
	if _, err := compileSchema([]byte("{")); err == nil {
		t.Error("schema que não é JSON")
	}
	if _, err := compileSchema([]byte(`{"type": 5}`)); err == nil {
		t.Error("schema inválido")
	}
	if err := validateEnvelope([]byte("{")); err == nil {
		t.Error("envelope que não é JSON")
	}
	orig := envelopeSchema
	defer func() { envelopeSchema = orig }()
	envelopeSchema = func() (*jsonschema.Schema, error) { return nil, errors.New("não compila") }
	if err := validateEnvelope([]byte("{}")); err == nil {
		t.Error("sem schema, nenhum evento passa sem validação")
	}
}
