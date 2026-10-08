package iaconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// maxRespostaBytes limita o que se lê do fornecedor (resposta gigante não
// consome a memória da API).
const maxRespostaBytes = 1 << 20

// Mensagem da conversa (papéis da API da OpenAI: system, user, assistant).
type Mensagem struct {
	Papel    string `json:"role"`
	Conteudo string `json:"content"`
}

// Cliente conversa com uma conexão pela API de chat compatível com a da
// OpenAI. http é injetável nos testes.
type Cliente struct {
	http func(timeout time.Duration) *http.Client
}

// NovoCliente cria o cliente HTTP padrão.
func NovoCliente() *Cliente {
	return &Cliente{http: func(t time.Duration) *http.Client { return &http.Client{Timeout: t} }}
}

// FalhaError descreve, para o administrador, por que a conversa falhou —
// sem ecoar o corpo da resposta (pode repetir o contexto enviado).
type FalhaError struct{ Msg string }

func (e FalhaError) Error() string { return e.Msg }

// Conversar envia as mensagens e devolve o texto da resposta.
func (cl *Cliente) Conversar(ctx context.Context, c Conexao, msgs []Mensagem, temperatura float64, maxTokens int) (string, error) {
	// Só strings, números e bool: o Marshal não tem como falhar.
	body, _ := json.Marshal(map[string]any{
		"model": c.Modelo, "messages": msgs, "temperature": temperatura, "max_tokens": maxTokens, "stream": false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlChat(c.Endpoint), bytes.NewReader(body))
	if err != nil {
		return "", FalhaError{"endereço inválido: " + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Chave != "" {
		if c.Provedor == ProvedorAzure {
			req.Header.Set("api-key", c.Chave)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.Chave)
		}
	}
	resp, err := cl.http(c.Timeout()).Do(req)
	if err != nil {
		return "", falhaDeRede(err, c)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxRespostaBytes))
		return "", falhaHTTP(resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRespostaBytes)).Decode(&out); err != nil {
		return "", FalhaError{"resposta fora do formato da API da OpenAI (confira o endereço)"}
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", FalhaError{"o fornecedor devolveu uma resposta vazia"}
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func falhaDeRede(err error, c Conexao) error {
	var ne net.Error
	switch {
	case errors.As(err, &ne) && ne.Timeout(), errors.Is(err, context.DeadlineExceeded):
		return FalhaError{fmt.Sprintf("sem resposta em %d s (aumente o tempo limite ou use um modelo menor)", c.TimeoutSegundos)}
	case strings.Contains(err.Error(), "no such host"):
		return FalhaError{"endereço não encontrado (DNS): confira o endereço"}
	case strings.Contains(err.Error(), "certificate"):
		return FalhaError{"certificado TLS do fornecedor não é confiável"}
	}
	return FalhaError{"não foi possível conectar ao fornecedor: confira o endereço e se o serviço está no ar"}
}

func falhaHTTP(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return FalhaError{fmt.Sprintf("chave de API recusada (HTTP %d)", status)}
	case http.StatusNotFound:
		return FalhaError{"endereço ou modelo não encontrado (HTTP 404) — no Ollama, o modelo foi baixado?"}
	case http.StatusTooManyRequests:
		return FalhaError{"limite de uso do fornecedor atingido (HTTP 429)"}
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return FalhaError{fmt.Sprintf("o fornecedor recusou o pedido (HTTP %d): confira o modelo", status)}
	}
	return FalhaError{fmt.Sprintf("o fornecedor respondeu HTTP %d", status)}
}

// Testar faz uma conversa mínima e mede o tempo de resposta.
func (cl *Cliente) Testar(ctx context.Context, c Conexao) Teste {
	inicio := time.Now()
	_, err := cl.Conversar(ctx, c, []Mensagem{
		{Papel: "system", Conteudo: "Responda apenas com a palavra OK."},
		{Papel: "user", Conteudo: "Teste de conexão."},
	}, 0, 5)
	t := Teste{OK: err == nil, Em: inicio, LatenciaMs: int(time.Since(inicio).Milliseconds())}
	if err != nil {
		t.Erro = err.Error()
	}
	return t
}
