package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

type innerRepo = domain.Repository

var errBoom = errors.New("falha simulada no repositório")

// faultRepo envolve o repositório real: na chamada failAt devolve erro; depois
// da chamada poisonAt "envenena" a transação (a instrução SQL seguinte — do
// repositório, do outbox ou da auditoria — falha). noTx marca veneno em
// leitura fora de transação, onde não há instrução seguinte na mesma conexão.
type faultRepo struct {
	inner                   innerRepo
	calls, failAt, poisonAt int
	trace                   []string
	noTx                    bool
}

func (fr *faultRepo) hook(name string) error {
	fr.calls++
	fr.trace = append(fr.trace, name)
	if fr.calls == fr.failAt {
		return errBoom
	}
	return nil
}

func (fr *faultRepo) post(ctx context.Context, db database.DBTX) {
	if fr.calls != fr.poisonAt {
		return
	}
	if _, inTx := db.(pgx.Tx); !inTx {
		fr.noTx = true
		return
	}
	_, _ = db.Exec(ctx, `SELECT 1/0`)
}

func (fr *faultRepo) Targets(ctx context.Context, db database.DBTX, onlyActive bool) ([]domain.Target, error) {
	if err := fr.hook("Targets"); err != nil {
		var z0 []domain.Target
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Targets(ctx, db, onlyActive)
}

func (fr *faultRepo) Target(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Target, error) {
	if err := fr.hook("Target"); err != nil {
		var z0 domain.Target
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Target(ctx, db, id)
}

func (fr *faultRepo) SaveTarget(ctx context.Context, db database.DBTX, t domain.Target, secretEncrypted *string) (domain.Target, error) {
	if err := fr.hook("SaveTarget"); err != nil {
		var z0 domain.Target
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.SaveTarget(ctx, db, t, secretEncrypted)
}

func (fr *faultRepo) DeleteTarget(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteTarget"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteTarget(ctx, db, id)
}

func (fr *faultRepo) EncryptedSecret(ctx context.Context, db database.DBTX, id uuid.UUID) (string, error) {
	if err := fr.hook("EncryptedSecret"); err != nil {
		var z0 string
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.EncryptedSecret(ctx, db, id)
}

func (fr *faultRepo) EnqueueDelivery(ctx context.Context, db database.DBTX, d domain.Delivery) error {
	if err := fr.hook("EnqueueDelivery"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.EnqueueDelivery(ctx, db, d)
}

func (fr *faultRepo) ClaimDue(ctx context.Context, db database.DBTX, limit int) ([]domain.Delivery, error) {
	if err := fr.hook("ClaimDue"); err != nil {
		var z0 []domain.Delivery
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ClaimDue(ctx, db, limit)
}

func (fr *faultRepo) MarkDelivered(ctx context.Context, db database.DBTX, id uuid.UUID, statusCode int) error {
	if err := fr.hook("MarkDelivered"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.MarkDelivered(ctx, db, id, statusCode)
}

func (fr *faultRepo) MarkFailed(ctx context.Context, db database.DBTX, id uuid.UUID, attempts int, statusCode *int, errMsg string, next time.Time, dead bool) error {
	if err := fr.hook("MarkFailed"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.MarkFailed(ctx, db, id, attempts, statusCode, errMsg, next, dead)
}

func (fr *faultRepo) Deliveries(ctx context.Context, db database.DBTX, targetID *uuid.UUID, status string, p pagination.Params) ([]domain.Delivery, int64, error) {
	if err := fr.hook("Deliveries"); err != nil {
		var z0 []domain.Delivery
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Deliveries(ctx, db, targetID, status, p)
}

func (fr *faultRepo) Redeliver(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("Redeliver"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Redeliver(ctx, db, id)
}
