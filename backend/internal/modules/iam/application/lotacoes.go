package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

// ---------------------------------------------------------------- lotações

// guardPerfilGrant: atribuir um perfil (lotação ou mapeamento do AD)
// concede todas as permissões dele.
func (s *Service) guardPerfilGrant(ctx context.Context, tx pgx.Tx, perfilID uuid.UUID) error {
	p, err := s.repo.GetPerfil(ctx, tx, perfilID)
	if err != nil {
		return err
	}
	return guardGrant(ctx, nil, p.Permissoes)
}

// ListLotacoes lista as lotações da conta; com iam:manage delegado, só as
// que a administração cobre.
func (s *Service) ListLotacoes(ctx context.Context, userID uuid.UUID) ([]domain.Lotacao, error) {
	all, err := s.repo.ListLotacoes(ctx, s.pool, userID)
	if err != nil {
		return nil, err
	}
	identity, _ := auth.IdentityFromContext(ctx)
	out := []domain.Lotacao{}
	for _, l := range all {
		if auth.Can(identity, auth.PermIAMManage, alvo(l.Scope)) {
			out = append(out, l)
		}
	}
	return out, nil
}

func (s *Service) CreateLotacao(ctx context.Context, in domain.Lotacao) (uuid.UUID, error) {
	identity, _ := auth.IdentityFromContext(ctx)
	in.CreatedBy = identity.Username
	var id uuid.UUID
	err := s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		if _, err := s.repo.GetUser(ctx, tx, in.UserID); err != nil {
			return audit.Entry{}, err
		}
		scope, err := s.repo.ScopeConsistent(ctx, tx, in.Scope)
		if err != nil {
			return audit.Entry{}, err
		}
		in.Scope = scope
		// A lotação precisa cair na área de quem lota, com um perfil cujas
		// permissões ele próprio tem ali (ADR 013).
		if err := alcanca(ctx, auth.PermIAMManage, alvo(scope)); err != nil {
			return audit.Entry{}, err
		}
		if err := s.guardPerfilAt(ctx, tx, in.PerfilID, alvo(scope), "conceder"); err != nil {
			return audit.Entry{}, err
		}
		if id, err = s.repo.CreateLotacao(ctx, tx, in); err != nil {
			return audit.Entry{}, err
		}
		in.ID = id
		return audit.Meta(ctx, "iam.lotacao.created", "user", in.UserID.String(), nil, in), nil
	})
	return id, err
}

// DeleteLotacao remove a lotação; retirar um perfil exige cobrir as
// permissões dele (mesma regra da concessão). A auditoria guarda o que foi
// retirado (perfil e escopo), não só o id.
func (s *Service) DeleteLotacao(ctx context.Context, userID, id uuid.UUID) error {
	return s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		prev, err := s.repo.GetLotacao(ctx, tx, userID, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if err := alcanca(ctx, auth.PermIAMManage, alvo(prev.Scope)); err != nil {
			return audit.Entry{}, err
		}
		if err := s.guardPerfilAt(ctx, tx, prev.PerfilID, alvo(prev.Scope), "retirar"); err != nil {
			return audit.Entry{}, err
		}
		return audit.Meta(ctx, "iam.lotacao.deleted", "user", userID.String(), prev, nil), s.repo.DeleteLotacao(ctx, tx, userID, id)
	})
}
