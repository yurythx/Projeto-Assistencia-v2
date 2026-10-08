// Package transport expõe a API HTTP do Diretório.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/directory/application"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Diretório.
type Handlers struct {
	svc         *application.Service
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers cria os handlers.
// authIdentity devolve o chamador autenticado (RequireAuth garante que existe).
func authIdentity(r *http.Request) auth.Identity {
	id, _ := auth.IdentityFromContext(r.Context())
	return id
}

func NewHandlers(svc *application.Service, logger *slog.Logger, maxPageSize int) *Handlers {
	return &Handlers{svc: svc, logger: logger, maxPageSize: maxPageSize}
}

// RegisterPublicRoutes — setores (dados institucionais, nunca pessoais).
func (h *Handlers) RegisterPublicRoutes(r chi.Router) {
	r.Get("/directory/public/sectors", h.Sectors)
}

// RegisterAuthedRoutes — pessoas (consulta interna).
func (h *Handlers) RegisterAuthedRoutes(r chi.Router) {
	r.Get("/directory/people", h.List)
	r.Get("/directory/people/{id}", h.Get)
	r.Get("/directory/sectors", h.Sectors)
	r.Get("/directory/me", h.Me)
	r.Put("/directory/me", h.UpdateMe)
	r.With(auth.RequirePermission(h.logger, auth.PermDirectoryManage)).Put("/directory/people/{id}", h.UpdatePerson)
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	un, err := httputil.OptionalUUIDQuery(r, "unidade_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	dep, err := httputil.OptionalUUIDQuery(r, "departamento_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p := httputil.Page(r, h.maxPageSize)
	people, total, err := h.svc.List(r.Context(), identity, domain.Filter{Query: httputil.Query(r, "q", 100), UnidadeID: un, DepartamentoID: dep}, p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, people, p, total)
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.Get(r.Context(), identity, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	p, err := h.svc.Get(r.Context(), identity, identity.UserID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

type profileRequest struct {
	JobTitle       string     `json:"job_title" validate:"max=120"`
	Phone          string     `json:"phone" validate:"max=30"`
	Extension      string     `json:"extension" validate:"max=15"`
	Bio            string     `json:"bio" validate:"max=1000"`
	Visible        *bool      `json:"visible"`
	UnidadeID      *uuid.UUID `json:"unidade_id"`
	DepartamentoID *uuid.UUID `json:"departamento_id"`
}

func (req profileRequest) input() domain.ProfileInput {
	visible := true
	if req.Visible != nil {
		visible = *req.Visible
	}
	return domain.ProfileInput{
		JobTitle: req.JobTitle, Phone: req.Phone, Extension: req.Extension, Bio: req.Bio, Visible: visible,
		UnidadeID: req.UnidadeID, DepartamentoID: req.DepartamentoID,
	}
}

func (h *Handlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	var req profileRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.SaveProfile(r.Context(), identity, identity.UserID, req.input(), true)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

func (h *Handlers) UpdatePerson(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req profileRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.SaveProfile(r.Context(), authIdentity(r), id, req.input(), false)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

func (h *Handlers) Sectors(w http.ResponseWriter, r *http.Request) {
	sectors, err := h.svc.Sectors(r.Context(), httputil.Query(r, "q", 100))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, sectors)
}
