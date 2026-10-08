package infrastructure

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// Cada instrução da carga pode falhar: as tabelas temporárias, os INSERTs,
// a comparação, as duas leituras (consulta, linha, leitura) e a gravação.
func TestCargaTTDDFalhas(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	um := 1
	carga := domain.CargaTTDD{
		Orgaos:     []domain.OrgaoTTDD{{Prefixo: "2.0", Nome: "Adm"}},
		Funcoes:    []domain.CargaFuncao{{Codigo: "2.0.01", Nome: "F"}},
		Subfuncoes: []domain.CargaSubfuncao{{Codigo: "2.0.01.00", Nome: "S"}},
		Series: []domain.CargaSerie{{Codigo: "2.0.01.00.01", Subfuncao: "2.0.01.00", PrazosTTDD: domain.PrazosTTDD{Descritor: "D",
			FaseCorrenteAnos: &um, DestinacaoFinal: func() *domain.DestinacaoFinal { d := domain.DestinacaoEliminacao; return &d }()}}},
	}
	fail := dbtest.ExecResult{Err: dbtest.ErrInjected}
	antesDasLeituras := len(tabelasCarga) + 4 + 1
	for n := 1; n <= antesDasLeituras+len(gravacaoCarga); n++ {
		execs := []dbtest.ExecResult{}
		for i := 1; i < n; i++ {
			execs = append(execs, dbtest.OK)
		}
		db := &dbtest.Seq{Execs: append(execs, fail), Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}, {Rows: dbtest.NoRows()}}}
		if _, err := r.CargaTTDD(ctx, db, carga, true); err == nil {
			t.Errorf("falha na instrução %d engolida", n)
		}
	}
	q := func(rs ...pgx.Rows) []dbtest.QueryResult {
		out := []dbtest.QueryResult{}
		for _, x := range rs {
			out = append(out, dbtest.QueryResult{Rows: x})
		}
		return out
	}
	falhou := dbtest.QueryResult{Err: dbtest.ErrInjected}
	for nome, seq := range map[string][]dbtest.QueryResult{
		"impacto: consulta":       {falhou},
		"impacto: linha":          q(dbtest.BadRows()),
		"impacto: leitura":        q(dbtest.ErrRows()),
		"procedimentos: consulta": append(q(dbtest.NoRows()), falhou),
		"procedimentos: linha":    q(dbtest.NoRows(), dbtest.BadRows()),
		"procedimentos: leitura":  q(dbtest.NoRows(), dbtest.ErrRows()),
	} {
		if _, err := r.CargaTTDD(ctx, &dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}, Queries: seq}, carga, false); err == nil {
			t.Errorf("%s: falha engolida", nome)
		}
	}
	// Simulação: nada é gravado (sem as instruções de gravação).
	db := &dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}, Queries: q(dbtest.NoRows(), dbtest.NoRows())}
	if out, err := r.CargaTTDD(ctx, db, carga, false); err != nil || len(out.Series) != 0 || out.Totais == nil {
		t.Fatalf("simulação: %+v %v", out, err)
	}
}

func TestPai(t *testing.T) {
	if pai("2.0.01.00.05", 3) != "2.0.01" || pai("2.0", 5) != "2.0" {
		t.Fatal("pai")
	}
}
