package auth

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func ptr(id uuid.UUID) *uuid.UUID { return &id }

// Estrutura: entidade E com a unidade-mãe U (subunidade F) e a unidade V;
// departamento D na unidade U. Outra entidade X com a unidade W.
var (
	idE, idX       = uuid.New(), uuid.New()
	idU, idF, idV  = uuid.New(), uuid.New(), uuid.New()
	idW, idD, idD2 = uuid.New(), uuid.New(), uuid.New()
)

func grant(perms []string, e, u, d *uuid.UUID, unidades ...uuid.UUID) Scope {
	return Scope{Permissions: perms, EntidadeID: e, UnidadeID: u, DepartamentoID: d, Unidades: unidades}
}

func TestScopeCovers(t *testing.T) {
	global := grant([]string{"blog:manage"}, nil, nil, nil)
	naEntidade := grant([]string{"blog:manage"}, ptr(idE), nil, nil, idU, idF, idV)
	naUnidade := grant([]string{"blog:manage"}, ptr(idE), ptr(idU), nil, idU, idF)
	semExpansao := grant([]string{"blog:manage"}, ptr(idE), ptr(idU), nil)
	noDepto := grant([]string{"blog:manage"}, ptr(idE), ptr(idU), ptr(idD))

	cases := []struct {
		name string
		s    Scope
		t    Target
		want bool
	}{
		{"global cobre institucional", global, Target{}, true},
		{"global cobre qualquer unidade", global, InUnidade(idW), true},
		{"entidade cobre as unidades dela", naEntidade, InUnidade(idV), true},
		{"entidade cobre subunidade", naEntidade, InUnidade(idF), true},
		{"entidade não cobre outra entidade", naEntidade, InUnidade(idW), false},
		{"entidade cobre recurso posicionado só na entidade", naEntidade, Target{EntidadeID: ptr(idE)}, true},
		{"entidade não cobre institucional", naEntidade, Target{}, false},
		{"unidade cobre a si", naUnidade, InUnidade(idU), true},
		{"unidade cobre a subunidade (herança)", naUnidade, InUnidade(idF), true},
		{"unidade não cobre a irmã", naUnidade, InUnidade(idV), false},
		{"unidade não cobre recurso só da entidade", naUnidade, Target{EntidadeID: ptr(idE)}, false},
		{"unidade cobre departamento dela", naUnidade, Target{UnidadeID: ptr(idU), DepartamentoID: ptr(idD)}, true},
		{"unidade sem expansão cobre a si", semExpansao, InUnidade(idU), true},
		{"unidade sem expansão não cobre a filha", semExpansao, InUnidade(idF), false},
		{"departamento cobre a si", noDepto, Target{UnidadeID: ptr(idU), DepartamentoID: ptr(idD)}, true},
		{"departamento não cobre outro departamento", noDepto, Target{UnidadeID: ptr(idU), DepartamentoID: ptr(idD2)}, false},
		{"departamento não cobre a unidade inteira", noDepto, InUnidade(idU), false},
	}
	for _, c := range cases {
		if got := c.s.Covers(c.t); got != c.want {
			t.Errorf("%s: Covers = %v, esperado %v", c.name, got, c.want)
		}
	}
}

func TestCanAndCanGlobal(t *testing.T) {
	gestorU := Identity{Scopes: []Scope{
		grant([]string{"blog:*"}, ptr(idE), ptr(idU), nil, idU, idF),
		grant([]string{"monitoring:read"}, ptr(idE), ptr(idU), nil, idU, idF), // plataforma com escopo: não vale
	}}
	if !Can(gestorU, PermBlogManage, InUnidade(idF)) {
		t.Error("curinga blog:* na unidade cobre a subunidade")
	}
	if Can(gestorU, PermBlogManage, InUnidade(idV)) || Can(gestorU, PermBlogManage, Target{}) {
		t.Error("gestor da unidade não gerencia a irmã nem o institucional")
	}
	if Can(gestorU, PermMonitoringRead, InUnidade(idU)) || CanGlobal(gestorU, PermMonitoringRead) || CanGlobal(gestorU, PermBlogManage) {
		t.Error("permissão de plataforma com escopo não vale; nada aqui é global")
	}
	global := Identity{Scopes: []Scope{grant([]string{"monitoring:read", "blog:manage"}, nil, nil, nil)}}
	if !Can(global, PermMonitoringRead, Target{}) || !CanGlobal(global, PermMonitoringRead) || !Can(global, PermBlogManage, InUnidade(idW)) {
		t.Error("concessão global vale em todo lugar, inclusive plataforma")
	}
	admin := Identity{Roles: []string{RoleAdmin}}
	if !Can(admin, PermBlogManage, InUnidade(idW)) || !CanGlobal(admin, PermModulesManage) {
		t.Error("nexus-admin equivale a * global")
	}
	if Can(Identity{}, PermBlogManage, Target{}) {
		t.Error("sem concessão, nada")
	}
}

func TestCoverageOf(t *testing.T) {
	id := Identity{Scopes: []Scope{
		grant([]string{"tramite:manage"}, ptr(idE), ptr(idU), nil, idU, idF),
		grant([]string{"tramite:manage"}, ptr(idX), nil, nil, idW),
		grant([]string{"tramite:manage"}, ptr(idE), ptr(idV), ptr(idD2)),
		grant([]string{"blog:manage"}, ptr(idE), ptr(idV), nil, idV),
	}}
	c := CoverageOf(id, PermTramiteManage)
	if c.All || !slices.Equal(c.Unidades, []uuid.UUID{idU, idF, idW}) || !slices.Equal(c.Entidades, []uuid.UUID{idX}) || !slices.Equal(c.Departamentos, []uuid.UUID{idD2}) {
		t.Errorf("cobertura: %+v", c)
	}
	if !CoverageOf(Identity{Roles: []string{RoleAdmin}}, PermTramiteManage).All {
		t.Error("admin cobre tudo")
	}
	if c := CoverageOf(id, PermMonitoringRead); c.All || len(c.Unidades)+len(c.Entidades)+len(c.Departamentos) != 0 {
		t.Errorf("permissão de plataforma sem concessão global não cobre nada: %+v", c)
	}
}

func TestEffectivePermissions(t *testing.T) {
	got := EffectivePermissions([]Scope{
		grant([]string{"monitoring:read", "users:*"}, nil, nil, nil),                      // global: conta inteira
		grant([]string{"*"}, ptr(idE), ptr(idU), nil),                                     // escopo: só as com escopo
		grant([]string{"tramite:*", "egress:manage"}, ptr(idE), nil, nil),                 // curinga de recurso
		grant([]string{"modules:manage", "catalog:manage"}, ptr(idE), ptr(idU), ptr(idD)), // plataforma cai
	})
	for _, want := range []string{"monitoring:read", "users:*", "tramite:create", "tramite:route", "tramite:manage", "blog:manage", "catalog:manage", "directory:manage"} {
		if !slices.Contains(got, want) {
			t.Errorf("faltou %s em %v", want, got)
		}
	}
	for _, not := range []string{"*", "egress:manage", "modules:manage", "tramite:*"} {
		if slices.Contains(got, not) {
			t.Errorf("%s não vale com escopo, mas veio em %v", not, got)
		}
	}
	if !slices.IsSorted(got) {
		t.Errorf("ordenado: %v", got)
	}
	if !IsScoped(PermWikiManage) || !IsScoped(PermFilesManage) || IsScoped(PermModulesManage) {
		t.Error("classificação das permissões")
	}
}

func TestLotadoEm(t *testing.T) {
	naMae := Identity{Scopes: []Scope{grant([]string{"wiki:read"}, ptr(idE), ptr(idU), nil, idU, idF)}}
	if !LotadoEm(naMae, idU) || !LotadoEm(naMae, idF) || LotadoEm(naMae, idV) {
		t.Error("lotação na unidade-mãe vale nela e nas subunidades, não na irmã")
	}
	noDepto := Identity{Scopes: []Scope{grant(nil, ptr(idE), ptr(idU), ptr(idD), idU, idF)}}
	if !LotadoEm(noDepto, idU) || LotadoEm(noDepto, idF) {
		t.Error("lotação no departamento vale na unidade dele, não nas subunidades")
	}
	global := Identity{Scopes: []Scope{grant([]string{"*"}, nil, nil, nil)}}
	if LotadoEm(global, idU) || LotadoEm(Identity{}, idU) {
		t.Error("concessão global não é lotação; sem concessão, nada")
	}
}
