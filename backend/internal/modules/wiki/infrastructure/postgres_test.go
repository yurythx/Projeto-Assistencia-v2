package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/wiki/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	calls := map[string]func(db database.DBTX) error{
		"Tree":            func(db database.DBTX) error { _, err := r.Tree(ctx, db); return err },
		"Get":             func(db database.DBTX) error { _, err := r.Get(ctx, db, id); return err },
		"GetBySlug":       func(db database.DBTX) error { _, err := r.GetBySlug(ctx, db, "x"); return err },
		"Breadcrumbs":     func(db database.DBTX) error { _, err := r.Breadcrumbs(ctx, db, id); return err },
		"Insert":          func(db database.DBTX) error { return r.Insert(ctx, db, domain.Page{ID: id}) },
		"UpdateIfVersion": func(db database.DBTX) error { return r.UpdateIfVersion(ctx, db, domain.Page{ID: id}, 1) },
		"Delete":          func(db database.DBTX) error { return r.Delete(ctx, db, id) },
		"IsDescendant":    func(db database.DBTX) error { _, err := r.IsDescendant(ctx, db, id, id); return err },
		"AddRevision":     func(db database.DBTX) error { return r.AddRevision(ctx, db, domain.Revision{PageID: id}) },
		"Revisions":       func(db database.DBTX) error { _, err := r.Revisions(ctx, db, id); return err },
		"Revision":        func(db database.DBTX) error { _, err := r.Revision(ctx, db, id, 1); return err },
		"Search":          func(db database.DBTX) error { _, _, err := r.Search(ctx, db, "x", 5); return err },
	}
	// A página foi lida, mas o público-alvo não (ADR 014).
	if _, err := r.Get(ctx, &dbtest.Seq{Row: rowOK{}}, id); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("público-alvo com o banco fora: %v", err)
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"Tree", "Breadcrumbs", "Revisions", "Search"} {
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
	// Versão diferente da esperada (ou página sumida): conflito de edição.
	if err := calls["UpdateIfVersion"](dbtest.ScanFail{}); !errors.Is(err, domain.ErrStale) {
		t.Errorf("edição concorrente: %v", err)
	}
}

// rowOK é uma linha que se lê sem erro (valores zerados).
type rowOK struct{}

func (rowOK) Scan(...any) error { return nil }
