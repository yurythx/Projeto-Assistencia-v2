package auth

import "strings"

// Roles de realm reconhecidos diretamente no token. O Keycloak é a fonte
// da verdade para roles; o controle fino é feito por Perfis (permissões
// "recurso:ação") resolvidos pelo IAM a partir das lotações e do
// mapeamento de grupos do AD. nexus-admin é o único role com efeito
// próprio (equivale à permissão "*"), para que um administrador nunca
// fique trancado fora da plataforma por um mapeamento mal configurado.
const (
	RoleAdmin RoleName = "nexus-admin"
	RoleUser  RoleName = "nexus-user"
)

type RoleName = string

// Permission é uma capacidade granular no formato "recurso:ação",
// verificada por RequirePermission ANTES da camada de aplicação (A01).
type Permission string

// Catálogo de permissões da plataforma. Cada plugin declara as suas no
// próprio Manifest (ver internal/platform/kernel); estas constantes
// existem para que as checagens no código não espalhem strings literais.
const (
	PermUsersRead      Permission = "users:read"
	PermUsersManage    Permission = "users:manage"
	PermIAMManage      Permission = "iam:manage"
	PermAuditRead      Permission = "audit:read"
	PermAuditVerify    Permission = "audit:verify"
	PermModulesManage  Permission = "modules:manage"
	PermKeycloakManage Permission = "keycloak:manage"
	PermBrandingManage Permission = "branding:manage"
	PermMonitoringRead Permission = "monitoring:read"
	// PermMonitoringManage: ações operacionais do painel (reprocessar o outbox).
	PermMonitoringManage Permission = "monitoring:manage"

	PermMercurioManage  Permission = "mercurio:manage"
	PermEgressManage    Permission = "egress:manage"
	PermBlogManage      Permission = "blog:manage"
	PermCatalogManage   Permission = "catalog:manage"
	PermContactRead     Permission = "contact:read"
	PermContactManage   Permission = "contact:manage"
	PermDirectoryManage Permission = "directory:manage"
	PermCalendarManage  Permission = "calendar:manage"
	PermFilesManage     Permission = "files:manage"
	PermWikiManage      Permission = "wiki:manage"
	PermSignumManage    Permission = "signum:manage"
	PermTramiteCreate   Permission = "tramite:create"
	PermTramiteRoute    Permission = "tramite:route"
	PermTramiteManage   Permission = "tramite:manage"
	PermExampleManage   Permission = "example:manage"
	PermAtlasRead       Permission = "atlas:read"
	PermAtlasManage     Permission = "atlas:manage"
	// PermIAManage: conexões de inteligência artificial (Configurações).
	PermIAManage Permission = "ia:manage"
)

// HasPermission reporta se identity possui permission, considerando os
// curingas "*" (tudo) e "recurso:*" (toda ação do recurso). O role
// nexus-admin equivale a "*".
func HasPermission(identity Identity, permission Permission) bool {
	if identity.HasRole(RoleAdmin) {
		return true
	}
	return MatchPermission(identity.Permissions, string(permission))
}

// MatchPermission aplica a regra de curingas sobre uma lista de
// permissões concedidas.
func MatchPermission(granted []string, want string) bool {
	resource, _, _ := strings.Cut(want, ":")
	for _, g := range granted {
		switch {
		case g == "*":
			return true
		case g == want:
			return true
		case strings.HasSuffix(g, ":*") && strings.TrimSuffix(g, ":*") == resource:
			return true
		}
	}
	return false
}
