package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/example/domain"
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

func (fr *faultRepo) Insert(ctx context.Context, db database.DBTX, item domain.Item) error {
	if err := fr.hook("Insert"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Insert(ctx, db, item)
}

func (fr *faultRepo) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Item, error) {
	if err := fr.hook("Get"); err != nil {
		var z0 domain.Item
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Get(ctx, db, id)
}

func (fr *faultRepo) List(ctx context.Context, db database.DBTX, p pagination.Params) ([]domain.Item, int64, error) {
	if err := fr.hook("List"); err != nil {
		var z0 []domain.Item
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.List(ctx, db, p)
}
