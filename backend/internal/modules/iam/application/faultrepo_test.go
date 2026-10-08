package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

type innerRepo = domain.Repository

var errBoom = errors.New("falha simulada no repositório")

// faultRepo envolve o repositório real: na chamada failAt devolve erro; depois
// da chamada poisonAt "envenena" a transação (a instrução SQL seguinte — do
// repositório, do outbox ou da auditoria — falha). noTx marca veneno em
// leitura fora de transação, onde não há instrução seguinte na mesma conexão.
type faultRepo struct {
	inner                   innerRepo
	calls, failAt, poisonAt int
	trace                   []string
	noTx                    bool
}

func (fr *faultRepo) hook(name string) error {
	fr.calls++
	fr.trace = append(fr.trace, name)
	if fr.calls == fr.failAt {
		return errBoom
	}
	return nil
}

func (fr *faultRepo) post(ctx context.Context, db database.DBTX) {
	if fr.calls != fr.poisonAt {
		return
	}
	if _, inTx := db.(pgx.Tx); !inTx {
		fr.noTx = true
		return
	}
	_, _ = db.Exec(ctx, `SELECT 1/0`)
}

func (fr *faultRepo) ListEntidades(ctx context.Context, db database.DBTX) ([]domain.Entidade, error) {
	if err := fr.hook("ListEntidades"); err != nil {
		var z0 []domain.Entidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListEntidades(ctx, db)
}

func (fr *faultRepo) GetEntidade(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Entidade, error) {
	if err := fr.hook("GetEntidade"); err != nil {
		var z0 domain.Entidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetEntidade(ctx, db, id)
}

func (fr *faultRepo) UpsertEntidade(ctx context.Context, db database.DBTX, e domain.Entidade) (domain.Entidade, error) {
	if err := fr.hook("UpsertEntidade"); err != nil {
		var z0 domain.Entidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpsertEntidade(ctx, db, e)
}

func (fr *faultRepo) DeleteEntidade(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteEntidade"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteEntidade(ctx, db, id)
}

func (fr *faultRepo) ListUnidades(ctx context.Context, db database.DBTX, entidadeID *uuid.UUID) ([]domain.Unidade, error) {
	if err := fr.hook("ListUnidades"); err != nil {
		var z0 []domain.Unidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListUnidades(ctx, db, entidadeID)
}

func (fr *faultRepo) GetUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Unidade, error) {
	if err := fr.hook("GetUnidade"); err != nil {
		var z0 domain.Unidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetUnidade(ctx, db, id)
}

func (fr *faultRepo) UpsertUnidade(ctx context.Context, db database.DBTX, u domain.Unidade) (domain.Unidade, error) {
	if err := fr.hook("UpsertUnidade"); err != nil {
		var z0 domain.Unidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpsertUnidade(ctx, db, u)
}

func (fr *faultRepo) UnidadeCreatesCycle(ctx context.Context, db database.DBTX, id uuid.UUID, parentID uuid.UUID) (bool, error) {
	if err := fr.hook("UnidadeCreatesCycle"); err != nil {
		var z0 bool
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UnidadeCreatesCycle(ctx, db, id, parentID)
}

func (fr *faultRepo) DeleteUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteUnidade"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteUnidade(ctx, db, id)
}

func (fr *faultRepo) ListDepartamentos(ctx context.Context, db database.DBTX, unidadeID *uuid.UUID) ([]domain.Departamento, error) {
	if err := fr.hook("ListDepartamentos"); err != nil {
		var z0 []domain.Departamento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListDepartamentos(ctx, db, unidadeID)
}

func (fr *faultRepo) GetDepartamento(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Departamento, error) {
	if err := fr.hook("GetDepartamento"); err != nil {
		var z0 domain.Departamento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetDepartamento(ctx, db, id)
}

func (fr *faultRepo) UpsertDepartamento(ctx context.Context, db database.DBTX, d domain.Departamento) (domain.Departamento, error) {
	if err := fr.hook("UpsertDepartamento"); err != nil {
		var z0 domain.Departamento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpsertDepartamento(ctx, db, d)
}

func (fr *faultRepo) DeleteDepartamento(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteDepartamento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteDepartamento(ctx, db, id)
}

func (fr *faultRepo) ListPerfis(ctx context.Context, db database.DBTX) ([]domain.Perfil, error) {
	if err := fr.hook("ListPerfis"); err != nil {
		var z0 []domain.Perfil
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListPerfis(ctx, db)
}

func (fr *faultRepo) GetPerfil(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Perfil, error) {
	if err := fr.hook("GetPerfil"); err != nil {
		var z0 domain.Perfil
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetPerfil(ctx, db, id)
}

func (fr *faultRepo) UpsertPerfil(ctx context.Context, db database.DBTX, p domain.Perfil) (domain.Perfil, error) {
	if err := fr.hook("UpsertPerfil"); err != nil {
		var z0 domain.Perfil
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpsertPerfil(ctx, db, p)
}

func (fr *faultRepo) DeletePerfil(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeletePerfil"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeletePerfil(ctx, db, id)
}

func (fr *faultRepo) ListMappings(ctx context.Context, db database.DBTX) ([]domain.ADMapping, error) {
	if err := fr.hook("ListMappings"); err != nil {
		var z0 []domain.ADMapping
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListMappings(ctx, db)
}

func (fr *faultRepo) GetMapping(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.ADMapping, error) {
	if err := fr.hook("GetMapping"); err != nil {
		var z0 domain.ADMapping
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetMapping(ctx, db, id)
}

func (fr *faultRepo) CreateMapping(ctx context.Context, db database.DBTX, m domain.ADMapping) (uuid.UUID, error) {
	if err := fr.hook("CreateMapping"); err != nil {
		var z0 uuid.UUID
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.CreateMapping(ctx, db, m)
}

func (fr *faultRepo) DeleteMapping(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteMapping"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteMapping(ctx, db, id)
}

func (fr *faultRepo) ListLotacoes(ctx context.Context, db database.DBTX, userID uuid.UUID) ([]domain.Lotacao, error) {
	if err := fr.hook("ListLotacoes"); err != nil {
		var z0 []domain.Lotacao
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListLotacoes(ctx, db, userID)
}

func (fr *faultRepo) GetLotacao(ctx context.Context, db database.DBTX, userID uuid.UUID, id uuid.UUID) (domain.Lotacao, error) {
	if err := fr.hook("GetLotacao"); err != nil {
		var z0 domain.Lotacao
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetLotacao(ctx, db, userID, id)
}

func (fr *faultRepo) CreateLotacao(ctx context.Context, db database.DBTX, l domain.Lotacao) (uuid.UUID, error) {
	if err := fr.hook("CreateLotacao"); err != nil {
		var z0 uuid.UUID
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.CreateLotacao(ctx, db, l)
}

func (fr *faultRepo) DeleteLotacao(ctx context.Context, db database.DBTX, userID uuid.UUID, id uuid.UUID) error {
	if err := fr.hook("DeleteLotacao"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteLotacao(ctx, db, userID, id)
}

func (fr *faultRepo) ScopeConsistent(ctx context.Context, db database.DBTX, s domain.Scope) (domain.Scope, error) {
	if err := fr.hook("ScopeConsistent"); err != nil {
		var z0 domain.Scope
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ScopeConsistent(ctx, db, s)
}

func (fr *faultRepo) ListUsers(ctx context.Context, db database.DBTX, f domain.UserFilter, p pagination.Params) ([]domain.User, int64, error) {
	if err := fr.hook("ListUsers"); err != nil {
		var z0 []domain.User
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListUsers(ctx, db, f, p)
}

func (fr *faultRepo) GetUser(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.User, error) {
	if err := fr.hook("GetUser"); err != nil {
		var z0 domain.User
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetUser(ctx, db, id)
}

func (fr *faultRepo) UserPermissions(ctx context.Context, db database.DBTX, id uuid.UUID) ([]string, error) {
	if err := fr.hook("UserPermissions"); err != nil {
		var z0 []string
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UserPermissions(ctx, db, id)
}

func (fr *faultRepo) UserGrants(ctx context.Context, db database.DBTX, id uuid.UUID) ([]domain.Scope, error) {
	if err := fr.hook("UserGrants"); err != nil {
		var z0 []domain.Scope
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UserGrants(ctx, db, id)
}

func (fr *faultRepo) UserVisible(ctx context.Context, db database.DBTX, id uuid.UUID, a domain.Area) (bool, error) {
	if err := fr.hook("UserVisible"); err != nil {
		var z0 bool
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UserVisible(ctx, db, id, a)
}

func (fr *faultRepo) UpdateUser(ctx context.Context, db database.DBTX, id uuid.UUID, displayName string, active bool, roles []string) error {
	if err := fr.hook("UpdateUser"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateUser(ctx, db, id, displayName, active, roles)
}

func (fr *faultRepo) CreateLocalUser(ctx context.Context, db database.DBTX, username string, email string, displayName string, hash string, roles []string) (uuid.UUID, error) {
	if err := fr.hook("CreateLocalUser"); err != nil {
		var z0 uuid.UUID
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.CreateLocalUser(ctx, db, username, email, displayName, hash, roles)
}

func (fr *faultRepo) SetPassword(ctx context.Context, db database.DBTX, id uuid.UUID, hash string) error {
	if err := fr.hook("SetPassword"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.SetPassword(ctx, db, id, hash)
}

func (fr *faultRepo) Unlock(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("Unlock"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Unlock(ctx, db, id)
}
