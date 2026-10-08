package domain

import (
	"strings"
	"testing"
	"time"
)

func TestSituacaoEValidacao(t *testing.T) {
	for _, s := range []string{SituacaoRascunho, SituacaoEmValidacao, SituacaoHomologado} {
		if !SituacaoValida(s) {
			t.Errorf("%s é válida", s)
		}
	}
	if SituacaoValida("PUBLICADO") {
		t.Fatal("situação inexistente aceita")
	}
	hoje := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	v := Validacao{RealizadaEm: hoje.AddDate(0, 0, -1), Unidade: " Licitações ", Participantes: " Ana ", Registro: " Fluxo confere ", Pendencias: " "}
	v.Normalizar()
	if v.Unidade != "Licitações" || v.Pendencias != "" || v.Validar(hoje) != nil {
		t.Fatalf("válida: %+v %v", v, v.Validar(hoje))
	}
	for name, x := range map[string]Validacao{
		"sem data":          {Unidade: "u", Registro: "r"},
		"data futura":       {RealizadaEm: hoje.AddDate(0, 0, 1), Unidade: "u", Registro: "r"},
		"sem unidade":       {RealizadaEm: hoje, Registro: "r"},
		"participantes":     {RealizadaEm: hoje, Unidade: "u", Registro: "r", Participantes: strings.Repeat("a", 1001)},
		"sem registro":      {RealizadaEm: hoje, Unidade: "u"},
		"pendências longas": {RealizadaEm: hoje, Unidade: "u", Registro: "r", Pendencias: strings.Repeat("a", 5001)},
	} {
		if x.Validar(hoje) == nil {
			t.Errorf("%s aceito", name)
		}
	}
}
