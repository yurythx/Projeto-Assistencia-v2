package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/logging"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Enricher completa uma Identity recém-verificada — o IAM resolve ali o
// id interno, as permissões e as lotações efetivas (ver
// internal/platform/iam). Um erro do Enricher recusa a requisição (fail
// closed): sem permissões resolvidas não há como autorizar nada com
// segurança.
type Enricher func(ctx context.Context, identity Identity) (Identity, error)

// RequireAuthentication extrai e verifica o bearer token do header
// Authorization, enriquece a Identity (quando enrich != nil) e a guarda no
// contexto. Precisa rodar antes de RequirePermission.
func RequireAuthentication(verifier *Verifier, enrich Enricher, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, err := bearerToken(r)
			if err != nil {
				httputil.WriteError(w, r, logger, err)
				return
			}

			identity, err := verifier.Verify(r.Context(), rawToken)
			if err != nil {
				httputil.WriteError(w, r, logger, apperrors.Unauthorized("invalid or expired access token"))
				return
			}

			if enrich != nil {
				identity, err = enrich(r.Context(), identity)
				if err != nil {
					httputil.WriteError(w, r, logger, err)
					return
				}
			}

			ctx := WithIdentity(r.Context(), identity)
			ctx = logging.WithUserID(ctx, identity.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthentication autentica quando há um bearer token válido e
// segue anônimo caso contrário — para rotas públicas que mostram mais
// conteúdo a quem está logado (ex.: catálogo com rascunhos para editores).
func OptionalAuthentication(verifier *Verifier, enrich Enricher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, err := bearerToken(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			identity, err := verifier.Verify(r.Context(), rawToken)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			if enrich != nil {
				if identity, err = enrich(r.Context(), identity); err != nil {
					next.ServeHTTP(w, r)
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), identity)))
		})
	}
}

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", apperrors.Unauthorized("missing Authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", apperrors.Unauthorized("Authorization header must use the Bearer scheme")
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", apperrors.Unauthorized("empty bearer token")
	}
	return token, nil
}

// RequirePermission autoriza a requisição somente se a identidade
// autenticada possuir permission ("recurso:ação"). Validado em middleware,
// antes de qualquer handler/caso de uso (A01).
func RequirePermission(logger *slog.Logger, permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := IdentityFromContext(r.Context())
			if !ok {
				httputil.WriteError(w, r, logger, apperrors.Unauthorized("authentication required"))
				return
			}
			if !HasPermission(identity, permission) {
				httputil.WriteError(w, r, logger, apperrors.Forbidden("insufficient permission: "+string(permission)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Require extrai a identidade do contexto dentro de um handler que já
// está atrás de RequireAuthentication; devolve um erro 401 pronto quando
// ela não existe.
func Require(ctx context.Context) (Identity, error) {
	identity, ok := IdentityFromContext(ctx)
	if !ok {
		return Identity{}, apperrors.Unauthorized("authentication required")
	}
	return identity, nil
}
