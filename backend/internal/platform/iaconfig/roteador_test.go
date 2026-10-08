package iaconfig

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

// memStore é o Store em memória, com falhas injetáveis por método.
type memStore struct {
	conexoes                                                     map[uuid.UUID]Conexao
	uso                                                          *Uso
	errListar, errObter, errSalvar, errExcluir, errTeste, errUso error
	errDefinir                                                   error
	testes                                                       int
}

func novoMem(cs ...Conexao) *memStore {
	m := &memStore{conexoes: map[uuid.UUID]Conexao{}}
	for _, c := range cs {
		m.conexoes[c.ID] = c
	}
	return m
}

func (m *memStore) Listar(context.Context) ([]Conexao, error) {
	var out []Conexao
	for _, c := range m.conexoes {
		out = append(out, c)
	}
	return out, m.errListar
}

func (m *memStore) Obter(_ context.Context, id uuid.UUID) (Conexao, error) {
	if m.errObter != nil {
		return Conexao{}, m.errObter
	}
	c, ok := m.conexoes[id]
	if !ok {
		return c, ErrNaoEncontrada
	}
	return c, nil
}

func (m *memStore) Salvar(_ context.Context, c Conexao, remover bool, por string) (Conexao, error) {
	if m.errSalvar != nil {
		return c, m.errSalvar
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	} else if c.Chave == "" && !remover {
		c.Chave = m.conexoes[c.ID].Chave
	}
	c.UpdatedBy = por
	m.conexoes[c.ID] = c
	return c, nil
}

func (m *memStore) Excluir(_ context.Context, id uuid.UUID) error {
	if m.errExcluir != nil {
		return m.errExcluir
	}
	delete(m.conexoes, id)
	return nil
}

func (m *memStore) RegistrarTeste(context.Context, uuid.UUID, Teste) error {
	m.testes++
	return m.errTeste
}

func (m *memStore) Uso(_ context.Context, f string) (Uso, error) {
	if m.uso == nil {
		return Uso{Funcao: f, MascararDadosPessoais: true}, m.errUso
	}
	return *m.uso, m.errUso
}

func (m *memStore) DefinirUso(_ context.Context, u Uso, por string) (Uso, error) {
	u.Configurado, u.UpdatedBy = true, por
	m.uso = &u
	return u, m.errDefinir
}

func TestConexaoDoAmbiente(t *testing.T) {
	if ConexaoDoAmbiente(" ", "k", "m", 9) != nil {
		t.Fatal("sem endereço, sem conexão")
	}
	c := ConexaoDoAmbiente("http://x", "k", "m", 0)
	if c.TimeoutSegundos != 1 || c.Externo || c.Chave != "k" {
		t.Fatalf("ambiente: %+v", c)
	}
	if NovoRoteador(novoMem(), NovoCliente(), c, logger).Ambiente() != c {
		t.Fatal("Ambiente()")
	}
}

func TestRoteador(t *testing.T) {
	ctx := context.Background()
	var recebida string
	ok := fornecedor(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Messages []struct{ Content string } }
		_ = json.NewDecoder(r.Body).Decode(&body)
		recebida = body.Messages[1].Content
		responde("resposta")(w, r)
	})
	pedido := Pedido{Sistema: "s", Pergunta: "meu CPF é 123.456.789-09", Montar: func(p string) string { return "P: " + p }}
	principal := Conexao{ID: uuid.New(), Nome: "principal", Endpoint: "http://127.0.0.1:1", Modelo: "m", TimeoutSegundos: 5}
	reserva := Conexao{ID: uuid.New(), Nome: "reserva", Endpoint: ok.URL, Modelo: "m", TimeoutSegundos: 5, Externo: true}
	ambiente := &Conexao{Nome: "ambiente", Endpoint: ok.URL, Modelo: "m", TimeoutSegundos: 5}

	// Nada configurado: ambiente, se houver.
	if _, err := NovoRoteador(novoMem(), NovoCliente(), nil, logger).Conversar(ctx, FuncaoAtlasAssistente, pedido); !errors.Is(err, ErrSemConexao) {
		t.Fatalf("sem ambiente: %v", err)
	}
	r, err := NovoRoteador(novoMem(), NovoCliente(), ambiente, logger).Conversar(ctx, FuncaoAtlasAssistente, pedido)
	if err != nil || r.Conexao != "ambiente" || recebida != "P: meu CPF é 123.456.789-09" {
		t.Fatalf("ambiente: %+v %v %q", r, err, recebida)
	}

	// Principal fora: a reserva (externa) responde, com a pergunta mascarada.
	m := novoMem(principal, reserva)
	m.uso = &Uso{Configurado: true, PrincipalID: &principal.ID, ReservaID: &reserva.ID, MascararDadosPessoais: true}
	r, err = NovoRoteador(m, NovoCliente(), ambiente, logger).Conversar(ctx, FuncaoAtlasAssistente, pedido)
	if err != nil || r.Texto != "resposta" || r.Conexao != "reserva" || recebida != "P: meu CPF é [CPF]" {
		t.Fatalf("reserva: %+v %v %q", r, err, recebida)
	}
	// Sem mascaramento pedido, a pergunta segue como veio.
	m.uso.MascararDadosPessoais = false
	_, _ = NovoRoteador(m, NovoCliente(), nil, logger).Conversar(ctx, FuncaoAtlasAssistente, pedido)
	if recebida != "P: meu CPF é 123.456.789-09" {
		t.Fatalf("sem mascarar: %q", recebida)
	}
	// Todas fora: o erro da última.
	m.uso.ReservaID = nil
	if _, err := NovoRoteador(m, NovoCliente(), nil, logger).Conversar(ctx, FuncaoAtlasAssistente, pedido); err == nil {
		t.Fatal("todas fora")
	}
	// Desligada pela tela (principal nula), mesmo com ambiente.
	m.uso.PrincipalID = nil
	if _, err := NovoRoteador(m, NovoCliente(), ambiente, logger).Conversar(ctx, FuncaoAtlasAssistente, pedido); !errors.Is(err, ErrSemConexao) {
		t.Fatalf("desligada: %v", err)
	}
	// Falhas do armazenamento sobem.
	m.uso.PrincipalID = &principal.ID
	m.errObter = errors.New("db")
	if _, err := NovoRoteador(m, NovoCliente(), nil, logger).Conversar(ctx, FuncaoAtlasAssistente, pedido); err == nil || errors.Is(err, ErrSemConexao) {
		t.Fatalf("obter: %v", err)
	}
	m.errUso = errors.New("db")
	if _, err := NovoRoteador(m, NovoCliente(), nil, logger).Conversar(ctx, FuncaoAtlasAssistente, pedido); err == nil {
		t.Fatal("uso")
	}
}
