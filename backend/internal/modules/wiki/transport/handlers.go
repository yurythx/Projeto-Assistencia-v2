// Package transport expõe a API HTTP da Wiki.
package transport

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki/application"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API da Wiki.
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
	r.Get("/wiki/tree", h.Tree)
	r.Get("/wiki/pages/{ref}", h.Get)
	r.Post("/wiki/pages", h.Create)
	r.Put("/wiki/pages/{id}", h.Update)
	r.Get("/wiki/pages/{id}/revisions", h.Revisions)
	r.Get("/wiki/pages/{id}/revisions/{version}", h.Revision)
	r.Post("/wiki/pages/{id}/revisions/{version}/restore", h.Restore)
	r.With(auth.RequirePermission(h.logger, auth.PermWikiManage)).Delete("/wiki/pages/{id}", h.Delete)
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func identity(r *http.Request) auth.Identity {
	id, _ := auth.IdentityFromContext(r.Context())
	return id
}

func (h *Handlers) Tree(w http.ResponseWriter, r *http.Request) {
	pages, err := h.svc.Tree(r.Context(), identity(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, pages)
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.Get(r.Context(), identity(r), chi.URLParam(r, "ref"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, page)
}

type pageRequest struct {
	ParentID *uuid.UUID `json:"parent_id"`
	Title    string     `json:"title" validate:"required,max=200"`
	Slug     string     `json:"slug" validate:"max=100"`
	Body     string     `json:"body" validate:"max=500000"`
	Position int        `json:"position" validate:"min=0,max=100000"`
	Summary  string     `json:"summary" validate:"max=300"`
	// UnidadeID é a unidade dona (ADR 013); vazio na criação herda a da mãe.
	UnidadeID *uuid.UUID `json:"unidade_id"`
	// Publico: público-alvo próprio (ADR 014); ausente na edição = mantém.
	Publico *auth.Publico `json:"publico" validate:"-"`
	// Version é a versão que o editor abriu (concorrência otimista).
	Version int `json:"version"`
}

func (req pageRequest) input() application.Input {
	return application.Input{ParentID: req.ParentID, Title: req.Title, Slug: req.Slug, Body: req.Body, Position: req.Position, Summary: req.Summary, UnidadeID: req.UnidadeID, Publico: req.Publico}
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	var req pageRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := h.svc.Create(r.Context(), identity(r), req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, page)
}

func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req pageRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Version < 1 {
		h.fail(w, r, apperrors.Validation("version (a versão aberta no editor) é obrigatória"))
		return
	}
	page, err := h.svc.Update(r.Context(), identity(r), id, req.Version, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, page)
}

func (h *Handlers) Revisions(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	revs, err := h.svc.Revisions(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, revs)
}

func versionParam(r *http.Request) (int, error) {
	v, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil || v < 1 {
		return 0, apperrors.BadRequest("versão inválida")
	}
	return v, nil
}

func (h *Handlers) Revision(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := versionParam(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	rev, err := h.svc.Revision(r.Context(), identity(r), id, v)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, rev)
}

func (h *Handlers) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := versionParam(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := h.svc.Restore(r.Context(), identity(r), id, v)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, page)
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
