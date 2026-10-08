package transport

import (
	"net/http"
	"strconv"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/application"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// ----------------------------------------------------------------- usuários

func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	p := httputil.Page(r, h.maxPageSize)
	f := domain.UserFilter{Query: httputil.Query(r, "q", 100)}
	if v := r.URL.Query().Get("active"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			h.fail(w, r, apperrors.BadRequest("o filtro active aceita true ou false"))
			return
		}
		f.Active = &b
	}
	users, total, err := h.svc.ListUsers(r.Context(), f, p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, users, p, total)
}

type userDetail struct {
	domain.User
	Lotacoes []domain.Lotacao `json:"lotacoes"`
}

func (h *Handlers) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	u, err := h.svc.GetUser(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	lot, err := h.svc.ListLotacoes(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, userDetail{User: u, Lotacoes: lot})
}

type updateUserRequest struct {
	DisplayName string   `json:"display_name" validate:"max=200"`
	Active      *bool    `json:"active" validate:"required"`
	Roles       []string `json:"roles" validate:"max=20,dive,max=80"`
}

func (h *Handlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req updateUserRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.UpdateUser(r.Context(), id, application.UpdateUserInput{
		DisplayName: req.DisplayName, Active: *req.Active, Roles: req.Roles,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

type createUserRequest struct {
	Username    string   `json:"username" validate:"required,min=3,max=80"`
	Email       string   `json:"email" validate:"required,email,max=200"`
	DisplayName string   `json:"display_name" validate:"max=200"`
	Password    string   `json:"password" validate:"required,max=256"`
	Roles       []string `json:"roles" validate:"max=20,dive,max=80"`
}

func (h *Handlers) CreateLocalUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.CreateLocalUser(r.Context(), application.CreateLocalUserInput{
		Username: req.Username, Email: req.Email, DisplayName: req.DisplayName, Password: req.Password, Roles: req.Roles,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, out)
}

type passwordRequest struct {
	Password string `json:"password" validate:"required,max=256"`
}

func (h *Handlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req passwordRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.ResetPassword(r.Context(), id, req.Password); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

func (h *Handlers) Unlock(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err == nil {
		err = h.svc.Unlock(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}
