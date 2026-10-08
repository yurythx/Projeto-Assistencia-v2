package auth

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

// Público-alvo (ADR 014): pertence quem tem lotação numa entidade do
// público ou numa unidade dele ou abaixo (Acima traz a unidade e as de
// cima); concessão só global não pertence; vazio é para todos.
func TestNoPublico(t *testing.T) {
	naFilha := Identity{Scopes: []Scope{{EntidadeID: ptr(idE), UnidadeID: ptr(idF), Acima: []uuid.UUID{idF, idU}}}}
	global := Identity{Scopes: []Scope{{Permissions: []string{"*"}}}}
	casos := []struct {
		nome string
		p    Publico
		quem Identity
		ve   bool
	}{
		{"vazio: todos", Publico{}, Identity{}, true},
		{"unidade acima da lotação", Publico{Unidades: []uuid.UUID{idU}}, naFilha, true},
		{"a própria unidade", Publico{Unidades: []uuid.UUID{idF}}, naFilha, true},
		{"entidade da lotação", Publico{Entidades: []uuid.UUID{idE}}, naFilha, true},
		{"unidade irmã", Publico{Unidades: []uuid.UUID{idV}}, naFilha, false},
		{"outra entidade", Publico{Entidades: []uuid.UUID{idX}}, naFilha, false},
		{"concessão global não pertence", Publico{Entidades: []uuid.UUID{idE}}, global, false},
	}
	for _, c := range casos {
		if got := NoPublico(c.quem, c.p); got != c.ve {
			t.Errorf("%s: %v", c.nome, got)
		}
	}
	dupla := Identity{Scopes: []Scope{
		{EntidadeID: ptr(idE), UnidadeID: ptr(idF), Acima: []uuid.UUID{idF, idU}},
		{EntidadeID: ptr(idE), UnidadeID: ptr(idU), Acima: []uuid.UUID{idU}},
	}}
	if e, u := Pertencimento(dupla); !slices.Equal(e, []uuid.UUID{idE}) || len(u) != 2 {
		t.Errorf("sem repetição: %v %v", e, u)
	}
	if e, u := Pertencimento(Identity{}); e == nil || u == nil {
		t.Error("listas vazias, não nulas")
	}
}

func TestPublicoNormalizado(t *testing.T) {
	p := Publico{Unidades: []uuid.UUID{idU, uuid.Nil, idU, idF}}.Normalizado()
	if len(p.Unidades) != 2 || p.Entidades == nil || p.Vazio() {
		t.Errorf("sem nulos nem repetidos, listas não nulas: %+v", p)
	}
	if !(Publico{Unidades: []uuid.UUID{idU, idF}}).Igual(Publico{Unidades: []uuid.UUID{idF, idU, idF}}) || (Publico{}).Igual(Publico{Entidades: []uuid.UUID{idE}}) {
		t.Error("igualdade ignora ordem e repetição")
	}
	if !(Publico{}).Normalizado().Vazio() {
		t.Error("vazio continua vazio")
	}
}
