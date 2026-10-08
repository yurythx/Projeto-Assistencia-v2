package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/domain"
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

func (fr *faultRepo) List(ctx context.Context, db database.DBTX, f domain.Filter, p pagination.Params) ([]domain.Person, int64, error) {
	if err := fr.hook("List"); err != nil {
		var z0 []domain.Person
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.List(ctx, db, f, p)
}

func (fr *faultRepo) Get(ctx context.Context, db database.DBTX, userID uuid.UUID) (domain.Person, error) {
	if err := fr.hook("Get"); err != nil {
		var z0 domain.Person
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Get(ctx, db, userID)
}

func (fr *faultRepo) SaveProfile(ctx context.Context, db database.DBTX, userID uuid.UUID, in domain.ProfileInput, keepLotacao bool) error {
	if err := fr.hook("SaveProfile"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.SaveProfile(ctx, db, userID, in, keepLotacao)
}

func (fr *faultRepo) Sectors(ctx context.Context, db database.DBTX, query string) ([]domain.Sector, error) {
	if err := fr.hook("Sectors"); err != nil {
		var z0 []domain.Sector
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Sectors(ctx, db, query)
}

func (fr *faultRepo) DepartamentoUnidade(ctx context.Context, db database.DBTX, departamentoID uuid.UUID) (uuid.UUID, error) {
	if err := fr.hook("DepartamentoUnidade"); err != nil {
		var z0 uuid.UUID
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.DepartamentoUnidade(ctx, db, departamentoID)
}

func (fr *faultRepo) DeleteProfile(ctx context.Context, db database.DBTX, userID uuid.UUID) error {
	if err := fr.hook("DeleteProfile"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteProfile(ctx, db, userID)
}
