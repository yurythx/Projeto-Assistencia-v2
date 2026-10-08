package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/calendar/domain"
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

func (fr *faultRepo) ListRooms(ctx context.Context, db database.DBTX, onlyActive bool, unidades []uuid.UUID) ([]domain.Room, error) {
	if err := fr.hook("ListRooms"); err != nil {
		var z0 []domain.Room
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListRooms(ctx, db, onlyActive, unidades)
}

func (fr *faultRepo) GetRoom(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Room, error) {
	if err := fr.hook("GetRoom"); err != nil {
		var z0 domain.Room
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetRoom(ctx, db, id)
}

func (fr *faultRepo) SaveRoom(ctx context.Context, db database.DBTX, r domain.Room) (domain.Room, error) {
	if err := fr.hook("SaveRoom"); err != nil {
		var z0 domain.Room
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.SaveRoom(ctx, db, r)
}

func (fr *faultRepo) DeleteRoom(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteRoom"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteRoom(ctx, db, id)
}

func (fr *faultRepo) RoomHasUpcoming(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	if err := fr.hook("RoomHasUpcoming"); err != nil {
		var z0 bool
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.RoomHasUpcoming(ctx, db, id)
}

func (fr *faultRepo) RoomBusy(ctx context.Context, db database.DBTX, roomID uuid.UUID, from time.Time, to time.Time) ([]domain.Busy, error) {
	if err := fr.hook("RoomBusy"); err != nil {
		var z0 []domain.Busy
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.RoomBusy(ctx, db, roomID, from, to)
}

func (fr *faultRepo) ListEvents(ctx context.Context, db database.DBTX, f domain.EventFilter) ([]domain.Event, error) {
	if err := fr.hook("ListEvents"); err != nil {
		var z0 []domain.Event
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListEvents(ctx, db, f)
}

func (fr *faultRepo) GetEvent(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Event, error) {
	if err := fr.hook("GetEvent"); err != nil {
		var z0 domain.Event
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetEvent(ctx, db, id)
}

func (fr *faultRepo) SaveEvent(ctx context.Context, db database.DBTX, e domain.Event) (domain.Event, error) {
	if err := fr.hook("SaveEvent"); err != nil {
		var z0 domain.Event
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.SaveEvent(ctx, db, e)
}

func (fr *faultRepo) DeleteEvent(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteEvent"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteEvent(ctx, db, id)
}
