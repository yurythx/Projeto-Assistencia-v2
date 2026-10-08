package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/application"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/iam/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

type env struct {
	t     *testing.T
	pool  *pgxpool.Pool
	admin auth.Identity
	ctx   context.Context
}

func newEnv(t *testing.T) *env {
	pool := dbtest.Pool(t)
	admin := auth.Identity{UserID: dbtest.User(t, pool), Username: "admin", Permissions: []string{"*"}, Scopes: []auth.Scope{{Permissions: []string{"*"}}}}
	return &env{t: t, pool: pool, admin: admin, ctx: auth.WithIdentity(context.Background(), admin)}
}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo, nil)
}

func (e *env) real() *application.Service { return e.svc(infrastructure.NewRepository()) }

func (e *env) must(err error) {
	e.t.Helper()
	if err != nil {
		e.t.Fatal(err)
	}
}

func slug() string { return "t" + strings.ReplaceAll(uuid.NewString()[:12], "-", "") }

// estrutura cria entidade > unidade > departamento.
func (e *env) estrutura() (domain.Entidade, domain.Unidade, domain.Departamento) {
	s := e.real()
	ent, err := s.SaveEntidade(e.ctx, domain.Entidade{Nome: "Órgão", Slug: slug(), Ativo: true})
	e.must(err)
	un, err := s.SaveUnidade(e.ctx, domain.Unidade{EntidadeID: ent.ID, Nome: "Unidade", Slug: slug(), Ativo: true})
	e.must(err)
	dep, err := s.SaveDepartamento(e.ctx, domain.Departamento{UnidadeID: un.ID, Nome: "Dep", Slug: slug(), Ativo: true})
	e.must(err)
	return ent, un, dep
}

func (e *env) perfil(perms ...string) domain.Perfil {
	p, err := e.real().SavePerfil(e.ctx, domain.Perfil{Nome: "Perfil", Slug: slug(), Permissoes: perms, Ativo: true})
	e.must(err)
	return p
}

// Cada chamada ao repositório de cada caso de uso é derrubada e depois
// envenenada: o caso de uso sempre devolve erro, nunca sucesso parcial.
func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := newEnv(t)
	ctx := e.ctx
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"OrgTree": func() func(*application.Service) error {
			e.estrutura()
			return func(s *application.Service) error { _, err := s.OrgTree(ctx); return err }
		},
		"ListEntidades": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ListEntidades(ctx); return err }
		},
		"SaveEntidade": func() func(*application.Service) error {
			ent, _, _ := e.estrutura()
			return func(s *application.Service) error { ent.Nome = "Novo"; _, err := s.SaveEntidade(ctx, ent); return err }
		},
		"CreateEntidade": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.SaveEntidade(ctx, domain.Entidade{Nome: "Nova " + slug()})
				return err
			}
		},
		"DeleteEntidade": func() func(*application.Service) error {
			ent, err := e.real().SaveEntidade(ctx, domain.Entidade{Nome: "Solta", Slug: slug()})
			e.must(err)
			return func(s *application.Service) error { return s.DeleteEntidade(ctx, ent.ID) }
		},
		"ListUnidades": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ListUnidades(ctx, nil); return err }
		},
		"SaveUnidade": func() func(*application.Service) error {
			ent, mae, _ := e.estrutura()
			filha, err := e.real().SaveUnidade(ctx, domain.Unidade{EntidadeID: ent.ID, Nome: "Filha", Slug: slug()})
			e.must(err)
			return func(s *application.Service) error {
				filha.ParentID = &mae.ID
				_, err := s.SaveUnidade(ctx, filha)
				return err
			}
		},
		"DeleteUnidade": func() func(*application.Service) error {
			ent, _, _ := e.estrutura()
			u, err := e.real().SaveUnidade(ctx, domain.Unidade{EntidadeID: ent.ID, Nome: "Solta", Slug: slug()})
			e.must(err)
			return func(s *application.Service) error { return s.DeleteUnidade(ctx, u.ID) }
		},
		"ListDepartamentos": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ListDepartamentos(ctx, nil); return err }
		},
		"SaveDepartamento": func() func(*application.Service) error {
			_, _, dep := e.estrutura()
			return func(s *application.Service) error {
				dep.Nome = "Outro"
				_, err := s.SaveDepartamento(ctx, dep)
				return err
			}
		},
		"DeleteDepartamento": func() func(*application.Service) error {
			_, _, dep := e.estrutura()
			return func(s *application.Service) error { return s.DeleteDepartamento(ctx, dep.ID) }
		},
		"ListPerfis": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ListPerfis(ctx); return err }
		},
		"SavePerfil": func() func(*application.Service) error {
			p := e.perfil("blog:manage")
			return func(s *application.Service) error { p.Ativo = false; _, err := s.SavePerfil(ctx, p); return err }
		},
		"DeletePerfil": func() func(*application.Service) error {
			p := e.perfil()
			return func(s *application.Service) error { return s.DeletePerfil(ctx, p.ID) }
		},
		"ListMappings": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ListMappings(ctx); return err }
		},
		"CreateMapping": func() func(*application.Service) error {
			_, un, _ := e.estrutura()
			p := e.perfil("blog:manage")
			return func(s *application.Service) error {
				_, err := s.CreateMapping(ctx, domain.ADMapping{ADGroup: "GRP_" + slug(), PerfilID: p.ID, Scope: domain.Scope{UnidadeID: &un.ID}})
				return err
			}
		},
		"DeleteMapping": func() func(*application.Service) error {
			p := e.perfil("blog:manage")
			id, err := e.real().CreateMapping(ctx, domain.ADMapping{ADGroup: "GRP_" + slug(), PerfilID: p.ID})
			e.must(err)
			return func(s *application.Service) error { return s.DeleteMapping(ctx, id) }
		},
		"ListUsers": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, _, err := s.ListUsers(ctx, domain.UserFilter{}, pagination.New(1, 5, 5))
				return err
			}
		},
		"GetUser": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.GetUser(ctx, e.admin.UserID); return err }
		},
		"UpdateUser": func() func(*application.Service) error {
			u := dbtest.User(t, e.pool)
			return func(s *application.Service) error {
				_, err := s.UpdateUser(ctx, u, application.UpdateUserInput{DisplayName: "X", Active: true})
				return err
			}
		},
		"CreateLocalUser": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.CreateLocalUser(ctx, application.CreateLocalUserInput{Username: slug(), Email: slug() + "@t.gov.br", Password: "Senha-Forte-123!"})
				return err
			}
		},
		"ResetPassword": func() func(*application.Service) error {
			u, err := e.real().CreateLocalUser(ctx, application.CreateLocalUserInput{Username: slug(), Email: slug() + "@t.gov.br", Password: "Senha-Forte-123!"})
			e.must(err)
			return func(s *application.Service) error { return s.ResetPassword(ctx, u.ID, "Outra-Senha-Forte-9") }
		},
		"Unlock": func() func(*application.Service) error {
			u := dbtest.User(t, e.pool)
			return func(s *application.Service) error { return s.Unlock(ctx, u) }
		},
		"ListLotacoes": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ListLotacoes(ctx, e.admin.UserID); return err }
		},
		"CreateLotacao": func() func(*application.Service) error {
			_, _, dep := e.estrutura()
			u := dbtest.User(t, e.pool)
			p := e.perfil("blog:manage")
			return func(s *application.Service) error {
				_, err := s.CreateLotacao(ctx, domain.Lotacao{UserID: u, PerfilID: p.ID, Principal: true, Scope: domain.Scope{DepartamentoID: &dep.ID}})
				return err
			}
		},
		"DeleteLotacao": func() func(*application.Service) error {
			u := dbtest.User(t, e.pool)
			p := e.perfil("blog:manage")
			id, err := e.real().CreateLotacao(ctx, domain.Lotacao{UserID: u, PerfilID: p.ID})
			e.must(err)
			return func(s *application.Service) error { return s.DeleteLotacao(ctx, u, id) }
		},
	}
	for name, o := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := o()(e.svc(probe)); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			for _, poison := range []bool{false, true} {
				r := &faultRepo{inner: infrastructure.NewRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := o()(e.svc(r)); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
}

func status(err error) int {
	if appErr, ok := apperrors.As(err); ok {
		return appErr.Status
	}
	return 0
}

// Regras que não dependem de falha do banco.
func TestServiceRules(t *testing.T) {
	e := newEnv(t)
	s := e.real()
	ctx := e.ctx

	// Invalidação do cache: chamada após mudanças de permissão.
	calls := 0
	withCache := application.NewService(e.pool, infrastructure.NewRepository(), func(context.Context) { calls++ })
	if _, err := withCache.SavePerfil(ctx, domain.Perfil{Nome: "Cache", Slug: slug()}); err != nil || calls != 1 {
		t.Fatalf("salvar perfil invalida o cache de permissões: %v (%d)", err, calls)
	}

	// Permissões: formato, normalização e deduplicação.
	p, err := s.SavePerfil(ctx, domain.Perfil{Nome: "Norm", Slug: slug(), Permissoes: []string{" Blog:Manage ", "blog:manage", "", "wiki:*"}})
	if err != nil || strings.Join(p.Permissoes, ",") != "blog:manage,wiki:*" {
		t.Fatalf("permissões normalizadas: %+v %v", p.Permissoes, err)
	}
	if _, err := s.SavePerfil(ctx, domain.Perfil{Nome: "Ruim", Permissoes: []string{"Blog Manage"}}); status(err) != http.StatusUnprocessableEntity {
		t.Errorf("permissão fora do formato recurso:ação: %v", err)
	}
	// O perfil administrador de sistema mantém "*" e o slug.
	var adm domain.Perfil
	perfis, _ := s.ListPerfis(ctx)
	for _, x := range perfis {
		if x.Slug == "administrador" {
			adm = x
		}
	}
	adm.Permissoes, adm.Slug, adm.Ativo = []string{"blog:manage"}, "outro", false
	if got, err := s.SavePerfil(ctx, adm); err != nil || got.Slug != "administrador" || strings.Join(got.Permissoes, ",") != "*" || !got.Ativo {
		t.Fatalf("administrador de sistema preservado: %+v %v", got, err)
	}
	if err := s.DeletePerfil(ctx, adm.ID); status(err) != http.StatusConflict {
		t.Errorf("perfil de sistema não é excluído: %v", err)
	}

	// Mapeamento exige grupo; conta não desativa a si mesma; senha fraca.
	if _, err := s.CreateMapping(ctx, domain.ADMapping{ADGroup: "  "}); status(err) != http.StatusUnprocessableEntity {
		t.Errorf("grupo do AD obrigatório: %v", err)
	}
	if _, err := s.UpdateUser(ctx, e.admin.UserID, application.UpdateUserInput{Active: false}); status(err) != http.StatusConflict {
		t.Errorf("ninguém desativa a própria conta: %v", err)
	}
	for _, bad := range []string{"curta1!", "somenteminusculas", "SOMENTEMAIUSCULAS1", "abcdefghijk1"} {
		if err := application.ValidatePasswordStrength(bad); err == nil {
			t.Errorf("senha fraca aceita: %q", bad)
		}
	}
	if err := application.ValidatePasswordStrength("Tres-Classes-1"); err != nil {
		t.Errorf("senha forte recusada: %v", err)
	}
	for _, in := range []application.CreateLocalUserInput{
		{Username: "a b", Password: "Senha-Forte-123!"},
		{Username: slug(), Password: "fraca"},
	} {
		if _, err := s.CreateLocalUser(ctx, in); status(err) != http.StatusUnprocessableEntity {
			t.Errorf("conta local inválida %+v: %v", in, err)
		}
	}
	if err := s.ResetPassword(ctx, uuid.New(), "fraca"); status(err) != http.StatusUnprocessableEntity {
		t.Errorf("redefinição com senha fraca: %v", err)
	}

	// Desbloqueio: sem Redis configurado basta o banco; falha no Redis é 503.
	u := dbtest.User(t, e.pool)
	if err := s.Unlock(ctx, u); err != nil {
		t.Fatalf("desbloqueio sem lockout distribuído: %v", err)
	}
	var cleared string
	ok := s.WithLoginLockoutReset(func(_ context.Context, username string) error { cleared = username; return nil })
	if err := ok.Unlock(ctx, u); err != nil || cleared == "" {
		t.Fatalf("desbloqueio limpa o lockout do usuário: %v %q", err, cleared)
	}
	broken := e.real().WithLoginLockoutReset(func(context.Context, string) error { return errors.New("redis fora") })
	if err := broken.Unlock(ctx, u); status(err) != http.StatusServiceUnavailable {
		t.Errorf("Redis fora no desbloqueio: esperado 503, veio %v", err)
	}
	if err := s.Unlock(ctx, uuid.New()); status(err) != http.StatusNotFound {
		t.Errorf("desbloquear conta inexistente: %v", err)
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Errorf("erro desconhecido passa adiante: %v", err)
	}
}

// Os handlers traduzem falhas do serviço em 500 sem vazar a causa.
func TestHandlersReportServiceFailures(t *testing.T) {
	e := newEnv(t)
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), func() []transport.PermissionGroup { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), e.admin)))
		})
	})
	h.RegisterRoutes(r)
	id := e.admin.UserID.String()
	for _, path := range []string{"/iam/org-tree", "/iam/entidades", "/iam/unidades", "/iam/departamentos", "/iam/perfis",
		"/iam/ad-mappings", "/users", "/users/" + id, "/users/" + id + "/lotacoes"} {
		down.calls = 0
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("GET %s com o banco fora: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

// Detalhe do usuário: a conta carrega, mas as lotações falham.
func TestUserDetailFailsWhenLotacoesFail(t *testing.T) {
	e := newEnv(t)
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 2}
	h := transport.NewHandlers(e.svc(down), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/"+e.admin.UserID.String(), nil)
	r.ServeHTTP(rec, req.WithContext(auth.WithIdentity(req.Context(), e.admin)))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("lotações indisponíveis: %d %s", rec.Code, rec.Body.String())
	}
}

// /me sem identidade é 401; com listas vazias devolve arrays (nunca null).
func TestMeEndpoint(t *testing.T) {
	e := newEnv(t)
	h := transport.NewHandlers(e.real(), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	rec := httptest.NewRecorder()
	h.Me(rec, httptest.NewRequest(http.MethodGet, "/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/me sem identidade: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	h.Me(rec, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{UserID: uuid.New(), Subject: "s"})))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"roles":[]`) || !strings.Contains(body, `"scopes":[]`) || strings.Contains(body, `":null,"`) {
		t.Errorf("/me com listas vazias devolve arrays: %d %s", rec.Code, body)
	}
}

// Entidade recém-criada, sem unidades nem departamentos, aparece na árvore
// com listas vazias (nunca null no JSON).
func TestOrgTreeListsEmptyBranches(t *testing.T) {
	e := newEnv(t)
	s := e.real()
	ent, err := s.SaveEntidade(e.ctx, domain.Entidade{Nome: "Órgão vazio", Slug: slug(), Ativo: true})
	e.must(err)
	un, err := s.SaveUnidade(e.ctx, domain.Unidade{EntidadeID: ent.ID, Nome: "Unidade vazia", Slug: slug(), Ativo: true})
	e.must(err)
	solo, err := s.SaveEntidade(e.ctx, domain.Entidade{Nome: "Órgão sem unidades", Slug: slug(), Ativo: true})
	e.must(err)

	tree, err := s.OrgTree(e.ctx)
	e.must(err)
	found := 0
	for _, n := range tree {
		switch n.Entidade.ID {
		case solo.ID:
			found++
			if n.Unidades == nil || len(n.Unidades) != 0 {
				t.Fatalf("entidade sem unidades: %+v", n.Unidades)
			}
		case ent.ID:
			found++
			if len(n.Unidades) != 1 || n.Unidades[0].Unidade.ID != un.ID || n.Unidades[0].Departamentos == nil {
				t.Fatalf("unidade sem departamentos: %+v", n.Unidades)
			}
		}
	}
	if found != 2 {
		t.Fatalf("entidades na árvore: %d", found)
	}
}
