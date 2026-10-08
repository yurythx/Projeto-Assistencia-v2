// Package infrastructure implementa o repositório da Agenda.
package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/modules/calendar/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/publico"
)

// tabelaPublico é onde fica o público-alvo dos eventos (ADR 014).
var tabelaPublico = publico.Tabela{Nome: "calendar_event_publico", Coluna: "event_id"}

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case database.IsNoRows(err):
		return domain.ErrNotFound
	case database.IsExclusionViolation(err):
		return domain.ErrRoomConflict
	case database.IsUniqueViolation(err):
		return domain.ErrDuplicate
	case database.IsCheckViolation(err):
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "calendar_events_range_check" {
			return domain.ErrInvalidRange
		}
		return domain.ErrInvalidValue
	}
	return fmt.Errorf("calendar: %w", err)
}

const roomCols = `id, name, location, capacity, resources, active, created_at, updated_at, unidade_id`

func scanRoom(row interface{ Scan(...any) error }) (domain.Room, error) {
	var r domain.Room
	err := row.Scan(&r.ID, &r.Name, &r.Location, &r.Capacity, &r.Resources, &r.Active, &r.CreatedAt, &r.UpdatedAt, &r.UnidadeID)
	return r, err
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func (r *Repository) ListRooms(ctx context.Context, db database.DBTX, onlyActive bool, unidades []uuid.UUID) ([]domain.Room, error) {
	rows, err := db.Query(ctx, `SELECT `+roomCols+` FROM calendar_rooms WHERE (NOT $1 OR active OR unidade_id = ANY($2::uuid[])) ORDER BY name`,
		onlyActive, uuidStrings(unidades))
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Room{}
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, room)
	}
	return out, rows.Err()
}

func (r *Repository) GetRoom(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Room, error) {
	room, err := scanRoom(db.QueryRow(ctx, `SELECT `+roomCols+` FROM calendar_rooms WHERE id = $1`, id))
	return room, wrap(err)
}

func (r *Repository) SaveRoom(ctx context.Context, db database.DBTX, room domain.Room) (domain.Room, error) {
	out, err := scanRoom(db.QueryRow(ctx, `
		INSERT INTO calendar_rooms (id, name, location, capacity, resources, active, unidade_id) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, location=EXCLUDED.location, capacity=EXCLUDED.capacity,
		    resources=EXCLUDED.resources, active=EXCLUDED.active, unidade_id=EXCLUDED.unidade_id
		RETURNING `+roomCols, room.ID, room.Name, room.Location, room.Capacity, room.Resources, room.Active, room.UnidadeID))
	return out, wrap(err)
}

func (r *Repository) DeleteRoom(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM calendar_rooms WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}

func (r *Repository) RoomHasUpcoming(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM calendar_events WHERE room_id = $1 AND status = 'confirmed' AND ends_at > now())`, id).Scan(&ok)
	return ok, wrap(err)
}

func (r *Repository) RoomBusy(ctx context.Context, db database.DBTX, roomID uuid.UUID, from, to time.Time) ([]domain.Busy, error) {
	rows, err := db.Query(ctx, `
		SELECT e.id, CASE WHEN e.visibility = 'private' OR NOT `+tabelaPublico.SemPublico("e.id")+` THEN 'Reservado' ELSE e.title END,
		       e.starts_at, e.ends_at
		FROM calendar_events e
		WHERE e.room_id = $1 AND e.status = 'confirmed' AND tstzrange(e.starts_at, e.ends_at, '[)') && tstzrange($2, $3, '[)')
		ORDER BY e.starts_at`, roomID, from, to)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Busy{}
	for rows.Next() {
		var b domain.Busy
		if err := rows.Scan(&b.EventID, &b.Title, &b.StartsAt, &b.EndsAt); err != nil {
			return nil, wrap(err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

const eventCols = `e.id, e.title, e.description, e.location, e.room_id, COALESCE(r.name,''), e.starts_at, e.ends_at,
	e.all_day, e.visibility, e.status, e.organizer_id, COALESCE(NULLIF(u.display_name,''), u.username, ''),
	e.created_at, e.updated_at, r.unidade_id`

const eventFrom = ` FROM calendar_events e LEFT JOIN calendar_rooms r ON r.id = e.room_id LEFT JOIN users u ON u.id = e.organizer_id `

func scanEvent(row interface{ Scan(...any) error }) (domain.Event, error) {
	var e domain.Event
	err := row.Scan(&e.ID, &e.Title, &e.Description, &e.Location, &e.RoomID, &e.RoomName, &e.StartsAt, &e.EndsAt,
		&e.AllDay, &e.Visibility, &e.Status, &e.OrganizerID, &e.OrganizerName, &e.CreatedAt, &e.UpdatedAt, &e.RoomUnidadeID)
	return e, err
}

func (r *Repository) ListEvents(ctx context.Context, db database.DBTX, f domain.EventFilter) ([]domain.Event, error) {
	args := []any{f.From, f.To, f.RoomID, f.IncludeCancel, f.OnlyPublic, f.SeeAll, f.ViewerID, uuidStrings(f.SeeUnidades)}
	// Público-alvo (ADR 014): no site, só sem público; dentro, quem está no
	// público, o organizador e a gestão da sala.
	leitura := tabelaPublico.Condicao("e.id", "e.organizer_id", "r.unidade_id",
		publico.Leitura{Leitor: f.Leitor, Autor: f.ViewerID, Tudo: f.SeeAll, Geridas: f.SeeUnidades}, &args)
	rows, err := db.Query(ctx, `SELECT `+eventCols+eventFrom+`
		WHERE tstzrange(e.starts_at, e.ends_at, '[)') && tstzrange($1, $2, '[)')
		  AND ($3::uuid IS NULL OR e.room_id = $3)
		  AND ($4 OR e.status = 'confirmed')
		  AND CASE
		        WHEN $5 THEN e.visibility = 'public' AND `+tabelaPublico.SemPublico("e.id")+`
		        WHEN $6 THEN true
		        ELSE (e.visibility <> 'private' OR e.organizer_id = $7 OR r.unidade_id = ANY($8::uuid[])) AND `+leitura+`
		      END
		ORDER BY e.starts_at LIMIT 2000`, args...)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(err)
	}
	rows.Close()
	return out, wrap(comPublico(ctx, db, out))
}

// comPublico carrega o público-alvo dos eventos.
func comPublico(ctx context.Context, db database.DBTX, events []domain.Event) error {
	ids := make([]uuid.UUID, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	m, err := tabelaPublico.Carregar(ctx, db, ids)
	if err != nil {
		return err
	}
	for i := range events {
		events[i].Publico = m[events[i].ID]
	}
	return nil
}

func (r *Repository) GetEvent(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Event, error) {
	e, err := scanEvent(db.QueryRow(ctx, `SELECT `+eventCols+eventFrom+`WHERE e.id = $1`, id))
	if err != nil {
		return e, wrap(err)
	}
	one := []domain.Event{e}
	if err := comPublico(ctx, db, one); err != nil {
		return domain.Event{}, wrap(err)
	}
	return one[0], nil
}

func (r *Repository) SaveEvent(ctx context.Context, db database.DBTX, e domain.Event) (domain.Event, error) {
	_, err := db.Exec(ctx, `
		INSERT INTO calendar_events (id, title, description, location, room_id, starts_at, ends_at, all_day, visibility, status, organizer_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title, description=EXCLUDED.description, location=EXCLUDED.location,
		    room_id=EXCLUDED.room_id, starts_at=EXCLUDED.starts_at, ends_at=EXCLUDED.ends_at, all_day=EXCLUDED.all_day,
		    visibility=EXCLUDED.visibility, status=EXCLUDED.status`,
		e.ID, e.Title, e.Description, e.Location, e.RoomID, e.StartsAt, e.EndsAt, e.AllDay, e.Visibility, e.Status, e.OrganizerID)
	if err != nil {
		return domain.Event{}, wrap(err)
	}
	if err := tabelaPublico.Gravar(ctx, db, e.ID, e.Publico); err != nil {
		return domain.Event{}, err
	}
	return r.GetEvent(ctx, db, e.ID)
}

func (r *Repository) DeleteEvent(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM calendar_events WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}
