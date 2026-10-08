package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// Toda falha do banco sobe como erro (nunca como sucesso vazio), e
// "nenhuma linha" vira ErrNotFound.
func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	u := uuid.New()
	identity := auth.Identity{UserID: u, Scopes: []auth.Scope{{UnidadeID: &id}, {Perfil: "sem-unidade"}}}
	page := pagination.New(1, 10, 100)

	calls := map[string]func(db database.DBTX) error{
		"Tipos":        func(db database.DBTX) error { _, err := r.Tipos(ctx, db); return err },
		"TipoAtivo":    func(db database.DBTX) error { _, err := r.TipoAtivo(ctx, db, id); return err },
		"UnidadeAtiva": func(db database.DBTX) error { _, err := r.UnidadeAtiva(ctx, db, id); return err },
		"NextNumero": func(db database.DBTX) error {
			_, err := r.NextNumero(ctx, db, 2026)
			return err
		},
		"Insert":   func(db database.DBTX) error { return r.Insert(ctx, db, domain.Processo{ID: id}) },
		"Get":      func(db database.DBTX) error { _, err := r.Get(ctx, db, id, true); return err },
		"HasGrant": func(db database.DBTX) error { _, err := r.HasGrant(ctx, db, id, u); return err },
		"ListVisible": func(db database.DBTX) error {
			_, _, err := r.ListVisible(ctx, db, identity, domain.Filter{}, page)
			return err
		},
		"Grant":      func(db database.DBTX) error { return r.Grant(ctx, db, id, u, u) },
		"Grants":     func(db database.DBTX) error { _, err := r.Grants(ctx, db, id); return err },
		"Documentos": func(db database.DBTX) error { _, err := r.Documentos(ctx, db, id); return err },
		"GetDocumento": func(db database.DBTX) error {
			_, err := r.GetDocumento(ctx, db, id)
			return err
		},
		"DocumentoByEnvelope": func(db database.DBTX) error {
			_, err := r.DocumentoByEnvelope(ctx, db, id)
			return err
		},
		"PendingSignatures": func(db database.DBTX) error {
			_, err := r.PendingSignatures(ctx, db, id)
			return err
		},
		"InsertDocumento": func(db database.DBTX) error { return r.InsertDocumento(ctx, db, domain.Documento{ID: id}) },
		"AddMovimento":    func(db database.DBTX) error { return r.AddMovimento(ctx, db, domain.Movimento{ProcessoID: id}) },
		"Movimentos":      func(db database.DBTX) error { _, err := r.Movimentos(ctx, db, id); return err },
		"Update":          func(db database.DBTX) error { return r.Update(ctx, db, domain.Processo{ID: id}) },
		"UpdateDocumento": func(db database.DBTX) error { return r.UpdateDocumento(ctx, db, domain.Documento{ID: id}) },
		"Revoke":          func(db database.DBTX) error { _, err := r.Revoke(ctx, db, id, u); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: esperado ErrInjected, veio %v", name, err)
		}
	}

	// Linhas ilegíveis: consultas de lista e de linha única falham.
	for _, name := range []string{"Tipos", "TipoAtivo", "UnidadeAtiva", "NextNumero", "Get", "HasGrant", "ListVisible", "Grants", "Documentos",
		"GetDocumento", "DocumentoByEnvelope", "PendingSignatures", "Movimentos"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
	}
	// UPDATE sem linha afetada é "não encontrado"; DELETE sem linha, "não existia".
	for _, name := range []string{"Update", "UpdateDocumento"} {
		if err := calls[name](dbtest.ScanFail{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s sem linha afetada: esperado ErrNotFound, veio %v", name, err)
		}
	}
	if existed, err := r.Revoke(ctx, dbtest.ScanFail{}, id, u); err != nil || existed {
		t.Errorf("revogar credencial inexistente: existed=%v err=%v", existed, err)
	}
	// Erro no meio da leitura (rows.Err) não é engolido.
	for _, name := range []string{"Tipos", "Grants", "Documentos", "Movimentos"} {
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro durante a leitura: %v", name, err)
		}
	}
	// ListVisible: a contagem funciona mas a página falha.
	if _, _, err := r.ListVisible(ctx, countOK{}, identity, domain.Filter{}, page); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("ListVisible com falha na página: %v", err)
	}
	if _, _, err := r.ListVisible(ctx, countOKScanFail{}, identity, domain.Filter{}, page); err == nil {
		t.Error("ListVisible com linha ilegível na página deveria falhar")
	}
}

// countOK responde a contagem e falha na consulta da página.
type countOK struct{ dbtest.Fail }

func (countOK) QueryRow(context.Context, string, ...any) pgxRow { return zeroRow{} }

// countOKScanFail responde a contagem e devolve uma linha ilegível.
type countOKScanFail struct{ dbtest.ScanFail }

func (countOKScanFail) QueryRow(context.Context, string, ...any) pgxRow { return zeroRow{} }

type pgxRow = pgx.Row

// zeroRow responde "0" a um SELECT count(*).
type zeroRow struct{}

func (zeroRow) Scan(dest ...any) error {
	if p, ok := dest[0].(*int64); ok {
		*p = 0
	}
	return nil
}

func TestProcessosDoProcedimentoFalhas(t *testing.T) {
	r := NewRepository()
	for name, db := range map[string]database.DBTX{"consulta": dbtest.Fail{}, "linha": dbtest.ScanFail{}, "leitura": dbtest.RowsErr{}} {
		if _, err := r.ProcessosDoProcedimento(context.Background(), db, uuid.New()); err == nil {
			t.Errorf("%s: falha engolida", name)
		}
	}
}
