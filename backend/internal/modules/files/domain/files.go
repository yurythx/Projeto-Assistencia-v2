// Package domain define pastas, arquivos e o controle de acesso (ACL) do
// plugin Arquivos.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

var (
	ErrNotFound  = errors.New("files: registro não encontrado")
	ErrDuplicate = errors.New("files: já existe um item com este nome na pasta")
	ErrForbidden = errors.New("files: acesso negado")
	ErrCycle     = errors.New("files: uma pasta não pode ser movida para dentro de si mesma")
	ErrNotEmpty  = errors.New("files: a pasta não está vazia")
)

// Folder é uma pasta.
type Folder struct {
	ID        uuid.UUID  `json:"id"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Name      string     `json:"name"`
	OwnerID   uuid.UUID  `json:"owner_id"`
	OwnerName string     `json:"owner_name"`
	UnidadeID *uuid.UUID `json:"unidade_id,omitempty"` // dona (ADR 013); herdada pelas subpastas
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// File é um arquivo armazenado no MinIO.
type File struct {
	ID          uuid.UUID `json:"id"`
	FolderID    uuid.UUID `json:"folder_id"`
	Name        string    `json:"name"`
	ObjectKey   string    `json:"-"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	Status      string    `json:"status"`
	OwnerID     uuid.UUID `json:"owner_id"`
	OwnerName   string    `json:"owner_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ACLEntry concede acesso a uma pasta (herdado pelas subpastas).
type ACLEntry struct {
	SubjectType string `json:"subject_type" validate:"required,oneof=everyone user perfil ad_group unidade departamento"`
	Subject     string `json:"subject" validate:"max=200"`
	CanWrite    bool   `json:"can_write"`
}

// ChainLink é uma pasta da cadeia até a raiz, com a ACL própria.
type ChainLink struct {
	FolderID uuid.UUID
	Depth    int
	OwnerID  uuid.UUID
	// UnidadeID é a unidade dona desta pasta (nil = não marcada; a dona
	// efetiva pode vir de uma pasta acima).
	UnidadeID *uuid.UUID
	ACL       []ACLEntry
}

// Access é o resultado da avaliação de permissão.
type Access struct {
	Read   bool `json:"read"`
	Write  bool `json:"write"`
	Manage bool `json:"manage"` // alterar ACL, renomear/excluir a pasta
}

// Evaluate calcula o acesso de identity a uma pasta dada a cadeia de
// pastas até a raiz (depth 0 = a própria pasta). Regras:
//   - files:manage global: acesso total;
//   - files:manage cobrindo a unidade dona da pasta ou de qualquer
//     ancestral (ADR 013): acesso total;
//   - dono da pasta ou de qualquer ancestral: acesso total;
//   - ACL de qualquer nível da cadeia (herança): leitura, e escrita se
//     can_write.
func Evaluate(identity auth.Identity, chain []ChainLink) Access {
	total := Access{Read: true, Write: true, Manage: true}
	if auth.CanGlobal(identity, auth.PermFilesManage) {
		return total
	}
	var a Access
	for _, link := range chain {
		if link.OwnerID == identity.UserID || Gere(identity, link.UnidadeID) {
			return total
		}
		for _, e := range link.ACL {
			if Matches(identity, e) {
				a.Read = true
				if e.CanWrite {
					a.Write = true
				}
			}
		}
	}
	return a
}

// Gere reporta se identity tem files:manage cobrindo a unidade (nil: não).
func Gere(identity auth.Identity, unidade *uuid.UUID) bool {
	return unidade != nil && auth.Can(identity, auth.PermFilesManage, auth.InUnidade(*unidade))
}

// PodeMarcar: só marca uma unidade como dona de pasta quem está lotado nela
// ou tem files:manage cobrindo-a (nil: sempre).
func PodeMarcar(identity auth.Identity, unidade *uuid.UUID) bool {
	return unidade == nil || auth.LotadoEm(identity, *unidade) || Gere(identity, unidade)
}

// Matches reporta se a entrada de ACL se aplica a identity.
func Matches(identity auth.Identity, e ACLEntry) bool {
	switch e.SubjectType {
	case "everyone":
		return true
	case "user":
		return e.Subject == identity.UserID.String()
	case "ad_group":
		return identity.HasGroup(e.Subject)
	case "perfil":
		for _, s := range identity.Scopes {
			if strings.EqualFold(s.Perfil, e.Subject) {
				return true
			}
		}
	case "unidade":
		for _, s := range identity.Scopes {
			if s.UnidadeID != nil && s.UnidadeID.String() == e.Subject {
				return true
			}
		}
	case "departamento":
		for _, s := range identity.Scopes {
			if s.DepartamentoID != nil && s.DepartamentoID.String() == e.Subject {
				return true
			}
		}
	}
	return false
}

// Repository é a porta de persistência.
type Repository interface {
	Chain(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]ChainLink, error)
	RootFolders(ctx context.Context, db database.DBTX) ([]Folder, map[uuid.UUID][]ACLEntry, error)
	Children(ctx context.Context, db database.DBTX, parentID uuid.UUID) ([]Folder, error)
	Breadcrumbs(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]Folder, error)
	GetFolder(ctx context.Context, db database.DBTX, id uuid.UUID) (Folder, error)
	SaveFolder(ctx context.Context, db database.DBTX, f Folder) (Folder, error)
	DeleteFolder(ctx context.Context, db database.DBTX, id uuid.UUID) ([]string, error)
	FolderEmpty(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error)
	IsDescendant(ctx context.Context, db database.DBTX, candidate, ancestor uuid.UUID) (bool, error)
	ACL(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]ACLEntry, error)
	ReplaceACL(ctx context.Context, db database.DBTX, folderID uuid.UUID, entries []ACLEntry, grantedBy string) error

	Files(ctx context.Context, db database.DBTX, folderID uuid.UUID) ([]File, error)
	GetFile(ctx context.Context, db database.DBTX, id uuid.UUID) (File, error)
	InsertFile(ctx context.Context, db database.DBTX, f File) (File, error)
	UpdateFile(ctx context.Context, db database.DBTX, f File) (File, error)
	DeleteFile(ctx context.Context, db database.DBTX, id uuid.UUID) error
	StalePending(ctx context.Context, db database.DBTX, olderThan time.Time, limit int) ([]File, error)
	SearchFiles(ctx context.Context, db database.DBTX, query string, limit int) ([]File, error)
}
