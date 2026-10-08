package iaconfig

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCatalogoEFuncoes(t *testing.T) {
	if _, ok := modelo("x"); ok {
		t.Fatal("fornecedor desconhecido")
	}
	if m, ok := modelo(ProvedorGemini); !ok || !m.Externo || !m.ExigeChave {
		t.Fatalf("gemini: %+v", m)
	}
	if !FuncaoValida(FuncaoAtlasAssistente) || FuncaoValida("x") {
		t.Fatal("funções")
	}
	if (InvalidaError{"m"}).Error() != "m" || (FalhaError{"f"}).Error() != "f" {
		t.Fatal("mensagens de erro")
	}
	if (Conexao{TimeoutSegundos: 3}).Timeout() != 3*time.Second {
		t.Fatal("timeout")
	}
}

func TestPublicaNaoExpoeAChave(t *testing.T) {
	p := Conexao{Nome: "n", Chave: "sk-123456789abcd"}.Publica()
	if !p.TemChave || p.ChaveFinal != "abcd" {
		t.Fatalf("chave longa: %+v", p)
	}
	if p := (Conexao{Chave: "curta"}).Publica(); !p.TemChave || p.ChaveFinal != "" {
		t.Fatalf("chave curta não revela o final: %+v", p)
	}
	if p := (Conexao{}).Publica(); p.TemChave {
		t.Fatal("sem chave")
	}
	b, _ := json.Marshal(Conexao{Chave: "sk-123456789abcd"}.Publica())
	if strings.Contains(string(b), "sk-1234") {
		t.Fatalf("chave no JSON: %s", b)
	}
}

func TestNormalizarEValidar(t *testing.T) {
	c := Conexao{Nome: "  OpenAI  ", Provedor: ProvedorOpenAI, Endpoint: " https://api.openai.com/v1/ ", Modelo: " gpt ", Chave: " k "}
	c.Normalizar()
	if c.Nome != "OpenAI" || c.Endpoint != "https://api.openai.com/v1" || c.Modelo != "gpt" || c.Chave != "k" ||
		c.TimeoutSegundos != 30 || !c.Externo {
		t.Fatalf("normalizar (nuvem é sempre externa): %+v", c)
	}
	local := Conexao{Provedor: ProvedorLocal, Externo: true}
	local.Normalizar()
	compat := Conexao{Provedor: ProvedorCompativel, Externo: true}
	compat.Normalizar()
	if local.Externo || !compat.Externo {
		t.Fatalf("local nunca é externa; compatível segue a escolha: %v %v", local.Externo, compat.Externo)
	}

	ok := Conexao{Nome: "n", Provedor: ProvedorOpenAI, Endpoint: "https://api.openai.com/v1", Modelo: "gpt-4o-mini", Chave: "k", TimeoutSegundos: 30}
	if err := ok.Validar(false); err != nil {
		t.Fatal(err)
	}
	semChave := ok
	semChave.Chave = ""
	if err := semChave.Validar(true); err != nil {
		t.Fatalf("chave já salva: %v", err)
	}
	for nome, mut := range map[string]func(*Conexao){
		"fornecedor":  func(c *Conexao) { c.Provedor = "x" },
		"nome":        func(c *Conexao) { c.Nome = "" },
		"nome longo":  func(c *Conexao) { c.Nome = strings.Repeat("a", 81) },
		"modelo":      func(c *Conexao) { c.Modelo = "com espaço" },
		"timeout":     func(c *Conexao) { c.TimeoutSegundos = 301 },
		"chave longa": func(c *Conexao) { c.Chave = strings.Repeat("k", 4097) },
		"exige chave": func(c *Conexao) { c.Chave = "" },
		"endereço":    func(c *Conexao) { c.Endpoint = "ftp://x" },
	} {
		c := ok
		mut(&c)
		var inv InvalidaError
		if err := c.Validar(false); !errors.As(err, &inv) {
			t.Errorf("%s: %v", nome, err)
		}
	}
}

func TestValidarEndpoint(t *testing.T) {
	for _, bom := range []string{"http://ia-local:11434", "https://api.openai.com/v1", "http://127.0.0.1:8000/v1"} {
		if err := ValidarEndpoint(bom); err != nil {
			t.Errorf("%s: %v", bom, err)
		}
	}
	for _, ruim := range []string{"ftp://x", "http://", "https://u:s@x", "https://x?a=1", "https://x#f", "http://%zz",
		"http://169.254.169.254/latest", "http://[fe80::1]/", "http://0.0.0.0", "http://metadata.google.internal"} {
		if err := ValidarEndpoint(ruim); err == nil {
			t.Errorf("%s aceito", ruim)
		}
	}
}

func TestURLChat(t *testing.T) {
	for in, want := range map[string]string{
		"http://ia-local:11434":                                   "http://ia-local:11434/v1/chat/completions",
		"http://ia-local:11434/":                                  "http://ia-local:11434/v1/chat/completions",
		"https://generativelanguage.googleapis.com/v1beta/openai": "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
		"https://x/v1/chat/completions":                           "https://x/v1/chat/completions",
		"http://%zz":                                              "http://%zz/chat/completions",
	} {
		if got := urlChat(in); got != want {
			t.Errorf("%s -> %s, quero %s", in, got, want)
		}
	}
}

func TestMascararDadosPessoais(t *testing.T) {
	in := "Sou João, CPF 123.456.789-09 (ou 12345678909), empresa 12.345.678/0001-90, e-mail joao.silva@prefeitura.mt.gov.br, " +
		"fone (66) 99999-8888 ou 3411 2233. Série 2.0.07.00.00 de 25/08/2025, edição 6.017."
	got := MascararDadosPessoais(in)
	for _, vazou := range []string{"123.456.789-09", "12345678909", "12.345.678/0001-90", "joao.silva", "99999-8888", "3411 2233"} {
		if strings.Contains(got, vazou) {
			t.Errorf("%q não foi mascarado: %s", vazou, got)
		}
	}
	for _, fica := range []string{"[CPF]", "[CNPJ]", "[e-mail]", "[telefone]", "2.0.07.00.00", "25/08/2025", "6.017", "João"} {
		if !strings.Contains(got, fica) {
			t.Errorf("faltou %q: %s", fica, got)
		}
	}
}

// fornecedor simula a API de chat; handler decide a resposta.
func fornecedor(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func responde(texto string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + texto + `"}}]}`))
	}
}

func TestClienteConversar(t *testing.T) {
	ctx := context.Background()
	var auth, apiKey, caminho string
	var corpo map[string]any
	srv := fornecedor(t, func(w http.ResponseWriter, r *http.Request) {
		auth, apiKey, caminho = r.Header.Get("Authorization"), r.Header.Get("api-key"), r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&corpo)
		responde("  ok  ")(w, r)
	})
	cl := NovoCliente()
	c := Conexao{Provedor: ProvedorOpenAI, Endpoint: srv.URL, Modelo: "m", Chave: "k", TimeoutSegundos: 5}
	if out, err := cl.Conversar(ctx, c, []Mensagem{{"user", "oi"}}, 0.1, 9); err != nil || out != "ok" {
		t.Fatalf("conversa: %q %v", out, err)
	}
	if auth != "Bearer k" || apiKey != "" || caminho != "/v1/chat/completions" || corpo["model"] != "m" || corpo["max_tokens"] != float64(9) {
		t.Fatalf("pedido: %q %q %q %+v", auth, apiKey, caminho, corpo)
	}
	c.Provedor = ProvedorAzure
	_, _ = cl.Conversar(ctx, c, nil, 0, 1)
	if auth != "" || apiKey != "k" {
		t.Fatalf("Azure usa o cabeçalho api-key: %q %q", auth, apiKey)
	}
	c.Chave = ""
	_, _ = cl.Conversar(ctx, c, nil, 0, 1)
	if auth != "" || apiKey != "" {
		t.Fatal("sem chave, sem cabeçalho")
	}
}

func TestClienteFalhas(t *testing.T) {
	ctx := context.Background()
	cl := NovoCliente()
	conv := func(endpoint string) string {
		_, err := cl.Conversar(ctx, Conexao{Endpoint: endpoint, Modelo: "m", TimeoutSegundos: 5}, nil, 0, 1)
		var f FalhaError
		if !errors.As(err, &f) {
			t.Fatalf("%s: erro sem descrição: %v", endpoint, err)
		}
		return f.Msg
	}
	status := func(code int) string {
		return conv(fornecedor(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "segredo do contexto", code) }).URL)
	}
	for code, want := range map[int]string{401: "chave de API recusada", 403: "chave de API recusada", 404: "modelo não encontrado",
		429: "limite de uso", 400: "confira o modelo", 422: "confira o modelo", 500: "HTTP 500"} {
		if got := status(code); !strings.Contains(got, want) || strings.Contains(got, "segredo") {
			t.Errorf("HTTP %d: %s", code, got)
		}
	}
	formato := fornecedor(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) })
	vazia := fornecedor(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"choices":[]}`)) })
	tls := httptest.NewTLSServer(responde("ok"))
	t.Cleanup(tls.Close)
	for endpoint, want := range map[string]string{
		formato.URL:                       "fora do formato",
		vazia.URL:                         "resposta vazia",
		"http://[::1":                     "endereço inválido",
		"http://127.0.0.1:1":              "não foi possível conectar",
		"http://nexus-nao-existe.invalid": "DNS",
		tls.URL:                           "certificado",
	} {
		if got := conv(endpoint); !strings.Contains(got, want) {
			t.Errorf("%s: %s", endpoint, got)
		}
	}

	// Sem resposta no tempo limite.
	lento := fornecedor(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		responde("ok")(w, nil)
	})
	rapido := &Cliente{http: func(time.Duration) *http.Client { return &http.Client{Timeout: 50 * time.Millisecond} }}
	_, err := rapido.Conversar(ctx, Conexao{Endpoint: lento.URL, Modelo: "m", TimeoutSegundos: 7}, nil, 0, 1)
	if err == nil || !strings.Contains(err.Error(), "sem resposta em 7 s") {
		t.Fatalf("tempo limite: %v", err)
	}
}

func TestClienteTestar(t *testing.T) {
	ok := NovoCliente().Testar(context.Background(), Conexao{Endpoint: fornecedor(t, responde("OK")).URL, Modelo: "m", TimeoutSegundos: 5})
	if !ok.OK || ok.Erro != "" || ok.Em.IsZero() {
		t.Fatalf("teste ok: %+v", ok)
	}
	falha := NovoCliente().Testar(context.Background(), Conexao{Endpoint: "http://127.0.0.1:1", Modelo: "m", TimeoutSegundos: 5})
	if falha.OK || falha.Erro == "" {
		t.Fatalf("teste com falha: %+v", falha)
	}
}
