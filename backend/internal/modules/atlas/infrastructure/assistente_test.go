package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/iaconfig"
)

// semUso: nada configurado pela tela (vale a conexão do ambiente).
type semUso struct{ iaconfig.Store }

func (semUso) Uso(_ context.Context, f string) (iaconfig.Uso, error) {
	return iaconfig.Uso{Funcao: f}, nil
}

func roteador(endpoint string) *iaconfig.Roteador {
	return iaconfig.NovoRoteador(semUso{}, iaconfig.NovoCliente(), iaconfig.ConexaoDoAmbiente(endpoint, "k", "m", 5),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestAssistenteIA(t *testing.T) {
	var got struct {
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
		Messages    []struct{ Role, Content string }
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  Orientação  "}}]}`))
	}))
	defer srv.Close()

	out, err := NewAssistenteIA(roteador(srv.URL)).Responder(context.Background(), "Como tramitar?", []string{"ADM.LIC.001 — Pregão", "2.0.02.00.07"})
	if err != nil || out != "Orientação" {
		t.Fatalf("resposta: %q %v", out, err)
	}
	if got.Model != "m" || got.Temperature != 0.05 || len(got.Messages) != 2 || got.Messages[0].Role != "system" ||
		!strings.Contains(got.Messages[0].Content, "EXCLUSIVAMENTE") ||
		!strings.Contains(got.Messages[1].Content, "CONTEXTO HOMOLOGADO:\n\n---\nADM.LIC.001 — Pregão\n---\n2.0.02.00.07") ||
		!strings.HasSuffix(got.Messages[1].Content, "\n\nPERGUNTA: Como tramitar?") {
		t.Fatalf("pedido ao modelo: %+v", got)
	}

	// Falha do fornecedor: o erro sobe (o serviço cai na síntese).
	if _, err := NewAssistenteIA(roteador("http://127.0.0.1:1")).Responder(context.Background(), "x", nil); err == nil ||
		errors.Is(err, domain.ErrIADesligada) {
		t.Fatalf("fornecedor fora: %v", err)
	}
	// Sem conexão nenhuma: IA desligada (síntese, sem aviso).
	sem := iaconfig.NovoRoteador(semUso{}, iaconfig.NovoCliente(), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := NewAssistenteIA(sem).Responder(context.Background(), "x", nil); !errors.Is(err, domain.ErrIADesligada) {
		t.Fatalf("sem conexão: %v", err)
	}
}
