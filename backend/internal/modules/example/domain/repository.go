package domain

import (
	"context"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository é a porta de persistência do módulo. Como em todos os
// plugins, cada método recebe o executor (database.DBTX): o pool numa
// leitura, a transação de negócio numa escrita — o repositório nunca abre
// transação própria, para que o INSERT, o outbox.Write do evento
// "example.item.created" e a auditoria (ver application.Service.CreateItem)
// commitem ou revertam juntos (Transactional Outbox, §16).
type Repository interface {
	Insert(ctx context.Context, db database.DBTX, item Item) error
	Get(ctx context.Context, db database.DBTX, id uuid.UUID) (Item, error)
	List(ctx context.Context, db database.DBTX, p pagination.Params) ([]Item, int64, error)
}
