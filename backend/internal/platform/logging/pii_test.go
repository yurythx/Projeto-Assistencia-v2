package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestMaskFormats(t *testing.T) {
	cases := []struct{ got, want string }{
		{MaskCPF("123.456.789-01"), "***.456.789-**"},
		{MaskCPF("12345678901"), "***.456.789-**"},
		{MaskCPF("123"), "***.***.***-**"},
		{MaskEmail("usuario@dominio.gov.br"), "u***o@dominio.gov.br"},
		{MaskEmail("a@x.br"), "a***@x.br"},
		{MaskEmail("invalido"), "***@***"},
		{MaskPhone("(66) 99876-1234"), "(**) *****-1234"},
		{MaskPhone("6634115000"), "(**) *****-5000"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestSanitizeAttrInterceptsLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: SanitizeAttr}))
	logger.Info("contato recebido de joao.silva@prefeitura.gov.br cpf 123.456.789-01",
		slog.String("telefone", "(66) 99876-1234"),
		slog.String("password", "hunter2"),
		slog.String("request_id", "550e8400-e29b-41d4-a716-446655440000"),
	)
	out := buf.String()
	for _, leak := range []string{"joao.silva@", "123.456.789-01", "99876-1234", "hunter2"} {
		if strings.Contains(out, leak) {
			t.Errorf("vazamento de %q no log: %s", leak, out)
		}
	}
	for _, want := range []string{"j***a@prefeitura.gov.br", "***.456.789-**", "(**) *****-1234", "[REDACTED]", "550e8400-e29b-41d4-a716-446655440000"} {
		if !strings.Contains(out, want) {
			t.Errorf("esperado %q no log: %s", want, out)
		}
	}
}
