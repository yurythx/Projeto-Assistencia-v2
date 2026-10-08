package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	p := pagination.New(1, 10, 10)
	calls := map[string]func(db database.DBTX) error{
		"Insert":        func(db database.DBTX) error { _, err := r.Insert(ctx, db, domain.Message{ID: id}); return err },
		"List":          func(db database.DBTX) error { _, _, err := r.List(ctx, db, domain.Filter{}, p); return err },
		"Get":           func(db database.DBTX) error { _, err := r.Get(ctx, db, id); return err },
		"UpdateTriage":  func(db database.DBTX) error { return r.UpdateTriage(ctx, db, id, domain.Triage{Status: "new"}) },
		"ActiveUser":    func(db database.DBTX) error { _, err := r.ActiveUser(ctx, db, id); return err },
		"ActiveUnidade": func(db database.DBTX) error { _, err := r.ActiveUnidade(ctx, db, id); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	if err := calls["UpdateTriage"](dbtest.ScanFail{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("triagem de mensagem inexistente: %v", err)
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
