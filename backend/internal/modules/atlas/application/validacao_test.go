package application_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// rascunho importa um procedimento como rascunho e devolve a versão criada.
func (e *env) rascunho(s *application.Service, item application.ProcedimentoArquivo) domain.Workflow {
	e.t.Helper()
	ctx := context.Background()
	c := arquivo(item)
	r, err := s.ImportarRascunhos(ctx, gestor, c, application.HashCarga(c), true)
	if err != nil || !r.Aplicada || r.Totais[domain.ImportErro] > 0 {
		e.t.Fatalf("importar rascunho: %+v %v", r, err)
	}
	versoes, _, err := s.List(ctx, domain.Filter{CodigoProcessual: item.CodigoProcessual, IncluirInativos: true}, todas)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, v := range versoes {
		if v.Versao == r.Itens[0].Versao {
			w, err := s.Get(ctx, v.ID, true)
			if err != nil {
				e.t.Fatal(err)
			}
			return w
		}
	}
	e.t.Fatalf("rascunho não encontrado: %+v", versoes)
	return domain.Workflow{}
}

func TestValidacaoDosProcedimentos(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real().WithNotificacoes(notificacoes.NewService(e.pool, &hubFalso{}, logger))
	seguidor := dbtest.User(t, e.pool)

	// Rascunho de um código novo: inativo, fora da consulta pública.
	novoRasc := e.rascunho(s, itemArquivo(codigoNovo(), "Rascunho novo", ""))
	if novoRasc.Ativo || novoRasc.Situacao != domain.SituacaoRascunho {
		t.Fatalf("rascunho novo: %+v", novoRasc)
	}
	if _, err := s.Get(ctx, novoRasc.ID, false); codigo(err) != "NOT_FOUND" {
		t.Fatalf("rascunho na consulta pública: %v", err)
	}
	if _, err := s.SetAtivo(ctx, novoRasc.ID, true); codigo(err) != "VALIDATION_ERROR" {
		t.Fatalf("ativar rascunho: %v", err)
	}

	// Rascunho de nova versão: a versão em vigor continua publicada e ninguém é avisado.
	vigente := e.workflow(true)
	if _, err := s.Seguir(ctx, auth.Identity{Subject: "s", UserID: seguidor}, vigente.ID, true); err != nil {
		t.Fatal(err)
	}
	rasc := e.rascunho(s, itemArquivo(vigente.CodigoProcessual, "Revisão em validação", ""))
	if rasc.Versao != vigente.Versao+1 || rasc.Ativo {
		t.Fatalf("rascunho de nova versão: %+v", rasc)
	}
	if v, err := s.Get(ctx, vigente.ID, false); err != nil || !v.Ativo {
		t.Fatalf("a versão em vigor saiu do ar: %+v %v", v, err)
	}
	if e.recebeu(seguidor, "atlas:procedimento:"+rasc.ID.String()) {
		t.Fatal("rascunho avisou o seguidor")
	}
	if c, _ := s.Cobertura(ctx); c.Totais.Rascunhos < 2 {
		t.Fatalf("painel sem os rascunhos: %+v", c.Totais)
	}

	// Situação: rascunho ↔ em validação; a entrevista também leva a em validação.
	if w, err := s.MudarSituacao(ctx, rasc.ID, domain.SituacaoEmValidacao); err != nil || w.Situacao != domain.SituacaoEmValidacao {
		t.Fatalf("em validação: %+v %v", w, err)
	}
	if w, err := s.MudarSituacao(ctx, rasc.ID, domain.SituacaoRascunho); err != nil || w.Situacao != domain.SituacaoRascunho {
		t.Fatalf("de volta a rascunho: %+v %v", w, err)
	}
	ent, err := s.RegistrarValidacao(ctx, gestor, rasc.ID, domain.Validacao{RealizadaEm: time.Now().AddDate(0, 0, -1),
		Unidade: "Licitações", Participantes: "Equipe da CPL", Registro: "Etapas conferidas; falta o parecer jurídico.", Pendencias: "Parecer"})
	if err != nil || ent.CodigoProcessual != vigente.CodigoProcessual || ent.CreatedBy != "gestor" {
		t.Fatalf("entrevista: %+v %v", ent, err)
	}
	if w, _ := s.Get(ctx, rasc.ID, true); w.Situacao != domain.SituacaoEmValidacao {
		t.Fatalf("a entrevista não levou a em validação: %+v", w.Situacao)
	}
	// Entrevista de procedimento já em validação não muda a situação.
	if _, err := s.RegistrarValidacao(ctx, gestor, rasc.ID, domain.Validacao{RealizadaEm: time.Now(), Unidade: "Jurídico", Registro: "Parecer incluído."}); err != nil {
		t.Fatal(err)
	}
	if lista, err := s.Validacoes(ctx, vigente.ID); err != nil || len(lista) != 2 {
		t.Fatalf("entrevistas pelo código (de qualquer versão): %+v %v", lista, err)
	}

	// Homologar: publica, substitui a versão anterior e avisa.
	pub, err := s.Homologar(ctx, gestor, rasc.ID)
	if err != nil || !pub.Ativo || pub.Situacao != domain.SituacaoHomologado {
		t.Fatalf("homologar: %+v %v", pub, err)
	}
	if v, _ := s.Get(ctx, vigente.ID, true); v.Ativo {
		t.Fatal("a versão anterior continua ativa")
	}
	if !e.recebeu(seguidor, "atlas:procedimento:"+rasc.ID.String()) {
		t.Fatal("seguidor sem o aviso da homologação")
	}
	// Homologar um código novo (sem versão anterior) não avisa ninguém, mas publica.
	if w, err := s.Homologar(ctx, gestor, novoRasc.ID); err != nil || !w.Ativo {
		t.Fatalf("homologar código novo: %+v %v", w, err)
	}

	// Regras.
	revogar := e.rascunho(s, itemArquivo(codigoNovo(), "Série revogada na validação", ""))
	serieTeste := "2.0.02.00.94"
	if _, err := e.pool.Exec(ctx, `INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos,
		fase_interm_anos, destinacao_final) VALUES ($1, '2.0.02.00', 'Série de teste da homologação', 1, 2, 'ELIMINACAO')
		ON CONFLICT (codigo) DO UPDATE SET revogada_em = NULL`, serieTeste); err != nil {
		t.Fatal(err)
	}
	item := itemArquivo(codigoNovo(), "Série que será revogada", "")
	item.CodigoTTDD = serieTeste
	naRevogada := e.rascunho(s, item)
	if _, err := e.pool.Exec(ctx, `UPDATE atlas_classificacao_ttdd SET revogada_em = now() WHERE codigo = $1`, serieTeste); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"homologar de novo": {func() error { _, err := s.Homologar(ctx, gestor, pub.ID); return err }(), "VALIDATION_ERROR"},
		"série revogada":    {func() error { _, err := s.Homologar(ctx, gestor, naRevogada.ID); return err }(), "VALIDATION_ERROR"},
		"homologado não volta": {func() error {
			_, err := s.MudarSituacao(ctx, pub.ID, domain.SituacaoRascunho)
			return err
		}(), "VALIDATION_ERROR"},
		"situação inválida": {func() error { _, err := s.MudarSituacao(ctx, revogar.ID, domain.SituacaoHomologado); return err }(), "VALIDATION_ERROR"},
		"entrevista futura": {func() error {
			_, err := s.RegistrarValidacao(ctx, gestor, revogar.ID, domain.Validacao{RealizadaEm: time.Now().AddDate(0, 0, 2), Unidade: "u", Registro: "r"})
			return err
		}(), "VALIDATION_ERROR"},
		"entrevista de inexistente": {func() error {
			_, err := s.RegistrarValidacao(ctx, gestor, uuid.New(), domain.Validacao{RealizadaEm: time.Now(), Unidade: "u", Registro: "r"})
			return err
		}(), "NOT_FOUND"},
		"lista de inexistente": {func() error { _, err := s.Validacoes(ctx, uuid.New()); return err }(), "NOT_FOUND"},
	} {
		if codigo(c.err) != c.want {
			t.Errorf("%s: %v, quero %s", name, c.err, c.want)
		}
	}
}

func TestValidacaoHTTP(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	s := e.real()
	r := func(svc *application.Service) *chi.Mux {
		m := chi.NewRouter()
		m.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), gestor)))
			})
		})
		transport.NewHandlers(svc, logger, 100).RegisterRoutes(m, unlimited{})
		return m
	}
	do := func(m *chi.Mux, method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		m.ServeHTTP(rec, req)
		return rec
	}
	ok := r(s)
	// Importação como rascunho pela API.
	c := arquivo(itemArquivo(codigoNovo(), "Rascunho pela API", ""))
	conteudo, _ := json.Marshal(c)
	corpo := `{"conteudo":` + string(conteudo) + `,"rascunho":true`
	if rec := do(ok, http.MethodPost, "/atlas/admin/workflows/importacao/aplicar", corpo+`,"hash":"`+application.HashCarga(c)+`"}`); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"aplicada":true`) {
		t.Fatalf("importar rascunho: %d %s", rec.Code, rec.Body.String())
	}
	// Cadastro pela tela como rascunho (ADR 028).
	novoCorpo := `{"codigo_processual":"` + codigoNovo() + `","titulo":"t","objetivo":"o","publico_alvo":"p","nivel_acesso":"PUBLICO","codigo_ttdd":"` + serieVigente + `","rascunho":true,"etapas":[{"ordem":1,"unidade_administrativa":"A","nome_setor":"S","atribuicoes_setor":"x"}]}`
	if rec := do(ok, http.MethodPost, "/atlas/admin/workflows", novoCorpo); rec.Code != http.StatusCreated ||
		!strings.Contains(rec.Body.String(), `"situacao":"RASCUNHO"`) || !strings.Contains(rec.Body.String(), `"ativo":false`) {
		t.Fatalf("cadastro como rascunho: %d %s", rec.Code, rec.Body.String())
	}
	w := e.rascunho(s, itemArquivo(codigoNovo(), "Outro rascunho", ""))
	base := "/atlas/admin/workflows/" + w.ID.String()
	hoje := time.Now().Format("2006-01-02")
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{http.MethodPost, base + "/situacao", `{"situacao":"EM_VALIDACAO"}`, http.StatusOK},
		{http.MethodPost, base + "/situacao", `{"situacao":"HOMOLOGADO"}`, http.StatusUnprocessableEntity},
		{http.MethodPost, base + "/validacoes", `{"realizada_em":"` + hoje + `","unidade":"Licitações","registro":"ok"}`, http.StatusCreated},
		{http.MethodPost, base + "/validacoes", `{"realizada_em":"04/10/2026","unidade":"x","registro":"r"}`, http.StatusUnprocessableEntity},
		{http.MethodGet, base + "/validacoes", "", http.StatusOK},
		{http.MethodPost, base + "/homologar", "", http.StatusOK},
		{http.MethodPost, base + "/homologar", "", http.StatusUnprocessableEntity},
		{http.MethodPost, "/atlas/admin/workflows/x/situacao", `{}`, http.StatusBadRequest},
		{http.MethodPost, "/atlas/admin/workflows/x/homologar", "", http.StatusBadRequest},
		{http.MethodGet, "/atlas/admin/workflows/x/validacoes", "", http.StatusBadRequest},
		{http.MethodPost, "/atlas/admin/workflows/x/validacoes", `{}`, http.StatusBadRequest},
		{http.MethodPost, base + "/situacao", `{`, http.StatusBadRequest},
		{http.MethodPost, base + "/validacoes", `{`, http.StatusBadRequest},
	} {
		if rec := do(ok, tc.method, tc.path, tc.body); rec.Code != tc.want {
			t.Errorf("%s %s %s: %d, quero %d — %s", tc.method, tc.path, tc.body, rec.Code, tc.want, rec.Body.String())
		}
	}
	outro := e.rascunho(s, itemArquivo(codigoNovo(), "Para o banco fora", ""))
	b2 := "/atlas/admin/workflows/" + outro.ID.String()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, b2 + "/situacao", `{"situacao":"EM_VALIDACAO"}`},
		{http.MethodPost, b2 + "/homologar", ""},
		{http.MethodGet, b2 + "/validacoes", ""},
		{http.MethodPost, b2 + "/validacoes", `{"realizada_em":"` + hoje + `","unidade":"u","registro":"r"}`},
	} {
		down := r(e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 1}))
		if rec := do(down, tc.method, tc.path, tc.body); rec.Code != http.StatusInternalServerError {
			t.Errorf("%s %s com o banco fora: %d", tc.method, tc.path, rec.Code)
		}
	}
}
