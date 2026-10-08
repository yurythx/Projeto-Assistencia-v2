// Package transport expõe a API HTTP do Blog.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/blog/application"
	"github.com/yurythx/projeto-nexus/internal/modules/blog/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Blog.
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

// RegisterRoutes monta as rotas (autenticadas: publicações INTERNAS).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/blog/posts", h.List)
	r.Get("/blog/posts/{ref}", h.Get)
	r.Group(func(m chi.Router) {
		m.Use(auth.RequirePermission(h.logger, auth.PermBlogManage))
		m.Post("/blog/posts", h.Create)
		m.Put("/blog/posts/{id}", h.Update)
		m.Post("/blog/posts/{id}/publish", h.transition(domain.StatusPublished))
		m.Post("/blog/posts/{id}/archive", h.transition(domain.StatusArchived))
		m.Post("/blog/posts/{id}/unpublish", h.transition(domain.StatusDraft))
		m.Delete("/blog/posts/{id}", h.Delete)
		m.Post("/blog/uploads", h.Upload)
	})
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	p := httputil.Page(r, h.maxPageSize)
	posts, total, err := h.svc.List(r.Context(), identity, domain.Filter{
		Status: httputil.Query(r, "status", 20),
		Kind:   httputil.Query(r, "kind", 20),
		Query:  httputil.Query(r, "q", 200),
	}, p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, posts, p, total)
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	identity, _ := auth.IdentityFromContext(r.Context())
	post, err := h.svc.Get(r.Context(), identity, chi.URLParam(r, "ref"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, post)
}

type postRequest struct {
	Title          string `json:"title" validate:"required,max=200"`
	Slug           string `json:"slug" validate:"max=100"`
	Summary        string `json:"summary" validate:"max=500"`
	Body           string `json:"body" validate:"max=200000"`
	Kind           string `json:"kind" validate:"omitempty,oneof=noticia comunicado"`
	Pinned         bool   `json:"pinned"`
	CoverObjectKey string `json:"cover_object_key" validate:"max=300"`
	// UnidadeID: unidade dona (ADR 013); vazio = institucional.
	UnidadeID *uuid.UUID `json:"unidade_id"`
	// Publico: público-alvo (ADR 014); vazio = todos.
	Publico auth.Publico `json:"publico" validate:"-"`
}

func (req postRequest) input() application.Input {
	return application.Input{
		Title: req.Title, Slug: req.Slug, Summary: req.Summary, Body: req.Body,
		Kind: req.Kind, Pinned: req.Pinned, CoverObjectKey: req.CoverObjectKey, UnidadeID: req.UnidadeID,
		Publico: req.Publico,
	}
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	var req postRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	post, err := h.svc.Create(r.Context(), authIdentity(r), req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, post)
}

func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req postRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	post, err := h.svc.Update(r.Context(), authIdentity(r), id, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, post)
}

func (h *Handlers) transition(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.UUIDParam(r, "id")
		if err != nil {
			h.fail(w, r, err)
			return
		}
		post, err := h.svc.Transition(r.Context(), authIdentity(r), id, to)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httputil.WriteOK(w, post)
	}
}

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.Delete(r.Context(), authIdentity(r), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

type uploadRequest struct {
	Filename    string `json:"filename" validate:"required,max=255"`
	ContentType string `json:"content_type" validate:"required,max=100"`
}

func (h *Handlers) Upload(w http.ResponseWriter, r *http.Request) {
	var req uploadRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	ticket, err := h.svc.CoverUpload(r.Context(), req.Filename, req.ContentType)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, ticket)
}
