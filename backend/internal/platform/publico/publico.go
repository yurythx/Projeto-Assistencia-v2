// Package publico guarda e aplica o público-alvo de conteúdos (ADR 014):
// cada módulo tem a própria tabela "<recurso>_publico" (recurso,
// entidade_id, unidade_id) e usa daqui o filtro SQL, a carga e a gravação.
//
// Tabela e coluna são constantes dos módulos, nunca entrada do usuário.
package publico

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// ErrInvalido: entidade ou unidade do público inexistente.
var ErrInvalido = errors.New("publico: entidade ou unidade do público-alvo inexistente")

// Tabela descreve a tabela de público de um módulo.
type Tabela struct {
	Nome   string // ex.: "blog_post_publico"
	Coluna string // ex.: "post_id"
}

// Leitor é onde quem lê pertence (auth.Pertencimento).
type Leitor struct {
	Entidades, Unidades []uuid.UUID
}

// LeitorDe monta o leitor a partir da identidade.
func LeitorDe(identity auth.Identity) Leitor {
	e, u := auth.Pertencimento(identity)
	return Leitor{Entidades: e, Unidades: u}
}

func strings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// Cond devolve a condição SQL "o leitor está no público do recurso ref"
// (sem público = todos) e acrescenta os argumentos a args.
func (t Tabela) Cond(ref string, l Leitor, args *[]any) string {
	n := len(*args)
	*args = append(*args, strings(l.Entidades), strings(l.Unidades))
	return fmt.Sprintf(`(NOT EXISTS (SELECT 1 FROM %[1]s x WHERE x.%[2]s = %[3]s)
		OR EXISTS (SELECT 1 FROM %[1]s x WHERE x.%[2]s = %[3]s
		           AND (x.entidade_id = ANY($%[4]d::uuid[]) OR x.unidade_id = ANY($%[5]d::uuid[]))))`,
		t.Nome, t.Coluna, ref, n+1, n+2)
}

// Leitura é quem lê e o que a gestão dele cobre: vê o conteúdo quem está
// no público, o autor, e a gestão (global, ou com escopo na unidade dona).
type Leitura struct {
	Leitor
	Autor uuid.UUID
	// Tudo: gestão global do módulo (vê todo conteúdo).
	Tudo bool
	// Geridas: unidades que a gestão com escopo cobre (dona do conteúdo).
	Geridas []uuid.UUID
}

// Condicao devolve a condição SQL de Leitura sobre o recurso ref, com as
// colunas do autor e da unidade dona ("" = o módulo não tem a coluna).
func (t Tabela) Condicao(ref, autor, dona string, l Leitura, args *[]any) string {
	*args = append(*args, l.Tudo)
	c := fmt.Sprintf("($%d::boolean OR %s", len(*args), t.Cond(ref, l.Leitor, args))
	if autor != "" {
		*args = append(*args, l.Autor)
		c += fmt.Sprintf(" OR %s = $%d", autor, len(*args))
	}
	if dona != "" {
		*args = append(*args, strings(l.Geridas))
		c += fmt.Sprintf(" OR %s = ANY($%d::uuid[])", dona, len(*args))
	}
	return c + ")"
}

// SemPublico devolve a condição SQL "o recurso ref é para todos" (sem
// público-alvo) — o que pode aparecer no site público.
func (t Tabela) SemPublico(ref string) string {
	return fmt.Sprintf(`NOT EXISTS (SELECT 1 FROM %s x WHERE x.%s = %s)`, t.Nome, t.Coluna, ref)
}

// Carregar devolve o público de cada recurso (os sem linhas ficam vazios).
func (t Tabela) Carregar(ctx context.Context, db database.DBTX, ids []uuid.UUID) (map[uuid.UUID]auth.Publico, error) {
	out := make(map[uuid.UUID]auth.Publico, len(ids))
	for _, id := range ids {
		out[id] = auth.Publico{}.Normalizado()
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT %s, entidade_id, unidade_id FROM %s WHERE %s = ANY($1::uuid[])`,
		t.Coluna, t.Nome, t.Coluna), strings(ids))
	if err != nil {
		return nil, fmt.Errorf("publico: carregar: %w", err)
	}
	return out, acumular(rows, out)
}

// Todos devolve o público de todo recurso que tem um (os demais ficam de
// fora do mapa: vazio = todos).
func (t Tabela) Todos(ctx context.Context, db database.DBTX) (map[uuid.UUID]auth.Publico, error) {
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT %s, entidade_id, unidade_id FROM %s`, t.Coluna, t.Nome))
	if err != nil {
		return nil, fmt.Errorf("publico: carregar: %w", err)
	}
	out := map[uuid.UUID]auth.Publico{}
	return out, acumular(rows, out)
}

// acumular lê as linhas (recurso, entidade, unidade) para out.
func acumular(rows pgx.Rows, out map[uuid.UUID]auth.Publico) error {
	defer rows.Close()
	for rows.Next() {
		var (
			id                uuid.UUID
			entidade, unidade *uuid.UUID
		)
		if err := rows.Scan(&id, &entidade, &unidade); err != nil {
			return fmt.Errorf("publico: ler: %w", err)
		}
		p := out[id]
		if entidade != nil {
			p.Entidades = append(p.Entidades, *entidade)
		}
		if unidade != nil {
			p.Unidades = append(p.Unidades, *unidade)
		}
		out[id] = p
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("publico: ler: %w", err)
	}
	for id, p := range out {
		out[id] = p.Normalizado()
	}
	return nil
}

// Um devolve o público de um recurso.
func (t Tabela) Um(ctx context.Context, db database.DBTX, id uuid.UUID) (auth.Publico, error) {
	m, err := t.Carregar(ctx, db, []uuid.UUID{id})
	if err != nil {
		return auth.Publico{}, err
	}
	return m[id], nil
}

// Gravar substitui o público do recurso (na transação de quem chama).
func (t Tabela) Gravar(ctx context.Context, db database.DBTX, id uuid.UUID, p auth.Publico) error {
	p = p.Normalizado()
	if _, err := db.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s = $1`, t.Nome, t.Coluna), id); err != nil {
		return fmt.Errorf("publico: limpar: %w", err)
	}
	if p.Vazio() {
		return nil
	}
	_, err := db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s (%[2]s, entidade_id, unidade_id)
		SELECT $1::uuid, e, NULL::uuid FROM unnest($2::uuid[]) e
		UNION ALL
		SELECT $1::uuid, NULL::uuid, u FROM unnest($3::uuid[]) u`, t.Nome, t.Coluna), id, strings(p.Entidades), strings(p.Unidades))
	if database.IsForeignKeyViolation(err) {
		return ErrInvalido
	}
	if err != nil {
		return fmt.Errorf("publico: gravar: %w", err)
	}
	return nil
}
