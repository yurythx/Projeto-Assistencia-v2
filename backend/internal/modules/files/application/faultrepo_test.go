package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/files/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

type innerRepo = domain.Repository

var errBoom = errors.New("falha simulada no repositório")

// faultRepo envolve o repositório real: na chamada failAt devolve erro; depois
// da chamada poisonAt "envenena" a transação (a instrução SQL seguinte — do
// repositório, do outbox ou da auditoria — falha). noTx marca veneno em
// leitura fora de transação, onde não há instrução seguinte na mesma conexão.
type faultRepo struct {
	inner                   innerRepo
	calls, failAt, poisonAt int
	trace                   []string
	noTx                    bool
}

func (fr *faultRepo) hook(name string) error {
	fr.calls++
	fr.trace = append(fr.trace, name)
	if fr.calls == fr.failAt {
		return errBoom
	}
	return nil
}

func (fr *faultRepo) post(ctx context.Context, db database.DBTX) {
	if fr.calls != fr.poisonAt {
		return
	}
	if _, inTx := db.(pgx.Tx); !inTx {
		fr.noTx = true
		return
	}
	_, _ = db.Exec(ctx, `SELECT 1/0`)
}

func (fr *faultRepo) Chain(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]domain.ChainLink, error) {
	if err := fr.hook("Chain"); err != nil {
		var z0 []domain.ChainLink
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Chain(ctx, db, folderID)
}

func (fr *faultRepo) RootFolders(ctx context.Context, db database.DBTX) ([]domain.Folder, map[uuid.UUID][]domain.ACLEntry, error) {
	if err := fr.hook("RootFolders"); err != nil {
		var z0 []domain.Folder
		var z1 map[uuid.UUID][]domain.ACLEntry
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.RootFolders(ctx, db)
}

func (fr *faultRepo) Children(ctx context.Context, db database.DBTX, parentID uuid.UUID) ([]domain.Folder, error) {
	if err := fr.hook("Children"); err != nil {
		var z0 []domain.Folder
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Children(ctx, db, parentID)
}

func (fr *faultRepo) Breadcrumbs(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]domain.Folder, error) {
	if err := fr.hook("Breadcrumbs"); err != nil {
		var z0 []domain.Folder
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Breadcrumbs(ctx, db, folderID)
}

func (fr *faultRepo) GetFolder(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Folder, error) {
	if err := fr.hook("GetFolder"); err != nil {
		var z0 domain.Folder
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetFolder(ctx, db, id)
}

func (fr *faultRepo) SaveFolder(ctx context.Context, db database.DBTX, f domain.Folder) (domain.Folder, error) {
	if err := fr.hook("SaveFolder"); err != nil {
		var z0 domain.Folder
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.SaveFolder(ctx, db, f)
}

func (fr *faultRepo) DeleteFolder(ctx context.Context, db database.DBTX, id uuid.UUID) ([]string, error) {
	if err := fr.hook("DeleteFolder"); err != nil {
		var z0 []string
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteFolder(ctx, db, id)
}

func (fr *faultRepo) FolderEmpty(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	if err := fr.hook("FolderEmpty"); err != nil {
		var z0 bool
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.FolderEmpty(ctx, db, id)
}

func (fr *faultRepo) IsDescendant(ctx context.Context, db database.DBTX, candidate uuid.UUID, ancestor uuid.UUID) (bool, error) {
	if err := fr.hook("IsDescendant"); err != nil {
		var z0 bool
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.IsDescendant(ctx, db, candidate, ancestor)
}

func (fr *faultRepo) ACL(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]domain.ACLEntry, error) {
	if err := fr.hook("ACL"); err != nil {
		var z0 []domain.ACLEntry
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ACL(ctx, db, folderID)
}

func (fr *faultRepo) ReplaceACL(ctx context.Context, db database.DBTX, folderID uuid.UUID, entries []domain.ACLEntry, grantedBy string) error {
	if err := fr.hook("ReplaceACL"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.ReplaceACL(ctx, db, folderID, entries, grantedBy)
}

func (fr *faultRepo) Files(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]domain.File, error) {
	if err := fr.hook("Files"); err != nil {
		var z0 []domain.File
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Files(ctx, db, folderID)
}

func (fr *faultRepo) GetFile(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.File, error) {
	if err := fr.hook("GetFile"); err != nil {
		var z0 domain.File
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetFile(ctx, db, id)
}

func (fr *faultRepo) InsertFile(ctx context.Context, db database.DBTX, f domain.File) (domain.File, error) {
	if err := fr.hook("InsertFile"); err != nil {
		var z0 domain.File
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertFile(ctx, db, f)
}

func (fr *faultRepo) UpdateFile(ctx context.Context, db database.DBTX, f domain.File) (domain.File, error) {
	if err := fr.hook("UpdateFile"); err != nil {
		var z0 domain.File
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateFile(ctx, db, f)
}

func (fr *faultRepo) DeleteFile(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteFile"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteFile(ctx, db, id)
}

func (fr *faultRepo) StalePending(ctx context.Context, db database.DBTX, olderThan time.Time, limit int) ([]domain.File, error) {
	if err := fr.hook("StalePending"); err != nil {
		var z0 []domain.File
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.StalePending(ctx, db, olderThan, limit)
}

func (fr *faultRepo) SearchFiles(ctx context.Context, db database.DBTX, query string, limit int) ([]domain.File, error) {
	if err := fr.hook("SearchFiles"); err != nil {
		var z0 []domain.File
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.SearchFiles(ctx, db, query, limit)
}
