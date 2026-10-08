// Package infrastructure implementa o repositório do Diretório.
package infrastructure

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

// A lotação exibida é a do perfil estendido; na falta dela, a lotação
// principal do IAM (user_scopes.principal).
const base = `
	FROM users u
	LEFT JOIN directory_profiles dp ON dp.user_id = u.id
	LEFT JOIN LATERAL (
		SELECT s.unidade_id, s.departamento_id FROM user_scopes s
		WHERE s.user_id = u.id ORDER BY s.principal DESC, s.created_at LIMIT 1
	) ls ON true
	LEFT JOIN unidades un ON un.id = COALESCE(dp.unidade_id, ls.unidade_id)
	LEFT JOIN departamentos d ON d.id = COALESCE(dp.departamento_id, ls.departamento_id)`

const cols = `u.id, COALESCE(NULLIF(u.display_name,''), u.username), u.username, u.email,
	COALESCE(dp.job_title,''), COALESCE(dp.phone,''), COALESCE(dp.extension,''), COALESCE(dp.bio,''),
	COALESCE(dp.visible, true), un.id, COALESCE(un.nome,''), d.id, COALESCE(d.nome,''), u.keycloak_subject IS NOT NULL`

func scan(row interface{ Scan(...any) error }) (domain.Person, error) {
	var p domain.Person
	err := row.Scan(&p.UserID, &p.Name, &p.Username, &p.Email, &p.JobTitle, &p.Phone, &p.Extension, &p.Bio,
		&p.Visible, &p.UnidadeID, &p.Unidade, &p.DepartamentoID, &p.Departamento, &p.FromAD)
	return p, err
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	if database.IsNoRows(err) {
		return domain.ErrNotFound
	}
	if database.IsForeignKeyViolation(err) {
		return domain.ErrSector
	}
	return fmt.Errorf("directory: %w", err)
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func (r *Repository) List(ctx context.Context, db database.DBTX, f domain.Filter, p pagination.Params) ([]domain.Person, int64, error) {
	const where = ` WHERE u.active AND u.username NOT LIKE 'anon\_%'
		AND ($1 OR COALESCE(dp.visible, true) OR un.id = ANY($5::uuid[]) OR d.id = ANY($6::uuid[]))
		AND ($2 = '' OR strpos(nexus_unaccent(lower(u.display_name || ' ' || u.username || ' ' || COALESCE(dp.job_title,''))),
		                        nexus_unaccent(lower($2))) > 0)
		AND ($3::uuid IS NULL OR un.id = $3)
		AND ($4::uuid IS NULL OR d.id = $4)`
	args := []any{f.IncludeHidden, f.Query, f.UnidadeID, f.DepartamentoID, uuidStrings(f.HiddenUnidades), uuidStrings(f.HiddenDepartamentos)}
	var total int64
	if err := db.QueryRow(ctx, `SELECT count(*)`+base+where, args...).Scan(&total); err != nil {
		return nil, 0, wrap(err)
	}
	rows, err := db.Query(ctx, `SELECT `+cols+base+where+`
		ORDER BY lower(COALESCE(NULLIF(u.display_name,''), u.username)) LIMIT $7 OFFSET $8`,
		append(args, p.Limit(), p.Offset())...)
	if err != nil {
		return nil, 0, wrap(err)
	}
	defer rows.Close()
	out := []domain.Person{}
	for rows.Next() {
		person, err := scan(rows)
		if err != nil {
			return nil, 0, wrap(err)
		}
		out = append(out, person)
	}
	return out, total, rows.Err()
}

func (r *Repository) Get(ctx context.Context, db database.DBTX, userID uuid.UUID) (domain.Person, error) {
	p, err := scan(db.QueryRow(ctx, `SELECT `+cols+base+` WHERE u.id = $1 AND u.active`, userID))
	return p, wrap(err)
}

// SaveProfile grava o perfil estendido. keepLotacao preserva a lotação
// exibida (autoatendimento não pode mudar a própria lotação).
func (r *Repository) SaveProfile(ctx context.Context, db database.DBTX, userID uuid.UUID, in domain.ProfileInput, keepLotacao bool) error {
	_, err := db.Exec(ctx, `
		INSERT INTO directory_profiles (user_id, job_title, phone, extension, bio, visible, unidade_id, departamento_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (user_id) DO UPDATE SET job_title=EXCLUDED.job_title, phone=EXCLUDED.phone,
		    extension=EXCLUDED.extension, bio=EXCLUDED.bio, visible=EXCLUDED.visible,
		    unidade_id = CASE WHEN $9 THEN directory_profiles.unidade_id ELSE EXCLUDED.unidade_id END,
		    departamento_id = CASE WHEN $9 THEN directory_profiles.departamento_id ELSE EXCLUDED.departamento_id END`,
		userID, in.JobTitle, in.Phone, in.Extension, in.Bio, in.Visible, in.UnidadeID, in.DepartamentoID, keepLotacao)
	return wrap(err)
}

// DeleteProfile apaga o perfil estendido — telefone, ramal, cargo e bio são
// dados pessoais sem valor de registro legal.
func (r *Repository) DeleteProfile(ctx context.Context, db database.DBTX, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM directory_profiles WHERE user_id = $1`, userID)
	return wrap(err)
}

func (r *Repository) DepartamentoUnidade(ctx context.Context, db database.DBTX, departamentoID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `SELECT unidade_id FROM departamentos WHERE id = $1`, departamentoID).Scan(&id)
	if database.IsNoRows(err) {
		return uuid.Nil, domain.ErrSector
	}
	return id, wrap(err)
}

func (r *Repository) Sectors(ctx context.Context, db database.DBTX, query string) ([]domain.Sector, error) {
	rows, err := db.Query(ctx, `
		SELECT un.id, 'unidade', un.nome, un.sigla, un.email, un.telefone, un.endereco, NULL::uuid, '', e.nome
		FROM unidades un JOIN entidades e ON e.id = un.entidade_id
		WHERE un.ativo AND e.ativo AND ($1 = '' OR nexus_unaccent(lower(un.nome || ' ' || un.sigla)) LIKE '%' || nexus_unaccent(lower($1)) || '%')
		UNION ALL
		SELECT d.id, 'departamento', d.nome, d.sigla, d.email, d.telefone, '', un.id, un.nome, e.nome
		FROM departamentos d JOIN unidades un ON un.id = d.unidade_id JOIN entidades e ON e.id = un.entidade_id
		WHERE d.ativo AND un.ativo AND e.ativo
		  AND ($1 = '' OR nexus_unaccent(lower(d.nome || ' ' || d.sigla || ' ' || un.nome)) LIKE '%' || nexus_unaccent(lower($1)) || '%')
		ORDER BY 10, 9, 3`, query)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Sector{}
	for rows.Next() {
		var s domain.Sector
		if err := rows.Scan(&s.ID, &s.Kind, &s.Nome, &s.Sigla, &s.Email, &s.Telefone, &s.Endereco, &s.UnidadeID, &s.Unidade, &s.Entidade); err != nil {
			return nil, wrap(err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
