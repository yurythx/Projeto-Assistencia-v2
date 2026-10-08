package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	page := pagination.New(1, 10, 100)
	dep, un, ent := uuid.New(), uuid.New(), uuid.New()
	calls := map[string]func(db database.DBTX) error{
		"ListEntidades":       func(db database.DBTX) error { _, err := r.ListEntidades(ctx, db); return err },
		"GetEntidade":         func(db database.DBTX) error { _, err := r.GetEntidade(ctx, db, id); return err },
		"UpsertEntidade":      func(db database.DBTX) error { _, err := r.UpsertEntidade(ctx, db, domain.Entidade{}); return err },
		"DeleteEntidade":      func(db database.DBTX) error { return r.DeleteEntidade(ctx, db, id) },
		"ListUnidades":        func(db database.DBTX) error { _, err := r.ListUnidades(ctx, db, nil); return err },
		"GetUnidade":          func(db database.DBTX) error { _, err := r.GetUnidade(ctx, db, id); return err },
		"UpsertUnidade":       func(db database.DBTX) error { _, err := r.UpsertUnidade(ctx, db, domain.Unidade{}); return err },
		"UnidadeCreatesCycle": func(db database.DBTX) error { _, err := r.UnidadeCreatesCycle(ctx, db, id, id); return err },
		"DeleteUnidade":       func(db database.DBTX) error { return r.DeleteUnidade(ctx, db, id) },
		"ListDepartamentos":   func(db database.DBTX) error { _, err := r.ListDepartamentos(ctx, db, nil); return err },
		"GetDepartamento":     func(db database.DBTX) error { _, err := r.GetDepartamento(ctx, db, id); return err },
		"UpsertDepartamento": func(db database.DBTX) error {
			_, err := r.UpsertDepartamento(ctx, db, domain.Departamento{})
			return err
		},
		"DeleteDepartamento": func(db database.DBTX) error { return r.DeleteDepartamento(ctx, db, id) },
		"ListPerfis":         func(db database.DBTX) error { _, err := r.ListPerfis(ctx, db); return err },
		"GetPerfil":          func(db database.DBTX) error { _, err := r.GetPerfil(ctx, db, id); return err },
		"UpsertPerfil":       func(db database.DBTX) error { _, err := r.UpsertPerfil(ctx, db, domain.Perfil{}); return err },
		"DeletePerfil":       func(db database.DBTX) error { return r.DeletePerfil(ctx, db, id) },
		"ListMappings":       func(db database.DBTX) error { _, err := r.ListMappings(ctx, db); return err },
		"GetMapping":         func(db database.DBTX) error { _, err := r.GetMapping(ctx, db, id); return err },
		"CreateMapping":      func(db database.DBTX) error { _, err := r.CreateMapping(ctx, db, domain.ADMapping{}); return err },
		"DeleteMapping":      func(db database.DBTX) error { return r.DeleteMapping(ctx, db, id) },
		"ListLotacoes":       func(db database.DBTX) error { _, err := r.ListLotacoes(ctx, db, id); return err },
		"GetLotacao":         func(db database.DBTX) error { _, err := r.GetLotacao(ctx, db, id, id); return err },
		"CreateLotacao": func(db database.DBTX) error {
			_, err := r.CreateLotacao(ctx, db, domain.Lotacao{Principal: true})
			return err
		},
		"DeleteLotacao":   func(db database.DBTX) error { return r.DeleteLotacao(ctx, db, id, id) },
		"ListUsers":       func(db database.DBTX) error { _, _, err := r.ListUsers(ctx, db, domain.UserFilter{}, page); return err },
		"GetUser":         func(db database.DBTX) error { _, err := r.GetUser(ctx, db, id); return err },
		"UserPermissions": func(db database.DBTX) error { _, err := r.UserPermissions(ctx, db, id); return err },
		"UserGrants":      func(db database.DBTX) error { _, err := r.UserGrants(ctx, db, id); return err },
		"UserVisible":     func(db database.DBTX) error { _, err := r.UserVisible(ctx, db, id, domain.Area{}); return err },
		"UpdateUser":      func(db database.DBTX) error { return r.UpdateUser(ctx, db, id, "", true, nil) },
		"CreateLocalUser": func(db database.DBTX) error {
			_, err := r.CreateLocalUser(ctx, db, "u", "e", "d", "h", nil)
			return err
		},
		"SetPassword": func(db database.DBTX) error { return r.SetPassword(ctx, db, id, "h") },
		"Unlock":      func(db database.DBTX) error { return r.Unlock(ctx, db, id) },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"ListEntidades", "ListUnidades", "ListDepartamentos", "ListPerfis", "ListMappings", "ListLotacoes", "ListUsers", "UserGrants"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
		if err := calls[name](dbtest.RowsErr{}); name != "ListUsers" && !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro durante a leitura: %v", name, err)
		}
	}
	// UPDATE/DELETE sem linha afetada: "não encontrado" (perfil: "de sistema").
	for _, name := range []string{"DeleteEntidade", "DeleteUnidade", "DeleteDepartamento", "DeleteMapping", "DeleteLotacao", "UpdateUser", "SetPassword", "Unlock"} {
		if err := calls[name](dbtest.ScanFail{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s sem linha afetada: %v", name, err)
		}
	}
	if err := calls["DeletePerfil"](dbtest.ScanFail{}); !errors.Is(err, domain.ErrSystemProfile) {
		t.Errorf("perfil de sistema não é excluído: %v", err)
	}
	if _, _, err := r.ListUsers(ctx, countOK{}, domain.UserFilter{}, page); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("ListUsers com falha na página: %v", err)
	}
	if _, _, err := r.ListUsers(ctx, countOKScanFail{}, domain.UserFilter{}, page); err == nil {
		t.Error("ListUsers com linha ilegível na página deveria falhar")
	}
	if _, err := r.CreateLotacao(ctx, &firstExecOK{}, domain.Lotacao{Principal: true}); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("CreateLotacao com falha ao inserir: %v", err)
	}

	// Consistência do escopo: departamento e unidade inexistentes, e
	// níveis que não batem.
	for name, s := range map[string]domain.Scope{
		"departamento inexistente": {DepartamentoID: &dep},
		"unidade inexistente":      {UnidadeID: &un},
	} {
		if _, err := r.ScopeConsistent(ctx, dbtest.Fail{}, s); !errors.Is(err, domain.ErrInvalidScope) {
			t.Errorf("%s: %v", name, err)
		}
	}
	other := uuid.New()
	if _, err := r.ScopeConsistent(ctx, fixedParent{parent: other}, domain.Scope{DepartamentoID: &dep, UnidadeID: &un}); !errors.Is(err, domain.ErrInvalidScope) {
		t.Errorf("departamento de outra unidade: %v", err)
	}
	if _, err := r.ScopeConsistent(ctx, fixedParent{parent: other}, domain.Scope{UnidadeID: &un, EntidadeID: &ent}); !errors.Is(err, domain.ErrInvalidScope) {
		t.Errorf("unidade de outra entidade: %v", err)
	}
	if s, err := r.ScopeConsistent(ctx, fixedParent{parent: ent}, domain.Scope{UnidadeID: &un}); err != nil || s.EntidadeID == nil || *s.EntidadeID != ent {
		t.Errorf("escopo completado com a entidade da unidade: %+v %v", s, err)
	}
}

// fixedParent responde toda consulta de "nível acima" com o mesmo id.
type fixedParent struct {
	dbtest.Fail
	parent uuid.UUID
}

func (f fixedParent) QueryRow(context.Context, string, ...any) pgx.Row { return idRow{f.parent} }

type idRow struct{ id uuid.UUID }

func (r idRow) Scan(dest ...any) error {
	*(dest[0].(*uuid.UUID)) = r.id
	return nil
}

type countOK struct{ dbtest.Fail }

func (countOK) QueryRow(context.Context, string, ...any) pgx.Row { return zeroRow{} }

type countOKScanFail struct{ dbtest.ScanFail }

func (countOKScanFail) QueryRow(context.Context, string, ...any) pgx.Row { return zeroRow{} }

type zeroRow struct{}

func (zeroRow) Scan(dest ...any) error {
	if p, ok := dest[0].(*int64); ok {
		*p = 0
	}
	return nil
}

// firstExecOK aceita o primeiro Exec e falha no QueryRow seguinte.
type firstExecOK struct{ dbtest.Fail }

func (*firstExecOK) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
