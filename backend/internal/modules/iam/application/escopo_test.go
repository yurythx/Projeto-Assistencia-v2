package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/iam/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// Administração delegada (ADR 013): falha do repositório nas checagens de
// área (lotações da conta, visibilidade) chega a quem chamou.
func TestDelegadoPropagaFalhasDaArea(t *testing.T) {
	e := newEnv(t)
	un := dbtest.Unidade(t, e.pool)
	alvo := dbtest.User(t, e.pool)
	delegado := auth.Identity{UserID: dbtest.User(t, e.pool), Scopes: []auth.Scope{{Permissions: []string{"users:*"}, UnidadeID: &un, Unidades: []uuid.UUID{un}}}}
	ctx := auth.WithIdentity(context.Background(), delegado)
	// Unlock: GetUser (1) e UserGrants (2).
	if err := e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 2}).Unlock(ctx, alvo); !errors.Is(err, errBoom) {
		t.Errorf("lotações da conta com o banco fora: %v", err)
	}
	// GetUser: GetUser (1) e UserVisible (2).
	if _, err := e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 2}).GetUser(ctx, alvo); !errors.Is(err, errBoom) {
		t.Errorf("visibilidade da conta com o banco fora: %v", err)
	}
}
