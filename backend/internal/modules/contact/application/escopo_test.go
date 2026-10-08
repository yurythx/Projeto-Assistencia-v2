package application_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/contact/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// gestor é a gestão global (nexus-admin equivale a "*" em todo lugar).
var gestor = auth.Identity{Roles: []string{auth.RoleAdmin}}

func noSetor(perms []string, un uuid.UUID) auth.Identity {
	return auth.Identity{Scopes: []auth.Scope{{Permissions: perms, UnidadeID: &un, Unidades: []uuid.UUID{un}}}}
}

func erroCom(t *testing.T, err error, trecho, oque string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), trecho) {
		t.Fatalf("%s: esperava %q, veio %v", oque, trecho, err)
	}
}

// Contato por setor (ADR 013, fase 3): a caixa geral é da gestão global; a
// triagem encaminha a um setor, e a gestão com escopo vê e trata só o que
// chegou à área dela — podendo devolver à caixa geral.
func TestContatoEncaminhadoPorSetor(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	svc := e.svc(infrastructure.NewRepository())
	setorA, setorB := dbtest.Unidade(t, e.pool), dbtest.Unidade(t, e.pool)
	gestorA := noSetor([]string{"contact:*"}, setorA)
	leitorA := noSetor([]string{"contact:read"}, setorA)
	m := e.submit()
	lista := func(who auth.Identity) []uuid.UUID {
		t.Helper()
		items, _, err := svc.List(ctx, who, domain.Filter{}, pagination.New(1, 100, 100))
		if err != nil {
			t.Fatal(err)
		}
		ids := []uuid.UUID{}
		for _, s := range items {
			ids = append(ids, s.ID)
		}
		return ids
	}

	// Caixa geral: só a gestão global.
	if slices.Contains(lista(gestorA), m.ID) {
		t.Fatal("caixa geral não aparece para a gestão do setor")
	}
	_, err := svc.Get(ctx, gestorA, m.ID)
	erroCom(t, err, "não encontrada", "gestão do setor lê a caixa geral")
	_, err = svc.Triage(ctx, gestorA, m.ID, domain.Triage{Status: "in_progress", UnidadeID: &setorA})
	erroCom(t, err, "não encontrada", "gestão do setor puxa da caixa geral")

	// A triagem global encaminha ao setor A.
	if _, err := svc.Triage(ctx, gestor, m.ID, domain.Triage{Status: "new", UnidadeID: &setorA}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(lista(gestorA), m.ID) || !slices.Contains(lista(leitorA), m.ID) {
		t.Fatal("mensagem encaminhada aparece para o setor A")
	}
	got, err := svc.Get(ctx, leitorA, m.ID)
	if err != nil || got.UnidadeID == nil || *got.UnidadeID != setorA {
		t.Fatalf("setor A lê a mensagem: %+v %v", got, err)
	}
	_, err = svc.Triage(ctx, leitorA, m.ID, domain.Triage{Status: "answered", UnidadeID: &setorA})
	erroCom(t, err, "escopo", "só leitura não trata")
	_, err = svc.Triage(ctx, gestorA, m.ID, domain.Triage{Status: "in_progress", UnidadeID: &setorB})
	erroCom(t, err, "escopo", "setor A não encaminha para B")
	if _, err := svc.Triage(ctx, gestorA, m.ID, domain.Triage{Status: "in_progress", Notes: "em análise", UnidadeID: &setorA}); err != nil {
		t.Fatalf("setor A trata a mensagem: %v", err)
	}

	// Unidade desativada não recebe encaminhamento.
	if _, err := e.pool.Exec(ctx, `UPDATE unidades SET ativo = false WHERE id = $1`, setorB); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Triage(ctx, gestor, m.ID, domain.Triage{Status: "in_progress", UnidadeID: &setorB})
	erroCom(t, err, "desativada", "encaminhar para setor desativado")

	// Devolver à caixa geral: o setor sempre pode.
	got, err = svc.Triage(ctx, gestorA, m.ID, domain.Triage{Status: "new"})
	if err != nil || got.UnidadeID != nil {
		t.Fatalf("setor A devolve à caixa geral: %+v %v", got, err)
	}
	if slices.Contains(lista(gestorA), m.ID) || !slices.Contains(lista(gestor), m.ID) {
		t.Fatal("devolvida, volta a ser só da gestão global")
	}
}
