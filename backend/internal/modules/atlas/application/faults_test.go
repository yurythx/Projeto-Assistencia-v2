package application_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/application"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

var (
	gestor = auth.Identity{Subject: "gestor", Permissions: []string{"*"}, Scopes: []auth.Scope{{Perfil: "administrador", Permissions: []string{"*"}}}}
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
)

type env struct {
	t     *testing.T
	pool  *pgxpool.Pool
	store *gravador
}

func (e *env) svc(repo domain.Repository) *application.Service {
	if e.store == nil {
		e.store = novoGravador()
	}
	return application.NewService(e.pool, repo, outbox.NewWriter("test"), nil, logger).WithStorage(e.store, bucket)
}

func (e *env) real() *application.Service { return e.svc(infrastructure.NewRepository()) }

func novo() domain.Workflow {
	return domain.Workflow{
		CodigoProcessual: "TST.FALHA." + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", "")),
		Titulo:           "Procedimento de teste", Objetivo: "Objetivo", PublicoAlvo: "Servidores",
		NivelAcesso: domain.NivelPublico, CodigoTTDD: "2.0.02.00.07",
		Etapas: []domain.Etapa{
			{Ordem: 1, UnidadeAdministrativa: "A", NomeSetor: "Setor A", AtribuicoesSetor: "Instruir",
				Documentos: []domain.EtapaDocumento{{NomeDocumento: "Peça", Formato: domain.FormatoNatoDigital, TipoAssinatura: domain.AssinaturaIndividual}},
				Transicoes: []domain.EtapaTransicao{{DestinoOrdem: 2, CondicaoTransicao: "ok"}}},
			{Ordem: 2, UnidadeAdministrativa: "B", NomeSetor: "Setor B", AtribuicoesSetor: "Decidir"},
		},
	}
}

func (e *env) workflow(ativo bool) domain.Workflow {
	e.t.Helper()
	w, err := e.real().Create(context.Background(), gestor, novo())
	if err != nil {
		e.t.Fatal(err)
	}
	if !ativo {
		if w, err = e.real().SetAtivo(context.Background(), w.ID, false); err != nil {
			e.t.Fatal(err)
		}
	}
	return w
}

// cargaTeste é uma TTDD de um órgão fictício (98.0): aplicá-la não toca
// nas séries dos outros órgãos.
const cargaTeste = `{"orgaos":{"98.0":{"nome":"Órgão de teste","edicao":"1","data":"2026-01-01","versao":"I"}},
	"funcoes":{"98.0.01":"Função"},"subfuncoes":{"98.0.01.00":{"nome":"Subfunção","recomendacao":""}},
	"itens":[{"codigo":"98.0.01.00.01","subfuncao":"98.0.01.00","descritor":"Série de teste","corrente_anos":1,
		"corrente_condicao":"","intermediaria_anos":2,"intermediaria_condicao":"","destinacao":"ELIMINACAO","observacoes":""}]}`

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	p := pagination.New(1, 5, 5)
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"ListTTDD": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, _, err := s.ListTTDD(ctx, domain.FiltroTTDD{Codigo: "2.0"}, p)
				return err
			}
		},
		"ExportarTTDD": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ExportarTTDD(ctx, domain.FiltroTTDD{}); return err }
		},
		"EstruturaTTDD": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.EstruturaTTDD(ctx); return err }
		},
		"GetTTDD": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.GetTTDD(ctx, "2.0.02.00.07"); return err }
		},
		"List": func() func(*application.Service) error {
			return func(s *application.Service) error { _, _, err := s.List(ctx, domain.Filter{}, p); return err }
		},
		"Get": func() func(*application.Service) error {
			w := e.workflow(true)
			return func(s *application.Service) error { _, err := s.Get(ctx, w.ID, false); return err }
		},
		"Search": func() func(*application.Service) error {
			return func(s *application.Service) error { _, _, err := s.Search(ctx, "pregão", 5); return err }
		},
		"Create": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Create(ctx, gestor, novo()); return err }
		},
		"HistoricoTTDD": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.HistoricoTTDD(ctx, "2.0.02.00.07"); return err }
		},
		"NovaVersao": func() func(*application.Service) error {
			w := e.workflow(true)
			return func(s *application.Service) error { _, err := s.NovaVersao(ctx, gestor, w.ID, novo()); return err }
		},
		"AplicarCarga": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.ImpactoTTDD(ctx, application.FormatoJSON, cargaTeste, application.HashCarga(cargaTeste), true)
				return err
			}
		},
		"Desativar": func() func(*application.Service) error {
			w := e.workflow(true)
			return func(s *application.Service) error { _, err := s.SetAtivo(ctx, w.ID, false); return err }
		},
		"Ativar": func() func(*application.Service) error {
			w := e.workflow(false)
			return func(s *application.Service) error { _, err := s.SetAtivo(ctx, w.ID, true); return err }
		},
		"ListModelos": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ListModelos(ctx, true); return err }
		},
		"GetModelo": func() func(*application.Service) error {
			m := e.modelo()
			return func(s *application.Service) error { _, err := s.GetModelo(ctx, m.ID); return err }
		},
		"ArquivoModelo": func() func(*application.Service) error {
			m := e.modelo()
			return func(s *application.Service) error {
				_, rc, err := s.ArquivoModelo(ctx, m.ID, 0)
				if err == nil {
					rc.Close()
				}
				return err
			}
		},
		"CriarModelo": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.CriarModelo(ctx, gestor, modeloNovo(), docx())
				return err
			}
		},
		"NovaVersaoModelo": func() func(*application.Service) error {
			m := e.modelo()
			return func(s *application.Service) error {
				_, err := s.NovaVersaoModelo(ctx, gestor, m.ID, docx())
				return err
			}
		},
		"AlterarModelo": func() func(*application.Service) error {
			m := e.modelo()
			return func(s *application.Service) error {
				_, err := s.AlterarModelo(ctx, gestor, domain.Modelo{ID: m.ID, Nome: m.Nome, Ativo: false})
				return err
			}
		},
		"SimularImportacao": func() func(*application.Service) error {
			igual, outro := e.workflow(true), e.workflow(true)
			c := arquivo(itemArquivo(codigoNovo(), "Novo", ""), itemArquivo(igual.CodigoProcessual, "Procedimento de teste", ""),
				itemArquivo(outro.CodigoProcessual, "Revisto", ""))
			return func(s *application.Service) error {
				_, err := s.ImportarProcedimentos(ctx, gestor, c, "", false)
				return err
			}
		},
		"AplicarImportacao": func() func(*application.Service) error {
			outro := e.workflow(true)
			c := arquivo(itemArquivo(codigoNovo(), "Novo", ""), itemArquivo(outro.CodigoProcessual, "Revisto", ""))
			return func(s *application.Service) error {
				_, err := s.ImportarProcedimentos(ctx, gestor, c, application.HashCarga(c), true)
				return err
			}
		},
		"ExportarProcedimentos": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ExportarProcedimentos(ctx); return err }
		},
		"ImportarRascunhos": func() func(*application.Service) error {
			outro := e.workflow(true)
			c := arquivo(itemArquivo(codigoNovo(), "Novo", ""), itemArquivo(outro.CodigoProcessual, "Revisto", ""))
			return func(s *application.Service) error {
				_, err := s.ImportarRascunhos(ctx, gestor, c, application.HashCarga(c), true)
				return err
			}
		},
		"MudarSituacao": func() func(*application.Service) error {
			w := e.rascunho(e.real(), itemArquivo(codigoNovo(), "Rascunho", ""))
			return func(s *application.Service) error {
				_, err := s.MudarSituacao(ctx, w.ID, domain.SituacaoEmValidacao)
				return err
			}
		},
		"Homologar": func() func(*application.Service) error {
			vigente := e.workflow(true)
			w := e.rascunho(e.real(), itemArquivo(vigente.CodigoProcessual, "Revisão", ""))
			return func(s *application.Service) error { _, err := s.Homologar(ctx, gestor, w.ID); return err }
		},
		"HomologarNovo": func() func(*application.Service) error {
			w := e.rascunho(e.real(), itemArquivo(codigoNovo(), "Sem versão anterior", ""))
			return func(s *application.Service) error { _, err := s.Homologar(ctx, gestor, w.ID); return err }
		},
		"Validacoes": func() func(*application.Service) error {
			w := e.workflow(true)
			return func(s *application.Service) error { _, err := s.Validacoes(ctx, w.ID); return err }
		},
		"RegistrarValidacao": func() func(*application.Service) error {
			w := e.rascunho(e.real(), itemArquivo(codigoNovo(), "Rascunho", ""))
			return func(s *application.Service) error {
				_, err := s.RegistrarValidacao(ctx, gestor, w.ID, domain.Validacao{RealizadaEm: time.Now().AddDate(0, 0, -1), Unidade: "u", Registro: "r"})
				return err
			}
		},
		"ProcedimentosPorOrgao": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ProcedimentosPorOrgao(ctx, true); return err }
		},
		"Organograma": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Organograma(ctx, true); return err }
		},
		"Cobertura": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Cobertura(ctx); return err }
		},
		"BuscaGlobal": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.BuscaGlobal(ctx, "atlas", "pregão", 5); return err }
		},
		"ModelosDaSerie": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.ModelosDaSerie(ctx, serieVigente); return err }
		},
		"LigarModeloSerie": func() func(*application.Service) error {
			m := e.modelo()
			return func(s *application.Service) error {
				_, err := s.LigarModeloSerie(ctx, gestor, serieVigente, m.ID)
				return err
			}
		},
		"DesligarModeloSerie": func() func(*application.Service) error {
			m := e.modelo()
			if _, err := e.real().LigarModeloSerie(ctx, gestor, serieVigente, m.ID); err != nil {
				t.Fatal(err)
			}
			return func(s *application.Service) error {
				_, err := s.DesligarModeloSerie(ctx, serieVigente, m.ID)
				return err
			}
		},
		"LigarModelo": func() func(*application.Service) error {
			w, m := e.workflow(true), e.modelo()
			return func(s *application.Service) error {
				_, err := s.LigarModelo(ctx, w.ID, w.Etapas[0].Documentos[0].ID, &m.ID)
				return err
			}
		},
		"CreateComModelo": func() func(*application.Service) error {
			w := comModelo(novo(), e.modelo().ID)
			return func(s *application.Service) error { _, err := s.Create(ctx, gestor, w); return err }
		},
		"Perguntar": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Perguntar(ctx, "pregão eletrônico"); return err }
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

// Desativado não aparece fora da gestão; a auditoria da consulta falha com
// o banco fora.
func TestServiceRules(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	w := e.workflow(false)
	if _, err := e.real().Get(ctx, w.ID, false); err == nil {
		t.Fatal("desativado na consulta pública")
	}
	if got, err := e.real().Get(ctx, w.ID, true); err != nil || got.Ativo {
		t.Fatalf("desativado na gestão: %+v %v", got, err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := e.real().Perguntar(cctx, "pregão"); err == nil {
		t.Fatal("banco indisponível na consulta ao assistente")
	}
	semAutoria, err := e.real().Create(ctx, auth.Identity{}, novo())
	if err != nil || semAutoria.CreatedBy != nil {
		t.Fatalf("cadastro sem usuário local não grava autoria: %+v %v", semAutoria.CreatedBy, err)
	}
}

type unlimited struct{}

func (unlimited) Allow(context.Context, string) (bool, error) { return true, nil }

func TestHandlersReportServiceFailures(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), logger, 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), gestor)))
		})
	})
	h.RegisterPublicRoutes(r)
	h.RegisterRoutes(r, unlimited{})
	id := uuid.NewString()
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/atlas/ttdd", ""},
		{http.MethodGet, "/atlas/ttdd/estrutura", ""},
		{http.MethodGet, "/atlas/ttdd/exportar", ""},
		{http.MethodGet, "/atlas/ttdd/2.0.02.00.07", ""},
		{http.MethodGet, "/atlas/ttdd/2.0.02.00.07/historico", ""},
		{http.MethodPost, "/atlas/admin/ttdd/carga/simular", cargaCorpo()},
		{http.MethodPost, "/atlas/admin/workflows/" + id + "/versoes", `{"codigo_processual":"X.Y","titulo":"t","objetivo":"o","publico_alvo":"p","nivel_acesso":"PUBLICO",
			"codigo_ttdd":"2.0.02.00.07","etapas":[{"ordem":1,"unidade_administrativa":"A","nome_setor":"S","atribuicoes_setor":"x"}]}`},
		{http.MethodGet, "/atlas/workflows", ""},
		{http.MethodGet, "/atlas/workflows/" + id, ""},
		{http.MethodGet, "/atlas/admin/workflows", ""},
		{http.MethodGet, "/atlas/workflows/secretarias", ""},
		{http.MethodGet, "/atlas/admin/workflows/secretarias", ""},
		{http.MethodGet, "/atlas/organograma", ""},
		{http.MethodGet, "/atlas/admin/organograma", ""},
		{http.MethodPost, "/atlas/admin/workflows/" + id + "/ativar", ""},
		{http.MethodPost, "/atlas/chat", `{"query":"pregão eletrônico"}`},
		{http.MethodPost, "/atlas/admin/workflows", `{"codigo_processual":"X.Y","titulo":"t","objetivo":"o","publico_alvo":"p","nivel_acesso":"PUBLICO",
			"codigo_ttdd":"2.0.02.00.07","etapas":[{"ordem":1,"unidade_administrativa":"A","nome_setor":"S","atribuicoes_setor":"x"}]}`},
	} {
		down.calls = 0
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("%s %s com o banco fora: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
}

func TestModule(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	deps := modkit.Deps{Pool: pool, Outbox: outbox.NewWriter("test"), Logger: logger,
		Config: &config.Config{Atlas: config.AtlasConfig{AIEndpoint: "http://127.0.0.1:1", AIModel: "m"}}}
	m := atlas.New(deps)
	if m.Manifest().Key != atlas.Key || len(m.Manifest().Permissions) != 2 || !m.Manifest().Public {
		t.Fatalf("manifesto: %+v", m.Manifest())
	}
	// Com ATLAS_AI_ENDPOINT o módulo liga o cliente de IA; a busca global
	// aponta para o detalhe na página do Atlas.
	w := (&env{t: t, pool: pool}).workflow(true)
	prov := m.SearchProviders()[0]
	res, err := prov.Search(ctx, auth.Identity{}, w.CodigoProcessual, 5)
	if err != nil || len(res) == 0 || res[0].URL != "/atlas/procedimentos/"+w.ID.String() || prov.Module() != atlas.Key {
		t.Fatalf("busca global: %+v %v", res, err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := prov.Search(cctx, auth.Identity{}, "x", 5); err == nil {
		t.Fatal("busca com o banco indisponível falha")
	}
}

func cargaCorpo() string {
	b, _ := json.Marshal(map[string]string{"formato": "json", "conteudo": cargaTeste})
	return string(b)
}
