// Package infrastructure implementa o repositório da Wiki.
package infrastructure

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/wiki/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/publico"
)

// tabelaPublico é onde fica o público-alvo próprio das páginas (ADR 014).
var tabelaPublico = publico.Tabela{Nome: "wiki_page_publico", Coluna: "page_id"}

func (r *Repository) Publicos(ctx context.Context, db database.DBTX) (map[uuid.UUID]auth.Publico, error) {
	return tabelaPublico.Todos(ctx, db)
}

func (r *Repository) SetPublico(ctx context.Context, db database.DBTX, id uuid.UUID, p auth.Publico) error {
	return tabelaPublico.Gravar(ctx, db, id, p)
}

// comPublico preenche o público-alvo próprio da página lida.
func comPublico(ctx context.Context, db database.DBTX, p domain.Page, err error) (domain.Page, error) {
	if err != nil {
		return p, wrap(err)
	}
	if p.Publico, err = tabelaPublico.Um(ctx, db, p.ID); err != nil {
		return domain.Page{}, err
	}
	return p, nil
}

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case database.IsNoRows(err):
		return domain.ErrNotFound
	case database.IsUniqueViolation(err):
		return domain.ErrSlugTaken
	case database.IsForeignKeyViolation(err):
		return domain.ErrHasChildren
	}
	return fmt.Errorf("wiki: %w", err)
}

const cols = `p.id, p.parent_id, p.slug, p.title, p.body, p.position, p.version, p.created_by, p.updated_by,
	COALESCE(NULLIF(u.display_name,''), u.username, ''), p.created_at, p.updated_at, p.unidade_id`
const from = ` FROM wiki_pages p LEFT JOIN users u ON u.id = p.updated_by `

func scan(row interface{ Scan(...any) error }, extra ...any) (domain.Page, error) {
	var p domain.Page
	err := row.Scan(append([]any{&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.Body, &p.Position, &p.Version, &p.CreatedBy,
		&p.UpdatedBy, &p.UpdatedByName, &p.CreatedAt, &p.UpdatedAt, &p.UnidadeID}, extra...)...)
	return p, err
}

func (r *Repository) Tree(ctx context.Context, db database.DBTX) ([]domain.Page, error) {
	rows, err := db.Query(ctx, `SELECT `+cols+from+` ORDER BY p.parent_id NULLS FIRST, p.position, lower(p.title)`)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Page{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, wrap(err)
		}
		p.Body = ""
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Page, error) {
	p, err := scan(db.QueryRow(ctx, `SELECT `+cols+from+` WHERE p.id = $1`, id))
	return comPublico(ctx, db, p, err)
}

func (r *Repository) GetBySlug(ctx context.Context, db database.DBTX, slug string) (domain.Page, error) {
	p, err := scan(db.QueryRow(ctx, `SELECT `+cols+from+` WHERE p.slug = $1`, slug))
	return comPublico(ctx, db, p, err)
}

func (r *Repository) Breadcrumbs(ctx context.Context, db database.DBTX, id uuid.UUID) ([]domain.Page, error) {
	rows, err := db.Query(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS depth FROM wiki_pages WHERE id = $1
			UNION ALL
			SELECT w.id, w.parent_id, up.depth + 1 FROM wiki_pages w JOIN up ON w.id = up.parent_id WHERE up.depth < 64
		)
		SELECT `+cols+from+` JOIN up ON up.id = p.id WHERE up.depth > 0 ORDER BY up.depth DESC`, id)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Page{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, wrap(err)
		}
		p.Body = ""
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) Insert(ctx context.Context, db database.DBTX, p domain.Page) error {
	_, err := db.Exec(ctx, `INSERT INTO wiki_pages (id, parent_id, slug, title, body, position, version, created_by, updated_by, unidade_id)
		VALUES ($1,$2,$3,$4,$5,$6,1,$7,$7,$8)`, p.ID, p.ParentID, p.Slug, p.Title, p.Body, p.Position, p.CreatedBy, p.UnidadeID)
	return wrap(err)
}

func (r *Repository) UpdateIfVersion(ctx context.Context, db database.DBTX, p domain.Page, expected int) error {
	tag, err := db.Exec(ctx, `UPDATE wiki_pages SET parent_id=$2, slug=$3, title=$4, body=$5, position=$6,
		version = version + 1, updated_by=$7, updated_at=now(), unidade_id=$9 WHERE id=$1 AND version=$8`,
		p.ID, p.ParentID, p.Slug, p.Title, p.Body, p.Position, p.UpdatedBy, expected, p.UnidadeID)
	if err != nil {
		return wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrStale
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM wiki_pages WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}

func (r *Repository) IsDescendant(ctx context.Context, db database.DBTX, candidate, ancestor uuid.UUID) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id FROM wiki_pages WHERE id = $1
			UNION ALL
			SELECT w.id, w.parent_id FROM wiki_pages w JOIN up ON w.id = up.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, candidate, ancestor).Scan(&ok)
	return ok, wrap(err)
}

func (r *Repository) AddRevision(ctx context.Context, db database.DBTX, rev domain.Revision) error {
	_, err := db.Exec(ctx, `INSERT INTO wiki_revisions (page_id, version, title, body, summary, edited_by)
		VALUES ($1,$2,$3,$4,$5,$6)`, rev.PageID, rev.Version, rev.Title, rev.Body, rev.Summary, rev.EditedBy)
	return wrap(err)
}

func (r *Repository) Revisions(ctx context.Context, db database.DBTX, pageID uuid.UUID) ([]domain.Revision, error) {
	rows, err := db.Query(ctx, `SELECT v.id, v.page_id, v.version, v.title, v.summary, v.edited_by,
		COALESCE(NULLIF(u.display_name,''), u.username, ''), v.edited_at
		FROM wiki_revisions v LEFT JOIN users u ON u.id = v.edited_by WHERE v.page_id = $1 ORDER BY v.version DESC`, pageID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Revision{}
	for rows.Next() {
		var v domain.Revision
		if err := rows.Scan(&v.ID, &v.PageID, &v.Version, &v.Title, &v.Summary, &v.EditedBy, &v.EditedByName, &v.EditedAt); err != nil {
			return nil, wrap(err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) Revision(ctx context.Context, db database.DBTX, pageID uuid.UUID, version int) (domain.Revision, error) {
	var v domain.Revision
	err := db.QueryRow(ctx, `SELECT v.id, v.page_id, v.version, v.title, v.body, v.summary, v.edited_by,
		COALESCE(NULLIF(u.display_name,''), u.username, ''), v.edited_at
		FROM wiki_revisions v LEFT JOIN users u ON u.id = v.edited_by WHERE v.page_id = $1 AND v.version = $2`, pageID, version).
		Scan(&v.ID, &v.PageID, &v.Version, &v.Title, &v.Body, &v.Summary, &v.EditedBy, &v.EditedByName, &v.EditedAt)
	return v, wrap(err)
}

func (r *Repository) Search(ctx context.Context, db database.DBTX, q string, limit int) ([]domain.Page, []float64, error) {
	rows, err := db.Query(ctx, `SELECT `+cols+`, ts_rank(p.search, q), ts_headline('portuguese', p.body, q,
		'MaxWords=25, MinWords=10, StartSel=**, StopSel=**')`+from+`, nexus_search_tsquery('portuguese', $1) q
		WHERE p.search @@ q ORDER BY 13 DESC LIMIT $2`, q, limit)
	if err != nil {
		return nil, nil, wrap(err)
	}
	defer rows.Close()
	var out []domain.Page
	var ranks []float64
	for rows.Next() {
		var rank float32
		var headline string
		p, err := scan(rows, &rank, &headline)
		if err != nil {
			return nil, nil, wrap(err)
		}
		p.Body = headline
		out = append(out, p)
		ranks = append(ranks, float64(rank))
	}
	return out, ranks, rows.Err()
}
