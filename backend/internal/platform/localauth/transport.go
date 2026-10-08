package localauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/passwords"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// ActionLoginFailed é registrado em audit_logs a cada tentativa de login
// local rejeitada (detecção de força bruta).
const ActionLoginFailed = "login.failed"

// Throttle é o controle distribuído de abuso do login (Redis): lockout
// progressivo por IP e por username, e penalidade adaptativa do rate
// limiter. Implementado por internal/platform/ratelimit; nil = desligado.
type Throttle interface {
	LockedFor(ctx context.Context, subject string) (time.Duration, error)
	RegisterFailure(ctx context.Context, subject string) (time.Duration, error)
	Reset(ctx context.Context, subject string) error
}

// Penalizer reduz o teto do rate limiter de login para um IP que erra.
type Penalizer interface {
	Penalize(ctx context.Context, key string) error
	Forgive(ctx context.Context, key string) error
}

// Handlers expõe o login local (fallback RS256 — contingência quando o
// Keycloak está indisponível ou em ambiente isolado).
// tokenIssuer emite o token do login (implementado por *auth.LocalSigner).
type tokenIssuer interface {
	IssueToken(account auth.LocalAccount) (string, time.Time, error)
}

type Handlers struct {
	store     Store
	signer    tokenIssuer // nil = login local desligado
	audit     *audit.Writer
	throttle  Throttle
	penalizer Penalizer
	logger    *slog.Logger
}

// NewHandlers monta os handlers. signer nil = login local desligado (404).
func NewHandlers(store Store, signer *auth.LocalSigner, auditWriter *audit.Writer, throttle Throttle, penalizer Penalizer, logger *slog.Logger) *Handlers {
	h := &Handlers{store: store, audit: auditWriter, throttle: throttle, penalizer: penalizer, logger: logger}
	if signer != nil { // um *LocalSigner nil dentro da interface não seria nil
		h.signer = signer
	}
	return h
}

type loginRequest struct {
	Username string `json:"username" validate:"required,max=150"`
	Password string `json:"password" validate:"required,max=1024"`
}

type loginResponse struct {
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresAt   string            `json:"expires_at"`
	User        loginResponseUser `json:"user"`
}

type loginResponseUser struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}

// Login trata POST /api/v1/auth/login. Sempre a mesma resposta genérica
// para usuário inexistente, conta inativa, bloqueada ou senha errada, e o
// mesmo custo de Argon2id em todos os caminhos (sem oráculo de tempo).
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if h.signer == nil {
		httputil.WriteError(w, r, h.logger, apperrors.NotFound("local login is not enabled"))
		return
	}

	var req loginRequest
	if err := httputil.DecodeJSON(w, r, &req); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	if err := httputil.Validate(req); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	ctx := r.Context()
	username := strings.TrimSpace(req.Username)
	ip := httpserver.ClientIPKey(r)
	userKey := "user:" + strings.ToLower(username)
	ipKey := "ip:" + ip

	// Lockout progressivo distribuído (IP e usuário) — checado antes de
	// tocar o banco.
	if h.throttle != nil {
		for _, subject := range []string{ipKey, userKey} {
			left, err := h.throttle.LockedFor(ctx, subject)
			if err != nil {
				h.logger.Warn("localauth: lockout indisponível — seguindo só com o bloqueio da conta", slog.Any("error", err))
				break
			}
			if left > 0 {
				passwords.DummyVerify(req.Password)
				h.recordFailure(r, username, "throttled")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(left.Seconds())+1))
				httputil.WriteError(w, r, h.logger, apperrors.RateLimited("muitas tentativas; tente novamente mais tarde"))
				return
			}
		}
	}

	account, err := h.store.GetByUsername(ctx, username)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, r, h.logger, err)
			return
		}
		passwords.DummyVerify(req.Password)
		h.fail(r, username, userKey, ipKey, "unknown username or no local password set")
		h.rejectInvalidCredentials(w, r)
		return
	}
	if !account.Active {
		passwords.DummyVerify(req.Password)
		h.fail(r, username, userKey, ipKey, "account is inactive")
		h.rejectInvalidCredentials(w, r)
		return
	}
	if account.Locked() {
		passwords.DummyVerify(req.Password)
		h.fail(r, username, userKey, ipKey, "account_locked")
		h.rejectInvalidCredentials(w, r)
		return
	}
	if err := passwords.Verify(account.PasswordHash, req.Password); err != nil {
		if regErr := h.store.RegisterFailedAttempt(ctx, account.ID); regErr != nil {
			h.logger.Warn("localauth: falha ao registrar tentativa", slog.Any("error", regErr))
		}
		h.fail(r, username, userKey, ipKey, "wrong password")
		h.rejectInvalidCredentials(w, r)
		return
	}

	// Rehash transparente (bcrypt legado ou parâmetros Argon2id antigos).
	if passwords.NeedsRehash(account.PasswordHash) {
		if newHash, err := passwords.Hash(req.Password); err == nil {
			if err := h.store.UpdatePasswordHash(ctx, account.ID, newHash); err != nil {
				h.logger.Warn("localauth: rehash para Argon2id falhou", slog.Any("error", err))
			}
		}
	}

	token, expiresAt, err := h.signer.IssueToken(auth.LocalAccount{
		ID:       account.ID.String(),
		Name:     account.DisplayName,
		Username: account.Username,
		Email:    account.Email,
		Roles:    account.Roles,
		Groups:   account.Groups,
	})
	if err != nil {
		httputil.WriteError(w, r, h.logger, apperrors.Internal(err))
		return
	}

	if err := h.store.ResetFailedAttempts(ctx, account.ID); err != nil {
		h.logger.Warn("localauth: falha ao zerar tentativas", slog.Any("error", err))
	}
	if err := h.store.TouchLastSeen(ctx, account.ID); err != nil {
		h.logger.Warn("localauth: falha ao atualizar last_seen_at", slog.Any("error", err))
	}
	if h.throttle != nil {
		_ = h.throttle.Reset(ctx, userKey)
	}
	if h.penalizer != nil {
		_ = h.penalizer.Forgive(ctx, ip)
	}

	if h.audit != nil {
		uid := account.ID
		entry := audit.FromRequest(r)
		entry.ActorID = &uid
		entry.ActorSubject = account.ID.String()
		entry.ActorRoles = account.Roles
		entry.Action = audit.ActionLogin
		entry.ResourceType = "user"
		entry.ResourceID = account.ID.String()
		entry.Metadata = map[string]any{"method": "local"}
		_ = h.audit.Record(ctx, entry)
	}

	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, loginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
		User: loginResponseUser{
			ID:          account.ID.String(),
			Username:    account.Username,
			Email:       account.Email,
			DisplayName: account.DisplayName,
			Roles:       account.Roles,
		},
	})
}

// fail registra a falha no lockout distribuído, na penalidade do rate
// limiter e na trilha de auditoria.
func (h *Handlers) fail(r *http.Request, username, userKey, ipKey, reason string) {
	ctx := r.Context()
	if h.throttle != nil {
		for _, subject := range []string{userKey, ipKey} {
			if d, err := h.throttle.RegisterFailure(ctx, subject); err == nil && d > 0 {
				h.logger.Warn("localauth: lockout progressivo aplicado", slog.String("subject_kind", strings.SplitN(subject, ":", 2)[0]), slog.Duration("duration", d))
			}
		}
	}
	if h.penalizer != nil {
		_ = h.penalizer.Penalize(ctx, httpserver.ClientIPKey(r))
	}
	h.recordFailure(r, username, reason)
}

func (h *Handlers) recordFailure(r *http.Request, username, reason string) {
	if h.audit == nil {
		return
	}
	entry := audit.FromRequest(r)
	entry.Action = ActionLoginFailed
	entry.ResourceType = "user"
	entry.ResourceID = username
	entry.Metadata = map[string]any{"method": "local", "reason": reason}
	_ = h.audit.Record(r.Context(), entry)
}

// Logout registra o encerramento da sessão (o token em si expira sozinho;
// o frontend descarta a sessão e, no fluxo Keycloak, encerra a sessão SSO).
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, r, h.logger, apperrors.Unauthorized("authentication required"))
		return
	}
	if h.audit != nil {
		entry := audit.FromRequest(r)
		entry.Action = audit.ActionLogout
		entry.ResourceType = "user"
		entry.ResourceID = identity.Subject
		entry.Metadata = map[string]any{"method": string(identity.Source)}
		_ = h.audit.Record(r.Context(), entry)
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, map[string]string{"status": "ok"})
}

func (h *Handlers) rejectInvalidCredentials(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteError(w, r, h.logger, apperrors.Unauthorized("invalid username or password"))
}

// RateLimitKey identifica o chamador do rate limiter de login (IP real).
func RateLimitKey(r *http.Request) string {
	return httpserver.ClientIPKey(r)
}

// RegisterRoutes monta POST /auth/login num router PÚBLICO já escopado em
// /api/v1.
func RegisterRoutes(r chi.Router, h *Handlers, logger *slog.Logger, limiter httpserver.Limiter) {
	r.With(httpserver.RateLimit(logger, limiter, RateLimitKey)).Post("/auth/login", h.Login)
}

// RegisterAuthedRoutes monta POST /auth/logout no grupo autenticado.
func RegisterAuthedRoutes(api chi.Router, h *Handlers) {
	api.Post("/auth/logout", h.Logout)
}
