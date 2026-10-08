// Package transport expõe a API HTTP do plugin Arquivos.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/files/application"
	"github.com/yurythx/projeto-nexus/internal/modules/files/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API de Arquivos.
type Handlers struct {
	svc    *application.Service
	logger *slog.Logger
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, logger: logger}
}

// RegisterRoutes monta as rotas (autenticadas; a ACL é avaliada no serviço).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/files/browse", h.Browse)
	r.Post("/files/folders", h.CreateFolder)
	r.Patch("/files/folders/{id}", h.UpdateFolder)
	r.Delete("/files/folders/{id}", h.DeleteFolder)
	r.Get("/files/folders/{id}/acl", h.GetACL)
	r.Put("/files/folders/{id}/acl", h.SetACL)
	r.Post("/files/uploads", h.StartUpload)
	r.Post("/files/objects/{id}/confirm", h.Confirm)
	r.Get("/files/objects/{id}/download", h.Download)
	r.Patch("/files/objects/{id}", h.UpdateFile)
	r.Delete("/files/objects/{id}", h.DeleteFile)
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func identity(r *http.Request) auth.Identity {
	id, _ := auth.IdentityFromContext(r.Context())
	return id
}

func (h *Handlers) Browse(w http.ResponseWriter, r *http.Request) {
	parent, err := httputil.OptionalUUIDQuery(r, "folder_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	listing, err := h.svc.Browse(r.Context(), identity(r), parent)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, listing)
}

type folderRequest struct {
	Name     string     `json:"name" validate:"required,max=200"`
	ParentID *uuid.UUID `json:"parent_id"`
	Move     bool       `json:"move"`
	// UnidadeID é a unidade dona (ADR 013); na edição vale com SetUnidade.
	UnidadeID  *uuid.UUID `json:"unidade_id"`
	SetUnidade bool       `json:"set_unidade"`
}

func (h *Handlers) CreateFolder(w http.ResponseWriter, r *http.Request) {
	var req folderRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := h.svc.CreateFolder(r.Context(), identity(r), req.ParentID, req.Name, req.UnidadeID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, f)
}

func (h *Handlers) UpdateFolder(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req folderRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := h.svc.UpdateFolder(r.Context(), identity(r), id, application.FolderChange{
		Name: req.Name, Parent: req.ParentID, Move: req.Move, Unidade: req.UnidadeID, SetUnidade: req.SetUnidade,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, f)
}

func (h *Handlers) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeleteFolder(r.Context(), identity(r), id, r.URL.Query().Get("recursive") == "true")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

func (h *Handlers) GetACL(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	acl, err := h.svc.ACL(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, acl)
}

type aclRequest struct {
	Entries []domain.ACLEntry `json:"entries" validate:"max=100,dive"`
}

func (h *Handlers) SetACL(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req aclRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	acl, err := h.svc.SetACL(r.Context(), identity(r), id, req.Entries)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, acl)
}

type uploadRequest struct {
	FolderID    uuid.UUID `json:"folder_id" validate:"required"`
	Filename    string    `json:"filename" validate:"required,max=255"`
	ContentType string    `json:"content_type" validate:"required,max=150"`
	Size        int64     `json:"size" validate:"required,min=1"`
}

func (h *Handlers) StartUpload(w http.ResponseWriter, r *http.Request) {
	var req uploadRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	res, err := h.svc.StartUpload(r.Context(), identity(r), req.FolderID, req.Filename, req.ContentType, req.Size)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, res)
}

func (h *Handlers) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := h.svc.ConfirmUpload(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, f)
}

func (h *Handlers) Download(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	url, f, err := h.svc.DownloadURL(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, map[string]any{"url": url, "file": f})
}

type fileRequest struct {
	Name     string    `json:"name" validate:"required,max=255"`
	FolderID uuid.UUID `json:"folder_id" validate:"required"`
}

func (h *Handlers) UpdateFile(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req fileRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := h.svc.UpdateFile(r.Context(), identity(r), id, req.Name, req.FolderID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, f)
}

func (h *Handlers) DeleteFile(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeleteFile(r.Context(), identity(r), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}
