package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/domain"
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

func (fr *faultRepo) Insert(ctx context.Context, db database.DBTX, m domain.Message) (domain.Message, error) {
	if err := fr.hook("Insert"); err != nil {
		var z0 domain.Message
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Insert(ctx, db, m)
}

func (fr *faultRepo) List(ctx context.Context, db database.DBTX, f domain.Filter, p pagination.Params) ([]domain.Summary, int64, error) {
	if err := fr.hook("List"); err != nil {
		var z0 []domain.Summary
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.List(ctx, db, f, p)
}

func (fr *faultRepo) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Message, error) {
	if err := fr.hook("Get"); err != nil {
		var z0 domain.Message
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Get(ctx, db, id)
}

func (fr *faultRepo) UpdateTriage(ctx context.Context, db database.DBTX, id uuid.UUID, t domain.Triage) error {
	if err := fr.hook("UpdateTriage"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateTriage(ctx, db, id, t)
}

func (fr *faultRepo) ActiveUser(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	if err := fr.hook("ActiveUser"); err != nil {
		var z0 bool
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ActiveUser(ctx, db, id)
}

func (fr *faultRepo) ActiveUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	if err := fr.hook("ActiveUnidade"); err != nil {
		var z0 bool
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ActiveUnidade(ctx, db, id)
}
