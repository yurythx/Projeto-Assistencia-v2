package database

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
)

func testConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(u.Port())
	pw, _ := u.User.Password()
	return config.DatabaseConfig{Host: u.Hostname(), Port: port, Name: u.Path[1:], User: u.User.Username(), Password: pw,
		SSLMode: "disable", MaxConns: 4, MinConns: 0, MaxConnLifetime: time.Minute, MaxConnIdleTime: time.Minute, ConnectTimeout: 2 * time.Second}
}

func TestNewPingAndWithTx(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	pool, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Ping(pool)(ctx); err != nil {
		t.Fatal(err)
	}

	// Commit e rollback.
	if err := WithTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT 1`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("regra de negócio")
	if err := WithTx(ctx, pool, func(context.Context, pgx.Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("erro de fn volta intacto: %v", err)
	}
	// Commit falha (contexto cancelado dentro de fn).
	c, cancel := context.WithCancel(ctx)
	if err := WithTx(c, pool, func(context.Context, pgx.Tx) error { cancel(); return nil }); err == nil {
		t.Fatal("commit com contexto cancelado")
	}
	// Rollback falha também: os dois erros aparecem.
	c2, cancel2 := context.WithCancel(ctx)
	if err := WithTx(c2, pool, func(context.Context, pgx.Tx) error { cancel2(); return boom }); !errors.Is(err, boom) {
		t.Fatalf("erro original preservado junto da falha de rollback: %v", err)
	}
	// Panic: reverte e repropaga.
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic repropagado")
			}
		}()
		_ = WithTx(ctx, pool, func(context.Context, pgx.Tx) error { panic("bug") })
	}()
	// Begin falha com o banco fora.
	pool.Close()
	if err := WithTx(ctx, pool, func(context.Context, pgx.Tx) error { return nil }); err == nil {
		t.Fatal("begin com o pool fechado")
	}
}

func TestNewFailsFast(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	bad := cfg
	bad.Port = 1 // ninguém escuta
	if _, err := New(ctx, bad); err == nil {
		t.Fatal("ping inicial falha")
	}
	bad = cfg
	bad.SSLMode = "banana"
	if _, err := New(ctx, bad); err == nil {
		t.Fatal("DSN inválido (sslmode desconhecido)")
	}
	bad = cfg
	bad.MaxConns = 0 // pgxpool recusa
	if _, err := New(ctx, bad); err == nil {
		t.Fatal("configuração de pool inválida")
	}
}

func TestErrorClassifiers(t *testing.T) {
	for code, fn := range map[string]func(error) bool{
		"23505": IsUniqueViolation, "23503": IsForeignKeyViolation, "23P01": IsExclusionViolation, "23514": IsCheckViolation,
	} {
		if !fn(&pgconn.PgError{Code: code}) || fn(&pgconn.PgError{Code: "00000"}) || fn(errors.New("x")) {
			t.Errorf("classificador de %s", code)
		}
	}
	if !IsNoRows(pgx.ErrNoRows) || IsNoRows(errors.New("x")) {
		t.Error("IsNoRows")
	}
}
