// Package transport expõe a API HTTP do Mercúrio (o envio é por HTTP; a
// recepção em tempo real é pelo WebSocket, tópico "mercurio:room:<id>").
package transport

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/application"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Mercúrio.
type Handlers struct {
	svc    *application.Service
	logger *slog.Logger
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, logger: logger}
}

// RegisterRoutes monta as rotas (autenticadas).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/mercurio/rooms", h.Rooms)
	r.Post("/mercurio/direct", h.Direct)
	r.Get("/mercurio/rooms/{id}/messages", h.Messages)
	r.Post("/mercurio/rooms/{id}/messages", h.Send)
	r.Post("/mercurio/rooms/{id}/read", h.Read)
	r.Patch("/mercurio/messages/{id}", h.Edit)
	r.Delete("/mercurio/messages/{id}", h.Delete)
	r.Group(func(m chi.Router) {
		m.Use(auth.RequirePermission(h.logger, auth.PermMercurioManage))
		m.Post("/mercurio/rooms", h.SaveRoom)
		m.Put("/mercurio/rooms/{id}", h.SaveRoom)
	})
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func identity(r *http.Request) auth.Identity {
	id, _ := auth.IdentityFromContext(r.Context())
	return id
}

func (h *Handlers) Rooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.svc.Rooms(r.Context(), identity(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, rooms)
}

type roomRequest struct {
	Kind           string     `json:"kind" validate:"required,oneof=global department"`
	Name           string     `json:"name" validate:"required,max=120"`
	Description    string     `json:"description" validate:"max=500"`
	ADGroup        string     `json:"ad_group" validate:"max=200"`
	DepartamentoID *uuid.UUID `json:"departamento_id"`
	Archived       bool       `json:"archived"`
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
	room, err := h.svc.SaveRoom(r.Context(), identity(r), id, application.RoomInput{
		Kind: req.Kind, Name: req.Name, Description: req.Description, ADGroup: req.ADGroup,
		DepartamentoID: req.DepartamentoID, Archived: req.Archived,
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

type directRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
}

func (h *Handlers) Direct(w http.ResponseWriter, r *http.Request) {
	var req directRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	room, err := h.svc.Direct(r.Context(), identity(r), req.UserID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, room)
}

func (h *Handlers) Messages(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var before *time.Time
	if v := r.URL.Query().Get("before"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			h.fail(w, r, apperrors.BadRequest("before inválido (RFC3339)"))
			return
		}
		before = &t
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	msgs, err := h.svc.Messages(r.Context(), identity(r), id, before, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, msgs)
}

type messageRequest struct {
	Body string `json:"body" validate:"required,max=4000"`
}

func (h *Handlers) Send(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req messageRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.Send(r.Context(), identity(r), id, req.Body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, m)
}

func (h *Handlers) Edit(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req messageRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.Edit(r.Context(), identity(r), id, req.Body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, m)
}

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.Delete(r.Context(), identity(r), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

func (h *Handlers) Read(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.MarkRead(r.Context(), identity(r), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}
