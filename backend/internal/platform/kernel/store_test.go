package kernel

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestPostgresStoreRoundTripAndNotify(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	s := NewPostgresStore(pool, quiet())
	key := "kt_" + uuid.NewString()[:8]
	if err := s.Ensure(ctx, key, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Ensure(ctx, key, false); err != nil { // não sobrescreve
		t.Fatal(err)
	}
	if all, err := s.LoadAll(ctx); err != nil || !all[key] {
		t.Fatalf("Ensure preserva o estado existente: %v %v", all[key], err)
	}

	var changes atomic.Int32
	lctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- s.Listen(lctx, func() { changes.Add(1) }) }()
	waitFor(t, func() bool { return changes.Load() == 1 }) // recarga ao conectar
	if err := s.Set(ctx, key, false, "tester", audit.Entry{Action: audit.ActionModuleToggled, ResourceType: "module", ResourceID: key}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return changes.Load() >= 2 }) // NOTIFY do gatilho
	if all, _ := s.LoadAll(ctx); all[key] {
		t.Fatal("Set grava o novo estado")
	}

	// Conexão de LISTEN derrubada: reconecta e recarrega de novo.
	s.minBackoff = time.Millisecond
	before := changes.Load()
	// pg_stat_activity cobre o cluster todo: filtra este banco, para não
	// derrubar o LISTEN de outro ambiente (e não passar sem derrubar nada).
	var killed int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE pg_terminate_backend(pid)) FROM pg_stat_activity
		WHERE pid <> pg_backend_pid() AND datname = current_database() AND query = 'LISTEN '||$1`, ModulesChannel).Scan(&killed); err != nil || killed != 1 {
		t.Fatalf("derrubar o LISTEN do Kernel: %d conexões, %v", killed, err)
	}
	waitFor(t, func() bool { return changes.Load() > before })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStoreFailures(t *testing.T) {
	ctx := context.Background()
	closed := closedPool(t)
	s := NewPostgresStore(closed, quiet())
	if err := s.Ensure(ctx, "x", true); err == nil {
		t.Error("Ensure com o banco fora")
	}
	if _, err := s.LoadAll(ctx); err == nil {
		t.Error("LoadAll com o banco fora")
	}
	if err := s.Set(ctx, "x", true, "t", audit.Entry{}); err == nil {
		t.Error("Set com o banco fora")
	}
	s.db = &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.BadRows()}}}
	if _, err := s.LoadAll(ctx); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("linha ilegível: %v", err)
	}

	// Set: o upsert falha (auditoria nunca é gravada sozinha).
	pool := dbtest.Pool(t)
	bad := NewPostgresStore(pool, quiet())
	if err := bad.Set(ctx, "com\x00nul", true, "t", audit.Entry{}); err == nil {
		t.Error("upsert inválido")
	}

	// Listen: sem conexão, tenta com backoff crescente até o cancelamento.
	s.minBackoff, s.maxBackoff = time.Millisecond, 2*time.Millisecond
	lctx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	if err := s.Listen(lctx, func() { t.Error("sem conexão não há recarga") }); err != nil {
		t.Fatal(err)
	}
	// LISTEN recusado (canal inválido) também volta ao laço de reconexão.
	bad.channel = "canal inválido"
	bad.minBackoff = time.Hour
	l2, cancel2 := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel2()
	if err := bad.Listen(l2, func() { t.Error("LISTEN falhou: sem recarga") }); err != nil {
		t.Fatal(err)
	}
}
