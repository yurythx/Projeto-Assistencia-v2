// Package transport expõe a API HTTP do Catálogo.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/catalog/application"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Catálogo.
type Handlers struct {
	svc         *application.Service
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers cria os handlers.
// identity devolve o chamador autenticado (RequireAuth garante que existe).
func identity(r *http.Request) auth.Identity {
	id, _ := auth.IdentityFromContext(r.Context())
	return id
}

func NewHandlers(svc *application.Service, logger *slog.Logger, maxPageSize int) *Handlers {
	return &Handlers{svc: svc, logger: logger, maxPageSize: maxPageSize}
}

// RegisterPublicRoutes — leitura anônima (site público).
func (h *Handlers) RegisterPublicRoutes(r chi.Router) {
	r.Get("/catalog/services", h.ListPublic)
	r.Get("/catalog/services/{slug}", h.GetPublic)
	r.Get("/catalog/categories", h.Categories)
}

// RegisterAdminRoutes — gestão (catalog:manage).
func (h *Handlers) RegisterAdminRoutes(r chi.Router) {
	r.Group(func(m chi.Router) {
		m.Use(auth.RequirePermission(h.logger, auth.PermCatalogManage))
		m.Get("/catalog/admin/services", h.ListAll)
		m.Get("/catalog/admin/services/{id}", h.Get)
		m.Post("/catalog/admin/services", h.Save)
		m.Put("/catalog/admin/services/{id}", h.Save)
		m.Post("/catalog/admin/services/{id}/publish", h.status(domain.StatusPublished))
		m.Post("/catalog/admin/services/{id}/archive", h.status(domain.StatusArchived))
		m.Post("/catalog/admin/services/{id}/unpublish", h.status(domain.StatusDraft))
		m.Delete("/catalog/admin/services/{id}", h.Delete)
	})
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func (h *Handlers) ListPublic(w http.ResponseWriter, r *http.Request) {
	p := httputil.Page(r, h.maxPageSize)
	items, total, err := h.svc.ListPublic(r.Context(), httputil.Query(r, "category", 80), httputil.Query(r, "q", 200), p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	httputil.WritePage(w, items, p, total)
}

func (h *Handlers) GetPublic(w http.ResponseWriter, r *http.Request) {
	svc, err := h.svc.GetPublic(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	httputil.WriteOK(w, svc)
}

func (h *Handlers) Categories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.Categories(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, cats)
}

func (h *Handlers) ListAll(w http.ResponseWriter, r *http.Request) {
	p := httputil.Page(r, h.maxPageSize)
	items, total, err := h.svc.ListAll(r.Context(), identity(r), httputil.Query(r, "status", 20), httputil.Query(r, "category", 80), httputil.Query(r, "q", 200), p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, items, p, total)
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	svc, err := h.svc.Get(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, svc)
}

type serviceRequest struct {
	Title                string           `json:"title" validate:"required,max=200"`
	Slug                 string           `json:"slug" validate:"max=100"`
	Summary              string           `json:"summary" validate:"max=500"`
	Description          string           `json:"description" validate:"max=50000"`
	Category             string           `json:"category" validate:"max=80"`
	Audience             string           `json:"audience" validate:"max=200"`
	Requirements         []string         `json:"requirements" validate:"max=50,dive,max=300"`
	Steps                []string         `json:"steps" validate:"max=50,dive,max=500"`
	Channels             []domain.Channel `json:"channels" validate:"max=20,dive"`
	SLA                  string           `json:"sla" validate:"max=120"`
	Cost                 string           `json:"cost" validate:"max=120"`
	Icon                 string           `json:"icon" validate:"max=40"`
	ResponsibleUnidadeID *uuid.UUID       `json:"responsible_unidade_id"`
	Position             int              `json:"position" validate:"min=0,max=10000"`
}

func (h *Handlers) Save(w http.ResponseWriter, r *http.Request) {
	id := uuid.Nil
	if chi.URLParam(r, "id") != "" {
		var err error
		if id, err = httputil.UUIDParam(r, "id"); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	var req serviceRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.Save(r.Context(), identity(r), id, domain.Service{
		Title: req.Title, Slug: req.Slug, Summary: req.Summary, Description: req.Description, Category: req.Category,
		Audience: req.Audience, Requirements: req.Requirements, Steps: req.Steps, Channels: req.Channels, SLA: req.SLA,
		Cost: req.Cost, Icon: req.Icon, ResponsibleUnidadeID: req.ResponsibleUnidadeID, Position: req.Position,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if id == uuid.Nil {
		httputil.WriteCreated(w, out)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) status(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.UUIDParam(r, "id")
		if err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.svc.SetStatus(r.Context(), identity(r), id, to)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httputil.WriteOK(w, out)
	}
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
