// Package domain contém as regras do Atlas: a Tabela de Temporalidade e
// Destinação de Documentos (TTDD/CCPAD) e os procedimentos canônicos de
// processo administrativo eletrônico (padrão SEI) — etapas, peças
// exigidas e regras de transição.
package domain

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Erros de domínio (traduzidos para HTTP em application.MapError).
var (
	ErrNotFound     = errors.New("atlas: procedimento não encontrado")
	ErrTTDDNotFound = errors.New("atlas: classificação TTDD não encontrada")
	ErrDuplicate    = errors.New("atlas: já existe procedimento com este código e versão")
	// ErrIADesligada: nenhuma conexão de IA para o assistente — a resposta
	// é a síntese canônica (não é falha).
	ErrIADesligada = errors.New("atlas: assistente sem IA configurada")
)

// InvalidError é uma violação de regra de cadastro (422 com a mensagem).
type InvalidError struct{ Msg string }

func (e InvalidError) Error() string { return e.Msg }

func invalid(format string, a ...any) error { return InvalidError{Msg: fmt.Sprintf(format, a...)} }

// DestinacaoFinal é o destino do documento ao fim da guarda (CONARQ).
type DestinacaoFinal string

const (
	DestinacaoGuardaPermanente DestinacaoFinal = "GUARDA_PERMANENTE"
	DestinacaoEliminacao       DestinacaoFinal = "ELIMINACAO"
)

// NivelAcesso é o nível de acesso SEI sugerido para os processos do tipo.
// Descreve o processo que nasce do procedimento — o roteiro em si é
// informação pública (transparência ativa, LAI art. 8º).
type NivelAcesso string

const (
	NivelPublico  NivelAcesso = "PUBLICO"
	NivelRestrito NivelAcesso = "RESTRITO"
	NivelSigiloso NivelAcesso = "SIGILOSO"
)

// FormatoDocumento distingue o documento nascido no sistema do digitalizado.
type FormatoDocumento string

const (
	FormatoNatoDigital         FormatoDocumento = "NATO_DIGITAL"
	FormatoExternoDigitalizado FormatoDocumento = "EXTERNO_DIGITALIZADO"
)

// TipoAssinatura é a forma de assinatura exigida da peça.
type TipoAssinatura string

const (
	AssinaturaIndividual         TipoAssinatura = "INDIVIDUAL"
	AssinaturaConjuntaMultinivel TipoAssinatura = "CONJUNTA_MULTINIVEL"
	AssinaturaEmBloco            TipoAssinatura = "EM_BLOCO"
)

// ClassificacaoTTDD é uma série documental da Tabela de Temporalidade
// (TTDD oficial: órgão > função > subfunção > série — ADR 017).
//
// Cada fase de guarda tem prazo em anos OU uma condição ("Enquanto estiver
// vigorando", "30 dias após a data do evento"); nenhum dos dois na fase
// intermediária = não há fase intermediária; na corrente = o documento
// não informa. Destinação nil = a TTDD não define (ex.: "X — entregue ao
// paciente").
type ClassificacaoTTDD struct {
	Codigo               string           `json:"codigo"`
	Descritor            string           `json:"descritor"`
	FaseCorrenteAnos     *int             `json:"fase_corrente_anos"`
	FaseCorrenteCondicao string           `json:"fase_corrente_condicao"`
	FaseIntermAnos       *int             `json:"fase_interm_anos"`
	FaseIntermCondicao   string           `json:"fase_interm_condicao"`
	DestinacaoFinal      *DestinacaoFinal `json:"destinacao_final"`
	Observacoes          string           `json:"observacoes"`
	Subfuncao            *SubfuncaoTTDD   `json:"subfuncao,omitempty"`
	// RevogadaEm: a série saiu da TTDD na publicação RevogadaEdicao (nil =
	// vigente). Continua consultável: documentos já produzidos seguem nela.
	RevogadaEm     *time.Time `json:"revogada_em"`
	RevogadaEdicao string     `json:"revogada_edicao"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Vigente informa se a série está na TTDD em vigor.
func (c ClassificacaoTTDD) Vigente() bool { return c.RevogadaEm == nil }

// Eventos do histórico de uma série (gatilho da migration 000135).
const (
	HistoricoAlterada      = "ALTERADA"
	HistoricoRevogada      = "REVOGADA"
	HistoricoRestabelecida = "RESTABELECIDA"
)

// PrazosTTDD são os valores de uma série guardados no histórico.
type PrazosTTDD struct {
	Descritor            string           `json:"descritor"`
	FaseCorrenteAnos     *int             `json:"fase_corrente_anos"`
	FaseCorrenteCondicao string           `json:"fase_corrente_condicao"`
	FaseIntermAnos       *int             `json:"fase_interm_anos"`
	FaseIntermCondicao   string           `json:"fase_interm_condicao"`
	DestinacaoFinal      *DestinacaoFinal `json:"destinacao_final"`
	Observacoes          string           `json:"observacoes"`
}

// HistoricoTTDD é uma mudança da série: o evento, os valores anteriores e
// a publicação que a trouxe.
type HistoricoTTDD struct {
	Evento       string     `json:"evento"`
	Anterior     PrazosTTDD `json:"anterior"`
	EdicaoDiario string     `json:"edicao_diario"`
	RegistradoEm time.Time  `json:"registrado_em"`
}

// SituacaoTTDD é o resultado da conferência da série no cadastro.
type SituacaoTTDD int

const (
	TTDDInexistente SituacaoTTDD = iota
	TTDDVigente
	TTDDRevogada
)

// OrgaoTTDD é o órgão dono de uma TTDD, com a publicação oficial dela.
type OrgaoTTDD struct {
	Prefixo        string     `json:"prefixo"`
	Nome           string     `json:"nome"`
	EdicaoDiario   string     `json:"edicao_diario"`
	DataPublicacao *time.Time `json:"data_publicacao"`
	Versao         string     `json:"versao"`
}

// FuncaoTTDD agrupa subfunções de um órgão.
type FuncaoTTDD struct {
	Codigo string    `json:"codigo"`
	Nome   string    `json:"nome"`
	Orgao  OrgaoTTDD `json:"orgao"`
}

// SubfuncaoTTDD agrupa séries; a recomendação vale para todas elas.
type SubfuncaoTTDD struct {
	Codigo       string     `json:"codigo"`
	Nome         string     `json:"nome"`
	Recomendacao string     `json:"recomendacao"`
	Funcao       FuncaoTTDD `json:"funcao"`
}

// EstruturaTTDD é a árvore órgão > função > subfunção com a contagem de séries.
type EstruturaTTDD struct {
	OrgaoTTDD
	Total   int                   `json:"total"`
	Funcoes []EstruturaFuncaoTTDD `json:"funcoes"`
}

type EstruturaFuncaoTTDD struct {
	Codigo     string                   `json:"codigo"`
	Nome       string                   `json:"nome"`
	Total      int                      `json:"total"`
	Subfuncoes []EstruturaSubfuncaoTTDD `json:"subfuncoes"`
}

type EstruturaSubfuncaoTTDD struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
	Total  int    `json:"total"`
}

// FiltroTTDD filtra a consulta da TTDD. Os códigos são prefixos
// hierárquicos (órgão "2.0", função "2.0.01", subfunção "2.0.01.00").
type FiltroTTDD struct {
	Query  string
	Codigo string
}

// Fase descreve o prazo de uma fase para leitura humana.
func Fase(anos *int, condicao string, corrente bool) string {
	switch {
	case anos != nil && *anos == 1:
		return "1 ano"
	case anos != nil:
		return fmt.Sprintf("%d anos", *anos)
	case condicao != "":
		return condicao
	case corrente:
		return "não informado na TTDD"
	}
	return "não há"
}

// PrazoTotalAnos soma as fases quando ambas são em anos (fase intermediária
// inexistente conta zero). Prazo por condição não tem total em anos.
func (c ClassificacaoTTDD) PrazoTotalAnos() (int, bool) {
	if c.FaseCorrenteAnos == nil || c.FaseCorrenteCondicao != "" || c.FaseIntermCondicao != "" {
		return 0, false
	}
	total := *c.FaseCorrenteAnos
	if c.FaseIntermAnos != nil {
		total += *c.FaseIntermAnos
	}
	return total, true
}

// Destinacao descreve a destinação final para leitura humana.
func (c ClassificacaoTTDD) Destinacao() string {
	switch {
	case c.DestinacaoFinal == nil:
		return "não definida na TTDD"
	case *c.DestinacaoFinal == DestinacaoGuardaPermanente:
		return "guarda permanente"
	}
	return "eliminação"
}

var codigoTTDD = regexp.MustCompile(`^\d{1,2}\.0(\.\d{2}){0,3}(-\d)?$`)

// CodigoTTDDValido aceita o código de uma série ou um prefixo hierárquico
// (órgão, função, subfunção) — usado nos filtros e na consulta.
func CodigoTTDDValido(c string) bool { return codigoTTDD.MatchString(c) }

// Workflow é o procedimento canônico de um tipo de processo.
type Workflow struct {
	ID               uuid.UUID `json:"id"`
	CodigoProcessual string    `json:"codigo_processual"`
	Titulo           string    `json:"titulo"`
	Objetivo         string    `json:"objetivo"`
	PublicoAlvo      string    `json:"publico_alvo"`
	Versao           int       `json:"versao"`
	Ativo            bool      `json:"ativo"`
	// Situacao da validação (ADR 028): RASCUNHO, EM_VALIDACAO ou HOMOLOGADO;
	// só o homologado fica ativo.
	Situacao      string             `json:"situacao"`
	NivelAcesso   NivelAcesso        `json:"nivel_acesso"`
	HipoteseLegal string             `json:"hipotese_legal_restricao"`
	CodigoTTDD    string             `json:"codigo_ttdd"`
	Classificacao *ClassificacaoTTDD `json:"classificacao,omitempty"`
	TotalEtapas   int                `json:"total_etapas"`
	Etapas        []Etapa            `json:"etapas"`
	CreatedBy     *uuid.UUID         `json:"created_by"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// Etapa é um nó de tramitação (setor) do percurso.
type Etapa struct {
	ID                      uuid.UUID        `json:"id"`
	Ordem                   int              `json:"ordem"`
	UnidadeAdministrativa   string           `json:"unidade_administrativa"`
	NomeSetor               string           `json:"nome_setor"`
	AtribuicoesSetor        string           `json:"atribuicoes_setor"`
	PrazoSLAEmDias          int              `json:"prazo_sla_em_dias"`
	ManterAbertoAposRemessa bool             `json:"manter_aberto_apos_remessa"`
	Documentos              []EtapaDocumento `json:"documentos"`
	Transicoes              []EtapaTransicao `json:"transicoes"`
}

// EtapaDocumento é uma peça exigida na etapa.
type EtapaDocumento struct {
	ID                    uuid.UUID        `json:"id"`
	NomeDocumento         string           `json:"nome_documento"`
	Obrigatorio           bool             `json:"obrigatorio"`
	Formato               FormatoDocumento `json:"formato"`
	TipoAssinatura        TipoAssinatura   `json:"tipo_assinatura"`
	ExigeConferenciaCopia bool             `json:"exige_conferencia_copia"`
	ModeloMinutaPadraoURL string           `json:"modelo_minuta_padrao_url"`
	// ModeloID liga a peça a um modelo da biblioteca (ADR 024); Modelo traz
	// a versão atual dele na leitura.
	ModeloID *uuid.UUID    `json:"modelo_id"`
	Modelo   *ModeloResumo `json:"modelo,omitempty"`
}

// EtapaTransicao leva o processo da etapa a outra (envio regular ou
// devolução em diligência). O destino é identificado pela ordem.
type EtapaTransicao struct {
	ID                    uuid.UUID `json:"id"`
	DestinoOrdem          int       `json:"destino_ordem"`
	CondicaoTransicao     string    `json:"condicao_transicao"`
	IsDevolucaoDiligencia bool      `json:"is_devolucao_diligencia"`
	DescricaoDiligencia   string    `json:"descricao_diligencia"`
}

// Limites de cadastro (espelham as colunas da migration 000132).
const (
	MaxEtapas      = 50
	MaxDocumentos  = 30
	MaxTransicoes  = 10
	MaxPrazoSLA    = 3650
	maxCodigo      = 64
	maxTitulo      = 255
	maxSigla       = 64
	maxNomeSetor   = 150
	maxNomeDoc     = 150
	maxCondicao    = 255
	maxHipotese    = 255
	maxObjetivo    = 5000
	maxAtribuicoes = 5000
)

var codigoProcessual = regexp.MustCompile(`^[A-Z0-9]+(\.[A-Z0-9]+)*$`)

// Normalize apara os textos e põe o código em caixa alta.
func (w *Workflow) Normalize() {
	w.CodigoProcessual = strings.ToUpper(strings.TrimSpace(w.CodigoProcessual))
	w.Titulo = strings.TrimSpace(w.Titulo)
	w.Objetivo = strings.TrimSpace(w.Objetivo)
	w.PublicoAlvo = strings.TrimSpace(w.PublicoAlvo)
	w.HipoteseLegal = strings.TrimSpace(w.HipoteseLegal)
	w.CodigoTTDD = strings.TrimSpace(w.CodigoTTDD)
	for i := range w.Etapas {
		e := &w.Etapas[i]
		e.UnidadeAdministrativa = strings.ToUpper(strings.TrimSpace(e.UnidadeAdministrativa))
		e.NomeSetor = strings.TrimSpace(e.NomeSetor)
		e.AtribuicoesSetor = strings.TrimSpace(e.AtribuicoesSetor)
		for j := range e.Documentos {
			d := &e.Documentos[j]
			d.NomeDocumento = strings.TrimSpace(d.NomeDocumento)
			d.ModeloMinutaPadraoURL = strings.TrimSpace(d.ModeloMinutaPadraoURL)
		}
		for j := range e.Transicoes {
			t := &e.Transicoes[j]
			t.CondicaoTransicao = strings.TrimSpace(t.CondicaoTransicao)
			t.DescricaoDiligencia = strings.TrimSpace(t.DescricaoDiligencia)
		}
	}
}

func tooLong(s string, max int) bool { return utf8.RuneCountInString(s) > max }

// Validate aplica as regras de cadastro do procedimento (chame Normalize
// antes). A existência do código TTDD é conferida na transação.
func (w Workflow) Validate() error {
	switch {
	case w.CodigoProcessual == "" || tooLong(w.CodigoProcessual, maxCodigo) || !codigoProcessual.MatchString(w.CodigoProcessual):
		return invalid("código processual inválido: use segmentos alfanuméricos separados por ponto (ex.: ADM.LIC.001)")
	case w.Titulo == "" || tooLong(w.Titulo, maxTitulo):
		return invalid("título obrigatório (até %d caracteres)", maxTitulo)
	case w.Objetivo == "" || tooLong(w.Objetivo, maxObjetivo):
		return invalid("objetivo obrigatório (até %d caracteres)", maxObjetivo)
	case w.PublicoAlvo == "" || tooLong(w.PublicoAlvo, maxTitulo):
		return invalid("público-alvo obrigatório (até %d caracteres)", maxTitulo)
	case w.CodigoTTDD == "":
		return invalid("código TTDD obrigatório")
	case w.Versao < 1:
		return invalid("versão deve ser maior ou igual a 1")
	}
	switch w.NivelAcesso {
	case NivelPublico:
	case NivelRestrito, NivelSigiloso:
		// SEI/LAI: restrição de acesso sempre aponta a hipótese legal.
		if w.HipoteseLegal == "" {
			return invalid("nível de acesso %s exige a hipótese legal da restrição", w.NivelAcesso)
		}
	default:
		return invalid("nível de acesso inválido (PUBLICO, RESTRITO ou SIGILOSO)")
	}
	if tooLong(w.HipoteseLegal, maxHipotese) {
		return invalid("hipótese legal até %d caracteres", maxHipotese)
	}
	if len(w.Etapas) == 0 || len(w.Etapas) > MaxEtapas {
		return invalid("o procedimento precisa de 1 a %d etapas", MaxEtapas)
	}
	ordens := map[int]bool{}
	for _, e := range w.Etapas {
		if e.Ordem < 1 {
			return invalid("ordem da etapa deve ser maior ou igual a 1")
		}
		if ordens[e.Ordem] {
			return invalid("ordem de etapa repetida: %d", e.Ordem)
		}
		ordens[e.Ordem] = true
	}
	for _, e := range w.Etapas {
		if err := e.validate(ordens); err != nil {
			return err
		}
	}
	return nil
}

func (e Etapa) validate(ordens map[int]bool) error {
	switch {
	case e.UnidadeAdministrativa == "" || tooLong(e.UnidadeAdministrativa, maxSigla):
		return invalid("etapa %d: sigla da unidade obrigatória (até %d caracteres)", e.Ordem, maxSigla)
	case e.NomeSetor == "" || tooLong(e.NomeSetor, maxNomeSetor):
		return invalid("etapa %d: nome do setor obrigatório (até %d caracteres)", e.Ordem, maxNomeSetor)
	case e.AtribuicoesSetor == "" || tooLong(e.AtribuicoesSetor, maxAtribuicoes):
		return invalid("etapa %d: atribuições obrigatórias (até %d caracteres)", e.Ordem, maxAtribuicoes)
	case e.PrazoSLAEmDias < 0 || e.PrazoSLAEmDias > MaxPrazoSLA:
		return invalid("etapa %d: prazo SLA entre 0 e %d dias", e.Ordem, MaxPrazoSLA)
	case len(e.Documentos) > MaxDocumentos:
		return invalid("etapa %d: no máximo %d peças", e.Ordem, MaxDocumentos)
	case len(e.Transicoes) > MaxTransicoes:
		return invalid("etapa %d: no máximo %d transições", e.Ordem, MaxTransicoes)
	}
	for _, d := range e.Documentos {
		if d.NomeDocumento == "" || tooLong(d.NomeDocumento, maxNomeDoc) {
			return invalid("etapa %d: nome da peça obrigatório (até %d caracteres)", e.Ordem, maxNomeDoc)
		}
		if d.Formato != FormatoNatoDigital && d.Formato != FormatoExternoDigitalizado {
			return invalid("etapa %d: formato da peça inválido (NATO_DIGITAL ou EXTERNO_DIGITALIZADO)", e.Ordem)
		}
		switch d.TipoAssinatura {
		case AssinaturaIndividual, AssinaturaConjuntaMultinivel, AssinaturaEmBloco:
		default:
			return invalid("etapa %d: tipo de assinatura inválido (INDIVIDUAL, CONJUNTA_MULTINIVEL ou EM_BLOCO)", e.Ordem)
		}
		if d.ModeloMinutaPadraoURL != "" && !httpURL(d.ModeloMinutaPadraoURL) {
			return invalid("etapa %d: o modelo de minuta precisa ser um endereço http(s) completo", e.Ordem)
		}
	}
	for _, t := range e.Transicoes {
		switch {
		case !ordens[t.DestinoOrdem]:
			return invalid("etapa %d: a transição aponta para uma etapa inexistente (%d)", e.Ordem, t.DestinoOrdem)
		case t.DestinoOrdem == e.Ordem:
			return invalid("etapa %d: a transição não pode apontar para a própria etapa", e.Ordem)
		case t.CondicaoTransicao == "" || tooLong(t.CondicaoTransicao, maxCondicao):
			return invalid("etapa %d: condição da transição obrigatória (até %d caracteres)", e.Ordem, maxCondicao)
		case t.IsDevolucaoDiligencia && t.DescricaoDiligencia == "":
			return invalid("etapa %d: devolução em diligência exige a descrição da diligência", e.Ordem)
		}
	}
	return nil
}

// httpURL aceita só endereço absoluto http(s) sem credenciais — o link é
// exibido como <a href>, e "javascript:" viraria XSS armazenado.
func httpURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}

// Filter filtra a listagem de procedimentos.
type Filter struct {
	Query      string
	CodigoTTDD string
	// CodigoProcessual: todas as versões de um procedimento (código exato).
	CodigoProcessual string
	// IncluirInativos: só a gestão (atlas:manage) vê os desativados.
	IncluirInativos bool
	// PrefixoTTDD: procedimentos enquadrados no órgão, na função ou na
	// subfunção (ex.: "2.0" = todos da Administração).
	PrefixoTTDD string
	// Situacao: RASCUNHO, EM_VALIDACAO ou HOMOLOGADO (só com IncluirInativos).
	Situacao string
	// Ultima: só a versão mais recente de cada código — onde está o rascunho
	// da revisão (só com IncluirInativos; a consulta pública já mostra só a
	// versão em vigor).
	Ultima bool
}

// Repository é a persistência do Atlas.
type Repository interface {
	ListTTDD(ctx context.Context, db database.DBTX, f FiltroTTDD, p pagination.Params) ([]ClassificacaoTTDD, int64, error)
	GetTTDD(ctx context.Context, db database.DBTX, codigo string) (ClassificacaoTTDD, error)
	EstruturaTTDD(ctx context.Context, db database.DBTX) ([]EstruturaTTDD, error)
	// ProcedimentosPorOrgao conta os procedimentos de cada órgão da TTDD.
	ProcedimentosPorOrgao(ctx context.Context, db database.DBTX) ([]ProcedimentosOrgao, error)
	// Organograma: secretaria → função → subfunção com séries e procedimentos.
	Organograma(ctx context.Context, db database.DBTX) ([]OrgaoOrganograma, error)
	// Candidatos devolve os procedimentos ativos (completos) que casam com
	// algum termo da pergunta — base do assistente sobre fluxos (ADR 023).
	Candidatos(ctx context.Context, db database.DBTX, pergunta string, limit int) ([]Workflow, error)
	// CandidatosTTDD devolve séries que casam com algum termo da pergunta.
	CandidatosTTDD(ctx context.Context, db database.DBTX, pergunta string, limit int) ([]ClassificacaoTTDD, error)
	// LockTTDD confere a série (inexistente, vigente ou revogada) e a trava
	// (FOR SHARE) até o fim da transação de cadastro.
	LockTTDD(ctx context.Context, db database.DBTX, codigo string) (SituacaoTTDD, error)
	// CargaTTDD compara a carga com o banco (séries novas, alteradas,
	// revogadas, restabelecidas e procedimentos afetados) e, com aplicar, grava
	// — na transação db (a simulação é desfeita por quem chama).
	CargaTTDD(ctx context.Context, db database.DBTX, c CargaTTDD, aplicar bool) (ImpactoCarga, error)
	// Biblioteca de modelos (ADR 024).
	ListModelos(ctx context.Context, db database.DBTX, incluirInativos bool) ([]Modelo, error)
	// GetModelo devolve o modelo com o histórico de versões.
	GetModelo(ctx context.Context, db database.DBTX, id uuid.UUID) (Modelo, error)
	InsertModelo(ctx context.Context, db database.DBTX, m Modelo, v ModeloVersao, por string) error
	// InsertModeloVersao grava a versão seguinte (trava o modelo) e devolve o número dela.
	InsertModeloVersao(ctx context.Context, db database.DBTX, id uuid.UUID, v ModeloVersao) (int, error)
	UpdateModelo(ctx context.Context, db database.DBTX, m Modelo, por string) error
	// VersaoModelo devolve uma versão (0 = a atual) com a chave do arquivo —
	// também de modelo desativado (as peças já ligadas continuam baixando).
	VersaoModelo(ctx context.Context, db database.DBTX, id uuid.UUID, versao int) (ModeloVersao, error)
	// ModelosAtivos devolve quantos dos ids são modelos ativos.
	ModelosAtivos(ctx context.Context, db database.DBTX, ids []uuid.UUID) (int, error)
	// ModeloDaPeca trava a peça do procedimento e devolve o modelo ligado
	// (ErrPecaNaoEncontrada se a peça não é desse procedimento).
	ModeloDaPeca(ctx context.Context, db database.DBTX, workflowID, docID uuid.UUID) (*uuid.UUID, error)
	// SetModeloPeca liga a peça a um modelo (nil desliga).
	SetModeloPeca(ctx context.Context, db database.DBTX, docID uuid.UUID, modeloID *uuid.UUID) error
	// ModelosDaSerie lista os modelos ligados à série da TTDD (por nome).
	ModelosDaSerie(ctx context.Context, db database.DBTX, codigo string) ([]Modelo, error)
	// LigarModeloSerie liga o modelo à série (repetir não duplica).
	LigarModeloSerie(ctx context.Context, db database.DBTX, codigo string, modeloID uuid.UUID, por string) error
	// DesligarModeloSerie retira o modelo da série (ErrModeloNaoEncontrado se não estava ligado).
	DesligarModeloSerie(ctx context.Context, db database.DBTX, codigo string, modeloID uuid.UUID) error
	// Seguidores e destinatários dos avisos de nova versão (ADR 027).
	Seguir(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) error
	DeixarDeSeguir(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) error
	Seguindo(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) (bool, error)
	Interessados(ctx context.Context, db database.DBTX, codigo string, siglas []string) ([]uuid.UUID, error)
	InteressadosModelo(ctx context.Context, db database.DBTX, modeloID uuid.UUID) ([]uuid.UUID, error)
	// Validação dos procedimentos (ADR 028).
	SetSituacao(ctx context.Context, db database.DBTX, id uuid.UUID, situacao string, ativo bool) error
	Validacoes(ctx context.Context, db database.DBTX, codigo string) ([]Validacao, error)
	InsertValidacao(ctx context.Context, db database.DBTX, v Validacao) error
	// Cobertura monta o painel de cobertura (ADR 025).
	Cobertura(ctx context.Context, db database.DBTX) (Cobertura, error)
	// HistoricoTTDD devolve as mudanças da série, da mais recente à mais antiga.
	HistoricoTTDD(ctx context.Context, db database.DBTX, codigo string) ([]HistoricoTTDD, error)

	List(ctx context.Context, db database.DBTX, f Filter, p pagination.Params) ([]Workflow, int64, error)
	// Get devolve o procedimento completo (etapas, peças e transições).
	Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (Workflow, error)
	// Insert grava o procedimento com etapas, peças e transições.
	Insert(ctx context.Context, db database.DBTX, w Workflow) error
	SetAtivo(ctx context.Context, db database.DBTX, id uuid.UUID, ativo bool) error
	// MaxVersao devolve a maior versão do código processual (0 se nenhuma),
	// travando as versões existentes até o fim da transação.
	MaxVersao(ctx context.Context, db database.DBTX, codigo string) (int, error)
	// DesativarVersoes desativa as versões ativas do código, exceto `exceto`,
	// e devolve as que mudaram.
	DesativarVersoes(ctx context.Context, db database.DBTX, codigo string, exceto uuid.UUID) ([]uuid.UUID, error)
	// Search é a busca full-text (ativos) com o rank de cada resultado.
	Search(ctx context.Context, db database.DBTX, query string, limit int) ([]Workflow, []float64, error)
}

// Assistente redige a orientação a partir EXCLUSIVAMENTE do contexto
// fornecido — as sínteses canônicas dos procedimentos e das séries da TTDD
// selecionados (porta implementada por um LLM compatível com a API OpenAI).
type Assistente interface {
	Responder(ctx context.Context, pergunta string, contexto []string) (string, error)
}
