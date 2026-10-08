// Package dbtest oferece implementações de database.DBTX que falham de
// propósito, para testar os caminhos de erro dos repositórios sem
// derrubar o banco de verdade.
//
//   - Fail: toda chamada devolve ErrInjected (conexão caída, timeout...).
//   - ScanFail: as consultas "funcionam", mas a leitura das linhas falha
//     (coluna com tipo inesperado) e o Exec não afeta nenhuma linha.
//   - RowsErr: a consulta devolve zero linhas e o erro só aparece em
//     rows.Err() (conexão perdida no meio da leitura).
package dbtest

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrInjected é o erro devolvido pelas falhas simuladas.
var ErrInjected = errors.New("dbtest: falha injetada")

// Fail falha em toda chamada.
type Fail struct{}

func (Fail) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, ErrInjected
}
func (Fail) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, ErrInjected }
func (Fail) QueryRow(context.Context, string, ...any) pgx.Row        { return row{} }

// ScanFail aceita as consultas mas falha ao ler cada linha; o Exec
// devolve zero linhas afetadas.
type ScanFail struct{}

func (ScanFail) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 0"), nil
}
func (ScanFail) Query(context.Context, string, ...any) (pgx.Rows, error) { return &rows{left: 1}, nil }
func (ScanFail) QueryRow(context.Context, string, ...any) pgx.Row        { return row{} }

// RowsErr devolve zero linhas e o erro em rows.Err().
type RowsErr struct{}

func (RowsErr) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, ErrInjected
}
func (RowsErr) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &rows{err: ErrInjected}, nil
}
func (RowsErr) QueryRow(context.Context, string, ...any) pgx.Row { return row{} }

type row struct{}

func (row) Scan(...any) error { return ErrInjected }

type rows struct {
	left int
	err  error
}

func (r *rows) Close()                                       {}
func (r *rows) Err() error                                   { return r.err }
func (r *rows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *rows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *rows) Scan(...any) error                            { return ErrInjected }
func (r *rows) Values() ([]any, error)                       { return nil, ErrInjected }
func (r *rows) RawValues() [][]byte                          { return nil }
func (r *rows) Conn() *pgx.Conn                              { return nil }
func (r *rows) Next() bool {
	if r.left == 0 {
		return false
	}
	r.left--
	return true
}

// NoRows é um resultado vazio e sem erro.
func NoRows() pgx.Rows { return &rows{} }

// BadRows tem uma linha ilegível (Scan falha).
func BadRows() pgx.Rows { return &rows{left: 1} }

// ErrRows não tem linhas e falha em rows.Err().
func ErrRows() pgx.Rows { return &rows{err: ErrInjected} }

// QueryResult e ExecResult roteirizam as respostas de Seq.
type (
	QueryResult struct {
		Rows pgx.Rows
		Err  error
	}
	ExecResult struct {
		Tag pgconn.CommandTag
		Err error
	}
)

// OK é um Exec bem-sucedido que afetou uma linha.
var OK = ExecResult{Tag: pgconn.NewCommandTag("UPDATE 1")}

// Seq responde em ordem às chamadas de Query e Exec (a última resposta se
// repete); QueryRow devolve Row (por padrão, uma linha que falha).
type Seq struct {
	Queries []QueryResult
	Execs   []ExecResult
	Row     pgx.Row
	qi, ei  int
}

func (s *Seq) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if len(s.Queries) == 0 {
		return nil, ErrInjected
	}
	r := s.Queries[min(s.qi, len(s.Queries)-1)]
	s.qi++
	return r.Rows, r.Err
}

func (s *Seq) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if len(s.Execs) == 0 {
		return pgconn.CommandTag{}, ErrInjected
	}
	r := s.Execs[min(s.ei, len(s.Execs)-1)]
	s.ei++
	return r.Tag, r.Err
}

func (s *Seq) QueryRow(context.Context, string, ...any) pgx.Row {
	if s.Row != nil {
		return s.Row
	}
	return row{}
}
