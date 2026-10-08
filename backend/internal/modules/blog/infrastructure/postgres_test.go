package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/publico"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	p := pagination.New(1, 10, 10)
	calls := map[string]func(db database.DBTX) error{
		"List":      func(db database.DBTX) error { _, _, err := r.List(ctx, db, domain.Filter{}, p); return err },
		"Get":       func(db database.DBTX) error { _, err := r.Get(ctx, db, id); return err },
		"GetBySlug": func(db database.DBTX) error { _, err := r.GetBySlug(ctx, db, "x"); return err },
		"Insert":    func(db database.DBTX) error { _, err := r.Insert(ctx, db, domain.Post{ID: id}); return err },
		"Update":    func(db database.DBTX) error { _, err := r.Update(ctx, db, domain.Post{ID: id}); return err },
		"Delete":    func(db database.DBTX) error { return r.Delete(ctx, db, id) },
		"Search":    func(db database.DBTX) error { _, _, err := r.Search(ctx, db, "x", 5, publico.Leitura{}); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	// O post foi lido, mas o público-alvo não (ADR 014).
	if _, err := r.Get(ctx, &dbtest.Seq{Row: rowOK{}}, id); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("público-alvo com o banco fora: %v", err)
	}
	if err := calls["Search"](dbtest.ScanFail{}); err == nil {
		t.Error("busca com linha ilegível deveria falhar")
	}
	if err := calls["Search"](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
		t.Error("busca interrompida")
	}
	for _, name := range []string{"Delete", "Update"} {
		if err := calls[name](dbtest.ScanFail{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s sem linha afetada: %v", name, err)
		}
	}
	for _, name := range []string{"Insert", "Update"} {
		if err := calls[name](&dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}); err == nil {
			t.Errorf("%s gravou mas não releu", name)
		}
	}
	page := &dbtest.Seq{Row: countRow{}, Queries: []dbtest.QueryResult{{Err: dbtest.ErrInjected}}}
	if _, _, err := r.List(ctx, page, domain.Filter{}, p); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("contagem ok, página falha: %v", err)
	}
	for _, rows := range []dbtest.QueryResult{{Rows: dbtest.BadRows()}, {Rows: dbtest.ErrRows()}} {
		if _, _, err := r.List(ctx, &dbtest.Seq{Row: countRow{}, Queries: []dbtest.QueryResult{rows}}, domain.Filter{}, p); err == nil {
			t.Error("página ilegível ou interrompida")
		}
	}
}

type countRow struct{}

func (countRow) Scan(dest ...any) error {
	*(dest[0].(*int64)) = 0
	return nil
}

// rowOK é uma linha que se lê sem erro (valores zerados).
type rowOK struct{}

func (rowOK) Scan(...any) error { return nil }
