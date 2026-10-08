package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// globalCom cria um usuário com concessão global de um perfil que tem as
// permissões dadas; devolve o token.
func (h *apiHarness) globalCom(t *testing.T, permissoes ...string) string {
	t.Helper()
	admin := h.admin()
	perms, _ := json.Marshal(permissoes)
	perfil := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", admin,
		`{"nome":"Perfil `+uuid.NewString()[:8]+`","permissoes":`+string(perms)+`}`)).ID
	id, _ := h.user("nexus-user")
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+id.String()+"/lotacoes", admin, `{"perfil_id":"`+perfil+`"}`)
	return h.relogin(id)
}

type atlasWorkflow struct {
	ID               string  `json:"id"`
	CodigoProcessual string  `json:"codigo_processual"`
	Ativo            bool    `json:"ativo"`
	CreatedBy        *string `json:"created_by"`
	TotalEtapas      int     `json:"total_etapas"`
	Etapas           []struct {
		Ordem      int `json:"ordem"`
		Documentos []struct {
			NomeDocumento string `json:"nome_documento"`
		} `json:"documentos"`
		Transicoes []struct {
			DestinoOrdem int `json:"destino_ordem"`
		} `json:"transicoes"`
	} `json:"etapas"`
}

func atlasBody(codigo, extra string) string {
	return `{"codigo_processual":"` + codigo + `","titulo":"Concessão de Diárias Atlasteste","objetivo":"Pagamento de diárias de viagem a serviço",
		"publico_alvo":"Servidores","nivel_acesso":"PUBLICO","codigo_ttdd":"2.0.05.00.00"` + extra + `,
		"etapas":[
			{"ordem":1,"unidade_administrativa":"sec/dem","nome_setor":"Setor Demandante","atribuicoes_setor":"Formalizar o pedido de diárias",
			 "prazo_sla_em_dias":2,"documentos":[{"nome_documento":"Requerimento de Diárias","obrigatorio":true,"formato":"NATO_DIGITAL","tipo_assinatura":"INDIVIDUAL"}],
			 "transicoes":[{"destino_ordem":2,"condicao_transicao":"Pedido instruído"}]},
			{"ordem":2,"unidade_administrativa":"SEFIN","nome_setor":"Finanças","atribuicoes_setor":"Empenhar e pagar",
			 "transicoes":[{"destino_ordem":1,"condicao_transicao":"Falta documento","is_devolucao_diligencia":true,"descricao_diligencia":"Anexar o comprovante"}]}
		]}`
}

// Atlas: consulta pública só de ativos, gestão com atlas:manage (evento e
// auditoria na mesma transação), validação de domínio, desativação e Busca
// Global.
func TestAtlasCatalogoHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	sfx := strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
	codigo := "TST.ATLAS." + sfx

	// Consulta pública (anônima): TTDD e procedimentos semeados.
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd?q=preg%C3%A3o", "", "").Body.String(); !strings.Contains(body, "2.0.02.00.07") {
		t.Fatalf("TTDD por descritor com acento: %s", body)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/2.0.02.00.07", "", "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/atlas/ttdd/9.0.99.99.99", "", "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/atlas/ttdd/abc", "", "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/atlas/ttdd?codigo=abc", "", "")

	// TTDD oficial (ADR 017): série com a hierarquia e a publicação; a
	// semente antiga trazia 5/50 anos e guarda permanente para esta série.
	serie := data[struct {
		FaseCorrenteAnos *int    `json:"fase_corrente_anos"`
		FaseIntermAnos   *int    `json:"fase_interm_anos"`
		DestinacaoFinal  *string `json:"destinacao_final"`
		Subfuncao        *struct {
			Codigo string `json:"codigo"`
			Funcao struct {
				Orgao struct {
					EdicaoDiario string `json:"edicao_diario"`
				} `json:"orgao"`
			} `json:"funcao"`
		} `json:"subfuncao"`
	}](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/2.0.07.00.00", "", ""))
	if serie.FaseCorrenteAnos == nil || *serie.FaseCorrenteAnos != 1 || serie.FaseIntermAnos == nil || *serie.FaseIntermAnos != 99 ||
		serie.DestinacaoFinal == nil || *serie.DestinacaoFinal != "ELIMINACAO" || serie.Subfuncao == nil ||
		serie.Subfuncao.Codigo != "2.0.07.00" || serie.Subfuncao.Funcao.Orgao.EdicaoDiario != "6.017" {
		t.Fatalf("série oficial 2.0.07.00.00: %+v", serie)
	}
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd?codigo=2.0.02&page_size=100", "", "").Body.String(); !strings.Contains(body, "2.0.02.00.07") ||
		!strings.Contains(body, "2.0.02.01.02") || strings.Contains(body, "2.0.01.") {
		t.Fatalf("filtro por função (prefixo hierárquico): %s", body)
	}
	// Exportação CSV (LAI): BOM + ";" para o Excel em português, filtro hierárquico.
	csvRec := h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/exportar?codigo=2.0.02", "", "")
	csvBody := csvRec.Body.String()
	if !strings.HasPrefix(csvBody, "\ufeffCódigo;Série documental;Órgão") || !strings.Contains(csvBody, "2.0.02.01.02;Processos de Dispensa de licitação;") ||
		strings.Contains(csvBody, "2.0.01.") || !strings.HasPrefix(csvRec.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(csvRec.Header().Get("Content-Disposition"), "ttdd.csv") {
		t.Fatalf("exportação CSV: %q %v", csvBody[:min(300, len(csvBody))], csvRec.Header())
	}
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/atlas/ttdd/exportar?codigo=x", "", "")
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/estrutura", "", "").Body.String(); !strings.Contains(body, `"prefixo":"2.0"`) ||
		!strings.Contains(body, `"codigo":"2.0.02.01"`) || !strings.Contains(body, `"total"`) {
		t.Fatalf("estrutura da TTDD: %s", body)
	}
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/atlas/ttdd/"+strings.Repeat("9", 33), "", "")
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/workflows?q=pregao&page_size=5", "", "").Body.String(); !strings.Contains(body, "ADM.LIC.001") ||
		!strings.Contains(body, `"total_items"`) {
		t.Fatalf("lista pública paginada com busca sem acento: %s", body)
	}

	// Gestão: anônimo 401, sem atlas:manage 403.
	_, comum := h.user("nexus-user")
	h.expect(http.StatusUnauthorized, http.MethodPost, "/api/v1/atlas/admin/workflows", "", atlasBody(codigo, ""))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/atlas/admin/workflows", comum, atlasBody(codigo, ""))
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/atlas/admin/workflows", comum, "")

	// Validação (422): domínio e TTDD inexistente.
	for _, body := range []string{
		atlasBody("COM ESPACO", ""),
		atlasBody(codigo, `,"nivel_acesso":"RESTRITO"`), // restrito sem hipótese legal
		strings.Replace(atlasBody(codigo, ""), `"destino_ordem":2`, `"destino_ordem":7`, 1),
		strings.Replace(atlasBody(codigo, ""), `"2.0.05.00.00"`, `"9.9.99.99.99"`, 1),
		strings.Replace(atlasBody(codigo, ""), `"obrigatorio":true`, `"obrigatorio":true,"modelo_minuta_padrao_url":"javascript:alert(1)"`, 1),
		`{"codigo_processual":"X","titulo":"t","objetivo":"o","publico_alvo":"p","nivel_acesso":"PUBLICO","codigo_ttdd":"2.0.05.00.00","etapas":[]}`,
	} {
		h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/atlas/admin/workflows", admin, body)
	}
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/atlas/admin/workflows", admin, `{`)

	// Cadastro: etapas, peças e transições (destino pela ordem) gravados;
	// código em caixa alta; autoria; evento e auditoria.
	gestor := h.globalCom(t, "atlas:manage")
	wf := data[atlasWorkflow](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/atlas/admin/workflows", gestor,
		atlasBody(strings.ToLower(codigo), "")))
	if wf.CodigoProcessual != codigo || !wf.Ativo || wf.CreatedBy == nil || wf.TotalEtapas != 2 || len(wf.Etapas) != 2 ||
		len(wf.Etapas[0].Documentos) != 1 || wf.Etapas[0].Transicoes[0].DestinoOrdem != 2 || wf.Etapas[1].Transicoes[0].DestinoOrdem != 1 {
		t.Fatalf("procedimento gravado: %+v", wf)
	}
	if outboxCount(t, h, "atlas.workflow.created", wf.ID) != 1 {
		t.Fatal("evento atlas.workflow.created no outbox")
	}
	var audits int
	if err := h.d.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE action = 'atlas.workflow.created' AND resource_id = $1`, wf.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("auditoria do cadastro: %d %v", audits, err)
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/atlas/admin/workflows", gestor, atlasBody(codigo, ""))
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/atlas/admin/workflows", gestor, atlasBody(codigo, `,"versao":2`))

	pub := data[atlasWorkflow](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/workflows/"+wf.ID, "", ""))
	if len(pub.Etapas) != 2 {
		t.Fatalf("detalhe público com etapas: %+v", pub)
	}
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q=Atlasteste&module=atlas", admin, "").Body.String(); !strings.Contains(body, wf.ID) {
		t.Fatalf("Busca Global: %s", body)
	}

	// Desativado: some da consulta pública, continua na gestão.
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/admin/workflows/"+wf.ID+"/desativar", gestor, "")
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/admin/workflows/"+wf.ID+"/desativar", gestor, "") // já desativado
	if outboxCount(t, h, "atlas.workflow.deactivated", wf.ID) != 1 {
		t.Fatal("desativar de novo não emite outro evento")
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/atlas/workflows/"+wf.ID, "", "")
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/workflows?q="+codigo, "", "").Body.String(); strings.Contains(body, wf.ID) {
		t.Fatalf("inativo na lista pública: %s", body)
	}
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/admin/workflows?q="+codigo, gestor, "").Body.String(); !strings.Contains(body, wf.ID) {
		t.Fatalf("inativo na lista de gestão: %s", body)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/admin/workflows/"+wf.ID, gestor, "")
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/admin/workflows/"+wf.ID+"/ativar", gestor, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/workflows/"+wf.ID, "", "")

	for _, path := range []string{"/api/v1/atlas/admin/workflows/x/ativar", "/api/v1/atlas/admin/workflows/x/desativar"} {
		h.expect(http.StatusBadRequest, http.MethodPost, path, gestor, "")
	}
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/atlas/workflows/x", "", "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/atlas/workflows/"+uuid.NewString(), "", "")
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/atlas/admin/workflows/"+uuid.NewString()+"/ativar", gestor, "")

	// Módulo desligado: superfície pública some (Guard do Kernel).
	h.setModule(admin, "atlas", false)
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/atlas/ttdd", "", "")
}

type atlasResposta struct {
	Answer  string  `json:"answer"`
	Score   float64 `json:"score"`
	Refused bool    `json:"refused"`
	Mode    string  `json:"mode"`
	Sources []struct {
		Tipo   string `json:"tipo"`
		Codigo string `json:"codigo"`
	} `json:"sources"`
}

// Assistente: atlas:read, só o acervo do Atlas — fluxos e TTDD (ADRs 021 e
// 023) — com grounding estrito (limiar), recusa para qualquer outro
// assunto, síntese canônica sem IA configurada, auditoria sem o texto da
// pergunta e limite por identidade.
func TestAtlasAssistenteHTTP(t *testing.T) {
	h := newHarness(t)
	_, comum := h.user("nexus-user")
	leitor := h.globalCom(t, "atlas:read")

	const pergunta = `{"query":"Qual o prazo de guarda dos processos de pregão eletrônico?"}`
	h.expect(http.StatusUnauthorized, http.MethodPost, "/api/v1/atlas/chat", "", pergunta)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/atlas/chat", comum, pergunta)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/atlas/chat", leitor, `{"query":"oi"}`)

	r := data[atlasResposta](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/chat", leitor, pergunta))
	if r.Refused || r.Mode != "sintese" || r.Score < 0.65 || len(r.Sources) == 0 || r.Sources[0].Tipo != "ttdd" ||
		r.Sources[0].Codigo != "2.0.02.00.07" || !strings.Contains(r.Answer, "2.0.02.00.07") || strings.Contains(r.Answer, "ADM.LIC.001") {
		t.Fatalf("resposta fundamentada só na TTDD: %+v", r)
	}
	// Pergunta só de temporalidade: a série da TTDD sustenta a resposta.
	ttdd := data[atlasResposta](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/chat", leitor,
		`{"query":"Qual o prazo de guarda da pasta funcional de servidores?"}`))
	if ttdd.Refused || len(ttdd.Sources) == 0 || ttdd.Sources[0].Tipo != "ttdd" || ttdd.Sources[0].Codigo != "2.0.07.00.00" ||
		!strings.Contains(ttdd.Answer, "99 anos") || !strings.Contains(ttdd.Answer, "eliminação") {
		t.Fatalf("resposta pela TTDD: %+v", ttdd)
	}
	// Fora do objetivo (outro assunto ou tarefa): a recusa canônica.
	recusa := data[atlasResposta](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/chat", leitor,
		`{"query":"licença para viagem internacional de férias"}`))
	if !recusa.Refused || recusa.Mode != "recusada" || len(recusa.Sources) != 0 || !strings.HasPrefix(recusa.Answer, "Esse assunto foge do objetivo da IA") {
		t.Fatalf("assunto fora da TTDD deve ser recusado: %+v", recusa)
	}
	// Fluxo (ADR 023): o procedimento do início ao fim, com etapas e peças.
	if proc := data[atlasResposta](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/chat", leitor,
		`{"query":"Como tramitar o processo de pregão eletrônico?"}`)); proc.Refused || len(proc.Sources) == 0 ||
		proc.Sources[0].Tipo != "procedimento" || proc.Sources[0].Codigo != "ADM.LIC.001" || !strings.Contains(proc.Answer, "Fluxo em") ||
		!strings.Contains(proc.Answer, "Peças exigidas") {
		t.Fatalf("fluxo do pregão: %+v", proc)
	}
	var vazou int
	if err := h.d.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs a WHERE action = 'atlas.assistente.consulta' AND row_to_json(a)::text ILIKE '%viagem internacional%'`).Scan(&vazou); err != nil || vazou != 0 {
		t.Fatalf("auditoria não guarda o texto da pergunta: %d %v", vazou, err)
	}

	// Limite por identidade (5/min no harness): 5 usadas acima (4 OK + 1
	// 422 também conta), a 6ª é barrada.
	h.expect(http.StatusTooManyRequests, http.MethodPost, "/api/v1/atlas/chat", leitor, pergunta)
}

type atlasVersao struct {
	ID               string `json:"id"`
	CodigoProcessual string `json:"codigo_processual"`
	Versao           int    `json:"versao"`
	Ativo            bool   `json:"ativo"`
	Classificacao    struct {
		RevogadaEm     *string `json:"revogada_em"`
		RevogadaEdicao string  `json:"revogada_edicao"`
	} `json:"classificacao"`
}

// Vigência da TTDD (ADR 019): série revogada sai da consulta mas continua
// por código, com o histórico de prazos; não aceita procedimento novo; o
// procedimento que já aponta para ela mostra a revogação. Nova versão de
// procedimento: mesmo código, versão seguinte, demais versões desativadas.
func TestAtlasVigenciaEVersoesHTTP(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	gestor := h.globalCom(t, "atlas:manage")
	codigo := "TST.VERS." + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
	const serie = "2.0.02.00.97"
	naSerie := func(body string) string { return strings.Replace(body, `"2.0.05.00.00"`, `"`+serie+`"`, 1) }

	if _, err := h.d.DB.Exec(ctx, `INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos,
		fase_interm_anos, destinacao_final) VALUES ($1, '2.0.02.00', 'Série de teste da vigência', 1, 2, 'ELIMINACAO')`, serie); err != nil {
		t.Fatal(err)
	}
	v1 := data[atlasVersao](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/atlas/admin/workflows", gestor, naSerie(atlasBody(codigo, ""))))

	// Nova publicação: muda o prazo (histórico ALTERADA, edição do órgão)
	// e depois retira a série (REVOGADA). Regravar igual não gera histórico.
	for _, sql := range []string{
		`UPDATE atlas_classificacao_ttdd SET fase_interm_anos = 5 WHERE codigo = $1`,
		`UPDATE atlas_classificacao_ttdd SET fase_interm_anos = 5 WHERE codigo = $1`,
		`UPDATE atlas_classificacao_ttdd SET revogada_em = '2027-03-01', revogada_edicao = '6.400' WHERE codigo = $1`,
	} {
		if _, err := h.d.DB.Exec(ctx, sql, serie); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/api/v1/atlas/ttdd?q=vig%C3%AAncia", "/api/v1/atlas/ttdd?codigo=2.0.02.00&page_size=100", "/api/v1/atlas/ttdd/exportar"} {
		if body := h.expect(http.StatusOK, http.MethodGet, path, "", "").Body.String(); strings.Contains(body, serie) {
			t.Fatalf("série revogada na consulta %s: %s", path, body)
		}
	}
	rev := data[struct {
		RevogadaEm     *string `json:"revogada_em"`
		RevogadaEdicao string  `json:"revogada_edicao"`
	}](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/"+serie, "", ""))
	if rev.RevogadaEm == nil || !strings.HasPrefix(*rev.RevogadaEm, "2027-03-01") || rev.RevogadaEdicao != "6.400" {
		t.Fatalf("série revogada por código: %+v", rev)
	}
	type evento struct {
		Evento   string `json:"evento"`
		Anterior struct {
			FaseIntermAnos *int `json:"fase_interm_anos"`
		} `json:"anterior"`
		EdicaoDiario string `json:"edicao_diario"`
	}
	hist := data[[]evento](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/"+serie+"/historico", "", ""))
	if len(hist) != 2 || hist[0].Evento != "REVOGADA" || hist[0].EdicaoDiario != "6.400" || *hist[0].Anterior.FaseIntermAnos != 5 ||
		hist[1].Evento != "ALTERADA" || hist[1].EdicaoDiario != "6.017" || *hist[1].Anterior.FaseIntermAnos != 2 {
		t.Fatalf("histórico da série: %+v", hist)
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/atlas/ttdd/9.0.99.99.99/historico", "", "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/atlas/ttdd/abc/historico", "", "")

	// O procedimento que já apontava continua, com a revogação visível; um
	// cadastro novo (ou nova versão) na série revogada é recusado.
	if got := data[atlasVersao](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/admin/workflows/"+v1.ID, gestor, "")); got.Classificacao.RevogadaEm == nil {
		t.Fatalf("procedimento sem a revogação da série: %+v", got)
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/atlas/admin/workflows", gestor, naSerie(atlasBody(codigo+".X", "")))
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/atlas/admin/workflows/"+v1.ID+"/versoes", gestor, naSerie(atlasBody(codigo, "")))

	// Nova versão (o código e a versão enviados são ignorados).
	v2 := data[atlasVersao](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/atlas/admin/workflows/"+v1.ID+"/versoes", gestor,
		atlasBody("OUTRO.CODIGO", `,"versao":9`)))
	if v2.CodigoProcessual != codigo || v2.Versao != 2 || !v2.Ativo || v2.Classificacao.RevogadaEm != nil {
		t.Fatalf("nova versão: %+v", v2)
	}
	if antiga := data[atlasVersao](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/admin/workflows/"+v1.ID, gestor, "")); antiga.Ativo {
		t.Fatal("versão anterior continua ativa")
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/atlas/workflows/"+v1.ID, "", "")
	if outboxCount(t, h, "atlas.workflow.deactivated", v1.ID) != 1 || outboxCount(t, h, "atlas.workflow.created", v2.ID) != 1 {
		t.Fatal("eventos da nova versão")
	}
	var audits int
	if err := h.d.DB.QueryRow(ctx, `SELECT count(*) FROM audit_logs a WHERE action = 'atlas.workflow.deactivated' AND resource_id = $1
		AND row_to_json(a)::text LIKE '%' || $2 || '%'`, v1.ID, v2.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("auditoria da substituição: %d %v", audits, err)
	}
	// A partir da versão antiga também: vira a 3 e desativa a 2.
	v3 := data[atlasVersao](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/atlas/admin/workflows/"+v1.ID+"/versoes", gestor, atlasBody(codigo, "")))
	if v3.Versao != 3 {
		t.Fatalf("terceira versão: %+v", v3)
	}
	versoes := data[[]atlasVersao](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/admin/workflows?codigo_processual="+strings.ToLower(codigo), gestor, ""))
	if len(versoes) != 3 || versoes[0].Versao != 3 || !versoes[0].Ativo || versoes[1].Ativo || versoes[2].Ativo {
		t.Fatalf("versões do procedimento: %+v", versoes)
	}
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/atlas/admin/workflows/x/versoes", gestor, atlasBody(codigo, ""))
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/atlas/admin/workflows/"+v1.ID+"/versoes", gestor, `{`)
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/atlas/admin/workflows/"+uuid.NewString()+"/versoes", gestor, atlasBody(codigo, ""))
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/atlas/admin/workflows/"+v1.ID+"/versoes", gestor,
		strings.Replace(atlasBody(codigo, ""), `"destino_ordem":2`, `"destino_ordem":7`, 1))

	// Série de volta numa publicação seguinte: RESTABELECIDA.
	if _, err := h.d.DB.Exec(ctx, `UPDATE atlas_classificacao_ttdd SET revogada_em = NULL, revogada_edicao = '' WHERE codigo = $1`, serie); err != nil {
		t.Fatal(err)
	}
	if hist := data[[]evento](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/"+serie+"/historico", "", "")); len(hist) != 3 || hist[0].Evento != "RESTABELECIDA" {
		t.Fatalf("restabelecimento no histórico: %+v", hist)
	}
}
