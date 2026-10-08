package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/modules/files/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	calls := map[string]func(db database.DBTX) error{
		"Chain":        func(db database.DBTX) error { _, err := r.Chain(ctx, db, id); return err },
		"RootFolders":  func(db database.DBTX) error { _, _, err := r.RootFolders(ctx, db); return err },
		"Children":     func(db database.DBTX) error { _, err := r.Children(ctx, db, id); return err },
		"Breadcrumbs":  func(db database.DBTX) error { _, err := r.Breadcrumbs(ctx, db, id); return err },
		"GetFolder":    func(db database.DBTX) error { _, err := r.GetFolder(ctx, db, id); return err },
		"SaveFolder":   func(db database.DBTX) error { _, err := r.SaveFolder(ctx, db, domain.Folder{ID: id}); return err },
		"DeleteFolder": func(db database.DBTX) error { _, err := r.DeleteFolder(ctx, db, id); return err },
		"FolderEmpty":  func(db database.DBTX) error { _, err := r.FolderEmpty(ctx, db, id); return err },
		"IsDescendant": func(db database.DBTX) error { _, err := r.IsDescendant(ctx, db, id, id); return err },
		"ACL":          func(db database.DBTX) error { _, err := r.ACL(ctx, db, id); return err },
		"ReplaceACL": func(db database.DBTX) error {
			return r.ReplaceACL(ctx, db, id, []domain.ACLEntry{{SubjectType: "everyone", Subject: "*"}}, "x")
		},
		"Files":        func(db database.DBTX) error { _, err := r.Files(ctx, db, id); return err },
		"GetFile":      func(db database.DBTX) error { _, err := r.GetFile(ctx, db, id); return err },
		"InsertFile":   func(db database.DBTX) error { _, err := r.InsertFile(ctx, db, domain.File{ID: id}); return err },
		"UpdateFile":   func(db database.DBTX) error { _, err := r.UpdateFile(ctx, db, domain.File{ID: id}); return err },
		"DeleteFile":   func(db database.DBTX) error { return r.DeleteFile(ctx, db, id) },
		"StalePending": func(db database.DBTX) error { _, err := r.StalePending(ctx, db, time.Now(), 10); return err },
		"SearchFiles":  func(db database.DBTX) error { _, err := r.SearchFiles(ctx, db, "x", 10); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"Chain", "RootFolders", "Children", "Breadcrumbs", "DeleteFolder", "ACL", "Files"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
	}
	for _, name := range []string{"Chain", "Children", "Breadcrumbs", "ACL", "Files"} {
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro durante a leitura: %v", name, err)
		}
	}
	for _, name := range []string{"UpdateFile", "DeleteFile"} {
		if err := calls[name](dbtest.ScanFail{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s sem linha afetada: %v", name, err)
		}
	}
	if err := calls["Chain"](&dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}}}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("pasta inexistente: cadeia vazia é ErrNotFound: %v", err)
	}

	// Consultas encadeadas: a segunda etapa falha.
	stages := map[string]struct {
		call func(db database.DBTX) error
		db   *dbtest.Seq
	}{
		"RootFolders: ACL falha":        {calls["RootFolders"], &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}, {Err: dbtest.ErrInjected}}}},
		"RootFolders: ACL ilegível":     {calls["RootFolders"], &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}, {Rows: dbtest.BadRows()}}}},
		"RootFolders: ACL interrompida": {calls["RootFolders"], &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}, {Rows: dbtest.ErrRows()}}}},
		"DeleteFolder: exclusão falha":  {calls["DeleteFolder"], &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}}}},
		"SaveFolder: releitura falha":   {calls["SaveFolder"], &dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}},
		"InsertFile: releitura falha":   {calls["InsertFile"], &dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}},
		"UpdateFile: releitura falha":   {calls["UpdateFile"], &dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}},
		"ReplaceACL: inclusão falha":    {calls["ReplaceACL"], &dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK, {Err: dbtest.ErrInjected}}}},
	}
	for name, st := range stages {
		if err := st.call(st.db); err == nil {
			t.Errorf("%s deveria falhar", name)
		}
	}
	gone := &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}}, Execs: []dbtest.ExecResult{{Tag: pgconn.NewCommandTag("DELETE 0")}}}
	if _, err := r.DeleteFolder(ctx, gone, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("pasta já removida: %v", err)
	}
}
