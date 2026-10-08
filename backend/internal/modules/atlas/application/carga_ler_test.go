package application

import (
	"errors"
	"strings"
	"testing"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
)

const cabecalho = "Código;Série documental;Órgão;Função;Subfunção;Fase corrente;Fase intermediária;Destinação final;Observações;Recomendação da subfunção;Fonte\n"

const linhaPasta = "2.0.07.00.00;Pasta funcional;Secretaria de Administração;2.0.07 Vida Funcional;2.0.07.00 Movimentação;1 ano;99 anos;eliminação;;Transferir;" +
	"TTDD da Secretaria de Administração, versão II, Diário Oficial nº 6.017 de 25/08/2025\n"

func TestLerCSV(t *testing.T) {
	bom := string([]byte{0xEF, 0xBB, 0xBF})
	csv := bom + cabecalho + linhaPasta +
		"2.0.07.00.01;Ficha;Secretaria de Administração;2.0.07 Vida Funcional;2.0.07.00 Movimentação;Enquanto estiver vigorando;não há;não definida na TTDD;obs;Transferir;\n" +
		"2.0.07.00.02;Livro;Secretaria de Administração;2.0.07 Vida Funcional;2.0.07.00 Movimentação;não informado na TTDD;Até a prescrição;guarda permanente;;Transferir;\n" +
		";;;;;;;;;;\n"
	c, err := LerCarga(FormatoCSV, csv)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Orgaos) != 1 || c.Orgaos[0].Prefixo != "2.0" || c.Orgaos[0].Versao != "II" || c.Orgaos[0].EdicaoDiario != "6.017" ||
		c.Orgaos[0].DataPublicacao.Format("2006-01-02") != "2025-08-25" || len(c.Funcoes) != 1 || c.Funcoes[0].Nome != "Vida Funcional" ||
		len(c.Subfuncoes) != 1 || c.Subfuncoes[0].Recomendacao != "Transferir" || len(c.Series) != 3 {
		t.Fatalf("carga: %+v", c)
	}
	p, f, l := c.Series[0], c.Series[1], c.Series[2]
	if *p.FaseCorrenteAnos != 1 || *p.FaseIntermAnos != 99 || *p.DestinacaoFinal != domain.DestinacaoEliminacao || p.Linha != 2 ||
		f.FaseCorrenteCondicao != "Enquanto estiver vigorando" || f.FaseIntermAnos != nil || f.FaseIntermCondicao != "" || f.DestinacaoFinal != nil ||
		l.FaseCorrenteAnos != nil || l.FaseCorrenteCondicao != "" || l.FaseIntermCondicao != "Até a prescrição" || *l.DestinacaoFinal != domain.DestinacaoGuardaPermanente {
		t.Fatalf("séries: %+v %+v %+v", p, f, l)
	}
	// Vírgula como separador (planilha salva fora do Excel em português).
	virgula := strings.ReplaceAll(cabecalho, ";", ",") + strings.ReplaceAll(strings.ReplaceAll(linhaPasta, ",", ""), ";", ",")
	if c, err := LerCarga(FormatoCSV, virgula); err != nil || len(c.Series) != 1 {
		t.Fatalf("separador vírgula: %v", err)
	}
}

func TestLerCSVFalhas(t *testing.T) {
	for nome, csv := range map[string]string{
		"cabeçalho":  "Código;Outra\n",
		"vazio":      "",
		"aspas":      cabecalho + "\"aberta;x\n",
		"colunas":    cabecalho + "2.0.07.00.00;x\n",
		"destinação": cabecalho + strings.Replace(linhaPasta, "eliminação", "queimar", 1),
		"data":       cabecalho + strings.Replace(linhaPasta, "25/08/2025", "31/02/2025", 1),
		"validação":  cabecalho + strings.Replace(linhaPasta, "2.0.07.00.00", "2.0.07", 1),
	} {
		var inv domain.InvalidError
		if _, err := LerCarga(FormatoCSV, csv); !errors.As(err, &inv) {
			t.Errorf("%s: %v", nome, err)
		}
	}
	if _, err := LerCarga("xlsx", "x"); err == nil {
		t.Fatal("formato desconhecido")
	}
}

func TestLerJSON(t *testing.T) {
	json := `{"orgaos":{"2.0":{"nome":"Administração","edicao":"6.017","data":"2025-08-25","versao":"II"}},
		"funcoes":{"2.0.07":"Vida Funcional"},"subfuncoes":{"2.0.07.00":{"nome":"Movimentação","recomendacao":"Transferir"}},
		"itens":[{"codigo":"2.0.07.00.00","subfuncao":"2.0.07.00","descritor":"Pasta","corrente_anos":1,"corrente_condicao":"",
			"intermediaria_anos":99,"intermediaria_condicao":"","destinacao":"ELIMINACAO","observacoes":""},
			{"codigo":"2.0.07.00.01","subfuncao":"2.0.07.00","descritor":"Ficha","corrente_anos":null,"corrente_condicao":"Enquanto estiver vigorando",
			"intermediaria_anos":null,"intermediaria_condicao":"","destinacao":null,"observacoes":"x"}],"divergencias":[]}`
	c, err := LerCarga(FormatoJSON, json)
	if err != nil || len(c.Series) != 2 || *c.Series[0].DestinacaoFinal != domain.DestinacaoEliminacao || c.Series[1].DestinacaoFinal != nil ||
		c.Orgaos[0].EdicaoDiario != "6.017" || c.Subfuncoes[0].Recomendacao != "Transferir" {
		t.Fatalf("json: %+v %v", c, err)
	}
	if _, err := LerCarga(FormatoJSON, "{"); err == nil {
		t.Fatal("JSON inválido")
	}
	if _, err := LerCarga(FormatoJSON, `{"orgaos":{"2.0":{"nome":"x","data":"amanhã"}}}`); err == nil {
		t.Fatal("data inválida")
	}
	if len(HashCarga("a")) != 64 || HashCarga("a") == HashCarga("b") {
		t.Fatal("hash")
	}
}
