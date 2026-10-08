package audit

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/storage/storagetest"
)

// wormStore é um storage com object-lock (storage.WORMWriter) e
// EnsureBucket, com falhas injetáveis.
type wormStore struct {
	*storagetest.Memory
	failLock, failBucket bool
	failPutAt, puts      int
	retention            []int
}

func (s *wormStore) EnsureImmutableBucket(context.Context, string, int) error {
	if s.failLock {
		return errors.New("bucket sem object-lock")
	}
	return nil
}

func (s *wormStore) PutImmutable(ctx context.Context, bucket, key string, r io.Reader, size int64, ct string, days int) error {
	s.retention = append(s.retention, days)
	return s.Put(ctx, bucket, key, r, size, ct)
}

func (s *wormStore) Put(ctx context.Context, bucket, key string, r io.Reader, size int64, ct string) error {
	s.puts++
	if s.failPutAt == s.puts {
		return errors.New("put falhou")
	}
	return s.Memory.Put(ctx, bucket, key, r, size, ct)
}

func (s *wormStore) EnsureBucket(context.Context, string) error {
	if s.failBucket {
		return errors.New("bucket indisponível")
	}
	return nil
}

// plainStore só tem Put comum: nem object-lock nem EnsureBucket.
type plainStore struct{ *storagetest.Memory }

// hookDB envolve o banco real e troca a resposta da n-ésima chamada. As
// transações abertas por Begin passam pelos mesmos ganchos e contadores.
type hookDB struct {
	database.DBTX
	row                 func(n int) pgx.Row
	query               func(n int) (pgx.Rows, error)
	exec                func(n int) error
	beginErr, commitErr error
	nr, nq, ne          int
}

// Begin abre um savepoint quando o banco envolvido é uma transação real;
// sobre um dublê sem Begin, a transação é só os ganchos.
func (h *hookDB) Begin(ctx context.Context) (pgx.Tx, error) {
	if h.beginErr != nil {
		return nil, h.beginErr
	}
	tx := &hookTx{h: h}
	if b, ok := h.DBTX.(txBeginner); ok {
		inner, err := b.Begin(ctx)
		if err != nil {
			return nil, err
		}
		tx.Tx = inner
	}
	return tx, nil
}

type hookTx struct {
	pgx.Tx
	h *hookDB
}

func (t *hookTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.h.QueryRow(ctx, sql, args...)
}

func (t *hookTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.h.Query(ctx, sql, args...)
}

func (t *hookTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.h.Exec(ctx, sql, args...)
}

func (t *hookTx) Commit(ctx context.Context) error {
	if t.h.commitErr != nil {
		return t.h.commitErr
	}
	if t.Tx == nil {
		return nil
	}
	return t.Tx.Commit(ctx)
}

func (t *hookTx) Rollback(ctx context.Context) error {
	if t.Tx == nil {
		return nil
	}
	return t.Tx.Rollback(ctx)
}

func (h *hookDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	h.nr++
	if h.row != nil {
		if r := h.row(h.nr); r != nil {
			return r
		}
	}
	return h.DBTX.QueryRow(ctx, sql, args...)
}

func (h *hookDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	h.nq++
	if h.query != nil {
		if r, err := h.query(h.nq); r != nil || err != nil {
			return r, err
		}
	}
	return h.DBTX.Query(ctx, sql, args...)
}

func (h *hookDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	h.ne++
	if h.exec != nil {
		if err := h.exec(h.ne); err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	return h.DBTX.Exec(ctx, sql, args...)
}

type scanRow func(dest ...any) error

func (f scanRow) Scan(dest ...any) error { return f(dest...) }

var noRows = scanRow(func(...any) error { return pgx.ErrNoRows })

// lockRow responde ao pg_try_advisory_xact_lock.
func lockRow(got bool) pgx.Row {
	return scanRow(func(dest ...any) error { *(dest[0].(*bool)) = got; return nil })
}

// wormTx abre uma transação que é sempre revertida: audit_worm_exports é
// append-only, então o teste não pode deixar marcas d'água no banco.
func wormTx(t *testing.T) pgx.Tx {
	t.Helper()
	pool := testPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if err := NewWriter(tx).Record(context.Background(), Entry{Action: "test.worm", IPAddress: "10.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	return tx
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func read(t *testing.T, s *wormStore, key string) []byte {
	t.Helper()
	rc, err := s.Get(context.Background(), "worm", key)
	if err != nil {
		t.Fatalf("objeto %s: %v", key, err)
	}
	defer func() { _ = rc.Close() }()
	b, _ := io.ReadAll(rc)
	return b
}

func TestWORMExportChainsDaysAndResumesFromWatermark(t *testing.T) {
	tx := wormTx(t)
	ctx := context.Background()
	store := &wormStore{Memory: storagetest.New()}
	e := newWORMExporter(tx, store, "worm", 30, quietLogger())
	day0, prev, err := e.nextDay(ctx, tx)
	if err != nil || day0.IsZero() || prev != "" {
		t.Fatalf("primeiro dia sem exportação: %v %q %v", day0, prev, err)
	}
	const d = 24 * time.Hour
	at := func(days int) func() time.Time {
		return func() time.Time { return day0.Add(time.Duration(days)*d + time.Hour) }
	}

	e.now = at(2) // exporta day0 e day0+1
	e.run(ctx)
	if store.puts != 4 || len(store.retention) != 4 || store.retention[0] != 30 {
		t.Fatalf("2 dias = 2 arquivos + 2 digests imutáveis com retenção: puts=%d ret=%v", store.puts, store.retention)
	}
	var shas []string
	for i := 0; i < 2; i++ {
		day := day0.Add(time.Duration(i) * d)
		key := day.Format("2006/01") + "/" + day.Format("2006-01-02") + ".jsonl"
		body := read(t, store, key)
		sum := sha256.Sum256(body)
		digest := hex.EncodeToString(sum[:])
		if got := string(read(t, store, key+".sha256")); !strings.HasPrefix(got, digest+"  ") {
			t.Fatalf("digest do dia %d não confere: %q", i, got)
		}
		var header map[string]any
		sc := bufio.NewScanner(bytes.NewReader(body))
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		sc.Scan()
		if err := json.Unmarshal(sc.Bytes(), &header); err != nil || header["_worm_header"] != true {
			t.Fatalf("cabeçalho: %s %v", sc.Bytes(), err)
		}
		want := ""
		if i > 0 {
			want = shas[i-1]
		}
		if header["prev_sha256"] != want {
			t.Fatalf("dia %d deveria encadear em %q, veio %v", i, want, header["prev_sha256"])
		}
		for sc.Scan() {
			if strings.Contains(sc.Text(), `"ip_address":"10.0.0.1/`) {
				t.Fatal("IP sai sem máscara (host), não como inet::text")
			}
		}
		var stored, storedPrev string
		if err := tx.QueryRow(ctx, `SELECT sha256, prev_sha256 FROM audit_worm_exports WHERE day = $1`, day).Scan(&stored, &storedPrev); err != nil || stored != digest || storedPrev != want {
			t.Fatalf("marca d'água do dia %d: %q/%q %v", i, stored, storedPrev, err)
		}
		shas = append(shas, digest)
	}

	// Retoma da marca d'água: só o dia novo, encadeado ao anterior.
	e.now = at(3)
	e.run(ctx)
	if store.puts != 6 {
		t.Fatalf("só o dia novo deveria sair: puts=%d", store.puts)
	}
	if next, prev, err := e.nextDay(ctx, tx); err != nil || !next.Equal(day0.Add(3*d)) {
		t.Fatalf("próximo dia: %v %v", next, err)
	} else {
		var p string
		_ = tx.QueryRow(ctx, `SELECT prev_sha256 FROM audit_worm_exports WHERE day = $1`, day0.Add(2*d)).Scan(&p)
		if p != shas[1] || prev == "" {
			t.Fatalf("terceiro dia deveria encadear no segundo: %q", p)
		}
	}
	e.run(ctx)
	if store.puts != 6 {
		t.Fatal("nada novo para exportar: nenhum put")
	}
	var n int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'audit.worm.exported' AND metadata->>'immutable' = 'true'`).Scan(&n)
	if n < 3 {
		t.Fatalf("cada exportação é auditada: %d", n)
	}
}

func TestWORMExportBucketFallbacks(t *testing.T) {
	tx := wormTx(t)
	ctx := context.Background()
	day0, _, err := newWORMExporter(tx, plainStore{storagetest.New()}, "worm", 1, quietLogger()).nextDay(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	tomorrow := func() time.Time { return day0.Add(25 * time.Hour) }

	// Sem object-lock: cai para Put comum (a cadeia de hash segue valendo).
	noLock := &wormStore{Memory: storagetest.New(), failLock: true}
	e := newWORMExporter(tx, noLock, "worm", 1, quietLogger())
	e.now = tomorrow
	e.run(ctx)
	if e.worm != nil || len(noLock.retention) != 0 || noLock.puts != 2 || !e.bucketReady {
		t.Fatalf("fallback para Put comum: worm=%v ret=%v puts=%d", e.worm, noLock.retention, noLock.puts)
	}
	if !e.ensureBucket(ctx) {
		t.Fatal("bucket já preparado")
	}

	// Nem lock nem bucket: não exporta nada e tenta de novo no próximo tick.
	down := &wormStore{Memory: storagetest.New(), failLock: true, failBucket: true}
	e = newWORMExporter(tx, down, "worm", 1, quietLogger())
	e.now = tomorrow
	e.run(ctx)
	if down.puts != 0 || e.bucketReady {
		t.Fatal("sem bucket não há exportação")
	}

	// Provider sem EnsureBucket nem lock: segue com o aviso.
	plain := plainStore{storagetest.New()}
	e = newWORMExporter(tx, plain, "worm", 1, quietLogger())
	if !e.ensureBucket(ctx) {
		t.Fatal("provider simples segue sem garantir o bucket")
	}
}

func TestWORMExportStopsOnEveryFailure(t *testing.T) {
	ctx := context.Background()
	fail := errors.New("falha")
	cases := map[string]struct {
		db    func(tx pgx.Tx) *hookDB
		store *wormStore
		saved bool // a marca d'água do dia fica gravada?
	}{
		"begin": {db: func(tx pgx.Tx) *hookDB { return &hookDB{DBTX: tx, beginErr: fail} }},
		"advisory lock ilegível": {db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, row: func(int) pgx.Row { return scanRow(func(...any) error { return fail }) }}
		}},
		"lock com outra réplica": {db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, row: func(n int) pgx.Row { return lockRow(false) }}
		}},
		"marca d'água ilegível": {db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, row: func(n int) pgx.Row {
				if n == 2 {
					return scanRow(func(...any) error { return fail })
				}
				return nil
			}}
		}},
		"início da trilha ilegível": {db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, row: func(n int) pgx.Row {
				switch n {
				case 1:
					return nil
				case 2:
					return noRows
				}
				return scanRow(func(...any) error { return fail })
			}}
		}},
		"commit": {db: func(tx pgx.Tx) *hookDB { return &hookDB{DBTX: tx, commitErr: fail} }},
		"consulta do dia": {db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, query: func(int) (pgx.Rows, error) { return nil, fail }}
		}},
		"linha ilegível": {db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, query: func(int) (pgx.Rows, error) { return dbtest.BadRows(), nil }}
		}},
		"leitura interrompida": {db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, query: func(int) (pgx.Rows, error) { return dbtest.ErrRows(), nil }}
		}},
		"put do arquivo": {db: func(tx pgx.Tx) *hookDB { return &hookDB{DBTX: tx} }, store: &wormStore{failPutAt: 1}},
		"put do digest":  {db: func(tx pgx.Tx) *hookDB { return &hookDB{DBTX: tx} }, store: &wormStore{failPutAt: 2}},
		"marca d'água": {db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, exec: func(n int) error {
				if n == 1 {
					return fail
				}
				return nil
			}}
		}},
		"trilha da exportação (só avisa)": {saved: true, db: func(tx pgx.Tx) *hookDB {
			return &hookDB{DBTX: tx, exec: func(n int) error {
				if n == 2 {
					return fail
				}
				return nil
			}}
		}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			tx := wormTx(t)
			day0, _, err := newWORMExporter(tx, plainStore{storagetest.New()}, "worm", 1, quietLogger()).nextDay(ctx, tx)
			if err != nil {
				t.Fatal(err)
			}
			store := c.store
			if store == nil {
				store = &wormStore{}
			}
			store.Memory = storagetest.New()
			e := newWORMExporter(c.db(tx), store, "worm", 1, quietLogger())
			e.now = func() time.Time { return day0.Add(49 * time.Hour) } // dois dias pendentes
			e.run(ctx)
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_worm_exports`).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if c.saved != (n == 2) || (!c.saved && n != 0) {
				t.Fatalf("marcas d'água gravadas: %d (esperado salvo=%v)", n, c.saved)
			}
		})
	}

	// Trilha vazia: nada a exportar.
	store := &wormStore{Memory: storagetest.New()}
	e := newWORMExporter(&hookDB{DBTX: dbtest.Fail{}, row: func(n int) pgx.Row {
		switch n {
		case 1:
			return lockRow(true)
		case 2:
			return noRows
		}
		return scanRow(func(dest ...any) error { *(dest[0].(**time.Time)) = nil; return nil })
	}}, store, "worm", 1, quietLogger())
	e.run(ctx)
	if store.puts != 0 {
		t.Fatal("trilha vazia não gera arquivo")
	}
}

// Duas réplicas: enquanto uma segura o advisory lock (outra conexão), a
// outra não exporta nada e desiste até o próximo tick.
func TestWORMExportSkipsWhileAnotherReplicaHoldsTheLock(t *testing.T) {
	ctx := context.Background()
	tx := wormTx(t)
	day0, _, err := newWORMExporter(tx, plainStore{storagetest.New()}, "worm", 1, quietLogger()).nextDay(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	other, err := testPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Rollback(ctx) }()
	if _, err := other.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, wormLockKey); err != nil {
		t.Fatal(err)
	}

	store := &wormStore{Memory: storagetest.New()}
	e := newWORMExporter(tx, store, "worm", 1, quietLogger())
	e.now = func() time.Time { return day0.Add(49 * time.Hour) }
	e.run(ctx)
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_worm_exports`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if store.puts != 0 || n != 0 {
		t.Fatalf("com o lock em outra réplica nada sai: puts=%d marcas=%d", store.puts, n)
	}

	// Liberado o lock, a mesma réplica exporta no tick seguinte.
	_ = other.Rollback(ctx)
	e.run(ctx)
	if store.puts != 4 {
		t.Fatalf("lock livre: os dois dias pendentes saem: puts=%d", store.puts)
	}
}

func TestWORMExporterLoopTicksUntilCancelled(t *testing.T) {
	store := &wormStore{Memory: storagetest.New(), failBucket: true, failLock: true}
	e := newWORMExporter(&hookDB{DBTX: dbtest.Fail{}}, store, "worm", 1, quietLogger())
	e.interval = 5 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if err := e.loop(ctx); err != nil {
		t.Fatal(err)
	}

	// O construtor público entrega o loop; com o contexto já cancelado ele
	// não exporta nada (a primeira consulta falha) e retorna.
	pool := testPool(t)
	done, stop := context.WithCancel(context.Background())
	stop()
	if err := WORMExporter(pool, &wormStore{Memory: storagetest.New()}, "worm", 1, quietLogger())(done); err != nil {
		t.Fatal(err)
	}
}
