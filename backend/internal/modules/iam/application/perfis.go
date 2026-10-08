package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

// ------------------------------------------------------------------ perfis

func (s *Service) ListPerfis(ctx context.Context) ([]domain.Perfil, error) {
	return s.repo.ListPerfis(ctx, s.pool)
}

func normalizePermissions(perms []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if !domain.ValidPermission(p) {
			return nil, fmt.Errorf("%w: %q (use recurso:acao, recurso:* ou *)", domain.ErrInvalidPermision, p)
		}
		if _, dup := seen[p]; !dup {
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Service) SavePerfil(ctx context.Context, in domain.Perfil) (domain.Perfil, error) {
	if err := soGlobal(ctx); err != nil {
		return domain.Perfil{}, err
	}
	perms, err := normalizePermissions(in.Permissoes)
	if err != nil {
		return domain.Perfil{}, MapError(err)
	}
	in.Permissoes = perms
	if in.Slug == "" {
		in.Slug = modkit.Slugify(in.Nome)
	}
	var out domain.Perfil
	err = s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		var before any
		var prevPerms []string
		if in.ID == uuid.Nil {
			in.ID = uuid.New()
		} else {
			prev, err := s.repo.GetPerfil(ctx, tx, in.ID)
			if err != nil {
				return audit.Entry{}, err
			}
			// O administrador de sistema mantém "*" sempre: um erro de
			// edição não pode trancar toda a plataforma.
			if prev.Sistema && prev.Slug == "administrador" {
				in.Permissoes = []string{"*"}
				in.Slug = prev.Slug
				in.Ativo = true
			}
			if prev.Sistema {
				in.Slug = prev.Slug
			}
			before = prev
			prevPerms = prev.Permissoes
			// Desativar (ou reativar) um perfil retira (ou devolve) todas
			// as permissões dele de quem o tem.
			if prev.Ativo != in.Ativo {
				if err := guardGrant(ctx, nil, prev.Permissoes); err != nil {
					return audit.Entry{}, err
				}
			}
		}
		if err := guardGrant(ctx, prevPerms, in.Permissoes); err != nil {
			return audit.Entry{}, err
		}
		var err error
		out, err = s.repo.UpsertPerfil(ctx, tx, in)
		return audit.Meta(ctx, "iam.perfil.saved", "perfil", out.ID.String(), before, out), err
	})
	return out, err
}

func (s *Service) DeletePerfil(ctx context.Context, id uuid.UUID) error {
	if err := soGlobal(ctx); err != nil {
		return err
	}
	return s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		prev, err := s.repo.GetPerfil(ctx, tx, id)
		if err != nil {
			return audit.Entry{}, err
		}
		return audit.Meta(ctx, "iam.perfil.deleted", "perfil", id.String(), prev, nil), s.repo.DeletePerfil(ctx, tx, id)
	})
}

// ------------------------------------------------------- mapeamentos do AD

func (s *Service) ListMappings(ctx context.Context) ([]domain.ADMapping, error) {
	if err := soGlobal(ctx); err != nil {
		return nil, err
	}
	return s.repo.ListMappings(ctx, s.pool)
}

func (s *Service) CreateMapping(ctx context.Context, in domain.ADMapping) (uuid.UUID, error) {
	if err := soGlobal(ctx); err != nil {
		return uuid.Nil, err
	}
	in.ADGroup = strings.TrimSpace(in.ADGroup)
	if in.ADGroup == "" {
		return uuid.Nil, apperrors.Validation("ad_group é obrigatório")
	}
	identity, _ := auth.IdentityFromContext(ctx)
	in.CreatedBy = identity.Username
	var id uuid.UUID
	err := s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		if err := s.guardPerfilGrant(ctx, tx, in.PerfilID); err != nil {
			return audit.Entry{}, err
		}
		scope, err := s.repo.ScopeConsistent(ctx, tx, in.Scope)
		if err != nil {
			return audit.Entry{}, err
		}
		in.Scope = scope
		if id, err = s.repo.CreateMapping(ctx, tx, in); err != nil {
			return audit.Entry{}, err
		}
		in.ID = id
		return audit.Meta(ctx, "iam.ad_mapping.created", "ad_group_mapping", id.String(), nil, in), nil
	})
	return id, err
}

func (s *Service) DeleteMapping(ctx context.Context, id uuid.UUID) error {
	if err := soGlobal(ctx); err != nil {
		return err
	}
	return s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		prev, err := s.repo.GetMapping(ctx, tx, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if err := s.guardPerfilGrant(ctx, tx, prev.PerfilID); err != nil {
			return audit.Entry{}, err
		}
		return audit.Meta(ctx, "iam.ad_mapping.deleted", "ad_group_mapping", id.String(), prev, nil), s.repo.DeleteMapping(ctx, tx, id)
	})
}
