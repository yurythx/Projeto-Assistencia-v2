// Package infrastructure implementa a persistência do IAM sobre PostgreSQL
// (pgx/pgxpool, queries 100% parametrizadas — A03).
package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository concentra as queries do IAM. Todo método recebe o DBTX para
// poder rodar dentro da transação que também grava a auditoria.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

func wrap(op string, err error) error {
	switch {
	case err == nil:
		return nil
	case database.IsNoRows(err):
		return domain.ErrNotFound
	case database.IsUniqueViolation(err):
		return domain.ErrConflict
	case database.IsForeignKeyViolation(err), database.IsCheckViolation(err):
		return fmt.Errorf("%w: %v", domain.ErrInvalidScope, err)
	default:
		return fmt.Errorf("iam: %s: %w", op, err)
	}
}

// wrapDelete: numa exclusão, violação de FK significa "ainda em uso"
// (ON DELETE RESTRICT — migration 000121), não escopo inválido.
func wrapDelete(op string, err error) error {
	if err != nil && database.IsForeignKeyViolation(err) {
		return domain.ErrInUse
	}
	return wrap(op, err)
}

// ---------------------------------------------------------------- entidades

const entidadeCols = `id, nome, sigla, slug, documento, ativo, created_at, updated_at`

func scanEntidade(row interface{ Scan(...any) error }) (domain.Entidade, error) {
	var e domain.Entidade
	err := row.Scan(&e.ID, &e.Nome, &e.Sigla, &e.Slug, &e.Documento, &e.Ativo, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

func (r *Repository) ListEntidades(ctx context.Context, db database.DBTX) ([]domain.Entidade, error) {
	rows, err := db.Query(ctx, `SELECT `+entidadeCols+` FROM entidades ORDER BY nome`)
	if err != nil {
		return nil, wrap("list entidades", err)
	}
	defer rows.Close()
	out := []domain.Entidade{}
	for rows.Next() {
		e, err := scanEntidade(rows)
		if err != nil {
			return nil, wrap("scan entidade", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) GetEntidade(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Entidade, error) {
	e, err := scanEntidade(db.QueryRow(ctx, `SELECT `+entidadeCols+` FROM entidades WHERE id = $1`, id))
	return e, wrap("get entidade", err)
}

func (r *Repository) UpsertEntidade(ctx context.Context, db database.DBTX, e domain.Entidade) (domain.Entidade, error) {
	out, err := scanEntidade(db.QueryRow(ctx, `
		INSERT INTO entidades (id, nome, sigla, slug, documento, ativo) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET nome=EXCLUDED.nome, sigla=EXCLUDED.sigla, slug=EXCLUDED.slug,
		    documento=EXCLUDED.documento, ativo=EXCLUDED.ativo
		RETURNING `+entidadeCols, e.ID, e.Nome, e.Sigla, e.Slug, e.Documento, e.Ativo))
	return out, wrap("upsert entidade", err)
}

func (r *Repository) DeleteEntidade(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM entidades WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrapDelete("delete entidade", err)
}

// ----------------------------------------------------------------- unidades

const unidadeCols = `id, entidade_id, parent_id, nome, sigla, slug, ad_group, email, telefone, endereco, ativo, created_at, updated_at`

func scanUnidade(row interface{ Scan(...any) error }) (domain.Unidade, error) {
	var u domain.Unidade
	err := row.Scan(&u.ID, &u.EntidadeID, &u.ParentID, &u.Nome, &u.Sigla, &u.Slug, &u.ADGroup,
		&u.Email, &u.Telefone, &u.Endereco, &u.Ativo, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *Repository) ListUnidades(ctx context.Context, db database.DBTX, entidadeID *uuid.UUID) ([]domain.Unidade, error) {
	rows, err := db.Query(ctx, `SELECT `+unidadeCols+` FROM unidades
		WHERE ($1::uuid IS NULL OR entidade_id = $1) ORDER BY nome`, entidadeID)
	if err != nil {
		return nil, wrap("list unidades", err)
	}
	defer rows.Close()
	out := []domain.Unidade{}
	for rows.Next() {
		u, err := scanUnidade(rows)
		if err != nil {
			return nil, wrap("scan unidade", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *Repository) GetUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Unidade, error) {
	u, err := scanUnidade(db.QueryRow(ctx, `SELECT `+unidadeCols+` FROM unidades WHERE id = $1`, id))
	return u, wrap("get unidade", err)
}

func (r *Repository) UpsertUnidade(ctx context.Context, db database.DBTX, u domain.Unidade) (domain.Unidade, error) {
	out, err := scanUnidade(db.QueryRow(ctx, `
		INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug, ad_group, email, telefone, endereco, ativo)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET entidade_id=EXCLUDED.entidade_id, parent_id=EXCLUDED.parent_id,
		    nome=EXCLUDED.nome, sigla=EXCLUDED.sigla, slug=EXCLUDED.slug, ad_group=EXCLUDED.ad_group,
		    email=EXCLUDED.email, telefone=EXCLUDED.telefone, endereco=EXCLUDED.endereco, ativo=EXCLUDED.ativo
		RETURNING `+unidadeCols,
		u.ID, u.EntidadeID, u.ParentID, u.Nome, u.Sigla, u.Slug, u.ADGroup, u.Email, u.Telefone, u.Endereco, u.Ativo))
	return out, wrap("upsert unidade", err)
}

// UnidadeCreatesCycle reporta se tornar parentID a mãe de id criaria um
// ciclo na hierarquia.
func (r *Repository) UnidadeCreatesCycle(ctx context.Context, db database.DBTX, id, parentID uuid.UUID) (bool, error) {
	var cycle bool
	err := db.QueryRow(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id FROM unidades WHERE id = $2
			UNION ALL
			SELECT u.id, u.parent_id FROM unidades u JOIN up ON u.id = up.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM up WHERE id = $1)`, id, parentID).Scan(&cycle)
	return cycle, wrap("cycle check", err)
}

func (r *Repository) DeleteUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM unidades WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrapDelete("delete unidade", err)
}

// ------------------------------------------------------------ departamentos

const departamentoCols = `id, unidade_id, nome, sigla, slug, ad_group, email, telefone, ativo, created_at, updated_at`

func scanDepartamento(row interface{ Scan(...any) error }) (domain.Departamento, error) {
	var d domain.Departamento
	err := row.Scan(&d.ID, &d.UnidadeID, &d.Nome, &d.Sigla, &d.Slug, &d.ADGroup, &d.Email, &d.Telefone, &d.Ativo, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (r *Repository) ListDepartamentos(ctx context.Context, db database.DBTX, unidadeID *uuid.UUID) ([]domain.Departamento, error) {
	rows, err := db.Query(ctx, `SELECT `+departamentoCols+` FROM departamentos
		WHERE ($1::uuid IS NULL OR unidade_id = $1) ORDER BY nome`, unidadeID)
	if err != nil {
		return nil, wrap("list departamentos", err)
	}
	defer rows.Close()
	out := []domain.Departamento{}
	for rows.Next() {
		d, err := scanDepartamento(rows)
		if err != nil {
			return nil, wrap("scan departamento", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) GetDepartamento(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Departamento, error) {
	d, err := scanDepartamento(db.QueryRow(ctx, `SELECT `+departamentoCols+` FROM departamentos WHERE id = $1`, id))
	return d, wrap("get departamento", err)
}

func (r *Repository) UpsertDepartamento(ctx context.Context, db database.DBTX, d domain.Departamento) (domain.Departamento, error) {
	out, err := scanDepartamento(db.QueryRow(ctx, `
		INSERT INTO departamentos (id, unidade_id, nome, sigla, slug, ad_group, email, telefone, ativo)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET unidade_id=EXCLUDED.unidade_id, nome=EXCLUDED.nome, sigla=EXCLUDED.sigla,
		    slug=EXCLUDED.slug, ad_group=EXCLUDED.ad_group, email=EXCLUDED.email, telefone=EXCLUDED.telefone, ativo=EXCLUDED.ativo
		RETURNING `+departamentoCols,
		d.ID, d.UnidadeID, d.Nome, d.Sigla, d.Slug, d.ADGroup, d.Email, d.Telefone, d.Ativo))
	return out, wrap("upsert departamento", err)
}

func (r *Repository) DeleteDepartamento(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM departamentos WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrapDelete("delete departamento", err)
}

// ------------------------------------------------------------------ perfis

const perfilCols = `id, slug, nome, descricao, permissoes, sistema, ativo, created_at, updated_at`

func scanPerfil(row interface{ Scan(...any) error }) (domain.Perfil, error) {
	var p domain.Perfil
	err := row.Scan(&p.ID, &p.Slug, &p.Nome, &p.Descricao, &p.Permissoes, &p.Sistema, &p.Ativo, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *Repository) ListPerfis(ctx context.Context, db database.DBTX) ([]domain.Perfil, error) {
	rows, err := db.Query(ctx, `SELECT `+perfilCols+` FROM perfis ORDER BY sistema DESC, nome`)
	if err != nil {
		return nil, wrap("list perfis", err)
	}
	defer rows.Close()
	out := []domain.Perfil{}
	for rows.Next() {
		p, err := scanPerfil(rows)
		if err != nil {
			return nil, wrap("scan perfil", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) GetPerfil(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Perfil, error) {
	p, err := scanPerfil(db.QueryRow(ctx, `SELECT `+perfilCols+` FROM perfis WHERE id = $1`, id))
	return p, wrap("get perfil", err)
}

func (r *Repository) UpsertPerfil(ctx context.Context, db database.DBTX, p domain.Perfil) (domain.Perfil, error) {
	out, err := scanPerfil(db.QueryRow(ctx, `
		INSERT INTO perfis (id, slug, nome, descricao, permissoes, ativo) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET slug=EXCLUDED.slug, nome=EXCLUDED.nome, descricao=EXCLUDED.descricao,
		    permissoes=EXCLUDED.permissoes, ativo=EXCLUDED.ativo
		RETURNING `+perfilCols, p.ID, p.Slug, p.Nome, p.Descricao, p.Permissoes, p.Ativo))
	return out, wrap("upsert perfil", err)
}

func (r *Repository) DeletePerfil(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM perfis WHERE id = $1 AND NOT sistema`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrSystemProfile
	}
	return wrapDelete("delete perfil", err)
}

// ------------------------------------------------------- escopos (rótulos)

// scopeLabelSQL monta "Entidade › Unidade › Departamento" para as tabelas
// com colunas entidade_id/unidade_id/departamento_id (alias t).
const scopeLabelSQL = `COALESCE(NULLIF(concat_ws(' › ', e.sigla, un.sigla, d.sigla), ''), 'Plataforma (todos os escopos)')`
const scopeJoins = `
	LEFT JOIN entidades e ON e.id = t.entidade_id
	LEFT JOIN unidades un ON un.id = t.unidade_id
	LEFT JOIN departamentos d ON d.id = t.departamento_id`

// ----------------------------------------------------------- mapeamentos AD

func (r *Repository) ListMappings(ctx context.Context, db database.DBTX) ([]domain.ADMapping, error) {
	rows, err := db.Query(ctx, mappingSelect+` ORDER BY lower(t.ad_group), p.nome`)
	if err != nil {
		return nil, wrap("list mappings", err)
	}
	defer rows.Close()
	out := []domain.ADMapping{}
	for rows.Next() {
		m, err := scanMapping(rows)
		if err != nil {
			return nil, wrap("scan mapping", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const mappingSelect = `
		SELECT t.id, t.ad_group, t.perfil_id, p.nome, t.entidade_id, t.unidade_id, t.departamento_id,
		       ` + scopeLabelSQL + `, t.descricao, t.created_at, t.created_by
		FROM ad_group_mappings t JOIN perfis p ON p.id = t.perfil_id ` + scopeJoins

func scanMapping(row interface{ Scan(...any) error }) (domain.ADMapping, error) {
	var m domain.ADMapping
	err := row.Scan(&m.ID, &m.ADGroup, &m.PerfilID, &m.PerfilNome, &m.EntidadeID, &m.UnidadeID, &m.DepartamentoID,
		&m.ScopeLabel, &m.Descricao, &m.CreatedAt, &m.CreatedBy)
	return m, err
}

func (r *Repository) GetMapping(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.ADMapping, error) {
	m, err := scanMapping(db.QueryRow(ctx, mappingSelect+` WHERE t.id = $1`, id))
	return m, wrap("get mapping", err)
}

func (r *Repository) CreateMapping(ctx context.Context, db database.DBTX, m domain.ADMapping) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO ad_group_mappings (ad_group, perfil_id, entidade_id, unidade_id, departamento_id, descricao, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		m.ADGroup, m.PerfilID, m.EntidadeID, m.UnidadeID, m.DepartamentoID, m.Descricao, m.CreatedBy).Scan(&id)
	return id, wrap("create mapping", err)
}

func (r *Repository) DeleteMapping(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM ad_group_mappings WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap("delete mapping", err)
}

// ---------------------------------------------------------------- lotações

const lotacaoSelect = `
		SELECT t.id, t.user_id, t.perfil_id, p.nome, t.entidade_id, t.unidade_id, t.departamento_id,
		       ` + scopeLabelSQL + `, t.principal, t.created_at, t.created_by
		FROM user_scopes t JOIN perfis p ON p.id = t.perfil_id ` + scopeJoins

func scanLotacao(row interface{ Scan(...any) error }) (domain.Lotacao, error) {
	var l domain.Lotacao
	err := row.Scan(&l.ID, &l.UserID, &l.PerfilID, &l.PerfilNome, &l.EntidadeID, &l.UnidadeID, &l.DepartamentoID,
		&l.ScopeLabel, &l.Principal, &l.CreatedAt, &l.CreatedBy)
	return l, err
}

func (r *Repository) ListLotacoes(ctx context.Context, db database.DBTX, userID uuid.UUID) ([]domain.Lotacao, error) {
	rows, err := db.Query(ctx, lotacaoSelect+` WHERE t.user_id = $1 ORDER BY t.principal DESC, p.nome`, userID)
	if err != nil {
		return nil, wrap("list lotacoes", err)
	}
	defer rows.Close()
	out := []domain.Lotacao{}
	for rows.Next() {
		l, err := scanLotacao(rows)
		if err != nil {
			return nil, wrap("scan lotacao", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repository) GetLotacao(ctx context.Context, db database.DBTX, userID, id uuid.UUID) (domain.Lotacao, error) {
	l, err := scanLotacao(db.QueryRow(ctx, lotacaoSelect+` WHERE t.id = $1 AND t.user_id = $2`, id, userID))
	return l, wrap("get lotacao", err)
}

func (r *Repository) CreateLotacao(ctx context.Context, db database.DBTX, l domain.Lotacao) (uuid.UUID, error) {
	if l.Principal {
		if _, err := db.Exec(ctx, `UPDATE user_scopes SET principal = false WHERE user_id = $1`, l.UserID); err != nil {
			return uuid.Nil, wrap("reset principal", err)
		}
	}
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO user_scopes (user_id, perfil_id, entidade_id, unidade_id, departamento_id, principal, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		l.UserID, l.PerfilID, l.EntidadeID, l.UnidadeID, l.DepartamentoID, l.Principal, l.CreatedBy).Scan(&id)
	return id, wrap("create lotacao", err)
}

func (r *Repository) DeleteLotacao(ctx context.Context, db database.DBTX, userID, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM user_scopes WHERE id = $1 AND user_id = $2`, id, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap("delete lotacao", err)
}

// ScopeConsistent confere que unidade pertence à entidade e departamento à
// unidade (quando informados), e completa os níveis superiores omitidos.
func (r *Repository) ScopeConsistent(ctx context.Context, db database.DBTX, s domain.Scope) (domain.Scope, error) {
	if s.DepartamentoID != nil {
		var unidadeID uuid.UUID
		if err := db.QueryRow(ctx, `SELECT unidade_id FROM departamentos WHERE id = $1`, *s.DepartamentoID).Scan(&unidadeID); err != nil {
			return s, domain.ErrInvalidScope
		}
		if s.UnidadeID != nil && *s.UnidadeID != unidadeID {
			return s, domain.ErrInvalidScope
		}
		s.UnidadeID = &unidadeID
	}
	if s.UnidadeID != nil {
		var entidadeID uuid.UUID
		if err := db.QueryRow(ctx, `SELECT entidade_id FROM unidades WHERE id = $1`, *s.UnidadeID).Scan(&entidadeID); err != nil {
			return s, domain.ErrInvalidScope
		}
		if s.EntidadeID != nil && *s.EntidadeID != entidadeID {
			return s, domain.ErrInvalidScope
		}
		s.EntidadeID = &entidadeID
	}
	return s, nil
}

// ------------------------------------------------------------------ users

const userCols = `id, username, email, display_name, active, keycloak_subject IS NOT NULL,
	password_hash IS NOT NULL AND password_hash <> 'ANONYMIZED', roles, groups, locked_until,
	created_at, last_seen_at, ad_synced_at`

func scanUser(row interface{ Scan(...any) error }) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Active, &u.Federated, &u.LocalLogin,
		&u.Roles, &u.Groups, &u.LockedUntil, &u.CreatedAt, &u.LastSeenAt, &u.ADSyncedAt)
	return u, err
}

// userGrantsSQL são os escopos das lotações da conta "users" da consulta
// externa: as manuais e as dos grupos do AD dela.
const userGrantsSQL = `(SELECT s.entidade_id, s.unidade_id, s.departamento_id FROM user_scopes s WHERE s.user_id = users.id
	UNION ALL
	SELECT m.entidade_id, m.unidade_id, m.departamento_id FROM ad_group_mappings m
	 WHERE lower(m.ad_group) = ANY (SELECT lower(g) FROM unnest(users.groups) g))`

// visibleSQL: a conta não tem lotação nenhuma, ou tem alguma na área
// (placeholders: entidades, unidades, departamentos).
func visibleSQL(e, u, d int) string {
	return fmt.Sprintf(`(NOT EXISTS %s OR EXISTS (SELECT 1 FROM %s g
		WHERE g.unidade_id = ANY($%d::uuid[]) OR g.departamento_id = ANY($%d::uuid[])
		   OR (g.unidade_id IS NULL AND g.entidade_id = ANY($%d::uuid[]))))`, userGrantsSQL, userGrantsSQL, u, d, e)
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func areaArgs(a domain.Area) []any {
	return []any{uuidStrings(a.Entidades), uuidStrings(a.Unidades), uuidStrings(a.Departamentos)}
}

func (r *Repository) UserVisible(ctx context.Context, db database.DBTX, id uuid.UUID, a domain.Area) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND `+visibleSQL(2, 3, 4)+`)`,
		append([]any{id}, areaArgs(a)...)...).Scan(&ok)
	return ok, wrap("user visible", err)
}

func (r *Repository) UserGrants(ctx context.Context, db database.DBTX, id uuid.UUID) ([]domain.Scope, error) {
	rows, err := db.Query(ctx, `SELECT g.entidade_id, g.unidade_id, g.departamento_id FROM users CROSS JOIN LATERAL `+userGrantsSQL+` g WHERE users.id = $1`, id)
	if err != nil {
		return nil, wrap("user grants", err)
	}
	defer rows.Close()
	out := []domain.Scope{}
	for rows.Next() {
		var s domain.Scope
		if err := rows.Scan(&s.EntidadeID, &s.UnidadeID, &s.DepartamentoID); err != nil {
			return nil, wrap("scan grant", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) ListUsers(ctx context.Context, db database.DBTX, f domain.UserFilter, p pagination.Params) ([]domain.User, int64, error) {
	q := "%" + strings.ToLower(f.Query) + "%"
	where := `WHERE ($1 = '%%' OR nexus_unaccent(lower(display_name || ' ' || username || ' ' || email)) LIKE nexus_unaccent($1))
		AND ($2::boolean IS NULL OR active = $2)`
	args := []any{q, f.Active}
	if f.Area != nil {
		where += " AND " + visibleSQL(3, 4, 5)
		args = append(args, areaArgs(*f.Area)...)
	}
	var total int64
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users `+where, args...).Scan(&total); err != nil {
		return nil, 0, wrap("count users", err)
	}
	n := len(args)
	rows, err := db.Query(ctx, `SELECT `+userCols+` FROM users `+where+fmt.Sprintf(`
		ORDER BY lower(COALESCE(NULLIF(display_name,''), username)) LIMIT $%d OFFSET $%d`, n+1, n+2), append(args, p.Limit(), p.Offset())...)
	if err != nil {
		return nil, 0, wrap("list users", err)
	}
	defer rows.Close()
	out := []domain.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, wrap("scan user", err)
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetUser(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.User, error) {
	u, err := scanUser(db.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
	return u, wrap("get user", err)
}

// UserPermissions aplica a mesma regra do resolvedor (platform/iam): perfis
// ativos das lotações, perfis ativos dos grupos do AD da conta e "*" para
// o papel nexus-admin.
func (r *Repository) UserPermissions(ctx context.Context, db database.DBTX, id uuid.UUID) ([]string, error) {
	var perms []string
	err := db.QueryRow(ctx, `
		WITH u AS (SELECT roles, groups FROM users WHERE id = $1),
		grants AS (
			SELECT unnest(p.permissoes) AS perm FROM user_scopes s JOIN perfis p ON p.id = s.perfil_id AND p.ativo
			 WHERE s.user_id = $1 AND nexus_scope_active(s.entidade_id, s.unidade_id, s.departamento_id)
			UNION
			SELECT unnest(p.permissoes) FROM ad_group_mappings m JOIN perfis p ON p.id = m.perfil_id AND p.ativo, u
			 WHERE lower(m.ad_group) = ANY (SELECT lower(g) FROM unnest(u.groups) g)
			   AND nexus_scope_active(m.entidade_id, m.unidade_id, m.departamento_id)
			UNION
			SELECT '*' FROM u WHERE $2 = ANY (u.roles)
		)
		SELECT COALESCE(array_agg(perm ORDER BY perm), '{}') FROM grants`, id, "nexus-admin").Scan(&perms)
	return perms, wrap("user permissions", err)
}

func (r *Repository) UpdateUser(ctx context.Context, db database.DBTX, id uuid.UUID, displayName string, active bool, roles []string) error {
	tag, err := db.Exec(ctx, `UPDATE users SET display_name = $2, active = $3, roles = $4 WHERE id = $1`, id, displayName, active, roles)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap("update user", err)
}

func (r *Repository) CreateLocalUser(ctx context.Context, db database.DBTX, username, email, displayName, hash string, roles []string) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `
		INSERT INTO users (username, email, display_name, password_hash, roles, active)
		VALUES ($1, $2, $3, $4, $5, true) RETURNING id`, username, email, displayName, hash, roles).Scan(&id)
	return id, wrap("create local user", err)
}

func (r *Repository) SetPassword(ctx context.Context, db database.DBTX, id uuid.UUID, hash string) error {
	tag, err := db.Exec(ctx, `UPDATE users SET password_hash = $2, failed_login_attempts = 0, locked_until = NULL WHERE id = $1`, id, hash)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap("set password", err)
}

func (r *Repository) Unlock(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `UPDATE users SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap("unlock", err)
}
