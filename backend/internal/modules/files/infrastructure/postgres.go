// Package infrastructure implementa o repositório do plugin Arquivos.
package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/files/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case database.IsNoRows(err):
		return domain.ErrNotFound
	case database.IsUniqueViolation(err):
		return domain.ErrDuplicate
	}
	return fmt.Errorf("files: %w", err)
}

const folderCols = `f.id, f.parent_id, f.name, f.owner_id, COALESCE(NULLIF(u.display_name,''), u.username, ''), f.created_at, f.updated_at, f.unidade_id`
const folderFrom = ` FROM files_folders f LEFT JOIN users u ON u.id = f.owner_id `

func scanFolder(row interface{ Scan(...any) error }) (domain.Folder, error) {
	var f domain.Folder
	err := row.Scan(&f.ID, &f.ParentID, &f.Name, &f.OwnerID, &f.OwnerName, &f.CreatedAt, &f.UpdatedAt, &f.UnidadeID)
	return f, err
}

func (r *Repository) Chain(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]domain.ChainLink, error) {
	rows, err := db.Query(ctx, `
		WITH RECURSIVE chain AS (
			SELECT id, parent_id, owner_id, unidade_id, 0 AS depth FROM files_folders WHERE id = $1
			UNION ALL
			SELECT f.id, f.parent_id, f.owner_id, f.unidade_id, c.depth + 1 FROM files_folders f JOIN chain c ON f.id = c.parent_id
			WHERE c.depth < 64
		)
		SELECT c.id, c.depth, c.owner_id, c.unidade_id, a.subject_type, a.subject, a.can_write
		FROM chain c LEFT JOIN files_folder_acl a ON a.folder_id = c.id
		ORDER BY c.depth`, folderID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	var out []domain.ChainLink
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var (
			id             uuid.UUID
			depth          int
			owner          uuid.UUID
			unidade        *uuid.UUID
			sType, subject *string
			canWrite       *bool
		)
		if err := rows.Scan(&id, &depth, &owner, &unidade, &sType, &subject, &canWrite); err != nil {
			return nil, wrap(err)
		}
		i, ok := index[id]
		if !ok {
			out = append(out, domain.ChainLink{FolderID: id, Depth: depth, OwnerID: owner, UnidadeID: unidade})
			i = len(out) - 1
			index[id] = i
		}
		if sType != nil {
			out[i].ACL = append(out[i].ACL, domain.ACLEntry{SubjectType: *sType, Subject: *subject, CanWrite: *canWrite})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(err)
	}
	if len(out) == 0 {
		return nil, domain.ErrNotFound
	}
	return out, nil
}

func (r *Repository) RootFolders(ctx context.Context, db database.DBTX) ([]domain.Folder, map[uuid.UUID][]domain.ACLEntry, error) {
	rows, err := db.Query(ctx, `SELECT `+folderCols+folderFrom+` WHERE f.parent_id IS NULL ORDER BY lower(f.name)`)
	if err != nil {
		return nil, nil, wrap(err)
	}
	var folders []domain.Folder
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			rows.Close()
			return nil, nil, wrap(err)
		}
		folders = append(folders, f)
	}
	rows.Close()
	acls := map[uuid.UUID][]domain.ACLEntry{}
	aclRows, err := db.Query(ctx, `SELECT a.folder_id, a.subject_type, a.subject, a.can_write
		FROM files_folder_acl a JOIN files_folders f ON f.id = a.folder_id WHERE f.parent_id IS NULL`)
	if err != nil {
		return nil, nil, wrap(err)
	}
	defer aclRows.Close()
	for aclRows.Next() {
		var id uuid.UUID
		var e domain.ACLEntry
		if err := aclRows.Scan(&id, &e.SubjectType, &e.Subject, &e.CanWrite); err != nil {
			return nil, nil, wrap(err)
		}
		acls[id] = append(acls[id], e)
	}
	return folders, acls, aclRows.Err()
}

func (r *Repository) Children(ctx context.Context, db database.DBTX, parentID uuid.UUID) ([]domain.Folder, error) {
	rows, err := db.Query(ctx, `SELECT `+folderCols+folderFrom+` WHERE f.parent_id = $1 ORDER BY lower(f.name)`, parentID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Folder{}
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *Repository) Breadcrumbs(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]domain.Folder, error) {
	rows, err := db.Query(ctx, `
		WITH RECURSIVE chain AS (
			SELECT id, parent_id, 0 AS depth FROM files_folders WHERE id = $1
			UNION ALL
			SELECT f.id, f.parent_id, c.depth + 1 FROM files_folders f JOIN chain c ON f.id = c.parent_id WHERE c.depth < 64
		)
		SELECT `+folderCols+folderFrom+` JOIN chain c ON c.id = f.id ORDER BY c.depth DESC`, folderID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Folder{}
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *Repository) GetFolder(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Folder, error) {
	f, err := scanFolder(db.QueryRow(ctx, `SELECT `+folderCols+folderFrom+` WHERE f.id = $1`, id))
	return f, wrap(err)
}

func (r *Repository) SaveFolder(ctx context.Context, db database.DBTX, f domain.Folder) (domain.Folder, error) {
	_, err := db.Exec(ctx, `
		INSERT INTO files_folders (id, parent_id, name, owner_id, unidade_id) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, name = EXCLUDED.name, unidade_id = EXCLUDED.unidade_id`,
		f.ID, f.ParentID, f.Name, f.OwnerID, f.UnidadeID)
	if err != nil {
		return domain.Folder{}, wrap(err)
	}
	return r.GetFolder(ctx, db, f.ID)
}

// DeleteFolder apaga a pasta (cascata no banco) e devolve as chaves dos
// objetos que precisam sair do MinIO.
// FolderEmpty informa se a pasta não tem subpastas nem arquivos.
func (r *Repository) FolderEmpty(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	var busy bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM files_folders WHERE parent_id = $1)
		    OR EXISTS (SELECT 1 FROM files_objects WHERE folder_id = $1)`, id).Scan(&busy)
	return !busy, wrap(err)
}

func (r *Repository) DeleteFolder(ctx context.Context, db database.DBTX, id uuid.UUID) ([]string, error) {
	rows, err := db.Query(ctx, `
		WITH RECURSIVE sub AS (
			SELECT id FROM files_folders WHERE id = $1
			UNION ALL
			SELECT f.id FROM files_folders f JOIN sub s ON f.parent_id = s.id
		)
		SELECT o.object_key FROM files_objects o JOIN sub s ON s.id = o.folder_id`, id)
	if err != nil {
		return nil, wrap(err)
	}
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return nil, wrap(err)
		}
		keys = append(keys, k)
	}
	rows.Close()
	tag, err := db.Exec(ctx, `DELETE FROM files_folders WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return nil, domain.ErrNotFound
	}
	return keys, wrap(err)
}

func (r *Repository) IsDescendant(ctx context.Context, db database.DBTX, candidate, ancestor uuid.UUID) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id FROM files_folders WHERE id = $1
			UNION ALL
			SELECT f.id, f.parent_id FROM files_folders f JOIN up ON f.id = up.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, candidate, ancestor).Scan(&ok)
	return ok, wrap(err)
}

func (r *Repository) ACL(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]domain.ACLEntry, error) {
	rows, err := db.Query(ctx, `SELECT subject_type, subject, can_write FROM files_folder_acl WHERE folder_id = $1
		ORDER BY subject_type, subject`, folderID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.ACLEntry{}
	for rows.Next() {
		var e domain.ACLEntry
		if err := rows.Scan(&e.SubjectType, &e.Subject, &e.CanWrite); err != nil {
			return nil, wrap(err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) ReplaceACL(ctx context.Context, db database.DBTX, folderID uuid.UUID, entries []domain.ACLEntry, grantedBy string) error {
	if _, err := db.Exec(ctx, `DELETE FROM files_folder_acl WHERE folder_id = $1`, folderID); err != nil {
		return wrap(err)
	}
	for _, e := range entries {
		if _, err := db.Exec(ctx, `INSERT INTO files_folder_acl (folder_id, subject_type, subject, can_write, granted_by)
			VALUES ($1,$2,$3,$4,$5)`, folderID, e.SubjectType, e.Subject, e.CanWrite, grantedBy); err != nil {
			return wrap(err)
		}
	}
	return nil
}

const fileCols = `o.id, o.folder_id, o.name, o.object_key, o.size_bytes, o.content_type, o.status, o.owner_id,
	COALESCE(NULLIF(u.display_name,''), u.username, ''), o.created_at, o.updated_at`
const fileFrom = ` FROM files_objects o LEFT JOIN users u ON u.id = o.owner_id `

func scanFile(row interface{ Scan(...any) error }) (domain.File, error) {
	var f domain.File
	err := row.Scan(&f.ID, &f.FolderID, &f.Name, &f.ObjectKey, &f.SizeBytes, &f.ContentType, &f.Status, &f.OwnerID,
		&f.OwnerName, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func (r *Repository) listFiles(ctx context.Context, db database.DBTX, q string, args ...any) ([]domain.File, error) {
	rows, err := db.Query(ctx, `SELECT `+fileCols+fileFrom+q, args...)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.File{}
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *Repository) Files(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]domain.File, error) {
	return r.listFiles(ctx, db, ` WHERE o.folder_id = $1 AND o.status = 'ready' ORDER BY lower(o.name)`, folderID)
}

func (r *Repository) GetFile(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.File, error) {
	f, err := scanFile(db.QueryRow(ctx, `SELECT `+fileCols+fileFrom+` WHERE o.id = $1`, id))
	return f, wrap(err)
}

func (r *Repository) InsertFile(ctx context.Context, db database.DBTX, f domain.File) (domain.File, error) {
	_, err := db.Exec(ctx, `INSERT INTO files_objects (id, folder_id, name, object_key, size_bytes, content_type, status, owner_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, f.ID, f.FolderID, f.Name, f.ObjectKey, f.SizeBytes, f.ContentType, f.Status, f.OwnerID)
	if err != nil {
		return domain.File{}, wrap(err)
	}
	return r.GetFile(ctx, db, f.ID)
}

func (r *Repository) UpdateFile(ctx context.Context, db database.DBTX, f domain.File) (domain.File, error) {
	tag, err := db.Exec(ctx, `UPDATE files_objects SET folder_id=$2, name=$3, size_bytes=$4, content_type=$5, status=$6 WHERE id=$1`,
		f.ID, f.FolderID, f.Name, f.SizeBytes, f.ContentType, f.Status)
	if err != nil {
		return domain.File{}, wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.File{}, domain.ErrNotFound
	}
	return r.GetFile(ctx, db, f.ID)
}

func (r *Repository) DeleteFile(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	tag, err := db.Exec(ctx, `DELETE FROM files_objects WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}

func (r *Repository) StalePending(ctx context.Context, db database.DBTX, olderThan time.Time, limit int) ([]domain.File, error) {
	return r.listFiles(ctx, db, ` WHERE o.status = 'pending' AND o.created_at < $1 ORDER BY o.created_at LIMIT $2`, olderThan, limit)
}

func (r *Repository) SearchFiles(ctx context.Context, db database.DBTX, query string, limit int) ([]domain.File, error) {
	return r.listFiles(ctx, db, ` WHERE o.status = 'ready'
		AND (o.search @@ nexus_search_tsquery('simple', $1)
		     OR strpos(nexus_unaccent(lower(o.name)), nexus_unaccent(lower($1))) > 0)
		ORDER BY o.updated_at DESC LIMIT $2`, query, limit)
}
