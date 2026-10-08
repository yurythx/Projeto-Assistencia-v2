package infrastructure

import (
	"context"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Avisos de nova versão (ADR 027): quem segue o procedimento e quem está
// lotado nas unidades que participam dele.

func (r *Repository) Seguir(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) error {
	_, err := db.Exec(ctx, `INSERT INTO atlas_seguidores (user_id, codigo_processual) VALUES ($1, $2) ON CONFLICT DO NOTHING`, usuario, codigo)
	return wrap(err)
}

func (r *Repository) DeixarDeSeguir(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) error {
	_, err := db.Exec(ctx, `DELETE FROM atlas_seguidores WHERE user_id = $1 AND codigo_processual = $2`, usuario, codigo)
	return wrap(err)
}

func (r *Repository) Seguindo(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM atlas_seguidores WHERE user_id = $1 AND codigo_processual = $2)`,
		usuario, codigo).Scan(&ok)
	return ok, wrap(err)
}

// Interessados: quem segue o código e quem está lotado (manualmente ou por
// grupo do AD) numa unidade ativa cuja sigla é uma das siglas do fluxo.
func (r *Repository) Interessados(ctx context.Context, db database.DBTX, codigo string, siglas []string) ([]uuid.UUID, error) {
	return usuarios(ctx, db, `SELECT user_id FROM atlas_seguidores WHERE codigo_processual = $1
		UNION
		SELECT us.user_id FROM user_scopes us JOIN unidades un ON un.id = us.unidade_id
			WHERE un.ativo AND un.sigla <> '' AND upper(un.sigla) = ANY($2)
		UNION
		SELECT u.id FROM users u
			JOIN ad_group_mappings m ON lower(m.ad_group) = ANY (SELECT lower(g) FROM unnest(u.groups) g)
			JOIN unidades un ON un.id = m.unidade_id
			WHERE un.ativo AND un.sigla <> '' AND upper(un.sigla) = ANY($2)`, codigo, siglas)
}

// InteressadosModelo: quem segue um procedimento em vigor que usa o modelo
// (numa peça ou pela série).
func (r *Repository) InteressadosModelo(ctx context.Context, db database.DBTX, modeloID uuid.UUID) ([]uuid.UUID, error) {
	return usuarios(ctx, db, `SELECT DISTINCT s.user_id FROM atlas_seguidores s
		JOIN atlas_workflows w ON w.codigo_processual = s.codigo_processual AND w.ativo
		WHERE EXISTS (SELECT 1 FROM atlas_etapa_documentos d JOIN atlas_etapas e ON e.id = d.etapa_id
				WHERE e.workflow_id = w.id AND d.modelo_id = $1)
			OR EXISTS (SELECT 1 FROM atlas_ttdd_modelos tm WHERE tm.codigo = w.codigo_ttdd AND tm.modelo_id = $1)`, modeloID)
}

func usuarios(ctx context.Context, db database.DBTX, sql string, args ...any) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, wrap(err)
		}
		out = append(out, id)
	}
	return out, wrap(rows.Err())
}
