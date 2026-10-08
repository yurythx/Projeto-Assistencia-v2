// Package domain define as publicações internas e comunicados do Blog.
package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/publico"
)

var (
	ErrNotFound     = errors.New("blog: publicação não encontrada")
	ErrSlugTaken    = errors.New("blog: slug já utilizado")
	ErrInvalidState = errors.New("blog: transição de estado inválida")
	ErrEmptyBody    = errors.New("blog: publicação sem texto não pode ser publicada")
	// ErrOutOfScope: blog:manage não cobre a unidade dona da publicação
	// (sem dono = institucional, só a gestão global — ADR 013).
	ErrOutOfScope = errors.New("blog: publicação fora do seu escopo de gestão")
)

// Estados e tipos.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"

	KindNoticia    = "noticia"
	KindComunicado = "comunicado"
)

// Post é uma publicação.
type Post struct {
	ID             uuid.UUID  `json:"id"`
	Slug           string     `json:"slug"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Body           string     `json:"body"`
	CoverObjectKey string     `json:"cover_object_key,omitempty"`
	CoverURL       string     `json:"cover_url,omitempty"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	Pinned         bool       `json:"pinned"`
	AuthorID       *uuid.UUID `json:"author_id,omitempty"`
	// UnidadeID é a unidade dona (ADR 013); nil = institucional.
	UnidadeID *uuid.UUID `json:"unidade_id,omitempty"`
	// Publico é o público-alvo (ADR 014); vazio = todos.
	Publico     auth.Publico `json:"publico"`
	AuthorName  string       `json:"author_name,omitempty"`
	PublishedAt *time.Time   `json:"published_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Filter restringe a listagem.
type Filter struct {
	Status string // vazio = só publicados
	Kind   string
	Query  string
	// Restrito: rascunhos/arquivados só das unidades em Unidades (gestão
	// com escopo); publicados aparecem sempre.
	Restrito bool
	Unidades []uuid.UUID
	// Leitura: quem lê (público-alvo, autor, gestão — ADR 014).
	Leitura publico.Leitura
}

// Repository é a porta de persistência.
type Repository interface {
	List(ctx context.Context, db database.DBTX, f Filter, p pagination.Params) ([]Post, int64, error)
	Get(ctx context.Context, db database.DBTX, id uuid.UUID) (Post, error)
	GetBySlug(ctx context.Context, db database.DBTX, slug string) (Post, error)
	Insert(ctx context.Context, db database.DBTX, p Post) (Post, error)
	Update(ctx context.Context, db database.DBTX, p Post) (Post, error)
	Delete(ctx context.Context, db database.DBTX, id uuid.UUID) error
	Search(ctx context.Context, db database.DBTX, query string, limit int, l publico.Leitura) ([]Post, []float64, error)
	// SetPublico substitui o público-alvo do post.
	SetPublico(ctx context.Context, db database.DBTX, id uuid.UUID, p auth.Publico) error
}

// CanTransition valida o ciclo de vida draft -> published -> archived
// (e retorno a rascunho).
func CanTransition(from, to string) bool {
	switch from {
	case StatusDraft:
		return to == StatusPublished || to == StatusArchived
	case StatusPublished:
		return to == StatusArchived || to == StatusDraft
	case StatusArchived:
		return to == StatusDraft || to == StatusPublished
	}
	return false
}
