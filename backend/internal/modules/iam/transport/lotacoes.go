package transport

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// ---------------------------------------------------------------- lotações

type lotacaoRequest struct {
	PerfilID uuid.UUID `json:"perfil_id" validate:"required"`
	scopeRequest
	Principal bool `json:"principal"`
}

func (h *Handlers) ListLotacoes(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.ListLotacoes(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) CreateLotacao(w http.ResponseWriter, r *http.Request) {
	userID, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req lotacaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	id, err := h.svc.CreateLotacao(r.Context(), domain.Lotacao{
		UserID: userID, PerfilID: req.PerfilID, Principal: req.Principal,
		Scope: domain.Scope{EntidadeID: req.EntidadeID, UnidadeID: req.UnidadeID, DepartamentoID: req.DepartamentoID},
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, map[string]string{"id": id.String()})
}

func (h *Handlers) DeleteLotacao(w http.ResponseWriter, r *http.Request) {
	userID, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	id, err := httputil.UUIDParam(r, "lotacaoID")
	if err == nil {
		err = h.svc.DeleteLotacao(r.Context(), userID, id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}
