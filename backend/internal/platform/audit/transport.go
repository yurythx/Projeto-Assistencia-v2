package audit

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers expõe a consulta e a verificação de integridade da trilha.
type Handlers struct {
	reader      *Reader
	exporter    *Exporter
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers constrói os handlers HTTP do módulo de auditoria.
func NewHandlers(reader *Reader, exporter *Exporter, logger *slog.Logger, maxPageSize int) *Handlers {
	return &Handlers{reader: reader, exporter: exporter, logger: logger, maxPageSize: maxPageSize}
}

// RegisterRoutes monta as rotas de auditoria (relativas ao grupo /api/v1
// autenticado).
func RegisterRoutes(r chi.Router, h *Handlers) {
	r.Group(func(read chi.Router) {
		read.Use(auth.RequirePermission(h.logger, auth.PermAuditRead))
		read.Get("/audit/logs", h.List)
		read.Get("/audit/logs/{id}", h.Get)
	})
	r.With(auth.RequirePermission(h.logger, auth.PermAuditVerify)).Get("/audit/verify", h.Verify)
	h.exporter.RegisterRoutes(r)
}

// List — GET /audit/logs?action=&actor_id=&resource_type=&resource_id=&from=&to=&page=&page_size=
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := Filter{
		Action:       q.Get("action"),
		ResourceType: q.Get("resource_type"),
		ResourceID:   q.Get("resource_id"),
	}
	if v := q.Get("actor_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httputil.WriteError(w, r, h.logger, apperrors.BadRequest("actor_id inválido"))
			return
		}
		f.ActorID = &id
	}
	for key, dst := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if v := q.Get(key); v != "" {
			t, err := parseDateOrRFC3339(v)
			if err != nil {
				httputil.WriteError(w, r, h.logger, apperrors.BadRequest(key+" inválido (use AAAA-MM-DD ou RFC3339)"))
				return
			}
			*dst = &t
		}
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	f.Area = AreaOf(identity)
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("page_size"))
	p := pagination.New(page, size, h.maxPageSize)

	records, total, err := h.reader.List(r.Context(), f, p)
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	httputil.WriteOKWithMeta(w, records, pagination.NewMeta(p, total))
}

// Get — GET /audit/logs/{id}
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, r, h.logger, apperrors.BadRequest("id inválido"))
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	rec, err := h.reader.Get(r.Context(), id, AreaOf(identity))
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.WriteError(w, r, h.logger, apperrors.NotFound("registro de auditoria não encontrado"))
		return
	}
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	httputil.WriteOK(w, rec)
}

// Verify — GET /audit/verify?from=1&limit=0 — recalcula a cadeia SHA-256.
func (h *Handlers) Verify(w http.ResponseWriter, r *http.Request) {
	from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if from < 1 {
		from = 1
	}
	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	res, err := h.reader.Verify(r.Context(), from, limit)
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	// A verificação em si é um ato auditável (quem verificou, e o que viu).
	entry := FromRequest(r)
	entry.Action = "audit.chain.verified"
	entry.ResourceType = "audit_logs"
	entry.Metadata = map[string]any{"from": from, "limit": limit, "checked": res.Checked, "valid": res.Valid}
	if err := NewWriter(h.reader.db).Record(r.Context(), entry); err != nil {
		h.logger.Warn("audit: falha ao registrar a verificação da cadeia", slog.Any("error", err))
	}
	httputil.WriteOK(w, res)
}
