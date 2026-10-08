// Package application contém os casos de uso do Diretório.
package application

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/directory/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Service implementa os casos de uso.
type Service struct {
	pool *pgxpool.Pool
	repo domain.Repository
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository) *Service {
	return &Service{pool: pool, repo: repo}
}

// MapError traduz erros de domínio.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("pessoa não encontrada no diretório")
	case errors.Is(err, domain.ErrSector):
		return apperrors.Validation(err.Error())
	case errors.Is(err, domain.ErrOutOfScope):
		return apperrors.Forbidden(err.Error())
	}
	return err
}

// List busca pessoas; perfis ocultos só aparecem para directory:manage.
func (s *Service) List(ctx context.Context, identity auth.Identity, f domain.Filter, p pagination.Params) ([]domain.Person, int64, error) {
	gestao := auth.CoverageOf(identity, auth.PermDirectoryManage)
	f.IncludeHidden, f.HiddenUnidades, f.HiddenDepartamentos = gestao.All, gestao.Unidades, gestao.Departamentos
	return s.repo.List(ctx, s.pool, f, p)
}

// Get devolve uma pessoa (oculta só para o próprio ou directory:manage).
func (s *Service) Get(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.Person, error) {
	p, err := s.repo.Get(ctx, s.pool, id)
	if err != nil {
		return p, MapError(err)
	}
	if !p.Visible && identity.UserID != id && !auth.Can(identity, auth.PermDirectoryManage, p.Posicao()) {
		return domain.Person{}, MapError(domain.ErrNotFound)
	}
	return p, nil
}

// SaveProfile atualiza o perfil estendido. self = autoatendimento (não
// altera a lotação exibida).
func (s *Service) SaveProfile(ctx context.Context, identity auth.Identity, userID uuid.UUID, in domain.ProfileInput, self bool) (domain.Person, error) {
	if self {
		// Autoatendimento nunca define a lotação exibida — nem no primeiro
		// salvamento (quando o perfil ainda não existe).
		in.UnidadeID, in.DepartamentoID = nil, nil
	}
	var out domain.Person
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.Get(ctx, tx, userID)
		if err != nil {
			return err
		}
		if in.DepartamentoID != nil {
			unidade, err := s.repo.DepartamentoUnidade(ctx, tx, *in.DepartamentoID)
			if err != nil {
				return err
			}
			if in.UnidadeID != nil && *in.UnidadeID != unidade {
				return domain.ErrSector
			}
			in.UnidadeID = &unidade
		}
		// Gestão (não autoatendimento): directory:manage precisa cobrir a
		// lotação exibida atual da pessoa e a nova (ADR 013).
		if !self && (!auth.Can(identity, auth.PermDirectoryManage, prev.Posicao()) ||
			!auth.Can(identity, auth.PermDirectoryManage, domain.Lotacao(in.UnidadeID, in.DepartamentoID))) {
			return domain.ErrOutOfScope
		}
		if err := s.repo.SaveProfile(ctx, tx, userID, in, self); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, userID); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "directory.profile.updated", "user", userID.String(), prev, out))
	})
	return out, MapError(err)
}

// Sectors lista unidades/departamentos ativos (consulta pública).
func (s *Service) Sectors(ctx context.Context, query string) ([]domain.Sector, error) {
	return s.repo.Sectors(ctx, s.pool, query)
}

// ExportPersonalData devolve o perfil do titular para o pacote de
// exportação da LGPD (art. 18, II e V). Titular sem conta ativa: nada.
func (s *Service) ExportPersonalData(ctx context.Context, db database.DBTX, userID uuid.UUID) (any, error) {
	p, err := s.repo.Get(ctx, db, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"job_title": p.JobTitle, "phone": p.Phone, "extension": p.Extension, "bio": p.Bio,
		"visible": p.Visible, "unidade": p.Unidade, "departamento": p.Departamento,
	}, nil
}

// ErasePersonalData apaga o perfil do titular na transação da eliminação.
func (s *Service) ErasePersonalData(ctx context.Context, tx database.DBTX, userID uuid.UUID) error {
	return s.repo.DeleteProfile(ctx, tx, userID)
}
