package infrastructure

import (
	"context"
	"testing"

	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// O organograma pode falhar na execução ou na linha.
func TestOrganogramaFalhas(t *testing.T) {
	r := NewRepository()
	for name, q := range map[string]dbtest.QueryResult{
		"consulta": {Err: dbtest.ErrInjected},
		"linha":    {Rows: dbtest.BadRows()},
	} {
		if _, err := r.Organograma(context.Background(), &dbtest.Seq{Queries: []dbtest.QueryResult{q}}); err == nil {
			t.Errorf("%s: falha engolida", name)
		}
	}
}

// A consulta por secretaria pode falhar na execução ou na linha.
func TestProcedimentosPorOrgaoFalhas(t *testing.T) {
	r := NewRepository()
	for name, q := range map[string]dbtest.QueryResult{
		"consulta": {Err: dbtest.ErrInjected},
		"linha":    {Rows: dbtest.BadRows()},
	} {
		if _, err := r.ProcedimentosPorOrgao(context.Background(), &dbtest.Seq{Queries: []dbtest.QueryResult{q}}); err == nil {
			t.Errorf("%s: falha engolida", name)
		}
	}
}
