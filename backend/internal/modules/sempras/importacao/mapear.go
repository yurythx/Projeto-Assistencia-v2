package importacao

import (
	"strings"
	"unicode"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
)

var semAcento = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ù", "u", "û", "u",
	"ü", "u", "ç", "c", "ñ", "n")

// chave reduz um rótulo das telas do 4.0 à forma de comparação: minúsculo,
// sem acento, só letras e dígitos separados por um espaço.
func chave(s string) string {
	s = semAcento.Replace(strings.ToLower(s))
	var b strings.Builder
	espaco := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if espaco && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			espaco = false
			continue
		}
		espaco = true
	}
	return b.String()
}

// naoInformado: o que o 4.0 grava quando o campo ficou em branco.
func naoInformado(s string) bool {
	switch chave(s) {
	case "", "n i", "nao informado", "selecione", "selecione a opcao":
		return true
	}
	return false
}

// rotulos: rótulo das telas do 4.0 (já em chave()) → valor da lista daqui.
type rotulos map[string]string

// valor traduz o rótulo; ok=false quando ele não é de nenhuma opção.
func (r rotulos) valor(rotulo string) (string, bool) {
	if naoInformado(rotulo) {
		return "", true
	}
	v, ok := r[chave(rotulo)]
	return v, ok
}

var (
	rotSexo        = rotulos{"masculino": "masculino", "feminino": "feminino", "m": "masculino", "f": "feminino"}
	rotGenero      = rotulos{"cisgenero": "cisgenero", "transgenero": "transgenero", "nao binario": "nao_binario", "outro": "outro"}
	rotOrientacao  = rotulos{"heterossexual": "heterossexual", "homossexual": "homossexual", "bissexual": "bissexual", "pansexual": "pansexual", "outro": "outro"}
	rotRacaCor     = rotulos{"branca": "branca", "preta": "preta", "parda": "parda", "amarela": "amarela", "indigena": "indigena", "quilombola": "quilombola"}
	rotSimNao      = rotulos{"sim": "sim", "nao": "nao", "true": "sim", "false": "nao"}
	rotSimNaoSei   = rotulos{"sim": "sim", "nao": "nao", "nao sei": "nao_sei"}
	rotTrabalho    = rotulos{"jovem aprendiz": "jovem_aprendiz", "meu primeiro trampo": "meu_primeiro_trampo", "trabalho formal": "formal", "trabalho informal": "informal", "nao": "nao"}
	rotPeriodo     = rotulos{"matutino": "matutino", "manha": "matutino", "vespertino": "vespertino", "tarde": "vespertino", "noturno": "noturno", "noite": "noturno", "integral": "integral"}
	rotStatusRel   = rotulos{"em elaboracao": "em_elaboracao", "revisao": "revisao", "aprovado": "aprovado", "entregue": "entregue"}
	rotStatusTaref = rotulos{"backlog": "backlog", "todo": "todo", "doing": "doing", "review": "review", "done": "done"}
	rotPrioridade  = rotulos{"baixa": "baixa", "media": "media", "alta": "alta", "critica": "critica"}
	rotTermo       = rotulos{"advertencia": "advertencia", "passeio": "passeio", "saida": "saida"}
	rotVisita      = rotulos{"acolhido": "acolhido", "instituicao": "instituicao", "doacao": "doacao", "voluntariado": "voluntariado"}
	rotSituacao    = rotulos{
		"acolhimento por situacao emergencial pelo conselho tutelar": "emergencial_conselho_tutelar",
		"em processo de adocao internacional":                        "adocao_internacional",
		"em processo de adocao nacional":                             "adocao_nacional",
		"em processo de guarda":                                      "processo_guarda",
		"em processo de tutela":                                      "processo_tutela",
		"poder familiar destituido":                                  "poder_familiar_destituido",
		"poder familiar suspenso":                                    "poder_familiar_suspenso",
		"preparacao para reintegracao familiar familia extensa":      "reintegracao_familia_extensa",
		"preparacao para reintegracao familiar familia natural":      "reintegracao_familia_natural",
		"sem previsao de finalizacao da medida protetiva":            "sem_previsao",
	}
	rotRelatorio = rotulos{
		"relatorio para 5 dias": "cinco_dias", "relatorio para 7 dias": "sete_dias", "relatorio quinzenal": "quinzenal",
		"relatorio mensal": "mensal", "relatorio bimestral": "bimestral", "relatorio trimestral": "trimestral",
		"relatorio anual": "anual", "relatorio psicossocial": "psicossocial", "relatorio circunstanciado": "circunstanciado",
		"relatorio informativo": "informativo",
	}
	rotMotivo = rotulos{
		"abandono":                                         "abandono",
		"abuso sexual":                                     "abuso_sexual",
		"ameaca de morte dos pais responsaveis":            "ameaca_morte_responsaveis",
		"ausencia dos pais responsaveis por doenca":        "ausencia_responsaveis_doenca",
		"ausencia dos pais responsaveis por prisao mse":    "ausencia_responsaveis_prisao",
		"conflito familiar":                                "conflito_familiar",
		"entrega voluntaria":                               "entrega_voluntaria",
		"exploracao sexual":                                "exploracao_sexual",
		"negligencia":                                      "negligencia",
		"orfandade":                                        "orfandade",
		"pais responsaveis abusivos de drogas":             "responsaveis_drogas",
		"pais responsaveis com algum tipo de deficiencia":  "responsaveis_deficiencia",
		"pais responsaveis com transtorno mental psiquico": "responsaveis_transtorno_mental",
		"pais responsaveis usuarios abusivos de alcool":    "responsaveis_alcool",
		"situacao de rua mendicancia":                      "situacao_rua",
		"trabalho infantil":                                "trabalho_infantil",
		"violencia fisica":                                 "violencia_fisica",
		"violencia psicologica":                            "violencia_psicologica",
		"outro":                                            "outro",
	}
	rotVinculo = rotulos{"mae": "mae", "pai": "pai", "madrasta": "madrasta", "padrasto": "padrasto", "irma": "irma", "irmao": "irmao",
		"tia": "tia", "tio": "tio", "prima": "prima", "primo": "primo", "familia extensa outros": "familia_extensa",
		"familia substituta": "familia_substituta", "instituicao": "instituicao", "sem parentesco": "sem_parentesco"}
)

// vinculo traduz o vínculo do responsável. "Avó" e "Avô" só se distinguem
// pelo acento, então são lidos antes de tirar os acentos.
func vinculo(rotulo string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(rotulo)) {
	case "avó":
		return "avo_feminino", true
	case "avô":
		return "avo_masculino", true
	}
	return rotVinculo.valor(rotulo)
}

// SemDetalhe é o detalhe de "outro" quando o 4.0 não o guardou.
const SemDetalhe = "não detalhado no sistema anterior"

// motivos separa o texto "Negligência, Abandono, Outro, <detalhe>" que o
// 4.0 grava (rótulos marcados unidos por vírgula, com o detalhe de "Outro"
// no fim, repetido em outro_motivo). O que não for rótulo vira o detalhe.
func motivos(texto, outroMotivo string) (lista []string, outro string) {
	outro = strings.TrimSpace(outroMotivo)
	vistos := map[string]bool{}
	var soltos []string
	for _, p := range strings.Split(texto, ",") {
		p = strings.TrimSpace(p)
		if naoInformado(p) || (outro != "" && chave(p) == chave(outro)) {
			continue
		}
		if v, ok := rotMotivo.valor(p); ok {
			if !vistos[v] {
				vistos[v] = true
				lista = append(lista, v)
			}
			continue
		}
		soltos = append(soltos, p)
	}
	if outro == "" {
		outro = strings.Join(soltos, ", ")
	}
	if outro != "" && !vistos["outro"] {
		lista = append(lista, "outro")
	}
	if outro == "" && vistos["outro"] {
		outro = SemDetalhe
	}
	return lista, outro
}

// tipoDesligamento deduz o tipo pelo texto do motivo (o 4.0 só tem o
// texto). inferido=false quando nada bate e o tipo fica "outro".
func tipoDesligamento(motivo string) (tipo string, inferido bool) {
	m := " " + chave(motivo) + " "
	switch {
	case strings.Contains(m, "adocao") || strings.Contains(m, "adotad"):
		return "adocao", true
	case strings.Contains(m, "transfer"):
		return "transferencia", true
	case strings.Contains(m, "maioridade") || strings.Contains(m, " 18 anos"):
		return "maioridade", true
	case strings.Contains(m, "evasao") || strings.Contains(m, "evadiu") || strings.Contains(m, " fuga") || strings.Contains(m, "fugiu"):
		return "evasao", true
	case strings.Contains(m, "reintegr") && strings.Contains(m, "extensa"):
		return "reintegracao_familia_extensa", true
	case strings.Contains(m, "reintegr") || strings.Contains(m, "familia natural") || strings.Contains(m, "retorno a familia"):
		return "reintegracao_familia_natural", true
	case strings.Contains(m, "guarda"):
		return "guarda", true
	}
	return "outro", false
}

// diagnosticosDoChecklist: as colunas do checklist de saúde do 4.0 → os
// nomes padronizados do assistente daqui (frontend casaAbrigo.ts).
var diagnosticosDoChecklist = []struct{ coluna, categoria, nome string }{
	{"doenca_diabetes", "doenca", "Diabetes"}, {"doenca_hipertensao", "doenca", "Hipertensão"},
	{"doenca_saf", "doenca", "Síndrome Alcoólica Fetal"}, {"doenca_hidrocefalia", "doenca", "Hidrocefalia"},
	{"doenca_hiv", "doenca", "HIV"}, {"doenca_hepatite", "doenca", "Hepatite"}, {"doenca_sifilis", "doenca", "Sífilis"},
	{"doenca_tuberculose", "doenca", "Tuberculose"}, {"doenca_outras", "doenca", "Outra(s) doença(s)"},
	{"def_fisica_leve", "deficiencia", "Deficiência física leve"}, {"def_fisica_moderada", "deficiencia", "Deficiência física moderada"},
	{"def_fisica_grave", "deficiencia", "Deficiência física grave"}, {"def_cadeirante", "deficiencia", "Deficiência física (cadeirante)"},
	{"def_nanismo", "deficiencia", "Nanismo"}, {"def_ostomia", "deficiencia", "Deficiência física com ostomia"},
	{"def_auditiva", "deficiencia", "Deficiência auditiva"}, {"def_surdez", "deficiencia", "Surdez"},
	{"def_visual", "deficiencia", "Deficiência visual"}, {"def_cegueira", "deficiencia", "Cegueira"},
	{"def_surdocegueira", "deficiencia", "Surdocegueira"},
	{"transtorno_intelectual_leve", "transtorno", "Deficiência intelectual leve"},
	{"transtorno_intelectual_moderada", "transtorno", "Deficiência intelectual moderada"},
	{"transtorno_intelectual_grave", "transtorno", "Deficiência intelectual grave"},
	{"transtorno_down", "transtorno", "Síndrome de Down"},
	{"transtorno_psicossocial_leve", "transtorno", "Deficiência psicossocial leve"},
	{"transtorno_psicossocial_moderada", "transtorno", "Deficiência psicossocial moderada"},
	{"transtorno_psicossocial_grave", "transtorno", "Deficiência psicossocial grave"},
	{"transtorno_autismo_1", "transtorno", "TEA - Autismo (Grau 1)"}, {"transtorno_autismo_2", "transtorno", "TEA - Autismo (Grau 2)"},
	{"transtorno_autismo_3", "transtorno", "TEA - Autismo (Grau 3)"},
}

// tipoEvolucao: o 4.0 grava "Geral"; o que falar em psicologia vai para o
// registro psicológico (com o sigilo daqui).
func tipoEvolucao(rotulo string) string {
	if strings.Contains(chave(rotulo), "psic") {
		return domain.TipoPsicologico
	}
	return domain.TipoSocial
}
