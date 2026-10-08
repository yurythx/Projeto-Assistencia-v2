// Package infrastructure implementa o repositório do Catálogo.
package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

const cols = `s.id, s.slug, s.title, s.summary, s.description, s.category, s.audience, s.requirements, s.steps,
	s.channels, s.sla, s.cost, s.icon, s.responsible_unidade_id, COALESCE(u.nome, ''), s.status, s.position,
	s.published_at, s.created_at, s.updated_at`

const from = ` FROM catalog_services s LEFT JOIN unidades u ON u.id = s.responsible_unidade_id `

func scan(row interface{ Scan(...any) error }, extra ...any) (domain.Service, error) {
	var s domain.Service
	var channels []byte
	dst := []any{&s.ID, &s.Slug, &s.Title, &s.Summary, &s.Description, &s.Category, &s.Audience, &s.Requirements, &s.Steps,
		&channels, &s.SLA, &s.Cost, &s.Icon, &s.ResponsibleUnidadeID, &s.ResponsibleUnidade, &s.Status, &s.Position,
		&s.PublishedAt, &s.CreatedAt, &s.UpdatedAt}
	if err := row.Scan(append(dst, extra...)...); err != nil {
		return s, err
	}
	s.Channels = []domain.Channel{}
	_ = json.Unmarshal(channels, &s.Channels)
	return s, nil
}

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case database.IsNoRows(err):
		return domain.ErrNotFound
	case database.IsUniqueViolation(err):
		return domain.ErrSlugTaken
	case database.IsForeignKeyViolation(err):
		return domain.ErrUnidade
	}
	return fmt.Errorf("catalog: %w", err)
}

func (r *Repository) List(ctx context.Context, db database.DBTX, f domain.Filter, p pagination.Params) ([]domain.Service, int64, error) {
	const where = `WHERE ($1 = 'all' OR s.status = $1) AND ($2 = '' OR s.category = $2)
		AND ($3 = '' OR s.search @@ nexus_search_tsquery('portuguese', $3))
		AND (NOT $4 OR s.responsible_unidade_id = ANY($5::uuid[]))`
	unidades := make([]string, 0, len(f.Unidades))
	for _, u := range f.Unidades {
		unidades = append(unidades, u.String())
	}
	var total int64
	if err := db.QueryRow(ctx, `SELECT count(*)`+from+where, f.Status, f.Category, f.Query, f.Restrito, unidades).Scan(&total); err != nil {
		return nil, 0, wrap(err)
	}
	rows, err := db.Query(ctx, `SELECT `+cols+from+where+` ORDER BY s.category, s.position, s.title LIMIT $6 OFFSET $7`,
		f.Status, f.Category, f.Query, f.Restrito, unidades, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, wrap(err)
	}
	defer rows.Close()
	out := []domain.Service{}
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, 0, wrap(err)
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}

func (r *Repository) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Service, error) {
	s, err := scan(db.QueryRow(ctx, `SELECT `+cols+from+`WHERE s.id = $1`, id))
	return s, wrap(err)
}

func (r *Repository) GetBySlug(ctx context.Context, db database.DBTX, slug string) (domain.Service, error) {
	s, err := scan(db.QueryRow(ctx, `SELECT `+cols+from+`WHERE s.slug = $1`, slug))
	return s, wrap(err)
}

func (r *Repository) Save(ctx context.Context, db database.DBTX, s domain.Service) (domain.Service, error) {
	channels, _ := json.Marshal(s.Channels)
	_, err := db.Exec(ctx, `
		INSERT INTO catalog_services (id, slug, title, summary, description, category, audience, requirements, steps,
		    channels, sla, cost, icon, responsible_unidade_id, status, position, published_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (id) DO UPDATE SET slug=EXCLUDED.slug, title=EXCLUDED.title, summary=EXCLUDED.summary,
		    description=EXCLUDED.description, category=EXCLUDED.category, audience=EXCLUDED.audience,
		    requirements=EXCLUDED.requirements, steps=EXCLUDED.steps, channels=EXCLUDED.channels, sla=EXCLUDED.sla,
		    cost=EXCLUDED.cost, icon=EXCLUDED.icon, responsible_unidade_id=EXCLUDED.responsible_unidade_id,
		    status=EXCLUDED.status, position=EXCLUDED.position, published_at=EXCLUDED.published_at`,
		s.ID, s.Slug, s.Title, s.Summary, s.Description, s.Category, s.Audience, s.Requirements, s.Steps,
		channels, s.SLA, s.Cost, s.Icon, s.ResponsibleUnidadeID, s.Status, s.Position, s.PublishedAt)
	if err != nil {
		return domain.Service{}, wrap(err)
	}
	return r.Get(ctx, db, s.ID)
}

func (r *Repository) Delete(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM catalog_services WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}

func (r *Repository) Categories(ctx context.Context, db database.DBTX) ([]domain.Category, error) {
	rows, err := db.Query(ctx, `SELECT category, count(*) FROM catalog_services WHERE status = 'published'
		GROUP BY category ORDER BY category`)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Category{}
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.Name, &c.Count); err != nil {
			return nil, wrap(err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) Search(ctx context.Context, db database.DBTX, query string, limit int) ([]domain.Service, []float64, error) {
	rows, err := db.Query(ctx, `SELECT `+cols+`, ts_rank(s.search, q)`+from+`, nexus_search_tsquery('portuguese', $1) q
		WHERE s.status = 'published' AND s.search @@ q ORDER BY 21 DESC LIMIT $2`, query, limit)
	if err != nil {
		return nil, nil, wrap(err)
	}
	defer rows.Close()
	var out []domain.Service
	var ranks []float64
	for rows.Next() {
		var rank float32
		s, err := scan(rows, &rank)
		if err != nil {
			return nil, nil, wrap(err)
		}
		out = append(out, s)
		ranks = append(ranks, float64(rank))
	}
	return out, ranks, rows.Err()
}
