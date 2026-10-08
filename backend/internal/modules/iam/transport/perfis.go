package transport

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// ------------------------------------------------------------------ perfis

type perfilRequest struct {
	Nome       string   `json:"nome" validate:"required,max=120"`
	Slug       string   `json:"slug" validate:"omitempty,max=80"`
	Descricao  string   `json:"descricao" validate:"max=500"`
	Permissoes []string `json:"permissoes" validate:"max=200,dive,max=80"`
	Ativo      *bool    `json:"ativo"`
}

func (h *Handlers) ListPerfis(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListPerfis(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) SavePerfil(w http.ResponseWriter, r *http.Request) {
	id, err := optionalID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req perfilRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.SavePerfil(r.Context(), domain.Perfil{
		ID: id, Nome: req.Nome, Slug: req.Slug, Descricao: req.Descricao, Permissoes: req.Permissoes, Ativo: boolOr(req.Ativo, true),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeSaved(w, id, out)
}

func (h *Handlers) DeletePerfil(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeletePerfil(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

// ----------------------------------------------------------- mapeamentos AD

type scopeRequest struct {
	EntidadeID     *uuid.UUID `json:"entidade_id"`
	UnidadeID      *uuid.UUID `json:"unidade_id"`
	DepartamentoID *uuid.UUID `json:"departamento_id"`
}

type mappingRequest struct {
	ADGroup  string    `json:"ad_group" validate:"required,max=200"`
	PerfilID uuid.UUID `json:"perfil_id" validate:"required"`
	scopeRequest
	Descricao string `json:"descricao" validate:"max=300"`
}

func (h *Handlers) ListMappings(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListMappings(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) CreateMapping(w http.ResponseWriter, r *http.Request) {
	var req mappingRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	id, err := h.svc.CreateMapping(r.Context(), domain.ADMapping{
		ADGroup: req.ADGroup, PerfilID: req.PerfilID, Descricao: req.Descricao,
		Scope: domain.Scope{EntidadeID: req.EntidadeID, UnidadeID: req.UnidadeID, DepartamentoID: req.DepartamentoID},
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, map[string]string{"id": id.String()})
}

func (h *Handlers) DeleteMapping(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeleteMapping(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}
