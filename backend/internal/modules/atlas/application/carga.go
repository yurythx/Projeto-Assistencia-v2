package application

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Formatos aceitos na atualização da TTDD (ADR 022).
const (
	FormatoCSV  = "csv"  // o mesmo da exportação (GET /atlas/ttdd/exportar), revisado no Excel
	FormatoJSON = "json" // o deploy/ttdd/ttdd.json gerado do PDF (scripts/ttdd)
)

// ColunasCSV é o cabeçalho da exportação — a importação exige o mesmo.
var ColunasCSV = []string{"Código", "Série documental", "Órgão", "Função", "Subfunção", "Fase corrente", "Fase intermediária",
	"Destinação final", "Observações", "Recomendação da subfunção", "Fonte"}

// HashCarga identifica o conteúdo enviado: aplicar exige o mesmo hash da
// simulação (o que se aplica é exatamente o que foi conferido).
func HashCarga(conteudo string) string {
	h := sha256.Sum256([]byte(conteudo))
	return hex.EncodeToString(h[:])
}

// LerCarga interpreta o conteúdo no formato dado e valida a carga.
func LerCarga(formato, conteudo string) (domain.CargaTTDD, error) {
	var (
		c   domain.CargaTTDD
		err error
	)
	switch formato {
	case FormatoCSV:
		c, err = lerCSV(conteudo)
	case FormatoJSON:
		c, err = lerJSON(conteudo)
	default:
		return c, domain.InvalidError{Msg: "formato desconhecido: use csv ou json"}
	}
	if err != nil {
		return c, err
	}
	return c, c.Validar()
}

// ------------------------------------------------------------------ JSON

type cargaJSON struct {
	Orgaos map[string]struct {
		Nome, Edicao, Data, Versao string
	} `json:"orgaos"`
	Funcoes    map[string]string `json:"funcoes"`
	Subfuncoes map[string]struct {
		Nome, Recomendacao string
	} `json:"subfuncoes"`
	Itens []struct {
		Codigo                string  `json:"codigo"`
		Subfuncao             string  `json:"subfuncao"`
		Descritor             string  `json:"descritor"`
		CorrenteAnos          *int    `json:"corrente_anos"`
		CorrenteCondicao      string  `json:"corrente_condicao"`
		IntermediariaAnos     *int    `json:"intermediaria_anos"`
		IntermediariaCondicao string  `json:"intermediaria_condicao"`
		Destinacao            *string `json:"destinacao"`
		Observacoes           string  `json:"observacoes"`
	} `json:"itens"`
}

func lerJSON(conteudo string) (domain.CargaTTDD, error) {
	var j cargaJSON
	var c domain.CargaTTDD
	if err := json.Unmarshal([]byte(conteudo), &j); err != nil {
		return c, domain.InvalidError{Msg: "JSON inválido: " + err.Error()}
	}
	for _, p := range ordenadas(j.Orgaos) {
		o := j.Orgaos[p]
		data, err := domain.DataPublicacao(o.Data)
		if err != nil {
			return c, err
		}
		c.Orgaos = append(c.Orgaos, domain.OrgaoTTDD{Prefixo: p, Nome: o.Nome, EdicaoDiario: o.Edicao, DataPublicacao: data, Versao: o.Versao})
	}
	for _, f := range ordenadas(j.Funcoes) {
		c.Funcoes = append(c.Funcoes, domain.CargaFuncao{Codigo: f, Nome: j.Funcoes[f]})
	}
	for _, s := range ordenadas(j.Subfuncoes) {
		c.Subfuncoes = append(c.Subfuncoes, domain.CargaSubfuncao{Codigo: s, Nome: j.Subfuncoes[s].Nome, Recomendacao: j.Subfuncoes[s].Recomendacao})
	}
	for _, i := range j.Itens {
		var dest *domain.DestinacaoFinal
		if i.Destinacao != nil && *i.Destinacao != "" {
			d := domain.DestinacaoFinal(*i.Destinacao)
			dest = &d
		}
		c.Series = append(c.Series, domain.CargaSerie{Codigo: i.Codigo, Subfuncao: i.Subfuncao, PrazosTTDD: domain.PrazosTTDD{
			Descritor: i.Descritor, FaseCorrenteAnos: i.CorrenteAnos, FaseCorrenteCondicao: i.CorrenteCondicao,
			FaseIntermAnos: i.IntermediariaAnos, FaseIntermCondicao: i.IntermediariaCondicao, DestinacaoFinal: dest, Observacoes: i.Observacoes,
		}})
	}
	return c, nil
}

func ordenadas[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ------------------------------------------------------------------- CSV

var (
	anosRe  = regexp.MustCompile(`^(\d{1,3})\s+anos?$`)
	fonteRe = regexp.MustCompile(`^TTDD da (.+?)(?:, versão ([^,]+))?(?:, Diário Oficial nº (\S+)(?: de (\d{2}/\d{2}/\d{4}))?)?$`)
)

// fase lê um prazo como a exportação escreve (domain.Fase): "N ano(s)",
// "não informado na TTDD", "não há" ou a condição.
func fase(texto string) (*int, string) {
	t := strings.TrimSpace(texto)
	if m := anosRe.FindStringSubmatch(strings.ToLower(t)); m != nil {
		n, _ := strconv.Atoi(m[1]) // \d{1,3}: sempre converte
		return &n, ""
	}
	switch strings.ToLower(t) {
	case "", "não há", "nao ha", "não informado na ttdd", "nao informado na ttdd", "-", "–":
		return nil, ""
	}
	return nil, t
}

func destinacao(texto string) (*domain.DestinacaoFinal, bool) {
	var d domain.DestinacaoFinal
	switch domain.Fold(strings.TrimSpace(texto)) {
	case "guarda permanente", "guarda_permanente":
		d = domain.DestinacaoGuardaPermanente
	case "eliminacao":
		d = domain.DestinacaoEliminacao
	case "", "nao definida na ttdd", "nao definida", "x":
		return nil, true
	default:
		return nil, false
	}
	return &d, true
}

// codigoNome separa "2.0.02 Gestão de Compras" em código e nome.
func codigoNome(texto string) (string, string) {
	codigo, nome, _ := strings.Cut(strings.TrimSpace(texto), " ")
	return codigo, strings.TrimSpace(nome)
}

func lerCSV(conteudo string) (domain.CargaTTDD, error) {
	var c domain.CargaTTDD
	conteudo = strings.TrimPrefix(conteudo, string([]byte{0xEF, 0xBB, 0xBF})) // BOM do Excel
	primeira, _, _ := strings.Cut(conteudo, "\n")
	r := csv.NewReader(strings.NewReader(conteudo))
	r.Comma = ';'
	if !strings.Contains(primeira, ";") {
		r.Comma = ','
	}
	r.FieldsPerRecord = -1
	linhas, err := r.ReadAll()
	if err != nil {
		return c, domain.InvalidError{Msg: "CSV ilegível: " + err.Error()}
	}
	if len(linhas) == 0 || len(linhas[0]) < len(ColunasCSV) || strings.Join(linhas[0][:len(ColunasCSV)], ";") != strings.Join(ColunasCSV, ";") {
		return c, domain.InvalidError{Msg: "cabeçalho diferente do da exportação: " + strings.Join(ColunasCSV, ";")}
	}
	orgaos := map[string]domain.OrgaoTTDD{}
	funcoes := map[string]string{}
	subs := map[string]domain.CargaSubfuncao{}
	var erros []string
	for n, l := range linhas[1:] {
		linha := n + 2
		if strings.TrimSpace(strings.Join(l, "")) == "" {
			continue // linha em branco (o Excel deixa ";;;;" no fim)
		}
		if len(l) < len(ColunasCSV) {
			erros = append(erros, fmt.Sprintf("linha %d: %d colunas (esperadas %d)", linha, len(l), len(ColunasCSV)))
			continue
		}
		codigo := strings.TrimSpace(l[0])
		prefixo := strings.SplitN(codigo, ".", 3)
		org := domain.OrgaoTTDD{Nome: strings.TrimSpace(l[2])}
		if len(prefixo) >= 2 {
			org.Prefixo = prefixo[0] + "." + prefixo[1]
		}
		if m := fonteRe.FindStringSubmatch(strings.TrimSpace(l[10])); m != nil {
			org.Versao, org.EdicaoDiario = m[2], m[3]
			if org.DataPublicacao, err = domain.DataPublicacao(m[4]); err != nil {
				erros = append(erros, fmt.Sprintf("linha %d: %v", linha, err))
			}
		}
		if _, ok := orgaos[org.Prefixo]; !ok {
			orgaos[org.Prefixo] = org
		}
		fCod, fNome := codigoNome(l[3])
		funcoes[fCod] = fNome
		sCod, sNome := codigoNome(l[4])
		subs[sCod] = domain.CargaSubfuncao{Codigo: sCod, Nome: sNome, Recomendacao: strings.TrimSpace(l[9])}
		dest, ok := destinacao(l[7])
		if !ok {
			erros = append(erros, fmt.Sprintf("linha %d: destinação %q (use guarda permanente, eliminação ou deixe vazio)", linha, l[7]))
		}
		ca, cc := fase(l[5])
		ia, ic := fase(l[6])
		c.Series = append(c.Series, domain.CargaSerie{Codigo: codigo, Subfuncao: sCod, Linha: linha, PrazosTTDD: domain.PrazosTTDD{
			Descritor: strings.TrimSpace(l[1]), FaseCorrenteAnos: ca, FaseCorrenteCondicao: cc, FaseIntermAnos: ia,
			FaseIntermCondicao: ic, DestinacaoFinal: dest, Observacoes: strings.TrimSpace(l[8]),
		}})
	}
	if len(erros) > 0 {
		return c, domain.InvalidError{Msg: "a planilha tem problemas: " + strings.Join(erros[:min(len(erros), domain.MaxErrosCarga)], "; ")}
	}
	for _, p := range ordenadas(orgaos) {
		c.Orgaos = append(c.Orgaos, orgaos[p])
	}
	for _, f := range ordenadas(funcoes) {
		c.Funcoes = append(c.Funcoes, domain.CargaFuncao{Codigo: f, Nome: funcoes[f]})
	}
	for _, s := range ordenadas(subs) {
		c.Subfuncoes = append(c.Subfuncoes, subs[s])
	}
	return c, nil
}

// ---------------------------------------------------------------- serviço

// errSimulacao desfaz a transação da simulação (nada é gravado).
var errSimulacao = errors.New("atlas: simulação da carga (desfeita)")

// ImpactoTTDD compara a carga com o banco e, com aplicar, grava (séries
// novas e alteradas, revogação das que saíram, histórico pelo gatilho) e
// audita. Simular roda as mesmas consultas numa transação desfeita.
// Aplicar exige o hash da simulação: grava-se o que foi conferido.
func (s *Service) ImpactoTTDD(ctx context.Context, formato, conteudo, hashConferido string, aplicar bool) (domain.ImpactoCarga, error) {
	hash := HashCarga(conteudo)
	if aplicar && hashConferido != hash {
		return domain.ImpactoCarga{}, MapError(domain.InvalidError{Msg: "o arquivo enviado não é o que foi simulado: simule de novo antes de aplicar"})
	}
	carga, err := LerCarga(formato, conteudo)
	if err != nil {
		return domain.ImpactoCarga{}, MapError(err)
	}
	var out domain.ImpactoCarga
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if out, err = s.repo.CargaTTDD(ctx, tx, carga, aplicar); err != nil {
			return err
		}
		if !aplicar {
			return errSimulacao
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "atlas.ttdd.carga.aplicada", "atlas_ttdd", hash, nil,
			map[string]any{"formato": formato, "totais": out.Totais, "orgaos": len(out.Orgaos), "procedimentos_afetados": len(out.Procedimentos)}))
	})
	if errors.Is(err, errSimulacao) {
		err = nil
	}
	out.Hash, out.Aplicada, out.Orgaos = hash, aplicar && err == nil, carga.Orgaos
	return out, MapError(err)
}
