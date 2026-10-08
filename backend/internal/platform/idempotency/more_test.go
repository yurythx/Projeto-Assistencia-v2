package idempotency

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// errStore falha na operação escolhida.
type errStore struct {
	*fakeStore
	claimErr, completeErr, failErr error
}

func (s *errStore) Claim(ctx context.Context, key, hash string) (*Record, bool, error) {
	if s.claimErr != nil {
		return nil, false, s.claimErr
	}
	return s.fakeStore.Claim(ctx, key, hash)
}

func (s *errStore) Complete(ctx context.Context, key string, st int, b []byte, ct string) error {
	if s.completeErr != nil {
		return s.completeErr
	}
	return s.fakeStore.Complete(ctx, key, st, b, ct)
}

func (s *errStore) Fail(ctx context.Context, key string) error {
	if s.failErr != nil {
		return s.failErr
	}
	return s.fakeStore.Fail(ctx, key)
}

func req(method, target, key, body string, subject string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if key != "" {
		r.Header.Set(LegacyHeader, key)
	}
	if subject != "" {
		r = withIdentity(r, subject)
	}
	return r
}

func TestMiddlewareEdgeCases(t *testing.T) {
	boom := errors.New("banco fora")
	calls := 0
	h := func(s Store, status int, body string) http.Handler {
		return Middleware(s, testLogger())(countingHandler(&calls, status, body))
	}
	serve := func(handler http.Handler, r *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec
	}

	// Sem identidade: passa direto (nunca aplica sem escopar por usuário).
	if rec := serve(h(newFakeStore(), 201, "{}"), req("POST", "/x", "k1", "{}", "")); rec.Code != 201 || calls != 1 {
		t.Fatalf("sem identidade: %d %d", rec.Code, calls)
	}
	// Corpo acima do limite: 400.
	big := strings.Repeat("a", 2<<20)
	if rec := serve(h(newFakeStore(), 201, "{}"), req("POST", "/x", "k2", big, "u")); rec.Code != http.StatusBadRequest {
		t.Fatalf("corpo grande: %d", rec.Code)
	}
	// Store fora: fail-open.
	if rec := serve(h(&errStore{fakeStore: newFakeStore(), claimErr: boom}, 201, "{}"), req("POST", "/x", "k3", "{}", "u")); rec.Code != 201 {
		t.Fatalf("fail-open: %d", rec.Code)
	}
	// Falhas ao finalizar só são registradas: a resposta segue.
	for name, s := range map[string]Store{
		"complete falha": &errStore{fakeStore: newFakeStore(), completeErr: boom},
	} {
		if rec := serve(h(s, 201, "{}"), req("POST", "/x", "k4", "{}", "u")); rec.Code != 201 {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if rec := serve(h(&errStore{fakeStore: newFakeStore(), failErr: boom}, 500, "{}"), req("POST", "/x", "k5", "{}", "u")); rec.Code != 500 {
		t.Fatalf("liberar após 500 falha: %d", rec.Code)
	}
	// Resposta grande demais para o replay: a chave é liberada.
	large := strings.Repeat("b", MaxCachedResponseBytes+1)
	fs := newFakeStore()
	serve(h(fs, 200, large), req("POST", "/x", "k6", "{}", "u"))
	if rec := serve(h(fs, 200, large), req("POST", "/x", "k6", "{}", "u")); rec.Header().Get("Idempotent-Replay") == "true" {
		t.Fatal("resposta grande não é guardada para replay")
	}
	serve(h(&errStore{fakeStore: newFakeStore(), failErr: boom}, 200, large), req("POST", "/x", "k7", "{}", "u"))

	// A query string faz parte da requisição: mesma chave, query diferente = reuso indevido.
	fs = newFakeStore()
	serve(h(fs, 200, "{}"), req("DELETE", "/pastas/1", "k8", "", "u"))
	if rec := serve(h(fs, 200, "{}"), req("DELETE", "/pastas/1?recursive=true", "k8", "", "u")); rec.Code != http.StatusConflict {
		t.Fatalf("query diferente com a mesma chave: %d", rec.Code)
	}
}

func TestResponseRecorderKeepsFirstStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	r := &responseRecorder{ResponseWriter: rec}
	r.WriteHeader(201)
	r.WriteHeader(500)
	if r.status != 201 || rec.Code != 201 {
		t.Fatalf("status: %d %d", r.status, rec.Code)
	}
	implicit := &responseRecorder{ResponseWriter: httptest.NewRecorder()}
	_, _ = implicit.Write([]byte("ok"))
	if implicit.status != http.StatusOK || implicit.body.String() != "ok" {
		t.Fatalf("Write sem WriteHeader assume 200: %d", implicit.status)
	}
}

func TestAbandonedProcessingKeyIsReclaimed(t *testing.T) {
	pool := testPool(t)
	s := NewPostgresStore(pool)
	ctx := context.Background()
	key := "abandonada-" + uuid.NewString()
	if _, claimed, err := s.Claim(ctx, key, "h"); err != nil || !claimed {
		t.Fatal(err)
	}
	if _, claimed, _ := s.Claim(ctx, key, "h"); claimed {
		t.Fatal("em andamento de verdade: não retoma")
	}
	if _, err := pool.Exec(ctx, `UPDATE idempotency_keys SET updated_at = now() - interval '6 minutes' WHERE key = $1`, key); err != nil {
		t.Fatal(err)
	}
	if _, claimed, _ := s.Claim(ctx, key, "outro-hash"); claimed {
		t.Fatal("abandonada, mas com outro payload: não retoma")
	}
	if _, claimed, err := s.Claim(ctx, key, "h"); err != nil || !claimed {
		t.Fatalf("chave abandonada é retomada pelo retry: %v %v", claimed, err)
	}
}

func TestStoreAndCleanupFailures(t *testing.T) {
	ctx := context.Background()
	s := &PostgresStore{db: dbtest.Fail{}}
	if _, _, err := s.Claim(ctx, "k", "h"); err == nil {
		t.Error("claim")
	}
	if err := s.Complete(ctx, "k", 200, nil, ""); err == nil {
		t.Error("complete")
	}
	if err := s.Fail(ctx, "k"); err == nil {
		t.Error("fail")
	}
	if err := cleanup(dbtest.Fail{}, time.Millisecond)(ctx); err == nil {
		t.Error("limpeza com o banco fora devolve erro (o supervisor reinicia)")
	}

	pool := testPool(t)
	key := "velha-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO idempotency_keys (key, request_hash, status, created_at, updated_at)
		VALUES ($1, 'h', 'completed', now() - interval '2 days', now() - interval '2 days')`, key); err != nil {
		t.Fatal(err)
	}
	c, cancel := context.WithCancel(ctx)
	var done atomic.Bool
	go func() { _ = cleanup(pool, time.Millisecond)(c); done.Store(true) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys WHERE key = $1`, key).Scan(&n)
		if n == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	for !done.Load() {
		time.Sleep(time.Millisecond)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys WHERE key = $1`, key).Scan(&n)
	if n != 0 {
		t.Fatal("chaves com mais de 24h são apagadas")
	}
	if Cleanup(pool) == nil {
		t.Fatal("construtor público")
	}
}
