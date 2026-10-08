package auth

import (
	"slices"

	"github.com/google/uuid"
)

// Publico é o público-alvo de um conteúdo (ADR 014): entidades
// (secretarias) e/ou unidades. Vazio = todos os autenticados.
type Publico struct {
	Entidades []uuid.UUID `json:"entidades"`
	Unidades  []uuid.UUID `json:"unidades"`
}

// Vazio reporta se o conteúdo é para todos.
func (p Publico) Vazio() bool { return len(p.Entidades) == 0 && len(p.Unidades) == 0 }

// Normalizado devolve o público sem nulos nem repetidos e com listas não
// nulas (para serializar como [] e comparar).
func (p Publico) Normalizado() Publico {
	limpa := func(ids []uuid.UUID) []uuid.UUID {
		out := make([]uuid.UUID, 0, len(ids))
		for _, id := range ids {
			if id != uuid.Nil && !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
		slices.SortFunc(out, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
		return out
	}
	return Publico{Entidades: limpa(p.Entidades), Unidades: limpa(p.Unidades)}
}

// Igual reporta se os dois públicos são o mesmo (ordem e repetições não contam).
func (p Publico) Igual(o Publico) bool {
	a, b := p.Normalizado(), o.Normalizado()
	return slices.Equal(a.Entidades, b.Entidades) && slices.Equal(a.Unidades, b.Unidades)
}

// Pertencimento é onde a pessoa pertence para o público-alvo: as entidades
// das lotações e as unidades delas com as de cima (Scope.Acima, calculado
// pelo resolvedor). Concessão só global não é pertencimento.
func Pertencimento(identity Identity) (entidades, unidades []uuid.UUID) {
	entidades, unidades = []uuid.UUID{}, []uuid.UUID{}
	for _, s := range identity.Scopes {
		if s.EntidadeID != nil && !slices.Contains(entidades, *s.EntidadeID) {
			entidades = append(entidades, *s.EntidadeID)
		}
		for _, u := range s.Acima {
			if !slices.Contains(unidades, u) {
				unidades = append(unidades, u)
			}
		}
	}
	return entidades, unidades
}

// NoPublico reporta se identity pertence ao público (vazio: todos).
func NoPublico(identity Identity, p Publico) bool {
	if p.Vazio() {
		return true
	}
	entidades, unidades := Pertencimento(identity)
	for _, e := range p.Entidades {
		if slices.Contains(entidades, e) {
			return true
		}
	}
	for _, u := range p.Unidades {
		if slices.Contains(unidades, u) {
			return true
		}
	}
	return false
}
