// Package transport expõe a API HTTP do Signum.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/signum/application"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Signum.
type Handlers struct {
	svc    *application.Service
	logger *slog.Logger
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, logger: logger}
}

// RegisterPublicRoutes — verificação pública de autenticidade.
func (h *Handlers) RegisterPublicRoutes(r chi.Router) {
	r.Get("/signum/verify/{id}", h.Verify)
}

// RegisterAuthedRoutes — cerimônia e gestão.
func (h *Handlers) RegisterAuthedRoutes(r chi.Router) {
	r.Get("/signum/envelopes", h.List)
	r.Post("/signum/envelopes", h.Open)
	r.Get("/signum/envelopes/{id}", h.Get)
	r.Post("/signum/envelopes/{id}/challenge", h.Challenge)
	r.Post("/signum/envelopes/{id}/sign", h.Sign)
	r.Post("/signum/envelopes/{id}/refuse", h.Refuse)
	r.Post("/signum/envelopes/{id}/cancel", h.Cancel)
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func identity(r *http.Request) auth.Identity {
	id, _ := auth.IdentityFromContext(r.Context())
	return id
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	envs, err := h.svc.List(r.Context(), identity(r), httputil.Query(r, "role", 20), httputil.Query(r, "status", 20))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, envs)
}

type openRequest struct {
	Title          string      `json:"title" validate:"required,max=200"`
	Description    string      `json:"description" validate:"max=2000"`
	DocumentSHA256 string      `json:"document_sha256" validate:"required,len=64"`
	SignerIDs      []uuid.UUID `json:"signer_ids" validate:"required,min=1,max=50"`
	Sequential     bool        `json:"sequential"`
}

func (h *Handlers) Open(w http.ResponseWriter, r *http.Request) {
	var req openRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Open(r.Context(), application.OpenRequest{
		Title: req.Title, Description: req.Description, DocumentSHA256: req.DocumentSHA256,
		SignerIDs: req.SignerIDs, Sequential: req.Sequential, SourceModule: "signum",
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, e)
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Get(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

func (h *Handlers) Challenge(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.Challenge(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteCreated(w, c)
}

type signRequest struct {
	ChallengeID   uuid.UUID `json:"challenge_id" validate:"required"`
	Nonce         string    `json:"nonce" validate:"required,len=64"`
	Password      string    `json:"password" validate:"required,max=1024"`
	ConfirmSHA256 string    `json:"confirm_document_sha256" validate:"required,len=64"`
}

func (h *Handlers) Sign(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req signRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Sign(r.Context(), identity(r), id, application.SignInput{
		ChallengeID: req.ChallengeID, Nonce: req.Nonce, Password: req.Password, ConfirmSHA256: req.ConfirmSHA256,
		IP: httpserver.ClientIPKey(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, e)
}

type refuseRequest struct {
	Reason string `json:"reason" validate:"required,min=3,max=1000"`
}

func (h *Handlers) Refuse(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req refuseRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Refuse(r.Context(), identity(r), id, req.Reason)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

func (h *Handlers) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Cancel(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

func (h *Handlers) Verify(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.Verify(r.Context(), id, httputil.Query(r, "document_sha256", 64))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, v)
}
