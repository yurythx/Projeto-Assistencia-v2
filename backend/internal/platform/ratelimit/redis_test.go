package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/yurythx/projeto-nexus/internal/platform/redisx"
)

func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestRedisLimiterFixedWindow(t *testing.T) {
	client, _ := newTestRedis(t)
	l := NewRedisLimiter(client, 60, 3, "test")
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		ok, err := l.Allow(ctx, "10.0.0.1")
		if err != nil || !ok {
			t.Fatalf("requisição %d deveria passar (ok=%v err=%v)", i, ok, err)
		}
	}
	if ok, _ := l.Allow(ctx, "10.0.0.1"); ok {
		t.Fatal("4ª requisição na janela deveria ser barrada")
	}
	if ok, _ := l.Allow(ctx, "10.0.0.2"); !ok {
		t.Fatal("outra chave não pode ser afetada")
	}
}

func TestRedisLimiterAdaptivePenalty(t *testing.T) {
	client, _ := newTestRedis(t)
	l := NewRedisLimiter(client, 60, 8, "adaptive")
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := l.Penalize(ctx, "ip"); err != nil {
			t.Fatal(err)
		}
	}
	allowed := 0
	for i := 0; i < 8; i++ {
		if ok, _ := l.Allow(ctx, "ip"); ok {
			allowed++
		}
	}
	if allowed != 4 {
		t.Fatalf("com penalidade 3 o teto deveria cair de 8 para 4, passaram %d", allowed)
	}
}

func TestEffectiveLimit(t *testing.T) {
	cases := []struct{ max, penalty, want int }{
		{10, 0, 10}, {10, 2, 10}, {10, 3, 5}, {10, 6, 2}, {10, 30, 1},
	}
	for _, c := range cases {
		if got := EffectiveLimit(c.max, c.penalty); got != c.want {
			t.Errorf("EffectiveLimit(%d,%d)=%d, want %d", c.max, c.penalty, got, c.want)
		}
	}
}

func TestProgressiveDelay(t *testing.T) {
	base, max := time.Minute, time.Hour
	cases := []struct {
		n    int
		want time.Duration
	}{
		{4, 0}, {5, time.Minute}, {6, 2 * time.Minute}, {7, 4 * time.Minute}, {12, time.Hour}, {99, time.Hour},
	}
	for _, c := range cases {
		if got := ProgressiveDelay(c.n, 5, base, max); got != c.want {
			t.Errorf("ProgressiveDelay(%d)=%v, want %v", c.n, got, c.want)
		}
	}
}

func TestLockoutLifecycle(t *testing.T) {
	client, _ := newTestRedis(t)
	lo := NewLockout(client, 2, time.Minute, time.Hour)
	ctx := context.Background()

	if d, _ := lo.RegisterFailure(ctx, "user:ana"); d != 0 {
		t.Fatalf("1ª falha não bloqueia, veio %v", d)
	}
	d, err := lo.RegisterFailure(ctx, "user:ana")
	if err != nil || d != time.Minute {
		t.Fatalf("2ª falha deveria bloquear 1min (d=%v err=%v)", d, err)
	}
	if left, _ := lo.LockedFor(ctx, "user:ana"); left <= 0 {
		t.Fatal("conta deveria estar bloqueada")
	}
	if err := lo.Reset(ctx, "user:ana"); err != nil {
		t.Fatal(err)
	}
	if left, _ := lo.LockedFor(ctx, "user:ana"); left != 0 {
		t.Fatal("reset deveria liberar a conta")
	}
}

// IP atrás de NAT: limite próprio (mais alto, janela curta); a conta segue
// com o limite rígido e o contador do IP expira na janela dele.
func TestLockoutPolicyPerSubjectKind(t *testing.T) {
	client, mr := newTestRedis(t)
	lo := NewLockout(client, 2, time.Minute, time.Hour).
		WithPolicy("ip:", LockoutPolicy{Threshold: 4, Base: time.Minute, Max: 10 * time.Minute, Window: time.Hour})
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		if d, _ := lo.RegisterFailure(ctx, "ip:10.0.0.1"); d != 0 {
			t.Fatalf("falha %d do IP não bloqueia (limite 4), veio %v", i, d)
		}
	}
	if d, _ := lo.RegisterFailure(ctx, "ip:10.0.0.1"); d != time.Minute {
		t.Fatalf("4ª falha do IP bloqueia 1min, veio %v", d)
	}
	lo.RegisterFailure(ctx, "user:ana")
	if d, _ := lo.RegisterFailure(ctx, "user:ana"); d != time.Minute {
		t.Fatalf("conta segue com o limite padrão (2), veio %v", d)
	}
	for i := 0; i < 10; i++ {
		lo.RegisterFailure(ctx, "ip:10.0.0.2")
	}
	if left, _ := lo.LockedFor(ctx, "ip:10.0.0.2"); left > 10*time.Minute {
		t.Fatalf("bloqueio do IP limitado ao máximo dele, veio %v", left)
	}
	if ttl := mr.TTL(redisx.Key("lockout", "fails", "ip:10.0.0.1")); ttl <= 0 || ttl > time.Hour {
		t.Fatalf("contador do IP expira na janela de 1h, TTL=%v", ttl)
	}
	if ttl := mr.TTL(redisx.Key("lockout", "fails", "user:ana")); ttl != 24*time.Hour {
		t.Fatalf("contador da conta segue com a janela de 24h, TTL=%v", ttl)
	}

	// Política sem janela herda a janela padrão (24h).
	lo.WithPolicy("svc:", LockoutPolicy{Threshold: 1, Base: time.Minute, Max: time.Minute})
	if d, _ := lo.RegisterFailure(ctx, "svc:x"); d != time.Minute {
		t.Fatalf("limite próprio do prefixo svc: (1), veio %v", d)
	}
	if ttl := mr.TTL(redisx.Key("lockout", "fails", "svc:x")); ttl != 24*time.Hour {
		t.Fatalf("sem Window, herda 24h: TTL=%v", ttl)
	}
}

// failOn faz o comando de nome cmd falhar (os demais seguem para o Redis).
type failOn struct{ cmd string }

func (f failOn) DialHook(next redis.DialHook) redis.DialHook { return next }
func (f failOn) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, c redis.Cmder) error {
		if c.Name() == f.cmd {
			c.SetErr(errors.New("falha injetada"))
			return c.Err()
		}
		return next(ctx, c)
	}
}
func (f failOn) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestRedisFailuresArePropagated(t *testing.T) {
	ctx := context.Background()
	client, mr := newTestRedis(t)
	l := NewRedisLimiter(client, 0, 0, "b") // padrões: 60s e 1 requisição
	if l.window != time.Minute || l.maxRequests != 1 {
		t.Fatalf("padrões: %v %d", l.window, l.maxRequests)
	}
	lock := NewLockout(client, 1, time.Minute, time.Hour)

	// SET falha depois do INCR: o bloqueio calculado não é aplicado e o erro sobe.
	withHook := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	withHook.AddHook(failOn{cmd: "set"})
	if _, err := NewLockout(withHook, 1, time.Minute, time.Hour).RegisterFailure(ctx, "ana"); err == nil {
		t.Fatal("falha ao gravar o bloqueio")
	}
	evalFails := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	evalFails.AddHook(failOn{cmd: "evalsha"})
	evalFails.AddHook(failOn{cmd: "eval"})
	if _, err := NewRedisLimiter(evalFails, 60, 5, "b").Allow(ctx, "k"); err == nil {
		t.Fatal("falha no script da janela")
	}

	mr.Close() // Redis fora
	if _, err := l.Allow(ctx, "k"); err == nil {
		t.Error("Allow com o Redis fora")
	}
	if _, err := lock.LockedFor(ctx, "ana"); err == nil {
		t.Error("LockedFor com o Redis fora")
	}
	if _, err := lock.RegisterFailure(ctx, "ana"); err == nil {
		t.Error("RegisterFailure com o Redis fora")
	}
}

func TestForgiveClearsPenalty(t *testing.T) {
	ctx := context.Background()
	client, _ := newTestRedis(t)
	l := NewRedisLimiter(client, 60, 8, "login")
	for i := 0; i < 6; i++ {
		if err := l.Penalize(ctx, "ip"); err != nil {
			t.Fatal(err)
		}
	}
	if p, _ := l.penalty(ctx, "ip"); p != 6 {
		t.Fatalf("penalidade acumulada: %d", p)
	}
	if err := l.Forgive(ctx, "ip"); err != nil {
		t.Fatal(err)
	}
	if p, _ := l.penalty(ctx, "ip"); p != 0 {
		t.Fatalf("login bem-sucedido zera a penalidade: %d", p)
	}
}
