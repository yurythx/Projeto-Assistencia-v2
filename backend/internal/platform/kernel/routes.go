package kernel

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// CodeModuleDisabled identifica a resposta de um módulo inativo.
const CodeModuleDisabled apperrors.Code = "MODULE_DISABLED"

// Guard responde 404 em toda a superfície de um módulo desativado,
// avaliado a CADA requisição (a desativação vale na hora, sem restart).
func (k *Kernel) Guard(key string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !k.Enabled(key) {
				httputil.WriteError(w, r, logger,
					apperrors.NotFound("módulo indisponível").WithCode(CodeModuleDisabled))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MountRoutes monta as rotas de cada RouteProvider sob o guard do módulo.
func (k *Kernel) MountRoutes(public, authed chi.Router, logger *slog.Logger) {
	for _, p := range k.Plugins() {
		rp, ok := p.(RouteProvider)
		if !ok {
			continue
		}
		key := p.Manifest().Key
		guard := k.Guard(key, logger)
		public.Group(func(pub chi.Router) {
			pub.Use(guard)
			authed.Group(func(au chi.Router) {
				au.Use(guard)
				rp.RegisterRoutes(Routes{Public: pub, Authed: au})
			})
		})
	}
}

// Handlers expõe o estado dos módulos e a administração de ativação.
type Handlers struct {
	kernel *Kernel
	logger *slog.Logger
}

// NewHandlers cria os handlers administrativos do Kernel.
func NewHandlers(k *Kernel, logger *slog.Logger) *Handlers {
	return &Handlers{kernel: k, logger: logger}
}

// RegisterAnonymousRoutes — GET /system/public-modules (sem sessão): só
// chave/estado dos módulos com superfície pública, para o site público
// esconder seções de módulos desativados.
func (h *Handlers) RegisterAnonymousRoutes(r chi.Router) {
	r.Get("/system/public-modules", h.PublicModules)
}

// RegisterRoutes monta as rotas autenticadas.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/system/modules", h.List)
	r.With(auth.RequirePermission(h.logger, auth.PermModulesManage)).
		Patch("/admin/modules/{key}", h.Toggle)
}

type publicModule struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// PublicModules — GET /system/public-modules
func (h *Handlers) PublicModules(w http.ResponseWriter, r *http.Request) {
	out := []publicModule{}
	for _, s := range h.kernel.Status() {
		if s.Public {
			out = append(out, publicModule{Key: s.Key, Enabled: s.Enabled})
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=15")
	httputil.WriteOK(w, out)
}

// List — GET /system/modules (qualquer autenticado: o frontend monta o
// menu a partir daqui).
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	httputil.WriteOK(w, h.kernel.Status())
}

type toggleRequest struct {
	Enabled *bool `json:"enabled" validate:"required"`
}

// Toggle — PATCH /admin/modules/{key} {"enabled": bool}
func (h *Handlers) Toggle(w http.ResponseWriter, r *http.Request) {
	var req toggleRequest
	if err := httputil.DecodeJSON(w, r, &req); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	if err := httputil.Validate(req); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	key := chi.URLParam(r, "key")
	err := h.kernel.SetEnabled(r.Context(), key, *req.Enabled, identity.Username)
	switch {
	case errors.Is(err, ErrUnknownModule):
		httputil.WriteError(w, r, h.logger, apperrors.NotFound("módulo não encontrado"))
		return
	case errors.Is(err, ErrCoreModule):
		httputil.WriteError(w, r, h.logger, apperrors.Conflict("módulos do núcleo (IAM, Auditoria) não podem ser desativados"))
		return
	case errors.Is(err, ErrDependencyMissing), errors.Is(err, ErrDependentActive):
		httputil.WriteError(w, r, h.logger, apperrors.Conflict(err.Error()))
		return
	case err != nil:
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	for _, s := range h.kernel.Status() {
		if s.Key == key {
			httputil.WriteOK(w, s)
			return
		}
	}
}
