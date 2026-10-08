// Package application contém os casos de uso do módulo-modelo.
package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/example/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// EventItemCreated é emitido (via Outbox) ao criar um item — já roteado em
// messaging/topology.go para nexus.notification.websocket e
// nexus.example.worker.
const EventItemCreated = "example.item.created"

// Service gerencia as regras de negócio para o módulo de exemplo.
type Service struct {
	pool   *pgxpool.Pool
	repo   domain.Repository
	outbox *outbox.Writer
	logger *slog.Logger
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, ob *outbox.Writer, logger *slog.Logger) *Service {
	return &Service{pool: pool, repo: repo, outbox: ob, logger: logger}
}

// MapError traduz os erros de domínio para erros de aplicação (HTTP). O
// transporte só chama MapError — nunca conhece os erros de domínio.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrExampleNotFound):
		return apperrors.NotFound("item não encontrado")
	case errors.Is(err, domain.ErrInvalidInput):
		return apperrors.Validation("informe um título (até 200 caracteres)")
	}
	return err
}

// CreateItem é o blueprint de referência do Transactional Outbox (§16): o
// INSERT do item, o outbox.Write do evento e a trilha de auditoria
// commitam na MESMA transação (database.WithTx) — uma falha parcial (item
// gravado mas evento perdido, ou vice-versa) nunca acontece.
func (s *Service) CreateItem(ctx context.Context, title, description string) (domain.Item, error) {
	item, err := domain.NewItem(title, description)
	if err != nil {
		return domain.Item{}, err
	}
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.Insert(ctx, tx, item); err != nil {
			return err
		}
		payload := map[string]string{"id": item.ID.String(), "title": item.Title}
		// uuid.Nil: sem correlation id de negócio próprio — events.New gera um.
		if err := s.outbox.Write(ctx, tx, EventItemCreated, "example_item", item.ID.String(), uuid.Nil, payload); err != nil {
			return err
		}
		// audit.NewWriter(tx) — não o Writer preso ao pool — para que item,
		// evento e auditoria commitem ou revertam juntos. audit.Meta extrai
		// do context o ator, suas roles e o escopo organizacional.
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventItemCreated, "example_item", item.ID.String(), nil, item))
	})
	if err != nil {
		return domain.Item{}, err
	}
	s.logger.Info("example: item criado", slog.String("id", item.ID.String()))
	return item, nil
}

// GetItem busca um item.
func (s *Service) GetItem(ctx context.Context, id uuid.UUID) (domain.Item, error) {
	return s.repo.Get(ctx, s.pool, id)
}

// ListItems pagina os itens (p já vem validado por pagination.New).
func (s *Service) ListItems(ctx context.Context, p pagination.Params) ([]domain.Item, int64, error) {
	return s.repo.List(ctx, s.pool, p)
}
