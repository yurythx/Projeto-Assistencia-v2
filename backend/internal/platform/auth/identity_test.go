package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestActorUUID(t *testing.T) {
	t.Run("sem identidade no contexto", func(t *testing.T) {
		id, subject, ok := ActorUUID(context.Background())
		if id != nil || subject != "" || ok {
			t.Errorf("esperava (nil, \"\", false), got (%v, %q, %v)", id, subject, ok)
		}
	})

	t.Run("id interno resolvido pelo IAM tem precedência", func(t *testing.T) {
		internal := uuid.New()
		ctx := WithIdentity(context.Background(), Identity{Subject: "kc-sub", Source: SourceKeycloak, UserID: internal})
		id, subject, ok := ActorUUID(ctx)
		if !ok || id == nil || *id != internal || subject != "kc-sub" {
			t.Errorf("esperava (%v, kc-sub, true), got (%v, %q, %v)", internal, id, subject, ok)
		}
	})

	t.Run("conta local: subject já é o id interno", func(t *testing.T) {
		want := uuid.New()
		ctx := WithIdentity(context.Background(), Identity{Subject: want.String(), Source: SourceLocal})
		id, _, ok := ActorUUID(ctx)
		if !ok || id == nil || *id != want {
			t.Errorf("esperava %v, got (%v, %v)", want, id, ok)
		}
	})

	t.Run("sub UUID do Keycloak NÃO é o id interno", func(t *testing.T) {
		ctx := WithIdentity(context.Background(), Identity{Subject: uuid.NewString(), Source: SourceKeycloak})
		if id, _, ok := ActorUUID(ctx); ok || id != nil {
			t.Errorf("sub do Keycloak sem resolução do IAM não pode virar actor_id: id=%v ok=%v", id, ok)
		}
	})
}

func TestHasGroupIsExactNotSubstring(t *testing.T) {
	id := Identity{Groups: []string{"/Nexus/TI", "Grupo_Admin_Temporario"}}
	if !id.HasGroup("ti") || !id.HasGroup("/Nexus/TI") {
		t.Error("grupo em formato de caminho deveria casar pelo nome")
	}
	if id.HasGroup("admin") {
		t.Error("HasGroup não pode casar por substring (\"admin\" em \"Grupo_Admin_Temporario\")")
	}
}

func TestIsAdmin(t *testing.T) {
	if (Identity{Groups: []string{"Administradores"}}).IsAdmin() {
		t.Error("nome de grupo com 'admin' não pode conceder administração")
	}
	if !(Identity{Roles: []string{RoleAdmin}}).IsAdmin() {
		t.Error("role nexus-admin deveria ser admin")
	}
	if !(Identity{Permissions: []string{"*"}}).IsAdmin() {
		t.Error("permissão curinga deveria ser admin")
	}
}
