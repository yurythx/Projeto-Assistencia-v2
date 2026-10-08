package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/calendar/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	calls := map[string]func(db database.DBTX) error{
		"ListRooms":       func(db database.DBTX) error { _, err := r.ListRooms(ctx, db, true, nil); return err },
		"GetRoom":         func(db database.DBTX) error { _, err := r.GetRoom(ctx, db, id); return err },
		"SaveRoom":        func(db database.DBTX) error { _, err := r.SaveRoom(ctx, db, domain.Room{ID: id}); return err },
		"DeleteRoom":      func(db database.DBTX) error { return r.DeleteRoom(ctx, db, id) },
		"RoomHasUpcoming": func(db database.DBTX) error { _, err := r.RoomHasUpcoming(ctx, db, id); return err },
		"RoomBusy":        func(db database.DBTX) error { _, err := r.RoomBusy(ctx, db, id, time.Now(), time.Now()); return err },
		"ListEvents":      func(db database.DBTX) error { _, err := r.ListEvents(ctx, db, domain.EventFilter{}); return err },
		"GetEvent":        func(db database.DBTX) error { _, err := r.GetEvent(ctx, db, id); return err },
		"SaveEvent":       func(db database.DBTX) error { _, err := r.SaveEvent(ctx, db, domain.Event{ID: id}); return err },
		"DeleteEvent":     func(db database.DBTX) error { return r.DeleteEvent(ctx, db, id) },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"ListRooms", "RoomBusy", "ListEvents"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro durante a leitura: %v", name, err)
		}
	}
	for _, name := range []string{"DeleteRoom", "DeleteEvent"} {
		if err := calls[name](dbtest.ScanFail{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s sem linha afetada: %v", name, err)
		}
	}
	// Público-alvo (ADR 014): o evento foi lido/gravado, mas o público não.
	if _, err := r.GetEvent(ctx, &dbtest.Seq{Row: rowOK{}}, id); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("público-alvo com o banco fora: %v", err)
	}
	if err := calls["SaveEvent"](&dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK, {Err: dbtest.ErrInjected}}}); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("gravar o público com o banco fora: %v", err)
	}
	if err := calls["SaveEvent"](&dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}); err == nil {
		t.Error("gravou mas não releu o evento")
	}
}

// As restrições do banco viram erros de domínio com a mensagem certa.
func TestConstraintViolationsMapToDomainErrors(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	org := dbtest.User(t, pool)
	now := time.Now().UTC()
	base := domain.Event{ID: uuid.New(), Title: "x", Visibility: "internal", Status: "confirmed", OrganizerID: org, StartsAt: now, EndsAt: now.Add(time.Hour)}
	back := base
	back.EndsAt = now.Add(-time.Hour)
	if _, err := r.SaveEvent(ctx, pool, back); !errors.Is(err, domain.ErrInvalidRange) {
		t.Errorf("fim antes do início: %v", err)
	}
	vis := base
	vis.Visibility = "secreta"
	if _, err := r.SaveEvent(ctx, pool, vis); !errors.Is(err, domain.ErrInvalidValue) {
		t.Errorf("visibilidade fora da lista: %v", err)
	}
	if _, err := r.SaveRoom(ctx, pool, domain.Room{ID: uuid.New(), Name: "Negativa " + uuid.NewString()[:6], Capacity: -1, Resources: []string{}}); !errors.Is(err, domain.ErrInvalidValue) {
		t.Errorf("capacidade negativa: %v", err)
	}
}

// rowOK é uma linha que se lê sem erro (valores zerados).
type rowOK struct{}

func (rowOK) Scan(...any) error { return nil }
