// Package infrastructure implementa o repositório do Mercúrio.
package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

func wrap(err error) error {
	if err == nil {
		return nil
	}
	if database.IsNoRows(err) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("mercurio: %w", err)
}

const roomCols = `r.id, r.kind, r.name, r.description, r.ad_group, r.departamento_id, r.archived, r.created_at,
	COALESCE((SELECT array_agg(m.user_id) FROM mercurio_room_members m WHERE m.room_id = r.id), '{}'),
	(SELECT max(created_at) FROM mercurio_messages mm WHERE mm.room_id = r.id AND mm.deleted_at IS NULL),
	(SELECT d.unidade_id FROM departamentos d WHERE d.id = r.departamento_id)`

func scanRoom(row interface{ Scan(...any) error }, extra ...any) (domain.Room, error) {
	var r domain.Room
	err := row.Scan(append([]any{&r.ID, &r.Kind, &r.Name, &r.Description, &r.ADGroup, &r.DepartamentoID, &r.Archived,
		&r.CreatedAt, &r.Members, &r.LastMessageAt, &r.UnidadeID}, extra...)...)
	return r, err
}

// Rooms lista as salas com contagem de não lidas de userID. O filtro de
// acesso (grupo/lotação) é aplicado no serviço (domain.CanAccess).
func (r *Repository) Rooms(ctx context.Context, db database.DBTX, userID uuid.UUID, includeArchived bool) ([]domain.Room, error) {
	rows, err := db.Query(ctx, `SELECT `+roomCols+`,
		(SELECT count(*) FROM mercurio_messages mm WHERE mm.room_id = r.id AND mm.deleted_at IS NULL AND mm.author_id <> $1
		   AND mm.created_at > COALESCE((SELECT last_read_at FROM mercurio_reads rd WHERE rd.room_id = r.id AND rd.user_id = $1), '-infinity'))
		FROM mercurio_rooms r
		WHERE ($2 OR NOT r.archived)
		  AND (r.kind <> 'direct' OR EXISTS (SELECT 1 FROM mercurio_room_members m WHERE m.room_id = r.id AND m.user_id = $1))
		ORDER BY r.kind = 'global' DESC, r.kind, r.name`, userID, includeArchived)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Room{}
	for rows.Next() {
		var unread int
		room, err := scanRoom(rows, &unread)
		if err != nil {
			return nil, wrap(err)
		}
		room.Unread = unread
		out = append(out, room)
	}
	return out, rows.Err()
}

func (r *Repository) Room(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Room, error) {
	room, err := scanRoom(db.QueryRow(ctx, `SELECT `+roomCols+` FROM mercurio_rooms r WHERE r.id = $1`, id))
	return room, wrap(err)
}

func (r *Repository) SaveRoom(ctx context.Context, db database.DBTX, room domain.Room, createdBy uuid.UUID) (domain.Room, error) {
	_, err := db.Exec(ctx, `INSERT INTO mercurio_rooms (id, kind, name, description, ad_group, departamento_id, archived, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, ad_group=EXCLUDED.ad_group,
		    departamento_id=EXCLUDED.departamento_id, archived=EXCLUDED.archived`,
		room.ID, room.Kind, room.Name, room.Description, room.ADGroup, room.DepartamentoID, room.Archived, createdBy)
	if err != nil {
		return domain.Room{}, wrap(err)
	}
	return r.Room(ctx, db, room.ID)
}

// DirectRoom obtém (ou cria) a sala direta entre a e b.
func (r *Repository) DirectRoom(ctx context.Context, db database.DBTX, a, b uuid.UUID, name string) (domain.Room, error) {
	key := domain.DMKey(a, b)
	var id uuid.UUID
	err := db.QueryRow(ctx, `INSERT INTO mercurio_rooms (kind, name, dm_key, created_by) VALUES ('direct', $1, $2, $3)
		ON CONFLICT (dm_key) DO UPDATE SET dm_key = EXCLUDED.dm_key RETURNING id`, name, key, a).Scan(&id)
	if err != nil {
		return domain.Room{}, wrap(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO mercurio_room_members (room_id, user_id) VALUES ($1,$2),($1,$3) ON CONFLICT DO NOTHING`, id, a, b); err != nil {
		return domain.Room{}, wrap(err)
	}
	return r.Room(ctx, db, id)
}

const msgCols = `m.id, m.room_id, m.author_id, COALESCE(NULLIF(u.display_name,''), u.username, ''),
	CASE WHEN m.deleted_at IS NULL THEN m.body ELSE '' END, m.created_at, m.edited_at, m.deleted_at IS NOT NULL`

func scanMsg(row interface{ Scan(...any) error }) (domain.Message, error) {
	var m domain.Message
	err := row.Scan(&m.ID, &m.RoomID, &m.AuthorID, &m.AuthorName, &m.Body, &m.CreatedAt, &m.EditedAt, &m.Deleted)
	return m, err
}

func (r *Repository) Messages(ctx context.Context, db database.DBTX, roomID uuid.UUID, before *time.Time, limit int) ([]domain.Message, error) {
	rows, err := db.Query(ctx, `SELECT `+msgCols+` FROM mercurio_messages m LEFT JOIN users u ON u.id = m.author_id
		WHERE m.room_id = $1 AND ($2::timestamptz IS NULL OR m.created_at < $2)
		ORDER BY m.created_at DESC LIMIT $3`, roomID, before, limit)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Message{}
	for rows.Next() {
		m, err := scanMsg(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, m)
	}
	// Devolve em ordem cronológica.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (r *Repository) InsertMessage(ctx context.Context, db database.DBTX, m domain.Message) (domain.Message, error) {
	if _, err := db.Exec(ctx, `INSERT INTO mercurio_messages (id, room_id, author_id, body) VALUES ($1,$2,$3,$4)`,
		m.ID, m.RoomID, m.AuthorID, m.Body); err != nil {
		return domain.Message{}, wrap(err)
	}
	return r.Message(ctx, db, m.ID)
}

func (r *Repository) Message(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Message, error) {
	m, err := scanMsg(db.QueryRow(ctx, `SELECT `+msgCols+` FROM mercurio_messages m LEFT JOIN users u ON u.id = m.author_id WHERE m.id = $1`, id))
	return m, wrap(err)
}

func (r *Repository) UpdateMessage(ctx context.Context, db database.DBTX, id uuid.UUID, body string, deleted bool) (domain.Message, error) {
	var err error
	if deleted {
		_, err = db.Exec(ctx, `UPDATE mercurio_messages SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	} else {
		_, err = db.Exec(ctx, `UPDATE mercurio_messages SET body = $2, edited_at = now() WHERE id = $1 AND deleted_at IS NULL`, id, body)
	}
	if err != nil {
		return domain.Message{}, wrap(err)
	}
	return r.Message(ctx, db, id)
}

func (r *Repository) MarkRead(ctx context.Context, db database.DBTX, roomID, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `INSERT INTO mercurio_reads (room_id, user_id, last_read_at) VALUES ($1,$2,now())
		ON CONFLICT (room_id, user_id) DO UPDATE SET last_read_at = now()`, roomID, userID)
	return wrap(err)
}

func (r *Repository) UserName(ctx context.Context, db database.DBTX, id uuid.UUID) (string, error) {
	var name string
	err := db.QueryRow(ctx, `SELECT COALESCE(NULLIF(display_name,''), username) FROM users WHERE id = $1 AND active`, id).Scan(&name)
	return name, wrap(err)
}
