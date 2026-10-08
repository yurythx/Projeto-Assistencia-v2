// Package application contém os casos de uso da Agenda.
package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/calendar/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/publico"
)

// EventCreated é emitido ao criar um evento.
const EventCreated = "calendar.event.created"

// Service implementa os casos de uso.
type Service struct {
	pool   *pgxpool.Pool
	repo   domain.Repository
	outbox *outbox.Writer
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, ob *outbox.Writer) *Service {
	return &Service{pool: pool, repo: repo, outbox: ob}
}

// MapError traduz erros de domínio.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("registro da agenda não encontrado")
	case errors.Is(err, domain.ErrRoomConflict):
		return apperrors.Conflict("a sala já está reservada neste horário")
	case errors.Is(err, domain.ErrInvalidRange):
		return apperrors.Validation("intervalo inválido: o fim precisa ser depois do início (máx. 31 dias)")
	case errors.Is(err, domain.ErrDuplicate):
		return apperrors.Conflict("já existe uma sala com este nome")
	case errors.Is(err, domain.ErrInvalidValue):
		return apperrors.Validation("valor fora do permitido (visibilidade, status ou capacidade)")
	case errors.Is(err, domain.ErrCancelled), errors.Is(err, domain.ErrRoomInUse):
		return apperrors.Conflict(err.Error())
	case errors.Is(err, domain.ErrOutOfScope):
		return apperrors.Forbidden(err.Error())
	case errors.Is(err, domain.ErrPublicoNoSite):
		return apperrors.Validation(err.Error())
	case errors.Is(err, publico.ErrInvalido):
		return apperrors.Validation("público-alvo com secretaria ou unidade inexistente")
	}
	return err
}

func checkWindow(from, to time.Time) error {
	if !to.After(from) || to.Sub(from) > domain.MaxRange {
		return apperrors.Validation("período de consulta inválido (máximo de 93 dias)")
	}
	return nil
}

// Events lista eventos do período visíveis a identity.
func (s *Service) Events(ctx context.Context, identity auth.Identity, from, to time.Time, roomID *uuid.UUID) ([]domain.Event, error) {
	if err := checkWindow(from, to); err != nil {
		return nil, err
	}
	gestao := auth.CoverageOf(identity, auth.PermCalendarManage)
	return s.repo.ListEvents(ctx, s.pool, domain.EventFilter{
		From: from, To: to, RoomID: roomID, ViewerID: identity.UserID,
		SeeAll: gestao.All, SeeUnidades: gestao.Unidades, Leitor: publico.LeitorDe(identity),
	})
}

// PublicEvents lista só eventos públicos confirmados.
func (s *Service) PublicEvents(ctx context.Context, from, to time.Time) ([]domain.Event, error) {
	if err := checkWindow(from, to); err != nil {
		return nil, err
	}
	events, err := s.repo.ListEvents(ctx, s.pool, domain.EventFilter{From: from, To: to, OnlyPublic: true})
	for i := range events {
		events[i].OrganizerName, events[i].OrganizerID = "", uuid.Nil
	}
	return events, err
}

// Event devolve um evento visível a identity.
func (s *Service) Event(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.Event, error) {
	e, err := s.repo.GetEvent(ctx, s.pool, id)
	if err != nil {
		return e, MapError(err)
	}
	gere := e.OrganizerID == identity.UserID || auth.Can(identity, auth.PermCalendarManage, e.Posicao())
	if !gere && (e.Visibility == "private" || !auth.NoPublico(identity, e.Publico)) {
		return domain.Event{}, MapError(domain.ErrNotFound)
	}
	return e, nil
}

// EventInput são os campos editáveis.
type EventInput struct {
	Title, Description, Location, Visibility string
	RoomID                                   *uuid.UUID
	StartsAt, EndsAt                         time.Time
	AllDay                                   bool
	// Publico é o público-alvo (ADR 014); vazio = todos.
	Publico auth.Publico
}

// validate confere o intervalo e, quando a reserva de sala é nova (ou
// troca de sala), que a sala está ativa — editar só o título de um evento
// numa sala desativada depois continua possível.
func (s *Service) validate(ctx context.Context, db database.DBTX, in EventInput, currentRoom *uuid.UUID) error {
	if err := domain.ValidateRange(in.StartsAt, in.EndsAt); err != nil {
		return err
	}
	if in.Visibility == "public" && !in.Publico.Vazio() {
		return domain.ErrPublicoNoSite
	}
	if in.RoomID != nil && (currentRoom == nil || *currentRoom != *in.RoomID) {
		room, err := s.repo.GetRoom(ctx, db, *in.RoomID)
		if err != nil {
			return err
		}
		if !room.Active {
			return apperrors.Conflict("sala inativa não aceita reservas")
		}
	}
	return nil
}

// CreateEvent cria um evento (self-service) e emite calendar.event.created.
func (s *Service) CreateEvent(ctx context.Context, identity auth.Identity, in EventInput) (domain.Event, error) {
	var out domain.Event
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.validate(ctx, tx, in, nil); err != nil {
			return err
		}
		var err error
		out, err = s.repo.SaveEvent(ctx, tx, domain.Event{
			ID: uuid.New(), Title: strings.TrimSpace(in.Title), Description: in.Description, Location: in.Location,
			RoomID: in.RoomID, StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt.UTC(), AllDay: in.AllDay,
			Visibility: in.Visibility, Status: "confirmed", OrganizerID: identity.UserID, Publico: in.Publico,
		})
		if err != nil {
			return err
		}
		if err := s.outbox.Write(ctx, tx, EventCreated, "calendar_event", out.ID.String(), uuid.Nil, map[string]any{
			"id": out.ID.String(), "title": out.Title, "starts_at": out.StartsAt, "ends_at": out.EndsAt,
			"room_id": out.RoomID, "visibility": out.Visibility,
			// Com público-alvo, a difusão geral (tempo real) descarta o aviso (ADR 014).
			"restrito": !out.Publico.Vazio(),
		}); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "calendar.event.created", "calendar_event", out.ID.String(), nil, out))
	})
	return out, MapError(err)
}

func (s *Service) ownOrManage(identity auth.Identity, e domain.Event) error {
	if e.OrganizerID != identity.UserID && !auth.Can(identity, auth.PermCalendarManage, e.Posicao()) {
		return apperrors.Forbidden("só o organizador ou quem tem calendar:manage na unidade da sala altera este evento")
	}
	return nil
}

// UpdateEvent altera um evento (organizador ou calendar:manage).
func (s *Service) UpdateEvent(ctx context.Context, identity auth.Identity, id uuid.UUID, in EventInput) (domain.Event, error) {
	var out domain.Event
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.GetEvent(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.ownOrManage(identity, prev); err != nil {
			return err
		}
		if prev.Status == "cancelled" {
			return domain.ErrCancelled
		}
		if err := s.validate(ctx, tx, in, prev.RoomID); err != nil {
			return err
		}
		next := prev
		next.Title, next.Description, next.Location, next.Visibility = strings.TrimSpace(in.Title), in.Description, in.Location, in.Visibility
		next.RoomID, next.StartsAt, next.EndsAt, next.AllDay = in.RoomID, in.StartsAt.UTC(), in.EndsAt.UTC(), in.AllDay
		next.Publico = in.Publico
		if out, err = s.repo.SaveEvent(ctx, tx, next); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "calendar.event.updated", "calendar_event", id.String(), prev, out))
	})
	return out, MapError(err)
}

// CancelEvent cancela (libera a sala) mantendo o registro.
func (s *Service) CancelEvent(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.Event, error) {
	var out domain.Event
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.GetEvent(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.ownOrManage(identity, prev); err != nil {
			return err
		}
		if prev.Status == "cancelled" {
			return domain.ErrCancelled
		}
		next := prev
		next.Status = "cancelled"
		if out, err = s.repo.SaveEvent(ctx, tx, next); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "calendar.event.cancelled", "calendar_event", id.String(),
			map[string]string{"status": prev.Status}, map[string]string{"status": "cancelled"}))
	})
	return out, MapError(err)
}

// DeleteEvent remove um evento.
func (s *Service) DeleteEvent(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return MapError(database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.GetEvent(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.ownOrManage(identity, prev); err != nil {
			return err
		}
		if err := s.repo.DeleteEvent(ctx, tx, id); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "calendar.event.deleted", "calendar_event", id.String(), prev, nil))
	}))
}

// Rooms lista salas (inativas só para a gestão que as cobre).
func (s *Service) Rooms(ctx context.Context, identity auth.Identity) ([]domain.Room, error) {
	gestao := auth.CoverageOf(identity, auth.PermCalendarManage)
	return s.repo.ListRooms(ctx, s.pool, !gestao.All, gestao.Unidades)
}

// RoomBusy devolve a ocupação de uma sala no período.
func (s *Service) RoomBusy(ctx context.Context, roomID uuid.UUID, from, to time.Time) ([]domain.Busy, error) {
	if err := checkWindow(from, to); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetRoom(ctx, s.pool, roomID); err != nil {
		return nil, MapError(err)
	}
	return s.repo.RoomBusy(ctx, s.pool, roomID, from, to)
}

// SaveRoom cria/atualiza uma sala: calendar:manage cobrindo a unidade dona
// (a atual e a nova; sem dona, institucional — ADR 013).
func (s *Service) SaveRoom(ctx context.Context, identity auth.Identity, id uuid.UUID, in domain.Room) (domain.Room, error) {
	if in.Resources == nil {
		in.Resources = []string{}
	}
	var out domain.Room
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if !auth.Can(identity, auth.PermCalendarManage, domain.Dona(in.UnidadeID)) {
			return domain.ErrOutOfScope
		}
		var before any
		if id == uuid.Nil {
			in.ID = uuid.New()
		} else {
			prev, err := s.repo.GetRoom(ctx, tx, id)
			if err != nil {
				return err
			}
			if !auth.Can(identity, auth.PermCalendarManage, domain.Dona(prev.UnidadeID)) {
				return domain.ErrOutOfScope
			}
			in.ID, before = id, prev
		}
		var err error
		if out, err = s.repo.SaveRoom(ctx, tx, in); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "calendar.room.saved", "calendar_room", out.ID.String(), before, out))
	})
	return out, MapError(err)
}

// DeleteRoom remove uma sala sem reservas futuras (os eventos passados
// ficam registrados, sem sala). Com reservas futuras: desativar.
func (s *Service) DeleteRoom(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return MapError(database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.GetRoom(ctx, tx, id)
		if err != nil {
			return err
		}
		if !auth.Can(identity, auth.PermCalendarManage, domain.Dona(prev.UnidadeID)) {
			return domain.ErrOutOfScope
		}
		upcoming, err := s.repo.RoomHasUpcoming(ctx, tx, id)
		if err != nil {
			return err
		}
		if upcoming {
			return domain.ErrRoomInUse
		}
		if err := s.repo.DeleteRoom(ctx, tx, id); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "calendar.room.deleted", "calendar_room", id.String(), prev, nil))
	}))
}
