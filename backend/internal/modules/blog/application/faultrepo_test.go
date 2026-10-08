package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/publico"
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

func (fr *faultRepo) List(ctx context.Context, db database.DBTX, f domain.Filter, p pagination.Params) ([]domain.Post, int64, error) {
	if err := fr.hook("List"); err != nil {
		var z0 []domain.Post
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.List(ctx, db, f, p)
}

func (fr *faultRepo) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Post, error) {
	if err := fr.hook("Get"); err != nil {
		var z0 domain.Post
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Get(ctx, db, id)
}

func (fr *faultRepo) GetBySlug(ctx context.Context, db database.DBTX, slug string) (domain.Post, error) {
	if err := fr.hook("GetBySlug"); err != nil {
		var z0 domain.Post
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetBySlug(ctx, db, slug)
}

func (fr *faultRepo) Insert(ctx context.Context, db database.DBTX, p domain.Post) (domain.Post, error) {
	if err := fr.hook("Insert"); err != nil {
		var z0 domain.Post
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Insert(ctx, db, p)
}

func (fr *faultRepo) Update(ctx context.Context, db database.DBTX, p domain.Post) (domain.Post, error) {
	if err := fr.hook("Update"); err != nil {
		var z0 domain.Post
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Update(ctx, db, p)
}

func (fr *faultRepo) Delete(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("Delete"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Delete(ctx, db, id)
}

func (fr *faultRepo) Search(ctx context.Context, db database.DBTX, query string, limit int, l publico.Leitura) ([]domain.Post, []float64, error) {
	if err := fr.hook("Search"); err != nil {
		var z0 []domain.Post
		var z1 []float64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Search(ctx, db, query, limit, l)
}

func (fr *faultRepo) SetPublico(ctx context.Context, db database.DBTX, id uuid.UUID, p auth.Publico) error {
	if err := fr.hook("SetPublico"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.SetPublico(ctx, db, id, p)
}
