// Package transport expõe a API administrativa do Egress (egress:manage).
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/egress/application"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Egress.
type Handlers struct {
	svc         *application.Service
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger, maxPageSize int) *Handlers {
	return &Handlers{svc: svc, logger: logger, maxPageSize: maxPageSize}
}

// RegisterRoutes monta as rotas (todas exigem egress:manage).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Group(func(m chi.Router) {
		m.Use(auth.RequirePermission(h.logger, auth.PermEgressManage))
		m.Get("/egress/targets", h.Targets)
		m.Post("/egress/targets", h.Save)
		m.Put("/egress/targets/{id}", h.Save)
		m.Delete("/egress/targets/{id}", h.Delete)
		m.Post("/egress/targets/{id}/test", h.Test)
		m.Get("/egress/deliveries", h.Deliveries)
		m.Post("/egress/deliveries/{id}/redeliver", h.Redeliver)
	})
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func (h *Handlers) Targets(w http.ResponseWriter, r *http.Request) {
	ts, err := h.svc.Targets(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, ts)
}

type targetRequest struct {
	Name          string   `json:"name" validate:"required,max=120"`
	Kind          string   `json:"kind" validate:"required,oneof=webhook n8n zabbix grafana"`
	URL           string   `json:"url" validate:"required,url,max=1000"`
	Secret        *string  `json:"secret" validate:"omitempty,max=500"`
	EventPatterns []string `json:"event_patterns" validate:"max=50,dive,max=120"`
	Active        *bool    `json:"active"`
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
	var req targetRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	t, err := h.svc.SaveTarget(r.Context(), id, application.TargetInput{
		Name: req.Name, Kind: req.Kind, URL: req.URL, Secret: req.Secret, EventPatterns: req.EventPatterns,
		Active: req.Active == nil || *req.Active, Actor: identity.Username,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if id == uuid.Nil {
		httputil.WriteCreated(w, t)
		return
	}
	httputil.WriteOK(w, t)
}

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeleteTarget(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

func (h *Handlers) Test(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	res, err := h.svc.Test(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, res)
}

func (h *Handlers) Deliveries(w http.ResponseWriter, r *http.Request) {
	target, err := httputil.OptionalUUIDQuery(r, "target_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p := httputil.Page(r, h.maxPageSize)
	items, total, err := h.svc.Deliveries(r.Context(), target, httputil.Query(r, "status", 20), p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, items, p, total)
}

func (h *Handlers) Redeliver(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.Redeliver(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}
