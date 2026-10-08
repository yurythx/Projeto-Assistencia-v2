// Package infrastructure implementa a persistência e a entrega HTTP do
// Egress.
package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

func wrap(err error) error {
	if err == nil {
		return nil
	}
	if database.IsNoRows(err) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("egress: %w", err)
}

const targetCols = `id, name, kind, url, secret_encrypted <> '', event_patterns, active, created_by, created_at, updated_at`

func scanTarget(row interface{ Scan(...any) error }) (domain.Target, error) {
	var t domain.Target
	err := row.Scan(&t.ID, &t.Name, &t.Kind, &t.URL, &t.HasSecret, &t.EventPatterns, &t.Active, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (r *Repository) Targets(ctx context.Context, db database.DBTX, onlyActive bool) ([]domain.Target, error) {
	rows, err := db.Query(ctx, `SELECT `+targetCols+` FROM egress_targets WHERE (NOT $1 OR active) ORDER BY name`, onlyActive)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Target{}
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) Target(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Target, error) {
	t, err := scanTarget(db.QueryRow(ctx, `SELECT `+targetCols+` FROM egress_targets WHERE id = $1`, id))
	return t, wrap(err)
}

// SaveTarget grava o destino; secretEncrypted nil mantém o segredo atual.
func (r *Repository) SaveTarget(ctx context.Context, db database.DBTX, t domain.Target, secretEncrypted *string) (domain.Target, error) {
	_, err := db.Exec(ctx, `INSERT INTO egress_targets (id, name, kind, url, secret_encrypted, event_patterns, active, created_by)
		VALUES ($1,$2,$3,$4,COALESCE($5,''),$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, kind=EXCLUDED.kind, url=EXCLUDED.url,
		    secret_encrypted = COALESCE($5, egress_targets.secret_encrypted),
		    event_patterns=EXCLUDED.event_patterns, active=EXCLUDED.active`,
		t.ID, t.Name, t.Kind, t.URL, secretEncrypted, t.EventPatterns, t.Active, t.CreatedBy)
	if err != nil {
		return domain.Target{}, wrap(err)
	}
	return r.Target(ctx, db, t.ID)
}

func (r *Repository) DeleteTarget(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM egress_targets WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}

func (r *Repository) EncryptedSecret(ctx context.Context, db database.DBTX, id uuid.UUID) (string, error) {
	var s string
	err := db.QueryRow(ctx, `SELECT secret_encrypted FROM egress_targets WHERE id = $1`, id).Scan(&s)
	return s, wrap(err)
}

// EnqueueDelivery é idempotente por (target_id, event_id).
func (r *Repository) EnqueueDelivery(ctx context.Context, db database.DBTX, d domain.Delivery) error {
	_, err := db.Exec(ctx, `INSERT INTO egress_deliveries (target_id, event_id, event_type, payload)
		VALUES ($1,$2,$3,$4) ON CONFLICT (target_id, event_id) DO NOTHING`, d.TargetID, d.EventID, d.EventType, d.Payload)
	return wrap(err)
}

// ClaimDue reserva entregas vencidas de destinos ATIVOS (as de destino
// desativado esperam a reativação) — SKIP LOCKED: várias réplicas de
// worker nunca entregam a mesma linha — empurrando next_attempt_at para
// frente como "lease" enquanto a tentativa acontece.
func (r *Repository) ClaimDue(ctx context.Context, db database.DBTX, limit int) ([]domain.Delivery, error) {
	rows, err := db.Query(ctx, `
		WITH due AS (
			SELECT d.id FROM egress_deliveries d JOIN egress_targets t ON t.id = d.target_id AND t.active
			WHERE d.status IN ('pending','failed') AND d.next_attempt_at <= now()
			ORDER BY d.next_attempt_at LIMIT $1 FOR UPDATE OF d SKIP LOCKED
		)
		UPDATE egress_deliveries d SET next_attempt_at = now() + interval '2 minutes'
		FROM due WHERE d.id = due.id
		RETURNING d.id, d.target_id, d.event_id, d.event_type, d.payload, d.status, d.attempts, d.next_attempt_at, d.created_at`, limit)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	var out []domain.Delivery
	for rows.Next() {
		var d domain.Delivery
		if err := rows.Scan(&d.ID, &d.TargetID, &d.EventID, &d.EventType, &d.Payload, &d.Status, &d.Attempts, &d.NextAttemptAt, &d.CreatedAt); err != nil {
			return nil, wrap(err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) MarkDelivered(ctx context.Context, db database.DBTX, id uuid.UUID, statusCode int) error {
	_, err := db.Exec(ctx, `UPDATE egress_deliveries SET status='delivered', attempts = attempts + 1, last_status_code = $2,
		last_error = NULL, delivered_at = now() WHERE id = $1`, id, statusCode)
	return wrap(err)
}

func (r *Repository) MarkFailed(ctx context.Context, db database.DBTX, id uuid.UUID, attempts int, statusCode *int, errMsg string, next time.Time, dead bool) error {
	status := "failed"
	if dead {
		status = "dead"
	}
	_, err := db.Exec(ctx, `UPDATE egress_deliveries SET status=$2, attempts=$3, last_status_code=$4, last_error=$5, next_attempt_at=$6
		WHERE id = $1`, id, status, attempts, statusCode, errMsg, next)
	return wrap(err)
}

func (r *Repository) Deliveries(ctx context.Context, db database.DBTX, targetID *uuid.UUID, status string, p pagination.Params) ([]domain.Delivery, int64, error) {
	const where = ` WHERE ($1::uuid IS NULL OR d.target_id = $1) AND ($2 = '' OR d.status = $2)`
	var total int64
	if err := db.QueryRow(ctx, `SELECT count(*) FROM egress_deliveries d`+where, targetID, status).Scan(&total); err != nil {
		return nil, 0, wrap(err)
	}
	rows, err := db.Query(ctx, `SELECT d.id, d.target_id, t.name, d.event_id, d.event_type, d.status, d.attempts, d.next_attempt_at,
		d.last_status_code, COALESCE(d.last_error,''), d.created_at, d.delivered_at
		FROM egress_deliveries d JOIN egress_targets t ON t.id = d.target_id`+where+`
		ORDER BY d.created_at DESC LIMIT $3 OFFSET $4`, targetID, status, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, wrap(err)
	}
	defer rows.Close()
	out := []domain.Delivery{}
	for rows.Next() {
		var d domain.Delivery
		if err := rows.Scan(&d.ID, &d.TargetID, &d.TargetName, &d.EventID, &d.EventType, &d.Status, &d.Attempts, &d.NextAttemptAt,
			&d.LastStatusCode, &d.LastError, &d.CreatedAt, &d.DeliveredAt); err != nil {
			return nil, 0, wrap(err)
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

func (r *Repository) Redeliver(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `UPDATE egress_deliveries SET status='pending', next_attempt_at = now(), attempts = 0
		WHERE id = $1 AND status IN ('failed','dead')`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}
