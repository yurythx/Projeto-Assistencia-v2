package domain

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// LimiarRelevancia é a relevância mínima (0..1) para o assistente
// responder. Abaixo dela a resposta é uma das recusas canônicas — nada de
// orientação inventada sem procedimento homologado ou série da TTDD que a
// sustente.
const LimiarRelevancia = 0.65

// O assistente responde SÓ sobre o acervo documental do Atlas — os fluxos
// homologados e a TTDD oficial (ADRs 021 e 023). Duas recusas:
const (
	// MensagemForaDoObjetivo: a pergunta não é sobre fluxos nem sobre
	// temporalidade de documentos.
	MensagemForaDoObjetivo = "Esse assunto foge do objetivo da IA: este assistente responde apenas sobre os fluxos " +
		"documentais do Atlas (como cada processo tramita, do início ao fim) e a Tabela de Temporalidade e Destinação de " +
		"Documentos (TTDD) — prazos de guarda e destinação final."
	// MensagemSemFonte: é sobre fluxo ou temporalidade, mas nenhum
	// procedimento homologado nem série da TTDD corresponde à pergunta.
	MensagemSemFonte = "Não localizei um fluxo homologado nem uma série da TTDD que corresponda à sua consulta. " +
		"Verifique o nome do processo ou do documento, ou consulte a unidade de gestão documental."
)

// termosTemporalidade indicam que a pergunta é sobre a TTDD mesmo quando
// nenhuma série casa ("qual o prazo de guarda do documento X?").
var termosTemporalidade = []string{
	"ttdd", "temporalidade", "prazo", "guarda", "guardar", "arquiv", "destinacao", "eliminac", "eliminar",
	"descart", "permanente", "intermediaria", "serie documental", "classificac", "ccpad", "recolhimento", "conservar",
}

// SobreTemporalidade informa se a pergunta trata de temporalidade de
// documentos (decide entre as duas recusas).
func SobreTemporalidade(pergunta string) bool {
	texto := " " + Fold(pergunta)
	for _, t := range termosTemporalidade {
		if strings.Contains(texto, " "+t) {
			return true
		}
	}
	return false
}

// termosForaDoObjetivo indicam pedido de TAREFA (redigir, resumir,
// traduzir…) — fora do objetivo, mesmo que palavras casem com uma série ou
// um procedimento ("Me ajuda a escrever um ofício?" x "Livro de registro de
// ofícios"). Só verbos inequívocos: "elaboração", "revisão", "receita",
// "conselho" aparecem em nomes de séries e órgãos da TTDD.
var termosForaDoObjetivo = []string{
	"escrev", "redig", "redacao", "resum", "traduz", "me ajud", "ajude", "poema", "piada", "opiniao",
}

// PedidoForaDoObjetivo informa se a pergunta pede uma tarefa (e não fala de
// fluxo nem de temporalidade): recusa direta. descritor é o nome da fonte
// mais relevante ("" se nenhuma): um termo que faz parte dele não conta
// como pedido (9 séries têm "resumo" no nome: "Relatório Resumido de
// Execução Orçamentária").
func PedidoForaDoObjetivo(pergunta, descritor string) bool {
	if SobreTemporalidade(pergunta) || SobreFluxo(pergunta) {
		return false
	}
	texto, nome := " "+Fold(pergunta), " "+Fold(descritor)
	for _, t := range termosForaDoObjetivo {
		if strings.Contains(texto, " "+t) && !strings.Contains(nome, " "+t) {
			return true
		}
	}
	return false
}

// ForaDoObjetivo reconhece a recusa por assunto na resposta do modelo
// (instruído a devolvê-la literalmente).
func ForaDoObjetivo(resposta string) bool {
	return strings.Contains(resposta, "foge do objetivo da IA")
}

// stopwords: palavras sem conteúdo que não contam na cobertura da pergunta
// ("Como tramitar UM processo DE pregão?" mede só tramitar/processo/pregão).
var stopwords = map[string]bool{
	"a": true, "o": true, "as": true, "os": true, "um": true, "uma": true, "uns": true, "umas": true,
	"de": true, "da": true, "do": true, "das": true, "dos": true, "em": true, "na": true, "no": true,
	"nas": true, "nos": true, "por": true, "para": true, "pra": true, "com": true, "sem": true, "e": true,
	"ou": true, "que": true, "qual": true, "quais": true, "como": true, "onde": true, "quando": true,
	"se": true, "ao": true, "aos": true, "sao": true, "ser": true, "sobre": true, "meu": true,
	"minha": true, "seu": true, "sua": true, "eu": true, "voce": true, "posso": true, "devo": true,
	"preciso": true, "fazer": true, "isso": true, "este": true, "esta": true, "esse": true, "essa": true,
	"pelo": true, "pela": true, "entre": true, "mais": true, "tem": true, "ter": true, "ha": true,
}

// Fold devolve o texto em minúsculas e sem acentos.
func Fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Termos extrai os termos de conteúdo da pergunta (sem acento, sem
// stopwords, com 2+ caracteres, sem repetição).
func Termos(pergunta string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range strings.FieldsFunc(Fold(pergunta), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.'
	}) {
		t = strings.Trim(t, ".")
		if len([]rune(t)) < 2 || stopwords[t] || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// contem casa o termo no INÍCIO de uma palavra — inteiro ou, a partir de 6
// letras, pelo radical sem as duas últimas (plural e gênero: "documentos" ~
// "documento", "licitacoes" ~ "licitacao"). No meio da palavra não vale:
// "licitação" não casa com "solicitação".
func contem(texto, termo string) bool {
	t := palavras(texto)
	if strings.Contains(t, " "+termo) {
		return true
	}
	r := []rune(termo)
	return len(r) >= 6 && strings.Contains(t, " "+string(r[:len(r)-2]))
}

// palavras separa o texto em palavras por espaço (pontuação vira espaço; o
// ponto fica, para os códigos "2.0.01.00.05").
func palavras(texto string) string {
	return " " + strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' {
			return r
		}
		return ' '
	}, texto)
}

// Temporalidade descreve prazos, destinação, recomendação e fonte de uma
// série da TTDD (uma linha por informação).
func Temporalidade(c ClassificacaoTTDD) string {
	var b strings.Builder
	if !c.Vigente() {
		fmt.Fprintf(&b, "ATENÇÃO: série revogada em %s%s — não está na TTDD em vigor; confirme a classificação com a gestão documental.\n",
			c.RevogadaEm.Format("02/01/2006"), edicaoDiario(c.RevogadaEdicao))
	}
	fmt.Fprintf(&b, "Temporalidade: fase corrente %s; fase intermediária %s; destinação final: %s.\n",
		Fase(c.FaseCorrenteAnos, c.FaseCorrenteCondicao, true), Fase(c.FaseIntermAnos, c.FaseIntermCondicao, false), c.Destinacao())
	if total, ok := c.PrazoTotalAnos(); ok {
		fmt.Fprintf(&b, "Prazo total de guarda antes da destinação: %d ano(s).\n", total)
	}
	if c.Observacoes != "" {
		fmt.Fprintf(&b, "Observações: %s\n", c.Observacoes)
	}
	if s := c.Subfuncao; s != nil {
		if s.Recomendacao != "" {
			fmt.Fprintf(&b, "Recomendação da subfunção: %s\n", s.Recomendacao)
		}
		fmt.Fprintf(&b, "Fonte: %s.\n", Fonte(s.Funcao.Orgao))
	}
	return b.String()
}

// SinteseTTDD responde com os dados oficiais de uma série documental.
func SinteseTTDD(c ClassificacaoTTDD) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", c.Codigo, c.Descritor)
	if s := c.Subfuncao; s != nil {
		fmt.Fprintf(&b, "Classificação: %s › %s › %s\n", s.Funcao.Orgao.Nome, s.Funcao.Nome, s.Nome)
	}
	b.WriteString(Temporalidade(c))
	return b.String()
}

// RelevanciaTTDD mede (0..1) quanto uma série da TTDD sustenta a pergunta:
// fração dos termos presentes no descritor (e na função/subfunção), bônus
// para o código exato e para termos do próprio descritor. Palavras de
// temporalidade que a série não contém não entram na conta. Determinística e
// auditável — o limiar não depende do modelo de linguagem.
func RelevanciaTTDD(c ClassificacaoTTDD, pergunta string) float64 {
	termos := Termos(pergunta)
	if len(termos) == 0 {
		return 0
	}
	descritor := Fold(c.Descritor)
	texto := descritor + " " + Fold(c.Codigo) + " " + Fold(c.Observacoes)
	if s := c.Subfuncao; s != nil {
		texto += " " + Fold(s.Nome+" "+s.Funcao.Nome)
	}
	considerados, matched, bonus := 0, 0, 0.0
	for _, t := range termos {
		casou := contem(texto, t)
		if !casou && vocabularioTTDD(t) {
			continue // "prazo", "destinação": falam da TTDD, não de qual série
		}
		considerados++
		if casou {
			matched++
		}
		if t == Fold(c.Codigo) {
			bonus += 0.3
		}
		if contem(descritor, t) {
			bonus += 0.15
		}
	}
	if matched == 0 {
		return 0
	}
	return min(1, float64(matched)/float64(considerados)+bonus)
}

// radicaisTTDD são palavras de temporalidade em si — da pergunta sobre a
// TTDD, não do nome da série ("Qual a destinação final dos organogramas?"
// busca "organogramas"). Só deixam de contar quando não aparecem na série.
var radicaisTTDD = []string{
	"prazo", "guard", "destin", "final", "elimin", "descart", "permanent", "corrent", "intermediar", "fase", "ttdd",
	"temporalid", "tabela", "serie", "documental", "classifica", "quanto", "tempo", "conserv", "manter", "ano",
}

func vocabularioTTDD(termo string) bool {
	for _, r := range radicaisTTDD {
		if strings.HasPrefix(termo, r) {
			return true
		}
	}
	return false
}

// Fonte descreve a publicação da TTDD do órgão ("TTDD da Secretaria X,
// versão II, Diário Oficial nº 6.275 de 11/09/2026").
func Fonte(o OrgaoTTDD) string {
	fonte := "TTDD da " + o.Nome
	if o.Versao != "" {
		fonte += ", versão " + o.Versao
	}
	if o.EdicaoDiario != "" {
		fonte += ", Diário Oficial nº " + o.EdicaoDiario
		if o.DataPublicacao != nil {
			fonte += " de " + o.DataPublicacao.Format("02/01/2006")
		}
	}
	return fonte
}

// edicaoDiario descreve a edição do Diário Oficial (vazia = não informada).
func edicaoDiario(e string) string {
	if e == "" {
		return ""
	}
	return " (Diário Oficial nº " + e + ")"
}
