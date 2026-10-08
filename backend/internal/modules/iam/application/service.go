// Package application contém os casos de uso do IAM. Toda mutação grava
// a auditoria (com diff antes/depois) na mesma transação e, ao final,
// invalida o cache de permissões de todas as réplicas.
package application

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/passwords"
)

// Service implementa os casos de uso do IAM.
type Service struct {
	pool         *pgxpool.Pool
	repo         domain.Repository
	invalidate   func(ctx context.Context)
	resetLockout func(ctx context.Context, username string) error
	hash         func(password string) (string, error)
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, invalidate func(context.Context)) *Service {
	if invalidate == nil {
		invalidate = func(context.Context) {}
	}
	return &Service{pool: pool, repo: repo, invalidate: invalidate, hash: passwords.Hash}
}

// WithLoginLockoutReset conecta a limpeza do bloqueio progressivo de login
// (Redis) ao desbloqueio administrativo.
func (s *Service) WithLoginLockoutReset(fn func(ctx context.Context, username string) error) *Service {
	s.resetLockout = fn
	return s
}

// ------------------------------------------------ administração delegada
//
// ADR 013, fase 3: iam:manage, users:read e users:manage valem no escopo
// em que foram concedidos. Entidades, perfis e mapeamentos do AD continuam
// da gestão global; a estrutura, as lotações e as contas, da área.

// soGlobal: entidades, perfis e mapeamentos do AD são da gestão global.
func soGlobal(ctx context.Context) error {
	identity, _ := auth.IdentityFromContext(ctx)
	if !auth.CanGlobal(identity, auth.PermIAMManage) {
		return apperrors.Forbidden("só a administração global altera entidades, perfis e mapeamentos do AD")
	}
	return nil
}

// alcanca: a permissão de administração precisa cobrir a posição t.
func alcanca(ctx context.Context, permission auth.Permission, t auth.Target) error {
	identity, _ := auth.IdentityFromContext(ctx)
	if !auth.Can(identity, permission, t) {
		return apperrors.Forbidden("fora da sua área de administração (" + string(permission) + ")")
	}
	return nil
}

// alvo é a posição de um escopo organizacional.
func alvo(s domain.Scope) auth.Target {
	return auth.Target{EntidadeID: s.EntidadeID, UnidadeID: s.UnidadeID, DepartamentoID: s.DepartamentoID}
}

// posicaoUnidade é onde uma unidade "fica" para ser criada ou excluída: na
// mãe, ou na entidade quando é de primeiro nível.
func posicaoUnidade(u domain.Unidade) auth.Target {
	if u.ParentID != nil {
		return auth.InUnidade(*u.ParentID)
	}
	return auth.Target{EntidadeID: &u.EntidadeID}
}

// areaOf é a área em que identity exerce permission; nil = tudo.
func areaOf(identity auth.Identity, permission auth.Permission) *domain.Area {
	c := auth.CoverageOf(identity, permission)
	if c.All {
		return nil
	}
	return &domain.Area{Entidades: c.Entidades, Unidades: c.Unidades, Departamentos: c.Departamentos}
}

// concedivel: identity pode conceder (ou retirar) a permissão p a quem
// fica na posição t — tem p numa concessão global, ou numa que cobre t
// quando p vale com escopo (curingas contam: fora das permissões com
// escopo, uma concessão com escopo não vale mesmo).
func concedivel(identity auth.Identity, p string, t auth.Target) bool {
	if identity.HasRole(auth.RoleAdmin) {
		return true
	}
	curinga := p == "*" || strings.HasSuffix(p, ":*")
	for _, s := range identity.Scopes {
		if !auth.MatchPermission(s.Permissions, p) {
			continue
		}
		if s.Global() || (s.Covers(t) && (curinga || auth.IsScoped(auth.Permission(p)))) {
			return true
		}
	}
	return false
}

// guardPerfilAt: lotar (ou retirar a lotação) com um perfil na posição t
// exige poder conceder ali cada permissão dele.
func (s *Service) guardPerfilAt(ctx context.Context, tx pgx.Tx, perfilID uuid.UUID, t auth.Target, verbo string) error {
	p, err := s.repo.GetPerfil(ctx, tx, perfilID)
	if err != nil {
		return err
	}
	identity, _ := auth.IdentityFromContext(ctx)
	for _, perm := range p.Permissoes {
		if !concedivel(identity, perm, t) {
			return apperrors.Forbidden("você não pode " + verbo + " a permissão " + perm + " neste escopo, porque não a possui nele")
		}
	}
	return nil
}

// guardArea: com administração delegada, a conta precisa estar
// inteiramente na área — ao menos uma lotação, todas cobertas, e sem o
// papel de administrador da plataforma. Senão o delegado redefiniria a
// senha de alguém de fora (ou de um administrador) e entraria como ele.
func (s *Service) guardArea(ctx context.Context, db database.DBTX, u domain.User, permission auth.Permission) error {
	identity, _ := auth.IdentityFromContext(ctx)
	if auth.CanGlobal(identity, permission) {
		return nil
	}
	fora := apperrors.Forbidden("esta conta não está inteiramente na sua área de administração")
	if slices.Contains(u.Roles, auth.RoleAdmin) {
		return fora
	}
	grants, err := s.repo.UserGrants(ctx, db, u.ID)
	if err != nil {
		return err
	}
	if len(grants) == 0 {
		return fora
	}
	for _, g := range grants {
		if !auth.Can(identity, permission, alvo(g)) {
			return fora
		}
	}
	return nil
}

// guardGrant aplica "ninguém concede nem retira o que não tem" (A01): cada
// permissão que muda (adicionada ou removida) precisa estar coberta pelas
// permissões efetivas de quem altera — senão um detentor de iam:manage
// criaria um perfil "*" e se lotaria nele, ou tiraria o "*" de quem o tem.
func guardGrant(ctx context.Context, before, after []string) error {
	identity, _ := auth.IdentityFromContext(ctx)
	check := func(list, other []string, verb string) error {
		for _, p := range list {
			if slices.Contains(other, p) {
				continue
			}
			if !auth.HasPermission(identity, auth.Permission(p)) {
				return apperrors.Forbidden("você não pode " + verb + " a permissão " + p + ", que não possui")
			}
		}
		return nil
	}
	if err := check(after, before, "conceder"); err != nil {
		return err
	}
	return check(before, after, "retirar")
}

// guardTarget: só administra uma conta (edita, desativa, redefine a senha)
// quem cobre todas as permissões efetivas dela — senão quem tem apenas
// users:manage redefiniria a senha de um administrador e entraria como ele.
func (s *Service) guardTarget(ctx context.Context, db database.DBTX, targetID uuid.UUID) error {
	perms, err := s.repo.UserPermissions(ctx, db, targetID)
	if err != nil {
		return err
	}
	identity, _ := auth.IdentityFromContext(ctx)
	for _, p := range perms {
		if !auth.HasPermission(identity, auth.Permission(p)) {
			return apperrors.Forbidden("esta conta tem a permissão " + p + ", que você não possui; só quem a cobre pode administrá-la")
		}
	}
	return nil
}

// guardAdminRole impede escalada de privilégio (A01): só quem já detém
// acesso total ("*") concede ou retira o papel de administrador da
// plataforma (nexus-admin equivale a "*").
func guardAdminRole(ctx context.Context, before, after []string) error {
	if slices.Contains(before, auth.RoleAdmin) == slices.Contains(after, auth.RoleAdmin) {
		return nil
	}
	identity, _ := auth.IdentityFromContext(ctx)
	if !auth.HasPermission(identity, "*") {
		return apperrors.Forbidden("apenas administradores com acesso total podem conceder ou retirar o papel " + auth.RoleAdmin)
	}
	return nil
}

// MapError traduz erros de domínio em erros HTTP.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("registro não encontrado")
	case errors.Is(err, domain.ErrConflict):
		return apperrors.Conflict("já existe um registro com esses dados (slug, grupo ou vínculo duplicado)")
	case errors.Is(err, domain.ErrInUse):
		return apperrors.Conflict("registro em uso: remova antes a estrutura abaixo, as lotações, os mapeamentos do AD e o público-alvo de conteúdos que o referenciam")
	case errors.Is(err, domain.ErrSystemProfile):
		return apperrors.Conflict("perfis de sistema não podem ser removidos")
	case errors.Is(err, domain.ErrInvalidPermision):
		return apperrors.Validation(err.Error())
	case errors.Is(err, domain.ErrInvalidScope):
		return apperrors.Validation("escopo organizacional inconsistente (unidade/departamento não pertencem ao nível acima)")
	}
	return err
}

// tx roda fn numa transação e grava a entrada de auditoria no final.
func (s *Service) tx(ctx context.Context, invalidate bool, fn func(ctx context.Context, tx pgx.Tx) (audit.Entry, error)) error {
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		entry, err := fn(ctx, tx)
		if err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, entry)
	})
	if err != nil {
		return MapError(err)
	}
	if invalidate {
		s.invalidate(ctx)
	}
	return nil
}
