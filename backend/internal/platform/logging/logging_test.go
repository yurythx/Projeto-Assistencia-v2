package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type stringer string

func (s stringer) String() string { return string(s) }

func TestLoggerMasksPIIInEveryValueKind(t *testing.T) {
	var buf bytes.Buffer
	log := newLogger(&buf, Options{Level: "debug", Format: "json", Service: "api", Environment: "test"})
	log.Info("x",
		slog.Any("error", errors.New("conta maria@orgao.gov.br com CPF 123.456.789-09 já existe")),
		slog.Any("alvo", stringer("ligue (61) 99999-1234")),
		slog.Int("tentativas", 3),
		slog.String("senha", "segredo"),
		slog.Any("conta", PIIEmail("ana@orgao.gov.br")),
		slog.Any("doc", PIICPF("12345678909")),
		slog.Any("tel", PIIPhone("61999991234")),
		slog.Any("chave", SecretString("abc")),
	)
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, leak := range []string{"maria@orgao.gov.br", "123.456.789-09", "99999-1234", "segredo", "ana@orgao.gov.br", "12345678909", "abc\""} {
		if strings.Contains(out, leak) {
			t.Errorf("vazou %q no log: %s", leak, out)
		}
	}
	if rec["service"] != "api" || rec["environment"] != "test" || rec["tentativas"] != float64(3) {
		t.Fatalf("atributos fixos e não-texto intactos: %v", rec)
	}
}

func TestLevelsAndTextFormat(t *testing.T) {
	for level, want := range map[string]slog.Level{"debug": slog.LevelDebug, "WARNING": slog.LevelWarn, "warn": slog.LevelWarn, "error": slog.LevelError, "?": slog.LevelInfo} {
		if got := parseLevel(level); got != want {
			t.Errorf("%s: %v", level, got)
		}
	}
	var buf bytes.Buffer
	newLogger(&buf, Options{Level: "warn", Format: "TEXT"}).Info("some")
	newLogger(&buf, Options{Level: "warn", Format: "text"}).Warn("aparece")
	if strings.Contains(buf.String(), "some") || !strings.Contains(buf.String(), "level=WARN") {
		t.Fatalf("texto e nível: %s", buf.String())
	}
	if New(Options{}) == nil {
		t.Fatal("construtor público")
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := WithUserID(WithCorrelationID(WithRequestID(context.Background(), "r1"), "c1"), "u1")
	if RequestID(ctx) != "r1" || CorrelationID(ctx) != "c1" || UserID(ctx) != "u1" {
		t.Fatal("ids no contexto")
	}
	if RequestID(context.Background()) != "" || CorrelationID(context.Background()) != "" || UserID(context.Background()) != "" {
		t.Fatal("contexto vazio")
	}
	var buf bytes.Buffer
	FromContext(ctx, newLogger(&buf, Options{})).Info("x")
	for _, k := range []string{`"request_id":"r1"`, `"correlation_id":"c1"`, `"user_id":"u1"`} {
		if !strings.Contains(buf.String(), k) {
			t.Errorf("faltou %s: %s", k, buf.String())
		}
	}
}

func TestMaskPhoneTooShort(t *testing.T) {
	if got := MaskPhone("123"); got != "(**) *****-****" {
		t.Fatalf("telefone curto demais é mascarado por inteiro: %q", got)
	}
}
