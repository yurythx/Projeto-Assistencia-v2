package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/domain"
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

func (fr *faultRepo) List(ctx context.Context, db database.DBTX, f domain.Filter, p pagination.Params) ([]domain.Service, int64, error) {
	if err := fr.hook("List"); err != nil {
		var z0 []domain.Service
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.List(ctx, db, f, p)
}

func (fr *faultRepo) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Service, error) {
	if err := fr.hook("Get"); err != nil {
		var z0 domain.Service
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Get(ctx, db, id)
}

func (fr *faultRepo) GetBySlug(ctx context.Context, db database.DBTX, slug string) (domain.Service, error) {
	if err := fr.hook("GetBySlug"); err != nil {
		var z0 domain.Service
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetBySlug(ctx, db, slug)
}

func (fr *faultRepo) Save(ctx context.Context, db database.DBTX, s domain.Service) (domain.Service, error) {
	if err := fr.hook("Save"); err != nil {
		var z0 domain.Service
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Save(ctx, db, s)
}

func (fr *faultRepo) Delete(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("Delete"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Delete(ctx, db, id)
}

func (fr *faultRepo) Categories(ctx context.Context, db database.DBTX) ([]domain.Category, error) {
	if err := fr.hook("Categories"); err != nil {
		var z0 []domain.Category
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Categories(ctx, db)
}

func (fr *faultRepo) Search(ctx context.Context, db database.DBTX, query string, limit int) ([]domain.Service, []float64, error) {
	if err := fr.hook("Search"); err != nil {
		var z0 []domain.Service
		var z1 []float64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Search(ctx, db, query, limit)
}
