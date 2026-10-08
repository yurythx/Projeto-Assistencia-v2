package application_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/application"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func codigoNovo() string {
	return "TST.IMP." + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
}

// itemArquivo é um procedimento do arquivo equivalente a novo().
func itemArquivo(codigo, titulo, modelo string) application.ProcedimentoArquivo {
	return application.ProcedimentoArquivo{CodigoProcessual: codigo, Titulo: titulo, Objetivo: "Objetivo", PublicoAlvo: "Servidores",
		NivelAcesso: "PUBLICO", CodigoTTDD: serieVigente, Etapas: []application.EtapaArquivo{
			{Ordem: 1, UnidadeAdministrativa: "A", NomeSetor: "Setor A", AtribuicoesSetor: "Instruir",
				Documentos: []application.DocumentoArquivo{{NomeDocumento: "Peça", Formato: "NATO_DIGITAL", TipoAssinatura: "INDIVIDUAL", Modelo: modelo}},
				Transicoes: []application.TransicaoArquivo{{DestinoOrdem: 2, CondicaoTransicao: "ok"}}},
			{Ordem: 2, UnidadeAdministrativa: "B", NomeSetor: "Setor B", AtribuicoesSetor: "Decidir"},
		}}
}

func arquivo(itens ...application.ProcedimentoArquivo) string {
	b, _ := json.Marshal(application.ArquivoProcedimentos{Procedimentos: itens})
	return string(b)
}

func situacoes(r domain.ImportacaoProcedimentos) string {
	out := make([]string, len(r.Itens))
	for i, it := range r.Itens {
		out[i] = it.Situacao + ":" + it.Erro
	}
	return strings.Join(out, ",")
}

var todas = pagination.New(1, 100, 100)

func TestImportarProcedimentos(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()
	m := e.modelo()
	existente := e.workflow(true) // mesmo conteúdo de itemArquivo(…, "Procedimento de teste", "")
	novoCod, alterado := codigoNovo(), e.workflow(true)

	conteudo := arquivo(
		itemArquivo(novoCod, "Novo pela importação", m.Nome),
		itemArquivo(existente.CodigoProcessual, "Procedimento de teste", ""),
		itemArquivo(alterado.CodigoProcessual, "Título revisto", ""),
	)
	sim, err := s.ImportarProcedimentos(ctx, gestor, conteudo, "", false)
	if err != nil || sim.Aplicada || situacoes(sim) != "NOVO:,INALTERADO:,NOVA_VERSAO:" ||
		sim.Itens[0].Versao != 1 || sim.Itens[1].Versao != existente.Versao || sim.Itens[2].Versao != alterado.Versao+1 ||
		sim.Itens[0].Linha != 1 || sim.Totais[domain.ImportNovo] != 1 {
		t.Fatalf("simulação: %+v %v", sim, err)
	}
	// Simular não grava.
	if l, _, _ := s.List(ctx, domain.Filter{CodigoProcessual: novoCod, IncluirInativos: true}, todas); len(l) != 0 {
		t.Fatal("a simulação gravou")
	}
	if _, err := s.ImportarProcedimentos(ctx, gestor, conteudo, strings.Repeat("0", 64), true); codigo(err) != "VALIDATION_ERROR" {
		t.Fatalf("aplicar outro arquivo: %v", err)
	}
	ap, err := s.ImportarProcedimentos(ctx, gestor, conteudo, sim.Hash, true)
	if err != nil || !ap.Aplicada || situacoes(ap) != "NOVO:,INALTERADO:,NOVA_VERSAO:" {
		t.Fatalf("aplicar: %+v %v", ap, err)
	}
	criados, _, _ := s.List(ctx, domain.Filter{CodigoProcessual: novoCod}, todas)
	if len(criados) != 1 {
		t.Fatalf("procedimento novo: %+v", criados)
	}
	criado, _ := s.Get(ctx, criados[0].ID, false)
	if d := criado.Etapas[0].Documentos[0]; d.ModeloID == nil || *d.ModeloID != m.ID {
		t.Fatalf("peça sem o modelo pelo nome: %+v", d)
	}

	// Exportar e importar de volta: nada muda.
	exp, err := s.ExportarProcedimentos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var meus []application.ProcedimentoArquivo
	for _, p := range exp.Procedimentos {
		if p.CodigoProcessual == novoCod || p.CodigoProcessual == alterado.CodigoProcessual {
			meus = append(meus, p)
		}
	}
	comModelo := false
	for _, p := range meus {
		comModelo = comModelo || p.Etapas[0].Documentos[0].Modelo == m.Nome
	}
	if len(meus) != 2 || !comModelo {
		t.Fatalf("exportação: %+v", meus)
	}
	volta, err := s.ImportarProcedimentos(ctx, gestor, arquivo(meus...), "", false)
	if err != nil || situacoes(volta) != "INALTERADO:,INALTERADO:" {
		t.Fatalf("exportar e importar de volta: %+v %v", volta, err)
	}

	// Erros por item: o relatório mostra e aplicar recusa.
	revogada := itemArquivo(codigoNovo(), "Série revogada", "")
	revogada.CodigoTTDD = e.serieRevogada()
	repetido := codigoNovo()
	comErros := arquivo(itemArquivo(codigoNovo(), "", ""), itemArquivo(codigoNovo(), "Modelo que não existe", "Não existe"), revogada,
		itemArquivo(repetido, "Primeiro", ""), itemArquivo(repetido, "Repetido", ""))
	r, err := s.ImportarProcedimentos(ctx, gestor, comErros, "", false)
	if err != nil || r.Totais[domain.ImportErro] != 4 || r.Itens[3].Situacao != domain.ImportNovo ||
		!strings.Contains(r.Itens[1].Erro, `modelo "Não existe" não está ativo`) || !strings.Contains(r.Itens[2].Erro, "revogada") ||
		r.Itens[4].Erro != "código processual repetido no arquivo" {
		t.Fatalf("erros por item: %+v %v", r, err)
	}
	if _, err := s.ImportarProcedimentos(ctx, gestor, comErros, r.Hash, true); codigo(err) != "VALIDATION_ERROR" {
		t.Fatalf("aplicar com erros: %v", err)
	}

	// Arquivo inválido.
	muitos := make([]application.ProcedimentoArquivo, application.MaxProcedimentosImportacao+1)
	for _, c := range []string{"{", `{"procedimentos":[],"x":1}`, `{"procedimentos":[]}`, arquivo(muitos...)} {
		if _, err := s.ImportarProcedimentos(ctx, gestor, c, "", false); codigo(err) != "VALIDATION_ERROR" {
			t.Errorf("arquivo inválido %.40q: %v", c, err)
		}
	}
}

func TestImportacaoHTTP(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	router := func(down bool) *chi.Mux {
		svc := e.real()
		if down {
			svc = e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 1})
		}
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), gestor)))
			})
		})
		h := transport.NewHandlers(svc, logger, 100)
		h.RegisterRoutes(r, unlimited{})
		return r
	}
	do := func(r *chi.Mux, method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}
	r := router(false)
	conteudo, _ := json.Marshal(arquivo(itemArquivo(codigoNovo(), "Pela API", "")))
	rec := do(r, http.MethodPost, "/atlas/admin/workflows/importacao/simular", `{"conteudo":`+string(conteudo)+`}`)
	var sim struct {
		Data domain.ImportacaoProcedimentos `json:"data"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &sim) != nil || sim.Data.Totais[domain.ImportNovo] != 1 {
		t.Fatalf("simular: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(r, http.MethodPost, "/atlas/admin/workflows/importacao/aplicar", `{"conteudo":`+string(conteudo)+`,"hash":"`+sim.Data.Hash+`"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"aplicada":true`) {
		t.Fatalf("aplicar: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(r, http.MethodGet, "/atlas/admin/workflows/exportar", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "procedimentos.json") ||
		!strings.Contains(rec.Body.String(), "Pela API") {
		t.Fatalf("exportar: %d %v", rec.Code, rec.Header())
	}
	if rec := do(r, http.MethodGet, "/atlas/admin/cobertura", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "pecas_sem_modelo") {
		t.Fatalf("cobertura: %d", rec.Code)
	}
	for body, want := range map[string]int{"{": http.StatusBadRequest, `{}`: http.StatusUnprocessableEntity} {
		if rec := do(r, http.MethodPost, "/atlas/admin/workflows/importacao/simular", body); rec.Code != want {
			t.Errorf("corpo %s: %d, quero %d", body, rec.Code, want)
		}
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/atlas/admin/workflows/exportar", ""},
		{http.MethodGet, "/atlas/admin/cobertura", ""},
		{http.MethodPost, "/atlas/admin/workflows/importacao/simular", `{"conteudo":` + string(conteudo) + `}`},
	} {
		if rec := do(router(true), c.method, c.path, c.body); rec.Code != http.StatusInternalServerError {
			t.Errorf("%s %s com o banco fora: %d", c.method, c.path, rec.Code)
		}
	}
}

func TestCobertura(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()
	w, solto := e.workflow(true), e.modelo()
	// Sem modelo na série: a peça não herda nenhum (ver o modelo da série).
	ligados, _ := s.ModelosDaSerie(ctx, serieVigente)
	for _, m := range ligados {
		if _, err := s.DesligarModeloSerie(ctx, serieVigente, m.ID); err != nil {
			t.Fatal(err)
		}
	}
	c, err := s.Cobertura(ctx)
	if err != nil || len(c.Orgaos) == 0 || c.Totais.Series == 0 || c.Totais.Procedimentos < 1 {
		t.Fatalf("painel: %+v %v", c.Totais, err)
	}
	pecaSem, modeloSolto := false, false
	for _, p := range c.PecasSemModelo {
		pecaSem = pecaSem || (p.WorkflowID == w.ID && p.Peca == "Peça" && p.Etapa == 1)
	}
	for _, m := range c.ModelosSemUso {
		modeloSolto = modeloSolto || m.ID == solto.ID
	}
	if !pecaSem || !modeloSolto || c.Totais.PecasComModelo != c.Totais.Pecas-len(c.PecasSemModelo) {
		t.Fatalf("listas do painel: peça %v, modelo %v, %+v", pecaSem, modeloSolto, c.Totais)
	}
	// Ligar o modelo à peça tira os dois das listas.
	if _, err := s.LigarModelo(ctx, w.ID, w.Etapas[0].Documentos[0].ID, &solto.ID); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Cobertura(ctx)
	for _, p := range c.PecasSemModelo {
		if p.WorkflowID == w.ID {
			t.Fatal("peça com modelo continua na lista")
		}
	}
	for _, m := range c.ModelosSemUso {
		if m.ID == solto.ID {
			t.Fatal("modelo em uso continua na lista")
		}
	}
}

// Lacunas: subfunção com série de processo e nenhum procedimento aparece no
// painel; some quando ganha um procedimento (mesmo rascunho).
func TestCoberturaLacunas(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()
	for _, sql := range []string{
		`INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome) VALUES ('2.0.02.98', '2.0.02', 'Subfunção de teste das lacunas')
			ON CONFLICT (codigo) DO NOTHING`,
		`INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_interm_anos, destinacao_final)
			VALUES ('2.0.02.98.01', '2.0.02.98', 'Processo de teste das lacunas', 1, 1, 'ELIMINACAO') ON CONFLICT (codigo) DO NOTHING`,
	} {
		if _, err := e.pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	tem := func() bool {
		c, err := s.Cobertura(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range c.Lacunas {
			if l.Codigo == "2.0.02.98" {
				return l.Series == 1 && l.Exemplos[0] == "Processo de teste das lacunas" && l.Orgao != ""
			}
		}
		return false
	}
	if !tem() {
		t.Fatal("subfunção sem procedimento fora das lacunas")
	}
	item := itemArquivo(codigoNovo(), "Cobre a lacuna", "")
	item.CodigoTTDD = "2.0.02.98.01"
	e.rascunho(s, item)
	if tem() {
		t.Fatal("subfunção com rascunho continua nas lacunas")
	}
}
