package application_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

// Procedimentos por secretaria: a contagem por órgão (rascunhos só para a
// gestão) e os filtros da lista (prefixo da TTDD, situação, última versão).
func TestProcedimentosPorSecretaria(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()
	pub := e.workflow(true)
	rasc := e.rascunho(s, itemArquivo(codigoNovo(), "Rascunho da secretaria", ""))

	orgao := func(gestao bool) domain.ProcedimentosOrgao {
		t.Helper()
		out, err := s.ProcedimentosPorOrgao(ctx, gestao)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range out {
			if o.Prefixo == "2.0" {
				return o
			}
		}
		t.Fatalf("órgão 2.0 fora da lista: %+v", out)
		return domain.ProcedimentosOrgao{}
	}
	g, p := orgao(true), orgao(false)
	if g.Publicados < 1 || g.Rascunhos < 1 || g.Nome == "" || p.Publicados != g.Publicados || p.Rascunhos != 0 || p.EmValidacao != 0 {
		t.Fatalf("contagem: gestão %+v, público %+v", g, p)
	}

	ids := func(f domain.Filter) map[uuid.UUID]bool {
		t.Helper()
		l, _, err := s.List(ctx, f, todas)
		if err != nil {
			t.Fatal(err)
		}
		out := map[uuid.UUID]bool{}
		for _, w := range l {
			out[w.ID] = true
		}
		return out
	}
	gestao := domain.Filter{Query: rasc.CodigoProcessual, PrefixoTTDD: "2.0", IncluirInativos: true, Situacao: domain.SituacaoRascunho, Ultima: true}
	if got := ids(gestao); !got[rasc.ID] {
		t.Fatal("rascunho fora da lista da secretaria")
	}
	if got := ids(domain.Filter{Query: rasc.CodigoProcessual, PrefixoTTDD: serieVigente, IncluirInativos: true}); !got[rasc.ID] {
		t.Fatal("prefixo igual à série deveria casar")
	}
	if got := ids(domain.Filter{Query: rasc.CodigoProcessual, PrefixoTTDD: "3.0", IncluirInativos: true}); got[rasc.ID] {
		t.Fatal("rascunho na secretaria errada")
	}
	if got := ids(domain.Filter{Query: rasc.CodigoProcessual, IncluirInativos: true, Situacao: domain.SituacaoHomologado}); got[rasc.ID] {
		t.Fatal("filtro de situação ignorado")
	}
	// Público: só a versão em vigor, e situação/última não se aplicam.
	if got := ids(domain.Filter{Query: pub.CodigoProcessual, PrefixoTTDD: "2.0", Situacao: domain.SituacaoRascunho, Ultima: true}); !got[pub.ID] {
		t.Fatal("procedimento publicado fora da consulta pública da secretaria")
	}
	if got := ids(domain.Filter{Query: rasc.CodigoProcessual, PrefixoTTDD: "2.0"}); got[rasc.ID] {
		t.Fatal("rascunho na consulta pública")
	}

	// Rascunho corrigido (nova versão): a anterior sai da "última versão" e
	// não conta duas vezes.
	antes, painel := orgao(true).Rascunhos, cobertura(t, s)
	nova := e.rascunho(s, itemArquivo(rasc.CodigoProcessual, "Rascunho corrigido na entrevista", ""))
	if got := ids(gestao); got[rasc.ID] || !got[nova.ID] {
		t.Fatalf("última versão: %v", got)
	}
	if depois := orgao(true).Rascunhos; depois != antes {
		t.Fatalf("rascunho contado duas vezes: %d → %d", antes, depois)
	}
	if depois := cobertura(t, s); depois != painel {
		t.Fatalf("painel contou o rascunho duas vezes: %d → %d", painel, depois)
	}
}

func cobertura(t *testing.T, s interface {
	Cobertura(context.Context) (domain.Cobertura, error)
}) int {
	t.Helper()
	c, err := s.Cobertura(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return c.Totais.Rascunhos
}

var comRascunho = regexp.MustCompile(`"rascunhos":[1-9]`)

// Rotas: o resumo público esconde os rascunhos; filtros inválidos dão 400.
func TestSecretariasHTTP(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	e.rascunho(e.real(), itemArquivo(codigoNovo(), "Rascunho para a rota", ""))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), gestor)))
		})
	})
	h := transport.NewHandlers(e.real(), logger, 100)
	h.RegisterPublicRoutes(r)
	h.RegisterRoutes(r, unlimited{})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	if rec := get("/atlas/admin/workflows/secretarias"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"prefixo":"2.0"`) ||
		!comRascunho.MatchString(rec.Body.String()) {
		t.Fatalf("resumo da gestão: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get("/atlas/workflows/secretarias"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"prefixo":"2.0"`) ||
		comRascunho.MatchString(rec.Body.String()) {
		t.Fatalf("resumo público: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get("/atlas/admin/workflows?prefixo_ttdd=2.0&situacao=rascunho&ultima=true"); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"situacao":"RASCUNHO"`) {
		t.Fatalf("lista da secretaria: %d %s", rec.Code, rec.Body.String())
	}
	for _, path := range []string{"/atlas/organograma", "/atlas/admin/organograma"} {
		if rec := get(path); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"subfuncoes":[{`) {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	for _, q := range []string{"prefixo_ttdd=abc", "situacao=PUBLICADO"} {
		if rec := get("/atlas/admin/workflows?" + q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", q, rec.Code)
		}
	}
}

// Organograma: a subfunção da série vigente conta o publicado e o rascunho
// (só a gestão vê o rascunho); os totais sobem para a função e o órgão.
func TestOrganograma(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()
	e.workflow(true)
	e.rascunho(s, itemArquivo(codigoNovo(), "Rascunho do organograma", ""))

	sub := func(gestao bool) (domain.OrgaoOrganograma, domain.SubfuncaoOrganograma) {
		t.Helper()
		out, err := s.Organograma(ctx, gestao)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range out {
			for _, f := range o.Funcoes {
				for _, sf := range f.Subfuncoes {
					if sf.Codigo == "2.0.02.00" {
						return o, sf
					}
				}
			}
		}
		t.Fatalf("subfunção 2.0.02.00 fora do organograma")
		return domain.OrgaoOrganograma{}, domain.SubfuncaoOrganograma{}
	}
	og, g := sub(true)
	if g.Series < 1 || g.Publicados < 1 || g.Rascunhos < 1 || g.Lacunas != 0 || g.Nome == "" {
		t.Fatalf("gestão: %+v", g)
	}
	soma := 0
	for _, f := range og.Funcoes {
		soma += f.Series
	}
	if og.Prefixo != "2.0" || og.Series != soma || og.Rascunhos < g.Rascunhos {
		t.Fatalf("totais do órgão: %+v (soma das funções %d)", og.ContagemOrganograma, soma)
	}
	if _, p := sub(false); p.Rascunhos != 0 || p.EmValidacao != 0 || p.Publicados != g.Publicados {
		t.Fatalf("público: %+v", p)
	}
}
