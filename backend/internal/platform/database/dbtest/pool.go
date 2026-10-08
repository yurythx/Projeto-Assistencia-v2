package dbtest

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool conecta ao banco de teste (TEST_DATABASE_URL, já migrado) ou pula o
// teste quando ele não está configurado.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// User cria um usuário local e devolve o id.
func User(t testing.TB, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	name := "u_" + uuid.NewString()[:12]
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, display_name, keycloak_subject) VALUES ($1, $1 || '@nexus.test', 'Usuário ' || $1, $1) RETURNING id`,
		name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// Unidade cria uma entidade com uma unidade e devolve o id da unidade.
func Unidade(t testing.TB, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	slug := "t-" + uuid.NewString()[:12]
	var ent, un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, slug).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, slug).Scan(&un); err != nil {
		t.Fatal(err)
	}
	return un
}
