// Package iaconfig implementa as conexões de inteligência artificial
// editáveis em tempo de execução (Configurações > Inteligência artificial,
// permissão ia:manage — ADR 020), no mesmo espírito de keycloakconfig:
//
//   - conexões persistidas no Postgres, com a chave de API cifrada
//     (secretcrypto, CONFIG_ENCRYPTION_KEY) e nunca devolvida pela API;
//   - toda conexão é TESTADA (uma conversa real) antes de ser gravada;
//   - cada função que usa IA (hoje, o assistente do Atlas) tem uma conexão
//     principal e uma de reserva; sem nada configurado vale o ambiente
//     (ATLAS_AI_*), e sem ambiente a função segue sem IA;
//   - fornecedor externo exige autorização explícita (transferência de
//     dados — LGPD art. 33) e, por padrão, CPF, CNPJ, e-mail e telefone são
//     mascarados na pergunta antes de sair da rede.
//
// Todos os fornecedores falam a API de chat compatível com a da OpenAI
// (/chat/completions): Ollama, OpenAI, Azure OpenAI, Google Gemini,
// Anthropic, Groq, OpenRouter, Mistral, vLLM, LiteLLM.
package iaconfig

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Provedor identifica o fornecedor (define o endereço sugerido, o
// cabeçalho da chave e se os dados saem da rede).
type Provedor string

const (
	ProvedorLocal      Provedor = "ollama"
	ProvedorOpenAI     Provedor = "openai"
	ProvedorAzure      Provedor = "azure"
	ProvedorGemini     Provedor = "gemini"
	ProvedorAnthropic  Provedor = "anthropic"
	ProvedorGroq       Provedor = "groq"
	ProvedorOpenRouter Provedor = "openrouter"
	ProvedorMistral    Provedor = "mistral"
	ProvedorCompativel Provedor = "compativel"
)

// Modelo pronto de um fornecedor, para a tela pré-preencher a conexão.
type Modelo struct {
	Provedor Provedor `json:"provedor"`
	Nome     string   `json:"nome"`
	Endpoint string   `json:"endpoint"`
	// ModeloSugerido é só uma sugestão (o catálogo de cada fornecedor muda).
	ModeloSugerido string `json:"modelo_sugerido"`
	// Externo: os dados saem da rede da instituição.
	Externo bool `json:"externo"`
	// ExigeChave: o fornecedor recusa chamadas sem chave de API.
	ExigeChave bool `json:"exige_chave"`
}

// Provedores é o catálogo exibido na tela (a ordem é a de exibição).
var Provedores = []Modelo{
	{ProvedorLocal, "IA local (Ollama, no servidor)", "http://ia-local:11434", "qwen2.5:1.5b", false, false},
	{ProvedorOpenAI, "OpenAI", "https://api.openai.com/v1", "gpt-4o-mini", true, true},
	{ProvedorAzure, "Azure OpenAI", "https://<recurso>.openai.azure.com/openai/v1", "<nome da implantação>", true, true},
	{ProvedorGemini, "Google Gemini", "https://generativelanguage.googleapis.com/v1beta/openai", "gemini-2.0-flash", true, true},
	{ProvedorAnthropic, "Anthropic (Claude)", "https://api.anthropic.com/v1", "claude-haiku-4-5", true, true},
	{ProvedorGroq, "Groq", "https://api.groq.com/openai/v1", "llama-3.3-70b-versatile", true, true},
	{ProvedorOpenRouter, "OpenRouter", "https://openrouter.ai/api/v1", "openai/gpt-4o-mini", true, true},
	{ProvedorMistral, "Mistral", "https://api.mistral.ai/v1", "mistral-small-latest", true, true},
	{ProvedorCompativel, "Outra compatível com a API da OpenAI", "", "", false, false},
}

// modelo devolve o catálogo do fornecedor (ok=false se desconhecido).
func modelo(p Provedor) (Modelo, bool) {
	for _, m := range Provedores {
		if m.Provedor == p {
			return m, true
		}
	}
	return Modelo{}, false
}

// Funções que usam IA. Cada uma tem principal e reserva (Uso).
const FuncaoAtlasAssistente = "atlas.assistente"

// Funcoes lista as funções configuráveis.
var Funcoes = []string{FuncaoAtlasAssistente}

// FuncaoValida informa se a função existe.
func FuncaoValida(f string) bool {
	for _, x := range Funcoes {
		if x == f {
			return true
		}
	}
	return false
}

// Teste é o resultado da última conversa de teste de uma conexão.
type Teste struct {
	OK         bool      `json:"ok"`
	Em         time.Time `json:"em"`
	LatenciaMs int       `json:"latencia_ms"`
	Erro       string    `json:"erro"`
}

// Conexao é um fornecedor configurado. Chave fica só na memória do servidor
// (cifrada no banco); a API devolve apenas o final dela (ver Publica).
type Conexao struct {
	ID              uuid.UUID
	Nome            string
	Provedor        Provedor
	Endpoint        string
	Modelo          string
	Chave           string
	Externo         bool
	TimeoutSegundos int
	UltimoTeste     *Teste
	UpdatedAt       time.Time
	UpdatedBy       string
}

// Timeout da conexão.
func (c Conexao) Timeout() time.Duration { return time.Duration(c.TimeoutSegundos) * time.Second }

// ConexaoPublica é a conexão como a API a devolve: sem a chave.
type ConexaoPublica struct {
	ID              uuid.UUID `json:"id"`
	Nome            string    `json:"nome"`
	Provedor        Provedor  `json:"provedor"`
	Endpoint        string    `json:"endpoint"`
	Modelo          string    `json:"modelo"`
	Externo         bool      `json:"externo"`
	TimeoutSegundos int       `json:"timeout_segundos"`
	TemChave        bool      `json:"tem_chave"`
	// ChaveFinal: os 4 últimos caracteres, para reconhecer qual chave está
	// salva (vazio se a chave tem menos de 12 caracteres).
	ChaveFinal  string    `json:"chave_final"`
	UltimoTeste *Teste    `json:"ultimo_teste"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
}

// Publica remove a chave (fica só o final).
func (c Conexao) Publica() ConexaoPublica {
	final := ""
	if utf8.RuneCountInString(c.Chave) >= 12 {
		final = c.Chave[len(c.Chave)-4:]
	}
	return ConexaoPublica{ID: c.ID, Nome: c.Nome, Provedor: c.Provedor, Endpoint: c.Endpoint, Modelo: c.Modelo,
		Externo: c.Externo, TimeoutSegundos: c.TimeoutSegundos, TemChave: c.Chave != "", ChaveFinal: final,
		UltimoTeste: c.UltimoTeste, UpdatedAt: c.UpdatedAt, UpdatedBy: c.UpdatedBy}
}

// Uso liga uma função às conexões. Configurado=false: nada foi salvo pela
// tela (vale o ambiente). PrincipalID nil com Configurado: IA desligada.
type Uso struct {
	Funcao                string     `json:"funcao"`
	Configurado           bool       `json:"configurado"`
	PrincipalID           *uuid.UUID `json:"principal_id"`
	ReservaID             *uuid.UUID `json:"reserva_id"`
	MascararDadosPessoais bool       `json:"mascarar_dados_pessoais"`
	ExternoAutorizadoPor  string     `json:"externo_autorizado_por"`
	ExternoAutorizadoEm   *time.Time `json:"externo_autorizado_em"`
	UpdatedAt             *time.Time `json:"updated_at"`
	UpdatedBy             string     `json:"updated_by"`
}

// Erros de domínio (traduzidos para HTTP em transport.go).
var (
	ErrNaoEncontrada = errors.New("iaconfig: conexão não encontrada")
	ErrNomeRepetido  = errors.New("iaconfig: já existe conexão com este nome")
	ErrEmUso         = errors.New("iaconfig: conexão em uso por uma função")
	// ErrSemConexao: a função não tem IA (desligada ou nada configurado).
	ErrSemConexao = errors.New("iaconfig: nenhuma conexão de IA configurada para a função")
)

// InvalidaError é uma regra de cadastro violada (422 com a mensagem).
type InvalidaError struct{ Msg string }

func (e InvalidaError) Error() string { return e.Msg }

func invalida(format string, a ...any) error { return InvalidaError{Msg: fmt.Sprintf(format, a...)} }

var modeloValido = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]{0,119}$`)

// Normalizar apara os textos e aplica o que o catálogo impõe: fornecedor
// de nuvem é sempre externo; a IA local nunca é.
func (c *Conexao) Normalizar() {
	c.Nome = strings.TrimSpace(c.Nome)
	c.Endpoint = strings.TrimRight(strings.TrimSpace(c.Endpoint), "/")
	c.Modelo = strings.TrimSpace(c.Modelo)
	c.Chave = strings.TrimSpace(c.Chave)
	if c.TimeoutSegundos == 0 {
		c.TimeoutSegundos = 30
	}
	if m, ok := modelo(c.Provedor); ok && c.Provedor != ProvedorCompativel {
		c.Externo = m.Externo
	}
}

// Validar confere a conexão (chame Normalizar antes). chaveSalva: edição
// que mantém a chave já gravada — não exige uma nova.
func (c Conexao) Validar(chaveSalva bool) error {
	m, ok := modelo(c.Provedor)
	switch {
	case !ok:
		return invalida("fornecedor desconhecido: %q", c.Provedor)
	case c.Nome == "" || utf8.RuneCountInString(c.Nome) > 80:
		return invalida("nome obrigatório (até 80 caracteres)")
	case !modeloValido.MatchString(c.Modelo):
		return invalida("modelo obrigatório (até 120 caracteres: letras, números e . _ : / @ + -)")
	case c.TimeoutSegundos < 1 || c.TimeoutSegundos > 300:
		return invalida("tempo limite entre 1 e 300 segundos")
	case len(c.Chave) > 4096:
		return invalida("chave de API longa demais")
	case m.ExigeChave && c.Chave == "" && !chaveSalva:
		return invalida("%s exige a chave de API", m.Nome)
	}
	return ValidarEndpoint(c.Endpoint)
}

// ValidarEndpoint aceita só URL http(s) com host, sem credenciais, consulta
// ou fragmento, e recusa endereços link-local (metadados de nuvem,
// 169.254.0.0/16 e fe80::/10) — o endereço é digitado pelo administrador,
// mas a API é quem faz a requisição.
func ValidarEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(u.Host, "<>") {
		return invalida("endereço inválido: use uma URL http(s) completa, sem usuário e senha (a chave vai no campo próprio)")
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); (ip != nil && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified())) ||
		strings.EqualFold(host, "metadata.google.internal") {
		return invalida("endereço não permitido: %s", host)
	}
	return nil
}

// urlChat monta o endereço de /chat/completions: sem caminho, presume a
// raiz de um servidor compatível (acrescenta /v1, como no Ollama); com
// caminho, usa o caminho dado (Gemini usa /v1beta/openai).
func urlChat(endpoint string) string {
	base := strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if u, err := url.Parse(base); err == nil && (u.Path == "" || u.Path == "/") {
		base += "/v1"
	}
	return base + "/chat/completions"
}

// Dados pessoais mascarados antes de a pergunta sair para um fornecedor
// externo. Ordem importa: CNPJ antes de CPF e de telefone.
var mascaras = []struct {
	re  *regexp.Regexp
	por string
}{
	{regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`), "[e-mail]"},
	{regexp.MustCompile(`\b\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}\b`), "[CNPJ]"},
	{regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`), "[CPF]"},
	{regexp.MustCompile(`(\(?\b\d{2}\)?[\s.-]?)?\b9?\d{4}[\s.-]?\d{4}\b`), "[telefone]"},
}

// MascararDadosPessoais troca e-mail, CNPJ, CPF e telefone por marcadores.
func MascararDadosPessoais(texto string) string {
	for _, m := range mascaras {
		texto = m.re.ReplaceAllString(texto, m.por)
	}
	return texto
}
