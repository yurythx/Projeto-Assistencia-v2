package domain

import (
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

func TestCanAccess(t *testing.T) {
	me, other := uuid.New(), uuid.New()
	dep := uuid.New()
	identity := auth.Identity{UserID: me, Groups: []string{"/Nexus/Financeiro"}, Scopes: []auth.Scope{{DepartamentoID: &dep}}}

	cases := []struct {
		name string
		room Room
		want bool
	}{
		{"global", Room{Kind: "global"}, true},
		{"global arquivada", Room{Kind: "global", Archived: true}, false},
		{"departamental por grupo do AD", Room{Kind: "department", ADGroup: "financeiro"}, true},
		{"departamental por lotação", Room{Kind: "department", DepartamentoID: &dep}, true},
		{"departamental de outro grupo", Room{Kind: "department", ADGroup: "juridico"}, false},
		{"direta participante", Room{Kind: "direct", Members: []uuid.UUID{me, other}}, true},
		{"direta alheia", Room{Kind: "direct", Members: []uuid.UUID{other, uuid.New()}}, false},
	}
	for _, c := range cases {
		if got := CanAccess(identity, c.room); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	mod := auth.Identity{Scopes: []auth.Scope{{Perfil: "moderador", Permissions: []string{"mercurio:manage"}}}}
	if CanAccess(mod, Room{Kind: "direct", Members: []uuid.UUID{me, other}}) {
		t.Error("moderação não lê conversas diretas alheias")
	}
	if !CanAccess(mod, Room{Kind: "department", ADGroup: "juridico"}) {
		t.Error("moderação acessa salas departamentais")
	}
}

func TestDMKeyIsSymmetric(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	if DMKey(a, b) != DMKey(b, a) {
		t.Fatal("chave de sala direta deve independer da ordem")
	}
}

// mercurio:manage com escopo (ADR 013): modera as salas dos departamentos
// da unidade dele; global, direta e sala só por grupo do AD são
// institucionais.
func TestModeracaoComEscopo(t *testing.T) {
	uA, uB, depA, depB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	modA := auth.Identity{Scopes: []auth.Scope{{Perfil: "moderador", UnidadeID: &uA, Unidades: []uuid.UUID{uA}, Permissions: []string{"mercurio:manage"}}}}
	salaA := Room{Kind: "department", DepartamentoID: &depA, UnidadeID: &uA}
	salaB := Room{Kind: "department", DepartamentoID: &depB, UnidadeID: &uB}
	if !Modera(modA, salaA) || Modera(modA, salaB) || Modera(modA, Room{Kind: "global"}) || Modera(modA, Room{Kind: "department", ADGroup: "g"}) {
		t.Error("moderador de A: só as salas dos departamentos de A")
	}
	if !CanAccess(modA, Room{Kind: "department", DepartamentoID: &depA, UnidadeID: &uA, Archived: true}) || CanAccess(modA, Room{Kind: "department", DepartamentoID: &depB, UnidadeID: &uB, Archived: true}) {
		t.Error("arquivada: só a moderação que cobre a sala vê")
	}
	if p := salaA.Posicao(); p.DepartamentoID == nil || *p.UnidadeID != uA {
		t.Errorf("posição da sala de departamento: %+v", p)
	}
}
