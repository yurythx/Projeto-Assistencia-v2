// Package transport expõe a API HTTP do módulo núcleo IAM & Usuários.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/iam/application"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// PermissionCatalog devolve as permissões declaradas por todos os plugins.
type PermissionCatalog func() []PermissionGroup

// PermissionGroup agrupa as permissões de um módulo.
type PermissionGroup struct {
	Module      string           `json:"module"`
	Name        string           `json:"name"`
	Permissions []PermissionItem `json:"permissions"`
}

// PermissionItem é uma permissão com descrição.
type PermissionItem struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// Handlers da API do IAM.
type Handlers struct {
	svc         *application.Service
	catalog     PermissionCatalog
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, catalog PermissionCatalog, logger *slog.Logger, maxPageSize int) *Handlers {
	return &Handlers{svc: svc, catalog: catalog, logger: logger, maxPageSize: maxPageSize}
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

// RegisterRoutes monta as rotas (grupo autenticado, relativo a /api/v1).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/me", h.Me)

	// Leitura da estrutura organizacional e do catálogo: qualquer
	// autenticado (formulários de outros módulos escolhem unidades).
	r.Get("/iam/org-tree", h.OrgTree)
	r.Get("/iam/entidades", h.ListEntidades)
	r.Get("/iam/unidades", h.ListUnidades)
	r.Get("/iam/departamentos", h.ListDepartamentos)
	r.Get("/iam/perfis", h.ListPerfis)
	r.Get("/iam/permissions", h.Permissions)

	r.Group(func(adm chi.Router) {
		adm.Use(auth.RequirePermission(h.logger, auth.PermIAMManage))
		adm.Post("/iam/entidades", h.SaveEntidade)
		adm.Put("/iam/entidades/{id}", h.SaveEntidade)
		adm.Delete("/iam/entidades/{id}", h.DeleteEntidade)
		adm.Post("/iam/unidades", h.SaveUnidade)
		adm.Put("/iam/unidades/{id}", h.SaveUnidade)
		adm.Delete("/iam/unidades/{id}", h.DeleteUnidade)
		adm.Post("/iam/departamentos", h.SaveDepartamento)
		adm.Put("/iam/departamentos/{id}", h.SaveDepartamento)
		adm.Delete("/iam/departamentos/{id}", h.DeleteDepartamento)
		adm.Post("/iam/perfis", h.SavePerfil)
		adm.Put("/iam/perfis/{id}", h.SavePerfil)
		adm.Delete("/iam/perfis/{id}", h.DeletePerfil)
		adm.Get("/iam/ad-mappings", h.ListMappings)
		adm.Post("/iam/ad-mappings", h.CreateMapping)
		adm.Delete("/iam/ad-mappings/{id}", h.DeleteMapping)
		adm.Get("/users/{id}/lotacoes", h.ListLotacoes)
		adm.Post("/users/{id}/lotacoes", h.CreateLotacao)
		adm.Delete("/users/{id}/lotacoes/{lotacaoID}", h.DeleteLotacao)
	})

	r.Group(func(read chi.Router) {
		read.Use(auth.RequirePermission(h.logger, auth.PermUsersRead))
		read.Get("/users", h.ListUsers)
		read.Get("/users/{id}", h.GetUser)
	})
	r.Group(func(adm chi.Router) {
		adm.Use(auth.RequirePermission(h.logger, auth.PermUsersManage))
		adm.Post("/users", h.CreateLocalUser)
		adm.Patch("/users/{id}", h.UpdateUser)
		adm.Post("/users/{id}/password", h.ResetPassword)
		adm.Post("/users/{id}/unlock", h.Unlock)
	})
}

type meResponse struct {
	ID          uuid.UUID    `json:"id"`
	Subject     string       `json:"subject"`
	Username    string       `json:"username"`
	Email       string       `json:"email"`
	Name        string       `json:"name"`
	Source      string       `json:"source"`
	Roles       []string     `json:"roles"`
	Groups      []string     `json:"groups"`
	Permissions []string     `json:"permissions"`
	Scopes      []auth.Scope `json:"scopes"`
}

// Me — GET /me: identidade efetiva (permissões e lotações resolvidas).
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	identity, err := auth.Require(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	nz := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	scopes := identity.Scopes
	if scopes == nil {
		scopes = []auth.Scope{}
	}
	httputil.WriteOK(w, meResponse{
		ID: identity.UserID, Subject: identity.Subject, Username: identity.Username, Email: identity.Email,
		Name: identity.Name, Source: string(identity.Source), Roles: nz(identity.Roles), Groups: nz(identity.Groups),
		Permissions: nz(identity.Permissions), Scopes: scopes,
	})
}

func (h *Handlers) Permissions(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteOK(w, h.catalog())
}

func (h *Handlers) OrgTree(w http.ResponseWriter, r *http.Request) {
	tree, err := h.svc.OrgTree(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, tree)
}
