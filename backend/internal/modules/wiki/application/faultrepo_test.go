package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/wiki/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
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

func (fr *faultRepo) Tree(ctx context.Context, db database.DBTX) ([]domain.Page, error) {
	if err := fr.hook("Tree"); err != nil {
		var z0 []domain.Page
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Tree(ctx, db)
}

func (fr *faultRepo) Get(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Page, error) {
	if err := fr.hook("Get"); err != nil {
		var z0 domain.Page
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Get(ctx, db, id)
}

func (fr *faultRepo) GetBySlug(ctx context.Context, db database.DBTX, slug string) (domain.Page, error) {
	if err := fr.hook("GetBySlug"); err != nil {
		var z0 domain.Page
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetBySlug(ctx, db, slug)
}

func (fr *faultRepo) Breadcrumbs(ctx context.Context, db database.DBTX, id uuid.UUID) ([]domain.Page, error) {
	if err := fr.hook("Breadcrumbs"); err != nil {
		var z0 []domain.Page
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Breadcrumbs(ctx, db, id)
}

func (fr *faultRepo) Insert(ctx context.Context, db database.DBTX, p domain.Page) error {
	if err := fr.hook("Insert"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Insert(ctx, db, p)
}

func (fr *faultRepo) UpdateIfVersion(ctx context.Context, db database.DBTX, p domain.Page, expected int) error {
	if err := fr.hook("UpdateIfVersion"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateIfVersion(ctx, db, p, expected)
}

func (fr *faultRepo) Delete(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("Delete"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Delete(ctx, db, id)
}

func (fr *faultRepo) IsDescendant(ctx context.Context, db database.DBTX, candidate uuid.UUID, ancestor uuid.UUID) (bool, error) {
	if err := fr.hook("IsDescendant"); err != nil {
		var z0 bool
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.IsDescendant(ctx, db, candidate, ancestor)
}

func (fr *faultRepo) AddRevision(ctx context.Context, db database.DBTX, r domain.Revision) error {
	if err := fr.hook("AddRevision"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.AddRevision(ctx, db, r)
}

func (fr *faultRepo) Revisions(ctx context.Context, db database.DBTX, pageID uuid.UUID) ([]domain.Revision, error) {
	if err := fr.hook("Revisions"); err != nil {
		var z0 []domain.Revision
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Revisions(ctx, db, pageID)
}

func (fr *faultRepo) Revision(ctx context.Context, db database.DBTX, pageID uuid.UUID, version int) (domain.Revision, error) {
	if err := fr.hook("Revision"); err != nil {
		var z0 domain.Revision
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Revision(ctx, db, pageID, version)
}

func (fr *faultRepo) Search(ctx context.Context, db database.DBTX, q string, limit int) ([]domain.Page, []float64, error) {
	if err := fr.hook("Search"); err != nil {
		var z0 []domain.Page
		var z1 []float64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Search(ctx, db, q, limit)
}

func (fr *faultRepo) Publicos(ctx context.Context, db database.DBTX) (map[uuid.UUID]auth.Publico, error) {
	if err := fr.hook("Publicos"); err != nil {
		var z0 map[uuid.UUID]auth.Publico
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Publicos(ctx, db)
}

func (fr *faultRepo) SetPublico(ctx context.Context, db database.DBTX, id uuid.UUID, p auth.Publico) error {
	if err := fr.hook("SetPublico"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.SetPublico(ctx, db, id, p)
}
