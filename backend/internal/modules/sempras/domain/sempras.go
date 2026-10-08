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
	ErrNotFound           = errors.New("sempras: registro não encontrado")
	ErrForbidden          = errors.New("sempras: sem permissão nesta unidade")
	ErrInvalidState       = errors.New("sempras: o atendimento não está no estado que a ação exige")
	ErrFilaVazia          = errors.New("sempras: não há ninguém aguardando na fila")
	ErrJaNaFila           = errors.New("sempras: o munícipe já está na fila desta unidade hoje")
	ErrDocumentoDuplicado = errors.New("sempras: já existe munícipe com este CPF ou NIS")
	ErrUnidadeInativa     = errors.New("sempras: unidade desativada")
	ErrAtendimentoOutro   = errors.New("sempras: o atendimento é de outro munícipe ou de outra unidade")
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

// Prontuario é a ficha do munícipe: cadastro, atendimentos e evoluções da
// área de quem consulta.
type Prontuario struct {
	Cidadao      Cidadao       `json:"cidadao"`
	Atendimentos []Atendimento `json:"atendimentos"`
	Registros    []Registro    `json:"registros"`
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
