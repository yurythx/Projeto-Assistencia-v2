package domain

import (
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Casa Abrigo: acolhimento institucional de criança e adolescente sob
// medida protetiva (ECA art. 101, VII), com paridade ao sistema que a
// unidade usa hoje (ADR 032). Tudo o que lá é lista de escolha aqui é
// enum — o 4.0 guarda texto livre e conta reintegrações procurando
// palavras nele.

// Respostas das perguntas fechadas ("" = não respondida).
var (
	SimNao       = []string{"", "sim", "nao"}
	SimNaoNaoSei = []string{"", "sim", "nao", "nao_sei"}
)

// MotivosAcolhimento são os 19 motivos do assistente (ADR 032 §1).
var MotivosAcolhimento = []string{"abandono", "abuso_sexual", "ameaca_morte_responsaveis", "ausencia_responsaveis_doenca",
	"ausencia_responsaveis_prisao", "conflito_familiar", "entrega_voluntaria", "exploracao_sexual", "negligencia",
	"orfandade", "responsaveis_drogas", "responsaveis_deficiencia", "responsaveis_transtorno_mental",
	"responsaveis_alcool", "situacao_rua", "trabalho_infantil", "violencia_fisica", "violencia_psicologica", "outro"}

// SituacoesJuridicas são as situações legais da medida protetiva.
var SituacoesJuridicas = []string{"", "emergencial_conselho_tutelar", "adocao_internacional", "adocao_nacional",
	"processo_guarda", "processo_tutela", "poder_familiar_destituido", "poder_familiar_suspenso",
	"reintegracao_familia_extensa", "reintegracao_familia_natural", "sem_previsao"}

// VinculosResponsavel são os vínculos de quem tinha a guarda antes da medida.
var VinculosResponsavel = []string{"", "mae", "pai", "madrasta", "padrasto", "avo_feminino", "avo_masculino", "irma",
	"irmao", "tia", "tio", "prima", "primo", "familia_extensa", "familia_substituta", "instituicao", "sem_parentesco"}

// Listas do perfil da criança e do contexto social.
var (
	IdentidadesGenero  = []string{"", "cisgenero", "transgenero", "nao_binario", "outro"}
	OrientacoesSexuais = []string{"", "heterossexual", "homossexual", "bissexual", "pansexual", "outro"}
	RacasCores         = []string{"", "branca", "preta", "parda", "amarela", "indigena", "quilombola"}
	RelacoesTrabalho   = []string{"", "jovem_aprendiz", "meu_primeiro_trampo", "formal", "informal", "nao"}
	sexosAcolhido      = []string{"masculino", "feminino"}
	sexosResponsavel   = []string{"", "masculino", "feminino"}
)

// TiposDesligamento: as estatísticas contam reintegrações, adoções e
// transferências por aqui (R5), nunca pelo texto do motivo.
var TiposDesligamento = []string{"reintegracao_familia_natural", "reintegracao_familia_extensa", "adocao", "guarda",
	"transferencia", "maioridade", "evasao", "outro"}

// Categorias de um diagnóstico de saúde/deficiência do acolhido.
var CategoriasDiagnostico = []string{"doenca", "deficiencia", "transtorno", "outro"}

// Estados de um acolhimento institucional.
const (
	AcolhimentoStatusAcolhido  = "acolhido"
	AcolhimentoStatusDesligado = "desligado"
)

// Faixas etárias do ECA (art. 2º): criança até 11 anos, adolescente de
// 12 a 17. A Casa Abrigo só recebe menores de 18 (R2).
const (
	IdadeAdolescente = 12
	IdadeMaioridade  = 18
	// PrazoReavaliacaoPia: o PIA é reavaliado no máximo a cada 3 meses
	// (ECA art. 19, §1º) — R7.
	PrazoReavaliacaoPia = 90
	avisoPia            = 10
	avisoRelatorio      = 7
	avisoMeta           = 7
	avisoTarefa         = 3
)

// AcolhimentoDados são os campos descritivos do assistente, os mesmos no
// cadastro e na edição (o desligamento tem a ação própria). Embutido em
// Acolhimento, sai no JSON no mesmo nível.
type AcolhimentoDados struct {
	// 1. criança (nome, CPF, NIS e nascimento estão no cadastro do munícipe)
	Sexo             string `json:"sexo"`
	SituacaoEspecial bool   `json:"situacao_especial"`
	PCD              bool   `json:"pcd"`
	Apelido          string `json:"apelido"`
	IdentidadeGenero string `json:"identidade_genero"`
	OrientacaoSexual string `json:"orientacao_sexual"`
	RacaCor          string `json:"raca_cor"`
	Filiacao1        string `json:"filiacao_1"`
	Filiacao2        string `json:"filiacao_2"`
	// 2. responsável antes da medida
	ResponsavelNaoPossui   bool       `json:"responsavel_nao_possui"`
	ResponsavelNome        string     `json:"responsavel_nome"`
	ResponsavelNomeSocial  string     `json:"responsavel_nome_social"`
	ResponsavelApelido     string     `json:"responsavel_apelido"`
	ResponsavelCPF         string     `json:"responsavel_cpf"`
	ResponsavelNIS         string     `json:"responsavel_nis"`
	ResponsavelNascimento  *time.Time `json:"responsavel_nascimento"`
	ResponsavelSexo        string     `json:"responsavel_sexo"`
	ResponsavelVinculo     string     `json:"responsavel_vinculo"`
	ResponsavelCEP         string     `json:"responsavel_cep"`
	ResponsavelLogradouro  string     `json:"responsavel_logradouro"`
	ResponsavelNumero      string     `json:"responsavel_numero"`
	ResponsavelComplemento string     `json:"responsavel_complemento"`
	ResponsavelBairro      string     `json:"responsavel_bairro"`
	ResponsavelCidade      string     `json:"responsavel_cidade"`
	ResponsavelUF          string     `json:"responsavel_uf"`
	ResponsavelTelefone    string     `json:"responsavel_telefone"`
	ResponsavelCelular     string     `json:"responsavel_celular"`
	ResponsavelEmail       string     `json:"responsavel_email"`
	// 3. acolhimento
	ProcessoJudicial               string     `json:"processo_judicial"`
	ProcessoJudicial2              string     `json:"processo_judicial_2"`
	GuiaAcolhimento                string     `json:"guia_acolhimento"`
	DataGuia                       *time.Time `json:"data_guia"`
	HistoricoAcolhidoAnteriormente string     `json:"historico_acolhido_anteriormente"`
	HistoricoIrmaosNestaUnidade    string     `json:"historico_irmaos_nesta_unidade"`
	HistoricoIrmaosOutraUnidade    string     `json:"historico_irmaos_outra_unidade"`
	Motivos                        []string   `json:"motivos"`
	OutroMotivo                    string     `json:"outro_motivo"`
	// 4. jurídico
	SituacaoJuridica     string     `json:"situacao_juridica"`
	DataSituacaoJuridica *time.Time `json:"data_situacao_juridica"`
	ConselheiroNome      string     `json:"conselheiro_nome"`
	ConselheiroTelefone  string     `json:"conselheiro_telefone"`
	CNA                  string     `json:"cna"`
	CNCAD                string     `json:"cncad"`
	// 5. saúde (o checklist de diagnósticos vai em Diagnosticos)
	LaudoMedico          string `json:"laudo_medico"`
	LaudoMedicoDescricao string `json:"laudo_medico_descricao"`
	MedicacaoContinua    string `json:"medicacao_continua"`
	DependenciaQuimica   string `json:"dependencia_quimica"`
	PossuiTranstorno     string `json:"possui_transtorno"`
	TranstornoDescricao  string `json:"transtorno_descricao"`
	// 6. contexto social
	Estuda           string `json:"estuda"`
	AmeacaMorte      string `json:"ameaca_morte"`
	RecebeVisita     string `json:"recebe_visita"`
	BeneficioGoverno string `json:"beneficio_governo"`
	BeneficioQual    string `json:"beneficio_qual"`
	ProgramaSocial   string `json:"programa_social"`
	RelacaoTrabalho  string `json:"relacao_trabalho"`
	Observacoes      string `json:"observacoes_gerais"`
}

type campoTexto struct {
	dst          *string
	nome         string
	minimo, maxi int
}

func normalizarTextos(campos []campoTexto) error {
	for _, c := range campos {
		v, err := texto(c.nome, *c.dst, c.minimo, c.maxi)
		if err != nil {
			return err
		}
		*c.dst = v
	}
	return nil
}

type campoEnum struct {
	valor  string
	nome   string
	opcoes []string
}

func conferirEnums(campos []campoEnum) error {
	for _, c := range campos {
		if !slices.Contains(c.opcoes, c.valor) {
			return invalid("%s inválido(a)", c.nome)
		}
	}
	return nil
}

// Normalizar valida o assistente e aplica R4: "não possui responsável"
// limpa o responsável; benefício, laudo e transtorno só guardam a
// descrição quando a resposta é "sim".
func (d AcolhimentoDados) Normalizar() (AcolhimentoDados, error) {
	d.Motivos = slices.Clone(d.Motivos)
	d.ResponsavelUF = strings.ToUpper(strings.TrimSpace(d.ResponsavelUF))
	d.ResponsavelEmail = strings.TrimSpace(d.ResponsavelEmail)
	if err := normalizarTextos([]campoTexto{
		{&d.Apelido, "apelido", 0, 100}, {&d.Filiacao1, "filiação 1", 0, 200}, {&d.Filiacao2, "filiação 2", 0, 200},
		{&d.ResponsavelNome, "nome do responsável", 0, 200}, {&d.ResponsavelNomeSocial, "nome social do responsável", 0, 200},
		{&d.ResponsavelApelido, "apelido do responsável", 0, 100}, {&d.ResponsavelLogradouro, "logradouro", 0, 200},
		{&d.ResponsavelNumero, "número", 0, 20}, {&d.ResponsavelComplemento, "complemento", 0, 100},
		{&d.ResponsavelBairro, "bairro", 0, 100}, {&d.ResponsavelCidade, "cidade", 0, 100},
		{&d.ResponsavelTelefone, "telefone do responsável", 0, 30}, {&d.ResponsavelCelular, "celular do responsável", 0, 30},
		{&d.ResponsavelEmail, "e-mail do responsável", 0, 200},
		{&d.ProcessoJudicial, "nº do 1º processo", 0, 200}, {&d.ProcessoJudicial2, "nº do 2º processo", 0, 200},
		{&d.GuiaAcolhimento, "nº da guia de acolhimento", 0, 2000}, {&d.OutroMotivo, "outro motivo", 0, 500},
		{&d.ConselheiroNome, "nome do conselheiro", 0, 200}, {&d.ConselheiroTelefone, "telefone do conselheiro", 0, 30},
		{&d.LaudoMedicoDescricao, "descrição do laudo", 0, 1000}, {&d.TranstornoDescricao, "descrição do transtorno", 0, 1000},
		{&d.BeneficioQual, "qual benefício", 0, 200}, {&d.Observacoes, "observações", 0, 4000},
	}); err != nil {
		return AcolhimentoDados{}, err
	}
	if err := conferirEnums([]campoEnum{
		{d.Sexo, "sexo", sexosAcolhido}, {d.IdentidadeGenero, "identidade de gênero", IdentidadesGenero},
		{d.OrientacaoSexual, "orientação sexual", OrientacoesSexuais}, {d.RacaCor, "raça/cor", RacasCores},
		{d.ResponsavelSexo, "sexo do responsável", sexosResponsavel}, {d.ResponsavelVinculo, "vínculo do responsável", VinculosResponsavel},
		{d.HistoricoAcolhidoAnteriormente, "acolhido anteriormente", SimNaoNaoSei},
		{d.HistoricoIrmaosNestaUnidade, "irmãos nesta unidade", SimNaoNaoSei},
		{d.HistoricoIrmaosOutraUnidade, "irmãos em outra unidade", SimNaoNaoSei},
		{d.SituacaoJuridica, "situação jurídica", SituacoesJuridicas}, {d.CNA, "CNA", SimNaoNaoSei}, {d.CNCAD, "CNCAD", SimNaoNaoSei},
		{d.LaudoMedico, "laudo médico", SimNao}, {d.MedicacaoContinua, "medicação contínua", SimNao},
		{d.DependenciaQuimica, "dependência química", SimNao}, {d.PossuiTranstorno, "possui transtorno", SimNao},
		{d.Estuda, "estuda", SimNao}, {d.AmeacaMorte, "ameaça de morte", SimNao}, {d.RecebeVisita, "recebe visita", SimNao},
		{d.BeneficioGoverno, "benefício do governo", SimNao}, {d.ProgramaSocial, "programa social", SimNao},
		{d.RelacaoTrabalho, "relação de trabalho", RelacoesTrabalho},
	}); err != nil {
		return AcolhimentoDados{}, err
	}
	if len(d.Motivos) == 0 {
		return AcolhimentoDados{}, invalid("motivos do acolhimento: informe ao menos um")
	}
	for _, m := range d.Motivos {
		if !slices.Contains(MotivosAcolhimento, m) {
			return AcolhimentoDados{}, invalid("motivo do acolhimento inválido: %s", m)
		}
	}
	slices.Sort(d.Motivos)
	d.Motivos = slices.Compact(d.Motivos)
	if slices.Contains(d.Motivos, "outro") != (d.OutroMotivo != "") {
		return AcolhimentoDados{}, invalid("outro motivo: especifique quando marcar \"outro\" (e só nesse caso)")
	}
	if d.ResponsavelNaoPossui {
		d.ResponsavelNome, d.ResponsavelNomeSocial, d.ResponsavelApelido, d.ResponsavelCPF, d.ResponsavelNIS = "", "", "", "", ""
		d.ResponsavelNascimento, d.ResponsavelSexo, d.ResponsavelVinculo = nil, "", ""
		d.ResponsavelCEP, d.ResponsavelLogradouro, d.ResponsavelNumero, d.ResponsavelComplemento = "", "", "", ""
		d.ResponsavelBairro, d.ResponsavelCidade, d.ResponsavelUF = "", "", ""
		d.ResponsavelTelefone, d.ResponsavelCelular, d.ResponsavelEmail = "", "", ""
	}
	if d.ResponsavelCPF = Digitos(d.ResponsavelCPF); d.ResponsavelCPF != "" && !CPFValido(d.ResponsavelCPF) {
		return AcolhimentoDados{}, invalid("CPF do responsável inválido")
	}
	if d.ResponsavelNIS = Digitos(d.ResponsavelNIS); d.ResponsavelNIS != "" && len(d.ResponsavelNIS) != 11 {
		return AcolhimentoDados{}, invalid("NIS do responsável: deve ter 11 dígitos")
	}
	if d.ResponsavelCEP = Digitos(d.ResponsavelCEP); d.ResponsavelCEP != "" && len(d.ResponsavelCEP) != 8 {
		return AcolhimentoDados{}, invalid("CEP do responsável: deve ter 8 dígitos")
	}
	if d.ResponsavelUF != "" && (len(d.ResponsavelUF) != 2 || strings.IndexFunc(d.ResponsavelUF, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0) {
		return AcolhimentoDados{}, invalid("UF do responsável: 2 letras")
	}
	if d.ResponsavelEmail != "" {
		if e, err := mail.ParseAddress(d.ResponsavelEmail); err != nil || e.Address != d.ResponsavelEmail {
			return AcolhimentoDados{}, invalid("e-mail do responsável inválido")
		}
	}
	if d.LaudoMedico != "sim" {
		d.LaudoMedicoDescricao = ""
	}
	if d.PossuiTranstorno != "sim" {
		d.TranstornoDescricao = ""
	}
	if d.BeneficioGoverno != "sim" {
		d.BeneficioQual = ""
	}
	return d, nil
}

// Acolhimento é a estadia de uma criança/adolescente na Casa Abrigo.
// Desligar só pela ação dedicada (Desligar): o PUT geral nunca toca o
// status nem o desligamento (só a equipe técnica desliga).
type Acolhimento struct {
	ID        uuid.UUID `json:"id"`
	CidadaoID uuid.UUID `json:"cidadao_id"`
	UnidadeID uuid.UUID `json:"unidade_id"`
	AcolhimentoDados
	DataEntrada          time.Time             `json:"data_entrada"`
	Status               string                `json:"status"`
	DesligamentoTipo     string                `json:"desligamento_tipo"`
	MotivoDesligamento   string                `json:"motivo_desligamento"`
	GuiaDesligamento     string                `json:"guia_desligamento"`
	DataGuiaDesligamento *time.Time            `json:"data_guia_desligamento"`
	RegistradoPor        *uuid.UUID            `json:"registrado_por"`
	DesligadoPor         *uuid.UUID            `json:"desligado_por"`
	DataDesligamento     *time.Time            `json:"data_desligamento"`
	Irmaos               []Irmao               `json:"irmaos"`
	Diagnosticos         []Diagnostico         `json:"diagnosticos"`
	Visitantes           []Visitante           `json:"visitantes"`
	Termos               []Termo               `json:"termos"`
	Pia                  *PiaFicha             `json:"pia"`
	PiaMetas             []PiaMeta             `json:"pia_metas"`
	Documentos           []DocumentoCasaAbrigo `json:"documentos"`
}

// dataDe é o dia (sem hora) de t no fuso do processo.
func dataDe(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// diaCivil é o dia de uma data "pura" (vinda do navegador como meia-noite
// UTC): o dia escrito, sem conversão de fuso.
func diaCivil(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// IdadeEm é a idade em anos completos na data ref (nascimento AAAA-MM-DD).
func IdadeEm(nascimento string, ref time.Time) (int, bool) {
	n, err := time.Parse(time.DateOnly, nascimento)
	if err != nil {
		return 0, false
	}
	idade := ref.Year() - n.Year()
	if ref.Month() < n.Month() || (ref.Month() == n.Month() && ref.Day() < n.Day()) {
		idade--
	}
	return max(idade, 0), true
}

// conferirEntrada aplica R1 e R2 à data na unidade.
func conferirEntrada(nascimento string, entrada, hoje time.Time) error {
	dia := diaCivil(entrada)
	if dia.After(dataDe(hoje)) {
		return invalid("data na unidade: não pode ser futura")
	}
	idade, ok := IdadeEm(nascimento, dia)
	if !ok {
		return invalid("o acolhido precisa ter a data de nascimento no cadastro")
	}
	if n, _ := time.Parse(time.DateOnly, nascimento); dia.Before(time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local)) {
		return invalid("data na unidade: anterior ao nascimento")
	}
	if idade >= IdadeMaioridade {
		return invalid("a Casa Abrigo acolhe menores de 18 anos (o acolhido tinha %d na data de entrada)", idade)
	}
	return nil
}

// NovoAcolhimento valida e monta um acolhimento recém-registrado (nasce
// 'acolhido'). nascimento é o do cadastro do munícipe (R2).
func NovoAcolhimento(cidadao, unidade uuid.UUID, nascimento string, dados AcolhimentoDados, entrada, hoje time.Time) (Acolhimento, error) {
	d, err := dados.Normalizar()
	if err != nil {
		return Acolhimento{}, err
	}
	if err := conferirEntrada(nascimento, entrada, hoje); err != nil {
		return Acolhimento{}, err
	}
	return Acolhimento{ID: uuid.New(), CidadaoID: cidadao, UnidadeID: unidade, AcolhimentoDados: d, DataEntrada: entrada,
		Status: AcolhimentoStatusAcolhido}, nil
}

// Atualizar regrava os campos descritivos e a data de entrada — nunca o
// status nem o desligamento (essa separação impede desligar por fora da
// ação dedicada).
func (a *Acolhimento) Atualizar(nascimento string, dados AcolhimentoDados, entrada, hoje time.Time) error {
	d, err := dados.Normalizar()
	if err != nil {
		return err
	}
	if err := conferirEntrada(nascimento, entrada, hoje); err != nil {
		return err
	}
	if a.DataDesligamento != nil && diaCivil(entrada).After(dataDe(*a.DataDesligamento)) {
		return invalid("data na unidade: posterior ao desligamento")
	}
	a.AcolhimentoDados, a.DataEntrada = d, entrada
	return nil
}

// Desligamento é o encerramento do acolhimento (R5).
type Desligamento struct {
	Tipo, Motivo, Guia string
	DataSaida          time.Time
	DataGuia           *time.Time
}

// Desligar encerra o acolhimento — ação restrita à equipe técnica, sem
// equivalente no PUT geral de edição.
func (a *Acolhimento) Desligar(d Desligamento, tecnico uuid.UUID, hoje time.Time) error {
	if a.Status != AcolhimentoStatusAcolhido {
		return ErrAcolhimentoEstado
	}
	if !slices.Contains(TiposDesligamento, d.Tipo) {
		return invalid("tipo de desligamento inválido")
	}
	motivo, err := texto("motivo do desligamento", d.Motivo, 3, 2000)
	if err != nil {
		return err
	}
	guia, err := texto("nº da guia de desligamento", d.Guia, 0, 200)
	if err != nil {
		return err
	}
	saida := diaCivil(d.DataSaida)
	if saida.After(dataDe(hoje)) {
		return invalid("data da saída: não pode ser futura")
	}
	if saida.Before(diaCivil(a.DataEntrada)) {
		return invalid("data da saída: anterior à entrada na unidade")
	}
	a.Status, a.DesligamentoTipo, a.MotivoDesligamento, a.GuiaDesligamento = AcolhimentoStatusDesligado, d.Tipo, motivo, guia
	a.DataGuiaDesligamento, a.DesligadoPor, a.DataDesligamento = d.DataGuia, &tecnico, &d.DataSaida
	return nil
}

// Irmao é um irmão do acolhido, para preservar o vínculo familiar mesmo
// quando não está na mesma unidade.
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
	i := Irmao{ID: uuid.New(), AcolhimentoID: acolhimento, Nome: nome, DataNascimento: dataNascimento, EstaAcolhido: estaAcolhido,
		UnidadeDescricao: unidadeDescricao, Observacao: observacao}
	if err := normalizarTextos([]campoTexto{{&i.Nome, "nome", 2, 200}, {&i.UnidadeDescricao, "unidade do irmão", 0, 200},
		{&i.Observacao, "observação", 0, 1000}}); err != nil {
		return Irmao{}, err
	}
	return i, nil
}

// Diagnostico é um diagnóstico de saúde, deficiência ou transtorno do
// acolhido, numa lista — o checklist do 4.0 vira diagnósticos com o nome
// padronizado (R6), não 30 colunas booleanas.
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
	d := Diagnostico{ID: uuid.New(), AcolhimentoID: acolhimento, Categoria: categoria, Diagnostico: diagnostico, Descricao: descricao}
	if err := normalizarTextos([]campoTexto{{&d.Diagnostico, "diagnóstico", 2, 255}, {&d.Descricao, "descrição", 0, 2000}}); err != nil {
		return Diagnostico{}, err
	}
	return d, nil
}

// ------------------------------------------------ visitantes e termos

// TiposVisita: visita a um acolhido, à instituição, doação ou voluntariado.
var TiposVisita = []string{"acolhido", "instituicao", "doacao", "voluntariado"}

// Visitante é uma visita recebida pela unidade — a um acolhido ou à
// instituição (doação, voluntariado). AcolhidoNome só vem nas listas da
// unidade.
type Visitante struct {
	ID                  uuid.UUID  `json:"id"`
	UnidadeID           uuid.UUID  `json:"unidade_id"`
	AcolhimentoID       *uuid.UUID `json:"acolhimento_id"`
	TipoVisita          string     `json:"tipo_visita"`
	NomeVisitante       string     `json:"nome_visitante"`
	DocumentoIdentidade string     `json:"documento_identidade"`
	ParentescoVinculo   string     `json:"parentesco_vinculo"`
	DataVisita          time.Time  `json:"data_visita"`
	HoraEntrada         string     `json:"hora_entrada"` // "HH:MM" ou ""
	HoraSaida           string     `json:"hora_saida"`
	Observacoes         string     `json:"observacoes"`
	AcolhidoNome        string     `json:"acolhido_nome,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

func hora(campo, v string) (string, error) {
	if v == "" {
		return "", nil
	}
	if len(v) != 5 || v[2] != ':' || !unicode.IsDigit(rune(v[0])) || !unicode.IsDigit(rune(v[1])) ||
		!unicode.IsDigit(rune(v[3])) || !unicode.IsDigit(rune(v[4])) || v[:2] > "23" || v[3:] > "59" {
		return "", invalid("%s: formato HH:MM", campo)
	}
	return v, nil
}

// VisitanteDados são os campos de uma visita.
type VisitanteDados struct {
	AcolhimentoID                                         *uuid.UUID
	TipoVisita                                            string
	NomeVisitante, DocumentoIdentidade, ParentescoVinculo string
	DataVisita                                            time.Time
	HoraEntrada, HoraSaida, Observacoes                   string
}

// NovoVisitante valida e monta uma visita: a um acolhido exige o acolhido.
func NovoVisitante(unidade uuid.UUID, d VisitanteDados) (Visitante, error) {
	if d.TipoVisita == "" {
		d.TipoVisita = "acolhido"
	}
	if !slices.Contains(TiposVisita, d.TipoVisita) {
		return Visitante{}, invalid("tipo de visita inválido")
	}
	if d.TipoVisita == "acolhido" && d.AcolhimentoID == nil {
		return Visitante{}, invalid("visita a acolhido: informe o acolhido")
	}
	v := Visitante{ID: uuid.New(), UnidadeID: unidade, AcolhimentoID: d.AcolhimentoID, TipoVisita: d.TipoVisita,
		NomeVisitante: d.NomeVisitante, DocumentoIdentidade: d.DocumentoIdentidade, ParentescoVinculo: d.ParentescoVinculo,
		DataVisita: d.DataVisita, Observacoes: d.Observacoes}
	if err := normalizarTextos([]campoTexto{{&v.NomeVisitante, "nome do visitante", 2, 200},
		{&v.DocumentoIdentidade, "documento de identidade", 0, 30}, {&v.ParentescoVinculo, "parentesco/vínculo", 0, 100},
		{&v.Observacoes, "observações", 0, 1000}}); err != nil {
		return Visitante{}, err
	}
	var err error
	if v.HoraEntrada, err = hora("hora de entrada", d.HoraEntrada); err != nil {
		return Visitante{}, err
	}
	if v.HoraSaida, err = hora("hora de saída", d.HoraSaida); err != nil {
		return Visitante{}, err
	}
	if v.HoraSaida != "" && v.HoraEntrada != "" && v.HoraSaida < v.HoraEntrada {
		return Visitante{}, invalid("hora de saída anterior à de entrada")
	}
	return v, nil
}

// RegistrarSaida anota a hora em que o visitante saiu (uma vez só).
func (v *Visitante) RegistrarSaida(h string) error {
	if v.HoraSaida != "" {
		return ErrVisitaEncerrada
	}
	saida, err := hora("hora de saída", h)
	if err != nil {
		return err
	}
	if saida == "" {
		return invalid("hora de saída: informe HH:MM")
	}
	if v.HoraEntrada != "" && saida < v.HoraEntrada {
		return invalid("hora de saída anterior à de entrada")
	}
	v.HoraSaida = saida
	return nil
}

// Tipos de termo da Casa Abrigo.
var TiposTermo = []string{"advertencia", "passeio", "saida", "outro"}

// Termo é um termo de advertência, passeio ou saída do acolhido (o
// documento é impresso na tela; a assinatura não é guardada).
type Termo struct {
	ID                    uuid.UUID `json:"id"`
	AcolhimentoID         uuid.UUID `json:"acolhimento_id"`
	TipoTermo             string    `json:"tipo_termo"`
	ResponsavelRetirada   string    `json:"responsavel_retirada"`
	ParentescoResponsavel string    `json:"parentesco_responsavel"`
	MotivoObservacao      string    `json:"motivo_observacao"`
	DataTermo             time.Time `json:"data_termo"`
	AcolhidoNome          string    `json:"acolhido_nome,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}

// NovoTermo valida e monta um termo registrado.
func NovoTermo(acolhimento uuid.UUID, tipoTermo, responsavelRetirada, parentescoResponsavel, motivoObservacao string, dataTermo time.Time) (Termo, error) {
	if !slices.Contains(TiposTermo, tipoTermo) {
		return Termo{}, invalid("tipo de termo inválido (advertencia, passeio, saida ou outro)")
	}
	t := Termo{ID: uuid.New(), AcolhimentoID: acolhimento, TipoTermo: tipoTermo, ResponsavelRetirada: responsavelRetirada,
		ParentescoResponsavel: parentescoResponsavel, MotivoObservacao: motivoObservacao, DataTermo: dataTermo}
	if err := normalizarTextos([]campoTexto{{&t.ResponsavelRetirada, "responsável pela retirada", 0, 200},
		{&t.ParentescoResponsavel, "parentesco do responsável", 0, 100}, {&t.MotivoObservacao, "motivo/observação", 0, 2000}}); err != nil {
		return Termo{}, err
	}
	return t, nil
}

// --------------------------------------------------------------- PIA

// Listas do PIA.
var (
	PeriodosEscolares  = []string{"", "matutino", "vespertino", "noturno", "integral"}
	MoradiasAnteriores = []string{"", "propria", "alugada", "cedida", "situacao_rua"}
)

// PiaFichaDados são as 6 abas do PIA (exceto as metas, que são lista).
type PiaFichaDados struct {
	// identificação e documentos
	Naturalidade          string `json:"naturalidade"`
	Nacionalidade         string `json:"nacionalidade"`
	Religiao              string `json:"religiao"`
	DocCarteiraVacinacao  bool   `json:"doc_carteira_vacinacao"`
	DocCertidaoNascimento bool   `json:"doc_certidao_nascimento"`
	DocRgCpf              bool   `json:"doc_rg_cpf"`
	DocCartaoSUS          bool   `json:"doc_cartao_sus"`
	DocNIS                bool   `json:"doc_nis"`
	// saúde e escola
	SinaisViolenciaFisica       bool   `json:"sinais_violencia_fisica"`
	Medicamentos                string `json:"medicamentos"`
	Alergias                    string `json:"alergias"`
	VacinacaoAtualizada         bool   `json:"vacinacao_atualizada"`
	CartaoSUS                   string `json:"cartao_sus"`
	TratamentoOdontologico      bool   `json:"tratamento_odontologico"`
	TratamentoMedicoPsicologico bool   `json:"tratamento_medico_psicologico"`
	DeficienciasRestricoes      string `json:"deficiencias_restricoes"`
	Escola                      string `json:"escola"`
	SerieEscolar                string `json:"serie_escolar"`
	MatriculaEscolar            string `json:"matricula_escolar"`
	PeriodoEscolar              string `json:"periodo_escolar"`
	FrequenciaEscolar           string `json:"frequencia_escolar"`
	AulaReforco                 bool   `json:"aula_reforco"`
	// situação familiar
	HistoricoFamiliar  string `json:"historico_familiar"`
	PossuiBeneficios   bool   `json:"possui_beneficios"`
	MoradiaAnterior    string `json:"moradia_anterior"`
	ContatoFamiliar    string `json:"contato_familiar"`
	VisitasAutorizadas string `json:"visitas_autorizadas"`
	ObservacoesFamilia string `json:"observacoes_familia"`
	// rotina
	AtividadesManha            string `json:"atividades_manha"`
	AtividadesTarde            string `json:"atividades_tarde"`
	AtividadesNoite            string `json:"atividades_noite"`
	AtividadesFimSemana        string `json:"atividades_fim_semana"`
	ObservacoesComportamentais string `json:"observacoes_comportamentais"`
	// intervenção familiar
	IntervencaoObjetivos   string `json:"intervencao_objetivos"`
	IntervencaoEstrategias string `json:"intervencao_estrategias"`
	IntervencaoMetasPrazos string `json:"intervencao_metas_prazos"`
}

// PiaFicha é a ficha 1:1 do Plano Individual de Atendimento.
// AtualizadoEm é a última reavaliação (R7).
type PiaFicha struct {
	AcolhimentoID uuid.UUID `json:"acolhimento_id"`
	PiaFichaDados
	AtualizadoPor *uuid.UUID `json:"atualizado_por"`
	AtualizadoEm  time.Time  `json:"atualizado_em"`
}

// NovaPiaFicha valida e monta a ficha do PIA.
func NovaPiaFicha(acolhimento uuid.UUID, d PiaFichaDados) (PiaFicha, error) {
	if err := normalizarTextos([]campoTexto{
		{&d.Naturalidade, "naturalidade", 0, 100}, {&d.Nacionalidade, "nacionalidade", 0, 100}, {&d.Religiao, "religião", 0, 100},
		{&d.Medicamentos, "medicamentos", 0, 2000}, {&d.Alergias, "alergias", 0, 2000}, {&d.CartaoSUS, "cartão SUS", 0, 30},
		{&d.DeficienciasRestricoes, "deficiências e restrições", 0, 2000}, {&d.Escola, "escola", 0, 200},
		{&d.SerieEscolar, "série escolar", 0, 100}, {&d.MatriculaEscolar, "matrícula escolar", 0, 100},
		{&d.FrequenciaEscolar, "frequência escolar", 0, 200}, {&d.HistoricoFamiliar, "histórico familiar", 0, 4000},
		{&d.ContatoFamiliar, "contato familiar", 0, 2000}, {&d.VisitasAutorizadas, "visitas autorizadas", 0, 2000},
		{&d.ObservacoesFamilia, "observações da família", 0, 4000}, {&d.AtividadesManha, "atividades da manhã", 0, 1000},
		{&d.AtividadesTarde, "atividades da tarde", 0, 1000}, {&d.AtividadesNoite, "atividades da noite", 0, 1000},
		{&d.AtividadesFimSemana, "atividades de fim de semana", 0, 1000},
		{&d.ObservacoesComportamentais, "observações comportamentais", 0, 2000},
		{&d.IntervencaoObjetivos, "objetivos da intervenção", 0, 4000}, {&d.IntervencaoEstrategias, "estratégias", 0, 4000},
		{&d.IntervencaoMetasPrazos, "metas e prazos", 0, 4000},
	}); err != nil {
		return PiaFicha{}, err
	}
	if err := conferirEnums([]campoEnum{{d.PeriodoEscolar, "período escolar", PeriodosEscolares},
		{d.MoradiaAnterior, "moradia anterior", MoradiasAnteriores}}); err != nil {
		return PiaFicha{}, err
	}
	return PiaFicha{AcolhimentoID: acolhimento, PiaFichaDados: d}, nil
}

// ProximaReavaliacaoPia é o prazo da próxima reavaliação (R7): a última
// gravação do PIA (ou a entrada, se ainda não houve) mais 90 dias.
func ProximaReavaliacaoPia(entrada time.Time, piaAtualizadoEm *time.Time) time.Time {
	base := diaCivil(entrada)
	if piaAtualizadoEm != nil {
		base = dataDe(*piaAtualizadoEm)
	}
	return base.AddDate(0, 0, PrazoReavaliacaoPia)
}

// Tipos de meta do PIA: plano de ação (necessidade/providências, mais
// esportes, cultura e lazer) e intervenção familiar.
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
	Esportes      string     `json:"esportes"`
	Cultura       string     `json:"cultura"`
	Lazer         string     `json:"lazer"`
	Concluida     bool       `json:"concluida"`
	CreatedAt     time.Time  `json:"created_at"`
}

// PiaMetaDados são os campos editáveis de uma meta.
type PiaMetaDados struct {
	Tipo, Descricao, Providencia, Responsavel string
	Prazo                                     *time.Time
	Esportes, Cultura, Lazer                  string
}

func (d PiaMetaDados) normalizar() (PiaMetaDados, error) {
	if d.Tipo != PiaMetaPlano && d.Tipo != PiaMetaIntervencao {
		return PiaMetaDados{}, invalid("tipo de meta inválido (plano ou intervencao)")
	}
	if err := normalizarTextos([]campoTexto{{&d.Descricao, "necessidade/descrição", 3, 1000},
		{&d.Providencia, "providência", 0, 1000}, {&d.Responsavel, "responsável", 0, 200},
		{&d.Esportes, "esportes", 0, 500}, {&d.Cultura, "cultura", 0, 500}, {&d.Lazer, "lazer", 0, 500}}); err != nil {
		return PiaMetaDados{}, err
	}
	return d, nil
}

// NovaPiaMeta valida e monta uma meta do PIA.
func NovaPiaMeta(acolhimento uuid.UUID, dados PiaMetaDados) (PiaMeta, error) {
	d, err := dados.normalizar()
	if err != nil {
		return PiaMeta{}, err
	}
	return PiaMeta{ID: uuid.New(), AcolhimentoID: acolhimento, Tipo: d.Tipo, Descricao: d.Descricao, Providencia: d.Providencia,
		Responsavel: d.Responsavel, Prazo: d.Prazo, Esportes: d.Esportes, Cultura: d.Cultura, Lazer: d.Lazer}, nil
}

// Editar regrava a meta (a tabela do plano de ação é editável).
func (m *PiaMeta) Editar(dados PiaMetaDados) error {
	d, err := dados.normalizar()
	if err != nil {
		return err
	}
	m.Tipo, m.Descricao, m.Providencia, m.Responsavel, m.Prazo = d.Tipo, d.Descricao, d.Providencia, d.Responsavel, d.Prazo
	m.Esportes, m.Cultura, m.Lazer = d.Esportes, d.Cultura, d.Lazer
	return nil
}

// --------------------------------------------------------- documentos

// Tipos de documento da Casa Abrigo.
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

// --------------------------------------------------- diário de bordo

// AnotacaoDiario é uma anotação do diário de bordo da unidade, geral ou
// de um acolhido, referente a um dia (pode ser futuro, como agenda). Só
// de inclusão: as ações do sistema já estão na trilha de auditoria.
type AnotacaoDiario struct {
	ID             uuid.UUID  `json:"id"`
	UnidadeID      uuid.UUID  `json:"unidade_id"`
	AcolhimentoID  *uuid.UUID `json:"acolhimento_id"`
	DataReferencia time.Time  `json:"data_referencia"`
	Texto          string     `json:"texto"`
	AutorNome      string     `json:"autor_nome"`
	AcolhidoNome   string     `json:"acolhido_nome,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// NovaAnotacaoDiario valida e monta uma anotação do diário.
func NovaAnotacaoDiario(unidade uuid.UUID, acolhimento *uuid.UUID, data time.Time, txt, autor string) (AnotacaoDiario, error) {
	t, err := texto("anotação", txt, 3, 4000)
	if err != nil {
		return AnotacaoDiario{}, err
	}
	return AnotacaoDiario{ID: uuid.New(), UnidadeID: unidade, AcolhimentoID: acolhimento, DataReferencia: data, Texto: t,
		AutorNome: autor}, nil
}

// --------------------------------------------------- relatórios escritos

// Tipos e etapas dos relatórios escritos.
var (
	TiposRelatorioCasaAbrigo = []string{"cinco_dias", "sete_dias", "quinzenal", "mensal", "bimestral", "trimestral", "anual",
		"psicossocial", "circunstanciado", "informativo", "outro"}
	StatusRelatorioCasaAbrigo = []string{"em_elaboracao", "revisao", "aprovado", "entregue"}
)

// RelatorioEntregue é a etapa final (R8).
const RelatorioEntregue = "entregue"

// rotulosRelatorio dão nome aos tipos nos avisos do monitor.
var rotulosRelatorio = map[string]string{"cinco_dias": "para 5 dias", "sete_dias": "para 7 dias", "quinzenal": "quinzenal",
	"mensal": "mensal", "bimestral": "bimestral", "trimestral": "trimestral", "anual": "anual", "psicossocial": "psicossocial",
	"circunstanciado": "circunstanciado", "informativo": "informativo"}

// Nome é como o relatório aparece nos avisos ("para 5 dias", ou o nome do "outro").
func (r RelatorioCasaAbrigo) Nome() string {
	if r.Tipo == "outro" {
		return r.TipoOutro
	}
	return rotulosRelatorio[r.Tipo]
}

// RelatorioCasaAbrigo é um relatório escrito da equipe (para o
// Judiciário, a rede ou a gestão), com prazo de entrega.
type RelatorioCasaAbrigo struct {
	ID            uuid.UUID  `json:"id"`
	UnidadeID     uuid.UUID  `json:"unidade_id"`
	AcolhimentoID *uuid.UUID `json:"acolhimento_id"`
	Tipo          string     `json:"tipo"`
	TipoOutro     string     `json:"tipo_outro"`
	Status        string     `json:"status"`
	DataEntrega   time.Time  `json:"data_entrega"`
	Conteudo      string     `json:"conteudo"`
	AcolhidoNome  string     `json:"acolhido_nome,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	AtualizadoEm  time.Time  `json:"atualizado_em"`
}

// RelatorioDados são os campos de um relatório.
type RelatorioDados struct {
	AcolhimentoID           *uuid.UUID
	Tipo, TipoOutro, Status string
	DataEntrega             time.Time
	Conteudo                string
}

func (d RelatorioDados) normalizar() (RelatorioDados, error) {
	if err := conferirEnums([]campoEnum{{d.Tipo, "tipo de relatório", TiposRelatorioCasaAbrigo},
		{d.Status, "etapa do relatório", StatusRelatorioCasaAbrigo}}); err != nil {
		return RelatorioDados{}, err
	}
	if d.Tipo != "outro" {
		d.TipoOutro = ""
	}
	if err := normalizarTextos([]campoTexto{{&d.TipoOutro, "nome do relatório", 0, 200}}); err != nil {
		return RelatorioDados{}, err
	}
	if d.Tipo == "outro" && d.TipoOutro == "" {
		return RelatorioDados{}, invalid("nome do relatório: informe quando o tipo é \"outro\"")
	}
	if len([]rune(d.Conteudo)) > 100000 {
		return RelatorioDados{}, invalid("conteúdo: até 100000 caracteres")
	}
	if d.DataEntrega.IsZero() {
		return RelatorioDados{}, invalid("data de entrega: obrigatória")
	}
	return d, nil
}

// NovoRelatorio valida e monta um relatório.
func NovoRelatorio(unidade uuid.UUID, dados RelatorioDados) (RelatorioCasaAbrigo, error) {
	d, err := dados.normalizar()
	if err != nil {
		return RelatorioCasaAbrigo{}, err
	}
	return RelatorioCasaAbrigo{ID: uuid.New(), UnidadeID: unidade, AcolhimentoID: d.AcolhimentoID, Tipo: d.Tipo,
		TipoOutro: d.TipoOutro, Status: d.Status, DataEntrega: d.DataEntrega, Conteudo: d.Conteudo}, nil
}

// Atualizar regrava o relatório — o entregue não muda mais (R8).
func (r *RelatorioCasaAbrigo) Atualizar(dados RelatorioDados) error {
	if r.Status == RelatorioEntregue {
		return ErrRelatorioEntregue
	}
	d, err := dados.normalizar()
	if err != nil {
		return err
	}
	r.AcolhimentoID, r.Tipo, r.TipoOutro, r.Status, r.DataEntrega, r.Conteudo = d.AcolhimentoID, d.Tipo, d.TipoOutro, d.Status,
		d.DataEntrega, d.Conteudo
	return nil
}

// PodeExcluir: o entregue não pode ser excluído (R8).
func (r RelatorioCasaAbrigo) PodeExcluir() error {
	if r.Status == RelatorioEntregue {
		return ErrRelatorioEntregue
	}
	return nil
}

// ------------------------------------------------- tarefas (Kanban)

// Colunas e prioridades do Kanban da equipe.
var (
	StatusTarefa      = []string{"backlog", "todo", "doing", "review", "done"}
	PrioridadesTarefa = []string{"baixa", "media", "alta", "critica"}
)

// TarefaConcluida é a última coluna (não gera aviso de prazo).
const TarefaConcluida = "done"

// TarefaCasaAbrigo é uma tarefa da equipe da unidade.
type TarefaCasaAbrigo struct {
	ID           uuid.UUID  `json:"id"`
	UnidadeID    uuid.UUID  `json:"unidade_id"`
	Titulo       string     `json:"titulo"`
	Descricao    string     `json:"descricao"`
	Status       string     `json:"status"`
	Prioridade   string     `json:"prioridade"`
	Responsavel  string     `json:"responsavel"`
	Sprint       string     `json:"sprint"`
	Prazo        *time.Time `json:"prazo"`
	Ordem        int        `json:"ordem"`
	CreatedAt    time.Time  `json:"created_at"`
	AtualizadoEm time.Time  `json:"atualizado_em"`
}

// TarefaDados são os campos de uma tarefa.
type TarefaDados struct {
	Titulo, Descricao, Status, Prioridade, Responsavel, Sprint string
	Prazo                                                      *time.Time
	Ordem                                                      int
}

func (d TarefaDados) normalizar() (TarefaDados, error) {
	if d.Status == "" {
		d.Status = "backlog"
	}
	if d.Prioridade == "" {
		d.Prioridade = "media"
	}
	if err := conferirEnums([]campoEnum{{d.Status, "coluna", StatusTarefa}, {d.Prioridade, "prioridade", PrioridadesTarefa}}); err != nil {
		return TarefaDados{}, err
	}
	if err := normalizarTextos([]campoTexto{{&d.Titulo, "título", 2, 200}, {&d.Responsavel, "responsável", 0, 200},
		{&d.Sprint, "sprint", 0, 100}}); err != nil {
		return TarefaDados{}, err
	}
	if d.Descricao = strings.TrimSpace(d.Descricao); len([]rune(d.Descricao)) > 4000 {
		return TarefaDados{}, invalid("descrição: até 4000 caracteres")
	}
	if d.Ordem < 0 {
		return TarefaDados{}, invalid("ordem: não pode ser negativa")
	}
	return d, nil
}

// NovaTarefa valida e monta uma tarefa.
func NovaTarefa(unidade uuid.UUID, dados TarefaDados) (TarefaCasaAbrigo, error) {
	d, err := dados.normalizar()
	if err != nil {
		return TarefaCasaAbrigo{}, err
	}
	return TarefaCasaAbrigo{ID: uuid.New(), UnidadeID: unidade, Titulo: d.Titulo, Descricao: d.Descricao, Status: d.Status,
		Prioridade: d.Prioridade, Responsavel: d.Responsavel, Sprint: d.Sprint, Prazo: d.Prazo, Ordem: d.Ordem}, nil
}

// Atualizar regrava a tarefa (inclusive a coluna, ao arrastar).
func (t *TarefaCasaAbrigo) Atualizar(dados TarefaDados) error {
	d, err := dados.normalizar()
	if err != nil {
		return err
	}
	t.Titulo, t.Descricao, t.Status, t.Prioridade, t.Responsavel = d.Titulo, d.Descricao, d.Status, d.Prioridade, d.Responsavel
	t.Sprint, t.Prazo, t.Ordem = d.Sprint, d.Prazo, d.Ordem
	return nil
}

// ------------------------------------------------- painel e relatórios

// AcolhidoResumo é uma linha do relatório de institucionalizados.
type AcolhidoResumo struct {
	AcolhimentoID        uuid.UUID  `json:"acolhimento_id"`
	CidadaoID            uuid.UUID  `json:"cidadao_id"`
	Nome                 string     `json:"nome"`
	NomeSocial           string     `json:"nome_social"`
	DataNascimento       string     `json:"data_nascimento"`
	Idade                int        `json:"idade"`
	Sexo                 string     `json:"sexo"`
	DataEntrada          time.Time  `json:"data_entrada"`
	ProcessoJudicial     string     `json:"processo_judicial"`
	GuiaAcolhimento      string     `json:"guia_acolhimento"`
	SituacaoJuridica     string     `json:"situacao_juridica"`
	Motivos              []string   `json:"motivos"`
	Status               string     `json:"status"`
	DesligamentoTipo     string     `json:"desligamento_tipo"`
	DataDesligamento     *time.Time `json:"data_desligamento"`
	PiaAtualizadoEm      *time.Time `json:"pia_atualizado_em"`
	ProximaReavaliacao   time.Time  `json:"proxima_reavaliacao_pia"`
	DocumentoAcolhido    bool       `json:"documento_acolhido"`
	DocumentoResponsavel bool       `json:"documento_responsavel"`
}

// NomeExibicao respeita o nome social.
func (a AcolhidoResumo) NomeExibicao() string {
	if a.NomeSocial != "" {
		return a.NomeSocial
	}
	return a.Nome
}

// Iniciais reduz o nome às iniciais ("Ana Maria da Silva" → "A. M. S."),
// como o 4.0 faz nos avisos e no histórico de termos (sigilo, ECA art. 143).
func Iniciais(nome string) string {
	var partes []string
	for _, p := range strings.Fields(nome) {
		if slices.Contains([]string{"de", "da", "do", "das", "dos", "e"}, strings.ToLower(p)) {
			continue
		}
		partes = append(partes, strings.ToUpper(string([]rune(p)[:1]))+".")
	}
	if len(partes) == 0 {
		return "N/I"
	}
	return strings.Join(partes, " ")
}

// Alerta é um aviso do monitor de prazos (ADR 032 §2 a §4).
type Alerta struct {
	Tipo          string     `json:"tipo"`  // pia, meta, relatorio, tarefa
	Nivel         string     `json:"nivel"` // aviso (vence em breve) ou atraso
	Mensagem      string     `json:"mensagem"`
	Prazo         time.Time  `json:"prazo"`
	Dias          int        `json:"dias"` // até o prazo; negativo = atrasado
	AcolhimentoID *uuid.UUID `json:"acolhimento_id,omitempty"`
	RefID         uuid.UUID  `json:"ref_id"`
}

// MetaPendente é uma meta do PIA ainda aberta, com prazo, de um acolhido
// ativo (o monitor avisa).
type MetaPendente struct {
	PiaMeta
	AcolhidoNome string `json:"acolhido_nome"`
}

func diasAte(hoje, prazo time.Time) int {
	return int(prazo.Sub(dataDe(hoje)).Hours() / 24)
}

func alerta(tipo, msgAviso, msgAtraso string, prazo, hoje time.Time, janela int, acolhimento *uuid.UUID, ref uuid.UUID) (Alerta, bool) {
	dias := diasAte(hoje, prazo)
	if dias > janela {
		return Alerta{}, false
	}
	a := Alerta{Tipo: tipo, Nivel: "aviso", Mensagem: msgAviso, Prazo: prazo, Dias: dias, AcolhimentoID: acolhimento, RefID: ref}
	if dias < 0 {
		a.Nivel, a.Mensagem = "atraso", msgAtraso
	}
	return a, true
}

// PainelCasaAbrigo é a tela inicial da unidade: os cartões do 4.0, o
// relatório de institucionalizados e o monitor de prazos.
type PainelCasaAbrigo struct {
	Criancas     int              `json:"criancas"`
	Adolescentes int              `json:"adolescentes"`
	Ativos       int              `json:"ativos"`
	PiasVencendo int              `json:"pias_vencendo"`
	Desacolhidos int              `json:"desacolhidos"`
	Acolhidos    []AcolhidoResumo `json:"acolhidos"`
	Desligados   []AcolhidoResumo `json:"desligados"`
	Alertas      []Alerta         `json:"alertas"`
}

// MontarPainel calcula os cartões e o monitor. todos são os acolhimentos
// da unidade (ativos e desligados); a idade é a de hoje.
func MontarPainel(hoje time.Time, todos []AcolhidoResumo, metas []MetaPendente, relatorios []RelatorioCasaAbrigo, tarefas []TarefaCasaAbrigo) PainelCasaAbrigo {
	p := PainelCasaAbrigo{Acolhidos: []AcolhidoResumo{}, Desligados: []AcolhidoResumo{}, Alertas: []Alerta{}}
	dia := dataDe(hoje)
	for _, a := range todos {
		a.Idade, _ = IdadeEm(a.DataNascimento, dia)
		a.ProximaReavaliacao = ProximaReavaliacaoPia(a.DataEntrada, a.PiaAtualizadoEm)
		if a.Status != AcolhimentoStatusAcolhido {
			p.Desacolhidos++
			p.Desligados = append(p.Desligados, a)
			continue
		}
		p.Ativos++
		if a.Idade < IdadeAdolescente {
			p.Criancas++
		} else {
			p.Adolescentes++
		}
		p.Acolhidos = append(p.Acolhidos, a)
		id := a.AcolhimentoID
		ini := Iniciais(a.NomeExibicao())
		if al, ok := alerta("pia", "Reavaliação do PIA de "+ini, "Reavaliação do PIA de "+ini+" atrasada",
			a.ProximaReavaliacao, hoje, avisoPia, &id, id); ok {
			p.PiasVencendo++
			p.Alertas = append(p.Alertas, al)
		}
	}
	for _, m := range metas {
		if m.Prazo == nil || m.Concluida {
			continue
		}
		id := m.AcolhimentoID
		ini := Iniciais(m.AcolhidoNome)
		if al, ok := alerta("meta", "Meta do PIA de "+ini+": "+m.Descricao, "Meta do PIA de "+ini+" vencida: "+m.Descricao,
			diaCivil(*m.Prazo), hoje, avisoMeta, &id, m.ID); ok {
			p.Alertas = append(p.Alertas, al)
		}
	}
	for _, r := range relatorios {
		if r.Status == RelatorioEntregue {
			continue
		}
		nome := r.Nome()
		if al, ok := alerta("relatorio", "Relatório "+nome+" a entregar", "Relatório "+nome+" atrasado",
			diaCivil(r.DataEntrega), hoje, avisoRelatorio, r.AcolhimentoID, r.ID); ok {
			p.Alertas = append(p.Alertas, al)
		}
	}
	for _, t := range tarefas {
		if t.Prazo == nil || t.Status == TarefaConcluida {
			continue
		}
		if al, ok := alerta("tarefa", "Tarefa: "+t.Titulo, "Tarefa atrasada: "+t.Titulo, diaCivil(*t.Prazo), hoje, avisoTarefa, nil, t.ID); ok {
			p.Alertas = append(p.Alertas, al)
		}
	}
	slices.SortStableFunc(p.Alertas, func(a, b Alerta) int { return a.Dias - b.Dias })
	return p
}

// EstatisticasCasaAbrigo é o relatório estatístico de um período
// (mensal, trimestral ou anual — a tela escolhe as datas).
type EstatisticasCasaAbrigo struct {
	De             string         `json:"de"`
	Ate            string         `json:"ate"`
	Entradas       int            `json:"entradas"`
	Saidas         int            `json:"saidas"`
	Reintegracoes  int            `json:"reintegracoes"`
	Adocoes        int            `json:"adocoes"`
	Transferencias int            `json:"transferencias"`
	SaidasPorTipo  map[string]int `json:"saidas_por_tipo"`
	Motivos        map[string]int `json:"motivos"`
	AtivosNoFim    int            `json:"ativos_no_fim"`
	Criancas       int            `json:"criancas"`
	Adolescentes   int            `json:"adolescentes"`
	Meninos        int            `json:"meninos"`
	Meninas        int            `json:"meninas"`
}

// Estatisticas conta entradas, saídas (pelo tipo, R5) e motivos das
// entradas entre de e ate (inclusive), e quem estava acolhido no fim.
func Estatisticas(todos []AcolhidoResumo, de, ate time.Time) (EstatisticasCasaAbrigo, error) {
	if ate.Before(de) {
		return EstatisticasCasaAbrigo{}, invalid("período inválido: o fim é anterior ao início")
	}
	if ate.Sub(de) > 366*24*time.Hour {
		return EstatisticasCasaAbrigo{}, invalid("período inválido: até 1 ano")
	}
	e := EstatisticasCasaAbrigo{De: de.Format(time.DateOnly), Ate: ate.Format(time.DateOnly), SaidasPorTipo: map[string]int{},
		Motivos: map[string]int{}}
	ini, fim := diaCivil(de), diaCivil(ate)
	dentro := func(t time.Time) bool { d := diaCivil(t); return !d.Before(ini) && !d.After(fim) }
	for _, a := range todos {
		if dentro(a.DataEntrada) {
			e.Entradas++
			for _, m := range a.Motivos {
				e.Motivos[m]++
			}
		}
		if a.DataDesligamento != nil && dentro(*a.DataDesligamento) {
			e.Saidas++
			e.SaidasPorTipo[a.DesligamentoTipo]++
			switch a.DesligamentoTipo {
			case "reintegracao_familia_natural", "reintegracao_familia_extensa":
				e.Reintegracoes++
			case "adocao":
				e.Adocoes++
			case "transferencia":
				e.Transferencias++
			}
		}
		ativoNoFim := !diaCivil(a.DataEntrada).After(fim) && (a.DataDesligamento == nil || diaCivil(*a.DataDesligamento).After(fim))
		if !ativoNoFim {
			continue
		}
		e.AtivosNoFim++
		if idade, _ := IdadeEm(a.DataNascimento, fim); idade < IdadeAdolescente {
			e.Criancas++
		} else {
			e.Adolescentes++
		}
		if a.Sexo == "feminino" {
			e.Meninas++
		} else {
			e.Meninos++
		}
	}
	return e, nil
}

// EvolucaoAcolhido é uma evolução do prontuário de um acolhido da unidade
// (a tela de evolução técnica lista todos, com busca por nome).
type EvolucaoAcolhido struct {
	Registro
	AcolhidoNome string `json:"acolhido_nome"`
}
