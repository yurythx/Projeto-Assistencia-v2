package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/domain"
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

func (fr *faultRepo) Rooms(ctx context.Context, db database.DBTX, userID uuid.UUID, includeArchived bool) ([]domain.Room, error) {
	if err := fr.hook("Rooms"); err != nil {
		var z0 []domain.Room
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Rooms(ctx, db, userID, includeArchived)
}

func (fr *faultRepo) Room(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Room, error) {
	if err := fr.hook("Room"); err != nil {
		var z0 domain.Room
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Room(ctx, db, id)
}

func (fr *faultRepo) SaveRoom(ctx context.Context, db database.DBTX, r domain.Room, createdBy uuid.UUID) (domain.Room, error) {
	if err := fr.hook("SaveRoom"); err != nil {
		var z0 domain.Room
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.SaveRoom(ctx, db, r, createdBy)
}

func (fr *faultRepo) DirectRoom(ctx context.Context, db database.DBTX, a uuid.UUID, b uuid.UUID, name string) (domain.Room, error) {
	if err := fr.hook("DirectRoom"); err != nil {
		var z0 domain.Room
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.DirectRoom(ctx, db, a, b, name)
}

func (fr *faultRepo) Messages(ctx context.Context, db database.DBTX, roomID uuid.UUID, before *time.Time, limit int) ([]domain.Message, error) {
	if err := fr.hook("Messages"); err != nil {
		var z0 []domain.Message
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Messages(ctx, db, roomID, before, limit)
}

func (fr *faultRepo) InsertMessage(ctx context.Context, db database.DBTX, m domain.Message) (domain.Message, error) {
	if err := fr.hook("InsertMessage"); err != nil {
		var z0 domain.Message
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertMessage(ctx, db, m)
}

func (fr *faultRepo) Message(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Message, error) {
	if err := fr.hook("Message"); err != nil {
		var z0 domain.Message
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Message(ctx, db, id)
}

func (fr *faultRepo) UpdateMessage(ctx context.Context, db database.DBTX, id uuid.UUID, body string, deleted bool) (domain.Message, error) {
	if err := fr.hook("UpdateMessage"); err != nil {
		var z0 domain.Message
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateMessage(ctx, db, id, body, deleted)
}

func (fr *faultRepo) MarkRead(ctx context.Context, db database.DBTX, roomID uuid.UUID, userID uuid.UUID) error {
	if err := fr.hook("MarkRead"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.MarkRead(ctx, db, roomID, userID)
}

func (fr *faultRepo) UserName(ctx context.Context, db database.DBTX, id uuid.UUID) (string, error) {
	if err := fr.hook("UserName"); err != nil {
		var z0 string
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UserName(ctx, db, id)
}
