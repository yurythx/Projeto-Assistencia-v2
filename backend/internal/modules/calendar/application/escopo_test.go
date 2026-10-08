package application_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/calendar/domain"
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

func temSala(rooms []domain.Room, id uuid.UUID) bool {
	return slices.ContainsFunc(rooms, func(r domain.Room) bool { return r.ID == id })
}

// Sala com unidade dona (ADR 013): calendar:manage gere as salas da
// unidade coberta e modera os eventos reservados nelas; sala e evento
// sem dona são institucionais (só a gestão global).
func TestUnidadeDonaEEscopo(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.real()
	un := dbtest.Unidade(t, e.pool)
	gestorUn := auth.Identity{UserID: dbtest.User(t, e.pool), Scopes: []auth.Scope{{Permissions: []string{"calendar:manage"}, UnidadeID: &un, Unidades: []uuid.UUID{un}}}}
	sala := func(who auth.Identity, unidade *uuid.UUID, ativa bool) (domain.Room, error) {
		return svc.SaveRoom(ctx, who, uuid.Nil, domain.Room{Name: "Sala " + uuid.NewString()[:8], Active: ativa, UnidadeID: unidade})
	}

	_, err := sala(gestorUn, nil, true)
	foraDoEscopo(t, err, "gestor da unidade cria sala institucional")
	daUn, err := sala(gestorUn, &un, true)
	if err != nil || daUn.UnidadeID == nil || *daUn.UnidadeID != un {
		t.Fatalf("gestor da unidade cria sala dela: %+v %v", daUn, err)
	}
	institucional := e.room()
	_, err = svc.SaveRoom(ctx, gestorUn, institucional.ID, domain.Room{Name: institucional.Name, Active: true, UnidadeID: &un})
	foraDoEscopo(t, err, "gestor da unidade toma sala institucional")
	_, err = svc.SaveRoom(ctx, gestorUn, daUn.ID, domain.Room{Name: daUn.Name, Active: true})
	foraDoEscopo(t, err, "gestor da unidade devolve sala ao institucional")

	// Salas inativas: só para a gestão que as cobre.
	inativaUn, err := sala(gestorUn, &un, false)
	if err != nil {
		t.Fatal(err)
	}
	inativaInst, err := sala(gestor, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	rooms, err := svc.Rooms(ctx, gestorUn)
	if err != nil || !temSala(rooms, inativaUn.ID) || temSala(rooms, inativaInst.ID) || !temSala(rooms, institucional.ID) {
		t.Fatalf("gestor da unidade vê as inativas dela e as ativas de todos: %v", err)
	}
	if rooms, _ := svc.Rooms(ctx, e.ana); temSala(rooms, inativaUn.ID) {
		t.Fatal("sem gestão, inativas não aparecem")
	}

	// Eventos: a gestão da unidade modera os das salas dela.
	in := e.input(&daUn.ID, 0)
	in.Visibility = "private"
	naSala, err := svc.CreateEvent(ctx, e.ana, in)
	if err != nil {
		t.Fatal(err)
	}
	in = e.input(nil, 2*time.Hour)
	in.Visibility = "private"
	semSala, err := svc.CreateEvent(ctx, e.ana, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Event(ctx, gestorUn, naSala.ID); err != nil {
		t.Fatalf("gestor da unidade vê o privado da sala dela: %v", err)
	}
	if _, err := svc.Event(ctx, gestorUn, semSala.ID); err == nil {
		t.Fatal("privado sem sala é institucional")
	}
	events, err := svc.Events(ctx, gestorUn, e.base.Add(-time.Hour), e.base.Add(4*time.Hour), nil)
	if err != nil || !slices.ContainsFunc(events, func(ev domain.Event) bool { return ev.ID == naSala.ID }) ||
		slices.ContainsFunc(events, func(ev domain.Event) bool { return ev.ID == semSala.ID }) {
		t.Fatalf("agenda do gestor da unidade: %v", err)
	}
	if _, err := svc.CancelEvent(ctx, gestorUn, naSala.ID); err != nil {
		t.Fatalf("gestor da unidade cancela evento da sala dela: %v", err)
	}
	if err := svc.DeleteEvent(ctx, gestorUn, semSala.ID); err == nil {
		t.Fatal("gestor da unidade não modera evento institucional")
	}

	// Excluir sala: a dona precisa estar no escopo.
	foraDoEscopo(t, svc.DeleteRoom(ctx, gestorUn, inativaInst.ID), "gestor da unidade exclui sala institucional")
	if err := svc.DeleteRoom(ctx, gestorUn, inativaUn.ID); err != nil {
		t.Fatalf("gestor da unidade exclui sala dela: %v", err)
	}
}
