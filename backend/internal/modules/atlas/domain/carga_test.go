package domain

import (
	"errors"
	"strings"
	"testing"
)

func cargaValida() CargaTTDD {
	um, gp := 1, DestinacaoGuardaPermanente
	return CargaTTDD{
		Orgaos:     []OrgaoTTDD{{Prefixo: "2.0", Nome: "Administração"}},
		Funcoes:    []CargaFuncao{{Codigo: "2.0.01", Nome: "Gestão"}},
		Subfuncoes: []CargaSubfuncao{{Codigo: "2.0.01.00", Nome: "Geral"}},
		Series: []CargaSerie{
			{Codigo: "2.0.01.00.01", Subfuncao: "2.0.01.00", PrazosTTDD: PrazosTTDD{Descritor: "Organogramas", FaseCorrenteAnos: &um, DestinacaoFinal: &gp}},
			{Codigo: "2.0.01.00.01-2", Subfuncao: "2.0.01.00", Linha: 3, PrazosTTDD: PrazosTTDD{Descritor: "Repetido no documento",
				FaseCorrenteCondicao: "Enquanto estiver vigorando"}},
		},
	}
}

func TestCargaValidar(t *testing.T) {
	if err := cargaValida().Validar(); err != nil {
		t.Fatal(err)
	}
	mil, um := 1000, 1
	dest := DestinacaoFinal("X")
	for nome, c := range map[string]struct {
		mut  func(*CargaTTDD)
		want string
	}{
		"vazia":           {func(c *CargaTTDD) { c.Series = nil }, "nenhuma série"},
		"órgão inválido":  {func(c *CargaTTDD) { c.Orgaos[0].Prefixo = "2" }, "órgão com código inválido"},
		"órgão sem nome":  {func(c *CargaTTDD) { c.Orgaos[0].Nome = " " }, "sem nome"},
		"órgão repetido":  {func(c *CargaTTDD) { c.Orgaos = append(c.Orgaos, c.Orgaos[0]) }, "repetido"},
		"função":          {func(c *CargaTTDD) { c.Funcoes[0].Codigo = "3.0.01" }, "função inválida"},
		"subfunção":       {func(c *CargaTTDD) { c.Subfuncoes[0].Nome = "" }, "subfunção inválida"},
		"código da série": {func(c *CargaTTDD) { c.Series[0].Codigo = "2.0.01" }, "código de série inválido"},
		"série repetida":  {func(c *CargaTTDD) { c.Series[1].Codigo = c.Series[0].Codigo }, "linha 3: série 2.0.01.00.01 repetida"},
		"outro ramo":      {func(c *CargaTTDD) { c.Series[0].Subfuncao = "2.0.01.01" }, "subfunção"},
		"sem descritor":   {func(c *CargaTTDD) { c.Series[0].Descritor = "" }, "sem descritor"},
		"destinação":      {func(c *CargaTTDD) { c.Series[0].DestinacaoFinal = &dest }, "destinação inválida"},
		"anos demais":     {func(c *CargaTTDD) { c.Series[0].FaseIntermAnos = &mil }, "fase intermediária"},
		"anos e condição": {func(c *CargaTTDD) { c.Series[1].FaseCorrenteAnos = &um }, "fase corrente"},
	} {
		carga := cargaValida()
		c.mut(&carga)
		var inv InvalidError
		if err := carga.Validar(); !errors.As(err, &inv) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", nome, err)
		}
	}
	// Muitas falhas: só as primeiras MaxErrosCarga, com a contagem do resto.
	muitas := cargaValida()
	for range 30 {
		muitas.Series = append(muitas.Series, CargaSerie{Codigo: "x"})
	}
	if err := muitas.Validar(); err == nil || !strings.Contains(err.Error(), "(e mais") {
		t.Fatalf("muitas falhas: %v", err)
	}
}

func TestDataPublicacao(t *testing.T) {
	for in, want := range map[string]string{"2025-08-25": "2025-08-25", "25/08/2025": "2025-08-25"} {
		if d, err := DataPublicacao(in); err != nil || d.Format("2006-01-02") != want {
			t.Errorf("%s: %v %v", in, d, err)
		}
	}
	if d, err := DataPublicacao(" "); d != nil || err != nil {
		t.Fatal("vazia = sem data")
	}
	if _, err := DataPublicacao("ontem"); err == nil {
		t.Fatal("data inválida")
	}
	if pai("2.0.01.00.05-2", 4) != "2.0.01.00" || pai("2.0", 5) != "2.0" {
		t.Fatal("pai")
	}
}
