package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Record é uma linha da trilha, como devolvida pela API de consulta.
type Record struct {
	ID            uuid.UUID       `json:"id"`
	ChainPos      int64           `json:"chain_pos"`
	ActorID       *uuid.UUID      `json:"actor_id,omitempty"`
	ActorSubject  string          `json:"actor_subject,omitempty"`
	ActorName     string          `json:"actor_name,omitempty"`
	ActorRoles    []string        `json:"actor_roles"`
	IPAddress     string          `json:"ip_address,omitempty"`
	UserAgent     string          `json:"user_agent,omitempty"`
	EntityContext json.RawMessage `json:"entity_context"`
	Action        string          `json:"action"`
	ResourceType  string          `json:"resource_type,omitempty"`
	ResourceID    string          `json:"resource_id,omitempty"`
	DiffBefore    json.RawMessage `json:"diff_before,omitempty"`
	DiffAfter     json.RawMessage `json:"diff_after,omitempty"`
	Metadata      json.RawMessage `json:"metadata"`
	CorrelationID *uuid.UUID      `json:"correlation_id,omitempty"`
	CreatedAt     time.Time       `json:"timestamp_utc"`
	PrevHash      string          `json:"prev_hash"`
	Hash          string          `json:"hash"`
}

// Filter restringe a consulta.
type Filter struct {
	Action       string
	ActorID      *uuid.UUID
	ResourceType string
	ResourceID   string
	From, To     *time.Time
	// Area: nil = a trilha toda (audit:read global).
	Area *Area
}

// Area restringe a trilha às ações de quem estava lotado na área quando
// agiu (ADR 013): a lotação do autor é a gravada no próprio registro
// (entity_context.scopes), protegida pela cadeia de hash — trocar de
// lotação depois não reescreve o passado. Ações sem lotação registrada
// (sistema, contas só globais) ficam só para a gestão global.
type Area struct {
	Entidades, Unidades, Departamentos []uuid.UUID
}

// AreaOf é a área em que identity lê a auditoria; nil = a trilha toda.
func AreaOf(identity auth.Identity) *Area {
	c := auth.CoverageOf(identity, auth.PermAuditRead)
	if c.All {
		return nil
	}
	return &Area{Entidades: c.Entidades, Unidades: c.Unidades, Departamentos: c.Departamentos}
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// cond devolve a condição SQL (sobre o alias "a") e acrescenta os
// argumentos a args; os placeholders começam em len(*args)+1.
func (area *Area) cond(args *[]any) string {
	n := len(*args)
	*args = append(*args, uuidStrings(area.Entidades), uuidStrings(area.Unidades), uuidStrings(area.Departamentos))
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(a.entity_context->'scopes') = 'array'
		THEN a.entity_context->'scopes' ELSE '[]'::jsonb END) s
		WHERE s->>'entidade_id' = ANY($%d::text[]) OR s->>'unidade_id' = ANY($%d::text[]) OR s->>'departamento_id' = ANY($%d::text[]))`,
		n+1, n+2, n+3)
}

// VerifyResult é o resultado de audit_verify_chain.
type VerifyResult struct {
	Checked         int64  `json:"checked"`
	Valid           bool   `json:"valid"`
	FirstInvalidPos *int64 `json:"first_invalid_pos,omitempty"`
	Reason          string `json:"reason,omitempty"`
	VerifiedAt      string `json:"verified_at"`
}

// Reader consulta audit_logs.
type Reader struct {
	db database.DBTX
}

// NewReader constrói um Reader (db: o pool, em produção).
func NewReader(db database.DBTX) *Reader {
	return &Reader{db: db}
}

// likeEscaper escapa os curingas do LIKE: no filtro de ação só "*" é
// curinga — "%" e "_" digitados valem como texto.
var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

const recordColumns = `
	a.id, a.chain_pos, a.actor_id, COALESCE(a.actor_subject,''), COALESCE(NULLIF(u.display_name,''), u.username, ''),
	a.actor_roles, COALESCE(host(a.ip_address),''), COALESCE(a.user_agent,''), a.entity_context,
	a.action, COALESCE(a.resource_type,''), COALESCE(a.resource_id,''), a.diff_before, a.diff_after,
	a.metadata, a.correlation_id, a.created_at, a.prev_hash, a.hash`

func scanRecord(row interface{ Scan(...any) error }) (Record, error) {
	var rec Record
	err := row.Scan(&rec.ID, &rec.ChainPos, &rec.ActorID, &rec.ActorSubject, &rec.ActorName,
		&rec.ActorRoles, &rec.IPAddress, &rec.UserAgent, &rec.EntityContext,
		&rec.Action, &rec.ResourceType, &rec.ResourceID, &rec.DiffBefore, &rec.DiffAfter,
		&rec.Metadata, &rec.CorrelationID, &rec.CreatedAt, &rec.PrevHash, &rec.Hash)
	return rec, err
}

// List devolve uma página da trilha, mais recente primeiro.
func (r *Reader) List(ctx context.Context, f Filter, p pagination.Params) ([]Record, int64, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Action != "" {
		add("a.action LIKE $%d", strings.ReplaceAll(likeEscaper.Replace(f.Action), "*", "%"))
	}
	if f.ActorID != nil {
		add("a.actor_id = $%d", *f.ActorID)
	}
	if f.ResourceType != "" {
		add("a.resource_type = $%d", f.ResourceType)
	}
	if f.ResourceID != "" {
		add("a.resource_id = $%d", f.ResourceID)
	}
	if f.From != nil {
		add("a.created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("a.created_at < $%d", *f.To)
	}
	if f.Area != nil {
		where = append(where, f.Area.cond(&args))
	}
	cond := ""
	if len(where) > 0 {
		cond = "WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM audit_logs a "+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit: count: %w", err)
	}

	args = append(args, p.Limit(), p.Offset())
	q := fmt.Sprintf(`SELECT %s FROM audit_logs a LEFT JOIN users u ON u.id = a.actor_id %s
		ORDER BY a.chain_pos DESC LIMIT $%d OFFSET $%d`, recordColumns, cond, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()
	out := make([]Record, 0, p.Limit())
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("audit: scan: %w", err)
		}
		out = append(out, rec)
	}
	return out, total, rows.Err()
}

// Get devolve um registro pelo id (fora da área: pgx.ErrNoRows).
func (r *Reader) Get(ctx context.Context, id uuid.UUID, area *Area) (Record, error) {
	args := []any{id}
	cond := ""
	if area != nil {
		cond = " AND " + area.cond(&args)
	}
	q := fmt.Sprintf(`SELECT %s FROM audit_logs a LEFT JOIN users u ON u.id = a.actor_id WHERE a.id = $1%s`, recordColumns, cond)
	return scanRecord(r.db.QueryRow(ctx, q, args...))
}

// Verify recalcula a cadeia SHA-256 a partir de from (1 = desde o início),
// examinando no máximo limit linhas (0 = todas).
func (r *Reader) Verify(ctx context.Context, from, limit int64) (VerifyResult, error) {
	var res VerifyResult
	var lim any
	if limit > 0 {
		lim = limit
	}
	var reason *string
	err := r.db.QueryRow(ctx, `SELECT checked, valid, first_invalid_pos, reason FROM audit_verify_chain($1, $2)`, from, lim).
		Scan(&res.Checked, &res.Valid, &res.FirstInvalidPos, &reason)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("audit: verify chain: %w", err)
	}
	if reason != nil {
		res.Reason = *reason
	}
	res.VerifiedAt = time.Now().UTC().Format(time.RFC3339)
	return res, nil
}
