package auth

// accessTokenClaims espelha o subconjunto das claims de um access token do
// Keycloak que a plataforma usa. O Keycloak coloca os roles de realm em
// "realm_access.roles" e os roles por client em
// "resource_access.<client_id>.roles" — os dois são mesclados em
// Identity.Roles. "groups" vem do mapper "Group Membership" do client,
// alimentado pela federação LDAP/LDAPS com o Active Directory.
type accessTokenClaims struct {
	Subject           string                   `json:"sub"`
	PreferredUsername string                   `json:"preferred_username"`
	Email             string                   `json:"email"`
	Name              string                   `json:"name"`
	RealmAccess       roleContainer            `json:"realm_access"`
	ResourceAccess    map[string]roleContainer `json:"resource_access"`
	Groups            []string                 `json:"groups"`
}

type roleContainer struct {
	Roles []string `json:"roles"`
}

// toIdentity mescla os roles de realm com os roles concedidos para
// clientID numa única Identity, sem duplicatas.
func (c accessTokenClaims) toIdentity(clientID string) Identity {
	seen := make(map[string]struct{}, len(c.RealmAccess.Roles))
	roles := make([]string, 0, len(c.RealmAccess.Roles))

	add := func(rs []string) {
		for _, r := range rs {
			if _, ok := seen[r]; ok {
				continue
			}
			seen[r] = struct{}{}
			roles = append(roles, r)
		}
	}

	add(c.RealmAccess.Roles)
	if clientRoles, ok := c.ResourceAccess[clientID]; ok {
		add(clientRoles.Roles)
	}

	return Identity{
		Subject:  c.Subject,
		Username: c.PreferredUsername,
		Email:    c.Email,
		Name:     c.Name,
		Roles:    roles,
		Groups:   c.Groups,
		Source:   SourceKeycloak,
	}
}
