package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func modeloRow() valRow {
	now := time.Now()
	return valRow{uuid.New(), "Modelo", "", true, now, "", 1, "a.docx", "x", int64(1), "h", "", now, "", "obj", 0}
}

func TestModeloRepositoryFailures(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	m := domain.Modelo{ID: id, Nome: "x"}
	v := domain.ModeloVersao{}
	calls := map[string]func(db database.DBTX) error{
		"ListModelos":         func(db database.DBTX) error { _, err := r.ListModelos(ctx, db, true); return err },
		"GetModelo":           func(db database.DBTX) error { _, err := r.GetModelo(ctx, db, id); return err },
		"InsertModelo":        func(db database.DBTX) error { return r.InsertModelo(ctx, db, m, v, "p") },
		"InsertModeloVersao":  func(db database.DBTX) error { _, err := r.InsertModeloVersao(ctx, db, id, v); return err },
		"UpdateModelo":        func(db database.DBTX) error { return r.UpdateModelo(ctx, db, m, "p") },
		"VersaoModelo":        func(db database.DBTX) error { _, err := r.VersaoModelo(ctx, db, id, 0); return err },
		"ModelosAtivos":       func(db database.DBTX) error { _, err := r.ModelosAtivos(ctx, db, []uuid.UUID{id}); return err },
		"ModeloDaPeca":        func(db database.DBTX) error { _, err := r.ModeloDaPeca(ctx, db, id, id); return err },
		"SetModeloPeca":       func(db database.DBTX) error { return r.SetModeloPeca(ctx, db, id, nil) },
		"ModelosDaSerie":      func(db database.DBTX) error { _, err := r.ModelosDaSerie(ctx, db, "1.0"); return err },
		"LigarModeloSerie":    func(db database.DBTX) error { return r.LigarModeloSerie(ctx, db, "1.0", id, "p") },
		"DesligarModeloSerie": func(db database.DBTX) error { return r.DesligarModeloSerie(ctx, db, "1.0", id) },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"ListModelos", "ModelosDaSerie"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível", name)
		}
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro na leitura: %v", name, err)
		}
	}
	if err := calls["ListModelos"](dbtest.ScanFail{}); err == nil {
		t.Error("lista com linha ilegível")
	}
	if err := calls["ListModelos"](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("lista com erro na leitura: %v", err)
	}
	// Detalhe: o modelo lê, o histórico falha (consulta, linha, leitura).
	for _, q := range []dbtest.QueryResult{{Err: dbtest.ErrInjected}, {Rows: dbtest.BadRows()}, {Rows: dbtest.ErrRows()}} {
		if _, err := r.GetModelo(ctx, &dbtest.Seq{Row: modeloRow(), Queries: []dbtest.QueryResult{q}}, id); err == nil {
			t.Error("histórico com falha engolida")
		}
	}
	ok := dbtest.ExecResult{Tag: pgconn.NewCommandTag("INSERT 0 1")}
	fail := dbtest.ExecResult{Err: dbtest.ErrInjected}
	if err := r.InsertModelo(ctx, &dbtest.Seq{Execs: []dbtest.ExecResult{ok, fail}}, m, v, "p"); err == nil {
		t.Error("versão 1 com falha engolida")
	}
	if _, err := r.InsertModeloVersao(ctx, &dbtest.Seq{Execs: []dbtest.ExecResult{ok}}, id, v); err == nil {
		t.Error("número da versão com falha engolido")
	}
	if err := r.UpdateModelo(ctx, &dbtest.Seq{Execs: []dbtest.ExecResult{{Tag: pgconn.NewCommandTag("UPDATE 0")}}}, m, "p"); !errors.Is(err, domain.ErrModeloNaoEncontrado) {
		t.Errorf("alterar inexistente: %v", err)
	}
	if _, err := r.VersaoModelo(ctx, noRows{}, id, 3); !errors.Is(err, domain.ErrModeloNaoEncontrado) {
		t.Errorf("versão inexistente: %v", err)
	}
	if _, err := r.ModeloDaPeca(ctx, noRows{}, id, id); !errors.Is(err, domain.ErrPecaNaoEncontrada) {
		t.Errorf("peça de outro procedimento: %v", err)
	}
	if err := wrapModelo(&pgconn.PgError{Code: "23505"}); !errors.Is(err, domain.ErrModeloRepetido) {
		t.Errorf("nome repetido: %v", err)
	}
}

func TestAvisosFalhas(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	calls := map[string]func(db database.DBTX) error{
		"Seguir":             func(db database.DBTX) error { return r.Seguir(ctx, db, id, "X") },
		"DeixarDeSeguir":     func(db database.DBTX) error { return r.DeixarDeSeguir(ctx, db, id, "X") },
		"Seguindo":           func(db database.DBTX) error { _, err := r.Seguindo(ctx, db, id, "X"); return err },
		"Interessados":       func(db database.DBTX) error { _, err := r.Interessados(ctx, db, "X", []string{"A"}); return err },
		"InteressadosModelo": func(db database.DBTX) error { _, err := r.InteressadosModelo(ctx, db, id); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"Interessados", "InteressadosModelo"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível", name)
		}
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro na leitura", name)
		}
	}
}

func TestValidacaoFalhas(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	if err := r.SetSituacao(ctx, dbtest.Fail{}, id, "RASCUNHO", false); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("situação com o banco fora: %v", err)
	}
	if err := r.SetSituacao(ctx, &dbtest.Seq{Execs: []dbtest.ExecResult{{Tag: pgconn.NewCommandTag("UPDATE 0")}}}, id, "RASCUNHO", false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("situação de inexistente: %v", err)
	}
	if err := r.InsertValidacao(ctx, dbtest.Fail{}, domain.Validacao{}); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("gravar entrevista: %v", err)
	}
	for name, db := range map[string]database.DBTX{"consulta": dbtest.Fail{}, "linha": dbtest.ScanFail{}, "leitura": dbtest.RowsErr{}} {
		if _, err := r.Validacoes(ctx, db, "X"); err == nil {
			t.Errorf("entrevistas (%s): falha engolida", name)
		}
	}
}
