// Package auth valida os access tokens emitidos pelo Keycloak dedicado do
// Nexus (OIDC, JWKS em cache local com rotação automática) e os tokens do
// fallback local RS256, e fornece os middlewares chi de autenticação e de
// autorização granular por escopo ("recurso:ação" — A01).
package auth

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// Source identifica qual verificador aceitou o token que produziu uma
// Identity: um Subject do Keycloak é o "sub" externo (chave de
// provisionamento por keycloak_subject), enquanto um Subject local já É o
// id interno da linha em "users".
type Source string

const (
	SourceKeycloak Source = "keycloak"
	SourceLocal    Source = "local"
)

// Scope é uma lotação efetiva do usuário: um perfil aplicado a um escopo
// organizacional (Entidade > Unidade > Departamento). Campos nulos = o
// perfil vale para todo o nível acima.
type Scope struct {
	Perfil         string     `json:"perfil"`
	EntidadeID     *uuid.UUID `json:"entidade_id,omitempty"`
	UnidadeID      *uuid.UUID `json:"unidade_id,omitempty"`
	DepartamentoID *uuid.UUID `json:"departamento_id,omitempty"`
	// Origem: "manual" (lotação cadastrada) ou "ad" (mapeamento de grupo).
	Origem string `json:"origem"`
	// Permissions são as permissões do perfil desta concessão (ADR 013:
	// valem no escopo dela — ver Can).
	Permissions []string `json:"permissions"`
	// Unidades é a árvore coberta, expandida pelo resolvedor: a unidade e as
	// subunidades dela ou, na entidade, todas as unidades da entidade.
	Unidades []uuid.UUID `json:"-"`
	// Acima é a unidade da concessão e as de cima (ADR 014: quem está numa
	// unidade pertence ao público-alvo de qualquer unidade acima dela).
	Acima []uuid.UUID `json:"-"`
}

// Identity é o chamador autenticado. Nunca carrega o token bruto — só o
// que autorização, auditoria e handlers precisam.
type Identity struct {
	Subject  string   // claim "sub" — id do Keycloak OU id interno (local)
	Username string   // "preferred_username"
	Email    string   // "email"
	Name     string   // "name" (nome de exibição vindo do AD)
	Roles    []string // roles de realm + client (Keycloak) ou users.roles (local)
	Groups   []string // grupos do Active Directory (via mapper de grupo do Keycloak)
	Source   Source

	// Preenchidos pelo middleware do IAM (internal/platform/iam) depois
	// da verificação do token.
	UserID      uuid.UUID // id interno em "users" (uuid.Nil antes da resolução)
	Permissions []string  // união das permissões de todos os perfis efetivos
	Scopes      []Scope   // lotações efetivas (manuais + mapeadas do AD)
}

// HasRole reporta se a identidade recebeu o role informado.
func (i Identity) HasRole(role string) bool {
	return slices.Contains(i.Roles, role)
}

// HasGroup reporta se a identidade pertence ao grupo do AD informado
// (comparação exata, case-insensitive, aceitando o formato de caminho do
// Keycloak "/Pai/Grupo" — nunca por substring, que concederia acesso a
// "Grupo_Admin_Temporario" por conter "admin").
func (i Identity) HasGroup(group string) bool {
	want := NormalizeGroup(group)
	for _, g := range i.Groups {
		if NormalizeGroup(g) == want {
			return true
		}
	}
	return false
}

// IsAdmin reporta se a identidade tem o role de administrador da
// plataforma ou a permissão curinga "*".
func (i Identity) IsAdmin() bool {
	return i.HasRole(RoleAdmin) || slices.Contains(i.Permissions, "*")
}

// NormalizeGroup reduz um nome de grupo à forma comparável: sem o caminho
// hierárquico do Keycloak ("/Nexus/TI" -> "ti") e em minúsculas.
func NormalizeGroup(g string) string {
	g = strings.TrimSpace(g)
	if idx := strings.LastIndex(g, "/"); idx >= 0 {
		g = g[idx+1:]
	}
	return strings.ToLower(g)
}

type ctxKey string

const identityCtxKey ctxKey = "auth.identity"

// WithIdentity retorna um context carregando a identidade autenticada.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey, id)
}

// IdentityFromContext extrai a identidade gravada por RequireAuthentication.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityCtxKey).(Identity)
	return id, ok
}

// ActorUUID resolve o autor de uma ação para auditoria/colunas `*_by`:
// o id interno (resolvido pelo IAM) quando disponível; senão o subject,
// se ele for um UUID (conta local). ok = false quando não há id interno.
func ActorUUID(ctx context.Context) (id *uuid.UUID, subject string, ok bool) {
	identity, found := IdentityFromContext(ctx)
	if !found {
		return nil, "", false
	}
	if identity.UserID != uuid.Nil {
		uid := identity.UserID
		return &uid, identity.Subject, true
	}
	parsed, err := uuid.Parse(identity.Subject)
	if err != nil || identity.Source != SourceLocal {
		return nil, identity.Subject, false
	}
	return &parsed, identity.Subject, true
}
