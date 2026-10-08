// Package application contém os casos de uso do Egress:
//   - dispatcher (consumidor da fila ligada a "#"): materializa uma entrega
//     por destino ativo cujo padrão casa com o tipo do evento;
//   - worker de entrega: POST assinado com HMAC, retentativa exponencial,
//     "dead" após o máximo de tentativas (DLQ lógica, reprocessável).
package application

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/netguard"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
)

// Config parametriza o worker de entrega.
type Config struct {
	MaxAttempts  int
	BatchSize    int
	PollInterval time.Duration
}

// Deliverer é a porta de entrega HTTP (implementada pelo cliente anti-SSRF
// da infraestrutura).
type Deliverer interface {
	Deliver(ctx context.Context, t domain.Target, d domain.Delivery) (int, error)
	Policy() netguard.Policy
}

// Service implementa os casos de uso.
type Service struct {
	pool      *pgxpool.Pool
	repo      domain.Repository
	deliverer Deliverer
	cipher    *secretcrypto.Cipher
	cfg       Config
	logger    *slog.Logger
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, deliverer Deliverer, cipher *secretcrypto.Cipher, cfg Config, logger *slog.Logger) *Service {
	return &Service{pool: pool, repo: repo, deliverer: deliverer, cipher: cipher, cfg: cfg, logger: logger}
}

// MapError traduz erros de domínio.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("destino ou entrega não encontrado")
	case errors.Is(err, netguard.ErrBlockedDestination):
		return apperrors.Validation(err.Error())
	}
	return err
}

var patternFormat = regexp.MustCompile(`^(#|\*|[a-z0-9_]+)(\.(#|\*|[a-z0-9_]+))*$`)

// TargetInput cria/edita um destino.
type TargetInput struct {
	Name          string
	Kind          string
	URL           string
	Secret        *string // nil = manter; "" = remover
	EventPatterns []string
	Active        bool
	Actor         string
}

// Targets lista os destinos.
func (s *Service) Targets(ctx context.Context) ([]domain.Target, error) {
	return s.repo.Targets(ctx, s.pool, false)
}

// SaveTarget valida (anti-SSRF) e grava o destino.
func (s *Service) SaveTarget(ctx context.Context, id uuid.UUID, in TargetInput) (domain.Target, error) {
	u, err := netguard.ValidateURL(in.URL, s.deliverer.Policy())
	if err != nil {
		return domain.Target{}, MapError(err)
	}
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := netguard.ResolveAndCheck(rctx, u.Hostname(), s.deliverer.Policy()); err != nil {
		return domain.Target{}, apperrors.Validation("destino recusado: " + err.Error())
	}
	patterns := []string{}
	for _, p := range in.EventPatterns {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if !patternFormat.MatchString(p) {
			return domain.Target{}, apperrors.Validation("padrão de evento inválido: " + p)
		}
		patterns = append(patterns, p)
	}
	if len(patterns) == 0 {
		patterns = []string{"#"}
	}
	var enc *string
	if in.Secret != nil {
		v := s.cipher.Encrypt(*in.Secret) // "" continua "" (sem segredo)
		enc = &v
	}
	t := domain.Target{ID: id, Name: strings.TrimSpace(in.Name), Kind: in.Kind, URL: u.String(),
		EventPatterns: patterns, Active: in.Active, CreatedBy: in.Actor}
	var out domain.Target
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var before any
		if t.ID == uuid.Nil {
			t.ID = uuid.New()
		} else {
			prev, err := s.repo.Target(ctx, tx, t.ID)
			if err != nil {
				return err
			}
			before = redact(prev)
		}
		var err error
		if out, err = s.repo.SaveTarget(ctx, tx, t, enc); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "egress.target.saved", "egress_target", out.ID.String(), before, redact(out)))
	})
	return out, MapError(err)
}

// redact devolve a forma auditável (URL sem query, que pode conter token).
func redact(t domain.Target) map[string]any {
	u := t.URL
	if parsed, err := url.Parse(t.URL); err == nil {
		parsed.RawQuery = ""
		u = parsed.String()
	}
	return map[string]any{"name": t.Name, "kind": t.Kind, "url": u, "has_secret": t.HasSecret,
		"event_patterns": t.EventPatterns, "active": t.Active}
}

// DeleteTarget remove um destino (e suas entregas).
func (s *Service) DeleteTarget(ctx context.Context, id uuid.UUID) error {
	return MapError(database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.Target(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.repo.DeleteTarget(ctx, tx, id); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "egress.target.deleted", "egress_target", id.String(), redact(prev), nil))
	}))
}

func (s *Service) withSecret(ctx context.Context, t domain.Target) (domain.Target, error) {
	enc, err := s.repo.EncryptedSecret(ctx, s.pool, t.ID)
	if err != nil || enc == "" {
		return t, err
	}
	t.Secret, err = s.cipher.Decrypt(enc)
	return t, err
}

// TestResult é o resultado do teste de um destino.
type TestResult struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
	TookMS     int64  `json:"took_ms"`
}

// Test envia um evento sintético "egress.test" ao destino, na hora.
func (s *Service) Test(ctx context.Context, id uuid.UUID) (TestResult, error) {
	t, err := s.repo.Target(ctx, s.pool, id)
	if err != nil {
		return TestResult{}, MapError(err)
	}
	if t, err = s.withSecret(ctx, t); err != nil {
		return TestResult{}, apperrors.Internal(err)
	}
	payload, _ := json.Marshal(map[string]string{"message": "Teste de conectividade do Nexus Egress"})
	start := time.Now()
	code, derr := s.deliverer.Deliver(ctx, t, domain.Delivery{ID: uuid.New(), EventID: uuid.New(), EventType: "egress.test",
		Payload: payload, CreatedAt: time.Now()})
	res := TestResult{OK: derr == nil, StatusCode: code, TookMS: time.Since(start).Milliseconds()}
	if derr != nil {
		res.Error = derr.Error()
	}
	_ = audit.NewWriter(s.pool).Record(ctx, audit.Meta(ctx, "egress.target.tested", "egress_target", id.String(), nil, res))
	return res, nil
}

// Deliveries lista entregas.
func (s *Service) Deliveries(ctx context.Context, targetID *uuid.UUID, status string, p pagination.Params) ([]domain.Delivery, int64, error) {
	return s.repo.Deliveries(ctx, s.pool, targetID, status, p)
}

// Redeliver recoloca uma entrega falha/morta na fila.
func (s *Service) Redeliver(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Redeliver(ctx, s.pool, id); err != nil {
		return MapError(err)
	}
	return audit.NewWriter(s.pool).Record(ctx, audit.Meta(ctx, "egress.delivery.requeued", "egress_delivery", id.String(), nil, nil))
}

// Dispatch é o handler do consumidor do barramento: enfileira uma entrega
// por destino compatível (idempotente por target_id+event_id).
func (s *Service) Dispatch(ctx context.Context, ev events.Event) error {
	targets, err := s.repo.Targets(ctx, s.pool, true)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if !t.Matches(ev.Type) {
			continue
		}
		if err := s.repo.EnqueueDelivery(ctx, s.pool, domain.Delivery{
			TargetID: t.ID, EventID: ev.ID, EventType: ev.Type, Payload: ev.Payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

// RunDeliveries é o worker de entrega (processo worker). Encerra quando
// ctx é cancelado (shutdown ou desativação do módulo).
func (s *Service) RunDeliveries(ctx context.Context) error {
	interval := s.cfg.PollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		s.deliverBatch(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (s *Service) deliverBatch(ctx context.Context) {
	due, err := s.repo.ClaimDue(ctx, s.pool, s.cfg.BatchSize)
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Warn("egress: falha ao reservar entregas", slog.Any("error", err))
		}
		return
	}
	targets := map[uuid.UUID]domain.Target{}
	for _, d := range due {
		if ctx.Err() != nil {
			return
		}
		t, ok := targets[d.TargetID]
		if !ok {
			if t, err = s.repo.Target(ctx, s.pool, d.TargetID); err != nil {
				// A reserva expira e a entrega volta na próxima rodada.
				s.logger.Warn("egress: destino da entrega indisponível", slog.String("delivery", d.ID.String()), slog.Any("error", err))
				continue
			}
			if t, err = s.withSecret(ctx, t); err != nil {
				s.fail(ctx, d, 0, errors.New("segredo do destino indecifrável (chave de cifragem trocada?)"))
				continue
			}
			targets[d.TargetID] = t
		}
		// Destino desativado depois da reserva: a entrega espera a
		// reativação (sem tentar nem gastar tentativas).
		if !t.Active {
			continue
		}
		code, derr := s.deliverer.Deliver(ctx, t, d)
		if derr != nil {
			s.fail(ctx, d, code, derr)
			continue
		}
		if err := s.repo.MarkDelivered(ctx, s.pool, d.ID, code); err != nil {
			s.logger.Error("egress: entrega feita mas não registrada (será reenviada; o destino deduplica pelo X-Idempotency-Key)",
				slog.String("delivery", d.ID.String()), slog.Any("error", err))
		}
	}
}

// fail registra a tentativa falha com retentativa exponencial; vira "dead"
// (DLQ lógica, reprocessável) no limite de tentativas ou se o destino foi
// bloqueado pela política anti-SSRF.
func (s *Service) fail(ctx context.Context, d domain.Delivery, code int, derr error) {
	attempts := d.Attempts + 1
	dead := attempts >= s.cfg.MaxAttempts || errors.Is(derr, netguard.ErrBlockedDestination)
	var sc *int
	if code > 0 {
		sc = &code
	}
	msg := derr.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if err := s.repo.MarkFailed(ctx, s.pool, d.ID, attempts, sc, msg, time.Now().Add(domain.Backoff(attempts)), dead); err != nil {
		s.logger.Error("egress: falha de entrega não registrada", slog.String("delivery", d.ID.String()), slog.Any("error", err))
	}
	s.logger.Warn("egress: entrega falhou", slog.String("delivery", d.ID.String()), slog.Int("attempt", attempts), slog.Bool("dead", dead))
}
