package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Validação dos procedimentos (ADR 028): rascunho → em validação →
// homologado. Só o homologado é publicado (ativo).
const (
	SituacaoRascunho    = "RASCUNHO"
	SituacaoEmValidacao = "EM_VALIDACAO"
	SituacaoHomologado  = "HOMOLOGADO"
)

// SituacaoValida diz se s é uma das situações.
func SituacaoValida(s string) bool {
	return s == SituacaoRascunho || s == SituacaoEmValidacao || s == SituacaoHomologado
}

// Validacao é o registro de uma entrevista ou reunião de validação do
// fluxo num departamento.
type Validacao struct {
	ID               uuid.UUID `json:"id"`
	CodigoProcessual string    `json:"codigo_processual"`
	RealizadaEm      time.Time `json:"realizada_em"`
	Unidade          string    `json:"unidade"`
	Participantes    string    `json:"participantes"`
	Registro         string    `json:"registro"`
	Pendencias       string    `json:"pendencias"`
	CreatedAt        time.Time `json:"created_at"`
	CreatedBy        string    `json:"created_by"`
}

// Normalizar apara os textos.
func (v *Validacao) Normalizar() {
	v.Unidade, v.Participantes = strings.TrimSpace(v.Unidade), strings.TrimSpace(v.Participantes)
	v.Registro, v.Pendencias = strings.TrimSpace(v.Registro), strings.TrimSpace(v.Pendencias)
}

// Validar confere o registro (chame Normalizar antes).
func (v Validacao) Validar(hoje time.Time) error {
	switch {
	case v.RealizadaEm.IsZero() || v.RealizadaEm.After(hoje):
		return invalid("data da entrevista obrigatória e não pode ser futura")
	case v.Unidade == "" || utf8.RuneCountInString(v.Unidade) > 200:
		return invalid("departamento entrevistado obrigatório (até 200 caracteres)")
	case utf8.RuneCountInString(v.Participantes) > 1000:
		return invalid("participantes acima de 1.000 caracteres")
	case v.Registro == "" || utf8.RuneCountInString(v.Registro) > 10000:
		return invalid("registro da entrevista obrigatório (até 10.000 caracteres)")
	case utf8.RuneCountInString(v.Pendencias) > 5000:
		return invalid("pendências acima de 5.000 caracteres")
	}
	return nil
}
