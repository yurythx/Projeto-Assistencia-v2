// Package transport expõe a API HTTP do Contato.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/application"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Contato.
type Handlers struct {
	svc         *application.Service
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger, maxPageSize int) *Handlers {
	return &Handlers{svc: svc, logger: logger, maxPageSize: maxPageSize}
}

// RegisterPublicRoutes — POST /contact/messages com rate limit dedicado.
func (h *Handlers) RegisterPublicRoutes(r chi.Router, limiter httpserver.Limiter) {
	r.With(httpserver.RateLimit(h.logger, limiter, httpserver.ClientIPKey)).Post("/contact/messages", h.Submit)
}

// RegisterAdminRoutes — caixa de entrada (contact:read / contact:manage).
func (h *Handlers) RegisterAdminRoutes(r chi.Router) {
	r.Group(func(m chi.Router) {
		m.Use(auth.RequirePermission(h.logger, auth.PermContactRead))
		m.Get("/contact/messages", h.List)
		m.Get("/contact/messages/{id}", h.Get)
	})
	r.With(auth.RequirePermission(h.logger, auth.PermContactManage)).Patch("/contact/messages/{id}", h.Triage)
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

type submitRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=150"`
	Email    string `json:"email" validate:"required,email,max=200"`
	Phone    string `json:"phone" validate:"max=30"`
	Subject  string `json:"subject" validate:"required,min=3,max=200"`
	Category string `json:"category" validate:"omitempty,oneof=duvida sugestao reclamacao elogio outro"`
	Message  string `json:"message" validate:"required,min=10,max=5000"`
	// Consent: aceite explícito do tratamento dos dados (LGPD art. 7º, I).
	Consent bool `json:"consent"`
	// Website é um honeypot: invisível para humanos, preenchido por bots.
	Website string `json:"website"`
}

func (h *Handlers) Submit(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Website != "" {
		// Bot: resposta idêntica à de sucesso, nada é gravado.
		httputil.WriteJSON(w, http.StatusAccepted, map[string]string{"protocol": "CT-RECEBIDO"}, nil)
		return
	}
	if !req.Consent {
		h.fail(w, r, httputilValidation("é necessário consentir com o tratamento dos dados (LGPD)"))
		return
	}
	m, err := h.svc.Submit(r.Context(), application.SubmitInput{
		Name: req.Name, Email: req.Email, Phone: req.Phone, Subject: req.Subject, Category: req.Category,
		Message: req.Message, IP: httpserver.ClientIPKey(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteJSON(w, http.StatusAccepted, map[string]any{"protocol": m.Protocol, "created_at": m.CreatedAt}, nil)
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	p := httputil.Page(r, h.maxPageSize)
	f := domain.Filter{Status: httputil.Query(r, "status", 20)}
	if v := httputil.Query(r, "unidade_id", 36); v != "" {
		u, err := uuid.Parse(v)
		if err != nil {
			h.fail(w, r, apperrors.Validation("unidade_id inválido"))
			return
		}
		f.UnidadeID = &u
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	items, total, err := h.svc.List(r.Context(), identity, f, p)
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
	identity, _ := auth.IdentityFromContext(r.Context())
	m, err := h.svc.Get(r.Context(), identity, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, m)
}

type triageRequest struct {
	Status     string     `json:"status" validate:"required,oneof=new in_progress answered archived"`
	Notes      string     `json:"notes" validate:"max=5000"`
	AssignedTo *uuid.UUID `json:"assigned_to"`
	// UnidadeID é o setor para onde a mensagem vai; vazio = caixa geral.
	UnidadeID *uuid.UUID `json:"unidade_id"`
}

func (h *Handlers) Triage(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req triageRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	m, err := h.svc.Triage(r.Context(), identity, id, domain.Triage{Status: req.Status, Notes: req.Notes, AssignedTo: req.AssignedTo, UnidadeID: req.UnidadeID})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, m)
}
