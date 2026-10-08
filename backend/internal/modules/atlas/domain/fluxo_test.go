package domain

import (
	"strings"
	"testing"
)

func TestSobreFluxo(t *testing.T) {
	for _, p := range []string{"como funciona o fluxo de licitação?", "quais as etapas do pregão", "como tramitar o pedido de diárias",
		"o que preciso para dar continuidade ao processo", "quem assina o termo de referência"} {
		if !SobreFluxo(p) {
			t.Errorf("%q é sobre fluxo", p)
		}
	}
	for _, p := range []string{"qual o prazo de guarda da pasta funcional", "receita de bolo"} {
		if SobreFluxo(p) {
			t.Errorf("%q não é sobre fluxo", p)
		}
	}
}

func TestRelevanciaProcedimento(t *testing.T) {
	w := validWorkflow()
	w.Normalize()
	w.Classificacao = &ClassificacaoTTDD{Codigo: "2.0.02.00.07", Descritor: "Processos de pregão"}
	for _, p := range []string{"Como funciona o fluxo do pregão eletrônico?", "ADM.LIC.001", "quais documentos do pregão eletrônico?",
		"o que preciso para dar continuidade ao pregão eletrônico do início ao fim"} {
		if r := RelevanciaProcedimento(w, p); r < LimiarRelevancia {
			t.Errorf("%q: %.2f abaixo do limiar", p, r)
		}
	}
	if r := RelevanciaProcedimento(w, "licença para viagem internacional"); r >= LimiarRelevancia {
		t.Fatalf("pergunta sem relação acima do limiar: %.2f", r)
	}
	if RelevanciaProcedimento(w, "") != 0 || RelevanciaProcedimento(w, "como funciona o fluxo") != 0 {
		t.Fatal("sem termo do procedimento, relevância zero")
	}
	if RelevanciaProcedimento(w, "2.0.02.00.07") < LimiarRelevancia {
		t.Fatal("código da TTDD do procedimento")
	}
}

func TestSinteseFluxo(t *testing.T) {
	w := validWorkflow()
	w.Normalize()
	w.Versao, w.NivelAcesso, w.HipoteseLegal = 2, NivelRestrito, "LAI, art. 31"
	w.Etapas[0].PrazoSLAEmDias, w.Etapas[1].PrazoSLAEmDias = 1, 10
	w.Etapas[0].ManterAbertoAposRemessa = true
	w.Etapas[0].Documentos = append(w.Etapas[0].Documentos, EtapaDocumento{NomeDocumento: "Cópia do RG", Formato: FormatoExternoDigitalizado,
		TipoAssinatura: AssinaturaEmBloco, ExigeConferenciaCopia: true, ModeloMinutaPadraoURL: "https://x/modelo.docx",
		Modelo: &ModeloResumo{Nome: "Requerimento padrão", Versao: 2}})
	um := 1
	w.Classificacao = &ClassificacaoTTDD{Codigo: "2.0.02.00.07", Descritor: "Pregão", FaseCorrenteAnos: &um, FaseIntermAnos: &um,
		DestinacaoFinal: ptr(DestinacaoGuardaPermanente)}
	s := SinteseFluxo(w)
	for _, want := range []string{
		"ADM.LIC.001 — Pregão Eletrônico (versão 2)", "Nível de acesso do processo: RESTRITO (hipótese legal: LAI, art. 31)",
		"Fluxo em 2 etapas, prazo previsto de 11 dias:",
		"1. Demandante (SEC/DEM) — prazo: 1 dia", "O que fazer: Elaborar o DFD",
		"• DFD — obrigatória, nato-digital, assinatura individual",
		"• Cópia do RG — opcional, externo digitalizado, assinatura em bloco, exige conferência da cópia; modelo na biblioteca do Atlas: Requerimento padrão (versão 2); modelo: https://x/modelo.docx",
		"Para seguir: DFD aprovado → etapa 2 (Licitações)",
		"mantém o processo aberto",
		"2. Licitações (SEMAD/LIC) — prazo: 10 dias",
		"Devolução em diligência: se Pendência, volta à etapa 1 (Demandante) — Completar o DFD",
		"Fim do fluxo.",
		"Guarda dos documentos (TTDD 2.0.02.00.07 — Pregão): fase corrente 1 ano; fase intermediária 1 ano; destinação final: guarda permanente",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("síntese sem %q:\n%s", want, s)
		}
	}
	// Público, sem classificação e com uma etapa só.
	p := validWorkflow()
	p.Etapas = p.Etapas[:1]
	if s := SinteseFluxo(p); strings.Contains(s, "Nível de acesso") || strings.Contains(s, "Guarda dos documentos") ||
		!strings.Contains(s, "Fluxo em 1 etapa") {
		t.Fatalf("procedimento público sem classificação:\n%s", s)
	}
}

// O caminho normal vem antes das devoluções em diligência, mesmo cadastrado depois.
func TestSinteseFluxoCaminhoNormalPrimeiro(t *testing.T) {
	w := validWorkflow()
	w.Etapas[1].Transicoes = append(w.Etapas[1].Transicoes, EtapaTransicao{DestinoOrdem: 1, CondicaoTransicao: "Homologado"})
	s := SinteseFluxo(w)
	normal, dilig := strings.Index(s, "Para seguir: Homologado"), strings.Index(s, "Devolução em diligência: se Pendência")
	if normal < 0 || dilig < 0 || normal > dilig {
		t.Fatalf("ordem das transições:\n%s", s)
	}
}
