package application

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

// ---------------------------------------------------------------- usuários

// ListUsers lista as contas; com users:read delegado, só as da área (e as
// sem lotação nenhuma, para poderem ser lotadas).
func (s *Service) ListUsers(ctx context.Context, f domain.UserFilter, p pagination.Params) ([]domain.User, int64, error) {
	identity, _ := auth.IdentityFromContext(ctx)
	f.Area = areaOf(identity, auth.PermUsersRead)
	return s.repo.ListUsers(ctx, s.pool, f, p)
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	u, err := s.repo.GetUser(ctx, s.pool, id)
	if err != nil {
		return u, MapError(err)
	}
	identity, _ := auth.IdentityFromContext(ctx)
	if area := areaOf(identity, auth.PermUsersRead); area != nil {
		ok, err := s.repo.UserVisible(ctx, s.pool, id, *area)
		if err != nil {
			return domain.User{}, err
		}
		if !ok {
			return domain.User{}, MapError(domain.ErrNotFound)
		}
	}
	return u, nil
}

// UpdateUserInput são os campos administráveis de uma conta.
type UpdateUserInput struct {
	DisplayName string
	Active      bool
	Roles       []string
}

func (s *Service) UpdateUser(ctx context.Context, id uuid.UUID, in UpdateUserInput) (domain.User, error) {
	identity, _ := auth.IdentityFromContext(ctx)
	if identity.UserID == id && !in.Active {
		return domain.User{}, apperrors.Conflict("você não pode desativar a própria conta")
	}
	if in.Roles == nil {
		in.Roles = []string{}
	}
	var out domain.User
	err := s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		prev, err := s.repo.GetUser(ctx, tx, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if err := s.guardArea(ctx, tx, prev, auth.PermUsersManage); err != nil {
			return audit.Entry{}, err
		}
		if err := s.guardTarget(ctx, tx, id); err != nil {
			return audit.Entry{}, err
		}
		if err := guardAdminRole(ctx, prev.Roles, in.Roles); err != nil {
			return audit.Entry{}, err
		}
		if err := s.repo.UpdateUser(ctx, tx, id, in.DisplayName, in.Active, in.Roles); err != nil {
			return audit.Entry{}, err
		}
		if out, err = s.repo.GetUser(ctx, tx, id); err != nil {
			return audit.Entry{}, err
		}
		return audit.Meta(ctx, audit.ActionUserUpdated, "user", id.String(), prev, out), nil
	})
	return out, err
}

// CreateLocalUserInput cria uma conta local (contingência/ambiente isolado).
type CreateLocalUserInput struct {
	Username    string
	Email       string
	DisplayName string
	Password    string
	Roles       []string
}

func (s *Service) CreateLocalUser(ctx context.Context, in CreateLocalUserInput) (domain.User, error) {
	if !usernamePattern.MatchString(in.Username) {
		return domain.User{}, apperrors.Validation("username deve ter 3 a 80 caracteres: letras, dígitos, ponto, hífen ou sublinhado")
	}
	if err := ValidatePasswordStrength(in.Password); err != nil {
		return domain.User{}, err
	}
	if err := guardAdminRole(ctx, nil, in.Roles); err != nil {
		return domain.User{}, err
	}
	hash, err := s.hash(in.Password)
	if err != nil {
		return domain.User{}, apperrors.Internal(err)
	}
	if in.Roles == nil {
		in.Roles = []string{}
	}
	var out domain.User
	err = s.tx(ctx, true, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		id, err := s.repo.CreateLocalUser(ctx, tx, strings.TrimSpace(in.Username), strings.TrimSpace(strings.ToLower(in.Email)), in.DisplayName, hash, in.Roles)
		if err != nil {
			return audit.Entry{}, err
		}
		if out, err = s.repo.GetUser(ctx, tx, id); err != nil {
			return audit.Entry{}, err
		}
		return audit.Meta(ctx, audit.ActionUserCreated, "user", id.String(), nil, out), nil
	})
	return out, err
}

func (s *Service) ResetPassword(ctx context.Context, id uuid.UUID, password string) error {
	if err := ValidatePasswordStrength(password); err != nil {
		return err
	}
	hash, err := s.hash(password)
	if err != nil {
		return apperrors.Internal(err)
	}
	return s.tx(ctx, false, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		u, err := s.repo.GetUser(ctx, tx, id)
		if err != nil {
			return audit.Entry{}, err
		}
		// Conta do AD autentica no AD: uma senha local abriria uma porta
		// de entrada que ignora a política (e a desativação) do diretório.
		if u.Federated {
			return audit.Entry{}, apperrors.Validation("conta federada (Keycloak/AD) não tem senha local; a senha é trocada no AD")
		}
		if err := s.guardArea(ctx, tx, u, auth.PermUsersManage); err != nil {
			return audit.Entry{}, err
		}
		if err := s.guardTarget(ctx, tx, id); err != nil {
			return audit.Entry{}, err
		}
		// Nunca a senha nem o hash vão para a auditoria.
		return audit.Meta(ctx, "user.password.reset", "user", id.String(), nil, nil), s.repo.SetPassword(ctx, tx, id, hash)
	})
}

// Unlock zera o bloqueio da conta (banco) E o bloqueio progressivo de
// login distribuído (Redis) — sem o segundo, o login continuaria
// recusado até a janela expirar.
func (s *Service) Unlock(ctx context.Context, id uuid.UUID) error {
	var username string
	err := s.tx(ctx, false, func(ctx context.Context, tx pgx.Tx) (audit.Entry, error) {
		u, err := s.repo.GetUser(ctx, tx, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if err := s.guardArea(ctx, tx, u, auth.PermUsersManage); err != nil {
			return audit.Entry{}, err
		}
		username = u.Username
		return audit.Meta(ctx, "user.unlocked", "user", id.String(), nil, nil), s.repo.Unlock(ctx, tx, id)
	})
	if err != nil || s.resetLockout == nil {
		return err
	}
	if err := s.resetLockout(ctx, username); err != nil {
		return apperrors.DependencyUnavailable("conta desbloqueada, mas o bloqueio de login distribuído não pôde ser limpo — tente novamente").WithCause(err)
	}
	return nil
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,80}$`)

// ValidatePasswordStrength aplica a política mínima de senha local.
func ValidatePasswordStrength(p string) error {
	if len(p) < 12 {
		return apperrors.Validation("a senha precisa ter pelo menos 12 caracteres")
	}
	var lower, upper, digit, other bool
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		default:
			other = true
		}
	}
	classes := 0
	for _, ok := range []bool{lower, upper, digit, other} {
		if ok {
			classes++
		}
	}
	if classes < 3 {
		return apperrors.Validation("a senha precisa combinar ao menos 3 tipos: minúsculas, maiúsculas, dígitos e símbolos")
	}
	return nil
}
