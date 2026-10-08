package publico

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

var blog = Tabela{Nome: "blog_post_publico", Coluna: "post_id"}

// Grava, carrega e filtra o público-alvo de verdade (ADR 014): entidade ou
// unidade (a do leitor ou uma acima dela); sem público = todos.
func TestTabelaGravaCarregaEFiltra(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	mae := dbtest.Unidade(t, pool)
	var ent, filha uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT entidade_id FROM unidades WHERE id = $1`, mae).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, parent_id, nome, slug) VALUES ($1, $2, 'Filha', $3) RETURNING id`,
		ent, mae, uuid.NewString()).Scan(&filha); err != nil {
		t.Fatal(err)
	}
	novo := func() uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO blog_posts (slug, title) VALUES ($1, 'Post') RETURNING id`, uuid.NewString()).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	paraTodos, daMae, daEntidade := novo(), novo(), novo()
	if err := blog.Gravar(ctx, pool, daMae, auth.Publico{Unidades: []uuid.UUID{mae, mae, uuid.Nil}}); err != nil {
		t.Fatal(err)
	}
	if err := blog.Gravar(ctx, pool, daEntidade, auth.Publico{Entidades: []uuid.UUID{ent}}); err != nil {
		t.Fatal(err)
	}
	if err := blog.Gravar(ctx, pool, paraTodos, auth.Publico{}); err != nil {
		t.Fatal(err)
	}
	if err := blog.Gravar(ctx, pool, paraTodos, auth.Publico{Unidades: []uuid.UUID{uuid.New()}}); !errors.Is(err, ErrInvalido) {
		t.Fatalf("unidade inexistente: %v", err)
	}
	m, err := blog.Carregar(ctx, pool, []uuid.UUID{paraTodos, daMae, daEntidade})
	if err != nil || !m[paraTodos].Vazio() || !slices.Equal(m[daMae].Unidades, []uuid.UUID{mae}) || !slices.Equal(m[daEntidade].Entidades, []uuid.UUID{ent}) {
		t.Fatalf("carregar: %+v %v", m, err)
	}
	if todos, err := blog.Todos(ctx, pool); err != nil || todos[daMae].Vazio() || !todos[paraTodos].Vazio() {
		t.Fatalf("todos: %v", err)
	}
	if p, err := blog.Um(ctx, pool, daMae); err != nil || len(p.Unidades) != 1 {
		t.Fatalf("um: %+v %v", p, err)
	}
	if m, err := blog.Carregar(ctx, pool, nil); err != nil || len(m) != 0 {
		t.Fatalf("sem ids: %v %v", m, err)
	}

	veem := func(l Leitor) []uuid.UUID {
		args := []any{[]string{paraTodos.String(), daMae.String(), daEntidade.String()}}
		rows, err := pool.Query(ctx, `SELECT p.id FROM blog_posts p WHERE p.id = ANY($1::uuid[]) AND `+blog.Cond("p.id", l, &args), args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := []uuid.UUID{}
		for rows.Next() {
			var id uuid.UUID
			_ = rows.Scan(&id)
			out = append(out, id)
		}
		return out
	}
	// Lotado na filha: pertence à mãe (acima) e à entidade.
	var acima []uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT nexus_unidades_acima($1)`, filha).Scan(&acima); err != nil || len(acima) != 2 {
		t.Fatalf("unidades acima da filha: %v %v", acima, err)
	}
	naFilha := LeitorDe(auth.Identity{Scopes: []auth.Scope{{EntidadeID: &ent, UnidadeID: &filha, Acima: acima}}})
	if got := veem(naFilha); len(got) != 3 {
		t.Errorf("lotado na filha vê os três: %v", got)
	}
	if got := veem(Leitor{}); !slices.Equal(got, []uuid.UUID{paraTodos}) {
		t.Errorf("sem lotação, só o de todos: %v", got)
	}
	// Leitura: além do público, o autor e a gestão (global ou da dona).
	if _, err := pool.Exec(ctx, `UPDATE blog_posts SET unidade_id = $2 WHERE id = $1`, daEntidade, mae); err != nil {
		t.Fatal(err)
	}
	autor := dbtest.User(t, pool)
	if _, err := pool.Exec(ctx, `UPDATE blog_posts SET author_id = $2 WHERE id = $1`, daMae, autor); err != nil {
		t.Fatal(err)
	}
	le := func(l Leitura, autorCol, donaCol string) int {
		args := []any{[]string{paraTodos.String(), daMae.String(), daEntidade.String()}}
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM blog_posts p WHERE p.id = ANY($1::uuid[]) AND `+blog.Condicao("p.id", autorCol, donaCol, l, &args), args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := le(Leitura{Tudo: true}, "", ""); n != 3 {
		t.Errorf("gestão global vê tudo: %d", n)
	}
	if n := le(Leitura{Autor: autor}, "p.author_id", ""); n != 2 {
		t.Errorf("autor vê o seu (e o de todos): %d", n)
	}
	if n := le(Leitura{Geridas: []uuid.UUID{mae}}, "p.author_id", "p.unidade_id"); n != 2 {
		t.Errorf("gestão da dona vê o dela (e o de todos): %d", n)
	}
	var semPublico int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blog_posts p WHERE p.id = ANY($1::uuid[]) AND `+blog.SemPublico("p.id"),
		[]string{paraTodos.String(), daMae.String()}).Scan(&semPublico); err != nil || semPublico != 1 {
		t.Errorf("sem público: %d %v", semPublico, err)
	}
}

// O banco fora ou ilegível chega a quem chamou.
func TestTabelaPropagaFalhas(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	if _, err := blog.Carregar(ctx, dbtest.Fail{}, []uuid.UUID{id}); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("carregar: %v", err)
	}
	if _, err := blog.Carregar(ctx, dbtest.ScanFail{}, []uuid.UUID{id}); err == nil {
		t.Error("linha ilegível")
	}
	if _, err := blog.Carregar(ctx, dbtest.RowsErr{}, []uuid.UUID{id}); err == nil {
		t.Error("erro na leitura")
	}
	if _, err := blog.Todos(ctx, dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("todos: %v", err)
	}
	if _, err := blog.Um(ctx, dbtest.Fail{}, id); err == nil {
		t.Error("um")
	}
	if err := blog.Gravar(ctx, dbtest.Fail{}, id, auth.Publico{}); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("limpar: %v", err)
	}
	if err := blog.Gravar(ctx, &insertFalha{}, id, auth.Publico{Unidades: []uuid.UUID{id}}); !errors.Is(err, dbtest.ErrInjected) || errors.Is(err, ErrInvalido) {
		t.Errorf("inserir: %v", err)
	}
}

// insertFalha: o DELETE passa, o INSERT falha (sem ser chave estrangeira).
type insertFalha struct {
	dbtest.Fail
	n int
}

func (f *insertFalha) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.n++
	if f.n == 1 {
		return pgconn.CommandTag{}, nil
	}
	return f.Fail.Exec(ctx, sql, args...)
}
