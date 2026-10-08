package domain

import (
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

// Unidade dona (ADR 013): files:manage cobrindo a dona de qualquer pasta da
// cadeia dá acesso total; marcar exige lotação ou gestão na unidade.
func TestEvaluateUnidadeDona(t *testing.T) {
	un, outra := uuid.New(), uuid.New()
	gestor := auth.Identity{UserID: uuid.New(), Scopes: []auth.Scope{{Permissions: []string{"files:manage"}, UnidadeID: &un, Unidades: []uuid.UUID{un}}}}
	lotado := auth.Identity{UserID: uuid.New(), Scopes: []auth.Scope{{UnidadeID: &outra, Unidades: []uuid.UUID{outra}}}}
	dono := uuid.New()
	herdada := []ChainLink{{Depth: 0, OwnerID: dono}, {Depth: 1, OwnerID: dono, UnidadeID: &un}}
	if got := Evaluate(gestor, herdada); !got.Manage {
		t.Error("gestão da unidade dona (herdada) tem acesso total")
	}
	if got := Evaluate(gestor, []ChainLink{{OwnerID: dono, UnidadeID: &outra}}); got.Read {
		t.Error("gestão de outra unidade não vê")
	}
	if !PodeMarcar(lotado, &outra) || PodeMarcar(lotado, &un) || !PodeMarcar(gestor, &un) || !PodeMarcar(auth.Identity{}, nil) {
		t.Error("marca a dona quem é lotado nela ou a gerencia")
	}
	if Gere(gestor, nil) {
		t.Error("sem dona, gestão com escopo não alcança")
	}
}
