package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/wiki/application"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// gestor é a gestão global (nexus-admin equivale a "*" em todo lugar).
var gestor = auth.Identity{Roles: []string{auth.RoleAdmin}}

func foraDoEscopo(t *testing.T, err error, oque string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "escopo") {
		t.Fatalf("%s: esperava fora do escopo, veio %v", oque, err)
	}
}

// Dono organizacional da página (ADR 013): qualquer autenticado cria e
// edita, mas só marca como dona uma unidade em que está lotado (ou que
// wiki:manage cobre); excluir exige wiki:manage cobrindo a dona.
func TestUnidadeDonaEEscopo(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	svc := application.NewService(pool, infrastructure.NewRepository())
	un := dbtest.Unidade(t, pool)
	estranho := auth.Identity{UserID: dbtest.User(t, pool)}
	lotado := auth.Identity{UserID: dbtest.User(t, pool), Scopes: []auth.Scope{{UnidadeID: &un, Unidades: []uuid.UUID{un}}}}
	gestorUn := auth.Identity{UserID: dbtest.User(t, pool), Scopes: []auth.Scope{{Permissions: []string{"wiki:manage"}, UnidadeID: &un, Unidades: []uuid.UUID{un}}}}
	titulo := func() string { return "Página " + uuid.NewString()[:8] }

	_, err := svc.Create(ctx, estranho, application.Input{Title: titulo(), UnidadeID: &un})
	foraDoEscopo(t, err, "estranho marca unidade alheia")
	raiz, err := svc.Create(ctx, lotado, application.Input{Title: titulo(), UnidadeID: &un})
	if err != nil || raiz.UnidadeID == nil || *raiz.UnidadeID != un {
		t.Fatalf("lotado marca a própria unidade: %+v %v", raiz, err)
	}
	filha, err := svc.Create(ctx, estranho, application.Input{ParentID: &raiz.ID, Title: titulo()})
	if err != nil || filha.UnidadeID == nil || *filha.UnidadeID != un {
		t.Fatalf("subpágina herda a dona da mãe: %+v %v", filha, err)
	}
	_, err = svc.Create(ctx, estranho, application.Input{ParentID: &raiz.ID, Title: titulo(), UnidadeID: &un})
	foraDoEscopo(t, err, "estranho marca explicitamente sob a mãe")
	if _, err := svc.Create(ctx, gestorUn, application.Input{ParentID: &raiz.ID, Title: titulo(), UnidadeID: &un}); err != nil {
		t.Fatalf("wiki:manage na unidade marca: %v", err)
	}

	// Editar sem trocar a dona é livre; trocar exige poder marcar as duas.
	filha, err = svc.Update(ctx, estranho, filha.ID, filha.Version, application.Input{ParentID: &raiz.ID, Title: filha.Title, Body: "editada", UnidadeID: &un})
	if err != nil {
		t.Fatalf("editar mantendo a dona: %v", err)
	}
	_, err = svc.Update(ctx, estranho, filha.ID, filha.Version, application.Input{ParentID: &raiz.ID, Title: filha.Title})
	foraDoEscopo(t, err, "estranho tira a dona")
	filha, err = svc.Update(ctx, lotado, filha.ID, filha.Version, application.Input{ParentID: &raiz.ID, Title: filha.Title, Body: "institucional"})
	if err != nil || filha.UnidadeID != nil {
		t.Fatalf("lotado devolve ao institucional: %+v %v", filha, err)
	}
	restaurada, err := svc.Restore(ctx, estranho, filha.ID, 1)
	if err != nil || restaurada.UnidadeID != nil {
		t.Fatalf("restaurar preserva a dona atual: %+v %v", restaurada, err)
	}

	// Excluir: wiki:manage cobrindo a dona; institucional só a gestão global.
	foraDoEscopo(t, svc.Delete(ctx, estranho, filha.ID), "estranho exclui")
	foraDoEscopo(t, svc.Delete(ctx, gestorUn, filha.ID), "gestor da unidade exclui institucional")
	if err := svc.Delete(ctx, gestor, filha.ID); err != nil {
		t.Fatalf("gestão global exclui institucional: %v", err)
	}
	outra, err := svc.Create(ctx, lotado, application.Input{Title: titulo(), UnidadeID: &un})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, gestorUn, outra.ID); err != nil {
		t.Fatalf("wiki:manage na unidade exclui a página dela: %v", err)
	}
}
