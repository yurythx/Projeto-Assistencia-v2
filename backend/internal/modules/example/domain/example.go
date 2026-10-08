package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrExampleNotFound = errors.New("example: item não encontrado")
	ErrInvalidInput    = errors.New("example: título obrigatório (até 200 caracteres)")
)

// StatusActive é o estado inicial de todo item.
const StatusActive = "ACTIVE"

// TitleMax é o tamanho máximo do título (em caracteres).
const TitleMax = 200

// Item representa uma entidade de exemplo genérica no Projeto Nexus.
// Serve como modelo para novos módulos a serem criados no sistema.
type Item struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewItem cria e valida um novo Item de exemplo. As invariantes moram no
// domínio (não só nas struct-tags do transporte): o título é aparado e
// não pode ficar vazio — "   " passaria no validate:"required".
func NewItem(title, description string) (Item, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > TitleMax {
		return Item{}, ErrInvalidInput
	}
	now := time.Now().UTC()
	return Item{
		ID:          uuid.New(),
		Title:       title,
		Description: strings.TrimSpace(description),
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
