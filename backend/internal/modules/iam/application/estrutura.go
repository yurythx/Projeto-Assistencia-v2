package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

// ------------------------------------------------------------ estrutura org.

// OrgTree devolve entidades > unidades > departamentos.
func (s *Service) OrgTree(ctx context.Context) ([]domain.OrgTree, error) {
	ents, err := s.repo.ListEntidades(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	unids, err := s.repo.ListUnidades(ctx, s.pool, nil)
	if err != nil {
		return nil, err
	}
	deps, err := s.repo.ListDepartamentos(ctx, s.pool, nil)
	if err != nil {
		return nil, err
	}
	byUnidade := map[uuid.UUID][]domain.Departamento{}
	for _, d := range deps {
		byUnidade[d.UnidadeID] = append(byUnidade[d.UnidadeID], d)
	}
	byEntidade := map[uuid.UUID][]domain.OrgUnidade{}
	for _, u := range unids {
		d := byUnidade[u.ID]
		if d == nil {
			d = []domain.Departamento{}
		}
		byEntidade[u.EntidadeID] = append(byEntidade[u.EntidadeID], domain.OrgUnidade{Unidade: u, Departamentos: d})
	}
	out := make([]domain.OrgTree, 0, len(ents))
	for _, e := range ents {
		u := byEntidade[e.ID]
		if u == nil {
			u = []domain.OrgUnidade{}
		}
		out = append(out, domain.OrgTree{Entidade: e, Unidades: u})
	}
	return out, nil
}

func (s *Service) ListEntidades(ctx context.Context) ([]domain.Entidade, error) {
	return s.repo.ListEntidades(ctx, s.pool)
}

// SaveEntidade cria (ID nil) ou atualiza uma entidade.
// SaveEntidade/SaveUnidade/SaveDepartamento invalidam o cache do IAM: o
// "ativo" de cada nível liga ou desliga as lotações e mapeamentos nele
// (nexus_scope_active, migration 000126).
func (s *Service) SaveEntidade(ctx context.Context, in domain.Entidade) (domain.Entidade, error) {
	if err := soGlobal(ctx); err != nil {
		return domain.Entidade{}, err
	}
	if in.Slug == "" {
		in.Slug = modkit.Slugify(in.Nome)
	}
	var out domain.Entidade
	err := s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		var before any
		if in.ID == uuid.Nil {
			in.ID = uuid.New()
		} else if prev, err := s.repo.GetEntidade(ctx, tx, in.ID); err == nil {
			before = prev
		} else {
			return audit.Entry{}, err
		}
		var err error
		out, err = s.repo.UpsertEntidade(ctx, tx, in)
		return audit.Meta(ctx, "iam.entidade.saved", "entidade", out.ID.String(), before, out), err
	})
	return out, err
}

func (s *Service) DeleteEntidade(ctx context.Context, id uuid.UUID) error {
	if err := soGlobal(ctx); err != nil {
		return err
	}
	return s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		prev, err := s.repo.GetEntidade(ctx, tx, id)
		if err != nil {
			return audit.Entry{}, err
		}
		return audit.Meta(ctx, "iam.entidade.deleted", "entidade", id.String(), prev, nil), s.repo.DeleteEntidade(ctx, tx, id)
	})
}

func (s *Service) ListUnidades(ctx context.Context, entidadeID *uuid.UUID) ([]domain.Unidade, error) {
	return s.repo.ListUnidades(ctx, s.pool, entidadeID)
}

func (s *Service) SaveUnidade(ctx context.Context, in domain.Unidade) (domain.Unidade, error) {
	if in.Slug == "" {
		in.Slug = modkit.Slugify(in.Nome)
	}
	var out domain.Unidade
	err := s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		var before any
		if in.ID == uuid.Nil {
			// Criar: a administração precisa cobrir onde a unidade fica.
			if err := alcanca(ctx, auth.PermIAMManage, posicaoUnidade(in)); err != nil {
				return audit.Entry{}, err
			}
			in.ID = uuid.New()
		} else if prev, err := s.repo.GetUnidade(ctx, tx, in.ID); err == nil {
			// Lotações e mapeamentos gravam a entidade da unidade: mudar a
			// unidade de entidade deixaria esses escopos inconsistentes.
			if prev.EntidadeID != in.EntidadeID {
				return audit.Entry{}, fmt.Errorf("%w: a unidade não pode mudar de entidade", domain.ErrInvalidScope)
			}
			// Editar: cobrir a unidade; mudar de mãe, cobrir também o destino.
			if err := alcanca(ctx, auth.PermIAMManage, auth.InUnidade(in.ID)); err != nil {
				return audit.Entry{}, err
			}
			if !sameID(prev.ParentID, in.ParentID) {
				if err := alcanca(ctx, auth.PermIAMManage, posicaoUnidade(in)); err != nil {
					return audit.Entry{}, err
				}
			}
			before = prev
		} else {
			return audit.Entry{}, err
		}
		if in.ParentID != nil {
			parent, err := s.repo.GetUnidade(ctx, tx, *in.ParentID)
			if errors.Is(err, domain.ErrNotFound) {
				return audit.Entry{}, fmt.Errorf("%w: unidade-mãe inexistente", domain.ErrInvalidScope)
			}
			if err != nil {
				return audit.Entry{}, err
			}
			if parent.EntidadeID != in.EntidadeID {
				return audit.Entry{}, fmt.Errorf("%w: a unidade-mãe pertence a outra entidade", domain.ErrInvalidScope)
			}
			cycle, err := s.repo.UnidadeCreatesCycle(ctx, tx, in.ID, *in.ParentID)
			if err != nil {
				return audit.Entry{}, err
			}
			if cycle {
				return audit.Entry{}, fmt.Errorf("%w: hierarquia circular", domain.ErrInvalidScope)
			}
		}
		var err error
		out, err = s.repo.UpsertUnidade(ctx, tx, in)
		return audit.Meta(ctx, "iam.unidade.saved", "unidade", out.ID.String(), before, out), err
	})
	return out, err
}

func (s *Service) DeleteUnidade(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		prev, err := s.repo.GetUnidade(ctx, tx, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if err := alcanca(ctx, auth.PermIAMManage, posicaoUnidade(prev)); err != nil {
			return audit.Entry{}, err
		}
		return audit.Meta(ctx, "iam.unidade.deleted", "unidade", id.String(), prev, nil), s.repo.DeleteUnidade(ctx, tx, id)
	})
}

func (s *Service) ListDepartamentos(ctx context.Context, unidadeID *uuid.UUID) ([]domain.Departamento, error) {
	return s.repo.ListDepartamentos(ctx, s.pool, unidadeID)
}

func (s *Service) SaveDepartamento(ctx context.Context, in domain.Departamento) (domain.Departamento, error) {
	if in.Slug == "" {
		in.Slug = modkit.Slugify(in.Nome)
	}
	var out domain.Departamento
	err := s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		if err := alcanca(ctx, auth.PermIAMManage, auth.InUnidade(in.UnidadeID)); err != nil {
			return audit.Entry{}, err
		}
		var before any
		if in.ID == uuid.Nil {
			in.ID = uuid.New()
		} else if prev, err := s.repo.GetDepartamento(ctx, tx, in.ID); err == nil {
			if prev.UnidadeID != in.UnidadeID {
				return audit.Entry{}, fmt.Errorf("%w: o departamento não pode mudar de unidade", domain.ErrInvalidScope)
			}
			before = prev
		} else {
			return audit.Entry{}, err
		}
		var err error
		out, err = s.repo.UpsertDepartamento(ctx, tx, in)
		return audit.Meta(ctx, "iam.departamento.saved", "departamento", out.ID.String(), before, out), err
	})
	return out, err
}

func (s *Service) DeleteDepartamento(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		prev, err := s.repo.GetDepartamento(ctx, tx, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if err := alcanca(ctx, auth.PermIAMManage, auth.InUnidade(prev.UnidadeID)); err != nil {
			return audit.Entry{}, err
		}
		return audit.Meta(ctx, "iam.departamento.deleted", "departamento", id.String(), prev, nil), s.repo.DeleteDepartamento(ctx, tx, id)
	})
}
