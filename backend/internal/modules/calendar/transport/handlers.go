// Package transport expõe a API HTTP da Agenda.
package transport

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/calendar/application"
	"github.com/yurythx/projeto-nexus/internal/modules/calendar/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API da Agenda.
type Handlers struct {
	svc    *application.Service
	logger *slog.Logger
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, logger: logger}
}

// RegisterPublicRoutes — eventos públicos.
func (h *Handlers) RegisterPublicRoutes(r chi.Router) {
	r.Get("/calendar/public/events", h.PublicEvents)
}

// RegisterAuthedRoutes — agenda interna e salas.
func (h *Handlers) RegisterAuthedRoutes(r chi.Router) {
	r.Get("/calendar/events", h.Events)
	r.Get("/calendar/events/{id}", h.Event)
	r.Post("/calendar/events", h.CreateEvent)
	r.Put("/calendar/events/{id}", h.UpdateEvent)
	r.Post("/calendar/events/{id}/cancel", h.CancelEvent)
	r.Delete("/calendar/events/{id}", h.DeleteEvent)
	r.Get("/calendar/rooms", h.Rooms)
	r.Get("/calendar/rooms/{id}/busy", h.RoomBusy)
	r.Group(func(m chi.Router) {
		m.Use(auth.RequirePermission(h.logger, auth.PermCalendarManage))
		m.Post("/calendar/rooms", h.SaveRoom)
		m.Put("/calendar/rooms/{id}", h.SaveRoom)
		m.Delete("/calendar/rooms/{id}", h.DeleteRoom)
	})
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

// window lê ?from=&to= (RFC3339); padrão: mês corrente.
func window(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	var err error
	if v := r.URL.Query().Get("from"); v != "" {
		if from, err = time.Parse(time.RFC3339, v); err != nil {
			return from, to, apperrors.BadRequest("from inválido (RFC3339)")
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if to, err = time.Parse(time.RFC3339, v); err != nil {
			return from, to, apperrors.BadRequest("to inválido (RFC3339)")
		}
	}
	return from, to, nil
}

func (h *Handlers) PublicEvents(w http.ResponseWriter, r *http.Request) {
	from, to, err := window(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	events, err := h.svc.PublicEvents(r.Context(), from, to)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	httputil.WriteOK(w, events)
}

func (h *Handlers) Events(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	from, to, err := window(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	room, err := httputil.OptionalUUIDQuery(r, "room_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	events, err := h.svc.Events(r.Context(), identity, from, to, room)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, events)
}

func (h *Handlers) Event(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Event(r.Context(), identity, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

type eventRequest struct {
	Title       string     `json:"title" validate:"required,max=200"`
	Description string     `json:"description" validate:"max=5000"`
	Location    string     `json:"location" validate:"max=200"`
	RoomID      *uuid.UUID `json:"room_id"`
	StartsAt    time.Time  `json:"starts_at" validate:"required"`
	EndsAt      time.Time  `json:"ends_at" validate:"required"`
	AllDay      bool       `json:"all_day"`
	Visibility  string     `json:"visibility" validate:"omitempty,oneof=public internal private"`
	// Publico: público-alvo (ADR 014); vazio = todos.
	Publico auth.Publico `json:"publico" validate:"-"`
}

func (req eventRequest) input() application.EventInput {
	vis := req.Visibility
	if vis == "" {
		vis = "internal"
	}
	return application.EventInput{
		Title: req.Title, Description: req.Description, Location: req.Location, Visibility: vis,
		RoomID: req.RoomID, StartsAt: req.StartsAt, EndsAt: req.EndsAt, AllDay: req.AllDay, Publico: req.Publico,
	}
}

func (h *Handlers) CreateEvent(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	var req eventRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.CreateEvent(r.Context(), identity, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, e)
}

func (h *Handlers) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req eventRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.UpdateEvent(r.Context(), identity, id, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

func (h *Handlers) CancelEvent(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.CancelEvent(r.Context(), identity, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

func (h *Handlers) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeleteEvent(r.Context(), identity, id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

func (h *Handlers) Rooms(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	rooms, err := h.svc.Rooms(r.Context(), identity)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, rooms)
}

func (h *Handlers) RoomBusy(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	from, to, err := window(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	busy, err := h.svc.RoomBusy(r.Context(), id, from, to)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, busy)
}

type roomRequest struct {
	Name      string   `json:"name" validate:"required,max=120"`
	Location  string   `json:"location" validate:"max=200"`
	Capacity  int      `json:"capacity" validate:"min=0,max=10000"`
	Resources []string `json:"resources" validate:"max=30,dive,max=80"`
	Active    *bool    `json:"active"`
	// UnidadeID é a unidade dona (ADR 013); vazio = institucional.
	UnidadeID *uuid.UUID `json:"unidade_id"`
}

func (h *Handlers) SaveRoom(w http.ResponseWriter, r *http.Request) {
	id := uuid.Nil
	if chi.URLParam(r, "id") != "" {
		var err error
		if id, err = httputil.UUIDParam(r, "id"); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	var req roomRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	active := req.Active == nil || *req.Active
	identity, _ := auth.IdentityFromContext(r.Context())
	room, err := h.svc.SaveRoom(r.Context(), identity, id, domain.Room{
		Name: req.Name, Location: req.Location, Capacity: req.Capacity, Resources: req.Resources, Active: active, UnidadeID: req.UnidadeID,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if id == uuid.Nil {
		httputil.WriteCreated(w, room)
		return
	}
	httputil.WriteOK(w, room)
}

func (h *Handlers) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		identity, _ := auth.IdentityFromContext(r.Context())
		err = h.svc.DeleteRoom(r.Context(), identity, id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}
