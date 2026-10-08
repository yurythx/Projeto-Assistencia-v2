package iaconfig

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
)

func cifra(t *testing.T, b byte) *secretcrypto.Cipher {
	t.Helper()
	c, err := secretcrypto.NewFromBase64Key(base64.StdEncoding.EncodeToString([]byte(strings.Repeat(string(rune(b)), 32))))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPostgresStore(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	s := NewPostgresStore(pool, cifra(t, 'a'))
	sfx := uuid.NewString()[:8]
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ia_uso`)
		_, _ = pool.Exec(ctx, `DELETE FROM ia_conexoes WHERE nome LIKE '%' || $1`, sfx)
	})

	local, err := s.Salvar(ctx, Conexao{Nome: "Local " + sfx, Provedor: ProvedorLocal, Endpoint: "http://ia-local:11434",
		Modelo: "qwen2.5:1.5b", TimeoutSegundos: 60}, false, "admin")
	if err != nil || local.ID == uuid.Nil || local.UltimoTeste != nil || local.UpdatedBy != "admin" || local.Chave != "" {
		t.Fatalf("inserir: %+v %v", local, err)
	}
	nuvem, err := s.Salvar(ctx, Conexao{Nome: "Nuvem " + sfx, Provedor: ProvedorOpenAI, Endpoint: "https://api.openai.com/v1",
		Modelo: "gpt", Chave: "sk-segredo", Externo: true, TimeoutSegundos: 30}, false, "admin")
	if err != nil || nuvem.Chave != "sk-segredo" {
		t.Fatalf("inserir com chave: %+v %v", nuvem, err)
	}
	// A chave fica cifrada no banco.
	var gravada string
	if err := pool.QueryRow(ctx, `SELECT chave_cifrada FROM ia_conexoes WHERE id = $1`, nuvem.ID).Scan(&gravada); err != nil ||
		gravada == "" || strings.Contains(gravada, "segredo") {
		t.Fatalf("chave em claro no banco: %q %v", gravada, err)
	}
	if _, err := s.Salvar(ctx, Conexao{Nome: "Local " + sfx, Provedor: ProvedorLocal, Endpoint: "http://x", Modelo: "m", TimeoutSegundos: 1},
		false, ""); !errors.Is(err, ErrNomeRepetido) {
		t.Fatalf("nome repetido: %v", err)
	}

	// Atualizar: chave vazia mantém; nova troca; remover apaga.
	nuvem.Modelo, nuvem.Chave = "gpt-4o-mini", ""
	if c, err := s.Salvar(ctx, nuvem, false, "outro"); err != nil || c.Chave != "sk-segredo" || c.Modelo != "gpt-4o-mini" || c.UpdatedBy != "outro" {
		t.Fatalf("manter a chave: %+v %v", c, err)
	}
	nuvem.Chave = "sk-nova"
	if c, _ := s.Salvar(ctx, nuvem, false, ""); c.Chave != "sk-nova" {
		t.Fatalf("trocar a chave: %+v", c)
	}
	nuvem.Chave = ""
	if c, _ := s.Salvar(ctx, nuvem, true, ""); c.Chave != "" {
		t.Fatalf("remover a chave: %+v", c)
	}
	if _, err := s.Salvar(ctx, Conexao{ID: uuid.New(), Nome: "x", Provedor: ProvedorLocal, Endpoint: "http://x", Modelo: "m", TimeoutSegundos: 1},
		false, ""); !errors.Is(err, ErrNaoEncontrada) {
		t.Fatalf("atualizar inexistente: %v", err)
	}

	teste := Teste{OK: false, Em: time.Now(), LatenciaMs: 1234, Erro: "sem resposta"}
	if err := s.RegistrarTeste(ctx, local.ID, teste); err != nil {
		t.Fatal(err)
	}
	if c, err := s.Obter(ctx, local.ID); err != nil || c.UltimoTeste == nil || c.UltimoTeste.LatenciaMs != 1234 || c.UltimoTeste.Erro != "sem resposta" {
		t.Fatalf("último teste: %+v %v", c, err)
	}
	if lista, err := s.Listar(ctx); err != nil || len(lista) < 2 {
		t.Fatalf("listar: %d %v", len(lista), err)
	}
	if _, err := s.Obter(ctx, uuid.New()); !errors.Is(err, ErrNaoEncontrada) {
		t.Fatalf("obter inexistente: %v", err)
	}

	// Uso: sem linha = não configurado (mascarar ligado).
	_, _ = pool.Exec(ctx, `DELETE FROM ia_uso`)
	if u, err := s.Uso(ctx, FuncaoAtlasAssistente); err != nil || u.Configurado || !u.MascararDadosPessoais {
		t.Fatalf("uso sem configuração: %+v %v", u, err)
	}
	agora := time.Now()
	u, err := s.DefinirUso(ctx, Uso{Funcao: FuncaoAtlasAssistente, PrincipalID: &local.ID, ReservaID: &nuvem.ID,
		ExternoAutorizadoPor: "admin", ExternoAutorizadoEm: &agora}, "admin")
	if err != nil || !u.Configurado || *u.ReservaID != nuvem.ID || u.MascararDadosPessoais || u.ExternoAutorizadoPor != "admin" {
		t.Fatalf("definir uso: %+v %v", u, err)
	}
	if u, err := s.Uso(ctx, FuncaoAtlasAssistente); err != nil || *u.PrincipalID != local.ID || u.UpdatedBy != "admin" {
		t.Fatalf("uso: %+v %v", u, err)
	}
	// Conexão em uso não é excluída.
	if err := s.Excluir(ctx, nuvem.ID); !errors.Is(err, ErrEmUso) {
		t.Fatalf("excluir em uso: %v", err)
	}
	if _, err := s.DefinirUso(ctx, Uso{Funcao: FuncaoAtlasAssistente, ReservaID: &nuvem.ID}, ""); err == nil {
		t.Fatal("reserva sem principal (CHECK)")
	}
	if _, err := s.DefinirUso(ctx, Uso{Funcao: FuncaoAtlasAssistente}, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Excluir(ctx, nuvem.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Excluir(ctx, nuvem.ID); !errors.Is(err, ErrNaoEncontrada) {
		t.Fatalf("excluir de novo: %v", err)
	}

	// Chave cifrada com outra CONFIG_ENCRYPTION_KEY: erro explícito.
	if _, err := s.Salvar(ctx, Conexao{Nome: "Chave " + sfx, Provedor: ProvedorOpenAI, Endpoint: "https://x", Modelo: "m", Chave: "k", TimeoutSegundos: 1}, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPostgresStore(pool, cifra(t, 'b')).Listar(ctx); err == nil || !strings.Contains(err.Error(), "CONFIG_ENCRYPTION_KEY") {
		t.Fatalf("chave ilegível: %v", err)
	}
}

func TestPostgresStoreFalhas(t *testing.T) {
	ctx := context.Background()
	s := NewPostgresStore(dbtest.Fail{}, cifra(t, 'a'))
	id := uuid.New()
	for nome, err := range map[string]error{
		"listar":  func() error { _, err := s.Listar(ctx); return err }(),
		"obter":   func() error { _, err := s.Obter(ctx, id); return err }(),
		"salvar":  func() error { _, err := s.Salvar(ctx, Conexao{}, false, ""); return err }(),
		"excluir": s.Excluir(ctx, id),
		"teste":   s.RegistrarTeste(ctx, id, Teste{}),
		"uso":     func() error { _, err := s.Uso(ctx, "x"); return err }(),
		"definir": func() error { _, err := s.DefinirUso(ctx, Uso{}, ""); return err }(),
		"linha":   func() error { _, err := NewPostgresStore(dbtest.ScanFail{}, nil).Listar(ctx); return err }(),
		"leitura": func() error { _, err := NewPostgresStore(dbtest.RowsErr{}, nil).Listar(ctx); return err }(),
	} {
		if err == nil || errors.Is(err, ErrNaoEncontrada) {
			t.Errorf("%s com o banco fora: %v", nome, err)
		}
	}
}
