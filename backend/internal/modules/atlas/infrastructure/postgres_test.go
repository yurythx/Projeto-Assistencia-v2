package infrastructure

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	p := pagination.New(1, 10, 10)
	wf := domain.Workflow{ID: id, Etapas: []domain.Etapa{{ID: uuid.New(), Ordem: 1,
		Documentos: []domain.EtapaDocumento{{ID: uuid.New()}}, Transicoes: []domain.EtapaTransicao{{ID: uuid.New(), DestinoOrdem: 1}}}}}
	calls := map[string]func(db database.DBTX) error{
		"ListTTDD": func(db database.DBTX) error {
			_, _, err := r.ListTTDD(ctx, db, domain.FiltroTTDD{Query: "x", Codigo: "2.0"}, p)
			return err
		},
		"EstruturaTTDD":  func(db database.DBTX) error { _, err := r.EstruturaTTDD(ctx, db); return err },
		"CandidatosTTDD": func(db database.DBTX) error { _, err := r.CandidatosTTDD(ctx, db, "x", 5); return err },
		"GetTTDD":        func(db database.DBTX) error { _, err := r.GetTTDD(ctx, db, "x"); return err },
		"LockTTDD":       func(db database.DBTX) error { _, err := r.LockTTDD(ctx, db, "x"); return err },
		"List": func(db database.DBTX) error {
			_, _, err := r.List(ctx, db, domain.Filter{Query: "x", CodigoTTDD: "y"}, p)
			return err
		},
		"Get":              func(db database.DBTX) error { _, err := r.Get(ctx, db, id, true); return err },
		"Insert":           func(db database.DBTX) error { return r.Insert(ctx, db, wf) },
		"SetAtivo":         func(db database.DBTX) error { return r.SetAtivo(ctx, db, id, true) },
		"Search":           func(db database.DBTX) error { _, _, err := r.Search(ctx, db, "x", 5); return err },
		"Candidatos":       func(db database.DBTX) error { _, err := r.Candidatos(ctx, db, "x", 5); return err },
		"HistoricoTTDD":    func(db database.DBTX) error { _, err := r.HistoricoTTDD(ctx, db, "x"); return err },
		"MaxVersao":        func(db database.DBTX) error { _, err := r.MaxVersao(ctx, db, "X"); return err },
		"DesativarVersoes": func(db database.DBTX) error { _, err := r.DesativarVersoes(ctx, db, "X", id); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"Search", "Candidatos", "EstruturaTTDD", "CandidatosTTDD", "HistoricoTTDD", "MaxVersao", "DesativarVersoes"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro durante a leitura: %v", name, err)
		}
	}
	// Linha inexistente vira o erro de domínio.
	if err := calls["Get"](dbtest.ScanFail{}); err == nil {
		t.Error("get com linha ilegível deveria falhar")
	}
	if _, err := r.GetTTDD(ctx, noRows{}, "x"); !errors.Is(err, domain.ErrTTDDNotFound) {
		t.Errorf("TTDD inexistente: %v", err)
	}
	if _, err := r.Get(ctx, noRows{}, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("procedimento inexistente: %v", err)
	}
	// Situação da série no cadastro: inexistente, vigente ou revogada.
	for db, want := range map[database.DBTX]domain.SituacaoTTDD{
		noRows{}: domain.TTDDInexistente, &dbtest.Seq{Row: valRow{true}}: domain.TTDDVigente, &dbtest.Seq{Row: valRow{false}}: domain.TTDDRevogada,
	} {
		if got, err := r.LockTTDD(ctx, db, "x"); err != nil || got != want {
			t.Errorf("LockTTDD: %v %v, quero %v", got, err, want)
		}
	}
}

func TestWrap(t *testing.T) {
	if wrap(nil) != nil {
		t.Fatal("nil continua nil")
	}
	if err := wrap(errors.New("x")); err == nil || err.Error() != "atlas: x" {
		t.Fatalf("erro genérico com prefixo: %v", err)
	}
}

// noRows simula a consulta sem resultado (pgx.ErrNoRows).
type noRows struct{ dbtest.Fail }

func (noRows) QueryRow(context.Context, string, ...any) pgx.Row { return noRow{} }

type noRow struct{}

func (noRow) Scan(...any) error { return pgx.ErrNoRows }

// valRows entrega linhas com valores reais (Scan preenche os destinos), para
// alcançar as falhas que só acontecem depois de uma leitura bem-sucedida.
type valRows struct {
	data [][]any
	i    int
}

func (r *valRows) Close()                                       {}
func (r *valRows) Err() error                                   { return nil }
func (r *valRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *valRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *valRows) Values() ([]any, error)                       { return nil, nil }
func (r *valRows) RawValues() [][]byte                          { return nil }
func (r *valRows) Conn() *pgx.Conn                              { return nil }
func (r *valRows) Next() bool                                   { r.i++; return r.i <= len(r.data) }
func (r *valRows) Scan(dest ...any) error                       { return fill(dest, r.data[r.i-1]) }

type valRow []any

func (v valRow) Scan(dest ...any) error { return fill(dest, v) }

func fill(dest, vals []any) error {
	for i, d := range dest {
		dv := reflect.ValueOf(d).Elem()
		if vals[i] == nil {
			dv.Set(reflect.Zero(dv.Type()))
			continue
		}
		dv.Set(reflect.ValueOf(vals[i]).Convert(dv.Type()))
	}
	return nil
}

func etapaRows(wfID uuid.UUID) pgx.Rows {
	return &valRows{data: [][]any{{uuid.New(), wfID, 1, "SIG", "Setor", "Atribuições", 5, false}}}
}

func workflowRow(id uuid.UUID) valRow {
	now := time.Now()
	// Procedimento (14 colunas, com a situação) + nº de etapas + série da TTDD (11) e hierarquia (10, nulas).
	return valRow{id, "ADM.X.1", "Título", "Objetivo", "Público", 1, true, "HOMOLOGADO", "PUBLICO", "", "1.0", nil, now, now, int64(1),
		"1.0", "Descritor", nil, "", nil, "", nil, "", nil, "", now, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil}
}

func TestRepositoryFailuresAfterSuccessfulReads(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	p := pagination.New(1, 10, 10)
	wfs := func() []domain.Workflow { return []domain.Workflow{{ID: id}} }
	q := func(rs ...pgx.Rows) []dbtest.QueryResult {
		out := make([]dbtest.QueryResult, len(rs))
		for i, x := range rs {
			out[i] = dbtest.QueryResult{Rows: x}
		}
		return out
	}
	fail := dbtest.QueryResult{Err: dbtest.ErrInjected}

	// Etapas, peças e transições: cada consulta pode falhar na execução, na
	// leitura de uma linha ou no meio da leitura.
	for name, seq := range map[string][]dbtest.QueryResult{
		"etapas: consulta":     {fail},
		"etapas: linha":        q(dbtest.BadRows()),
		"etapas: leitura":      q(dbtest.ErrRows()),
		"peças: consulta":      append(q(etapaRows(id)), fail),
		"peças: linha":         q(etapaRows(id), dbtest.BadRows()),
		"peças: leitura":       q(etapaRows(id), dbtest.ErrRows()),
		"transições: consulta": append(q(etapaRows(id), dbtest.NoRows()), fail),
		"transições: linha":    q(etapaRows(id), dbtest.NoRows(), dbtest.BadRows()),
		"transições: leitura":  q(etapaRows(id), dbtest.NoRows(), dbtest.ErrRows()),
	} {
		if _, err := r.loadEtapas(ctx, &dbtest.Seq{Queries: seq}, wfs()); err == nil {
			t.Errorf("%s: falha engolida", name)
		}
	}
	if out, err := r.loadEtapas(ctx, &dbtest.Seq{Queries: q(dbtest.NoRows())}, wfs()); err != nil || len(out[0].Etapas) != 0 {
		t.Errorf("procedimento sem etapas: %+v %v", out, err)
	}
	if out, err := r.loadEtapas(ctx, &dbtest.Seq{}, nil); err != nil || out != nil {
		t.Errorf("lista vazia não consulta: %v", err)
	}
	if _, err := r.Get(ctx, &dbtest.Seq{Row: workflowRow(id), Queries: []dbtest.QueryResult{fail}}, id, false); err == nil {
		t.Error("get: falha ao carregar etapas engolida")
	}
	if _, err := r.Candidatos(ctx, &dbtest.Seq{Queries: append(q(&valRows{data: [][]any{workflowRow(id)}}), fail)}, "x", 5); err == nil {
		t.Error("candidatos: falha ao carregar etapas engolida")
	}

	// Listagens: contagem ok, página falha (consulta, linha, leitura).
	count := valRow{int64(1)}
	pages := func() []dbtest.QueryResult {
		return []dbtest.QueryResult{fail, {Rows: dbtest.BadRows()}, {Rows: dbtest.ErrRows()}}
	}
	for i := range pages() {
		if _, _, err := r.List(ctx, &dbtest.Seq{Row: count, Queries: []dbtest.QueryResult{pages()[i]}}, domain.Filter{IncluirInativos: true}, p); err == nil {
			t.Error("lista: página com falha")
		}
		if _, _, err := r.ListTTDD(ctx, &dbtest.Seq{Row: count, Queries: []dbtest.QueryResult{pages()[i]}}, domain.FiltroTTDD{}, p); err == nil {
			t.Error("TTDD: página com falha")
		}
	}

	// Cadastro: cada INSERT pode falhar.
	wf := domain.Workflow{ID: id, Etapas: []domain.Etapa{{ID: uuid.New(), Ordem: 1,
		Documentos: []domain.EtapaDocumento{{ID: uuid.New()}}, Transicoes: []domain.EtapaTransicao{{ID: uuid.New(), DestinoOrdem: 1}}}}}
	failExec := dbtest.ExecResult{Err: dbtest.ErrInjected}
	for n := 1; n <= 4; n++ {
		execs := make([]dbtest.ExecResult, 0, n)
		for i := 1; i < n; i++ {
			execs = append(execs, dbtest.OK)
		}
		if err := r.Insert(ctx, &dbtest.Seq{Execs: append(execs, failExec)}, wf); err == nil {
			t.Errorf("insert: falha no comando %d engolida", n)
		}
	}
	if err := r.Insert(ctx, &dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}, wf); err != nil {
		t.Errorf("insert completo: %v", err)
	}
	if err := r.SetAtivo(ctx, &dbtest.Seq{Execs: []dbtest.ExecResult{{Tag: pgconn.NewCommandTag("UPDATE 0")}}}, id, true); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ativar inexistente: %v", err)
	}
	if err := r.SetAtivo(ctx, &dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}, id, true); err != nil {
		t.Errorf("ativar: %v", err)
	}
	if err := wrap(&pgconn.PgError{Code: "23505"}); !errors.Is(err, domain.ErrDuplicate) {
		t.Errorf("violação de unicidade vira duplicado: %v", err)
	}
}
