// Command migrate aplica as migrations do Nexus (goose, SQL embutido).
//
//	migrate up | down | down-to <versão> | status | redo | version| down | status | redo | version
//
// Usa as mesmas variáveis DB_* da API (config.LoadDatabase). No
// docker-compose roda como serviço one-shot antes da API e do worker.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/migrations"
)

func main() {
	cmd, args := "up", []string{}
	if len(os.Args) > 1 {
		cmd, args = os.Args[1], os.Args[2:]
	}
	if err := run(cmd, args); err != nil {
		fmt.Fprintln(os.Stderr, "migrate: fatal:", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	dbCfg, err := config.LoadDatabase()
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", dbCfg.DSN())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// O Postgres pode ainda estar subindo quando o container de migração
	// inicia: tenta por até 60s antes de desistir.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for {
		if err = db.PingContext(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("database unreachable: %w", err)
		case <-time.After(2 * time.Second):
		}
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.RunContext(context.Background(), cmd, db, ".", args...)
}
