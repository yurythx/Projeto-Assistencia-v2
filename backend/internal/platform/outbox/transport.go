package outbox

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// StatsHandlers expõe a contagem de outbox_events por status ao painel
// de Monitoramento. Restrito à mesma permissão que já gate a exportação
// de auditoria naquela tela (auth.PermAuditRead) — dado operacional
// interno, não uma configuração alterável, então não justifica uma
// permissão nova só pra si.
type StatsHandlers struct {
	stats  *Stats
	logger *slog.Logger
}

func NewStatsHandlers(stats *Stats, logger *slog.Logger) *StatsHandlers {
	return &StatsHandlers{stats: stats, logger: logger}
}

// Get trata GET /api/v1/monitoring/outbox-stats.
func (h *StatsHandlers) Get(w http.ResponseWriter, r *http.Request) {
	counts, err := h.stats.Get(r.Context())
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	httputil.WriteOK(w, counts)
}

// RegisterStatsRoutes monta a rota de estatísticas do outbox. r já deve
// estar atrás de auth.RequireAuthentication; caminho relativo ao grupo em
// que é montado (mesmo cuidado de configflags/keycloakconfig — ver o
// achado de rota duplicada corrigido em lgpd/audit).
func RegisterStatsRoutes(r chi.Router, h *StatsHandlers, logger *slog.Logger) {
	r.With(auth.RequirePermission(logger, auth.PermMonitoringRead)).
		Get("/monitoring/outbox-stats", h.Get)
	r.With(auth.RequirePermission(logger, auth.PermMonitoringManage)).
		Post("/monitoring/outbox/requeue", h.Requeue)
}

// Requeue trata POST /api/v1/monitoring/outbox/requeue: devolve à fila os
// eventos que esgotaram as tentativas (ex.: depois de o RabbitMQ voltar).
func (h *StatsHandlers) Requeue(w http.ResponseWriter, r *http.Request) {
	n, err := h.stats.Requeue(r.Context())
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	httputil.WriteOK(w, map[string]int64{"requeued": n})
}
