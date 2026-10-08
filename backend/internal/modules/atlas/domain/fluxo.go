package domain

import (
	"fmt"
	"strings"
)

// Fluxos documentais no assistente (ADR 023): além da TTDD, ele explica os
// procedimentos homologados do Atlas do início ao fim — etapas, setores,
// prazos, peças, condição para seguir e diligências.

// termosFluxo indicam pergunta sobre como um processo tramita.
var termosFluxo = []string{
	"fluxo", "tramit", "etapa", "procediment", "passo a passo", "setor", "peca", "andamento", "continuidade",
	"encaminh", "protocol", "assin", "como funciona", "o que precisa", "o que preciso", "documentos necessarios",
	"instruir", "instrucao", "quem aprova", "quem analisa", "proxima fase", "proximo passo",
}

// SobreFluxo informa se a pergunta trata de como um processo tramita.
func SobreFluxo(pergunta string) bool {
	texto := " " + Fold(pergunta)
	for _, t := range termosFluxo {
		if strings.Contains(texto, " "+t) {
			return true
		}
	}
	return false
}

// radicaisFluxo descrevem a pergunta sobre fluxo, não qual procedimento
// ("Como funciona o fluxo do pregão?" busca "pregão"). Só deixam de contar
// quando não aparecem no procedimento.
var radicaisFluxo = []string{
	"fluxo", "funcion", "tramit", "etapa", "passo", "andament", "procediment", "inicio", "fim", "continu", "precis",
	"necessari", "process", "document", "seguir", "fazer",
}

func vocabularioFluxo(termo string) bool {
	for _, r := range radicaisFluxo {
		if strings.HasPrefix(termo, r) {
			return true
		}
	}
	return vocabularioTTDD(termo)
}

// corpus é o texto pesquisável do procedimento completo.
func corpus(w Workflow) string {
	var b strings.Builder
	b.WriteString(w.Titulo + " " + w.Objetivo + " " + w.PublicoAlvo + " " + w.CodigoProcessual + " " + w.CodigoTTDD)
	if w.Classificacao != nil {
		b.WriteString(" " + w.Classificacao.Descritor)
	}
	for _, e := range w.Etapas {
		b.WriteString(" " + e.NomeSetor + " " + e.UnidadeAdministrativa + " " + e.AtribuicoesSetor)
		for _, d := range e.Documentos {
			b.WriteString(" " + d.NomeDocumento)
		}
	}
	return Fold(b.String())
}

// RelevanciaProcedimento mede (0..1) quanto o procedimento sustenta a
// pergunta, com a mesma regra das séries: fração dos termos presentes no
// procedimento (palavras de fluxo e de temporalidade que ele não contém não
// contam), bônus para o código exato e para termos do título.
func RelevanciaProcedimento(w Workflow, pergunta string) float64 {
	texto, titulo := corpus(w), Fold(w.Titulo)
	codigos := []string{Fold(w.CodigoProcessual), Fold(w.CodigoTTDD)}
	considerados, matched, bonus := 0, 0, 0.0
	for _, t := range Termos(pergunta) {
		casou := contem(texto, t)
		if !casou && vocabularioFluxo(t) {
			continue
		}
		considerados++
		if casou {
			matched++
		}
		if t == codigos[0] || t == codigos[1] {
			bonus += 0.3
		}
		if contem(titulo, t) {
			bonus += 0.15
		}
	}
	if matched == 0 {
		return 0
	}
	return min(1, float64(matched)/float64(considerados)+bonus)
}

func plural(n int, um, varios string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, um)
	}
	return fmt.Sprintf("%d %s", n, varios)
}

var formatos = map[FormatoDocumento]string{FormatoNatoDigital: "nato-digital", FormatoExternoDigitalizado: "externo digitalizado"}

var assinaturas = map[TipoAssinatura]string{
	AssinaturaIndividual: "individual", AssinaturaConjuntaMultinivel: "conjunta (multinível)", AssinaturaEmBloco: "em bloco",
}

// SinteseFluxo descreve o procedimento do início ao fim: em cada etapa, o
// setor, o prazo, o que fazer, as peças exigidas, a condição para seguir e
// as devoluções em diligência; depois, a temporalidade dos documentos.
func SinteseFluxo(w Workflow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s (versão %d)\n", w.CodigoProcessual, w.Titulo, w.Versao)
	fmt.Fprintf(&b, "Objetivo: %s\nPúblico: %s\n", w.Objetivo, w.PublicoAlvo)
	if w.NivelAcesso != NivelPublico {
		fmt.Fprintf(&b, "Nível de acesso do processo: %s (hipótese legal: %s)\n", w.NivelAcesso, w.HipoteseLegal)
	}
	total := 0
	setor := map[int]string{}
	for _, e := range w.Etapas {
		total += e.PrazoSLAEmDias
		setor[e.Ordem] = e.NomeSetor
	}
	fmt.Fprintf(&b, "Fluxo em %s, prazo previsto de %s:\n", plural(len(w.Etapas), "etapa", "etapas"), plural(total, "dia", "dias"))
	for i, e := range w.Etapas {
		fmt.Fprintf(&b, "\n%d. %s (%s) — prazo: %s\n   O que fazer: %s\n", e.Ordem, e.NomeSetor, e.UnidadeAdministrativa,
			plural(e.PrazoSLAEmDias, "dia", "dias"), e.AtribuicoesSetor)
		if len(e.Documentos) > 0 {
			b.WriteString("   Peças exigidas:\n")
			for _, d := range e.Documentos {
				req := "opcional"
				if d.Obrigatorio {
					req = "obrigatória"
				}
				fmt.Fprintf(&b, "   • %s — %s, %s, assinatura %s", d.NomeDocumento, req, formatos[d.Formato], assinaturas[d.TipoAssinatura])
				if d.ExigeConferenciaCopia {
					b.WriteString(", exige conferência da cópia")
				}
				if d.Modelo != nil {
					fmt.Fprintf(&b, "; modelo na biblioteca do Atlas: %s (versão %d)", d.Modelo.Nome, d.Modelo.Versao)
				}
				if d.ModeloMinutaPadraoURL != "" {
					b.WriteString("; modelo: " + d.ModeloMinutaPadraoURL)
				}
				b.WriteString("\n")
			}
		}
		// O caminho normal primeiro; as devoluções em diligência depois.
		for _, t := range e.Transicoes {
			if !t.IsDevolucaoDiligencia {
				fmt.Fprintf(&b, "   Para seguir: %s → etapa %d (%s)\n", t.CondicaoTransicao, t.DestinoOrdem, setor[t.DestinoOrdem])
			}
		}
		for _, t := range e.Transicoes {
			if t.IsDevolucaoDiligencia {
				fmt.Fprintf(&b, "   Devolução em diligência: se %s, volta à etapa %d (%s) — %s\n", t.CondicaoTransicao, t.DestinoOrdem,
					setor[t.DestinoOrdem], t.DescricaoDiligencia)
			}
		}
		if e.ManterAbertoAposRemessa {
			b.WriteString("   A unidade mantém o processo aberto para acompanhamento após a remessa.\n")
		}
		if i == len(w.Etapas)-1 {
			b.WriteString("   Fim do fluxo.\n")
		}
	}
	if c := w.Classificacao; c != nil {
		fmt.Fprintf(&b, "\nGuarda dos documentos (TTDD %s — %s): ", c.Codigo, c.Descritor)
		b.WriteString(strings.TrimPrefix(Temporalidade(*c), "Temporalidade: "))
	}
	return b.String()
}

// ComModelos acrescenta à síntese os modelos ativos da série para baixar
// (ADR 024); sem modelo ativo, a síntese fica como está.
func ComModelos(sintese, codigo string, modelos []Modelo) string {
	var nomes []string
	for _, m := range modelos {
		if m.Ativo {
			nomes = append(nomes, fmt.Sprintf("%s (versão %d)", m.Nome, m.Atual.Versao))
		}
	}
	if len(nomes) == 0 {
		return sintese
	}
	return strings.TrimRight(sintese, "\n") + "\nModelos de documento para baixar (biblioteca do Atlas, série " + codigo + "): " +
		strings.Join(nomes, "; ") + "\n"
}

// CasaModelo diz se todos os termos da busca aparecem (no início de uma
// palavra, sem acento) no nome ou na descrição do modelo.
func CasaModelo(m Modelo, busca string) bool {
	termos := Termos(busca)
	if len(termos) == 0 {
		return false
	}
	texto := Fold(m.Nome + " " + m.Descricao)
	for _, t := range termos {
		if !contem(texto, t) {
			return false
		}
	}
	return true
}
