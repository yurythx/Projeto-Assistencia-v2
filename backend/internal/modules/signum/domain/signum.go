// Package domain define o motor de assinatura eletrônica Signum.
package domain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

var (
	ErrNotFound         = errors.New("signum: envelope não encontrado")
	ErrNotSigner        = errors.New("signum: você não é signatário pendente deste envelope")
	ErrNotYourTurn      = errors.New("signum: aguarde a assinatura dos signatários anteriores")
	ErrClosed           = errors.New("signum: envelope não está mais pendente")
	ErrChallenge        = errors.New("signum: desafio inválido, expirado ou já utilizado")
	ErrReauth           = errors.New("signum: reautenticação falhou")
	ErrDocumentMismatch = errors.New("signum: o hash do documento confirmado não corresponde ao envelope")
	ErrInvalidHash      = errors.New("signum: hash SHA-256 inválido")
)

// Estados.
const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusRefused   = "refused"
	StatusCancelled = "cancelled"
)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ValidSHA256 reporta se h é um SHA-256 hexadecimal minúsculo.
func ValidSHA256(h string) bool { return sha256Hex.MatchString(h) }

// Envelope agrupa um documento (pelo hash) e seus signatários.
type Envelope struct {
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	DocumentSHA256 string     `json:"document_sha256"`
	SourceModule   string     `json:"source_module"`
	SourceRef      string     `json:"source_ref"`
	Sequential     bool       `json:"sequential"`
	Status         string     `json:"status"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	CreatedByName  string     `json:"created_by_name"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Signers        []Signer   `json:"signers"`
}

// Signer é um signatário do envelope.
type Signer struct {
	ID            uuid.UUID  `json:"id"`
	EnvelopeID    uuid.UUID  `json:"envelope_id"`
	UserID        uuid.UUID  `json:"user_id"`
	Name          string     `json:"name"`
	Position      int        `json:"position"`
	Status        string     `json:"status"`
	SignedAt      *time.Time `json:"signed_at,omitempty"`
	SignatureHash string     `json:"signature_hash,omitempty"`
	Method        string     `json:"method,omitempty"`
	IPAddress     string     `json:"-"`
	UserAgent     string     `json:"-"`
	Reason        string     `json:"reason,omitempty"`
}

// Challenge é o desafio de uso único da cerimônia.
type Challenge struct {
	ID         uuid.UUID
	EnvelopeID uuid.UUID
	UserID     uuid.UUID
	NonceHash  string
	ExpiresAt  time.Time
	UsedAt     *time.Time
}

// CanSign confere se userID pode assinar agora: envelope pendente, e — no
// modo sequencial — ser o primeiro signatário pendente na ordem.
func (e Envelope) CanSign(userID uuid.UUID) error {
	if e.Status != StatusPending {
		return ErrClosed
	}
	for _, s := range e.Signers {
		if s.Status == StatusPending {
			if s.UserID == userID {
				return nil
			}
			if e.Sequential {
				// primeiro pendente na ordem não é o usuário
				return notYourTurnOrNotSigner(e, userID)
			}
		}
	}
	return ErrNotSigner
}

func notYourTurnOrNotSigner(e Envelope, userID uuid.UUID) error {
	for _, s := range e.Signers {
		if s.UserID == userID && s.Status == StatusPending {
			return ErrNotYourTurn
		}
	}
	return ErrNotSigner
}

// AllSigned reporta se todos já assinaram.
func (e Envelope) AllSigned() bool {
	for _, s := range e.Signers {
		if s.Status != "signed" {
			return false
		}
	}
	return len(e.Signers) > 0
}

// SignatureSeal calcula o selo de integridade de uma assinatura:
// HMAC-SHA256(chave do servidor, envelope|documento|signatário|instante|método).
// Qualquer alteração no documento, no signatário ou no instante invalida
// o selo; sem a chave do servidor, nem quem tem acesso ao banco consegue
// forjar um selo válido.
func SignatureSeal(key []byte, envelopeID uuid.UUID, documentSHA256 string, userID uuid.UUID, signedAt time.Time, method string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(strings.Join([]string{
		envelopeID.String(), documentSHA256, userID.String(), signedAt.UTC().Format(time.RFC3339Nano), method,
	}, "|")))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySeal confere um selo em tempo constante.
func VerifySeal(key []byte, envelopeID uuid.UUID, documentSHA256 string, s Signer) bool {
	if s.SignedAt == nil || s.SignatureHash == "" {
		return false
	}
	want := SignatureSeal(key, envelopeID, documentSHA256, s.UserID, *s.SignedAt, s.Method)
	return hmac.Equal([]byte(want), []byte(s.SignatureHash))
}

// HashNonce é o SHA-256 do nonce do desafio (só o hash é persistido).
func HashNonce(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}

// Filter restringe a listagem.
type Filter struct {
	UserID uuid.UUID
	Role   string // to_sign | created | all
	Status string
}

// Repository é a porta de persistência.
type Repository interface {
	Insert(ctx context.Context, db database.DBTX, e Envelope) error
	// UnavailableSigners devolve, dentre ids, os que não existem ou estão
	// desativados (não poderiam assinar nunca).
	UnavailableSigners(ctx context.Context, db database.DBTX, ids []uuid.UUID) ([]uuid.UUID, error)
	Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (Envelope, error)
	List(ctx context.Context, db database.DBTX, f Filter, limit int) ([]Envelope, error)
	SetEnvelopeStatus(ctx context.Context, db database.DBTX, id uuid.UUID, status string) error
	UpdateSigner(ctx context.Context, db database.DBTX, s Signer) error
	InsertChallenge(ctx context.Context, db database.DBTX, c Challenge) error
	ConsumeChallenge(ctx context.Context, db database.DBTX, id, envelopeID, userID uuid.UUID, nonceHash string) error
}

// Reauthenticator confere a senha do usuário na hora da assinatura.
type Reauthenticator interface {
	Reauthenticate(ctx context.Context, userID uuid.UUID, username string, federated bool, password string) (method string, err error)
}
