// Package domain define o Diretório de pessoas e setores. A identidade
// (nome, e-mail, grupos) vem do Active Directory, sincronizada no login
// federado; o Diretório acrescenta o perfil estendido.
package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// ErrNotFound indica pessoa inexistente/invisível.
var ErrNotFound = errors.New("directory: pessoa não encontrada")

// ErrSector indica unidade/departamento inexistente ou inconsistente (o
// departamento precisa pertencer à unidade informada).
var ErrSector = errors.New("directory: unidade ou departamento inexistente ou inconsistente")

// ErrOutOfScope: directory:manage não cobre a lotação exibida da pessoa
// (a atual ou a nova) — ADR 013.
var ErrOutOfScope = errors.New("directory: pessoa fora do seu escopo de gestão")

// Person é uma entrada do diretório.
type Person struct {
	UserID         uuid.UUID  `json:"user_id"`
	Name           string     `json:"name"`
	Username       string     `json:"username"`
	Email          string     `json:"email"`
	JobTitle       string     `json:"job_title"`
	Phone          string     `json:"phone"`
	Extension      string     `json:"extension"`
	Bio            string     `json:"bio"`
	Visible        bool       `json:"visible"`
	UnidadeID      *uuid.UUID `json:"unidade_id,omitempty"`
	Unidade        string     `json:"unidade"`
	DepartamentoID *uuid.UUID `json:"departamento_id,omitempty"`
	Departamento   string     `json:"departamento"`
	FromAD         bool       `json:"from_ad"`
}

// Sector é um setor (departamento) na consulta pública.
type Sector struct {
	ID        uuid.UUID  `json:"id"`
	Kind      string     `json:"kind"` // unidade | departamento
	Nome      string     `json:"nome"`
	Sigla     string     `json:"sigla"`
	Email     string     `json:"email"`
	Telefone  string     `json:"telefone"`
	Endereco  string     `json:"endereco,omitempty"`
	UnidadeID *uuid.UUID `json:"unidade_id,omitempty"`
	Unidade   string     `json:"unidade,omitempty"`
	Entidade  string     `json:"entidade"`
}

// Filter restringe a busca de pessoas.
type Filter struct {
	Query          string
	UnidadeID      *uuid.UUID
	DepartamentoID *uuid.UUID
	IncludeHidden  bool
	// Perfis ocultos visíveis só nas unidades/departamentos que a gestão
	// (directory:manage com escopo) cobre.
	HiddenUnidades      []uuid.UUID
	HiddenDepartamentos []uuid.UUID
}

// Posicao é a lotação exibida da pessoa; sem ela, a pessoa é
// "institucional" (só a gestão global a edita — ADR 013).
func (p Person) Posicao() auth.Target {
	return Lotacao(p.UnidadeID, p.DepartamentoID)
}

// Lotacao é o alvo de uma lotação exibida.
func Lotacao(unidade, departamento *uuid.UUID) auth.Target {
	return auth.Target{UnidadeID: unidade, DepartamentoID: departamento}
}

// ProfileInput são os campos do perfil estendido.
type ProfileInput struct {
	JobTitle       string
	Phone          string
	Extension      string
	Bio            string
	Visible        bool
	UnidadeID      *uuid.UUID
	DepartamentoID *uuid.UUID
}

// Repository é a porta de persistência.
type Repository interface {
	List(ctx context.Context, db database.DBTX, f Filter, p pagination.Params) ([]Person, int64, error)
	Get(ctx context.Context, db database.DBTX, userID uuid.UUID) (Person, error)
	SaveProfile(ctx context.Context, db database.DBTX, userID uuid.UUID, in ProfileInput, keepLotacao bool) error
	Sectors(ctx context.Context, db database.DBTX, query string) ([]Sector, error)
	// DepartamentoUnidade devolve a unidade a que o departamento pertence.
	DepartamentoUnidade(ctx context.Context, db database.DBTX, departamentoID uuid.UUID) (uuid.UUID, error)
	// DeleteProfile apaga o perfil estendido (eliminação LGPD, art. 18, VI).
	DeleteProfile(ctx context.Context, db database.DBTX, userID uuid.UUID) error
}
