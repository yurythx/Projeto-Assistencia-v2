// Package application contém os casos de uso do Contato.
package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// EventMessageSubmitted é emitido a cada mensagem recebida. O payload NÃO
// carrega PII (nome, e-mail, telefone): consumidores externos (Egress ->
// n8n, e-mail) consultam o detalhe pela API autenticada, se precisarem.
const EventMessageSubmitted = "contact.message.submitted"

// Service implementa os casos de uso.
type Service struct {
	pool   *pgxpool.Pool
	repo   domain.Repository
	outbox *outbox.Writer
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, ob *outbox.Writer) *Service {
	return &Service{pool: pool, repo: repo, outbox: ob}
}

// MapError traduz erros de domínio.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("mensagem não encontrada")
	case errors.Is(err, domain.ErrAssignee), errors.Is(err, domain.ErrUnidade):
		return apperrors.Validation(err.Error())
	case errors.Is(err, domain.ErrOutOfScope):
		return apperrors.Forbidden(err.Error())
	}
	return err
}

// SubmitInput é a mensagem enviada pelo visitante.
type SubmitInput struct {
	Name, Email, Phone, Subject, Category, Message, IP, UserAgent string
}

// Submit registra a mensagem e emite o evento na mesma transação.
func (s *Service) Submit(ctx context.Context, in SubmitInput) (domain.Message, error) {
	m := domain.Message{
		ID: uuid.New(), Name: strings.TrimSpace(in.Name), Email: strings.ToLower(strings.TrimSpace(in.Email)),
		Phone: strings.TrimSpace(in.Phone), Subject: strings.TrimSpace(in.Subject), Category: in.Category,
		Message: strings.TrimSpace(in.Message), ConsentAt: time.Now().UTC(), IPAddress: in.IP, UserAgent: in.UserAgent,
	}
	if m.Category == "" {
		m.Category = "duvida"
	}
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if m, err = s.repo.Insert(ctx, tx, m); err != nil {
			return err
		}
		if err := s.outbox.Write(ctx, tx, EventMessageSubmitted, "contact_message", m.ID.String(), uuid.Nil, map[string]any{
			"id": m.ID.String(), "protocol": m.Protocol, "category": m.Category, "subject": m.Subject,
		}); err != nil {
			return err
		}
		entry := audit.FromContext(ctx)
		entry.Action, entry.ResourceType, entry.ResourceID = "contact.message.submitted", "contact_message", m.ID.String()
		entry.IPAddress, entry.UserAgent = in.IP, in.UserAgent
		entry.Metadata = map[string]any{"protocol": m.Protocol, "category": m.Category}
		return audit.NewWriter(tx).Record(ctx, entry)
	})
	return m, err
}

// List lista mensagens (sem corpo): a gestão global vê todas; com escopo,
// só as encaminhadas aos setores que cobre (ADR 013).
func (s *Service) List(ctx context.Context, identity auth.Identity, f domain.Filter, p pagination.Params) ([]domain.Summary, int64, error) {
	cobertura := auth.CoverageOf(identity, auth.PermContactRead)
	f.Restrito, f.Unidades = !cobertura.All, cobertura.Unidades
	return s.repo.List(ctx, s.pool, f, p)
}

// Get lê a mensagem completa — um acesso a PII, portanto auditado. Fora do
// escopo, responde como inexistente.
func (s *Service) Get(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.Message, error) {
	m, err := s.repo.Get(ctx, s.pool, id)
	if err != nil {
		return m, MapError(err)
	}
	if !auth.Can(identity, auth.PermContactRead, m.Posicao()) {
		return domain.Message{}, MapError(domain.ErrNotFound)
	}
	_ = audit.NewWriter(s.pool).Record(ctx, audit.Meta(ctx, "contact.message.viewed", "contact_message", id.String(), nil, nil))
	return m, nil
}

// Triage atualiza status, anotações, responsável e setor. Exige
// contact:manage onde a mensagem está; encaminhar exige cobrir também o
// setor de destino — devolver à caixa geral (sem setor) é sempre possível.
func (s *Service) Triage(ctx context.Context, identity auth.Identity, id uuid.UUID, t domain.Triage) (domain.Message, error) {
	var out domain.Message
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if !auth.Can(identity, auth.PermContactManage, prev.Posicao()) {
			if auth.Can(identity, auth.PermContactRead, prev.Posicao()) {
				return domain.ErrOutOfScope
			}
			return domain.ErrNotFound
		}
		if t.UnidadeID != nil && !sameUnidade(prev.UnidadeID, t.UnidadeID) {
			if !auth.Can(identity, auth.PermContactManage, auth.InUnidade(*t.UnidadeID)) {
				return domain.ErrOutOfScope
			}
			ok, err := s.repo.ActiveUnidade(ctx, tx, *t.UnidadeID)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrUnidade
			}
		}
		if t.AssignedTo != nil {
			ok, err := s.repo.ActiveUser(ctx, tx, *t.AssignedTo)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrAssignee
			}
		}
		if err := s.repo.UpdateTriage(ctx, tx, id, t); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, id); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "contact.message.triaged", "contact_message", id.String(),
			map[string]any{"status": prev.Status, "assigned_to": prev.AssignedTo, "unidade_id": prev.UnidadeID},
			map[string]any{"status": out.Status, "assigned_to": out.AssignedTo, "unidade_id": out.UnidadeID}))
	})
	return out, MapError(err)
}

func sameUnidade(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
