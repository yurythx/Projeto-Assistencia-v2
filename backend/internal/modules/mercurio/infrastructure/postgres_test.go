package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	calls := map[string]func(db database.DBTX) error{
		"Rooms":         func(db database.DBTX) error { _, err := r.Rooms(ctx, db, id, true); return err },
		"Room":          func(db database.DBTX) error { _, err := r.Room(ctx, db, id); return err },
		"SaveRoom":      func(db database.DBTX) error { _, err := r.SaveRoom(ctx, db, domain.Room{ID: id}, id); return err },
		"DirectRoom":    func(db database.DBTX) error { _, err := r.DirectRoom(ctx, db, id, uuid.New(), "DM"); return err },
		"Messages":      func(db database.DBTX) error { _, err := r.Messages(ctx, db, id, nil, 10); return err },
		"InsertMessage": func(db database.DBTX) error { _, err := r.InsertMessage(ctx, db, domain.Message{ID: id}); return err },
		"Message":       func(db database.DBTX) error { _, err := r.Message(ctx, db, id); return err },
		"Edit":          func(db database.DBTX) error { _, err := r.UpdateMessage(ctx, db, id, "x", false); return err },
		"Delete":        func(db database.DBTX) error { _, err := r.UpdateMessage(ctx, db, id, "", true); return err },
		"MarkRead":      func(db database.DBTX) error { return r.MarkRead(ctx, db, id, id) },
		"UserName":      func(db database.DBTX) error { _, err := r.UserName(ctx, db, id); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"Rooms", "Messages"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro durante a leitura: %v", name, err)
		}
	}
	// A gravação funciona, a releitura falha.
	for _, name := range []string{"SaveRoom", "InsertMessage", "Edit", "Delete"} {
		if err := calls[name](&dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}); err == nil {
			t.Errorf("%s: falha ao reler deveria subir", name)
		}
	}
	// Sala direta: a sala nasce, os participantes não.
	if _, err := r.DirectRoom(ctx, &dbtest.Seq{Row: idRow{id}}, id, uuid.New(), "DM"); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("falha ao incluir os participantes da conversa: %v", err)
	}
}

type idRow struct{ id uuid.UUID }

func (r idRow) Scan(dest ...any) error {
	*(dest[0].(*uuid.UUID)) = r.id
	return nil
}
