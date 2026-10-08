package infrastructure

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// ------------------------------------------------- biblioteca de modelos

// wrapModelo traduz os erros do catálogo de modelos (ADR 024).
func wrapModelo(err error) error {
	switch {
	case database.IsNoRows(err):
		return domain.ErrModeloNaoEncontrado
	case database.IsUniqueViolation(err):
		return domain.ErrModeloRepetido
	}
	return wrap(err)
}

// modeloSelect: o modelo, a versão atual (a de maior número) e quantas
// peças de procedimentos ativos o usam.
const modeloSelect = `SELECT m.id, m.nome, m.descricao, m.ativo, m.updated_at, m.updated_by,
	v.versao, v.arquivo_nome, v.content_type, v.tamanho, v.sha256, v.nota, v.created_at, v.created_by, v.objeto,
	(SELECT count(*) FROM atlas_etapa_documentos d JOIN atlas_etapas e ON e.id = d.etapa_id
		JOIN atlas_workflows w ON w.id = e.workflow_id WHERE d.modelo_id = m.id AND w.ativo)
	FROM atlas_modelos m
	JOIN LATERAL (SELECT * FROM atlas_modelo_versoes WHERE modelo_id = m.id ORDER BY versao DESC LIMIT 1) v ON TRUE`

func scanModelo(row pgx.Row) (domain.Modelo, error) {
	var m domain.Modelo
	v := &m.Atual
	err := row.Scan(&m.ID, &m.Nome, &m.Descricao, &m.Ativo, &m.UpdatedAt, &m.UpdatedBy,
		&v.Versao, &v.ArquivoNome, &v.ContentType, &v.Tamanho, &v.SHA256, &v.Nota, &v.CreatedAt, &v.CreatedBy, &v.Objeto,
		&m.PecasLigadas)
	return m, err
}

func (r *Repository) ListModelos(ctx context.Context, db database.DBTX, incluirInativos bool) ([]domain.Modelo, error) {
	rows, err := db.Query(ctx, modeloSelect+` WHERE $1 OR m.ativo ORDER BY lower(m.nome)`, incluirInativos)
	if err != nil {
		return nil, wrapModelo(err)
	}
	defer rows.Close()
	out := []domain.Modelo{}
	for rows.Next() {
		m, err := scanModelo(rows)
		if err != nil {
			return nil, wrapModelo(err)
		}
		out = append(out, m)
	}
	return out, wrapModelo(rows.Err())
}

// GetModelo devolve o modelo com o histórico de versões (da mais recente
// à mais antiga).
func (r *Repository) GetModelo(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Modelo, error) {
	m, err := scanModelo(db.QueryRow(ctx, modeloSelect+` WHERE m.id = $1`, id))
	if err != nil {
		return m, wrapModelo(err)
	}
	rows, err := db.Query(ctx, `SELECT versao, arquivo_nome, content_type, tamanho, sha256, nota, created_at, created_by, objeto
		FROM atlas_modelo_versoes WHERE modelo_id = $1 ORDER BY versao DESC`, id)
	if err != nil {
		return m, wrapModelo(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.ModeloVersao
		if err := rows.Scan(&v.Versao, &v.ArquivoNome, &v.ContentType, &v.Tamanho, &v.SHA256, &v.Nota, &v.CreatedAt,
			&v.CreatedBy, &v.Objeto); err != nil {
			return m, wrapModelo(err)
		}
		m.Versoes = append(m.Versoes, v)
	}
	return m, wrapModelo(rows.Err())
}

// InsertModelo grava o modelo com a versão 1.
func (r *Repository) InsertModelo(ctx context.Context, db database.DBTX, m domain.Modelo, v domain.ModeloVersao, por string) error {
	if _, err := db.Exec(ctx, `INSERT INTO atlas_modelos (id, nome, descricao, ativo, updated_by) VALUES ($1,$2,$3,TRUE,$4)`,
		m.ID, m.Nome, m.Descricao, por); err != nil {
		return wrapModelo(err)
	}
	_, err := db.Exec(ctx, `INSERT INTO atlas_modelo_versoes (modelo_id, versao, arquivo_nome, content_type, tamanho, sha256,
		objeto, nota, created_by) VALUES ($1,1,$2,$3,$4,$5,$6,$7,$8)`,
		m.ID, v.ArquivoNome, v.ContentType, v.Tamanho, v.SHA256, v.Objeto, v.Nota, por)
	return wrapModelo(err)
}

// InsertModeloVersao publica a versão seguinte (trava o modelo: duas
// publicações simultâneas se serializam) e devolve o número dela.
func (r *Repository) InsertModeloVersao(ctx context.Context, db database.DBTX, id uuid.UUID, v domain.ModeloVersao) (int, error) {
	if _, err := db.Exec(ctx, `UPDATE atlas_modelos SET updated_at = now(), updated_by = $2 WHERE id = $1`, id, v.CreatedBy); err != nil {
		return 0, wrapModelo(err)
	}
	var versao int
	err := db.QueryRow(ctx, `INSERT INTO atlas_modelo_versoes (modelo_id, versao, arquivo_nome, content_type, tamanho, sha256,
		objeto, nota, created_by)
		SELECT $1, COALESCE(max(versao), 0) + 1, $2, $3, $4, $5, $6, $7, $8 FROM atlas_modelo_versoes WHERE modelo_id = $1
		RETURNING versao`, id, v.ArquivoNome, v.ContentType, v.Tamanho, v.SHA256, v.Objeto, v.Nota, v.CreatedBy).Scan(&versao)
	return versao, wrapModelo(err)
}

func (r *Repository) UpdateModelo(ctx context.Context, db database.DBTX, m domain.Modelo, por string) error {
	tag, err := db.Exec(ctx, `UPDATE atlas_modelos SET nome = $2, descricao = $3, ativo = $4, updated_at = now(), updated_by = $5
		WHERE id = $1`, m.ID, m.Nome, m.Descricao, m.Ativo, por)
	if err != nil {
		return wrapModelo(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrModeloNaoEncontrado
	}
	return nil
}

func (r *Repository) VersaoModelo(ctx context.Context, db database.DBTX, id uuid.UUID, versao int) (domain.ModeloVersao, error) {
	var v domain.ModeloVersao
	err := db.QueryRow(ctx, `SELECT versao, arquivo_nome, content_type, tamanho, sha256, nota, created_at, created_by, objeto
		FROM atlas_modelo_versoes WHERE modelo_id = $1 AND ($2 = 0 OR versao = $2) ORDER BY versao DESC LIMIT 1`, id, versao).
		Scan(&v.Versao, &v.ArquivoNome, &v.ContentType, &v.Tamanho, &v.SHA256, &v.Nota, &v.CreatedAt, &v.CreatedBy, &v.Objeto)
	return v, wrapModelo(err)
}

// ModelosAtivos conta quantos dos ids são modelos ativos (com trava de
// leitura: não somem nem são desativados até o fim da transação).
func (r *Repository) ModelosAtivos(ctx context.Context, db database.DBTX, ids []uuid.UUID) (int, error) {
	var n int
	err := db.QueryRow(ctx, `SELECT count(*) FROM (SELECT id FROM atlas_modelos WHERE id = ANY($1) AND ativo FOR SHARE) x`, ids).Scan(&n)
	return n, wrapModelo(err)
}

func (r *Repository) ModeloDaPeca(ctx context.Context, db database.DBTX, workflowID, docID uuid.UUID) (*uuid.UUID, error) {
	var modelo *uuid.UUID
	err := db.QueryRow(ctx, `SELECT d.modelo_id FROM atlas_etapa_documentos d JOIN atlas_etapas e ON e.id = d.etapa_id
		WHERE d.id = $1 AND e.workflow_id = $2 FOR UPDATE OF d`, docID, workflowID).Scan(&modelo)
	if database.IsNoRows(err) {
		return nil, domain.ErrPecaNaoEncontrada
	}
	return modelo, wrap(err)
}

func (r *Repository) SetModeloPeca(ctx context.Context, db database.DBTX, docID uuid.UUID, modeloID *uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE atlas_etapa_documentos SET modelo_id = $2 WHERE id = $1`, docID, modeloID)
	return wrap(err)
}

func (r *Repository) ModelosDaSerie(ctx context.Context, db database.DBTX, codigo string) ([]domain.Modelo, error) {
	rows, err := db.Query(ctx, modeloSelect+` JOIN atlas_ttdd_modelos s ON s.modelo_id = m.id WHERE s.codigo = $1
		ORDER BY lower(m.nome)`, codigo)
	if err != nil {
		return nil, wrapModelo(err)
	}
	defer rows.Close()
	out := []domain.Modelo{}
	for rows.Next() {
		m, err := scanModelo(rows)
		if err != nil {
			return nil, wrapModelo(err)
		}
		out = append(out, m)
	}
	return out, wrapModelo(rows.Err())
}

func (r *Repository) LigarModeloSerie(ctx context.Context, db database.DBTX, codigo string, modeloID uuid.UUID, por string) error {
	_, err := db.Exec(ctx, `INSERT INTO atlas_ttdd_modelos (codigo, modelo_id, created_by) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, codigo, modeloID, por)
	return wrapModelo(err)
}

func (r *Repository) DesligarModeloSerie(ctx context.Context, db database.DBTX, codigo string, modeloID uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM atlas_ttdd_modelos WHERE codigo = $1 AND modelo_id = $2`, codigo, modeloID)
	if err != nil {
		return wrapModelo(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrModeloNaoEncontrado
	}
	return nil
}
