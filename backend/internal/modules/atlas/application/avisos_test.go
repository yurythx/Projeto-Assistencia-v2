package application_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/application"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
)

type hubFalso struct {
	mu      sync.Mutex
	topicos []string
}

func (h *hubFalso) Publish(_ context.Context, topic, _ string, _ any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.topicos = append(h.topicos, topic)
	return nil
}

// recebeu diz se o usuário tem o aviso com a chave.
func (e *env) recebeu(usuario uuid.UUID, chave string) bool {
	e.t.Helper()
	var ok bool
	if err := e.pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM notificacoes WHERE user_id = $1 AND chave = $2)`,
		usuario, chave).Scan(&ok); err != nil {
		e.t.Fatal(err)
	}
	return ok
}

// lotar põe o usuário numa unidade de sigla dada: manualmente ou pelo grupo do AD.
func (e *env) lotar(usuario uuid.UUID, sigla string, porAD bool) {
	e.t.Helper()
	ctx := context.Background()
	un := dbtest.Unidade(e.t, e.pool)
	var perfil uuid.UUID
	sqls := []string{`UPDATE unidades SET sigla = $2 WHERE id = $1`}
	if err := e.pool.QueryRow(ctx, `SELECT id FROM perfis LIMIT 1`).Scan(&perfil); err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, sqls[0], un, sigla); err != nil {
		e.t.Fatal(err)
	}
	if !porAD {
		if _, err := e.pool.Exec(ctx, `INSERT INTO user_scopes (user_id, perfil_id, unidade_id) VALUES ($1, $2, $3)`, usuario, perfil, un); err != nil {
			e.t.Fatal(err)
		}
		return
	}
	grupo := "GRP-" + strings.ToUpper(uuid.NewString()[:8])
	if _, err := e.pool.Exec(ctx, `UPDATE users SET groups = ARRAY[$2] WHERE id = $1`, usuario, strings.ToLower(grupo)); err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `INSERT INTO ad_group_mappings (ad_group, perfil_id, unidade_id) VALUES ($1, $2, $3)`, grupo, perfil, un); err != nil {
		e.t.Fatal(err)
	}
}

func TestAvisosDeNovaVersao(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	hub := &hubFalso{}
	s := e.real().WithNotificacoes(notificacoes.NewService(e.pool, hub, logger))
	seguidor, lotado, peloAD, autorUser := dbtest.User(t, e.pool), dbtest.User(t, e.pool), dbtest.User(t, e.pool), dbtest.User(t, e.pool)
	sigla := "U" + strings.ToUpper(uuid.NewString()[:6])
	e.lotar(lotado, sigla, false)
	e.lotar(peloAD, sigla+"/SUB", true) // a sigla inteira da etapa ("SEC/SUB")
	base := novo()
	base.Etapas[1].UnidadeAdministrativa = sigla + "/SUB"
	w, err := s.Create(ctx, gestor, base)
	if err != nil {
		t.Fatal(err)
	}
	quem := func(u uuid.UUID) auth.Identity { return auth.Identity{Subject: "s", UserID: u} }

	// Seguir (pelo código: vale para as próximas versões).
	if sg, err := s.Seguir(ctx, quem(seguidor), w.ID, true); err != nil || !sg.Seguindo || sg.CodigoProcessual != w.CodigoProcessual {
		t.Fatalf("seguir: %+v %v", sg, err)
	}
	if _, err := s.Seguir(ctx, quem(autorUser), w.ID, true); err != nil {
		t.Fatal(err)
	}
	if sg, err := s.Seguindo(ctx, quem(seguidor), w.ID); err != nil || !sg.Seguindo {
		t.Fatalf("seguindo: %+v %v", sg, err)
	}
	if _, err := s.Seguir(ctx, auth.Identity{}, w.ID, true); codigo(err) != "VALIDATION_ERROR" {
		t.Fatalf("sem usuário local: %v", err)
	}
	if _, err := s.Seguindo(ctx, quem(seguidor), uuid.New()); codigo(err) != "NOT_FOUND" {
		t.Fatalf("procedimento inexistente: %v", err)
	}

	// Nova versão: seguidor e as unidades do fluxo (manual e AD); o autor não.
	nova := novo()
	nova.Etapas[0].UnidadeAdministrativa = sigla
	v2, err := s.NovaVersao(ctx, quem(autorUser), w.ID, nova)
	if err != nil {
		t.Fatal(err)
	}
	chave := "atlas:procedimento:" + v2.ID.String()
	if !e.recebeu(seguidor, chave) || !e.recebeu(lotado, chave) || !e.recebeu(peloAD, chave) || e.recebeu(autorUser, chave) {
		t.Fatalf("destinatários: seguidor %v lotado %v AD %v autor %v", e.recebeu(seguidor, chave), e.recebeu(lotado, chave),
			e.recebeu(peloAD, chave), e.recebeu(autorUser, chave))
	}
	entregue := false
	for _, tp := range hub.topicos {
		entregue = entregue || tp == "user:"+seguidor.String()
	}
	if !entregue {
		t.Fatalf("sem entrega em tempo real: %v", hub.topicos)
	}

	// Modelo usado pelo procedimento: quem segue é avisado da versão nova.
	m := e.modelo()
	if _, err := s.LigarModelo(ctx, v2.ID, v2.Etapas[0].Documentos[0].ID, &m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NovaVersaoModelo(ctx, gestor, m.ID, docx()); err != nil {
		t.Fatal(err)
	}
	if !e.recebeu(seguidor, "atlas:modelo:"+m.ID.String()+":2") {
		t.Fatal("seguidor sem o aviso do modelo")
	}

	// Importação aplicada também avisa.
	imp := itemArquivo(w.CodigoProcessual, "Revisto pela importação", "")
	conteudo := arquivo(imp)
	r, err := s.ImportarProcedimentos(ctx, gestor, conteudo, application.HashCarga(conteudo), true)
	if err != nil || !r.Aplicada {
		t.Fatalf("importação: %+v %v", r, err)
	}
	atual, _, _ := s.List(ctx, domain.Filter{CodigoProcessual: w.CodigoProcessual}, todas)
	if len(atual) != 1 || !e.recebeu(seguidor, "atlas:procedimento:"+atual[0].ID.String()) {
		t.Fatalf("importação sem aviso: %+v", atual)
	}

	if sg, err := s.Seguir(ctx, quem(seguidor), atual[0].ID, false); err != nil || sg.Seguindo {
		t.Fatalf("deixar de seguir: %+v %v", sg, err)
	}
}

func TestSeguirHTTP(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	w := e.workflow(true)
	eu := dbtest.User(t, e.pool)
	router := func(down bool) *chi.Mux {
		svc := e.real()
		if down {
			svc = e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 2})
		}
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(rw, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Subject: "s", UserID: eu})))
			})
		})
		transport.NewHandlers(svc, logger, 100).RegisterRoutes(r, unlimited{})
		return r
	}
	do := func(r *chi.Mux, method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}
	base := "/atlas/workflows/" + w.ID.String() + "/seguir"
	r := router(false)
	for _, c := range []struct {
		method, path, want string
		code               int
	}{
		{http.MethodPut, base, `"seguindo":true`, http.StatusOK},
		{http.MethodGet, base, `"seguindo":true`, http.StatusOK},
		{http.MethodDelete, base, `"seguindo":false`, http.StatusOK},
		{http.MethodGet, "/atlas/workflows/x/seguir", "", http.StatusBadRequest},
	} {
		rec := do(r, c.method, c.path)
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s %s: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		if rec := do(router(true), m, base); rec.Code != http.StatusInternalServerError {
			t.Errorf("%s com o banco fora: %d", m, rec.Code)
		}
	}
}
