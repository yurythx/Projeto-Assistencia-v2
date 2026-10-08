// Package transport expõe o módulo-modelo via HTTP.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/example/application"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// CreateItemRequest — as tags validate:"..." são conferidas por
// httputil.Validate (blueprint: todo módulo valida a entrada por
// struct-tag, e o domínio revalida as invariantes).
type CreateItemRequest struct {
	Title       string `json:"title" validate:"required,max=200"`
	Description string `json:"description" validate:"max=2000"`
}

// Handlers agrupa os handlers HTTP.
type Handlers struct {
	service     *application.Service
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers cria os handlers.
func NewHandlers(service *application.Service, logger *slog.Logger) *Handlers {
	return &Handlers{service: service, logger: logger, maxPageSize: 100}
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

// Create cria um item (exige example:manage — ver routes.go).
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateItemRequest
	// Bind = DecodeJSON (MaxBytesReader de 1 MiB + DisallowUnknownFields)
	// + Validate (struct-tags).
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	item, err := h.service.CreateItem(r.Context(), req.Title, req.Description)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, item)
}

// Get devolve um item.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, r, apperrors.BadRequest("ID inválido"))
		return
	}
	item, err := h.service.GetItem(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, item)
}

// List pagina os itens (?page=&page_size=, meta de paginação padrão).
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	p := httputil.Page(r, h.maxPageSize)
	items, total, err := h.service.ListItems(r.Context(), p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, items, p, total)
}
