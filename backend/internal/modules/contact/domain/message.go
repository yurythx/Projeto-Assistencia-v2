// Package domain define as mensagens do formulário público de Contato.
package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// ErrNotFound indica mensagem inexistente.
var ErrNotFound = errors.New("contact: mensagem não encontrada")

// ErrAssignee indica responsável inexistente ou desativado.
var ErrAssignee = errors.New("contact: responsável inexistente ou desativado")

// ErrUnidade: encaminhamento para unidade inexistente ou desativada.
var ErrUnidade = errors.New("contact: unidade de destino inexistente ou desativada")

// ErrOutOfScope: contact:manage não cobre a unidade de destino (ADR 013).
var ErrOutOfScope = errors.New("contact: setor fora do seu escopo")

// Message é uma mensagem recebida (contém PII de visitante externo).
type Message struct {
	ID         uuid.UUID  `json:"id"`
	Protocol   string     `json:"protocol"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	Phone      string     `json:"phone"`
	Subject    string     `json:"subject"`
	Category   string     `json:"category"`
	Message    string     `json:"message"`
	ConsentAt  time.Time  `json:"consent_at"`
	IPAddress  string     `json:"ip_address,omitempty"`
	UserAgent  string     `json:"-"`
	Status     string     `json:"status"`
	AssignedTo *uuid.UUID `json:"assigned_to,omitempty"`
	UnidadeID  *uuid.UUID `json:"unidade_id,omitempty"` // setor encaminhado; nil = caixa geral
	Notes      string     `json:"notes"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Summary é a forma de listagem (sem o corpo e sem telefone/IP).
type Summary struct {
	ID        uuid.UUID  `json:"id"`
	Protocol  string     `json:"protocol"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Subject   string     `json:"subject"`
	Category  string     `json:"category"`
	Status    string     `json:"status"`
	UnidadeID *uuid.UUID `json:"unidade_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Filter restringe a listagem. Com Restrito, só as mensagens encaminhadas
// às Unidades (contact:read com escopo); sem, todas (gestão global).
type Filter struct {
	Status    string
	Restrito  bool
	Unidades  []uuid.UUID
	UnidadeID *uuid.UUID // filtro opcional por setor
}

// Triage é o que a triagem altera.
type Triage struct {
	Status, Notes string
	AssignedTo    *uuid.UUID
	UnidadeID     *uuid.UUID
}

// Posicao é onde a mensagem está: o setor encaminhado; sem ele, a caixa
// geral (institucional — só a gestão global).
func (m Message) Posicao() auth.Target {
	if m.UnidadeID == nil {
		return auth.Target{}
	}
	return auth.InUnidade(*m.UnidadeID)
}

// Repository é a porta de persistência.
type Repository interface {
	Insert(ctx context.Context, db database.DBTX, m Message) (Message, error)
	List(ctx context.Context, db database.DBTX, f Filter, p pagination.Params) ([]Summary, int64, error)
	Get(ctx context.Context, db database.DBTX, id uuid.UUID) (Message, error)
	UpdateTriage(ctx context.Context, db database.DBTX, id uuid.UUID, t Triage) error
	// ActiveUser reporta se a conta existe e está ativa (responsável).
	ActiveUser(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error)
	// ActiveUnidade reporta se a unidade existe e está ativa (ela e a entidade).
	ActiveUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error)
}
