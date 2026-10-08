package lgpd

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Consentimento de visitante NÃO autenticado (gap G-11).
//
// POST /api/v1/lgpd/accept-anon — rota PÚBLICA (fora do grupo autenticado
// de /api/v1), rate-limited por IP. Grava em anonymous_consents. O corpo
// traz um device_hash (identificador opaco gerado no cliente, sem PII) e
// a versão dos termos.

type anonAcceptRequest struct {
	DeviceHash  string `json:"device_hash" validate:"required,min=8,max=128"`
	TermVersion string `json:"term_version"`
}

func (s *Service) handleAcceptAnon(w http.ResponseWriter, r *http.Request) {
	var req anonAcceptRequest
	if err := httputil.DecodeJSON(w, r, &req); err != nil {
		httputil.WriteError(w, r, s.logger, err)
		return
	}
	if req.TermVersion == "" {
		req.TermVersion = CurrentTermVersion
	}
	if err := httputil.Validate(req); err != nil {
		httputil.WriteError(w, r, s.logger, err)
		return
	}

	ip := httpserver.ClientIP(r, s.trustedProxies)
	const q = `
		INSERT INTO anonymous_consents (device_hash, term_version, ip_address, user_agent, accepted_at)
		VALUES ($1, $2, NULLIF($3,'')::inet, NULLIF($4,''), now())
		ON CONFLICT (device_hash, term_version) DO UPDATE SET accepted_at = now()`
	if _, err := s.db.Exec(r.Context(), q, req.DeviceHash, req.TermVersion, ip, r.UserAgent()); err != nil {
		httputil.WriteError(w, r, s.logger, apperrors.Internal(err))
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, map[string]any{
		"status":       "ok",
		"term_version": req.TermVersion,
		"accepted_at":  time.Now().UTC(),
	})
}

// RegisterPublicRoutes monta a rota pública de consentimento anônimo. r
// deve ser um router SEM auth.RequireAuthentication; limiter aplica o
// rate limit por IP (mesma ideia do login local).
func RegisterPublicRoutes(r chi.Router, s *Service, limiter httpserver.Limiter) {
	r.With(httpserver.RateLimit(s.logger, limiter, httpserver.ClientIPKey)).
		Post("/lgpd/accept-anon", s.handleAcceptAnon)
}
