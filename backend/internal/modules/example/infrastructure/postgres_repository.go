// Package infrastructure implementa a persistência do módulo-modelo em
// PostgreSQL (queries parametrizadas via pgx).
package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/example/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

const cols = `id, title, description, status, created_at, updated_at`

// PostgresRepository implementa domain.Repository.
type PostgresRepository struct{}

// NewPostgresRepository cria o repositório.
func NewPostgresRepository() *PostgresRepository { return &PostgresRepository{} }

func scan(row pgx.Row) (domain.Item, error) {
	var it domain.Item
	err := row.Scan(&it.ID, &it.Title, &it.Description, &it.Status, &it.CreatedAt, &it.UpdatedAt)
	return it, err
}

// Insert grava o item no executor recebido (a transação de negócio).
func (r *PostgresRepository) Insert(ctx context.Context, db database.DBTX, it domain.Item) error {
	if _, err := db.Exec(ctx, `INSERT INTO example_items (`+cols+`) VALUES ($1, $2, $3, $4, $5, $6)`,
		it.ID, it.Title, it.Description, it.Status, it.CreatedAt, it.UpdatedAt); err != nil {
		return fmt.Errorf("example.Insert: %w", err)
	}
	return nil
}

// Get busca um item pelo id.
func (r *PostgresRepository) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Item, error) {
	it, err := scan(db.QueryRow(ctx, `SELECT `+cols+` FROM example_items WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Item{}, domain.ErrExampleNotFound
	}
	if err != nil {
		return domain.Item{}, fmt.Errorf("example.Get: %w", err)
	}
	return it, nil
}

// List pagina os itens (mais recentes primeiro). Nunca devolve nil: uma
// página vazia sai como [] no JSON.
func (r *PostgresRepository) List(ctx context.Context, db database.DBTX, p pagination.Params) ([]domain.Item, int64, error) {
	var total int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM example_items`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("example.List count: %w", err)
	}
	rows, err := db.Query(ctx, `SELECT `+cols+` FROM example_items ORDER BY created_at DESC, id LIMIT $1 OFFSET $2`, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("example.List: %w", err)
	}
	defer rows.Close()
	items := []domain.Item{}
	for rows.Next() {
		it, err := scan(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("example.List scan: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("example.List rows: %w", err)
	}
	return items, total, nil
}
