package domain

import (
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

func TestEvaluateInheritanceAndSubjects(t *testing.T) {
	me := uuid.New()
	dep := uuid.New()
	identity := auth.Identity{
		UserID: me,
		Groups: []string{"/Nexus/Juridico"},
		Scopes: []auth.Scope{{Perfil: "protocolo", DepartamentoID: &dep}},
	}
	other := uuid.New()

	cases := []struct {
		name  string
		chain []ChainLink
		want  Access
	}{
		{"sem ACL e sem dono: nada", []ChainLink{{Depth: 0, OwnerID: other}}, Access{}},
		{"dono de ancestral: total", []ChainLink{{Depth: 0, OwnerID: other}, {Depth: 1, OwnerID: me}}, Access{true, true, true}},
		{"grupo do AD herdado do pai: leitura", []ChainLink{
			{Depth: 0, OwnerID: other},
			{Depth: 1, OwnerID: other, ACL: []ACLEntry{{SubjectType: "ad_group", Subject: "juridico"}}},
		}, Access{Read: true}},
		{"departamento com escrita", []ChainLink{
			{Depth: 0, OwnerID: other, ACL: []ACLEntry{{SubjectType: "departamento", Subject: dep.String(), CanWrite: true}}},
		}, Access{Read: true, Write: true}},
		{"perfil", []ChainLink{{Depth: 0, OwnerID: other, ACL: []ACLEntry{{SubjectType: "perfil", Subject: "protocolo"}}}}, Access{Read: true}},
		{"grupo por substring NÃO casa", []ChainLink{{Depth: 0, OwnerID: other, ACL: []ACLEntry{{SubjectType: "ad_group", Subject: "jur"}}}}, Access{}},
		{"everyone", []ChainLink{{Depth: 0, OwnerID: other, ACL: []ACLEntry{{SubjectType: "everyone"}}}}, Access{Read: true}},
	}
	for _, c := range cases {
		if got := Evaluate(identity, c.chain); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	if got := Evaluate(auth.Identity{Scopes: []auth.Scope{{Permissions: []string{"files:manage"}}}}, []ChainLink{{OwnerID: other}}); !got.Manage {
		t.Error("files:manage deveria ter acesso total")
	}
}
