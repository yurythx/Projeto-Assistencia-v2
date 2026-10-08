package notificacoes

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da caixa de notificações do próprio usuário (qualquer pessoa
// autenticada; cada um só vê e mexe na sua).
type Handlers struct {
	svc    *Service
	logger *slog.Logger
}

// NewHandlers cria os handlers.
func NewHandlers(svc *Service, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, logger: logger}
}

// RegisterRoutes monta as rotas (r já autenticado).
func RegisterRoutes(r chi.Router, h *Handlers) {
	r.Get("/notificacoes", h.Caixa)
	r.Post("/notificacoes/lidas", h.MarcarTodas)
	r.Post("/notificacoes/{id}/lida", h.MarcarLida)
	r.Get("/notificacoes/preferencias", h.Preferencias)
	r.Put("/notificacoes/preferencias", h.DefinirPreferencia)
}

// MapError traduz os erros do pacote.
func MapError(err error) error {
	var inv ErrInvalida
	switch {
	case errors.As(err, &inv):
		return apperrors.Validation(inv.Msg)
	case errors.Is(err, ErrNaoEncontrada):
		return apperrors.NotFound("notificação não encontrada")
	}
	return err
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, MapError(err))
}

// usuario exige o usuário local resolvido (o IAM preenche UserID).
func usuario(r *http.Request) (auth.Identity, error) {
	id, _ := auth.IdentityFromContext(r.Context())
	if id.UserID == uuid.Nil {
		return id, apperrors.Forbidden("usuário sem cadastro local")
	}
	return id, nil
}

func (h *Handlers) Caixa(w http.ResponseWriter, r *http.Request) {
	id, err := usuario(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	limite, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	c, err := h.svc.Caixa(r.Context(), id.UserID, limite)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, c)
}

func (h *Handlers) MarcarLida(w http.ResponseWriter, r *http.Request) {
	id, err := usuario(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	nid, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.MarcarLida(r.Context(), id.UserID, nid); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) MarcarTodas(w http.ResponseWriter, r *http.Request) {
	id, err := usuario(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.MarcarTodas(r.Context(), id.UserID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) Preferencias(w http.ResponseWriter, r *http.Request) {
	id, err := usuario(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.Preferencias(r.Context(), id.UserID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

type preferenciaRequest struct {
	Modulo string `json:"modulo" validate:"required,max=40"`
	Ativo  bool   `json:"ativo"`
}

func (h *Handlers) DefinirPreferencia(w http.ResponseWriter, r *http.Request) {
	id, err := usuario(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req preferenciaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DefinirPreferencia(r.Context(), id.UserID, Preferencia(req)); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, Preferencia(req))
}
