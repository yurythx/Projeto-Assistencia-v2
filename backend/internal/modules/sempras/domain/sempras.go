// Package domain contém as regras do SEMPRAS (ADR 030): identificação do
// munícipe, ciclo de vida da fila de atendimento, prioridade legal, escopo
// por unidade e sigilo psicológico. Não conhece o banco.
package domain

import (
	"errors"
	"fmt"
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
	ErrServicoIndisponivel    = errors.New("sempras: esta unidade não oferta este serviço")
	ErrCasaMulherEstado       = errors.New("sempras: o caso da Casa da Mulher não está no estado que a ação exige")
	ErrCasaMulherJaAtivo      = errors.New("sempras: a mulher já tem um caso ativo nesta unidade")
	ErrAcolhimentoAtivo       = errors.New("sempras: a criança já tem um acolhimento ativo")
	ErrVisitaEncerrada        = errors.New("sempras: a saída deste visitante já foi registrada")
	ErrRelatorioEntregue      = errors.New("sempras: relatório entregue não pode ser alterado nem excluído")
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
	Sexo           string    `json:"sexo"`            // feminino, masculino ou "" (não informado)
	NomeMae        string    `json:"nome_mae"`
	Telefone       string    `json:"telefone"`
	Endereco       string    `json:"endereco"`
	Anonimo        bool      `json:"anonimo"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CidadaoInput são os campos editáveis do cadastro. Anonimo é a 4ª via
// de identificação (Centro POP — abordagem de rua às vezes não consegue
// nem o fallback de nascimento+mãe): dispensa CPF/NIS/nascimento/mãe,
// mas não dispensa um nome — mesmo que seja um apelido ou "Cidadão não
// identificado", escolha de quem cadastra, não um padrão escondido.
type CidadaoInput struct {
	Nome, NomeSocial, CPF, NIS, DataNascimento, NomeMae, Telefone, Endereco string
	Sexo                                                                    string
	Anonimo                                                                 bool
}

// Sexos aceitos no cadastro ("" = não informado).
var Sexos = []string{"", "feminino", "masculino"}

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
	if c.Sexo = strings.TrimSpace(in.Sexo); !slices.Contains(Sexos, c.Sexo) {
		return Cidadao{}, invalid("sexo inválido (feminino, masculino ou não informado)")
	}
	if c.CPF == "" && c.NIS == "" && (c.DataNascimento == "" || c.NomeMae == "") && !in.Anonimo {
		return Cidadao{}, invalid("informe CPF, NIS ou data de nascimento e nome da mãe (ou marque cadastro anônimo)")
	}
	c.Anonimo = in.Anonimo
	return c, nil
}

// Unidade é uma unidade de atendimento (Unidade do IAM). Tipo e
// PainelExibeNome vêm de sempras_unidades (ADR 031): sem linha lá, Tipo é
// "" e a unidade não oferta serviço nenhum.
type Unidade struct {
	ID              uuid.UUID `json:"id"`
	EntidadeID      uuid.UUID `json:"entidade_id"`
	Nome            string    `json:"nome"`
	Sigla           string    `json:"sigla"`
	Ativa           bool      `json:"ativa"`
	Tipo            string    `json:"tipo"`
	PainelExibeNome bool      `json:"painel_exibe_nome"`
	Servicos        []string  `json:"servicos"`
	Demandas        []string  `json:"demandas"`
}

// Tipos de unidade (ADR 031 §1) — a Tipificação Nacional (CNAS 109/2009).
const (
	UnidadeCras            = "cras"
	UnidadeScfv            = "scfv"
	UnidadeCreas           = "creas"
	UnidadeCentroPop       = "centro_pop"
	UnidadeCasaMulher      = "casa_mulher"
	UnidadeCasaAbrigo      = "casa_abrigo"
	UnidadeConselhoTutelar = "conselho_tutelar"
)

// Serviços que uma unidade pode ofertar (ADR 031 §2). Cada caso de uso
// declara o seu, e a aplicação recusa o que o tipo da unidade não oferta.
const (
	ServicoFila           = "fila"           // recepção, fila do dia e painel de chamada
	ServicoProntuario     = "prontuario"     // evolução social/psicológica
	ServicoEncaminhamento = "encaminhamento" // entre unidades da SEMPRAS
	ServicoBeneficio      = "beneficio"      // benefício eventual (LOAS art. 22)
	ServicoPaif           = "paif"
	ServicoOficio         = "oficio"
	ServicoColetivo       = "coletivo"
	ServicoPaefi          = "paefi"
	ServicoMse            = "mse"
	ServicoNotificacao    = "notificacao" // notificação de ente externo (CREAS)
	ServicoCentroPop      = "centro_pop"  // perfil de rua, documentos e encaminhamento externo
	ServicoAcolhimento    = "acolhimento" // acolhimento institucional (Casa Abrigo)
	ServicoConselho       = "conselho"    // medida protetiva (art. 101 do ECA)
	ServicoCasaMulher     = "casa_mulher" // caso de violência doméstica (Lei 11.340/2006)
)

// servicosPorTipo é o catálogo do ADR 031 §2.
var servicosPorTipo = map[string][]string{
	UnidadeCras: {ServicoFila, ServicoProntuario, ServicoEncaminhamento, ServicoBeneficio, ServicoPaif, ServicoOficio,
		ServicoColetivo},
	UnidadeScfv:            {ServicoFila, ServicoProntuario, ServicoEncaminhamento, ServicoColetivo},
	UnidadeCreas:           {ServicoFila, ServicoProntuario, ServicoEncaminhamento, ServicoPaefi, ServicoMse, ServicoNotificacao},
	UnidadeCentroPop:       {ServicoFila, ServicoProntuario, ServicoEncaminhamento, ServicoBeneficio, ServicoCentroPop},
	UnidadeCasaMulher:      {ServicoFila, ServicoProntuario, ServicoEncaminhamento, ServicoCasaMulher},
	UnidadeCasaAbrigo:      {ServicoProntuario, ServicoEncaminhamento, ServicoAcolhimento},
	UnidadeConselhoTutelar: {ServicoFila, ServicoProntuario, ServicoEncaminhamento, ServicoConselho},
}

// TiposUnidade são os tipos válidos (a ordem é a de exibição).
var TiposUnidade = []string{UnidadeCras, UnidadeScfv, UnidadeCreas, UnidadeCentroPop, UnidadeCasaMulher,
	UnidadeCasaAbrigo, UnidadeConselhoTutelar}

// ServicosDoTipo devolve os serviços ofertados pelo tipo (nenhum, para um
// tipo desconhecido ou vazio). Devolve uma cópia: o catálogo é imutável.
func ServicosDoTipo(tipo string) []string {
	return append([]string{}, servicosPorTipo[tipo]...)
}

// DemandaOutra é a demanda livre — exige o detalhe.
const DemandaOutra = "outra"

// demandasPorTipo são os motivos de comparecimento da recepção (ADR 031
// §3), vindos do legado (CrasAttendanceManagement, SocialAssistance,
// CreasAttendanceManagement, ConselhoTutelarManagement).
var demandasPorTipo = map[string][]string{
	UnidadeCras: {"atendimento_tecnico", "cadunico_bolsa_familia", "beneficio_eventual", "encaminhamento_rede",
		"orientacao_geral", DemandaOutra},
	UnidadeScfv: {"inscricao_scfv", "atividade_grupo", "atendimento_tecnico", "orientacao_geral", DemandaOutra},
	UnidadeCreas: {"acolhida_inicial", "atendimento_psicossocial", "orientacao_juridico_social", "acompanhamento_paefi",
		"medida_socioeducativa", DemandaOutra},
	UnidadeCentroPop: {"alimentacao_higiene", "banho_lavanderia", "guarda_pertences", "passagem", "pernoite_abrigo",
		"acolhida_geral", DemandaOutra},
	UnidadeCasaMulher: {"acolhida_inicial", "atendimento_psicossocial", "orientacao_juridico_social", "medida_protetiva",
		DemandaOutra},
	UnidadeConselhoTutelar: {"demanda_espontanea", "atendimento_familia", "acompanhamento_medida", "requisicao_servico",
		DemandaOutra},
}

// DemandasDoTipo devolve as demandas da recepção do tipo (cópia).
func DemandasDoTipo(tipo string) []string {
	return append([]string{}, demandasPorTipo[tipo]...)
}

// ValidarEntrada aplica as regras do tipo da unidade à inclusão na fila
// (ADR 031 §3): a demanda é da lista do tipo ("outra" pede o detalhe) e a
// identificação é a exigida — o CRAS referencia a família no CadÚnico (CPF
// ou NIS); o cadastro anônimo só é atendido no Centro POP (Decreto
// 7.053/2009). Devolve demanda e detalhe normalizados.
func (u Unidade) ValidarEntrada(c Cidadao, demanda, detalhe string) (string, string, error) {
	demanda = strings.TrimSpace(demanda)
	if !slices.Contains(demandasPorTipo[u.Tipo], demanda) {
		return "", "", invalid("demanda inválida para esta unidade")
	}
	detalhe, err := texto("detalhe da demanda", detalhe, 0, 200)
	if err != nil {
		return "", "", err
	}
	if demanda == DemandaOutra && detalhe == "" {
		return "", "", invalid("descreva a demanda")
	}
	switch {
	case u.Tipo == UnidadeCras && c.CPF == "" && c.NIS == "":
		return "", "", invalid("o CRAS exige CPF ou NIS do munícipe (referência no CadÚnico)")
	case c.Anonimo && u.Tipo != UnidadeCentroPop:
		return "", "", invalid("cadastro anônimo só é atendido no Centro POP — identifique o munícipe")
	}
	return demanda, detalhe, nil
}

// Oferta reporta se a unidade oferta o serviço.
func (u Unidade) Oferta(servico string) bool {
	return slices.Contains(servicosPorTipo[u.Tipo], servico)
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
	Demanda        string     `json:"demanda"`
	DemandaDetalhe string     `json:"demanda_detalhe"`
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
	Cidadao                 Cidadao                 `json:"cidadao"`
	Atendimentos            []Atendimento           `json:"atendimentos"`
	Registros               []Registro              `json:"registros"`
	RedeApoio               []ContatoRedeApoio      `json:"rede_apoio"`
	Beneficios              []Beneficio             `json:"beneficios"`
	Encaminhamentos         []Encaminhamento        `json:"encaminhamentos"`
	PaifVinculos            []PaifVinculo           `json:"paif_vinculos"`
	ConselhoAtendimentos    []ConselhoAtendimento   `json:"conselho_atendimentos"`
	PaefiCasos              []PaefiCaso             `json:"paefi_casos"`
	MseMedidas              []MseMedida             `json:"mse_medidas"`
	NotificacoesCreas       []NotificacaoCreas      `json:"notificacoes_creas"`
	OficiosCras             []OficioCras            `json:"oficios_cras"`
	CasaAbrigoAcolhimentos  []Acolhimento           `json:"casa_abrigo_acolhimentos"`
	CentroPopPerfil         *CentroPopPerfil        `json:"centro_pop_perfil"`
	EncaminhamentosExternos []EncaminhamentoExterno `json:"encaminhamentos_externos"`
	CentroPopDocumentos     []DocumentoCentroPop    `json:"centro_pop_documentos"`
	CasaMulherCasos         []CasoCasaMulher        `json:"casa_mulher_casos"`
	Abordagens              []Abordagem             `json:"abordagens"`
	// Fora da área de quem consulta (ADR 031 §4): só o resumo das outras
	// unidades e a evolução social das que encaminharam o munícipe para a
	// área — o psicológico e os registros especializados nunca cruzam.
	OutrasUnidades  []Acompanhamento `json:"outras_unidades"`
	RegistrosOrigem []Registro       `json:"registros_origem"`
}

// Acompanhamento é o que o munícipe tem numa unidade fora da área de quem
// consulta: a unidade e os serviços em curso, nunca o conteúdo.
type Acompanhamento struct {
	UnidadeID   uuid.UUID `json:"unidade_id"`
	UnidadeNome string    `json:"unidade_nome"`
	Tipo        string    `json:"tipo"`
	// Ativos: "paif", "paefi", "mse", "acolhimento", "conselho" (em curso)
	// e "atendimento" (passou pela fila nos últimos 180 dias).
	Ativos []string `json:"ativos"`
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
	CasaAbrigoCriancasAtivas          int     `json:"casa_abrigo_criancas_ativas"`
	CasaAbrigoAdolescentesAtivos      int     `json:"casa_abrigo_adolescentes_ativos"`
	CasaAbrigoReintegracoesNoMes      int     `json:"casa_abrigo_reintegracoes_no_mes"`
	CasaAbrigoAdocoesNoMes            int     `json:"casa_abrigo_adocoes_no_mes"`
	CasaAbrigoTransferenciasNoMes     int     `json:"casa_abrigo_transferencias_no_mes"`
	CasaAbrigoPiasAtrasados           int     `json:"casa_abrigo_pias_atrasados"`
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

	// Recepção (ADR 031 §3): entradas na fila no mês, por demanda.
	AtendimentosPorDemanda map[string]int `json:"atendimentos_por_demanda"`
	// Pessoas diferentes atendidas no mês por "sexo|faixa" (o recorte do
	// MDS); sexo "" ou faixa "" = não informado.
	PessoasPorSexoFaixa map[string]int `json:"pessoas_por_sexo_faixa"`

	// RMA do CREAS (ADR 031, fase E) — PAEFI e MSE em meio aberto.
	PaefiCasosInicioMes   int            `json:"paefi_casos_inicio_mes"`
	PaefiDesligadosNoMes  int            `json:"paefi_desligados_no_mes"`
	PaefiNovosPorViolacao map[string]int `json:"paefi_novos_por_violacao"`
	MseLaAtivas           int            `json:"mse_la_ativas"`
	MsePscAtivas          int            `json:"mse_psc_ativas"`
	MseNovasLaNoMes       int            `json:"mse_novas_la_no_mes"`
	MseNovasPscNoMes      int            `json:"mse_novas_psc_no_mes"`

	// RMA do Centro POP — pessoas atendidas no mês e o perfil delas.
	PopPessoasAtendidasNoMes     int `json:"pop_pessoas_atendidas_no_mes"`
	PopPessoasNovasNoMes         int `json:"pop_pessoas_novas_no_mes"`
	PopPessoasUsoDrogas          int `json:"pop_pessoas_uso_drogas"`
	PopPessoasComDoenca          int `json:"pop_pessoas_com_doenca"`
	PopPessoasTrecheiras         int `json:"pop_pessoas_trecheiras"`
	EncaminhamentosExternosNoMes int `json:"encaminhamentos_externos_no_mes"`
	AbordagensNoMes              int `json:"abordagens_no_mes"`
	AbordagensAceitouAcolhimento int `json:"abordagens_aceitou_acolhimento"`
	AbordagensRecusas            int `json:"abordagens_recusas"`
	AbordagensPessoasIdentific   int `json:"abordagens_pessoas_identificadas"`

	// Casa da Mulher (indicadores municipais; o MDS não tem RMA próprio).
	CasaMulherCasosAtivos      int `json:"casa_mulher_casos_ativos"`
	CasaMulherCasosNovosNoMes  int `json:"casa_mulher_casos_novos_no_mes"`
	CasaMulherRiscoAltoExtremo int `json:"casa_mulher_risco_alto_extremo"`
	CasaMulherAbrigadas        int `json:"casa_mulher_abrigadas"`
	CasaMulherMedidasDeferidas int `json:"casa_mulher_medidas_deferidas"`
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
	BeneficioNatalidade       = "natalidade"
	BeneficioFuneral          = "funeral"
	BeneficioCestaBasica      = "cesta_basica"
	BeneficioPassagem         = "passagem"
	BeneficioAluguelSocial    = "aluguel_social"
	BeneficioOutro            = "outro"
	BeneficioAuxilioGas       = "auxilio_gas"
	BeneficioCobertorAgasalho = "cobertor_agasalho"
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
	BeneficioAluguelSocial, BeneficioOutro, BeneficioAuxilioGas, BeneficioCobertorAgasalho}

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

// ---------------------------------------------------------- Casa da Mulher

// Formas de violência doméstica e familiar contra a mulher (Lei 11.340/2006,
// art. 7º).
var ViolenciasCasaMulher = []string{"fisica", "psicologica", "sexual", "patrimonial", "moral"}

// Vínculo da mulher com o agressor (art. 5º: âmbito doméstico, familiar ou
// relação íntima de afeto).
var RelacoesAgressor = []string{"conjuge_companheiro", "ex_conjuge_companheiro", "namorado", "ex_namorado", "familiar", "outro"}

// Níveis de risco da avaliação (Formulário Nacional de Avaliação de Risco).
var NiveisRisco = []string{"baixo", "medio", "alto", "extremo"}

// Situações da medida protetiva de urgência (arts. 18 a 24).
var StatusMedidaProtetiva = []string{"nao_solicitada", "em_analise", "deferida", "indeferida", "descumprida"}

// Estados de um caso da Casa da Mulher.
const (
	CasaMulherStatusAtivo     = "ativo"
	CasaMulherStatusEncerrado = "encerrado"
)

// Motivos de encerramento de um caso da Casa da Mulher.
var MotivosEncerramentoCasaMulher = []string{"superacao_violencia", "mudanca_municipio", "transferencia_rede", "desistencia", "outro"}

// CasoCasaMulher é o acompanhamento de uma mulher em situação de violência
// doméstica e familiar pela Casa da Mulher. O relato e as evoluções ficam
// no prontuário (com o sigilo da psicologia); o caso guarda a tipificação,
// o risco, a medida protetiva e o abrigamento sigiloso.
type CasoCasaMulher struct {
	ID                      uuid.UUID  `json:"id"`
	CasoCodigo              string     `json:"caso_codigo"`
	CidadaoID               uuid.UUID  `json:"cidadao_id"`
	UnidadeID               uuid.UUID  `json:"unidade_id"`
	Violencias              []string   `json:"violencias"`
	RelacaoAgressor         string     `json:"relacao_agressor"`
	NivelRisco              string     `json:"nivel_risco"`
	MedidaProtetivaStatus   string     `json:"medida_protetiva_status"`
	MedidaProtetivaProcesso string     `json:"medida_protetiva_processo"`
	PatrulhaMariaDaPenha    bool       `json:"patrulha_maria_da_penha"`
	Filhos                  int        `json:"filhos"`
	Abrigada                bool       `json:"abrigada"`
	DataAbrigamento         *time.Time `json:"data_abrigamento"`
	Status                  string     `json:"status"`
	MotivoEncerramento      string     `json:"motivo_encerramento"`
	TecnicoReferenciaID     *uuid.UUID `json:"tecnico_referencia_id"`
	Observacoes             string     `json:"observacoes"`
	DataAbertura            time.Time  `json:"data_abertura"`
	AtualizadoEm            *time.Time `json:"atualizado_em"`
	DataEncerramento        *time.Time `json:"data_encerramento"`
}

// CasoCasaMulherInput são os dados editáveis do caso.
type CasoCasaMulherInput struct {
	Violencias                                                  []string
	RelacaoAgressor, NivelRisco                                 string
	MedidaProtetivaStatus, MedidaProtetivaProcesso, Observacoes string
	PatrulhaMariaDaPenha, Abrigada                              bool
	Filhos                                                      int
}

// aplicar valida o input e o grava no caso. O abrigamento guarda a data em
// que começou (e a perde quando a mulher sai do abrigo).
func (c *CasoCasaMulher) aplicar(in CasoCasaMulherInput, agora time.Time) error {
	if len(in.Violencias) == 0 {
		return invalid("formas de violência: informe ao menos uma")
	}
	for _, v := range in.Violencias {
		if !slices.Contains(ViolenciasCasaMulher, v) {
			return invalid("forma de violência inválida: %s", v)
		}
	}
	switch {
	case !slices.Contains(RelacoesAgressor, in.RelacaoAgressor):
		return invalid("relação com o agressor inválida")
	case !slices.Contains(NiveisRisco, in.NivelRisco):
		return invalid("nível de risco inválido")
	case !slices.Contains(StatusMedidaProtetiva, in.MedidaProtetivaStatus):
		return invalid("situação da medida protetiva inválida")
	case in.Filhos < 0 || in.Filhos > 30:
		return invalid("filhos: entre 0 e 30")
	}
	processo, err := texto("processo da medida protetiva", in.MedidaProtetivaProcesso, 0, 60)
	if err != nil {
		return err
	}
	obs, err := texto("observações", in.Observacoes, 0, 4000)
	if err != nil {
		return err
	}
	switch {
	case in.Abrigada && !c.Abrigada:
		c.DataAbrigamento = &agora
	case !in.Abrigada:
		c.DataAbrigamento = nil
	}
	c.Violencias, c.RelacaoAgressor, c.NivelRisco = slices.Clone(in.Violencias), in.RelacaoAgressor, in.NivelRisco
	c.MedidaProtetivaStatus, c.MedidaProtetivaProcesso, c.Observacoes = in.MedidaProtetivaStatus, processo, obs
	c.PatrulhaMariaDaPenha, c.Abrigada, c.Filhos = in.PatrulhaMariaDaPenha, in.Abrigada, in.Filhos
	return nil
}

// NovoCasoCasaMulher valida e monta um caso recém-aberto (nasce 'ativo').
func NovoCasoCasaMulher(cidadao, unidade uuid.UUID, in CasoCasaMulherInput, agora time.Time) (CasoCasaMulher, error) {
	id := uuid.New()
	c := CasoCasaMulher{ID: id, CasoCodigo: "CM-" + strings.ToUpper(id.String()[:8]), CidadaoID: cidadao, UnidadeID: unidade,
		Status: CasaMulherStatusAtivo, DataAbertura: agora}
	if err := c.aplicar(in, agora); err != nil {
		return CasoCasaMulher{}, err
	}
	return c, nil
}

// Atualizar regrava a avaliação do caso (risco, medida, abrigamento...) —
// só enquanto ele está ativo.
func (c *CasoCasaMulher) Atualizar(in CasoCasaMulherInput, tecnico uuid.UUID, agora time.Time) error {
	if c.Status != CasaMulherStatusAtivo {
		return ErrCasaMulherEstado
	}
	if err := c.aplicar(in, agora); err != nil {
		return err
	}
	c.TecnicoReferenciaID, c.AtualizadoEm = &tecnico, &agora
	return nil
}

// Encerrar fecha o caso — exige motivo e encerra o abrigamento.
func (c *CasoCasaMulher) Encerrar(motivo string, tecnico uuid.UUID, agora time.Time) error {
	if c.Status != CasaMulherStatusAtivo {
		return ErrCasaMulherEstado
	}
	if !slices.Contains(MotivosEncerramentoCasaMulher, motivo) {
		return invalid("motivo de encerramento inválido")
	}
	c.Status, c.MotivoEncerramento, c.TecnicoReferenciaID, c.DataEncerramento = CasaMulherStatusEncerrado, motivo, &tecnico, &agora
	c.Abrigada, c.DataAbrigamento = false, nil
	return nil
}

// ------------------------------------------- Centro POP: abordagem social

// Resultados de uma abordagem social.
var ResultadosAbordagem = []string{"aceitou_acolhimento", "aceitou_encaminhamento", "orientado", "recusou"}

// Abordagem é uma busca ativa da equipe do Centro POP na rua: onde, quando,
// a quem — o munícipe cadastrado (CidadaoID) ou só uma descrição, quando a
// pessoa não se identifica — e o resultado.
type Abordagem struct {
	ID                uuid.UUID  `json:"id"`
	UnidadeID         uuid.UUID  `json:"unidade_id"`
	CidadaoID         *uuid.UUID `json:"cidadao_id"`
	DescricaoPessoa   string     `json:"descricao_pessoa"`
	Local             string     `json:"local"`
	DataAbordagem     time.Time  `json:"data_abordagem"`
	Resultado         string     `json:"resultado"`
	UsoDrogasAparente bool       `json:"uso_drogas_aparente"`
	DemandaSaude      bool       `json:"demanda_saude"`
	Observacoes       string     `json:"observacoes"`
	CreatedAt         time.Time  `json:"created_at"`
}

// AbordagemInput são os dados de uma abordagem.
type AbordagemInput struct {
	CidadaoID                              *uuid.UUID
	DescricaoPessoa, Local, Resultado, Obs string
	DataAbordagem                          time.Time
	UsoDrogasAparente, DemandaSaude        bool
}

// NovaAbordagem valida e monta uma abordagem.
func NovaAbordagem(unidade uuid.UUID, in AbordagemInput, agora time.Time) (Abordagem, error) {
	desc, err := texto("descrição da pessoa", in.DescricaoPessoa, 0, 300)
	if err != nil {
		return Abordagem{}, err
	}
	if in.CidadaoID == nil && desc == "" {
		return Abordagem{}, invalid("identifique o munícipe ou descreva a pessoa abordada")
	}
	local, err := texto("local", in.Local, 3, 300)
	if err != nil {
		return Abordagem{}, err
	}
	if !slices.Contains(ResultadosAbordagem, in.Resultado) {
		return Abordagem{}, invalid("resultado da abordagem inválido")
	}
	if in.DataAbordagem.IsZero() || in.DataAbordagem.After(agora) {
		return Abordagem{}, invalid("data da abordagem inválida (não pode ser futura)")
	}
	obs, err := texto("observações", in.Obs, 0, 2000)
	if err != nil {
		return Abordagem{}, err
	}
	return Abordagem{ID: uuid.New(), UnidadeID: unidade, CidadaoID: in.CidadaoID, DescricaoPessoa: desc, Local: local,
		DataAbordagem: in.DataAbordagem, Resultado: in.Resultado, UsoDrogasAparente: in.UsoDrogasAparente,
		DemandaSaude: in.DemandaSaude, Observacoes: obs, CreatedAt: agora}, nil
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

// ----------------------------------------------------------------- Centro POP

// Situações habitacionais do perfil de rua.
var SituacoesRua = []string{"situacao_rua", "abrigado_institucionalmente", "egresso_sistema_prisional", "trecheiro", "albergado", "outro"}

// Motivos de vulnerabilidade do perfil de rua.
var MotivosVulnerabilidade = []string{"conflito_familiar", "uso_de_drogas", "alcoolismo", "desemprego", "perda_de_moradia",
	"egresso_sistema_prisional", "migracao", "escolha_propria", "outro"}

// CentroPopPerfil é o perfil de rua de um munícipe atendido no Centro
// POP — extensão 1:1 de Cidadao, não campo solto nele (não faz sentido
// pra uma família do CRAS). Saúde e drogas são sigilosos como o
// registro psicológico: o Prontuario só inclui isso pra quem tem
// sempras:atendimento.
type CentroPopPerfil struct {
	CidadaoID             uuid.UUID  `json:"cidadao_id"`
	Situacao              string     `json:"situacao"`
	TempoRuaAnos          *float64   `json:"tempo_rua_anos"`
	MotivoVulnerabilidade string     `json:"motivo_vulnerabilidade"`
	PossuiDoenca          bool       `json:"possui_doenca"`
	DoencaDescricao       string     `json:"doenca_descricao"`
	UsaMedicacao          bool       `json:"usa_medicacao"`
	MedicacaoDescricao    string     `json:"medicacao_descricao"`
	UsoDrogas             bool       `json:"uso_drogas"`
	DrogasDescricao       string     `json:"drogas_descricao"`
	AtualizadoPor         *uuid.UUID `json:"atualizado_por"`
	AtualizadoEm          time.Time  `json:"atualizado_em"`
}

// NovoCentroPopPerfil valida e monta o perfil de rua (upsert — um por
// cidadão).
func NovoCentroPopPerfil(cidadao uuid.UUID, situacao string, tempoRuaAnos *float64, motivo string,
	possuiDoenca bool, doencaDescricao string, usaMedicacao bool, medicacaoDescricao string,
	usoDrogas bool, drogasDescricao string) (CentroPopPerfil, error) {
	if !slices.Contains(SituacoesRua, situacao) {
		return CentroPopPerfil{}, invalid("situação inválida")
	}
	if !slices.Contains(MotivosVulnerabilidade, motivo) {
		return CentroPopPerfil{}, invalid("motivo de vulnerabilidade inválido")
	}
	if tempoRuaAnos != nil && *tempoRuaAnos < 0 {
		return CentroPopPerfil{}, invalid("tempo de rua não pode ser negativo")
	}
	doenca, err := texto("descrição da doença", doencaDescricao, 0, 1000)
	if err != nil {
		return CentroPopPerfil{}, err
	}
	medicacao, err := texto("descrição da medicação", medicacaoDescricao, 0, 1000)
	if err != nil {
		return CentroPopPerfil{}, err
	}
	drogas, err := texto("descrição do uso de drogas", drogasDescricao, 0, 1000)
	if err != nil {
		return CentroPopPerfil{}, err
	}
	return CentroPopPerfil{CidadaoID: cidadao, Situacao: situacao, TempoRuaAnos: tempoRuaAnos, MotivoVulnerabilidade: motivo,
		PossuiDoenca: possuiDoenca, DoencaDescricao: doenca, UsaMedicacao: usaMedicacao, MedicacaoDescricao: medicacao,
		UsoDrogas: usoDrogas, DrogasDescricao: drogas}, nil
}

// Destinos de um encaminhamento externo (pra fora do sistema).
var DestinosExternos = []string{"caps_ad", "caps_iii", "albergue_noturno", "abrigo_municipal", "saude_upa_hospital",
	"sine_qualificacao", "conselho_tutelar", "policia_civil_deam", "outro"}

// Prioridades de um encaminhamento externo.
var PrioridadesEncaminhamentoExterno = []string{"baixa", "media", "alta", "urgente"}

// Estados de um encaminhamento externo — gerenciado por quem registrou
// (não há "outra ponta" no nosso sistema pra aceitar/recusar, diferente
// de Encaminhamento entre unidades nossas).
const (
	EncaminhamentoExternoStatusPendente      = "pendente"
	EncaminhamentoExternoStatusEmAtendimento = "em_atendimento"
	EncaminhamentoExternoStatusConcluido     = "concluido"
	EncaminhamentoExternoStatusCancelado     = "cancelado"
)

var statusEncaminhamentoExterno = []string{EncaminhamentoExternoStatusPendente, EncaminhamentoExternoStatusEmAtendimento,
	EncaminhamentoExternoStatusConcluido, EncaminhamentoExternoStatusCancelado}

// EncaminhamentoExterno é um registro de que o munícipe foi encaminhado
// pra fora do sistema (CAPS AD, albergue, saúde, polícia etc.) — log,
// não um fluxo de aceite de 2 pontas.
type EncaminhamentoExterno struct {
	ID           uuid.UUID  `json:"id"`
	CidadaoID    uuid.UUID  `json:"cidadao_id"`
	UnidadeID    uuid.UUID  `json:"unidade_id"`
	Destino      string     `json:"destino"`
	Prioridade   string     `json:"prioridade"`
	Motivo       string     `json:"motivo"`
	Status       string     `json:"status"`
	Observacoes  string     `json:"observacoes"`
	TecnicoID    *uuid.UUID `json:"tecnico_id"`
	CreatedAt    time.Time  `json:"created_at"`
	AtualizadoEm time.Time  `json:"atualizado_em"`
}

// NovoEncaminhamentoExterno valida e monta o registro (nasce 'pendente').
func NovoEncaminhamentoExterno(cidadao, unidade uuid.UUID, destino, prioridade, motivo string) (EncaminhamentoExterno, error) {
	if !slices.Contains(DestinosExternos, destino) {
		return EncaminhamentoExterno{}, invalid("destino inválido")
	}
	if !slices.Contains(PrioridadesEncaminhamentoExterno, prioridade) {
		return EncaminhamentoExterno{}, invalid("prioridade inválida (baixa, media, alta ou urgente)")
	}
	m, err := texto("motivo", motivo, 3, 2000)
	if err != nil {
		return EncaminhamentoExterno{}, err
	}
	return EncaminhamentoExterno{ID: uuid.New(), CidadaoID: cidadao, UnidadeID: unidade, Destino: destino,
		Prioridade: prioridade, Motivo: m, Status: EncaminhamentoExternoStatusPendente}, nil
}

// Transitar muda o status (gerenciado por quem atende, sem fluxo de
// aceite — ver comentário do tipo).
func (e *EncaminhamentoExterno) Transitar(status, observacoes string, tecnico uuid.UUID, agora time.Time) error {
	if !slices.Contains(statusEncaminhamentoExterno, status) {
		return invalid("status inválido")
	}
	obs, err := texto("observações", observacoes, 0, 2000)
	if err != nil {
		return err
	}
	e.Status, e.Observacoes, e.TecnicoID, e.AtualizadoEm = status, obs, &tecnico, agora
	return nil
}

// Tipos de documento do Centro POP.
const (
	DocumentoCentroPopIdentificacao = "identificacao"
	DocumentoCentroPopLaudoSaude    = "laudo_saude"
	DocumentoCentroPopOutro         = "outro"
)

var tiposDocumentoCentroPop = []string{DocumentoCentroPopIdentificacao, DocumentoCentroPopLaudoSaude, DocumentoCentroPopOutro}

// DocumentoCentroPop é o metadado de um documento do munícipe atendido
// no Centro POP — mesmo padrão de DocumentoCasaAbrigo. UnidadeID é a
// unidade de quem subiu (Cidadao não tem unidade própria), pro escopo
// de área.
type DocumentoCentroPop struct {
	ID           uuid.UUID `json:"id"`
	CidadaoID    uuid.UUID `json:"cidadao_id"`
	UnidadeID    uuid.UUID `json:"unidade_id"`
	Tipo         string    `json:"tipo"`
	NomeOriginal string    `json:"nome_original"`
	ObjectKey    string    `json:"object_key"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// NovoDocumentoCentroPop valida e monta o metadado pendente.
func NovoDocumentoCentroPop(cidadao, unidade uuid.UUID, tipo, nomeOriginal, objectKey, contentType string) (DocumentoCentroPop, error) {
	if !slices.Contains(tiposDocumentoCentroPop, tipo) {
		return DocumentoCentroPop{}, invalid("tipo de documento inválido (identificacao, laudo_saude ou outro)")
	}
	nome, err := texto("nome do arquivo", nomeOriginal, 1, 255)
	if err != nil {
		return DocumentoCentroPop{}, err
	}
	return DocumentoCentroPop{ID: uuid.New(), CidadaoID: cidadao, UnidadeID: unidade, Tipo: tipo, NomeOriginal: nome,
		ObjectKey: objectKey, ContentType: contentType, Status: DocumentoStatusPending}, nil
}
