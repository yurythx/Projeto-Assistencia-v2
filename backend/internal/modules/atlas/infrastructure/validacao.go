package infrastructure

import (
	"context"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Validação dos procedimentos (ADR 028).

func (r *Repository) SetSituacao(ctx context.Context, db database.DBTX, id uuid.UUID, situacao string, ativo bool) error {
	tag, err := db.Exec(ctx, `UPDATE atlas_workflows SET situacao = $2, ativo = $3 WHERE id = $1`, id, situacao, ativo)
	if err != nil {
		return wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) Validacoes(ctx context.Context, db database.DBTX, codigo string) ([]domain.Validacao, error) {
	rows, err := db.Query(ctx, `SELECT id, codigo_processual, realizada_em, unidade, participantes, registro, pendencias,
		created_at, created_by FROM atlas_validacoes WHERE codigo_processual = $1 ORDER BY realizada_em DESC, created_at DESC`, codigo)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Validacao{}
	for rows.Next() {
		var v domain.Validacao
		if err := rows.Scan(&v.ID, &v.CodigoProcessual, &v.RealizadaEm, &v.Unidade, &v.Participantes, &v.Registro, &v.Pendencias,
			&v.CreatedAt, &v.CreatedBy); err != nil {
			return nil, wrap(err)
		}
		out = append(out, v)
	}
	return out, wrap(rows.Err())
}

func (r *Repository) InsertValidacao(ctx context.Context, db database.DBTX, v domain.Validacao) error {
	_, err := db.Exec(ctx, `INSERT INTO atlas_validacoes (id, codigo_processual, realizada_em, unidade, participantes, registro,
		pendencias, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		v.ID, v.CodigoProcessual, v.RealizadaEm, v.Unidade, v.Participantes, v.Registro, v.Pendencias, v.CreatedBy)
	return wrap(err)
}
