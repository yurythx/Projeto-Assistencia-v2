package kernel

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
)

var errStore = errors.New("store fora do ar")

// failStore é um memStore cujas operações podem falhar.
type failStore struct {
	*memStore
	failEnsure, failSet bool
	failLoad            atomic.Bool
}

func (s *failStore) Ensure(ctx context.Context, key string, enabled bool) error {
	if s.failEnsure {
		return errStore
	}
	return s.memStore.Ensure(ctx, key, enabled)
}

func (s *failStore) LoadAll(ctx context.Context) (map[string]bool, error) {
	if s.failLoad.Load() {
		return nil, errStore
	}
	return s.memStore.LoadAll(ctx)
}

func (s *failStore) Set(ctx context.Context, key string, enabled bool, actor string, e audit.Entry) error {
	if s.failSet {
		return errStore
	}
	return s.memStore.Set(ctx, key, enabled, actor, e)
}

type lifecyclePlugin struct {
	fakePlugin
	fail            bool
	enable, disable atomic.Int32
}

func (p *lifecyclePlugin) OnEnable(context.Context) error {
	p.enable.Add(1)
	if p.fail {
		return errStore
	}
	return nil
}

func (p *lifecyclePlugin) OnDisable(context.Context) error {
	p.disable.Add(1)
	if p.fail {
		return errStore
	}
	return nil
}

type consumerPlugin struct {
	fakePlugin
	consumers []Consumer
}

func (p *consumerPlugin) Consumers() []Consumer { return p.consumers }

// bare não implementa nenhuma capacidade opcional.
type bare struct{ m Manifest }

func (b bare) Manifest() Manifest { return b.m }

func TestKernelPropagatesStoreFailures(t *testing.T) {
	ctx := context.Background()
	st := &failStore{memStore: newMemStore(), failEnsure: true}
	k := New(st, quiet())
	k.MustRegister(&fakePlugin{m: Manifest{Key: "blog"}})
	if err := k.Start(ctx); !errors.Is(err, errStore) {
		t.Fatalf("Start com Ensure falhando: %v", err)
	}

	st = &failStore{memStore: newMemStore()}
	k = New(st, quiet())
	k.MustRegister(&fakePlugin{m: Manifest{Key: "blog", DefaultEnabled: true}})
	if err := k.Start(ctx); err != nil {
		t.Fatal(err)
	}
	st.failLoad.Store(true)
	if err := k.Reload(ctx); !errors.Is(err, errStore) {
		t.Fatalf("Reload: %v", err)
	}
	st.failLoad.Store(false)
	st.failSet = true
	if err := k.SetEnabled(ctx, "blog", false, "t"); !errors.Is(err, errStore) || !k.Enabled("blog") {
		t.Fatalf("SetEnabled com o store fora não muda o estado: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("MustRegister com chave inválida deveria entrar em pânico")
		}
	}()
	k2 := New(newMemStore(), quiet())
	k2.MustRegister(bare{m: Manifest{Key: "X"}})
}

func TestStartDetectsIndirectCycle(t *testing.T) {
	k := New(newMemStore(), quiet())
	k.MustRegister(
		&fakePlugin{m: Manifest{Key: "aa", DependsOn: []string{"bb"}}},
		&fakePlugin{m: Manifest{Key: "bb", DependsOn: []string{"cc"}}},
		&fakePlugin{m: Manifest{Key: "cc", DependsOn: []string{"bb"}}},
	)
	if err := k.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "circular") {
		t.Fatalf("ciclo indireto: %v", err)
	}
}

func TestLifecycleHooksAndFailuresAreLogged(t *testing.T) {
	ok := &lifecyclePlugin{fakePlugin: fakePlugin{m: Manifest{Key: "okay", DefaultEnabled: true}}}
	bad := &lifecyclePlugin{fakePlugin: fakePlugin{m: Manifest{Key: "ruim", DefaultEnabled: true}}, fail: true}
	k, _ := newKernel(t, ok, bad, bare{m: Manifest{Key: "nada", DefaultEnabled: true}})
	ctx := context.Background()
	for _, key := range []string{"okay", "ruim", "nada"} {
		if err := k.SetEnabled(ctx, key, false, "t"); err != nil {
			t.Fatal(err)
		}
		if err := k.SetEnabled(ctx, key, true, "t"); err != nil {
			t.Fatal(err)
		}
	}
	if ok.enable.Load() != 1 || ok.disable.Load() != 1 || bad.enable.Load() != 1 || bad.disable.Load() != 1 {
		t.Fatal("OnEnable/OnDisable chamados uma vez cada, mesmo quando falham")
	}
}

func TestWatchPollingAndReloadFailures(t *testing.T) {
	st := &failStore{memStore: newMemStore()}
	logs := &logRecorder{}
	k := New(st, slog.New(logs))
	k.pollInterval = 5 * time.Millisecond
	k.MustRegister(&fakePlugin{m: Manifest{Key: "blog", DefaultEnabled: true}})
	if err := k.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = k.Watch(ctx); close(done) }()

	// Mudança sem NOTIFY: o polling de segurança pega.
	st.mu.Lock()
	st.state["blog"] = false
	st.mu.Unlock()
	waitFor(t, func() bool { return !k.Enabled("blog") })

	// Store fora: polling e reload pós-NOTIFY só registram o aviso.
	st.failLoad.Store(true)
	st.notify <- struct{}{}
	waitFor(t, func() bool {
		return logs.has("kernel: polling de estado falhou") && logs.has("kernel: reload após NOTIFY falhou")
	})
	cancel()
	<-done
}

// logRecorder é um slog.Handler que guarda as mensagens registradas: o
// teste espera o que o código de fato fez, em vez de um tempo fixo que
// depende da velocidade da máquina.
type logRecorder struct {
	mu   sync.Mutex
	msgs []string
}

func (l *logRecorder) Enabled(context.Context, slog.Level) bool { return true }
func (l *logRecorder) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, r.Message)
	return nil
}
func (l *logRecorder) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *logRecorder) WithGroup(string) slog.Handler      { return l }
func (l *logRecorder) has(msg string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range l.msgs {
		if m == msg {
			return true
		}
	}
	return false
}

func TestQueuesAndMountSkipPluginsWithoutCapabilities(t *testing.T) {
	q := messaging.QueueSpec{Name: "nexus.teste"}
	k, _ := newKernel(t, bare{m: Manifest{Key: "nada"}}, &consumerPlugin{fakePlugin: fakePlugin{m: Manifest{Key: "fila"}}, consumers: []Consumer{{Queue: q}}})
	if qs := k.Queues(); len(qs) != 1 || qs[0].Name != q.Name {
		t.Fatalf("filas declaradas: %v", qs)
	}
	k.MountRoutes(chi.NewRouter(), chi.NewRouter(), quiet()) // plugin sem rotas não quebra
}

func TestSuperviseRestartsFailingUnitsWithBackoff(t *testing.T) {
	var fails, panics, long atomic.Int32
	p := &fakePlugin{m: Manifest{Key: "egress", DefaultEnabled: true}, workers: []Worker{
		{Name: "erro", Process: ProcessWorker, Run: func(context.Context) error { fails.Add(1); return errors.New("caiu") }},
		{Name: "panico", Process: ProcessWorker, Run: func(context.Context) error { panics.Add(1); panic("boom") }},
		{Name: "longo", Process: ProcessWorker, Run: func(context.Context) error {
			long.Add(1)
			time.Sleep(15 * time.Millisecond) // mais que o backoff máximo: recomeça do mínimo
			return errors.New("caiu depois de rodar bem")
		}},
		{Name: "api", Process: ProcessAPI, Run: func(context.Context) error { t.Error("worker de outro processo não roda"); return nil }},
	}}
	off := &fakePlugin{m: Manifest{Key: "desligado"}, workers: []Worker{{Name: "w", Process: ProcessWorker, Run: func(context.Context) error {
		t.Error("módulo desativado não roda worker")
		return nil
	}}}}
	k, _ := newKernel(t, p, off)
	k.minBackoff, k.maxBackoff = time.Millisecond, 4*time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = k.Supervise(ctx, ProcessWorker, nil, nil); close(done) }()
	waitFor(t, func() bool { return fails.Load() >= 4 && panics.Load() >= 4 && long.Load() >= 3 })
	cancel()
	<-done
	k.subsMu.Lock()
	defer k.subsMu.Unlock()
	if len(k.subs) != 0 {
		t.Fatalf("unidades encerradas devem cancelar a assinatura: %d restantes", len(k.subs))
	}
}

func TestSuperviseShutdownDuringBackoff(t *testing.T) {
	var runs atomic.Int32
	p := &fakePlugin{m: Manifest{Key: "egress", DefaultEnabled: true}, workers: []Worker{
		{Name: "erro", Process: ProcessWorker, Run: func(context.Context) error { runs.Add(1); return errors.New("caiu") }},
	}}
	k, _ := newKernel(t, p)
	logs := &logRecorder{}
	k.logger = slog.New(logs)
	k.minBackoff, k.maxBackoff = time.Hour, time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = k.Supervise(ctx, ProcessWorker, nil, nil); close(done) }()
	// o log sai logo antes da espera do backoff: daqui em diante o
	// supervisor está (ou vai estar) esperando o backoff de 1 h
	waitFor(t, func() bool {
		return runs.Load() == 1 && logs.has("kernel: unidade terminou inesperadamente, reiniciando")
	})
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown durante o backoff deveria encerrar na hora")
	}

	// Unidade que termina junto com o shutdown não é tratada como falha.
	started := make(chan struct{})
	p2 := &fakePlugin{m: Manifest{Key: "egress", DefaultEnabled: true}, workers: []Worker{
		{Name: "fim", Process: ProcessWorker, Run: func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }},
	}}
	k2, _ := newKernel(t, p2)
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan struct{})
	go func() {
		k2.superviseUnit(ctx2, unit{module: "egress", name: "fim", run: p2.workers[0].Run})
		close(done2)
	}()
	<-started
	cancel2()
	<-done2
}

func TestSuperviseWaitsWhileDisabledUntilShutdown(t *testing.T) {
	k, _ := newKernel(t, &fakePlugin{m: Manifest{Key: "desligado"}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		k.superviseUnit(ctx, unit{module: "desligado", name: "w", run: func(context.Context) error { return nil }})
		close(done)
	}()
	// inscrito em mudanças = já passou do início e vai esperar a ativação
	waitFor(t, func() bool { k.subsMu.Lock(); defer k.subsMu.Unlock(); return len(k.subs) > 0 })
	cancel()
	<-done
}

// Sem nenhuma unidade, Supervise ainda bloqueia até ctx acabar — antes
// voltava na hora e o processo da API o reiniciava em loop com WARN.
func TestSuperviseWithoutUnitsBlocksUntilShutdown(t *testing.T) {
	k, _ := newKernel(t, &fakePlugin{m: Manifest{Key: "semworkers"}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- k.Supervise(ctx, ProcessAPI, nil, nil) }()
	select {
	case err := <-done:
		t.Fatalf("Supervise voltou antes do shutdown: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Supervise: %v", err)
	}
}

func TestSuperviseRunsConsumersWithDedup(t *testing.T) {
	pool := dbtest.Pool(t)
	var handled atomic.Int32
	var mu sync.Mutex
	queues := map[string]events.MessageHandler{}
	cp := &consumerPlugin{fakePlugin: fakePlugin{m: Manifest{Key: "fila", DefaultEnabled: true}}, consumers: []Consumer{{
		Queue:   messaging.QueueSpec{Name: "nexus.kernel.teste." + uuid.NewString()[:8]},
		Handler: func(context.Context, events.Event) error { handled.Add(1); return nil },
	}}}
	k, _ := newKernel(t, cp)
	consume := func(ctx context.Context, queue string, h events.MessageHandler) error {
		mu.Lock()
		queues[queue] = h
		mu.Unlock()
		<-ctx.Done()
		return nil
	}
	for _, dedup := range []*pgxpool.Pool{nil, pool} {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { _ = k.Supervise(ctx, ProcessWorker, consume, dedup); close(done) }()
		waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return queues[cp.consumers[0].Queue.Name] != nil })
		mu.Lock()
		h := queues[cp.consumers[0].Queue.Name]
		delete(queues, cp.consumers[0].Queue.Name)
		mu.Unlock()
		ev := events.Event{ID: uuid.New()}
		_ = h(context.Background(), ev)
		_ = h(context.Background(), ev) // reentrega
		cancel()
		<-done
	}
	// Sem dedup: 2 execuções; com dedup: 1 — total 3.
	if handled.Load() != 3 {
		t.Fatalf("handler rodou %d vezes (esperado 3: 2 sem dedup, 1 com)", handled.Load())
	}
}

func TestIdempotentCommitsOnlyOnSuccess(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	consumer := "kernel.teste." + uuid.NewString()[:8]
	var calls atomic.Int32
	fail := true
	h := Idempotent(pool, consumer, func(context.Context, events.Event) error {
		calls.Add(1)
		if fail {
			return errors.New("handler falhou")
		}
		return nil
	})
	ev := events.Event{ID: uuid.New()}
	if err := h(ctx, ev); err == nil {
		t.Fatal("erro do handler é devolvido (para o RabbitMQ reentregar)")
	}
	fail = false
	if err := h(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if err := h(ctx, ev); err != nil || calls.Load() != 2 {
		t.Fatalf("reentrega após sucesso não reexecuta: %d %v", calls.Load(), err)
	}

	// Commit falha (contexto cancelado no handler): o marcador não fica.
	cctx, cancel := context.WithCancel(ctx)
	ev2 := events.Event{ID: uuid.New()}
	if err := Idempotent(pool, consumer, func(context.Context, events.Event) error { cancel(); return nil })(cctx, ev2); err == nil {
		t.Fatal("commit com contexto cancelado deveria falhar")
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE event_id = $1`, ev2.ID).Scan(&n)
	if n != 0 {
		t.Fatal("marcador não pode ficar sem commit")
	}

	// Insert inválido (NUL no nome do consumidor) e banco fora.
	if err := Idempotent(pool, "com\x00nul", h)(ctx, ev); err == nil {
		t.Fatal("falha no insert do marcador")
	}
	closed := closedPool(t)
	if err := Idempotent(closed, consumer, h)(ctx, ev); err == nil {
		t.Fatal("banco fora no Begin")
	}
}

func TestCleanupProcessedEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var ran atomic.Int32
	db := &countingDB{n: &ran}
	done := make(chan struct{})
	go func() { _ = cleanupProcessedEvents(db, quiet(), 2*time.Millisecond)(ctx); close(done) }()
	waitFor(t, func() bool { return ran.Load() >= 2 })
	cancel()
	<-done

	// Construtor público: executa a limpeza de verdade uma vez e para no cancelamento.
	pool := dbtest.Pool(t)
	c2, stop := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); stop() }()
	if err := CleanupProcessedEvents(pool, quiet())(c2); err != nil {
		t.Fatal(err)
	}
}

// countingDB conta as limpezas tentadas; todas falham (banco fora).
type countingDB struct {
	dbtest.Fail
	n *atomic.Int32
}

func (c *countingDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.n.Add(1)
	return c.Fail.Exec(ctx, sql, args...)
}

func closedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}
	p, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
	return p
}

func TestToggleHandlerErrors(t *testing.T) {
	st := &failStore{memStore: newMemStore()}
	k := New(st, quiet())
	k.MustRegister(&fakePlugin{m: Manifest{Key: "blog", DefaultEnabled: true}})
	if err := k.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Permissions: []string{"*"}})))
		})
	})
	NewHandlers(k, quiet()).RegisterRoutes(r)
	patch := func(body string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/admin/modules/blog", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := patch(`{`); code != http.StatusBadRequest {
		t.Errorf("JSON malformado: %d", code)
	}
	if code := patch(`{}`); code != http.StatusUnprocessableEntity {
		t.Errorf("sem enabled: %d", code)
	}
	st.failSet = true
	if code := patch(`{"enabled":false}`); code != http.StatusInternalServerError {
		t.Errorf("store fora: %d", code)
	}
}

// flagCtx é um contexto cujo Err() liga sem fechar Done(): torna
// determinístico o caso "a unidade terminou junto com o shutdown".
type flagCtx struct {
	context.Context
	stopped atomic.Bool
}

func (c *flagCtx) Done() <-chan struct{} { return nil }
func (c *flagCtx) Err() error {
	if c.stopped.Load() {
		return context.Canceled
	}
	return nil
}

func TestSuperviseUnitHonoursShutdownRaces(t *testing.T) {
	k, _ := newKernel(t, &fakePlugin{m: Manifest{Key: "egress", DefaultEnabled: true}})
	if k.Enabled("nao_existe") {
		t.Fatal("módulo desconhecido nunca está ativo")
	}
	ctx := &flagCtx{Context: context.Background()}
	ctx.stopped.Store(true)
	k.superviseUnit(ctx, unit{module: "egress", name: "w", run: func(context.Context) error {
		t.Error("shutdown já pedido: a unidade nem começa")
		return nil
	}})

	ctx = &flagCtx{Context: context.Background()}
	var runs atomic.Int32
	k.superviseUnit(ctx, unit{module: "egress", name: "w", run: func(context.Context) error {
		runs.Add(1)
		ctx.stopped.Store(true) // o shutdown chega enquanto a unidade termina
		return errors.New("encerrando")
	}})
	if runs.Load() != 1 {
		t.Fatalf("término durante o shutdown não reinicia: %d", runs.Load())
	}
}
