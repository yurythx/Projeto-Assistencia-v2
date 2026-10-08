// Package application contém a cerimônia de assinatura do Signum.
//
// Fluxo: (1) o signatário pede um desafio (nonce de uso único, curto);
// (2) confirma o hash SHA-256 do documento que está vendo e digita a
// senha; (3) o Signum reautentica, consome o desafio, sela a assinatura
// com HMAC-SHA256 e — se foi a última — fecha o envelope. Tudo numa
// transação com o evento no Outbox e a auditoria.
package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/signum/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// Eventos emitidos.
const (
	EventSigned    = "signum.envelope.signed"
	EventCompleted = "signum.envelope.completed"
	EventRefused   = "signum.envelope.refused"
	EventCancelled = "signum.envelope.cancelled"
)

// Throttle limita tentativas de senha na cerimônia (lockout progressivo).
type Throttle interface {
	LockedFor(ctx context.Context, subject string) (time.Duration, error)
	RegisterFailure(ctx context.Context, subject string) (time.Duration, error)
	Reset(ctx context.Context, subject string) error
}

// Service implementa os casos de uso.
type Service struct {
	pool         *pgxpool.Pool
	repo         domain.Repository
	reauth       domain.Reauthenticator
	throttle     Throttle
	outbox       *outbox.Writer
	sealKey      []byte
	challengeTTL time.Duration
	logger       *slog.Logger
}

// NewService cria o serviço. secret deriva a chave do selo HMAC.
func NewService(pool *pgxpool.Pool, repo domain.Repository, reauth domain.Reauthenticator, throttle Throttle, ob *outbox.Writer, secret string, challengeTTL time.Duration, logger *slog.Logger) *Service {
	key := sha256.Sum256([]byte("nexus-signum-seal:" + secret))
	return &Service{pool: pool, repo: repo, reauth: reauth, throttle: throttle, outbox: ob, sealKey: key[:], challengeTTL: challengeTTL, logger: logger}
}

// MapError traduz erros de domínio.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("envelope não encontrado")
	case errors.Is(err, domain.ErrNotSigner):
		return apperrors.Forbidden(err.Error())
	case errors.Is(err, domain.ErrNotYourTurn), errors.Is(err, domain.ErrClosed):
		return apperrors.Conflict(err.Error())
	case errors.Is(err, domain.ErrChallenge), errors.Is(err, domain.ErrDocumentMismatch), errors.Is(err, domain.ErrInvalidHash):
		return apperrors.Validation(err.Error())
	case errors.Is(err, domain.ErrReauth):
		return apperrors.Unauthorized("senha incorreta — a assinatura não foi realizada").WithCode("SIGNUM_REAUTH_FAILED")
	}
	return err
}

// OpenRequest abre um envelope.
type OpenRequest struct {
	Title          string
	Description    string
	DocumentSHA256 string
	SignerIDs      []uuid.UUID
	Sequential     bool
	SourceModule   string
	SourceRef      string
}

// OpenTx abre um envelope DENTRO da transação de quem chama (ex.: o
// Trâmite, que muda o status do documento na mesma transação).
func (s *Service) OpenTx(ctx context.Context, tx pgx.Tx, req OpenRequest) (domain.Envelope, error) {
	req.DocumentSHA256 = strings.ToLower(strings.TrimSpace(req.DocumentSHA256))
	if !domain.ValidSHA256(req.DocumentSHA256) {
		return domain.Envelope{}, MapError(domain.ErrInvalidHash)
	}
	if len(req.SignerIDs) == 0 || len(req.SignerIDs) > 50 {
		return domain.Envelope{}, apperrors.Validation("informe de 1 a 50 signatários")
	}
	creator, _, ok := auth.ActorUUID(ctx)
	if !ok {
		return domain.Envelope{}, apperrors.Unauthorized("identidade necessária para abrir um envelope")
	}
	unavailable, err := s.repo.UnavailableSigners(ctx, tx, req.SignerIDs)
	if err != nil {
		return domain.Envelope{}, err
	}
	if len(unavailable) > 0 {
		return domain.Envelope{}, apperrors.Validation("signatário inexistente ou desativado: " + unavailable[0].String())
	}
	seen := map[uuid.UUID]bool{}
	e := domain.Envelope{
		ID: uuid.New(), Title: strings.TrimSpace(req.Title), Description: req.Description, DocumentSHA256: req.DocumentSHA256,
		SourceModule: req.SourceModule, SourceRef: req.SourceRef, Sequential: req.Sequential, Status: domain.StatusPending, CreatedBy: *creator,
	}
	for i, id := range req.SignerIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		e.Signers = append(e.Signers, domain.Signer{UserID: id, Position: i, Status: domain.StatusPending})
	}
	if err := s.repo.Insert(ctx, tx, e); err != nil {
		return domain.Envelope{}, err
	}
	if err := audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "signum.envelope.opened", "signum_envelope", e.ID.String(), nil,
		map[string]any{"title": e.Title, "document_sha256": e.DocumentSHA256, "signers": req.SignerIDs, "source_module": e.SourceModule, "source_ref": e.SourceRef})); err != nil {
		return domain.Envelope{}, err
	}
	return s.repo.Get(ctx, tx, e.ID, false)
}

// Open abre um envelope avulso (API do próprio Signum).
func (s *Service) Open(ctx context.Context, req OpenRequest) (domain.Envelope, error) {
	var out domain.Envelope
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = s.OpenTx(ctx, tx, req)
		return err
	})
	return out, MapError(err)
}

func canView(identity auth.Identity, e domain.Envelope) bool {
	if e.CreatedBy == identity.UserID || auth.HasPermission(identity, auth.PermSignumManage) {
		return true
	}
	for _, sg := range e.Signers {
		if sg.UserID == identity.UserID {
			return true
		}
	}
	return false
}

// Get devolve um envelope visível ao usuário.
func (s *Service) Get(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.Envelope, error) {
	e, err := s.repo.Get(ctx, s.pool, id, false)
	if err != nil {
		return e, MapError(err)
	}
	if !canView(identity, e) {
		return domain.Envelope{}, MapError(domain.ErrNotFound)
	}
	return e, nil
}

// List lista envelopes do usuário (role: to_sign | created | all; "any"
// exige signum:manage).
func (s *Service) List(ctx context.Context, identity auth.Identity, role, status string) ([]domain.Envelope, error) {
	if role == "any" && !auth.HasPermission(identity, auth.PermSignumManage) {
		role = "all"
	}
	return s.repo.List(ctx, s.pool, domain.Filter{UserID: identity.UserID, Role: role, Status: status}, 200)
}

// ChallengeResponse é devolvido ao iniciar a cerimônia.
type ChallengeResponse struct {
	ChallengeID    uuid.UUID `json:"challenge_id"`
	Nonce          string    `json:"nonce"`
	ExpiresAt      time.Time `json:"expires_at"`
	DocumentSHA256 string    `json:"document_sha256"`
}

// Challenge inicia a cerimônia para o signatário da vez.
func (s *Service) Challenge(ctx context.Context, identity auth.Identity, envelopeID uuid.UUID) (ChallengeResponse, error) {
	e, err := s.repo.Get(ctx, s.pool, envelopeID, false)
	if err != nil {
		return ChallengeResponse{}, MapError(err)
	}
	if err := e.CanSign(identity.UserID); err != nil {
		return ChallengeResponse{}, MapError(err)
	}
	raw := make([]byte, 32)
	_, _ = rand.Read(raw) // crypto/rand.Read nunca falha (Go 1.24+)
	nonce := hex.EncodeToString(raw)
	c := domain.Challenge{ID: uuid.New(), EnvelopeID: envelopeID, UserID: identity.UserID, NonceHash: domain.HashNonce(nonce),
		ExpiresAt: time.Now().Add(s.challengeTTL)}
	if err := s.repo.InsertChallenge(ctx, s.pool, c); err != nil {
		return ChallengeResponse{}, err
	}
	return ChallengeResponse{ChallengeID: c.ID, Nonce: nonce, ExpiresAt: c.ExpiresAt, DocumentSHA256: e.DocumentSHA256}, nil
}

// SignInput é a conclusão da cerimônia.
type SignInput struct {
	ChallengeID   uuid.UUID
	Nonce         string
	Password      string
	ConfirmSHA256 string
	IP, UserAgent string
}

// precheck valida envelope, vez e documento ANTES de pedir a senha: uma
// tentativa em envelope errado não consome tentativas de senha nem gera
// alarme falso de reautenticação. A mesma checagem se repete sob lock.
func (s *Service) precheck(ctx context.Context, db database.DBTX, identity auth.Identity, envelopeID uuid.UUID, confirmSHA256 string, forUpdate bool) (domain.Envelope, error) {
	e, err := s.repo.Get(ctx, db, envelopeID, forUpdate)
	if err != nil {
		return e, err
	}
	if err := e.CanSign(identity.UserID); err != nil {
		return e, err
	}
	if strings.ToLower(strings.TrimSpace(confirmSHA256)) != e.DocumentSHA256 {
		return e, domain.ErrDocumentMismatch
	}
	return e, nil
}

// Sign conclui a assinatura.
func (s *Service) Sign(ctx context.Context, identity auth.Identity, envelopeID uuid.UUID, in SignInput) (domain.Envelope, error) {
	if _, err := s.precheck(ctx, s.pool, identity, envelopeID, in.ConfirmSHA256, false); err != nil {
		return domain.Envelope{}, MapError(err)
	}
	throttleKey := "signum:" + identity.UserID.String()
	if s.throttle != nil {
		if left, err := s.throttle.LockedFor(ctx, throttleKey); err == nil && left > 0 {
			return domain.Envelope{}, apperrors.RateLimited("muitas tentativas de senha; aguarde para assinar novamente")
		}
	}
	method, err := s.reauth.Reauthenticate(ctx, identity.UserID, identity.Username, identity.Source == auth.SourceKeycloak, in.Password)
	if err != nil {
		if errors.Is(err, domain.ErrReauth) {
			if s.throttle != nil {
				_, _ = s.throttle.RegisterFailure(ctx, throttleKey)
			}
			_ = audit.NewWriter(s.pool).Record(ctx, audit.Meta(ctx, "signum.reauth.failed", "signum_envelope", envelopeID.String(), nil, nil))
		}
		return domain.Envelope{}, MapError(err)
	}
	if s.throttle != nil {
		_ = s.throttle.Reset(ctx, throttleKey)
	}

	var out domain.Envelope
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		e, err := s.precheck(ctx, tx, identity, envelopeID, in.ConfirmSHA256, true)
		if err != nil {
			return err
		}
		if err := s.repo.ConsumeChallenge(ctx, tx, in.ChallengeID, envelopeID, identity.UserID, domain.HashNonce(in.Nonce)); err != nil {
			return err
		}
		// Precisão do timestamptz do Postgres (microssegundos): o selo tem
		// de ser calculado sobre o MESMO instante que fica persistido, senão
		// a verificação posterior nunca confere.
		now := time.Now().UTC().Truncate(time.Microsecond)
		var signer domain.Signer
		for _, sg := range e.Signers {
			if sg.UserID == identity.UserID {
				signer = sg
			}
		}
		signer.Status, signer.SignedAt, signer.Method = "signed", &now, method
		signer.IPAddress, signer.UserAgent = in.IP, truncate(in.UserAgent, 400)
		signer.SignatureHash = domain.SignatureSeal(s.sealKey, e.ID, e.DocumentSHA256, identity.UserID, now, method)
		if err := s.repo.UpdateSigner(ctx, tx, signer); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, envelopeID, false); err != nil {
			return err
		}
		payload := s.payload(out)
		payload["signer_id"] = identity.UserID.String()
		if err := s.outbox.Write(ctx, tx, EventSigned, "signum_envelope", e.ID.String(), uuid.Nil, payload); err != nil {
			return err
		}
		if out.AllSigned() {
			if err := s.repo.SetEnvelopeStatus(ctx, tx, e.ID, domain.StatusCompleted); err != nil {
				return err
			}
			if out, err = s.repo.Get(ctx, tx, envelopeID, false); err != nil {
				return err
			}
			if err := s.outbox.Write(ctx, tx, EventCompleted, "signum_envelope", e.ID.String(), uuid.Nil, s.payload(out)); err != nil {
				return err
			}
		}
		entry := audit.Meta(ctx, "signum.envelope.signed", "signum_envelope", e.ID.String(), nil,
			map[string]any{"document_sha256": e.DocumentSHA256, "method": method, "signature_hash": signer.SignatureHash, "completed": out.Status == domain.StatusCompleted})
		entry.IPAddress, entry.UserAgent = in.IP, in.UserAgent
		return audit.NewWriter(tx).Record(ctx, entry)
	})
	return out, MapError(err)
}

// Refuse registra a recusa (encerra o envelope).
func (s *Service) Refuse(ctx context.Context, identity auth.Identity, envelopeID uuid.UUID, reason string) (domain.Envelope, error) {
	var out domain.Envelope
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		e, err := s.repo.Get(ctx, tx, envelopeID, true)
		if err != nil {
			return err
		}
		if err := e.CanSign(identity.UserID); err != nil && !errors.Is(err, domain.ErrNotYourTurn) {
			return err
		}
		for _, sg := range e.Signers {
			if sg.UserID == identity.UserID {
				now := time.Now().UTC()
				sg.Status, sg.Reason, sg.SignedAt = domain.StatusRefused, strings.TrimSpace(reason), &now
				if err := s.repo.UpdateSigner(ctx, tx, sg); err != nil {
					return err
				}
			}
		}
		if err := s.repo.SetEnvelopeStatus(ctx, tx, envelopeID, domain.StatusRefused); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, envelopeID, false); err != nil {
			return err
		}
		payload := s.payload(out)
		payload["refused_by"] = identity.UserID.String()
		if err := s.outbox.Write(ctx, tx, EventRefused, "signum_envelope", envelopeID.String(), uuid.Nil, payload); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "signum.envelope.refused", "signum_envelope", envelopeID.String(), nil,
			map[string]string{"reason": reason}))
	})
	return out, MapError(err)
}

// Cancel cancela um envelope pendente (criador ou signum:manage).
func (s *Service) Cancel(ctx context.Context, identity auth.Identity, envelopeID uuid.UUID) (domain.Envelope, error) {
	var out domain.Envelope
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		e, err := s.repo.Get(ctx, tx, envelopeID, true)
		if err != nil {
			return err
		}
		if e.CreatedBy != identity.UserID && !auth.HasPermission(identity, auth.PermSignumManage) {
			return domain.ErrNotSigner
		}
		if e.Status != domain.StatusPending {
			return domain.ErrClosed
		}
		if err := s.repo.SetEnvelopeStatus(ctx, tx, envelopeID, domain.StatusCancelled); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, envelopeID, false); err != nil {
			return err
		}
		if err := s.outbox.Write(ctx, tx, EventCancelled, "signum_envelope", envelopeID.String(), uuid.Nil, s.payload(out)); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "signum.envelope.cancelled", "signum_envelope", envelopeID.String(),
			map[string]string{"status": e.Status}, map[string]string{"status": domain.StatusCancelled}))
	})
	return out, MapError(err)
}

// Verification é a verificação pública de autenticidade.
type Verification struct {
	EnvelopeID     uuid.UUID        `json:"envelope_id"`
	Title          string           `json:"title"`
	DocumentSHA256 string           `json:"document_sha256"`
	Status         string           `json:"status"`
	DocumentMatch  *bool            `json:"document_match,omitempty"`
	Signatures     []SignatureCheck `json:"signatures"`
	CheckedAt      time.Time        `json:"checked_at"`
}

// SignatureCheck é o resultado por signatário.
type SignatureCheck struct {
	Name     string     `json:"name"`
	Status   string     `json:"status"`
	SignedAt *time.Time `json:"signed_at,omitempty"`
	Method   string     `json:"method,omitempty"`
	Valid    bool       `json:"valid"`
}

// Verify confere os selos de um envelope; se documentHash vier, confere
// também se o documento apresentado é o assinado.
func (s *Service) Verify(ctx context.Context, envelopeID uuid.UUID, documentHash string) (Verification, error) {
	e, err := s.repo.Get(ctx, s.pool, envelopeID, false)
	if err != nil {
		return Verification{}, MapError(err)
	}
	v := Verification{EnvelopeID: e.ID, Title: e.Title, DocumentSHA256: e.DocumentSHA256, Status: e.Status, CheckedAt: time.Now().UTC(),
		Signatures: []SignatureCheck{}}
	if documentHash != "" {
		match := strings.ToLower(documentHash) == e.DocumentSHA256
		v.DocumentMatch = &match
	}
	for _, sg := range e.Signers {
		v.Signatures = append(v.Signatures, SignatureCheck{
			Name: sg.Name, Status: sg.Status, SignedAt: sg.SignedAt, Method: sg.Method,
			Valid: sg.Status == "signed" && domain.VerifySeal(s.sealKey, e.ID, e.DocumentSHA256, sg),
		})
	}
	return v, nil
}

func (s *Service) payload(e domain.Envelope) map[string]any {
	return map[string]any{
		"envelope_id": e.ID.String(), "status": e.Status, "document_sha256": e.DocumentSHA256,
		"source_module": e.SourceModule, "source_ref": e.SourceRef, "title": e.Title,
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
