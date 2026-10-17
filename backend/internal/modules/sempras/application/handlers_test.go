package application_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/transport"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
)

// api roda as rotas do módulo com o repositório de verdade e uma
// identidade global (todas as permissões em todas as unidades).
type api struct {
	t *testing.T
	r http.Handler
}

func novaAPI(t *testing.T, c cenario) api {
	h := transport.NewHandlers(c.limpo(), testLogger())
	r := chi.NewRouter()
	identidade := c.global()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), identidade)))
		})
	})
	h.RegisterRoutes(r)
	return api{t: t, r: r}
}

// do faz a requisição, exige o status e devolve o campo data da resposta.
func (a api) do(want int, method, path, body string) map[string]any {
	a.t.Helper()
	rec := httptest.NewRecorder()
	a.r.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	if rec.Code != want {
		a.t.Fatalf("%s %s: esperado %d, veio %d %s", method, path, want, rec.Code, rec.Body.String())
	}
	var env struct {
		Data any `json:"data"`
	}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			a.t.Fatalf("%s %s: resposta ilegível: %v", method, path, err)
		}
	}
	if m, ok := env.Data.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// id cria o recurso e devolve o id dele.
func (a api) id(method, path, body string) string {
	a.t.Helper()
	return a.do(http.StatusCreated, method, path, body)["id"].(string)
}

// TestHandlersCaminhoFeliz percorre todas as rotas do módulo com sucesso —
// o contrato HTTP (status, corpo, cabeçalhos) de cada caso de uso.
func TestHandlersCaminhoFeliz(t *testing.T) {
	c := novoCenario(t)
	a := novaAPI(t, c)
	cras, creas, pop, abrigo, conselho, mulher := c.unidade.String(), c.unCreas.String(), c.unPop.String(),
		c.unAbrigo.String(), c.unConselho.String(), c.unMulher.String()
	nis := func() string { return fmt.Sprintf("%011d", rand.Int64N(1e11)) }
	pessoa := func() string { return a.id("POST", "/sempras/cidadaos", `{"nome":"Pessoa Teste","nis":"`+nis()+`"}`) }

	// Cadastro, busca, painel e relatórios.
	cid := pessoa()
	a.id("POST", "/sempras/cidadaos", `{"nome":"Maria Teste","nis":"`+nis()+`","sexo":"feminino","data_nascimento":"1990-01-01"}`)
	a.do(422, "POST", "/sempras/cidadaos", `{"nome":"Maria Teste","nis":"`+nis()+`","sexo":"outro"}`)
	a.do(200, "GET", "/sempras/cidadaos/"+cid, "")
	a.do(200, "GET", "/sempras/cidadaos?q=Pessoa", "")
	a.do(200, "PUT", "/sempras/cidadaos/"+cid, `{"nome":"Pessoa Editada","nis":"`+nis()+`"}`)
	a.do(200, "GET", "/sempras/unidades", "")
	a.do(200, "GET", "/sempras/painel", "")
	a.do(200, "GET", "/sempras/relatorios/rma?competencia=2026-10", "")
	a.do(200, "GET", "/sempras/unidades/configuracao", "")
	a.do(200, "PUT", "/sempras/unidades/"+c.unidade2.String()+"/configuracao", `{"tipo":"creas","painel_exibe_nome":false}`)

	// Prontuário e direitos do titular.
	a.do(200, "GET", "/sempras/cidadaos/"+cid+"/prontuario", "")
	a.id("POST", "/sempras/cidadaos/"+cid+"/registros", `{"unidade_id":"`+cras+`","tipo":"social","texto":"evolução"}`)
	a.do(200, "GET", "/sempras/cidadaos/"+cid+"/pedidos/acesso?justificativa=titular+presente", "")
	pedido := a.id("POST", "/sempras/cidadaos/"+pessoa()+"/pedidos", `{"tipo":"anonimizacao","justificativa":"esquecer"}`)
	a.do(200, "GET", "/sempras/pedidos", "")
	a.do(200, "POST", "/sempras/pedidos/"+pedido+"/decidir", `{"aprovar":false,"justificativa":"há obrigação legal de guarda"}`)

	// Rede de apoio, benefício, encaminhamento entre unidades.
	contato := a.id("POST", "/sempras/cidadaos/"+cid+"/rede-apoio", `{"nome":"Maria","parentesco":"Irmã"}`)
	a.do(200, "DELETE", "/sempras/rede-apoio/"+contato, "")
	ben := a.id("POST", "/sempras/cidadaos/"+cid+"/beneficios",
		`{"unidade_id":"`+cras+`","tipo":"cesta_basica","forma_acesso":"demanda_espontanea","quantidade":1}`)
	a.do(200, "POST", "/sempras/beneficios/"+ben+"/decidir", `{"acao":"conceder"}`)
	enc := a.id("POST", "/sempras/cidadaos/"+cid+"/encaminhamentos",
		`{"unidade_origem_id":"`+cras+`","unidade_destino_id":"`+creas+`","motivo":"acompanhamento especializado"}`)
	a.do(200, "GET", "/sempras/unidades/"+creas+"/encaminhamentos", "")
	a.do(200, "POST", "/sempras/encaminhamentos/"+enc+"/decidir", `{"aceitar":true}`)

	// CRAS: PAIF, ofício, coletivos.
	paif := a.id("POST", "/sempras/cidadaos/"+cid+"/paif", `{"unidade_id":"`+cras+`","tipo_atendimento":"particularizado"}`)
	a.do(200, "POST", "/sempras/paif/"+paif+"/excluir", `{"motivo":"mudou de território"}`)
	of := a.id("POST", "/sempras/cidadaos/"+cid+"/oficios-cras", `{"unidade_id":"`+cras+`","orgao_emissor":"MPE","numero_oficio":"1/2026"}`)
	a.do(200, "POST", "/sempras/oficios-cras/"+of+"/responder", `{"numero_resposta":"R-1"}`)
	a.id("POST", "/sempras/unidades/"+cras+"/coletivos", `{"data_evento":"2026-10-10T00:00:00Z","categoria":"scfv_0_6","participantes":5}`)
	a.do(200, "GET", "/sempras/unidades/"+cras+"/coletivos?mes=2026-10", "")

	// Conselho Tutelar, CREAS (PAEFI, MSE, notificação) e Casa da Mulher.
	ct := a.id("POST", "/sempras/cidadaos/"+cid+"/conselho-tutelar",
		`{"unidade_id":"`+conselho+`","origem_demanda":"escola","motivo_violacao":"evasao_escolar"}`)
	a.do(200, "POST", "/sempras/conselho-tutelar/"+ct+"/transitar", `{"acao":"arquivar"}`)
	paefi := a.id("POST", "/sempras/cidadaos/"+cid+"/paefi", `{"unidade_id":"`+creas+`","violacoes":["negligencia"]}`)
	a.do(200, "POST", "/sempras/paefi/"+paefi+"/pia", "")
	a.do(200, "POST", "/sempras/paefi/"+paefi+"/encerrar", `{"motivo":"objetivos_alcancados"}`)
	mse := a.id("POST", "/sempras/cidadaos/"+cid+"/mse",
		`{"unidade_id":"`+creas+`","ato_infracional":"furto","tipo_medida":"liberdade_assistida"}`)
	a.do(200, "POST", "/sempras/mse/"+mse+"/concluir", `{"resultado":"cumprida"}`)
	nt := a.id("POST", "/sempras/cidadaos/"+cid+"/creas-notificacoes",
		`{"unidade_id":"`+creas+`","origem_entidade":"ministerio_publico","assunto":"requisição de relatório"}`)
	a.do(200, "POST", "/sempras/creas-notificacoes/"+nt+"/transitar", `{"acao":"iniciar_diligencia"}`)
	cm := a.id("POST", "/sempras/cidadaos/"+cid+"/casa-mulher", casoMulherBody(mulher))
	a.do(200, "PUT", "/sempras/casa-mulher/"+cm, casoMulherBody(""))
	a.do(200, "POST", "/sempras/casa-mulher/"+cm+"/encerrar", `{"motivo":"superacao_violencia"}`)

	// Casa Abrigo: acolhimento e tudo o que pende dele.
	ac := a.id("POST", "/sempras/cidadaos/"+pessoa()+"/casa-abrigo",
		`{"unidade_id":"`+abrigo+`","sexo":"feminino","motivos":["negligencia_abandono"]}`)
	a.do(200, "PUT", "/sempras/casa-abrigo/"+ac, `{"unidade_id":"`+abrigo+`","sexo":"feminino","motivos":["orfandade_ausencia_responsavel"]}`)
	irmao := a.id("POST", "/sempras/casa-abrigo/"+ac+"/irmaos", `{"nome":"Irmão"}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-irmaos/"+irmao, "")
	diag := a.id("POST", "/sempras/casa-abrigo/"+ac+"/diagnosticos", `{"categoria":"doenca","diagnostico":"asma"}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-diagnosticos/"+diag, "")
	vis := a.id("POST", "/sempras/casa-abrigo/"+ac+"/visitantes", `{"nome_visitante":"Avó","data_visita":"2026-10-10T00:00:00Z"}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-visitantes/"+vis, "")
	termo := a.id("POST", "/sempras/casa-abrigo/"+ac+"/termos", `{"tipo_termo":"passeio","data_termo":"2026-10-10T00:00:00Z"}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-termos/"+termo, "")
	a.do(200, "PUT", "/sempras/casa-abrigo/"+ac+"/pia", `{"escola":"EMEB Centro"}`)
	meta := a.id("POST", "/sempras/casa-abrigo/"+ac+"/pia-metas", `{"tipo":"plano","descricao":"retomar a escola"}`)
	a.do(200, "POST", "/sempras/casa-abrigo-pia-metas/"+meta+"/concluir", `{"concluida":true}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-pia-metas/"+meta, "")
	up := a.do(201, "POST", "/sempras/casa-abrigo/"+ac+"/documentos/uploads",
		`{"tipo":"acolhido","filename":"doc.pdf","content_type":"application/pdf","size":8}`)
	doc := enviar(t, c, up)
	a.do(200, "POST", "/sempras/casa-abrigo-documentos/"+doc+"/confirm", "")
	a.do(200, "GET", "/sempras/casa-abrigo-documentos/"+doc+"/download", "")
	a.do(200, "DELETE", "/sempras/casa-abrigo-documentos/"+doc, "")
	a.do(200, "POST", "/sempras/casa-abrigo/"+ac+"/desligar", `{"motivo":"reintegração familiar"}`)

	// Centro POP: abordagem social (com e sem cadastro).
	a.id("POST", "/sempras/unidades/"+pop+"/abordagens",
		`{"cidadao_id":"`+cid+`","local":"Praça Brasil","resultado":"aceitou_acolhimento","data_abordagem":"2026-01-01T10:00:00Z"}`)
	a.id("POST", "/sempras/unidades/"+pop+"/abordagens",
		`{"descricao_pessoa":"homem, uns 40 anos","local":"Rodoviária","resultado":"recusou","data_abordagem":"2026-01-01T11:00:00Z"}`)
	a.do(200, "GET", "/sempras/unidades/"+pop+"/abordagens?mes=2026-01", "")
	a.do(200, "GET", "/sempras/unidades/"+pop+"/abordagens", "")

	// Centro POP: perfil, encaminhamento externo, documentos.
	a.do(200, "PUT", "/sempras/cidadaos/"+cid+"/centro-pop-perfil",
		`{"unidade_id":"`+pop+`","situacao":"situacao_rua","motivo_vulnerabilidade":"desemprego"}`)
	ext := a.id("POST", "/sempras/cidadaos/"+cid+"/encaminhamentos-externos",
		`{"unidade_id":"`+pop+`","destino":"caps_ad","prioridade":"alta","motivo":"acompanhamento"}`)
	a.do(200, "POST", "/sempras/encaminhamentos-externos/"+ext+"/transitar", `{"status":"concluido"}`)
	upCp := a.do(201, "POST", "/sempras/cidadaos/"+cid+"/centro-pop-documentos/uploads",
		`{"unidade_id":"`+pop+`","tipo":"identificacao","filename":"rg.pdf","content_type":"application/pdf","size":8}`)
	docCp := enviar(t, c, upCp)
	a.do(200, "POST", "/sempras/centro-pop-documentos/"+docCp+"/confirm", "")
	a.do(200, "GET", "/sempras/centro-pop-documentos/"+docCp+"/download", "")
	a.do(200, "DELETE", "/sempras/centro-pop-documentos/"+docCp, "")
}

// enviar faz o PUT do arquivo direto no armazenamento (o que o navegador
// faria com a URL pré-assinada) e devolve o id do documento.
func enviar(t *testing.T, c cenario, resp map[string]any) string {
	t.Helper()
	key := resp["upload"].(map[string]any)["object_key"].(string)
	if err := c.store.Put(context.Background(), bucket, key, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
		t.Fatal(err)
	}
	return resp["documento"].(map[string]any)["id"].(string)
}
