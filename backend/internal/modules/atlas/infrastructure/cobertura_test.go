package infrastructure

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// Cada uma das quatro consultas do painel pode falhar (na execução, na
// linha ou no meio da leitura).
func TestCoberturaFalhas(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	ok := func() dbtest.QueryResult { return dbtest.QueryResult{Rows: dbtest.NoRows()} }
	for name, q := range map[string][]dbtest.QueryResult{
		"órgãos: consulta":  {{Err: dbtest.ErrInjected}},
		"órgãos: linha":     {{Rows: dbtest.BadRows()}},
		"órgãos: leitura":   {{Rows: dbtest.ErrRows()}},
		"peças: consulta":   {ok(), {Err: dbtest.ErrInjected}},
		"peças: linha":      {ok(), {Rows: dbtest.BadRows()}},
		"modelos: consulta": {ok(), ok(), {Err: dbtest.ErrInjected}},
		"modelos: linha":    {ok(), ok(), {Rows: dbtest.BadRows()}},
		"lacunas: consulta": {ok(), ok(), ok(), {Err: dbtest.ErrInjected}},
		"lacunas: linha":    {ok(), ok(), ok(), {Rows: dbtest.BadRows()}},
		"contagens":         {ok(), ok(), ok(), ok()},
	} {
		if _, err := r.Cobertura(ctx, &dbtest.Seq{Queries: q}); err == nil {
			t.Errorf("%s: falha engolida", name)
		}
	}
	var _ pgx.Rows = dbtest.NoRows()
}
