package auth

import "testing"

func TestHasPermission_AdminRoleHasEverything(t *testing.T) {
	admin := Identity{Roles: []string{RoleAdmin}}
	for _, p := range []Permission{PermUsersManage, PermAuditRead, PermModulesManage, PermKeycloakManage, PermTramiteManage} {
		if !HasPermission(admin, p) {
			t.Errorf("nexus-admin deveria ter %s", p)
		}
	}
}

func TestHasPermission_Wildcards(t *testing.T) {
	cases := []struct {
		granted []string
		want    Permission
		ok      bool
	}{
		{[]string{"*"}, PermSignumManage, true},
		{[]string{"tramite:*"}, PermTramiteRoute, true},
		{[]string{"tramite:*"}, PermSignumManage, false},
		{[]string{"blog:manage"}, PermBlogManage, true},
		{[]string{"blog:manage"}, PermCatalogManage, false},
		{[]string{"blog"}, PermBlogManage, false},
		{nil, PermUsersRead, false},
	}
	for _, c := range cases {
		if got := HasPermission(Identity{Permissions: c.granted}, c.want); got != c.ok {
			t.Errorf("granted=%v want=%s: got %v, esperado %v", c.granted, c.want, got, c.ok)
		}
	}
}

func TestHasPermission_UserRoleGrantsNothingByItself(t *testing.T) {
	user := Identity{Roles: []string{RoleUser}}
	for _, p := range []Permission{PermUsersRead, PermAuditRead, PermModulesManage} {
		if HasPermission(user, p) {
			t.Errorf("nexus-user não deveria ter %s sem perfil atribuído", p)
		}
	}
}
