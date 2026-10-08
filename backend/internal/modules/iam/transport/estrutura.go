package transport

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// ---------------------------------------------------------------- entidades

type entidadeRequest struct {
	Nome      string `json:"nome" validate:"required,max=200"`
	Sigla     string `json:"sigla" validate:"max=30"`
	Slug      string `json:"slug" validate:"omitempty,max=80"`
	Documento string `json:"documento" validate:"max=40"`
	Ativo     *bool  `json:"ativo"`
}

// writeSaved responde 201 na criação (POST, sem id na rota) e 200 na
// atualização (PUT /{id}).
func writeSaved(w http.ResponseWriter, id uuid.UUID, v any) {
	if id == uuid.Nil {
		httputil.WriteCreated(w, v)
		return
	}
	httputil.WriteOK(w, v)
}

func optionalID(r *http.Request) (uuid.UUID, error) {
	if chi.URLParam(r, "id") == "" {
		return uuid.Nil, nil
	}
	return httputil.UUIDParam(r, "id")
}

func boolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func (h *Handlers) ListEntidades(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListEntidades(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) SaveEntidade(w http.ResponseWriter, r *http.Request) {
	id, err := optionalID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req entidadeRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.SaveEntidade(r.Context(), domain.Entidade{
		ID: id, Nome: req.Nome, Sigla: req.Sigla, Slug: req.Slug, Documento: req.Documento, Ativo: boolOr(req.Ativo, true),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeSaved(w, id, out)
}

func (h *Handlers) DeleteEntidade(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeleteEntidade(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

// ----------------------------------------------------------------- unidades

type unidadeRequest struct {
	EntidadeID uuid.UUID  `json:"entidade_id" validate:"required"`
	ParentID   *uuid.UUID `json:"parent_id"`
	Nome       string     `json:"nome" validate:"required,max=200"`
	Sigla      string     `json:"sigla" validate:"max=30"`
	Slug       string     `json:"slug" validate:"omitempty,max=80"`
	ADGroup    string     `json:"ad_group" validate:"max=200"`
	Email      string     `json:"email" validate:"omitempty,email,max=200"`
	Telefone   string     `json:"telefone" validate:"max=40"`
	Endereco   string     `json:"endereco" validate:"max=300"`
	Ativo      *bool      `json:"ativo"`
}

func (h *Handlers) ListUnidades(w http.ResponseWriter, r *http.Request) {
	ent, err := httputil.OptionalUUIDQuery(r, "entidade_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.ListUnidades(r.Context(), ent)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) SaveUnidade(w http.ResponseWriter, r *http.Request) {
	id, err := optionalID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req unidadeRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.SaveUnidade(r.Context(), domain.Unidade{
		ID: id, EntidadeID: req.EntidadeID, ParentID: req.ParentID, Nome: req.Nome, Sigla: req.Sigla, Slug: req.Slug,
		ADGroup: req.ADGroup, Email: req.Email, Telefone: req.Telefone, Endereco: req.Endereco, Ativo: boolOr(req.Ativo, true),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeSaved(w, id, out)
}

func (h *Handlers) DeleteUnidade(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeleteUnidade(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

// ------------------------------------------------------------ departamentos

type departamentoRequest struct {
	UnidadeID uuid.UUID `json:"unidade_id" validate:"required"`
	Nome      string    `json:"nome" validate:"required,max=200"`
	Sigla     string    `json:"sigla" validate:"max=30"`
	Slug      string    `json:"slug" validate:"omitempty,max=80"`
	ADGroup   string    `json:"ad_group" validate:"max=200"`
	Email     string    `json:"email" validate:"omitempty,email,max=200"`
	Telefone  string    `json:"telefone" validate:"max=40"`
	Ativo     *bool     `json:"ativo"`
}

func (h *Handlers) ListDepartamentos(w http.ResponseWriter, r *http.Request) {
	un, err := httputil.OptionalUUIDQuery(r, "unidade_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.ListDepartamentos(r.Context(), un)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) SaveDepartamento(w http.ResponseWriter, r *http.Request) {
	id, err := optionalID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req departamentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.SaveDepartamento(r.Context(), domain.Departamento{
		ID: id, UnidadeID: req.UnidadeID, Nome: req.Nome, Sigla: req.Sigla, Slug: req.Slug,
		ADGroup: req.ADGroup, Email: req.Email, Telefone: req.Telefone, Ativo: boolOr(req.Ativo, true),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeSaved(w, id, out)
}

func (h *Handlers) DeleteDepartamento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.DeleteDepartamento(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}
