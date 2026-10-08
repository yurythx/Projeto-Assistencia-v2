package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Conexões de IA (ADR 020): só ia:manage configura; a conexão é testada
// antes de gravar; a chave nunca volta pela API nem vai para a auditoria;
// fornecedor externo exige autorização; o assistente do Atlas passa a
// responder pela conexão configurada, com dados pessoais mascarados.
func TestIAConfiguracaoHTTP(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	admin := h.admin()
	_, comum := h.user("nexus-user")
	leitor := h.globalCom(t, "atlas:read")
	t.Cleanup(func() {
		_, _ = h.d.DB.Exec(ctx, `DELETE FROM ia_uso`)
		_, _ = h.d.DB.Exec(ctx, `DELETE FROM ia_conexoes`)
	})

	var mu sync.Mutex
	var perguntas []string
	fornecedor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		perguntas = append(perguntas, body.Messages[len(body.Messages)-1].Content)
		mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer sk-teste-1234567890" {
			http.Error(w, "chave", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Resposta redigida pela IA."}}]}`))
	}))
	defer fornecedor.Close()

	h.expect(http.StatusUnauthorized, http.MethodGet, "/api/v1/ia/conexoes", "", "")
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/ia/conexoes", comum, "")
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/ia/provedores", admin, "").Body.String(); !strings.Contains(body, `"gemini"`) {
		t.Fatalf("catálogo: %s", body)
	}

	conexao := `{"nome":"Fornecedor de teste","provedor":"compativel","endpoint":"` + fornecedor.URL + `","modelo":"modelo-x","externo":true`
	// Chave errada: o teste falha e nada é gravado.
	if body := h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/ia/conexoes", admin, conexao+`,"chave":"errada"}`).Body.String(); !strings.Contains(body, "chave de API recusada") {
		t.Fatalf("teste com chave errada: %s", body)
	}
	criada := data[struct {
		ID         string `json:"id"`
		Externo    bool   `json:"externo"`
		TemChave   bool   `json:"tem_chave"`
		ChaveFinal string `json:"chave_final"`
	}](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/ia/conexoes", admin, conexao+`,"chave":"sk-teste-1234567890"}`))
	if !criada.Externo || !criada.TemChave || criada.ChaveFinal != "7890" {
		t.Fatalf("conexão criada: %+v", criada)
	}
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/ia/conexoes", admin, "").Body.String(); strings.Contains(body, "sk-teste") {
		t.Fatalf("chave devolvida pela API: %s", body)
	}
	var vazou int
	if err := h.d.DB.QueryRow(ctx, `SELECT count(*) FROM audit_logs a WHERE action LIKE 'ia.%' AND row_to_json(a)::text LIKE '%sk-teste%'`).Scan(&vazou); err != nil || vazou != 0 {
		t.Fatalf("chave na auditoria: %d %v", vazou, err)
	}

	// Externo sem autorização: 422; com autorização, o Atlas usa a IA.
	uso := `{"principal_id":"` + criada.ID + `","mascarar_dados_pessoais":true`
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/ia/uso/atlas.assistente", admin, uso+`}`)
	if body := h.expect(http.StatusOK, http.MethodPut, "/api/v1/ia/uso/atlas.assistente", admin, uso+`,"autorizo_envio_externo":true}`).Body.String(); !strings.Contains(body, `"configurado":true`) {
		t.Fatalf("uso: %s", body)
	}
	r := data[atlasResposta](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/chat", leitor,
		`{"query":"Qual o prazo de guarda dos processos de pregão eletrônico? Meu CPF é 123.456.789-09"}`))
	if r.Mode != "ia" || r.Answer != "Resposta redigida pela IA." {
		t.Fatalf("assistente pela IA configurada: %+v", r)
	}
	mu.Lock()
	ultima := perguntas[len(perguntas)-1]
	mu.Unlock()
	if strings.Contains(ultima, "123.456.789-09") || !strings.Contains(ultima, "[CPF]") || !strings.Contains(ultima, "CONTEXTO HOMOLOGADO") {
		t.Fatalf("pergunta enviada ao fornecedor externo: %s", ultima)
	}

	// Em uso, não exclui; desligada a IA, o assistente volta à síntese.
	h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/ia/conexoes/"+criada.ID, admin, "")
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/ia/uso/atlas.assistente", admin, `{}`)
	if r := data[atlasResposta](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/atlas/chat", leitor,
		`{"query":"Qual o prazo de guarda dos processos de pregão eletrônico?"}`)); r.Mode != "sintese" {
		t.Fatalf("IA desligada: %+v", r)
	}
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/ia/conexoes/"+criada.ID, admin, "")
}
