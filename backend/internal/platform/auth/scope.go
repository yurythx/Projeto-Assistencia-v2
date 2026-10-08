package auth

import (
	"slices"
	"sort"

	"github.com/google/uuid"
)

// Permissão com escopo e herança (ADR 013).
//
// Cada Scope da identidade é uma concessão: um perfil (com as permissões
// dele) aplicado a um escopo organizacional. A permissão vale ONDE foi
// concedida:
//
//   - sem escopo (global): em toda a plataforma, inclusive nos recursos
//     institucionais (sem dono);
//   - entidade E: nos recursos de E e de todas as unidades de E;
//   - unidade U: nos recursos de U e das subunidades de U;
//   - departamento D: nos recursos de D.
//
// Só as permissões de ScopedPermissions — as dos módulos cujos recursos
// têm posição na organização — valem com escopo. As demais (plataforma,
// e as de módulos ainda sem dono organizacional) só valem com concessão
// global.

// ScopedPermissions são as permissões que valem no escopo em que foram
// concedidas (o módulo decide sobre cada recurso com Can).
var ScopedPermissions = []Permission{
	PermTramiteCreate, PermTramiteRoute, PermTramiteManage,
	PermCatalogManage, PermBlogManage, PermWikiManage,
	PermCalendarManage, PermMercurioManage, PermDirectoryManage,
	PermContactRead, PermContactManage, PermAuditRead, PermFilesManage,
	PermIAMManage, PermUsersRead, PermUsersManage,
}

// IsScoped reporta se a permissão (concreta, sem curinga) vale com escopo.
func IsScoped(permission Permission) bool {
	return slices.Contains(ScopedPermissions, permission)
}

// Target é a posição de um recurso na organização. Todo nível conhecido
// deve vir preenchido (o departamento com a unidade dele). Target vazio =
// recurso institucional: só concessão global o cobre.
type Target struct {
	EntidadeID     *uuid.UUID
	UnidadeID      *uuid.UUID
	DepartamentoID *uuid.UUID
}

// InUnidade é o alvo de um recurso posicionado numa unidade.
func InUnidade(id uuid.UUID) Target { return Target{UnidadeID: &id} }

// Global reporta se a concessão não tem escopo (vale na plataforma toda).
func (s Scope) Global() bool {
	return s.EntidadeID == nil && s.UnidadeID == nil && s.DepartamentoID == nil
}

// Covers reporta se a concessão s alcança o recurso na posição t.
func (s Scope) Covers(t Target) bool {
	switch {
	case s.Global():
		return true
	case s.DepartamentoID != nil:
		return t.DepartamentoID != nil && *t.DepartamentoID == *s.DepartamentoID
	case t.UnidadeID != nil && slices.Contains(s.coveredUnidades(), *t.UnidadeID):
		return true
	default:
		// concessão na entidade alcança recurso posicionado só na entidade
		return s.UnidadeID == nil && t.EntidadeID != nil && *t.EntidadeID == *s.EntidadeID
	}
}

// coveredUnidades: a árvore expandida pelo resolvedor (a própria unidade e
// as subunidades; na entidade, todas as unidades dela) ou, sem expansão, a
// própria unidade.
func (s Scope) coveredUnidades() []uuid.UUID {
	if len(s.Unidades) > 0 || s.UnidadeID == nil {
		return s.Unidades
	}
	return []uuid.UUID{*s.UnidadeID}
}

// LotadoEm reporta se identity tem alguma concessão (de qualquer perfil)
// na unidade — nela, num departamento dela ou numa unidade acima dela.
// Concessão global não conta: é "lotação" na plataforma, não na unidade.
func LotadoEm(identity Identity, unidade uuid.UUID) bool {
	for _, s := range identity.Scopes {
		if s.UnidadeID != nil && *s.UnidadeID == unidade {
			return true
		}
		if !s.Global() && s.DepartamentoID == nil && s.Covers(InUnidade(unidade)) {
			return true
		}
	}
	return false
}

// Can reporta se identity tem permission sobre o recurso na posição t.
// Permissão que não vale com escopo exige concessão global.
func Can(identity Identity, permission Permission, t Target) bool {
	if identity.HasRole(RoleAdmin) {
		return true
	}
	if !IsScoped(permission) {
		return CanGlobal(identity, permission)
	}
	for _, s := range identity.Scopes {
		if MatchPermission(s.Permissions, string(permission)) && s.Covers(t) {
			return true
		}
	}
	return false
}

// CanGlobal reporta se identity tem permission por concessão sem escopo
// (o papel nexus-admin equivale a "*" global).
func CanGlobal(identity Identity, permission Permission) bool {
	if identity.HasRole(RoleAdmin) {
		return true
	}
	for _, s := range identity.Scopes {
		if s.Global() && MatchPermission(s.Permissions, string(permission)) {
			return true
		}
	}
	return false
}

// Coverage é onde identity tem uma permissão — para filtrar listagens no
// SQL (All = em toda a plataforma).
type Coverage struct {
	All           bool
	Entidades     []uuid.UUID
	Unidades      []uuid.UUID
	Departamentos []uuid.UUID
}

// CoverageOf devolve onde identity tem permission.
func CoverageOf(identity Identity, permission Permission) Coverage {
	if CanGlobal(identity, permission) {
		return Coverage{All: true}
	}
	var c Coverage
	if !IsScoped(permission) {
		return c
	}
	for _, s := range identity.Scopes {
		if !MatchPermission(s.Permissions, string(permission)) {
			continue
		}
		switch {
		case s.DepartamentoID != nil:
			c.Departamentos = append(c.Departamentos, *s.DepartamentoID)
		default:
			c.Unidades = append(c.Unidades, s.coveredUnidades()...)
			if s.UnidadeID == nil && s.EntidadeID != nil {
				c.Entidades = append(c.Entidades, *s.EntidadeID)
			}
		}
	}
	return c
}

// EffectivePermissions é o que as concessões permitem fazer em ALGUM lugar
// (Identity.Permissions: menus, RequirePermission e a regra de não conceder
// o que não se tem). Concessão global conta inteira; concessão com escopo
// só nas permissões que valem com escopo — curingas expandidos para elas.
func EffectivePermissions(scopes []Scope) []string {
	set := map[string]struct{}{}
	for _, s := range scopes {
		for _, g := range s.Permissions {
			if s.Global() {
				set[g] = struct{}{}
				continue
			}
			for _, p := range ScopedPermissions {
				if MatchPermission([]string{g}, string(p)) {
					set[string(p)] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
