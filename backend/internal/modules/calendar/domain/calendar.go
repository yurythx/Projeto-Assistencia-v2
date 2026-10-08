// Package domain define a Agenda corporativa: eventos e salas.
package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/publico"
)

var (
	ErrNotFound     = errors.New("calendar: registro não encontrado")
	ErrRoomConflict = errors.New("calendar: sala já reservada no horário")
	ErrInvalidRange = errors.New("calendar: intervalo inválido")
	ErrDuplicate    = errors.New("calendar: registro duplicado")
	ErrInvalidValue = errors.New("calendar: valor fora do permitido")
	ErrCancelled    = errors.New("calendar: evento cancelado não pode ser alterado; crie um novo")
	ErrRoomInUse    = errors.New("calendar: a sala tem reservas futuras; desative-a em vez de excluir")
	// ErrOutOfScope: calendar:manage não cobre a unidade dona da sala (a
	// atual ou a nova) — ADR 013.
	ErrOutOfScope = errors.New("calendar: sala fora do seu escopo de gestão")
	// ErrPublicoNoSite: evento com público-alvo não pode ser público.
	ErrPublicoNoSite = errors.New("calendar: evento com público-alvo não pode ser público (site) — use interno")
)

// MaxRange limita consultas de período (evita varreduras gigantes).
const MaxRange = 93 * 24 * time.Hour

// Room é uma sala reservável.
type Room struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Location  string     `json:"location"`
	Capacity  int        `json:"capacity"`
	Resources []string   `json:"resources"`
	Active    bool       `json:"active"`
	UnidadeID *uuid.UUID `json:"unidade_id,omitempty"` // dona (ADR 013); nil = institucional
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Event é um evento/reserva.
type Event struct {
	ID            uuid.UUID  `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Location      string     `json:"location"`
	RoomID        *uuid.UUID `json:"room_id,omitempty"`
	RoomName      string     `json:"room_name,omitempty"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        time.Time  `json:"ends_at"`
	AllDay        bool       `json:"all_day"`
	Visibility    string     `json:"visibility"`
	Status        string     `json:"status"`
	OrganizerID   uuid.UUID  `json:"organizer_id"`
	OrganizerName string     `json:"organizer_name"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	RoomUnidadeID *uuid.UUID `json:"-"` // dona da sala: quem modera o evento (ADR 013)
	// Publico é o público-alvo (ADR 014); vazio = todos. Com público, o
	// evento não pode ser "public" (não vai ao site).
	Publico auth.Publico `json:"publico"`
}

// Dona é a posição de um recurso da Agenda para calendar:manage: a unidade
// dona da sala; sem ela, institucional (só a gestão global).
func Dona(unidade *uuid.UUID) auth.Target {
	if unidade == nil {
		return auth.Target{}
	}
	return auth.InUnidade(*unidade)
}

// Posicao é a posição do evento: a da sala reservada; sem sala, institucional.
func (e Event) Posicao() auth.Target { return Dona(e.RoomUnidadeID) }

// Busy é um intervalo ocupado de uma sala.
type Busy struct {
	EventID  uuid.UUID `json:"event_id"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

// EventFilter restringe a listagem.
type EventFilter struct {
	From, To    time.Time
	RoomID      *uuid.UUID
	ViewerID    uuid.UUID
	SeeAll      bool        // calendar:manage global enxerga eventos privados de terceiros
	SeeUnidades []uuid.UUID // com escopo: os privados das salas destas unidades
	// Leitor: onde quem lê pertence (público-alvo, ADR 014).
	Leitor        publico.Leitor
	OnlyPublic    bool
	IncludeCancel bool
}

// ValidateRange confere o intervalo de um evento.
func ValidateRange(start, end time.Time) error {
	if !end.After(start) || end.Sub(start) > 31*24*time.Hour {
		return ErrInvalidRange
	}
	return nil
}

// Repository é a porta de persistência.
type Repository interface {
	// ListRooms lista as salas; com onlyActive, as inativas só aparecem se
	// forem de uma das unidades dadas (gestão com escopo).
	ListRooms(ctx context.Context, db database.DBTX, onlyActive bool, unidades []uuid.UUID) ([]Room, error)
	GetRoom(ctx context.Context, db database.DBTX, id uuid.UUID) (Room, error)
	SaveRoom(ctx context.Context, db database.DBTX, r Room) (Room, error)
	DeleteRoom(ctx context.Context, db database.DBTX, id uuid.UUID) error
	// RoomHasUpcoming reporta se a sala tem reserva confirmada que ainda
	// não terminou.
	RoomHasUpcoming(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error)
	RoomBusy(ctx context.Context, db database.DBTX, roomID uuid.UUID, from, to time.Time) ([]Busy, error)

	ListEvents(ctx context.Context, db database.DBTX, f EventFilter) ([]Event, error)
	GetEvent(ctx context.Context, db database.DBTX, id uuid.UUID) (Event, error)
	SaveEvent(ctx context.Context, db database.DBTX, e Event) (Event, error)
	DeleteEvent(ctx context.Context, db database.DBTX, id uuid.UUID) error
}
