package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	p := pagination.New(1, 10, 10)
	calls := map[string]func(db database.DBTX) error{
		"List": func(db database.DBTX) error {
			_, _, err := r.List(ctx, db, domain.Filter{Status: "all"}, p)
			return err
		},
		"Get":        func(db database.DBTX) error { _, err := r.Get(ctx, db, id); return err },
		"GetBySlug":  func(db database.DBTX) error { _, err := r.GetBySlug(ctx, db, "x"); return err },
		"Save":       func(db database.DBTX) error { _, err := r.Save(ctx, db, domain.Service{ID: id}); return err },
		"Delete":     func(db database.DBTX) error { return r.Delete(ctx, db, id) },
		"Categories": func(db database.DBTX) error { _, err := r.Categories(ctx, db); return err },
		"Search":     func(db database.DBTX) error { _, _, err := r.Search(ctx, db, "x", 5); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"Categories", "Search"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro durante a leitura: %v", name, err)
		}
	}
	if err := calls["Delete"](dbtest.ScanFail{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("excluir inexistente: %v", err)
	}
	if err := calls["Save"](&dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}); err == nil {
		t.Error("gravou mas não releu")
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
