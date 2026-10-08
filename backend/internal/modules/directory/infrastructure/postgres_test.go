package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	p := pagination.New(1, 10, 10)
	calls := map[string]func(db database.DBTX) error{
		"DeleteProfile":       func(db database.DBTX) error { return r.DeleteProfile(ctx, db, uuid.New()) },
		"List":                func(db database.DBTX) error { _, _, err := r.List(ctx, db, domain.Filter{}, p); return err },
		"Get":                 func(db database.DBTX) error { _, err := r.Get(ctx, db, id); return err },
		"SaveProfile":         func(db database.DBTX) error { return r.SaveProfile(ctx, db, id, domain.ProfileInput{}, true) },
		"Sectors":             func(db database.DBTX) error { _, err := r.Sectors(ctx, db, ""); return err },
		"DepartamentoUnidade": func(db database.DBTX) error { _, err := r.DepartamentoUnidade(ctx, db, id); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	if err := calls["Sectors"](dbtest.ScanFail{}); err == nil {
		t.Error("setores com linha ilegível")
	}
	if err := calls["Sectors"](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
		t.Error("setores interrompidos")
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
