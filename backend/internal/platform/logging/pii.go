package logging

// Higienização e mascaramento de PII (LGPD — Lei 13.709/2018, M02).
//
// Duas camadas:
//   - Tipos LogValuer (PIICPF, PIIEmail, PIIPhone, SecretString) para quem
//     sabe que está logando um dado pessoal;
//   - SanitizeAttr, plugado como ReplaceAttr do handler slog em New():
//     intercepta TODO atributo antes da saída, redige chaves sensíveis
//     (senha, token, segredo, authorization) e mascara padrões de CPF,
//     e-mail e telefone encontrados em qualquer string — defesa para o
//     log que alguém escreveu sem pensar em LGPD.
//
// Formatos (skill §5):
//   CPF      123.456.789-01      -> ***.456.789-**
//   E-mail   usuario@dominio.br  -> u***o@dominio.br
//   Telefone (66) 99876-1234     -> (**) *****-1234

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// SecretString oculta completamente o valor quando passado para o slog.
type SecretString string

func (s SecretString) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

// PIICPF mascara um CPF.
type PIICPF string

func (c PIICPF) LogValue() slog.Value { return slog.StringValue(MaskCPF(string(c))) }

// PIIEmail mascara um e-mail.
type PIIEmail string

func (e PIIEmail) LogValue() slog.Value { return slog.StringValue(MaskEmail(string(e))) }

// PIIPhone mascara um telefone.
type PIIPhone string

func (p PIIPhone) LogValue() slog.Value { return slog.StringValue(MaskPhone(string(p))) }

func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

// MaskCPF mantém visíveis só os 6 dígitos centrais: ***.456.789-**.
func MaskCPF(cpf string) string {
	d := digitsOnly(cpf)
	if len(d) != 11 {
		return "***.***.***-**"
	}
	return "***." + d[3:6] + "." + d[6:9] + "-**"
}

// MaskEmail mantém a primeira e a última letra do usuário e o domínio:
// u***o@dominio.gov.br.
func MaskEmail(email string) string {
	user, domain, ok := strings.Cut(email, "@")
	if !ok || user == "" || domain == "" {
		return "***@***"
	}
	r := []rune(user)
	if len(r) == 1 {
		return string(r[0]) + "***@" + domain
	}
	return string(r[0]) + "***" + string(r[len(r)-1]) + "@" + domain
}

// MaskPhone mantém só os 4 últimos dígitos: (**) *****-1234.
func MaskPhone(phone string) string {
	d := digitsOnly(phone)
	if len(d) < 8 {
		return "(**) *****-****"
	}
	return "(**) *****-" + d[len(d)-4:]
}

var (
	cpfPattern   = regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`)
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phonePattern = regexp.MustCompile(`\(?\b\d{2}\)?[\s\-]?9?\d{4}[\s\-]?\d{4}\b`)
)

// sensitiveKeys são nomes de atributo cujo valor é sempre redigido.
var sensitiveKeys = []string{
	"password", "senha", "secret", "segredo", "token", "authorization",
	"cookie", "client_secret", "private_key", "api_key", "apikey",
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// SanitizeString mascara CPF, e-mail e telefone encontrados em s.
func SanitizeString(s string) string {
	if s == "" {
		return s
	}
	s = emailPattern.ReplaceAllStringFunc(s, MaskEmail)
	s = cpfPattern.ReplaceAllStringFunc(s, MaskCPF)
	s = phonePattern.ReplaceAllStringFunc(s, MaskPhone)
	return s
}

// SanitizeAttr é o ReplaceAttr do handler slog da plataforma.
func SanitizeAttr(_ []string, a slog.Attr) slog.Attr {
	// Campos de correlação são UUIDs/IDs técnicos — nunca PII, e o
	// padrão de telefone poderia casar trechos numéricos deles.
	switch a.Key {
	case slog.TimeKey, slog.LevelKey, "request_id", "correlation_id", "user_id", "trace_id", "span_id", "id", "event_id":
		return a
	}
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, "[REDACTED]")
	}
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, SanitizeString(a.Value.String()))
	case slog.KindAny:
		// Erros (slog.Any("error", err)) e Stringers também passam pelo
		// filtro: uma mensagem de erro que carregue um e-mail ou CPF não
		// pode sair em claro só por não ser uma string.
		switch v := a.Value.Any().(type) {
		case error:
			return slog.String(a.Key, SanitizeString(v.Error()))
		case fmt.Stringer:
			return slog.String(a.Key, SanitizeString(v.String()))
		}
	}
	return a
}
