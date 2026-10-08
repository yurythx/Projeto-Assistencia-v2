package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/example/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewPostgresRepository()
	ctx := context.Background()
	p := pagination.New(1, 10, 10)
	calls := map[string]func(db database.DBTX) error{
		"Insert": func(db database.DBTX) error { return r.Insert(ctx, db, domain.Item{ID: uuid.New()}) },
		"Get":    func(db database.DBTX) error { _, err := r.Get(ctx, db, uuid.New()); return err },
		"List":   func(db database.DBTX) error { _, _, err := r.List(ctx, db, p); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	page := &dbtest.Seq{Row: countRow{}, Queries: []dbtest.QueryResult{{Err: dbtest.ErrInjected}}}
	if _, _, err := r.List(ctx, page, p); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("contagem ok, página falha: %v", err)
	}
	for _, rows := range []dbtest.QueryResult{{Rows: dbtest.BadRows()}, {Rows: dbtest.ErrRows()}} {
		if _, _, err := r.List(ctx, &dbtest.Seq{Row: countRow{}, Queries: []dbtest.QueryResult{rows}}, p); err == nil {
			t.Error("página ilegível ou interrompida")
		}
	}
}

type countRow struct{}

func (countRow) Scan(dest ...any) error {
	*(dest[0].(*int64)) = 0
	return nil
}
