// Command importar-casa-abrigo traz o histórico do Casa Abrigo 4.0 para a
// Casa Abrigo do SEMPRAS (ADR 033). Sempre simula primeiro: sem -aplicar,
// roda a importação inteira e desfaz no fim, imprimindo o relatório do que
// entraria, do que entra com ajuste e do que fica de fora.
//
// O banco de destino vem das mesmas variáveis DB_* da API. O do 4.0 e a
// chave dele vêm só do ambiente — nunca de argumento (que fica no
// histórico do shell e na lista de processos):
//
//	LEGADO_DATABASE_URL=postgres://...  LEGADO_ENCRYPTION_KEY=...  \
//	  ./importar-casa-abrigo -unidade <uuid da Casa Abrigo> -autor <username>
//	# conferido o relatório:
//	  ./importar-casa-abrigo -unidade ... -autor ... -aplicar
//
// Pode rodar quantas vezes for preciso: o que já entrou não entra de novo.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/importacao"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/config"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "importar-casa-abrigo:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer) error {
	fs := flag.NewFlagSet("importar-casa-abrigo", flag.ContinueOnError)
	fs.SetOutput(out)
	unidade := fs.String("unidade", "", "id da Casa Abrigo de destino (obrigatório)")
	autor := fs.String("autor", "", "username de quem roda a importação (obrigatório; fica na auditoria)")
	aplicar := fs.Bool("aplicar", false, "grava de verdade (sem isto, só simula)")
	schema := fs.String("schema", importacao.SchemaLegado, "schema do banco do 4.0")
	relatorio := fs.String("relatorio", "", "arquivo para o relatório em JSON (opcional)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	un, err := uuid.Parse(*unidade)
	if err != nil {
		return errors.New("-unidade: informe o id da Casa Abrigo de destino")
	}
	if *autor == "" {
		return errors.New("-autor: informe o username de quem roda a importação")
	}
	urlLegado, chave := getenv("LEGADO_DATABASE_URL"), getenv("LEGADO_ENCRYPTION_KEY")
	if urlLegado == "" || chave == "" {
		return errors.New("defina LEGADO_DATABASE_URL e LEGADO_ENCRYPTION_KEY no ambiente")
	}
	dbCfg, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("banco de destino (DB_*): %w", err)
	}
	destino, err := pgxpool.New(ctx, dbCfg.DSN())
	if err != nil {
		return fmt.Errorf("banco de destino: %w", err)
	}
	defer destino.Close()
	origem, err := pgx.Connect(ctx, urlLegado)
	if err != nil {
		return fmt.Errorf("banco do 4.0: %w", err)
	}
	defer origem.Close(context.Background())

	var autorID uuid.UUID
	if err := destino.QueryRow(ctx, `SELECT id FROM users WHERE username = $1 AND active`, *autor).Scan(&autorID); err != nil {
		return fmt.Errorf("-autor %q: usuário ativo não encontrado: %w", *autor, err)
	}
	// leitura consistente do 4.0 (uma foto só, mesmo com a unidade usando)
	tx, err := origem.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("banco do 4.0: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	leg, err := importacao.Ler(ctx, tx, *schema)
	if err != nil {
		return fmt.Errorf("lendo o 4.0: %w", err)
	}
	rel, err := importacao.Importar(ctx, destino, infrastructure.NewRepository(), leg, importacao.NovoDecifrador(chave),
		importacao.Opcoes{Unidade: un, Autor: autorID, Aplicar: *aplicar, Agora: time.Now()})
	if err != nil {
		return err
	}
	fmt.Fprint(out, rel.Resumo())
	if *relatorio == "" {
		return nil
	}
	b, _ := json.MarshalIndent(rel, "", "  ")
	return os.WriteFile(*relatorio, b, 0o600)
}
