// Package domain contém as regras do SEMPRAS (ADR 030): identificação do
// munícipe, ciclo de vida da fila de atendimento, prioridade legal, escopo
// por unidade e sigilo psicológico. Não conhece o banco.
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/logging"
)

var (
	ErrNotFound               = errors.New("sempras: registro não encontrado")
	ErrForbidden              = errors.New("sempras: sem permissão nesta unidade")
	ErrInvalidState           = errors.New("sempras: o atendimento não está no estado que a ação exige")
	ErrFilaVazia              = errors.New("sempras: não há ninguém aguardando na fila")
	ErrJaNaFila               = errors.New("sempras: o munícipe já está na fila desta unidade hoje")
	ErrDocumentoDuplicado     = errors.New("sempras: já existe munícipe com este CPF ou NIS")
	ErrUnidadeInativa         = errors.New("sempras: unidade desativada")
	ErrAtendimentoOutro       = errors.New("sempras: o atendimento é de outro munícipe ou de outra unidade")
	ErrPedidoDecidido         = errors.New("sempras: este pedido não é uma anonimização pendente")
	ErrBeneficioEstado        = errors.New("sempras: o benefício não está no estado que a ação exige")
	ErrEncaminhamentoDecidido = errors.New("sempras: este encaminhamento já foi decidido")
	ErrMesmaUnidade           = errors.New("sempras: a unidade de destino deve ser diferente da de origem")
	ErrPaifJaAtivo            = errors.New("sempras: a família já tem vínculo ativo com este CRAS")
	ErrPaifJaExcluido         = errors.New("sempras: este vínculo já foi excluído")
	ErrConselhoEstado         = errors.New("sempras: o atendimento do Conselho Tutelar não está no estado que a ação exige")
	ErrPaefiEstado            = errors.New("sempras: o caso do PAEFI não está no estado que a ação exige")
	ErrMseEstado              = errors.New("sempras: a medida socioeducativa não está no estado que a ação exige")
	ErrNotificacaoEstado      = errors.New("sempras: a notificação não está no estado que a ação exige")
	ErrAcolhimentoEstado      = errors.New("sempras: o acolhimento não está no estado que a ação exige")
	ErrOficioRespondido       = errors.New("sempras: este ofício já foi respondido")
)

// ValidationError é um dado de entrada inválido (422), com a explicação.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// Tipos de atendimento.
const (
	TipoSocial      = "social"
	TipoPsicologico = "psicologico"
)

// Estados da fila (ADR 030 §4).
const (
	StatusAguardando    = "aguardando"
	StatusChamado       = "chamado"
	StatusEmAtendimento = "em_atendimento"
	StatusConcluido     = "concluido"
	StatusNaoCompareceu = "nao_compareceu"
	StatusCancelado     = "cancelado"
)

// Prioridades legais (Lei 10.048/2000, Lei 12.764/2012 e Lei 13.466/2017).
var Prioridades = []string{"idoso", "idoso_80", "deficiencia", "tea", "gestante", "lactante", "crianca_de_colo", "obeso"}

// transicoes: de onde cada ação pode partir e para onde leva.
var transicoes = map[string]struct {
	de   []string
	para string
}{
	"chamar":   {[]string{StatusAguardando, StatusChamado}, StatusChamado},
	"iniciar":  {[]string{StatusChamado}, StatusEmAtendimento},
	"concluir": {[]string{StatusEmAtendimento}, StatusConcluido},
	"ausente":  {[]string{StatusChamado}, StatusNaoCompareceu},
	"cancelar": {[]string{StatusAguardando, StatusChamado}, StatusCancelado},
}

// Cidadao é o munícipe. O cadastro é único na secretaria.
type Cidadao struct {
	ID             uuid.UUID `json:"id"`
	Nome           string    `json:"nome"`
	NomeSocial     string    `json:"nome_social"`
	CPF            string    `json:"cpf"`
	NIS            string    `json:"nis"`
	DataNascimento string    `json:"data_nascimento"` // AAAA-MM-DD ou ""
	NomeMae        string    `json:"nome_mae"`
	Telefone       string    `json:"telefone"`
	Endereco       string    `json:"endereco"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CidadaoInput são os campos editáveis do cadastro.
type CidadaoInput struct {
	Nome, NomeSocial, CPF, NIS, DataNascimento, NomeMae, Telefone, Endereco string
}

// NomeExibicao respeita o nome social (Decreto 8.727/2016).
func (c Cidadao) NomeExibicao() string {
	if c.NomeSocial != "" {
		return c.NomeSocial
	}
	return c.Nome
}

// Resumo é o que a busca mostra: identificação mínima, documentos
// mascarados e o histórico só da área de quem busca.
type Resumo struct {
	ID                 uuid.UUID `json:"id"`
	Nome               string    `json:"nome"`
	NomeSocial         string    `json:"nome_social"`
	CPF                string    `json:"cpf"`
	NIS                string    `json:"nis"`
	DataNascimento     string    `json:"data_nascimento"`
	AtendimentosNaArea int       `json:"atendimentos_na_area"`
}

// Resumir mascara os documentos do cadastro.
func (c Cidadao) Resumir(historico int) Resumo {
	return Resumo{ID: c.ID, Nome: c.Nome, NomeSocial: c.NomeSocial, CPF: MascararCPF(c.CPF), NIS: MascararNIS(c.NIS),
		DataNascimento: c.DataNascimento, AtendimentosNaArea: historico}
}

// Digitos remove tudo o que não é dígito.
func Digitos(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
}

// CPFValido confere tamanho e dígitos verificadores (11 dígitos).
func CPFValido(cpf string) bool {
	if len(cpf) != 11 || strings.Count(cpf, cpf[:1]) == 11 {
		return false
	}
	for _, n := range []int{9, 10} {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(cpf[i]-'0') * (n + 1 - i)
		}
		dv := soma * 10 % 11 % 10
		if dv != int(cpf[n]-'0') {
			return false
		}
	}
	return true
}

// MascararCPF reaproveita a máscara dos logs (***.456.789-**).
func MascararCPF(cpf string) string {
	if cpf == "" {
		return ""
	}
	return logging.MaskCPF(cpf)
}

// MascararNIS mantém só os 4 últimos dígitos.
func MascararNIS(nis string) string {
	if len(nis) < 4 {
		return ""
	}
	return "*******" + nis[len(nis)-4:]
}

func texto(campo, v string, minimo, maximo int) (string, error) {
	v = strings.Join(strings.Fields(v), " ")
	if n := utf8.RuneCountInString(v); n < minimo || n > maximo {
		if minimo > 0 {
			return "", invalid("%s: entre %d e %d caracteres", campo, minimo, maximo)
		}
		return "", invalid("%s: até %d caracteres", campo, maximo)
	}
	return v, nil
}

// Normalizar valida o cadastro e aplica as regras de identificação.
func (in CidadaoInput) Normalizar(hoje time.Time) (Cidadao, error) {
	var c Cidadao
	var err error
	campos := []struct {
		dst         *string
		nome, valor string
		minimo, max int
	}{
		{&c.Nome, "nome", in.Nome, 2, 200},
		{&c.NomeSocial, "nome social", in.NomeSocial, 0, 200},
		{&c.NomeMae, "nome da mãe", in.NomeMae, 0, 200},
		{&c.Telefone, "telefone", in.Telefone, 0, 30},
		{&c.Endereco, "endereço", in.Endereco, 0, 300},
	}
	for _, f := range campos {
		if *f.dst, err = texto(f.nome, f.valor, f.minimo, f.max); err != nil {
			return Cidadao{}, err
		}
	}
	if c.CPF = Digitos(in.CPF); c.CPF != "" && !CPFValido(c.CPF) {
		return Cidadao{}, invalid("CPF inválido")
	}
	if c.NIS = Digitos(in.NIS); c.NIS != "" && len(c.NIS) != 11 {
		return Cidadao{}, invalid("NIS deve ter 11 dígitos")
	}
	if d := strings.TrimSpace(in.DataNascimento); d != "" {
		nasc, err := time.Parse(time.DateOnly, d)
		if err != nil || nasc.After(hoje) || nasc.Year() < 1900 {
			return Cidadao{}, invalid("data de nascimento inválida (AAAA-MM-DD, não futura)")
		}
		c.DataNascimento = d
	}
	if c.CPF == "" && c.NIS == "" && (c.DataNascimento == "" || c.NomeMae == "") {
		return Cidadao{}, invalid("informe CPF, NIS ou data de nascimento e nome da mãe")
	}
	return c, nil
}

// Unidade é uma unidade de atendimento (Unidade do IAM).
type Unidade struct {
	ID         uuid.UUID `json:"id"`
	EntidadeID uuid.UUID `json:"entidade_id"`
	Nome       string    `json:"nome"`
	Sigla      string    `json:"sigla"`
	Ativa      bool      `json:"ativa"`
}

// Alvo é a posição da unidade para auth.Can: entidade e unidade, para que a
// concessão na entidade cubra mesmo sem a árvore expandida (ADR 030 §1).
func (u Unidade) Alvo() auth.Target {
	return auth.Target{EntidadeID: &u.EntidadeID, UnidadeID: &u.ID}
}

// Pode reporta se identity tem a permissão na unidade.
func (u Unidade) Pode(identity auth.Identity, perm auth.Permission) bool {
	return auth.Can(identity, perm, u.Alvo())
}

// PodeAlguma reporta se identity tem qualquer uma das permissões na unidade.
func (u Unidade) PodeAlguma(identity auth.Identity, perms ...auth.Permission) bool {
	return slices.ContainsFunc(perms, func(p auth.Permission) bool { return u.Pode(identity, p) })
}

// PodePsicologia é o sigilo psicológico (ADR 030 §5): vale só por
// concessão que cubra a unidade — o papel de administrador da plataforma
// não dá acesso a sigilo profissional.
func (u Unidade) PodePsicologia(identity auth.Identity) bool {
	alvo := u.Alvo()
	return slices.ContainsFunc(identity.Scopes, func(s auth.Scope) bool {
		return auth.MatchPermission(s.Permissions, string(auth.PermSemprasPsicologia)) && s.Covers(alvo)
	})
}

// PodeTipo reporta se identity atende o tipo na unidade.
func (u Unidade) PodeTipo(identity auth.Identity, tipo string) bool {
	if tipo == TipoPsicologico {
		return u.PodePsicologia(identity)
	}
	return u.Pode(identity, auth.PermSemprasAtendimento)
}

// TodasAsPermissoes do módulo — qualquer uma dá acesso à fila da unidade.
var TodasAsPermissoes = []auth.Permission{
	auth.PermSemprasRecepcao, auth.PermSemprasAtendimento, auth.PermSemprasPsicologia, auth.PermSemprasGestao,
}

// Area é onde identity tem alguma das permissões — para filtrar no SQL.
type Area struct {
	All       bool
	Entidades []uuid.UUID
	Unidades  []uuid.UUID
}

// Vazia reporta se a área não alcança nada.
func (a Area) Vazia() bool { return !a.All && len(a.Entidades) == 0 && len(a.Unidades) == 0 }

// AreaDe une a cobertura (ADR 013) de cada permissão.
func AreaDe(identity auth.Identity, perms ...auth.Permission) Area {
	var a Area
	for _, p := range perms {
		c := auth.CoverageOf(identity, p)
		a.All = a.All || c.All
		a.Entidades = append(a.Entidades, c.Entidades...)
		a.Unidades = append(a.Unidades, c.Unidades...)
	}
	return a
}

// Atendimento é uma entrada da fila e o atendimento que dela resulta.
type Atendimento struct {
	ID             uuid.UUID  `json:"id"`
	CidadaoID      uuid.UUID  `json:"cidadao_id"`
	UnidadeID      uuid.UUID  `json:"unidade_id"`
	Dia            string     `json:"dia"`
	Senha          int        `json:"senha"`
	Tipo           string     `json:"tipo"`
	Prioridade     string     `json:"prioridade"`
	Status         string     `json:"status"`
	Local          string     `json:"local"`
	IncluidoPor    *uuid.UUID `json:"incluido_por"`
	ProfissionalID *uuid.UUID `json:"profissional_id"`
	CreatedAt      time.Time  `json:"created_at"`
	ChamadoEm      *time.Time `json:"chamado_em"`
	IniciadoEm     *time.Time `json:"iniciado_em"`
	EncerradoEm    *time.Time `json:"encerrado_em"`
}

// SenhaExibida: "P007" para prioridade, "N012" para os demais.
func (a Atendimento) SenhaExibida() string {
	prefixo := "N"
	if a.Prioridade != "" {
		prefixo = "P"
	}
	return fmt.Sprintf("%s%03d", prefixo, a.Senha)
}

// NovaEntrada valida tipo e prioridade de uma inclusão na fila.
func NovaEntrada(cidadao, unidade uuid.UUID, tipo, prioridade string, autor uuid.UUID, dia string) (Atendimento, error) {
	if tipo != TipoSocial && tipo != TipoPsicologico {
		return Atendimento{}, invalid("tipo de atendimento inválido (social ou psicologico)")
	}
	if prioridade != "" && !slices.Contains(Prioridades, prioridade) {
		return Atendimento{}, invalid("prioridade inválida")
	}
	return Atendimento{ID: uuid.New(), CidadaoID: cidadao, UnidadeID: unidade, Dia: dia, Tipo: tipo,
		Prioridade: prioridade, Status: StatusAguardando, IncluidoPor: &autor}, nil
}

// Transitar aplica a ação ao atendimento (estados do ADR 030 §4).
func (a *Atendimento) Transitar(acao string, profissional uuid.UUID, local string, agora time.Time) error {
	t := transicoes[acao]
	if !slices.Contains(t.de, a.Status) {
		return ErrInvalidState
	}
	a.Status = t.para
	switch acao {
	case "chamar":
		a.ProfissionalID, a.ChamadoEm = &profissional, &agora
		if local = strings.TrimSpace(local); local != "" {
			a.Local = local
		}
	case "iniciar":
		a.ProfissionalID, a.IniciadoEm = &profissional, &agora
	default:
		a.EncerradoEm = &agora
	}
	return nil
}

// FilaItem é a linha da fila na tela: só o necessário para chamar.
type FilaItem struct {
	Atendimento
	SenhaExibida string `json:"senha_exibida"`
	Nome         string `json:"nome"`
}

// Registro é uma evolução do prontuário (só de inclusão).
type Registro struct {
	ID            uuid.UUID  `json:"id"`
	CidadaoID     uuid.UUID  `json:"cidadao_id"`
	UnidadeID     uuid.UUID  `json:"unidade_id"`
	AtendimentoID *uuid.UUID `json:"atendimento_id"`
	Tipo          string     `json:"tipo"`
	Texto         string     `json:"texto"`
	Sigiloso      bool       `json:"sigiloso"`
	AutorID       *uuid.UUID `json:"autor_id"`
	AutorNome     string     `json:"autor_nome"`
	CreatedAt     time.Time  `json:"created_at"`
}

// NovoRegistro valida a evolução.
func NovoRegistro(cidadao, unidade uuid.UUID, atendimento *uuid.UUID, tipo, txt string, autor uuid.UUID) (Registro, error) {
	if tipo != TipoSocial && tipo != TipoPsicologico {
		return Registro{}, invalid("tipo de registro inválido (social ou psicologico)")
	}
	txt = strings.TrimSpace(txt)
	if n := utf8.RuneCountInString(txt); n == 0 || n > 20000 {
		return Registro{}, invalid("texto: entre 1 e 20000 caracteres")
	}
	return Registro{ID: uuid.New(), CidadaoID: cidadao, UnidadeID: unidade, AtendimentoID: atendimento,
		Tipo: tipo, Texto: txt, AutorID: &autor}, nil
}

// Ocultar esconde o texto de um registro psicológico de quem não tem o
// sigilo — fica a data e o autor, para que se saiba que existe.
func (r Registro) Ocultar() Registro {
	r.Texto, r.Sigiloso = "", true
	return r
}

// Prontuario é a ficha do munícipe: cadastro, atendimentos, evoluções da
// área de quem consulta e a rede de apoio (secretaria-wide, como o cadastro).
type Prontuario struct {
	Cidadao                Cidadao               `json:"cidadao"`
	Atendimentos           []Atendimento         `json:"atendimentos"`
	Registros              []Registro            `json:"registros"`
	RedeApoio              []ContatoRedeApoio    `json:"rede_apoio"`
	Beneficios             []Beneficio           `json:"beneficios"`
	Encaminhamentos        []Encaminhamento      `json:"encaminhamentos"`
	PaifVinculos           []PaifVinculo         `json:"paif_vinculos"`
	ConselhoAtendimentos   []ConselhoAtendimento `json:"conselho_atendimentos"`
	PaefiCasos             []PaefiCaso           `json:"paefi_casos"`
	MseMedidas             []MseMedida           `json:"mse_medidas"`
	NotificacoesCreas      []NotificacaoCreas    `json:"notificacoes_creas"`
	OficiosCras            []OficioCras          `json:"oficios_cras"`
	CasaAbrigoAcolhimentos []Acolhimento         `json:"casa_abrigo_acolhimentos"`
}

// ContatoRedeApoio é uma pessoa de apoio do munícipe (família, vizinho,
// referência) — não é unidade-específico, como o cadastro.
type ContatoRedeApoio struct {
	ID         uuid.UUID `json:"id"`
	CidadaoID  uuid.UUID `json:"cidadao_id"`
	Nome       string    `json:"nome"`
	Parentesco string    `json:"parentesco"`
	Telefone   string    `json:"telefone"`
	CreatedAt  time.Time `json:"created_at"`
}

// NovoContatoRedeApoio valida um contato da rede de apoio.
func NovoContatoRedeApoio(cidadao uuid.UUID, nome, parentesco, telefone string) (ContatoRedeApoio, error) {
	n, err := texto("nome", nome, 2, 200)
	if err != nil {
		return ContatoRedeApoio{}, err
	}
	p, err := texto("parentesco", parentesco, 0, 100)
	if err != nil {
		return ContatoRedeApoio{}, err
	}
	t, err := texto("telefone", telefone, 0, 30)
	if err != nil {
		return ContatoRedeApoio{}, err
	}
	return ContatoRedeApoio{ID: uuid.New(), CidadaoID: cidadao, Nome: n, Parentesco: p, Telefone: t}, nil
}

// PainelUnidade são os números do dia de uma unidade (gestão).
type PainelUnidade struct {
	Unidade         Unidade `json:"unidade"`
	Aguardando      int     `json:"aguardando"`
	EmAtendimento   int     `json:"em_atendimento"`
	Concluidos      int     `json:"concluidos"`
	NaoCompareceram int     `json:"nao_compareceram"`
	EsperaMediaMin  float64 `json:"espera_media_min"`
}

// RmaUnidade é o Relatório Mensal de Atendimento (RMA/MDS) de uma
// unidade numa competência — consolida indicadores reais (booleans,
// enums, datas), não texto livre casado por substring como faz o
// legado de referência. Os campos "Ativos"/"Acolhidos" são a
// fotografia no fim da competência; os demais são contagens de eventos
// dentro do mês.
type RmaUnidade struct {
	Unidade                           Unidade `json:"unidade"`
	Atendimentos                      int     `json:"atendimentos"`
	AtendimentosSociais               int     `json:"atendimentos_sociais"`
	AtendimentosPsicologicos          int     `json:"atendimentos_psicologicos"`
	PaifAtivos                        int     `json:"paif_ativos"`
	PaifExtremaPobreza                int     `json:"paif_extrema_pobreza"`
	PaifBolsaFamilia                  int     `json:"paif_bolsa_familia"`
	PaifBPC                           int     `json:"paif_bpc"`
	PaifSCFVParticipantes             int     `json:"paif_scfv_participantes"`
	BeneficiosConcedidosNoMes         int     `json:"beneficios_concedidos_no_mes"`
	PaefiCasosAtivos                  int     `json:"paefi_casos_ativos"`
	PaefiCasosAbertosNoMes            int     `json:"paefi_casos_abertos_no_mes"`
	MseAtivas                         int     `json:"mse_ativas"`
	MseConcluidasNoMes                int     `json:"mse_concluidas_no_mes"`
	ConselhoAtendimentosNoMes         int     `json:"conselho_atendimentos_no_mes"`
	CreasNotificacoesRecebidasNoMes   int     `json:"creas_notificacoes_recebidas_no_mes"`
	CreasNotificacoesRespondidasNoMes int     `json:"creas_notificacoes_respondidas_no_mes"`
	CasaAbrigoAcolhidosAtivos         int     `json:"casa_abrigo_acolhidos_ativos"`
	CasaAbrigoEntradasNoMes           int     `json:"casa_abrigo_entradas_no_mes"`
	CasaAbrigoDesligamentosNoMes      int     `json:"casa_abrigo_desligamentos_no_mes"`
	EncaminhamentosEnviadosNoMes      int     `json:"encaminhamentos_enviados_no_mes"`
	OficiosCrasRecebidosNoMes         int     `json:"oficios_cras_recebidos_no_mes"`
	OficiosCrasRespondidosNoMes       int     `json:"oficios_cras_respondidos_no_mes"`
	ColetivoPaifFamiliasNoMes         int     `json:"coletivo_paif_familias_no_mes"`
	ColetivoSCFV0a6NoMes              int     `json:"coletivo_scfv_0_6_no_mes"`
	ColetivoSCFV7a14NoMes             int     `json:"coletivo_scfv_7_14_no_mes"`
	ColetivoSCFV15a17NoMes            int     `json:"coletivo_scfv_15_17_no_mes"`
	ColetivoSCFV18a59NoMes            int     `json:"coletivo_scfv_18_59_no_mes"`
	ColetivoSCFVIdososNoMes           int     `json:"coletivo_scfv_idosos_no_mes"`
	ColetivoPalestrasNoMes            int     `json:"coletivo_palestras_no_mes"`
	ColetivoPCDNoMes                  int     `json:"coletivo_pcd_no_mes"`
	ColetivoTotalNoMes                int     `json:"coletivo_total_no_mes"`
}

// ------------------------------------- Casa Abrigo: visitantes e termos

// Visitante é uma visita recebida pelo acolhido — existia no legado de
// referência como model e rota prontos, mas nenhuma tela jamais usou.
type Visitante struct {
	ID                  uuid.UUID `json:"id"`
	AcolhimentoID       uuid.UUID `json:"acolhimento_id"`
	NomeVisitante       string    `json:"nome_visitante"`
	DocumentoIdentidade string    `json:"documento_identidade"`
	ParentescoVinculo   string    `json:"parentesco_vinculo"`
	DataVisita          time.Time `json:"data_visita"`
	HoraEntrada         string    `json:"hora_entrada"` // "HH:MM" ou ""
	HoraSaida           string    `json:"hora_saida"`
	Observacoes         string    `json:"observacoes"`
	CreatedAt           time.Time `json:"created_at"`
}

var horaRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func hora(campo, v string) (string, error) {
	if v == "" {
		return "", nil
	}
	if !horaRe.MatchString(v) {
		return "", invalid("%s: formato HH:MM", campo)
	}
	return v, nil
}

// NovoVisitante valida e monta uma visita registrada.
func NovoVisitante(acolhimento uuid.UUID, nomeVisitante, documento, parentesco string, dataVisita time.Time, horaEntrada, horaSaida, observacoes string) (Visitante, error) {
	nome, err := texto("nome do visitante", nomeVisitante, 2, 200)
	if err != nil {
		return Visitante{}, err
	}
	doc, err := texto("documento de identidade", documento, 0, 30)
	if err != nil {
		return Visitante{}, err
	}
	par, err := texto("parentesco/vínculo", parentesco, 0, 100)
	if err != nil {
		return Visitante{}, err
	}
	entrada, err := hora("hora de entrada", horaEntrada)
	if err != nil {
		return Visitante{}, err
	}
	saida, err := hora("hora de saída", horaSaida)
	if err != nil {
		return Visitante{}, err
	}
	obs, err := texto("observações", observacoes, 0, 1000)
	if err != nil {
		return Visitante{}, err
	}
	return Visitante{ID: uuid.New(), AcolhimentoID: acolhimento, NomeVisitante: nome, DocumentoIdentidade: doc,
		ParentescoVinculo: par, DataVisita: dataVisita, HoraEntrada: entrada, HoraSaida: saida, Observacoes: obs}, nil
}

// Tipos de termo da Casa Abrigo.
var TiposTermo = []string{"advertencia", "passeio", "saida", "outro"}

// Termo é um termo de advertência, passeio ou saída do acolhido.
type Termo struct {
	ID                    uuid.UUID `json:"id"`
	AcolhimentoID         uuid.UUID `json:"acolhimento_id"`
	TipoTermo             string    `json:"tipo_termo"`
	ResponsavelRetirada   string    `json:"responsavel_retirada"`
	ParentescoResponsavel string    `json:"parentesco_responsavel"`
	MotivoObservacao      string    `json:"motivo_observacao"`
	DataTermo             time.Time `json:"data_termo"`
	CreatedAt             time.Time `json:"created_at"`
}

// NovoTermo valida e monta um termo registrado.
func NovoTermo(acolhimento uuid.UUID, tipoTermo, responsavelRetirada, parentescoResponsavel, motivoObservacao string, dataTermo time.Time) (Termo, error) {
	if !slices.Contains(TiposTermo, tipoTermo) {
		return Termo{}, invalid("tipo de termo inválido (advertencia, passeio, saida ou outro)")
	}
	resp, err := texto("responsável pela retirada", responsavelRetirada, 0, 200)
	if err != nil {
		return Termo{}, err
	}
	par, err := texto("parentesco do responsável", parentescoResponsavel, 0, 100)
	if err != nil {
		return Termo{}, err
	}
	mot, err := texto("motivo/observação", motivoObservacao, 0, 2000)
	if err != nil {
		return Termo{}, err
	}
	return Termo{ID: uuid.New(), AcolhimentoID: acolhimento, TipoTermo: tipoTermo, ResponsavelRetirada: resp,
		ParentescoResponsavel: par, MotivoObservacao: mot, DataTermo: dataTermo}, nil
}

// --------------------------------------------- Casa Abrigo: PIA

// PiaFicha é a ficha 1:1 do Plano Individual de Atendimento (saúde,
// família, rotina) — o legado de referência espalha isso em 3 tabelas
// (Saúde/Família/Rotina) sem nenhuma usada por tela nenhuma.
type PiaFicha struct {
	AcolhimentoID       uuid.UUID  `json:"acolhimento_id"`
	MatriculaEscolar    string     `json:"matricula_escolar"`
	SerieEscolar        string     `json:"serie_escolar"`
	Escola              string     `json:"escola"`
	Medicamentos        string     `json:"medicamentos"`
	Alergias            string     `json:"alergias"`
	VacinacaoAtualizada bool       `json:"vacinacao_atualizada"`
	CartaoSUS           string     `json:"cartao_sus"`
	ContatoFamiliar     string     `json:"contato_familiar"`
	VisitasAutorizadas  string     `json:"visitas_autorizadas"`
	HistoricoFamiliar   string     `json:"historico_familiar"`
	AtividadesManha     string     `json:"atividades_manha"`
	AtividadesTarde     string     `json:"atividades_tarde"`
	AtividadesNoite     string     `json:"atividades_noite"`
	AtividadesFimSemana string     `json:"atividades_fim_semana"`
	AtualizadoPor       *uuid.UUID `json:"atualizado_por"`
	AtualizadoEm        time.Time  `json:"atualizado_em"`
}

// NovaPiaFicha valida e monta a ficha do PIA.
func NovaPiaFicha(acolhimento uuid.UUID, matricula, serie, escola, medicamentos, alergias string, vacinacaoAtualizada bool,
	cartaoSUS, contatoFamiliar, visitasAutorizadas, historicoFamiliar, manha, tarde, noite, fimSemana string) (PiaFicha, error) {
	campos := []struct {
		nome, valor string
		max         int
	}{
		{"matrícula escolar", matricula, 100}, {"série escolar", serie, 100}, {"escola", escola, 200},
		{"medicamentos", medicamentos, 2000}, {"alergias", alergias, 2000}, {"cartão SUS", cartaoSUS, 30},
		{"contato familiar", contatoFamiliar, 2000}, {"visitas autorizadas", visitasAutorizadas, 2000},
		{"histórico familiar", historicoFamiliar, 4000}, {"atividades da manhã", manha, 1000},
		{"atividades da tarde", tarde, 1000}, {"atividades da noite", noite, 1000}, {"atividades de fim de semana", fimSemana, 1000},
	}
	valores := make([]string, len(campos))
	for i, c := range campos {
		v, err := texto(c.nome, c.valor, 0, c.max)
		if err != nil {
			return PiaFicha{}, err
		}
		valores[i] = v
	}
	return PiaFicha{AcolhimentoID: acolhimento, MatriculaEscolar: valores[0], SerieEscolar: valores[1], Escola: valores[2],
		Medicamentos: valores[3], Alergias: valores[4], VacinacaoAtualizada: vacinacaoAtualizada, CartaoSUS: valores[5],
		ContatoFamiliar: valores[6], VisitasAutorizadas: valores[7], HistoricoFamiliar: valores[8], AtividadesManha: valores[9],
		AtividadesTarde: valores[10], AtividadesNoite: valores[11], AtividadesFimSemana: valores[12]}, nil
}

// Tipos de meta do PIA: plano (necessidade/providências) e intervenção
// (objetivos/estratégias) — o legado de referência trata como 2 tabelas
// com campos equivalentes; aqui é uma lista só, com "tipo" dizendo qual é qual.
const (
	PiaMetaPlano       = "plano"
	PiaMetaIntervencao = "intervencao"
)

// PiaMeta é uma meta do plano ou da intervenção do PIA.
type PiaMeta struct {
	ID            uuid.UUID  `json:"id"`
	AcolhimentoID uuid.UUID  `json:"acolhimento_id"`
	Tipo          string     `json:"tipo"`
	Descricao     string     `json:"descricao"`
	Providencia   string     `json:"providencia"`
	Responsavel   string     `json:"responsavel"`
	Prazo         *time.Time `json:"prazo"`
	Concluida     bool       `json:"concluida"`
	CreatedAt     time.Time  `json:"created_at"`
}

// NovaPiaMeta valida e monta uma meta do PIA.
func NovaPiaMeta(acolhimento uuid.UUID, tipo, descricao, providencia, responsavel string, prazo *time.Time) (PiaMeta, error) {
	if tipo != PiaMetaPlano && tipo != PiaMetaIntervencao {
		return PiaMeta{}, invalid("tipo de meta inválido (plano ou intervencao)")
	}
	desc, err := texto("descrição", descricao, 3, 1000)
	if err != nil {
		return PiaMeta{}, err
	}
	prov, err := texto("providência", providencia, 0, 1000)
	if err != nil {
		return PiaMeta{}, err
	}
	resp, err := texto("responsável", responsavel, 0, 200)
	if err != nil {
		return PiaMeta{}, err
	}
	return PiaMeta{ID: uuid.New(), AcolhimentoID: acolhimento, Tipo: tipo, Descricao: desc, Providencia: prov,
		Responsavel: resp, Prazo: prazo}, nil
}

// CompetenciaMes converte "AAAA-MM" no intervalo [início, fim) do mês.
func CompetenciaMes(competencia string) (time.Time, time.Time, error) {
	inicio, err := time.Parse("2006-01", competencia)
	if err != nil {
		return time.Time{}, time.Time{}, invalid("competência inválida (use AAAA-MM)")
	}
	return inicio, inicio.AddDate(0, 1, 0), nil
}

// -------------------------------------------------------- direitos do titular

// Tipos de pedido do titular (ADR 030 §8, LGPD art. 18).
const (
	PedidoAcesso       = "acesso"
	PedidoCorrecao     = "correcao"
	PedidoAnonimizacao = "anonimizacao"
)

// Estados de um pedido. Acesso e correção nascem já concluídos (quem
// atende resolve na hora, com o titular presente); só a anonimização
// nasce pendente, à espera da gestão.
const (
	PedidoStatusPendente  = "pendente"
	PedidoStatusConcluido = "concluido"
	PedidoStatusNegado    = "negado"
)

var tiposPedido = []string{PedidoAcesso, PedidoCorrecao, PedidoAnonimizacao}

// PedidoTitular é o exercício de um direito do munícipe (art. 18) sobre o
// próprio cadastro — registrado por quem o atende, já que ele não tem
// conta na plataforma.
type PedidoTitular struct {
	ID        uuid.UUID `json:"id"`
	CidadaoID uuid.UUID `json:"cidadao_id"`
	// CidadaoNome é o nome de exibição (NomeExibicao) de quem o pedido é —
	// sem isto, quem decide não saberia de qual munícipe se trata.
	CidadaoNome              string     `json:"cidadao_nome"`
	Tipo                     string     `json:"tipo"`
	Status                   string     `json:"status"`
	JustificativaSolicitacao string     `json:"justificativa_solicitacao"`
	SolicitadoPor            uuid.UUID  `json:"solicitado_por"`
	SolicitadoPorNome        string     `json:"solicitado_por_nome"`
	JustificativaDecisao     string     `json:"justificativa_decisao"`
	DecididoPor              *uuid.UUID `json:"decidido_por"`
	DecididoPorNome          string     `json:"decidido_por_nome"`
	DecididoEm               *time.Time `json:"decidido_em"`
	CreatedAt                time.Time  `json:"created_at"`
}

// NovoPedido valida e monta um pedido recém-registrado (ainda sem decisão).
func NovoPedido(cidadao, solicitante uuid.UUID, tipo, justificativa string) (PedidoTitular, error) {
	if !slices.Contains(tiposPedido, tipo) {
		return PedidoTitular{}, invalid("tipo de pedido inválido (acesso, correcao ou anonimizacao)")
	}
	just, err := texto("justificativa", justificativa, 3, 1000)
	if err != nil {
		return PedidoTitular{}, err
	}
	return PedidoTitular{ID: uuid.New(), CidadaoID: cidadao, Tipo: tipo, Status: PedidoStatusPendente,
		JustificativaSolicitacao: just, SolicitadoPor: solicitante}, nil
}

// JustificativaDecisaoValida confere o tamanho de uma justificativa de
// decisão (negar sempre exige; aprovar aceita vazio).
func JustificativaDecisaoValida(justificativa string, exigida bool) (string, error) {
	just := strings.Join(strings.Fields(justificativa), " ")
	if just == "" {
		if exigida {
			return "", invalid("justificativa: obrigatória para negar o pedido")
		}
		return "", nil
	}
	return texto("justificativa", just, 3, 1000)
}

// ------------------------------------------------------- benefícios eventuais

// Tipos de benefício eventual (LOAS art. 22; Decreto 6.307/2007).
const (
	BeneficioNatalidade    = "natalidade"
	BeneficioFuneral       = "funeral"
	BeneficioCestaBasica   = "cesta_basica"
	BeneficioPassagem      = "passagem"
	BeneficioAluguelSocial = "aluguel_social"
	BeneficioOutro         = "outro"
)

// Formas de acesso de um benefício eventual — como a demanda chegou até a
// unidade (NOB-SUAS), usado no RMA/MDS.
const (
	FormaAcessoDemandaEspontanea             = "demanda_espontanea"
	FormaAcessoBuscaAtiva                    = "busca_ativa"
	FormaAcessoEncaminhamentoRedeSocioass    = "encaminhamento_rede_socioassistencial"
	FormaAcessoEncaminhamentoSistemaGarantia = "encaminhamento_sistema_garantia_direitos"
)

var formasAcessoBeneficio = []string{FormaAcessoDemandaEspontanea, FormaAcessoBuscaAtiva,
	FormaAcessoEncaminhamentoRedeSocioass, FormaAcessoEncaminhamentoSistemaGarantia}

var tiposBeneficio = []string{BeneficioNatalidade, BeneficioFuneral, BeneficioCestaBasica, BeneficioPassagem,
	BeneficioAluguelSocial, BeneficioOutro}

// Estados de um benefício: solicitado → concedido (entregue) ou negado;
// concedido → suspenso, se a condição que o justificava deixar de valer.
const (
	BeneficioStatusSolicitado = "solicitado"
	BeneficioStatusConcedido  = "concedido"
	BeneficioStatusNegado     = "negado"
	BeneficioStatusSuspenso   = "suspenso"
)

var transicoesBeneficio = map[string]struct {
	de   []string
	para string
}{
	"conceder":  {[]string{BeneficioStatusSolicitado}, BeneficioStatusConcedido},
	"negar":     {[]string{BeneficioStatusSolicitado}, BeneficioStatusNegado},
	"suspender": {[]string{BeneficioStatusConcedido}, BeneficioStatusSuspenso},
}

// Beneficio é um benefício eventual concedido (ou em análise) a um
// munícipe. SolicitanteID é quem pediu, quando não é o próprio
// beneficiário (ex.: um parente pedindo auxílio funeral).
type Beneficio struct {
	ID               uuid.UUID  `json:"id"`
	BeneficiarioID   uuid.UUID  `json:"beneficiario_id"`
	SolicitanteID    *uuid.UUID `json:"solicitante_id"`
	UnidadeID        uuid.UUID  `json:"unidade_id"`
	Tipo             string     `json:"tipo"`
	FormaAcesso      string     `json:"forma_acesso"`
	Quantidade       int        `json:"quantidade"`
	Status           string     `json:"status"`
	Observacao       string     `json:"observacao"`
	MotivoDecisao    string     `json:"motivo_decisao"`
	TecnicoDecisaoID *uuid.UUID `json:"tecnico_decisao_id"`
	DataSolicitacao  time.Time  `json:"data_solicitacao"`
	DataDecisao      *time.Time `json:"data_decisao"`
}

// NovoBeneficio valida e monta um pedido de benefício eventual.
func NovoBeneficio(beneficiario uuid.UUID, solicitante *uuid.UUID, unidade uuid.UUID, tipo, formaAcesso string, quantidade int, observacao string) (Beneficio, error) {
	if !slices.Contains(tiposBeneficio, tipo) {
		return Beneficio{}, invalid("tipo de benefício inválido (natalidade, funeral, cesta_basica, passagem, aluguel_social ou outro)")
	}
	if !slices.Contains(formasAcessoBeneficio, formaAcesso) {
		return Beneficio{}, invalid("forma de acesso inválida (demanda_espontanea, busca_ativa, encaminhamento_rede_socioassistencial ou encaminhamento_sistema_garantia_direitos)")
	}
	if quantidade < 1 || quantidade > 100 {
		return Beneficio{}, invalid("quantidade: entre 1 e 100")
	}
	obs, err := texto("observação", observacao, 0, 2000)
	if err != nil {
		return Beneficio{}, err
	}
	return Beneficio{ID: uuid.New(), BeneficiarioID: beneficiario, SolicitanteID: solicitante, UnidadeID: unidade,
		Tipo: tipo, FormaAcesso: formaAcesso, Quantidade: quantidade, Status: BeneficioStatusSolicitado, Observacao: obs}, nil
}

// Decidir aplica conceder/negar/suspender — negar e suspender exigem
// motivo (fica no histórico de por que o benefício foi recusado ou cortado).
func (b *Beneficio) Decidir(acao, motivo string, tecnico uuid.UUID, agora time.Time) error {
	t, ok := transicoesBeneficio[acao]
	if !ok || !slices.Contains(t.de, b.Status) {
		return ErrBeneficioEstado
	}
	just := strings.Join(strings.Fields(motivo), " ")
	if acao != "conceder" && utf8.RuneCountInString(just) < 3 {
		return invalid("motivo: obrigatório para negar ou suspender o benefício")
	}
	b.Status, b.MotivoDecisao, b.TecnicoDecisaoID, b.DataDecisao = t.para, just, &tecnico, &agora
	return nil
}

// --------------------------------------------------------- encaminhamentos

// Estados de um encaminhamento entre unidades (ex.: CRAS → CREAS).
const (
	EncaminhamentoStatusPendente = "pendente"
	EncaminhamentoStatusAceito   = "aceito"
	EncaminhamentoStatusRecusado = "recusado"
)

// Encaminhamento é um caso passado de uma unidade a outra (proteção básica
// para média/alta complexidade, por exemplo). Quem recebe decide.
type Encaminhamento struct {
	ID                uuid.UUID  `json:"id"`
	CidadaoID         uuid.UUID  `json:"cidadao_id"`
	UnidadeOrigemID   uuid.UUID  `json:"unidade_origem_id"`
	UnidadeDestinoID  uuid.UUID  `json:"unidade_destino_id"`
	Motivo            string     `json:"motivo"`
	Status            string     `json:"status"`
	ObservacaoDecisao string     `json:"observacao_decisao"`
	EncaminhadoPor    *uuid.UUID `json:"encaminhado_por"`
	DecididoPor       *uuid.UUID `json:"decidido_por"`
	CreatedAt         time.Time  `json:"created_at"`
	DecididoEm        *time.Time `json:"decidido_em"`
}

// NovoEncaminhamento valida e monta um encaminhamento recém-criado.
func NovoEncaminhamento(cidadao, origem, destino uuid.UUID, motivo string) (Encaminhamento, error) {
	if origem == destino {
		return Encaminhamento{}, ErrMesmaUnidade
	}
	m, err := texto("motivo", motivo, 3, 2000)
	if err != nil {
		return Encaminhamento{}, err
	}
	return Encaminhamento{ID: uuid.New(), CidadaoID: cidadao, UnidadeOrigemID: origem, UnidadeDestinoID: destino,
		Motivo: m, Status: EncaminhamentoStatusPendente}, nil
}

// Decidir aceita ou recusa um encaminhamento pendente.
func (e *Encaminhamento) Decidir(aceitar bool, observacao string, tecnico uuid.UUID, agora time.Time) error {
	if e.Status != EncaminhamentoStatusPendente {
		return ErrEncaminhamentoDecidido
	}
	obs, err := texto("observação", observacao, 0, 1000)
	if err != nil {
		return err
	}
	if aceitar {
		e.Status = EncaminhamentoStatusAceito
	} else {
		e.Status = EncaminhamentoStatusRecusado
	}
	e.ObservacaoDecisao, e.DecididoPor, e.DecididoEm = obs, &tecnico, &agora
	return nil
}

// ------------------------------------------------------------------- PAIF

// Estados de um vínculo PAIF — real, não inferido da presença/ausência de
// data de exclusão (o legado de referência faz isso e falha com dado
// incompleto).
const (
	PaifStatusAtivo    = "ativo"
	PaifStatusExcluido = "excluido"
)

// Tipos de atendimento de um vínculo PAIF.
const (
	PaifAtendimentoParticularizado = "particularizado"
	PaifAtendimentoColetivo        = "coletivo"
	PaifAtendimentoAmbos           = "ambos"
)

var tiposAtendimentoPaif = []string{PaifAtendimentoParticularizado, PaifAtendimentoColetivo, PaifAtendimentoAmbos}

// PaifVinculo é o vínculo de uma família ao Serviço de Proteção e
// Atendimento Integral à Família de um CRAS. ExtremaPobreza/BolsaFamilia/
// BPC são os indicadores de público prioritário do RMA/MDS — flags reais,
// não texto livre casado por substring (o legado faz isso e qualquer erro
// de digitação some do relatório sem avisar).
type PaifVinculo struct {
	ID                uuid.UUID  `json:"id"`
	CidadaoID         uuid.UUID  `json:"cidadao_id"`
	UnidadeID         uuid.UUID  `json:"unidade_id"`
	Status            string     `json:"status"`
	TipoAtendimento   string     `json:"tipo_atendimento"`
	ExtremaPobreza    bool       `json:"extrema_pobreza"`
	BolsaFamilia      bool       `json:"bolsa_familia"`
	BPC               bool       `json:"bpc"`
	SCFVParticipantes int        `json:"scfv_participantes"`
	MotivoExclusao    string     `json:"motivo_exclusao"`
	IncluidoPor       *uuid.UUID `json:"incluido_por"`
	ExcluidoPor       *uuid.UUID `json:"excluido_por"`
	DataInclusao      time.Time  `json:"data_inclusao"`
	DataExclusao      *time.Time `json:"data_exclusao"`
}

// NovoPaifVinculo valida e monta uma inclusão no PAIF (nasce 'ativo').
func NovoPaifVinculo(cidadao, unidade uuid.UUID, tipoAtendimento string, extremaPobreza, bolsaFamilia, bpc bool, scfv int) (PaifVinculo, error) {
	if !slices.Contains(tiposAtendimentoPaif, tipoAtendimento) {
		return PaifVinculo{}, invalid("tipo de atendimento inválido (particularizado, coletivo ou ambos)")
	}
	if scfv < 0 || scfv > 50 {
		return PaifVinculo{}, invalid("participantes do SCFV: entre 0 e 50")
	}
	return PaifVinculo{ID: uuid.New(), CidadaoID: cidadao, UnidadeID: unidade, Status: PaifStatusAtivo, TipoAtendimento: tipoAtendimento,
		ExtremaPobreza: extremaPobreza, BolsaFamilia: bolsaFamilia, BPC: bpc, SCFVParticipantes: scfv}, nil
}

// Excluir encerra o vínculo ativo — a família pode ser reincluída depois
// (um novo vínculo), mas este registro fica congelado como histórico.
func (p *PaifVinculo) Excluir(motivo string, tecnico uuid.UUID, agora time.Time) error {
	if p.Status != PaifStatusAtivo {
		return ErrPaifJaExcluido
	}
	m, err := texto("motivo", motivo, 3, 1000)
	if err != nil {
		return err
	}
	p.Status, p.MotivoExclusao, p.ExcluidoPor, p.DataExclusao = PaifStatusExcluido, m, &tecnico, &agora
	return nil
}

// ---------------------------------------------- Atendimentos coletivos (SCFV)

// Categorias de atendimento coletivo do SCFV (Bloco III do RMA/MDS) — um
// enum real, não texto livre casado por substring (o legado faz isso:
// um erro de digitação em "event_type" some do relatório sem avisar).
const (
	ColetivoPaifFamilias           = "paif_familias"
	ColetivoSCFV0a6                = "scfv_0_6"
	ColetivoSCFV7a14               = "scfv_7_14"
	ColetivoSCFV15a17              = "scfv_15_17"
	ColetivoSCFV18a59              = "scfv_18_59"
	ColetivoSCFVIdosos             = "scfv_idosos"
	ColetivoPalestrasNaoContinuado = "palestras_nao_continuado"
	ColetivoPCD                    = "pcd"
)

var CategoriasColetivo = []string{ColetivoPaifFamilias, ColetivoSCFV0a6, ColetivoSCFV7a14, ColetivoSCFV15a17,
	ColetivoSCFV18a59, ColetivoSCFVIdosos, ColetivoPalestrasNaoContinuado, ColetivoPCD}

// AtendimentoColetivo é um evento de grupo (SCFV) da unidade — não é por
// munícipe. Sem transição de estado: é um registro de log, como no
// legado (sem workflow, só criação).
type AtendimentoColetivo struct {
	ID            uuid.UUID `json:"id"`
	UnidadeID     uuid.UUID `json:"unidade_id"`
	DataEvento    time.Time `json:"data_evento"`
	Categoria     string    `json:"categoria"`
	Participantes int       `json:"participantes"`
	Descricao     string    `json:"descricao"`
	CreatedAt     time.Time `json:"created_at"`
}

// NovoAtendimentoColetivo valida e monta o registro de um evento de grupo.
func NovoAtendimentoColetivo(unidade uuid.UUID, dataEvento time.Time, categoria string, participantes int, descricao string) (AtendimentoColetivo, error) {
	if !slices.Contains(CategoriasColetivo, categoria) {
		return AtendimentoColetivo{}, invalid("categoria inválida")
	}
	if participantes < 0 || participantes > 1000 {
		return AtendimentoColetivo{}, invalid("participantes: entre 0 e 1000")
	}
	desc, err := texto("descrição", descricao, 0, 2000)
	if err != nil {
		return AtendimentoColetivo{}, err
	}
	return AtendimentoColetivo{ID: uuid.New(), UnidadeID: unidade, DataEvento: dataEvento, Categoria: categoria,
		Participantes: participantes, Descricao: desc}, nil
}

// ------------------------------------------------------- Conselho Tutelar

var origensDemanda = []string{"disque_100", "escola", "unidade_saude", "plantao_24h", "demanda_espontanea", "policia"}
var motivosViolacao = []string{"negligencia_abandono", "violencia_fisica", "abuso_exploracao_sexual", "evasao_escolar", "trabalho_infantil", "conflito_familiar"}

// Medidas protetivas do Art. 101 do ECA.
var MedidasProtetivas = []string{"termo_responsabilidade", "inclusao_programa_social", "matricula_escolar_obrigatoria", "tratamento_medico_psicologico", "acolhimento_emergencial"}

// Estados de um atendimento do Conselho Tutelar.
const (
	ConselhoStatusEmAndamento    = "em_andamento"
	ConselhoStatusMedidaAplicada = "medida_aplicada"
	ConselhoStatusEncaminhadoMP  = "encaminhado_mp"
	ConselhoStatusArquivado      = "arquivado"
)

var transicoesConselho = map[string]struct {
	de          []string
	para        string
	exigeMedida bool
}{
	"aplicar_medida": {[]string{ConselhoStatusEmAndamento}, ConselhoStatusMedidaAplicada, true},
	"encaminhar_mp":  {[]string{ConselhoStatusEmAndamento}, ConselhoStatusEncaminhadoMP, false},
	"arquivar":       {[]string{ConselhoStatusEmAndamento, ConselhoStatusMedidaAplicada, ConselhoStatusEncaminhadoMP}, ConselhoStatusArquivado, false},
}

// ConselhoAtendimento é um atendimento do Conselho Tutelar a uma criança
// ou adolescente. O responsável não tem ficha própria — fica em campos
// descritivos.
type ConselhoAtendimento struct {
	ID                  uuid.UUID  `json:"id"`
	Protocolo           string     `json:"protocolo"`
	CriancaID           uuid.UUID  `json:"crianca_id"`
	UnidadeID           uuid.UUID  `json:"unidade_id"`
	ResponsavelNome     string     `json:"responsavel_nome"`
	ResponsavelCPF      string     `json:"responsavel_cpf"`
	ResponsavelTelefone string     `json:"responsavel_telefone"`
	OrigemDemanda       string     `json:"origem_demanda"`
	MotivoViolacao      string     `json:"motivo_violacao"`
	MedidaProtetiva     string     `json:"medida_protetiva"`
	Status              string     `json:"status"`
	Observacoes         string     `json:"observacoes"`
	ConselheiroID       *uuid.UUID `json:"conselheiro_id"`
	CreatedAt           time.Time  `json:"created_at"`
	AtualizadoEm        *time.Time `json:"atualizado_em"`
}

// NovoConselhoAtendimento valida e monta um atendimento recém-registrado
// (nasce 'em_andamento', sem medida — ainda não decidida).
func NovoConselhoAtendimento(crianca, unidade uuid.UUID, respNome, respCPF, respTelefone, origemDemanda, motivoViolacao, observacoes string) (ConselhoAtendimento, error) {
	if !slices.Contains(origensDemanda, origemDemanda) {
		return ConselhoAtendimento{}, invalid("origem da demanda inválida")
	}
	if !slices.Contains(motivosViolacao, motivoViolacao) {
		return ConselhoAtendimento{}, invalid("motivo da violação inválido")
	}
	nome, err := texto("nome do responsável", respNome, 0, 200)
	if err != nil {
		return ConselhoAtendimento{}, err
	}
	cpf := Digitos(respCPF)
	if cpf != "" && !CPFValido(cpf) {
		return ConselhoAtendimento{}, invalid("CPF do responsável inválido")
	}
	tel, err := texto("telefone do responsável", respTelefone, 0, 30)
	if err != nil {
		return ConselhoAtendimento{}, err
	}
	obs, err := texto("observações", observacoes, 0, 4000)
	if err != nil {
		return ConselhoAtendimento{}, err
	}
	id := uuid.New()
	return ConselhoAtendimento{ID: id, Protocolo: "CT-" + strings.ToUpper(id.String()[:8]), CriancaID: crianca, UnidadeID: unidade,
		ResponsavelNome: nome, ResponsavelCPF: cpf, ResponsavelTelefone: tel, OrigemDemanda: origemDemanda,
		MotivoViolacao: motivoViolacao, Status: ConselhoStatusEmAndamento, Observacoes: obs}, nil
}

// Transitar aplica medida (Art. 101 ECA), encaminha ao MP ou arquiva.
func (c *ConselhoAtendimento) Transitar(acao, medida string, conselheiro uuid.UUID, agora time.Time) error {
	t, ok := transicoesConselho[acao]
	if !ok || !slices.Contains(t.de, c.Status) {
		return ErrConselhoEstado
	}
	if t.exigeMedida {
		if !slices.Contains(MedidasProtetivas, medida) {
			return invalid("medida protetiva inválida (Art. 101 do ECA)")
		}
		c.MedidaProtetiva = medida
	}
	c.Status, c.ConselheiroID, c.AtualizadoEm = t.para, &conselheiro, &agora
	return nil
}

// ------------------------------------------------------------------- PAEFI

// Situações de violação de direitos atendidas pelo PAEFI (Tipificação
// Nacional de Serviços Socioassistenciais, Resolução CNAS nº 109/2009).
var ViolacoesPaefi = []string{"violencia_fisica", "violencia_psicologica", "violencia_sexual", "negligencia",
	"abandono", "trabalho_infantil", "discriminacao", "medida_socioeducativa_meio_aberto"}

// Estados de um caso do PAEFI.
const (
	PaefiStatusEmAcompanhamento = "em_acompanhamento"
	PaefiStatusDesligado        = "desligado"
)

// Motivos de desligamento de um caso do PAEFI.
var MotivosEncerramentoPaefi = []string{"objetivos_alcancados", "mudanca_territorio", "nao_localizado", "a_pedido_da_familia", "outro"}

// PaefiCaso é o acompanhamento de uma família ou indivíduo em situação de
// violação de direitos pelo PAEFI (CREAS). CidadaoID é o representante
// da família (ou o indivíduo, quando não há núcleo familiar) — os
// demais membros ficam descritos em MembrosVulneraveis, como no
// cadastro de rede de apoio (não ganham ficha própria).
type PaefiCaso struct {
	ID                  uuid.UUID  `json:"id"`
	CasoCodigo          string     `json:"caso_codigo"`
	CidadaoID           uuid.UUID  `json:"cidadao_id"`
	UnidadeID           uuid.UUID  `json:"unidade_id"`
	Violacoes           []string   `json:"violacoes"`
	MembrosVulneraveis  string     `json:"membros_vulneraveis"`
	PiaElaborado        bool       `json:"pia_elaborado"`
	Status              string     `json:"status"`
	MotivoEncerramento  string     `json:"motivo_encerramento"`
	TecnicoReferenciaID *uuid.UUID `json:"tecnico_referencia_id"`
	Observacoes         string     `json:"observacoes"`
	DataAbertura        time.Time  `json:"data_abertura"`
	DataEncerramento    *time.Time `json:"data_encerramento"`
}

// NovoPaefiCaso valida e monta um caso recém-aberto (nasce 'em_acompanhamento').
func NovoPaefiCaso(cidadao, unidade uuid.UUID, violacoes []string, membrosVulneraveis, observacoes string) (PaefiCaso, error) {
	if len(violacoes) == 0 {
		return PaefiCaso{}, invalid("violações: informe ao menos um tipo")
	}
	for _, v := range violacoes {
		if !slices.Contains(ViolacoesPaefi, v) {
			return PaefiCaso{}, invalid("tipo de violação inválido: %s", v)
		}
	}
	membros, err := texto("membros vulneráveis", membrosVulneraveis, 0, 2000)
	if err != nil {
		return PaefiCaso{}, err
	}
	obs, err := texto("observações", observacoes, 0, 4000)
	if err != nil {
		return PaefiCaso{}, err
	}
	id := uuid.New()
	return PaefiCaso{ID: id, CasoCodigo: "PAEFI-" + strings.ToUpper(id.String()[:8]), CidadaoID: cidadao, UnidadeID: unidade,
		Violacoes: slices.Clone(violacoes), MembrosVulneraveis: membros, Status: PaefiStatusEmAcompanhamento, Observacoes: obs}, nil
}

// MarcarPiaElaborado registra a elaboração do Plano de Acompanhamento
// Individual/Familiar — só enquanto o caso está em acompanhamento.
func (p *PaefiCaso) MarcarPiaElaborado(tecnico uuid.UUID) error {
	if p.Status != PaefiStatusEmAcompanhamento {
		return ErrPaefiEstado
	}
	p.PiaElaborado, p.TecnicoReferenciaID = true, &tecnico
	return nil
}

// Encerrar desliga o caso do PAEFI — exige motivo.
func (p *PaefiCaso) Encerrar(motivo string, tecnico uuid.UUID, agora time.Time) error {
	if p.Status != PaefiStatusEmAcompanhamento {
		return ErrPaefiEstado
	}
	if !slices.Contains(MotivosEncerramentoPaefi, motivo) {
		return invalid("motivo de encerramento inválido")
	}
	p.Status, p.MotivoEncerramento, p.TecnicoReferenciaID, p.DataEncerramento = PaefiStatusDesligado, motivo, &tecnico, &agora
	return nil
}

// --------------------------------------------------------------------- MSE

// Tipos de medida socioeducativa em meio aberto (ECA art. 112, III e VI).
// Quando o adolescente cumpre as duas ao mesmo tempo, são dois registros
// — cada medida com seu próprio acompanhamento de cumprimento.
const (
	MseTipoLiberdadeAssistida         = "liberdade_assistida"
	MseTipoPrestacaoServicoComunidade = "prestacao_servico_comunidade"
)

var TiposMse = []string{MseTipoLiberdadeAssistida, MseTipoPrestacaoServicoComunidade}

// Estados de cumprimento da medida.
const (
	MseStatusEmCumprimento = "em_cumprimento"
	MseStatusCumprida      = "cumprida"
	MseStatusDescumprida   = "descumprida"
	MseStatusExtinta       = "extinta"
)

// ResultadosMse são os desfechos possíveis ao concluir o acompanhamento.
var ResultadosMse = []string{MseStatusCumprida, MseStatusDescumprida, MseStatusExtinta}

// MseMedida é o acompanhamento de uma medida socioeducativa em meio
// aberto aplicada a um adolescente.
type MseMedida struct {
	ID                  uuid.UUID  `json:"id"`
	AdolescenteID       uuid.UUID  `json:"adolescente_id"`
	UnidadeID           uuid.UUID  `json:"unidade_id"`
	NumeroProcesso      string     `json:"numero_processo"`
	VaraOrigem          string     `json:"vara_origem"`
	AtoInfracional      string     `json:"ato_infracional"`
	TipoMedida          string     `json:"tipo_medida"`
	InstituicaoParceira string     `json:"instituicao_parceira"`
	Status              string     `json:"status"`
	TecnicoReferenciaID *uuid.UUID `json:"tecnico_referencia_id"`
	Observacoes         string     `json:"observacoes"`
	DataInicio          time.Time  `json:"data_inicio"`
	PrazoPrevisto       *time.Time `json:"prazo_previsto"`
	DataConclusao       *time.Time `json:"data_conclusao"`
}

// NovaMseMedida valida e monta uma medida recém-registrada (nasce 'em_cumprimento').
func NovaMseMedida(adolescente, unidade uuid.UUID, numeroProcesso, varaOrigem, atoInfracional, tipoMedida, instituicaoParceira string) (MseMedida, error) {
	if !slices.Contains(TiposMse, tipoMedida) {
		return MseMedida{}, invalid("tipo de medida inválido (liberdade_assistida ou prestacao_servico_comunidade)")
	}
	ato, err := texto("ato infracional", atoInfracional, 3, 500)
	if err != nil {
		return MseMedida{}, err
	}
	proc, err := texto("número do processo", numeroProcesso, 0, 100)
	if err != nil {
		return MseMedida{}, err
	}
	vara, err := texto("vara de origem", varaOrigem, 0, 200)
	if err != nil {
		return MseMedida{}, err
	}
	inst, err := texto("instituição parceira", instituicaoParceira, 0, 200)
	if err != nil {
		return MseMedida{}, err
	}
	return MseMedida{ID: uuid.New(), AdolescenteID: adolescente, UnidadeID: unidade, NumeroProcesso: proc, VaraOrigem: vara,
		AtoInfracional: ato, TipoMedida: tipoMedida, InstituicaoParceira: inst, Status: MseStatusEmCumprimento}, nil
}

// Concluir encerra o acompanhamento com um dos desfechos possíveis.
func (m *MseMedida) Concluir(resultado string, tecnico uuid.UUID, agora time.Time) error {
	if m.Status != MseStatusEmCumprimento {
		return ErrMseEstado
	}
	if !slices.Contains(ResultadosMse, resultado) {
		return invalid("resultado inválido (cumprida, descumprida ou extinta)")
	}
	m.Status, m.TecnicoReferenciaID, m.DataConclusao = resultado, &tecnico, &agora
	return nil
}

// ----------------------------------------------------- notificações (CREAS)

// Entidades que notificam o CREAS (ofícios, requisições).
var OrigensNotificacaoCreas = []string{"ministerio_publico", "poder_judiciario", "conselho_tutelar", "disque_100_180", "delegacia", "outro"}

// Estados de uma notificação recebida pelo CREAS.
const (
	NotificacaoStatusRecebida     = "recebida"
	NotificacaoStatusEmDiligencia = "em_diligencia"
	NotificacaoStatusRespondida   = "respondida"
	NotificacaoStatusArquivada    = "arquivada"
)

var transicoesNotificacao = map[string]struct {
	de            []string
	para          string
	exigeResposta bool
}{
	"iniciar_diligencia": {[]string{NotificacaoStatusRecebida}, NotificacaoStatusEmDiligencia, false},
	"responder":          {[]string{NotificacaoStatusRecebida, NotificacaoStatusEmDiligencia}, NotificacaoStatusRespondida, true},
	"arquivar":           {[]string{NotificacaoStatusRecebida, NotificacaoStatusEmDiligencia, NotificacaoStatusRespondida}, NotificacaoStatusArquivada, false},
}

// NotificacaoCreas é um ofício/requisição recebido de um ente externo
// (Ministério Público, Judiciário, Conselho Tutelar, Disque 100/180,
// Delegacia) sobre uma vítima de violação de direitos. A vítima é um
// munícipe já cadastrado (VitimaID) — sem isso não há como vincular a
// notificação a um caso PAEFI/MSE da mesma família.
type NotificacaoCreas struct {
	ID                  uuid.UUID  `json:"id"`
	Protocolo           string     `json:"protocolo"`
	VitimaID            uuid.UUID  `json:"vitima_id"`
	UnidadeID           uuid.UUID  `json:"unidade_id"`
	OrigemEntidade      string     `json:"origem_entidade"`
	Assunto             string     `json:"assunto"`
	Status              string     `json:"status"`
	Resposta            string     `json:"resposta"`
	TecnicoReferenciaID *uuid.UUID `json:"tecnico_referencia_id"`
	DataEntrada         time.Time  `json:"data_entrada"`
	PrazoResposta       *time.Time `json:"prazo_resposta"`
	DataResposta        *time.Time `json:"data_resposta"`
}

// NovaNotificacaoCreas valida e monta uma notificação recém-recebida
// (nasce 'recebida').
func NovaNotificacaoCreas(vitima, unidade uuid.UUID, origemEntidade, assunto string, prazoResposta *time.Time) (NotificacaoCreas, error) {
	if !slices.Contains(OrigensNotificacaoCreas, origemEntidade) {
		return NotificacaoCreas{}, invalid("origem da notificação inválida")
	}
	a, err := texto("assunto", assunto, 3, 2000)
	if err != nil {
		return NotificacaoCreas{}, err
	}
	id := uuid.New()
	return NotificacaoCreas{ID: id, Protocolo: "OF-" + strings.ToUpper(id.String()[:8]), VitimaID: vitima, UnidadeID: unidade,
		OrigemEntidade: origemEntidade, Assunto: a, Status: NotificacaoStatusRecebida, PrazoResposta: prazoResposta}, nil
}

// Transitar inicia diligência, registra a resposta ou arquiva a notificação.
func (n *NotificacaoCreas) Transitar(acao, resposta string, tecnico uuid.UUID, agora time.Time) error {
	t, ok := transicoesNotificacao[acao]
	if !ok || !slices.Contains(t.de, n.Status) {
		return ErrNotificacaoEstado
	}
	if t.exigeResposta {
		r, err := texto("resposta", resposta, 3, 4000)
		if err != nil {
			return err
		}
		n.Resposta, n.DataResposta = r, &agora
	}
	n.Status, n.TecnicoReferenciaID = t.para, &tecnico
	return nil
}

// --------------------------------------------------------------- Ofícios (CRAS)

// Estados de um ofício recebido pelo CRAS.
const (
	OficioStatusPendente   = "pendente"
	OficioStatusRespondido = "respondido"
)

// OficioCras é um ofício recebido de um ente externo (Conselho Tutelar,
// Ministério Público etc.) sobre uma família acompanhada pelo CRAS. Mesmo
// conceito da NotificacaoCreas, mas com os campos próprios do CRAS (nº do
// ofício, data de emissão, nº da resposta).
type OficioCras struct {
	ID                   uuid.UUID  `json:"id"`
	CidadaoID            uuid.UUID  `json:"cidadao_id"`
	UnidadeID            uuid.UUID  `json:"unidade_id"`
	OrgaoEmissor         string     `json:"orgao_emissor"`
	NumeroOficio         string     `json:"numero_oficio"`
	DataOficio           *time.Time `json:"data_oficio"`
	DataRecebimento      time.Time  `json:"data_recebimento"`
	Assunto              string     `json:"assunto"`
	Status               string     `json:"status"`
	NumeroResposta       string     `json:"numero_resposta"`
	DataResposta         *time.Time `json:"data_resposta"`
	TecnicoResponsavelID *uuid.UUID `json:"tecnico_responsavel_id"`
}

// NovoOficioCras valida e monta um ofício recém-recebido (nasce 'pendente').
func NovoOficioCras(cidadao, unidade uuid.UUID, orgaoEmissor, numeroOficio string, dataOficio *time.Time, assunto string) (OficioCras, error) {
	org, err := texto("órgão emissor", orgaoEmissor, 3, 200)
	if err != nil {
		return OficioCras{}, err
	}
	num, err := texto("número do ofício", numeroOficio, 1, 100)
	if err != nil {
		return OficioCras{}, err
	}
	a, err := texto("assunto", assunto, 0, 2000)
	if err != nil {
		return OficioCras{}, err
	}
	return OficioCras{ID: uuid.New(), CidadaoID: cidadao, UnidadeID: unidade, OrgaoEmissor: org, NumeroOficio: num,
		DataOficio: dataOficio, Assunto: a, Status: OficioStatusPendente}, nil
}

// Responder registra a resposta de um ofício pendente.
func (o *OficioCras) Responder(numeroResposta string, dataResposta time.Time, tecnico uuid.UUID) error {
	if o.Status != OficioStatusPendente {
		return ErrOficioRespondido
	}
	num, err := texto("número da resposta", numeroResposta, 1, 100)
	if err != nil {
		return err
	}
	o.Status, o.NumeroResposta, o.DataResposta, o.TecnicoResponsavelID = OficioStatusRespondido, num, &dataResposta, &tecnico
	return nil
}

// --------------------------------------------------------------- Casa Abrigo

// Motivos do acolhimento institucional. Fonte única (o legado de
// referência duplica isso num campo de texto livre e numa tabela
// normalizada que não se sincronizam entre si).
var MotivosAcolhimento = []string{"negligencia_abandono", "violencia_fisica", "violencia_psicologica", "abuso_sexual",
	"orfandade_ausencia_responsavel", "usuario_drogas_responsavel", "situacao_rua", "trabalho_infantil", "outro"}

// Categorias de um diagnóstico de saúde/deficiência do acolhido.
var CategoriasDiagnostico = []string{"doenca", "deficiencia", "transtorno", "outro"}

// Estados de um acolhimento institucional.
const (
	AcolhimentoStatusAcolhido  = "acolhido"
	AcolhimentoStatusDesligado = "desligado"
)

// Acolhimento é a estadia de uma criança/adolescente na Casa Abrigo. O
// responsável (quem tinha a guarda antes da medida protetiva) não tem
// ficha própria — fica em campos descritivos, como no Conselho Tutelar.
// Desligar só pode ser feito pela ação dedicada (Desligar): o legado de
// referência deixa esses campos editáveis também pelo PUT geral, o que
// permite burlar a regra de que só a equipe técnica desliga (RN-14).
type Acolhimento struct {
	ID                             uuid.UUID             `json:"id"`
	CidadaoID                      uuid.UUID             `json:"cidadao_id"`
	UnidadeID                      uuid.UUID             `json:"unidade_id"`
	Sexo                           string                `json:"sexo"`
	ResponsavelNome                string                `json:"responsavel_nome"`
	ResponsavelNIS                 string                `json:"responsavel_nis"`
	ResponsavelTelefone            string                `json:"responsavel_telefone"`
	ResponsavelVinculo             string                `json:"responsavel_vinculo"`
	ResponsavelEndereco            string                `json:"responsavel_endereco"`
	Motivos                        []string              `json:"motivos"`
	ProcessoJudicial               string                `json:"processo_judicial"`
	GuiaAcolhimento                string                `json:"guia_acolhimento"`
	ConselheiroNome                string                `json:"conselheiro_nome"`
	ConselheiroTelefone            string                `json:"conselheiro_telefone"`
	SituacaoJuridica               string                `json:"situacao_juridica"`
	HistoricoAcolhidoAnteriormente bool                  `json:"historico_acolhido_anteriormente"`
	HistoricoIrmaosOutraUnidade    bool                  `json:"historico_irmaos_outra_unidade"`
	Observacoes                    string                `json:"observacoes_gerais"`
	Status                         string                `json:"status"`
	MotivoDesligamento             string                `json:"motivo_desligamento"`
	RegistradoPor                  *uuid.UUID            `json:"registrado_por"`
	DesligadoPor                   *uuid.UUID            `json:"desligado_por"`
	DataEntrada                    time.Time             `json:"data_entrada"`
	DataDesligamento               *time.Time            `json:"data_desligamento"`
	Irmaos                         []Irmao               `json:"irmaos"`
	Diagnosticos                   []Diagnostico         `json:"diagnosticos"`
	Visitantes                     []Visitante           `json:"visitantes"`
	Termos                         []Termo               `json:"termos"`
	Pia                            *PiaFicha             `json:"pia"`
	PiaMetas                       []PiaMeta             `json:"pia_metas"`
	Documentos                     []DocumentoCasaAbrigo `json:"documentos"`
}

// NovoAcolhimento valida e monta um acolhimento recém-registrado (nasce 'acolhido').
func NovoAcolhimento(cidadao, unidade uuid.UUID, sexo string, motivos []string, responsavelNome, responsavelNIS, responsavelTelefone,
	responsavelVinculo, responsavelEndereco, processoJudicial, guiaAcolhimento, conselheiroNome, conselheiroTelefone,
	situacaoJuridica string, historicoAnteriormente, historicoIrmaos bool, observacoes string) (Acolhimento, error) {
	if sexo != "masculino" && sexo != "feminino" {
		return Acolhimento{}, invalid("sexo inválido")
	}
	if len(motivos) == 0 {
		return Acolhimento{}, invalid("motivos do acolhimento: informe ao menos um")
	}
	for _, m := range motivos {
		if !slices.Contains(MotivosAcolhimento, m) {
			return Acolhimento{}, invalid("motivo do acolhimento inválido: %s", m)
		}
	}
	respNome, err := texto("nome do responsável", responsavelNome, 0, 200)
	if err != nil {
		return Acolhimento{}, err
	}
	nis := Digitos(responsavelNIS)
	if nis != "" && len(nis) != 11 {
		return Acolhimento{}, invalid("NIS do responsável: deve ter 11 dígitos")
	}
	respTel, err := texto("telefone do responsável", responsavelTelefone, 0, 30)
	if err != nil {
		return Acolhimento{}, err
	}
	respVinculo, err := texto("vínculo do responsável", responsavelVinculo, 0, 100)
	if err != nil {
		return Acolhimento{}, err
	}
	respEndereco, err := texto("endereço do responsável", responsavelEndereco, 0, 300)
	if err != nil {
		return Acolhimento{}, err
	}
	processo, err := texto("processo judicial", processoJudicial, 0, 200)
	if err != nil {
		return Acolhimento{}, err
	}
	guia, err := texto("guia de acolhimento", guiaAcolhimento, 0, 2000)
	if err != nil {
		return Acolhimento{}, err
	}
	consNome, err := texto("nome do conselheiro", conselheiroNome, 0, 200)
	if err != nil {
		return Acolhimento{}, err
	}
	consTel, err := texto("telefone do conselheiro", conselheiroTelefone, 0, 30)
	if err != nil {
		return Acolhimento{}, err
	}
	situacao, err := texto("situação jurídica", situacaoJuridica, 0, 500)
	if err != nil {
		return Acolhimento{}, err
	}
	obs, err := texto("observações", observacoes, 0, 4000)
	if err != nil {
		return Acolhimento{}, err
	}
	return Acolhimento{ID: uuid.New(), CidadaoID: cidadao, UnidadeID: unidade, Sexo: sexo, Motivos: slices.Clone(motivos),
		ResponsavelNome: respNome, ResponsavelNIS: nis, ResponsavelTelefone: respTel, ResponsavelVinculo: respVinculo,
		ResponsavelEndereco: respEndereco, ProcessoJudicial: processo, GuiaAcolhimento: guia, ConselheiroNome: consNome,
		ConselheiroTelefone: consTel, SituacaoJuridica: situacao, HistoricoAcolhidoAnteriormente: historicoAnteriormente,
		HistoricoIrmaosOutraUnidade: historicoIrmaos, Observacoes: obs, Status: AcolhimentoStatusAcolhido}, nil
}

// Atualizar regrava os campos descritivos do acolhimento (nunca o
// status, o motivo ou a data de desligamento — essa separação é o que
// impede o desligamento de ser feito por fora da ação dedicada, Desligar).
func (a *Acolhimento) Atualizar(sexo string, motivos []string, responsavelNome, responsavelNIS, responsavelTelefone,
	responsavelVinculo, responsavelEndereco, processoJudicial, guiaAcolhimento, conselheiroNome, conselheiroTelefone,
	situacaoJuridica string, historicoAnteriormente, historicoIrmaos bool, observacoes string) error {
	novo, err := NovoAcolhimento(a.CidadaoID, a.UnidadeID, sexo, motivos, responsavelNome, responsavelNIS, responsavelTelefone,
		responsavelVinculo, responsavelEndereco, processoJudicial, guiaAcolhimento, conselheiroNome, conselheiroTelefone,
		situacaoJuridica, historicoAnteriormente, historicoIrmaos, observacoes)
	if err != nil {
		return err
	}
	a.Sexo, a.Motivos, a.ResponsavelNome, a.ResponsavelNIS, a.ResponsavelTelefone = novo.Sexo, novo.Motivos, novo.ResponsavelNome, novo.ResponsavelNIS, novo.ResponsavelTelefone
	a.ResponsavelVinculo, a.ResponsavelEndereco, a.ProcessoJudicial, a.GuiaAcolhimento = novo.ResponsavelVinculo, novo.ResponsavelEndereco, novo.ProcessoJudicial, novo.GuiaAcolhimento
	a.ConselheiroNome, a.ConselheiroTelefone, a.SituacaoJuridica = novo.ConselheiroNome, novo.ConselheiroTelefone, novo.SituacaoJuridica
	a.HistoricoAcolhidoAnteriormente, a.HistoricoIrmaosOutraUnidade, a.Observacoes = novo.HistoricoAcolhidoAnteriormente, novo.HistoricoIrmaosOutraUnidade, novo.Observacoes
	return nil
}

// Desligar encerra o acolhimento — ação restrita à equipe técnica
// (RN-14), sem equivalente no PUT geral de edição.
func (a *Acolhimento) Desligar(motivo string, tecnico uuid.UUID, agora time.Time) error {
	if a.Status != AcolhimentoStatusAcolhido {
		return ErrAcolhimentoEstado
	}
	m, err := texto("motivo do desligamento", motivo, 3, 2000)
	if err != nil {
		return err
	}
	a.Status, a.MotivoDesligamento, a.DesligadoPor, a.DataDesligamento = AcolhimentoStatusDesligado, m, &tecnico, &agora
	return nil
}

// Irmao é um irmão do acolhido, para preservar o vínculo familiar
// mesmo quando não está na mesma unidade (RN-07).
type Irmao struct {
	ID               uuid.UUID  `json:"id"`
	AcolhimentoID    uuid.UUID  `json:"acolhimento_id"`
	Nome             string     `json:"nome"`
	DataNascimento   *time.Time `json:"data_nascimento"`
	EstaAcolhido     bool       `json:"esta_acolhido"`
	UnidadeDescricao string     `json:"unidade_descricao"`
	Observacao       string     `json:"observacao"`
	CreatedAt        time.Time  `json:"created_at"`
}

// NovoIrmao valida e monta um irmão do acolhido.
func NovoIrmao(acolhimento uuid.UUID, nome string, dataNascimento *time.Time, estaAcolhido bool, unidadeDescricao, observacao string) (Irmao, error) {
	n, err := texto("nome", nome, 2, 200)
	if err != nil {
		return Irmao{}, err
	}
	ud, err := texto("unidade do irmão", unidadeDescricao, 0, 200)
	if err != nil {
		return Irmao{}, err
	}
	obs, err := texto("observação", observacao, 0, 1000)
	if err != nil {
		return Irmao{}, err
	}
	return Irmao{ID: uuid.New(), AcolhimentoID: acolhimento, Nome: n, DataNascimento: dataNascimento, EstaAcolhido: estaAcolhido,
		UnidadeDescricao: ud, Observacao: obs}, nil
}

// Diagnostico é um diagnóstico de saúde, deficiência ou transtorno do
// acolhido, normalizado numa lista (RN-12) — não em colunas booleanas
// fixas na tabela principal, como faz o legado de referência.
type Diagnostico struct {
	ID            uuid.UUID `json:"id"`
	AcolhimentoID uuid.UUID `json:"acolhimento_id"`
	Categoria     string    `json:"categoria"`
	Diagnostico   string    `json:"diagnostico"`
	Descricao     string    `json:"descricao"`
	CreatedAt     time.Time `json:"created_at"`
}

// NovoDiagnostico valida e monta um diagnóstico do acolhido.
func NovoDiagnostico(acolhimento uuid.UUID, categoria, diagnostico, descricao string) (Diagnostico, error) {
	if !slices.Contains(CategoriasDiagnostico, categoria) {
		return Diagnostico{}, invalid("categoria de diagnóstico inválida (doenca, deficiencia, transtorno ou outro)")
	}
	d, err := texto("diagnóstico", diagnostico, 2, 255)
	if err != nil {
		return Diagnostico{}, err
	}
	desc, err := texto("descrição", descricao, 0, 2000)
	if err != nil {
		return Diagnostico{}, err
	}
	return Diagnostico{ID: uuid.New(), AcolhimentoID: acolhimento, Categoria: categoria, Diagnostico: d, Descricao: desc}, nil
}

// Tipos de documento da Casa Abrigo (RN-15 do legado de referência).
const (
	DocumentoTipoAcolhido    = "acolhido"
	DocumentoTipoResponsavel = "responsavel"
)

// Estados de um documento — nasce 'pending' (o navegador ainda está
// enviando pro object storage) e passa a 'ready' quando confirmado.
const (
	DocumentoStatusPending = "pending"
	DocumentoStatusReady   = "ready"
)

// DocumentoCasaAbrigo é o metadado de um PDF do acolhido ou do
// responsável — o conteúdo em si mora no object storage (object_key).
type DocumentoCasaAbrigo struct {
	ID            uuid.UUID `json:"id"`
	AcolhimentoID uuid.UUID `json:"acolhimento_id"`
	Tipo          string    `json:"tipo"`
	NomeOriginal  string    `json:"nome_original"`
	ObjectKey     string    `json:"object_key"`
	ContentType   string    `json:"content_type"`
	SizeBytes     int64     `json:"size_bytes"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// NovoDocumentoCasaAbrigo valida e monta o metadado pendente — a chave do
// objeto e o content type vêm do ticket de upload (modkit.NewUpload).
func NovoDocumentoCasaAbrigo(acolhimento uuid.UUID, tipo, nomeOriginal, objectKey, contentType string) (DocumentoCasaAbrigo, error) {
	if tipo != DocumentoTipoAcolhido && tipo != DocumentoTipoResponsavel {
		return DocumentoCasaAbrigo{}, invalid("tipo de documento inválido (acolhido ou responsavel)")
	}
	nome, err := texto("nome do arquivo", nomeOriginal, 1, 255)
	if err != nil {
		return DocumentoCasaAbrigo{}, err
	}
	return DocumentoCasaAbrigo{ID: uuid.New(), AcolhimentoID: acolhimento, Tipo: tipo, NomeOriginal: nome,
		ObjectKey: objectKey, ContentType: contentType, Status: DocumentoStatusPending}, nil
}
