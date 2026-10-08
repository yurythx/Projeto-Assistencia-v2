package application_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// Lotado na unidade em que o processo está age sobre ele, mas tramitar
// exige tramite:route (ou manage) ali e reabrir exige tramite:manage ali —
// ter a permissão em outra unidade não basta (ADR 013).
func TestLotadoSemPermissaoNaUnidadeAtual(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	p := e.processo(domain.SigiloPublico)
	outra := dbtest.Unidade(t, e.pool)
	lotado := auth.Identity{UserID: dbtest.User(t, e.pool), Scopes: []auth.Scope{
		{Perfil: "servidor", UnidadeID: &e.unidade},
		{Perfil: "gestor", UnidadeID: &outra, Unidades: []uuid.UUID{outra}, Permissions: []string{"tramite:*"}},
	}}
	proibido := func(err error, oque string) {
		t.Helper()
		var ae *apperrors.Error
		if !errors.As(err, &ae) || ae.Status != http.StatusForbidden {
			t.Errorf("%s: esperava 403, veio %v", oque, err)
		}
	}
	_, err := e.real().Tramitar(ctx, lotado, p.ID, outra, "Encaminho")
	proibido(err, "tramitar sem tramite:route na unidade atual")
	_, err = e.real().Reabrir(ctx, lotado, p.ID, "Reabro")
	proibido(err, "reabrir sem tramite:manage na unidade atual")
}
