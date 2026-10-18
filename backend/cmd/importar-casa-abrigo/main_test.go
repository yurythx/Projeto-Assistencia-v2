package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
)

func TestArgumentos(t *testing.T) {
	ctx := context.Background()
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	un := uuid.NewString()
	casos := []struct {
		nome   string
		args   []string
		env    map[string]string
		trecho string
	}{
		{"flag desconhecida", []string{"-x"}, nil, "not defined"},
		{"sem unidade", nil, nil, "-unidade"},
		{"sem autor", []string{"-unidade", un}, nil, "-autor"},
		{"sem banco do 4.0", []string{"-unidade", un, "-autor", "admin"}, map[string]string{"LEGADO_ENCRYPTION_KEY": "k"}, "LEGADO_DATABASE_URL"},
	}
	if err := run(ctx, []string{"-h"}, env(nil), &bytes.Buffer{}); err != nil {
		t.Errorf("-h: %v", err)
	}
	for _, c := range casos {
		var out bytes.Buffer
		if err := run(ctx, c.args, env(c.env), &out); err == nil || !strings.Contains(err.Error(), c.trecho) {
			t.Errorf("%s: %v", c.nome, err)
		}
	}
}

// Simulação de ponta a ponta contra o banco de teste (o "4.0" é um schema
// vazio nele mesmo).
func TestSimulacao(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	pool := dbtest.Pool(t)
	ctx := context.Background()
	schema := "Casa Abrigo cmd" + uuid.NewString()[:6]
	if _, err := pool.Exec(ctx, `CREATE SCHEMA `+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP SCHEMA `+pgx.Identifier{schema}.Sanitize()+` CASCADE`)
	})
	un := dbtest.Unidade(t, pool)
	if _, err := pool.Exec(ctx, `INSERT INTO sempras_unidades (unidade_id, tipo, painel_exibe_nome) VALUES ($1, 'casa_abrigo', false)`, un); err != nil {
		t.Fatal(err)
	}
	var username string
	if err := pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, dbtest.User(t, pool)).Scan(&username); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"LEGADO_DATABASE_URL": url, "LEGADO_ENCRYPTION_KEY": "k"}
	arq := filepath.Join(t.TempDir(), "rel.json")
	var out bytes.Buffer
	if err := run(ctx, []string{"-unidade", un.String(), "-autor", username, "-schema", schema, "-relatorio", arq},
		func(k string) string { return env[k] }, &out); err != nil {
		t.Fatal(err)
	}
	var rel map[string]any
	b, _ := os.ReadFile(arq)
	if !strings.Contains(out.String(), "SIMULAÇÃO") || json.Unmarshal(b, &rel) != nil || rel["aplicada"] != false {
		t.Errorf("saída: %s\nrelatório: %s", out.String(), b)
	}
	if err := run(ctx, []string{"-unidade", un.String(), "-autor", "ninguem-" + uuid.NewString()}, func(k string) string { return env[k] }, &out); err == nil {
		t.Error("autor inexistente")
	}
	env["LEGADO_DATABASE_URL"] = "postgres://x@127.0.0.1:1/x?connect_timeout=1"
	if err := run(ctx, []string{"-unidade", un.String(), "-autor", username}, func(k string) string { return env[k] }, &out); err == nil {
		t.Error("banco do 4.0 fora")
	}
}
