package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// CargaTTDD é uma nova publicação da TTDD enviada pela gestão (planilha
// exportada e revisada, ou o ttdd.json gerado do PDF) — ADR 022. Os
// órgãos presentes delimitam o que pode ser revogado.
type CargaTTDD struct {
	Orgaos     []OrgaoTTDD
	Funcoes    []CargaFuncao
	Subfuncoes []CargaSubfuncao
	Series     []CargaSerie
}

// CargaFuncao é uma função do plano de classificação.
type CargaFuncao struct {
	Codigo string
	Nome   string
}

// CargaSubfuncao é uma subfunção, com a recomendação que vale para as
// séries dela.
type CargaSubfuncao struct {
	Codigo       string
	Nome         string
	Recomendacao string
}

// CargaSerie é uma série da nova publicação.
type CargaSerie struct {
	Codigo    string
	Subfuncao string
	PrazosTTDD
	// Linha da planilha (0 = sem linha, ex.: JSON) — para as mensagens.
	Linha int
}

// Situações de uma série na comparação com o banco.
const (
	SituacaoNova          = "NOVA"
	SituacaoAlterada      = "ALTERADA"
	SituacaoRevogada      = "REVOGADA"
	SituacaoRestabelecida = "RESTABELECIDA"
	SituacaoInalterada    = "INALTERADA"
)

// SerieImpacto é a série com os valores de antes e de depois da carga.
type SerieImpacto struct {
	Codigo    string      `json:"codigo"`
	Descritor string      `json:"descritor"`
	Situacao  string      `json:"situacao"`
	Antes     *PrazosTTDD `json:"antes"`
	Depois    *PrazosTTDD `json:"depois"`
}

// ProcedimentoAfetado aponta para uma série alterada ou revogada.
type ProcedimentoAfetado struct {
	ID               uuid.UUID `json:"id"`
	CodigoProcessual string    `json:"codigo_processual"`
	Versao           int       `json:"versao"`
	Ativo            bool      `json:"ativo"`
	CodigoTTDD       string    `json:"codigo_ttdd"`
	Situacao         string    `json:"situacao"`
}

// ImpactoCarga é o que a carga muda: totais por situação, as séries que
// mudam (as inalteradas só entram nos totais) e os procedimentos afetados.
type ImpactoCarga struct {
	Hash          string                `json:"hash"`
	Aplicada      bool                  `json:"aplicada"`
	Orgaos        []OrgaoTTDD           `json:"orgaos"`
	Totais        map[string]int        `json:"totais"`
	Series        []SerieImpacto        `json:"series"`
	Procedimentos []ProcedimentoAfetado `json:"procedimentos"`
}

var (
	prefixoOrgao   = regexp.MustCompile(`^\d{1,2}\.0$`)
	codigoFuncao   = regexp.MustCompile(`^\d{1,2}\.0\.\d{2}$`)
	codigoSubfunc  = regexp.MustCompile(`^\d{1,2}\.0\.\d{2}\.\d{2}$`)
	codigoSerieTTD = regexp.MustCompile(`^\d{1,2}\.0\.\d{2}\.\d{2}\.\d{2}(-\d)?$`)
)

// MaxErrosCarga limita as mensagens devolvidas (a primeira dezena basta
// para corrigir a planilha).
const MaxErrosCarga = 20

// pai devolve os n primeiros segmentos do código ("2.0.01.00.05", 3 → "2.0.01").
func pai(codigo string, n int) string {
	partes := strings.Split(strings.SplitN(codigo, "-", 2)[0], ".")
	return strings.Join(partes[:min(n, len(partes))], ".")
}

// Validar confere a carga inteira e devolve todas as falhas (até
// MaxErrosCarga) numa só mensagem, com a linha quando houver.
func (c CargaTTDD) Validar() error {
	var erros []string
	falha := func(linha int, format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		if linha > 0 {
			msg = fmt.Sprintf("linha %d: %s", linha, msg)
		}
		erros = append(erros, msg)
	}
	if len(c.Series) == 0 {
		falha(0, "a carga não tem nenhuma série")
	}
	orgaos := map[string]bool{}
	for _, o := range c.Orgaos {
		switch {
		case !prefixoOrgao.MatchString(o.Prefixo):
			falha(0, "órgão com código inválido: %q", o.Prefixo)
		case strings.TrimSpace(o.Nome) == "":
			falha(0, "órgão %s sem nome", o.Prefixo)
		case orgaos[o.Prefixo]:
			falha(0, "órgão %s repetido", o.Prefixo)
		}
		orgaos[o.Prefixo] = true
	}
	funcoes := map[string]bool{}
	for _, f := range c.Funcoes {
		if !codigoFuncao.MatchString(f.Codigo) || !orgaos[pai(f.Codigo, 2)] || strings.TrimSpace(f.Nome) == "" {
			falha(0, "função inválida ou de órgão ausente: %q %q", f.Codigo, f.Nome)
		}
		funcoes[f.Codigo] = true
	}
	subs := map[string]bool{}
	for _, s := range c.Subfuncoes {
		if !codigoSubfunc.MatchString(s.Codigo) || !funcoes[pai(s.Codigo, 3)] || strings.TrimSpace(s.Nome) == "" {
			falha(0, "subfunção inválida ou de função ausente: %q %q", s.Codigo, s.Nome)
		}
		subs[s.Codigo] = true
	}
	vistas := map[string]bool{}
	for _, s := range c.Series {
		fase := func(nome string, anos *int, condicao string) {
			if anos != nil && (*anos < 0 || *anos > 999 || condicao != "") {
				falha(s.Linha, "%s %s: prazo em anos (0 a 999) ou condição, não os dois", s.Codigo, nome)
			}
		}
		switch {
		case !codigoSerieTTD.MatchString(s.Codigo):
			falha(s.Linha, "código de série inválido: %q (ex.: 2.0.01.00.05)", s.Codigo)
		case vistas[s.Codigo]:
			falha(s.Linha, "série %s repetida", s.Codigo)
		case !subs[s.Subfuncao] || pai(s.Codigo, 4) != s.Subfuncao:
			falha(s.Linha, "série %s: subfunção %q ausente da carga ou de outro ramo", s.Codigo, s.Subfuncao)
		case strings.TrimSpace(s.Descritor) == "" || utf8.RuneCountInString(s.Descritor) > 2000:
			falha(s.Linha, "série %s sem descritor (ou acima de 2.000 caracteres)", s.Codigo)
		case s.DestinacaoFinal != nil && *s.DestinacaoFinal != DestinacaoGuardaPermanente && *s.DestinacaoFinal != DestinacaoEliminacao:
			falha(s.Linha, "série %s: destinação inválida", s.Codigo)
		}
		fase("fase corrente", s.FaseCorrenteAnos, s.FaseCorrenteCondicao)
		fase("fase intermediária", s.FaseIntermAnos, s.FaseIntermCondicao)
		vistas[s.Codigo] = true
	}
	if len(erros) == 0 {
		return nil
	}
	resto := ""
	if len(erros) > MaxErrosCarga {
		resto = fmt.Sprintf(" (e mais %d)", len(erros)-MaxErrosCarga)
		erros = erros[:MaxErrosCarga]
	}
	return invalid("a carga tem %d problema(s)%s: %s", len(erros), resto, strings.Join(erros, "; "))
}

// DataPublicacao interpreta a data da publicação (AAAA-MM-DD ou
// DD/MM/AAAA); vazio = sem data.
func DataPublicacao(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t, nil
		}
	}
	return nil, invalid("data de publicação inválida: %q (use AAAA-MM-DD ou DD/MM/AAAA)", s)
}
