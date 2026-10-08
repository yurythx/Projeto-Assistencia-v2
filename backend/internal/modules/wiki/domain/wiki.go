// Package domain define a base de conhecimento (Wiki) em árvore.
package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

var (
	ErrNotFound    = errors.New("wiki: página não encontrada")
	ErrSlugTaken   = errors.New("wiki: slug já utilizado")
	ErrStale       = errors.New("wiki: a página foi alterada por outra pessoa")
	ErrHasChildren = errors.New("wiki: a página possui subpáginas")
	ErrCycle       = errors.New("wiki: uma página não pode ficar dentro de si mesma")
	// ErrOutOfScope: a unidade dona escolhida não é da pessoa, ou wiki:manage
	// não cobre a dona da página a excluir (ADR 013).
	ErrOutOfScope = errors.New("wiki: página fora do seu escopo")
)

// Page é uma página da Wiki.
type Page struct {
	ID            uuid.UUID    `json:"id"`
	ParentID      *uuid.UUID   `json:"parent_id,omitempty"`
	UnidadeID     *uuid.UUID   `json:"unidade_id,omitempty"` // dona (ADR 013); nil = institucional
	Publico       auth.Publico `json:"publico"`              // público-alvo próprio (ADR 014); vale com o das páginas acima
	Slug          string       `json:"slug"`
	Title         string       `json:"title"`
	Body          string       `json:"body,omitempty"`
	Position      int          `json:"position"`
	Version       int          `json:"version"`
	CreatedBy     uuid.UUID    `json:"created_by"`
	UpdatedBy     uuid.UUID    `json:"updated_by"`
	UpdatedByName string       `json:"updated_by_name"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// Revision é uma versão histórica.
type Revision struct {
	ID           uuid.UUID `json:"id"`
	PageID       uuid.UUID `json:"page_id"`
	Version      int       `json:"version"`
	Title        string    `json:"title"`
	Body         string    `json:"body,omitempty"`
	Summary      string    `json:"summary"`
	EditedBy     uuid.UUID `json:"edited_by"`
	EditedByName string    `json:"edited_by_name"`
	EditedAt     time.Time `json:"edited_at"`
}

// Repository é a porta de persistência.
type Repository interface {
	Tree(ctx context.Context, db database.DBTX) ([]Page, error)
	Get(ctx context.Context, db database.DBTX, id uuid.UUID) (Page, error)
	GetBySlug(ctx context.Context, db database.DBTX, slug string) (Page, error)
	Breadcrumbs(ctx context.Context, db database.DBTX, id uuid.UUID) ([]Page, error)
	Insert(ctx context.Context, db database.DBTX, p Page) error
	// UpdateIfVersion aplica a edição só se a versão atual for expected.
	UpdateIfVersion(ctx context.Context, db database.DBTX, p Page, expected int) error
	Delete(ctx context.Context, db database.DBTX, id uuid.UUID) error
	IsDescendant(ctx context.Context, db database.DBTX, candidate, ancestor uuid.UUID) (bool, error)
	AddRevision(ctx context.Context, db database.DBTX, r Revision) error
	Revisions(ctx context.Context, db database.DBTX, pageID uuid.UUID) ([]Revision, error)
	Revision(ctx context.Context, db database.DBTX, pageID uuid.UUID, version int) (Revision, error)
	Search(ctx context.Context, db database.DBTX, q string, limit int) ([]Page, []float64, error)
	// Publicos devolve o público-alvo próprio de cada página que tem um.
	Publicos(ctx context.Context, db database.DBTX) (map[uuid.UUID]auth.Publico, error)
	// SetPublico substitui o público-alvo próprio da página.
	SetPublico(ctx context.Context, db database.DBTX, id uuid.UUID, p auth.Publico) error
}
