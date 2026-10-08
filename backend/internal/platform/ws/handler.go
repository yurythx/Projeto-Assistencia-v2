package ws

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/metrics"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// TicketResponse é o corpo de POST /api/v1/ws/ticket.
type TicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresAt string `json:"expires_at"`
}

// TicketHandler emite um ticket para a identidade autenticada.
func TicketHandler(store *TicketStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.IdentityFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, r, logger, apperrors.Unauthorized("authentication required"))
			return
		}
		info := ClientInfo{
			Subject:     identity.Subject,
			Username:    identity.Username,
			Roles:       identity.Roles,
			Groups:      identity.Groups,
			Permissions: identity.Permissions,
			Scopes:      identity.Scopes,
		}
		if identity.UserID != uuid.Nil {
			info.UserID = identity.UserID.String()
		}
		ticket, err := store.Issue(r.Context(), info)
		if err != nil {
			httputil.WriteError(w, r, logger, apperrors.DependencyUnavailable("não foi possível emitir o ticket de conexão").WithCause(err))
			return
		}
		httputil.WriteCreated(w, TicketResponse{
			Ticket:    ticket.Value,
			ExpiresAt: ticket.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}

// UpgradeHandler faz o upgrade de GET /ws?ticket=... validando a origem
// contra allowedOrigins (lista separada por vírgula).
func UpgradeHandler(hub *Hub, store *TicketStore, allowedOrigins string, logger *slog.Logger) http.HandlerFunc {
	allowed := map[string]struct{}{}
	for _, o := range strings.Split(allowedOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = struct{}{}
		}
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // clientes não-navegador (sem risco de CSWSH)
			}
			_, ok := allowed[origin]
			return ok
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		info, err := store.Redeem(r.Context(), r.URL.Query().Get("ticket"))
		if err != nil {
			logger.Warn("ws: upgrade recusado", slog.Any("error", err))
			http.Error(w, "invalid or expired ticket", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Warn("ws: upgrade falhou", slog.Any("error", err))
			metrics.WebSocketErrorsTotal.Inc()
			return
		}
		// O contexto da requisição é cancelado quando o handler retorna;
		// o ciclo de vida da conexão é o do próprio readPump.
		newClient(hub, conn, info, logger).run(r.Context())
	}
}
