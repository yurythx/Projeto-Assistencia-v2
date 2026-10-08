// Package infrastructure implementa a persistência e a reautenticação do
// Signum.
package infrastructure

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/signum/domain"
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
	return fmt.Errorf("signum: %w", err)
}

func (r *Repository) Insert(ctx context.Context, db database.DBTX, e domain.Envelope) error {
	if _, err := db.Exec(ctx, `INSERT INTO signum_envelopes (id, title, description, document_sha256, source_module, source_ref, sequential, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',$8)`,
		e.ID, e.Title, e.Description, e.DocumentSHA256, e.SourceModule, e.SourceRef, e.Sequential, e.CreatedBy); err != nil {
		return wrap(err)
	}
	for _, s := range e.Signers {
		if _, err := db.Exec(ctx, `INSERT INTO signum_signers (id, envelope_id, user_id, position) VALUES ($1,$2,$3,$4)`,
			uuid.New(), e.ID, s.UserID, s.Position); err != nil {
			return wrap(err)
		}
	}
	return nil
}

func (r *Repository) UnavailableSigners(ctx context.Context, db database.DBTX, ids []uuid.UUID) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `SELECT t.sid FROM unnest($1::uuid[]) AS t(sid)
		WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = t.sid AND u.active)`, ids)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, wrap(err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

const envelopeCols = `e.id, e.title, e.description, e.document_sha256, e.source_module, e.source_ref, e.sequential, e.status,
	e.created_by, COALESCE(NULLIF(u.display_name,''), u.username, ''), e.created_at, e.completed_at`

func scanEnvelope(row interface{ Scan(...any) error }) (domain.Envelope, error) {
	var e domain.Envelope
	err := row.Scan(&e.ID, &e.Title, &e.Description, &e.DocumentSHA256, &e.SourceModule, &e.SourceRef, &e.Sequential, &e.Status,
		&e.CreatedBy, &e.CreatedByName, &e.CreatedAt, &e.CompletedAt)
	return e, err
}

func (r *Repository) signers(ctx context.Context, db database.DBTX, envelopeID uuid.UUID) ([]domain.Signer, error) {
	rows, err := db.Query(ctx, `SELECT s.id, s.envelope_id, s.user_id, COALESCE(NULLIF(u.display_name,''), u.username, ''),
		s.position, s.status, s.signed_at, COALESCE(s.signature_hash,''), s.method, COALESCE(host(s.ip_address),''), s.user_agent, s.reason
		FROM signum_signers s LEFT JOIN users u ON u.id = s.user_id WHERE s.envelope_id = $1 ORDER BY s.position, s.id`, envelopeID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Signer{}
	for rows.Next() {
		var s domain.Signer
		if err := rows.Scan(&s.ID, &s.EnvelopeID, &s.UserID, &s.Name, &s.Position, &s.Status, &s.SignedAt, &s.SignatureHash,
			&s.Method, &s.IPAddress, &s.UserAgent, &s.Reason); err != nil {
			return nil, wrap(err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Get carrega o envelope com os signatários (FOR UPDATE trava o envelope
// na transação da assinatura — duas assinaturas simultâneas não disputam
// o fechamento).
func (r *Repository) Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Envelope, error) {
	if forUpdate {
		if _, err := db.Exec(ctx, `SELECT 1 FROM signum_envelopes WHERE id = $1 FOR UPDATE`, id); err != nil {
			return domain.Envelope{}, wrap(err)
		}
	}
	e, err := scanEnvelope(db.QueryRow(ctx, `SELECT `+envelopeCols+` FROM signum_envelopes e LEFT JOIN users u ON u.id = e.created_by WHERE e.id = $1`, id))
	if err != nil {
		return e, wrap(err)
	}
	e.Signers, err = r.signers(ctx, db, id)
	return e, err
}

func (r *Repository) List(ctx context.Context, db database.DBTX, f domain.Filter, limit int) ([]domain.Envelope, error) {
	rows, err := db.Query(ctx, `SELECT `+envelopeCols+` FROM signum_envelopes e LEFT JOIN users u ON u.id = e.created_by
		WHERE ($2 = '' OR e.status = $2) AND CASE $3
			WHEN 'to_sign' THEN EXISTS (SELECT 1 FROM signum_signers s WHERE s.envelope_id = e.id AND s.user_id = $1 AND s.status = 'pending')
			WHEN 'created' THEN e.created_by = $1
			WHEN 'any' THEN true
			ELSE e.created_by = $1 OR EXISTS (SELECT 1 FROM signum_signers s WHERE s.envelope_id = e.id AND s.user_id = $1)
		END
		ORDER BY e.created_at DESC LIMIT $4`, f.UserID, f.Status, f.Role, limit)
	if err != nil {
		return nil, wrap(err)
	}
	var out []domain.Envelope
	for rows.Next() {
		e, err := scanEnvelope(rows)
		if err != nil {
			rows.Close()
			return nil, wrap(err)
		}
		out = append(out, e)
	}
	rows.Close()
	for i := range out {
		if out[i].Signers, err = r.signers(ctx, db, out[i].ID); err != nil {
			return nil, err
		}
	}
	if out == nil {
		out = []domain.Envelope{}
	}
	return out, nil
}

func (r *Repository) SetEnvelopeStatus(ctx context.Context, db database.DBTX, id uuid.UUID, status string) error {
	_, err := db.Exec(ctx, `UPDATE signum_envelopes SET status = $2,
		completed_at = CASE WHEN $2 IN ('completed','refused','cancelled') THEN now() ELSE completed_at END
		WHERE id = $1`, id, status)
	return wrap(err)
}

func (r *Repository) UpdateSigner(ctx context.Context, db database.DBTX, s domain.Signer) error {
	_, err := db.Exec(ctx, `UPDATE signum_signers SET status=$2, signed_at=$3, signature_hash=NULLIF($4,''), method=$5,
		ip_address=NULLIF($6,'')::inet, user_agent=$7, reason=$8 WHERE id=$1`,
		s.ID, s.Status, s.SignedAt, s.SignatureHash, s.Method, s.IPAddress, s.UserAgent, s.Reason)
	return wrap(err)
}

func (r *Repository) InsertChallenge(ctx context.Context, db database.DBTX, c domain.Challenge) error {
	_, err := db.Exec(ctx, `INSERT INTO signum_challenges (id, envelope_id, user_id, nonce_hash, expires_at) VALUES ($1,$2,$3,$4,$5)`,
		c.ID, c.EnvelopeID, c.UserID, c.NonceHash, c.ExpiresAt)
	return wrap(err)
}

// ConsumeChallenge marca o desafio como usado — atômico: só funciona uma
// vez, dentro do prazo, para o mesmo envelope/usuário e nonce.
func (r *Repository) ConsumeChallenge(ctx context.Context, db database.DBTX, id, envelopeID, userID uuid.UUID, nonceHash string) error {
	tag, err := db.Exec(ctx, `UPDATE signum_challenges SET used_at = now()
		WHERE id = $1 AND envelope_id = $2 AND user_id = $3 AND nonce_hash = $4 AND used_at IS NULL AND expires_at > now()`,
		id, envelopeID, userID, nonceHash)
	if err != nil {
		return wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrChallenge
	}
	return nil
}
