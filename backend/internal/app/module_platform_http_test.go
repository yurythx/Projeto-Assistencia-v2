package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestMercurioHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	anaID, ana := h.user("nexus-user")
	betoID, beto := h.user("nexus-user")
	_, carla := h.user("nexus-user")

	// Canal global semeado é visível a todos.
	var geral string
	for _, r := range data[[]struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/mercurio/rooms", ana, "")) {
		if r.Kind == "global" {
			geral = r.ID
		}
	}
	if geral == "" {
		t.Fatal("canal global esperado")
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/mercurio/rooms/"+geral+"/messages", ana, `{"body":""}`)
	msg := data[struct {
		ID string `json:"id"`
	}](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms/"+geral+"/messages", ana, `{"body":"Bom dia!"}`))
	if outboxCount(t, h, "mercurio.message.created", msg.ID) != 1 {
		t.Fatal("mensagem deveria emitir mercurio.message.created")
	}

	// Não lidas: conta para Beto até ele marcar como lida.
	unread := func(tok string) string {
		return h.expect(http.StatusOK, http.MethodGet, "/api/v1/mercurio/rooms", tok, "").Body.String()
	}
	if !strings.Contains(unread(beto), `"unread":`) {
		t.Fatal("contador de não lidas esperado")
	}
	h.expect(http.StatusNoContent, http.MethodPost, "/api/v1/mercurio/rooms/"+geral+"/read", beto, "")

	// Edição só pelo autor; remoção pelo autor ou moderação.
	h.expect(http.StatusForbidden, http.MethodPatch, "/api/v1/mercurio/messages/"+msg.ID, beto, `{"body":"hackeado"}`)
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/mercurio/messages/"+msg.ID, ana, `{"body":"Bom dia, equipe!"}`)
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/mercurio/messages/"+msg.ID, beto, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/mercurio/messages/"+msg.ID, admin, "")
	hist := h.expect(http.StatusOK, http.MethodGet, "/api/v1/mercurio/rooms/"+geral+"/messages?limit=100", beto, "").Body.String()
	if strings.Contains(hist, "Bom dia, equipe!") {
		t.Fatal("mensagem removida não pode expor o conteúdo no histórico")
	}

	// Conversa direta: só os dois participantes; é idempotente.
	dm := data[idResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/mercurio/direct", ana, `{"user_id":"`+betoID.String()+`"}`))
	again := data[idResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/mercurio/direct", beto, `{"user_id":"`+anaID.String()+`"}`))
	if dm.ID != again.ID {
		t.Fatal("a conversa direta entre as mesmas duas pessoas é única")
	}
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms/"+dm.ID+"/messages", beto, `{"body":"oi, Ana"}`)
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/mercurio/rooms/"+dm.ID+"/messages", carla, "")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/mercurio/rooms/"+dm.ID+"/messages", carla, `{"body":"intrusa"}`)

	// Salas: gestão exige mercurio:manage; sala arquivada é somente leitura.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/mercurio/rooms", ana, `{"kind":"global","name":"x"}`)
	room := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms", admin, `{"kind":"global","name":"Avisos `+uuid.NewString()[:5]+`"}`))
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/mercurio/rooms/"+room.ID, admin, `{"kind":"global","name":"Avisos (arquivo)","archived":true}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/mercurio/rooms/"+room.ID+"/messages", admin, `{"body":"ainda dá?"}`)
}

func TestEgressHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, plain := h.user("nexus-user")

	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/egress/targets", plain, "")

	// A10 — SSRF: destinos internos recusados no cadastro.
	for _, url := range []string{
		"https://127.0.0.1/hook", "https://10.1.2.3/hook", "https://192.168.0.10/x", "https://169.254.169.254/latest/meta-data",
		"https://localhost/hook", "http://8.8.8.8/sem-tls", "ftp://8.8.8.8/x", "https://user:pass@8.8.8.8/x",
	} {
		rec := h.do(http.MethodPost, "/api/v1/egress/targets", admin, `{"name":"x","kind":"webhook","url":"`+url+`"}`)
		if rec.Code < 400 {
			t.Errorf("destino %s deveria ser recusado (SSRF), veio %d", url, rec.Code)
		}
	}

	target := data[struct {
		ID        string `json:"id"`
		HasSecret bool   `json:"has_secret"`
	}](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/egress/targets", admin,
		`{"name":"n8n","kind":"n8n","url":"https://8.8.8.8/webhook","secret":"s3gredo-hmac","event_patterns":["blog.*"]}`))
	if !target.HasSecret {
		t.Fatal("segredo salvo (e nunca devolvido em claro)")
	}
	list := h.expect(http.StatusOK, http.MethodGet, "/api/v1/egress/targets", admin, "").Body.String()
	if strings.Contains(list, "s3gredo-hmac") {
		t.Fatal("o segredo HMAC nunca pode sair na API")
	}
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/egress/targets/"+target.ID, admin,
		`{"name":"n8n","kind":"n8n","url":"https://8.8.8.8/webhook","event_patterns":["blog.*","tramite.*"],"active":false}`)
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/egress/deliveries?status=dead", admin, "")
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/egress/deliveries/"+uuid.NewString()+"/redeliver", admin, "")
	// Teste de conectividade: o 8.8.8.8 não é um webhook — responde com
	// falha, mas o teste em si é 200 com o resultado.
	if res := h.expect(http.StatusOK, http.MethodPost, "/api/v1/egress/targets/"+target.ID+"/test", admin, "").Body.String(); !strings.Contains(res, `"ok":`) {
		t.Fatalf("resultado do teste: %s", res)
	}
	// Reentrega de uma entrega morta.
	var delID string
	if err := h.d.DB.QueryRow(context.Background(), `INSERT INTO egress_deliveries (target_id, event_id, event_type, payload, status)
		VALUES ($1, gen_random_uuid(), 'blog.post.published', '{}', 'dead') RETURNING id`, target.ID).Scan(&delID); err != nil {
		t.Fatal(err)
	}
	h.expect(http.StatusNoContent, http.MethodPost, "/api/v1/egress/deliveries/"+delID+"/redeliver", admin, "")
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/egress/targets/x", `{"name":"x","kind":"webhook","url":"https://8.8.8.8/x"}`},
		{http.MethodDelete, "/api/v1/egress/targets/x", ""},
		{http.MethodPost, "/api/v1/egress/targets/x/test", ""},
		{http.MethodPost, "/api/v1/egress/deliveries/x/redeliver", ""},
		{http.MethodGet, "/api/v1/egress/deliveries?target_id=x", ""},
		{http.MethodPost, "/api/v1/egress/targets", `{`},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, admin, c.body)
	}
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/egress/targets/"+uuid.NewString()+"/test", admin, "")
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/egress/targets/"+uuid.NewString(), admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/egress/targets/"+target.ID, admin, "")
}

func TestSearchHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, reader := h.user("nexus-user")
	term := "Zircônio" + strings.ReplaceAll(uuid.NewString()[:6], "-", "")

	post := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":"Notícia `+term+`","summary":"s","body":"Texto da notícia"}`))
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/blog/posts/"+post.ID+"/publish", admin, "")
	page := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", reader, `{"title":"Página `+term+`","body":"corpo"}`))

	h.expect(http.StatusUnprocessableEntity, http.MethodGet, "/api/v1/search?q=a", reader, "")
	res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+term, reader, "").Body.String()
	if !strings.Contains(res, post.ID) || !strings.Contains(res, page.ID) {
		t.Fatalf("busca deveria achar blog e wiki: %s", res)
	}
	if only := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+term+"&module=wiki", reader, "").Body.String(); strings.Contains(only, post.ID) {
		t.Fatal("filtro por módulo")
	}

	// Degradação: módulo desativado some da busca na hora.
	h.setModule(admin, "wiki", false)
	res = h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+term, reader, "").Body.String()
	if strings.Contains(res, page.ID) || !strings.Contains(res, post.ID) || strings.Contains(res, `"wiki"`) {
		t.Fatalf("wiki desativada não pode aparecer na busca: %s", res)
	}
	// Rascunho nunca aparece para leitores.
	draft := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":"Rascunho `+term+`"}`))
	if res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+term, reader, "").Body.String(); strings.Contains(res, draft.ID) {
		t.Fatal("rascunho vazou na busca")
	}
}

// Busca por prefixo (nexus_search_tsquery): quem digita o começo das
// palavras acha o conteúdo — antes "vacina" não achava "vacinação" —,
// vários prefixos são E, e consulta com operador mantém o sentido.
func TestSearchPrefixHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, reader := h.user("nexus-user")
	term := "Zircônio" + strings.ReplaceAll(uuid.NewString()[:6], "-", "")

	post := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":"Campanha de vacinação `+term+`","summary":"s","body":"b"}`))
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/blog/posts/"+post.ID+"/publish", admin, "")
	page := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", reader, `{"title":"Rotina `+term+`","body":"corpo"}`))

	partial := term[:len(term)-3]
	res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+partial, reader, "").Body.String()
	if !strings.Contains(res, post.ID) || !strings.Contains(res, page.ID) {
		t.Fatalf("prefixo %q deveria achar blog e wiki: %s", partial, res)
	}
	res = h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q=vacina+"+partial, reader, "").Body.String()
	if !strings.Contains(res, post.ID) || strings.Contains(res, page.ID) {
		t.Fatalf("prefixos combinados (E) — só o post tem 'vacina…': %s", res)
	}
	res = h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+term+"+-campanha", reader, "").Body.String()
	if strings.Contains(res, post.ID) || !strings.Contains(res, page.ID) {
		t.Fatalf("exclusão (-campanha) mantém o sentido: %s", res)
	}
}

func TestAuditHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, plain := h.user("nexus-user")
	marker := "audit-" + uuid.NewString()[:8]
	post := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":"`+marker+`"}`))

	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/audit/logs", plain, "")
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/audit/verify", plain, "")
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/audit/export", plain, "")

	logs := h.expect(http.StatusOK, http.MethodGet, "/api/v1/audit/logs?resource_type=blog_post&resource_id="+post.ID, admin, "").Body.String()
	for _, want := range []string{`"action":"blog.post.created"`, `"hash":"`, `"prev_hash":"`, `"ip_address":"203.0.113.10"`, `"user_agent":"nexus-test/1.0"`, `"correlation_id":"`} {
		if !strings.Contains(logs, want) {
			t.Fatalf("registro de auditoria deveria ter %s: %s", want, logs)
		}
	}
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/audit/logs?from=ontem", admin, "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/audit/logs?actor_id=xyz", admin, "")

	verify := h.expect(http.StatusOK, http.MethodGet, "/api/v1/audit/verify", admin, "").Body.String()
	if !strings.Contains(verify, `"valid":true`) {
		t.Fatalf("cadeia íntegra: %s", verify)
	}

	for format, ctype := range map[string]string{"csv": "text/csv", "json": "application/json", "xml": "xml"} {
		rec := h.expect(http.StatusOK, http.MethodGet, "/api/v1/audit/export?format="+format+"&action=blog.post.created", admin, "")
		if !strings.Contains(rec.Header().Get("Content-Type"), ctype) {
			t.Errorf("export %s: content-type %q", format, rec.Header().Get("Content-Type"))
		}
	}
	// O próprio acesso à exportação é auditado (LAI).
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/audit/logs?action=audit.exported", admin, "").Body.String(); !strings.Contains(body, "audit.exported") {
		t.Fatal("exportação deveria gerar audit.exported")
	}
}

func TestLGPDAndTransparencyHTTP(t *testing.T) {
	h := newHarness(t)
	_, tok := h.user("nexus-user")

	status := h.expect(http.StatusOK, http.MethodGet, "/api/v1/lgpd/status", tok, "").Body.String()
	if !strings.Contains(status, "accepted") {
		t.Fatalf("status do consentimento: %s", status)
	}
	// Sem versão explícita, registra o aceite da versão vigente dos termos.
	acc := h.expect(http.StatusOK, http.MethodPost, "/api/v1/lgpd/accept", tok, `{}`).Body.String()
	if !strings.Contains(acc, `"term_version":"`) {
		t.Fatalf("aceite registra a versão: %s", acc)
	}

	export := h.expect(http.StatusOK, http.MethodGet, "/api/v1/lgpd/meus-dados", tok, "").Body.String()
	for _, want := range []string{`"account"`, `"consents"`, `"audit_trail"`} {
		if !strings.Contains(export, want) {
			t.Fatalf("pacote do titular deveria conter %s", want)
		}
	}
	if strings.Contains(export, "password_hash") || strings.Contains(export, "$argon2") {
		t.Fatal("o pacote do titular nunca expõe o hash da senha")
	}
	first := h.expect(http.StatusAccepted, http.MethodPost, "/api/v1/lgpd/solicitar-exclusao", tok, "").Body.String()
	second := h.expect(http.StatusOK, http.MethodPost, "/api/v1/lgpd/solicitar-exclusao", tok, "").Body.String()
	if !strings.Contains(first, `"pending"`) || !strings.Contains(second, "em andamento") {
		t.Fatalf("pedido de exclusão idempotente: %s / %s", first, second)
	}
	if list := h.expect(http.StatusOK, http.MethodGet, "/api/v1/lgpd/minhas-solicitacoes", tok, "").Body.String(); !strings.Contains(list, "erasure") {
		t.Fatalf("acompanhamento: %s", list)
	}

	// Consentimento anônimo (visitante) e transparência ativa: públicos.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/lgpd/accept-anon", "", `{"device_hash":"curto"}`)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/lgpd/accept-anon", "", `{"device_hash":"`+strings.Repeat("a", 32)+`","term_version":"2026-09"}`)
	for _, path := range []string{"datasets", "plataforma", "modulos", "auditoria/acoes"} {
		h.expect(http.StatusOK, http.MethodGet, "/api/v1/transparencia/"+path, "", "")
	}
	if csv := h.do(http.MethodGet, "/api/v1/transparencia/plataforma?format=csv", "", ""); !strings.Contains(csv.Header().Get("Content-Type"), "csv") {
		t.Fatalf("dados abertos em CSV: %q", csv.Header().Get("Content-Type"))
	}
	// Sem PII nos dados abertos.
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/transparencia/modulos", "", "").Body.String(); strings.Contains(body, "@nexus.test") {
		t.Fatal("dados abertos não podem conter dados pessoais")
	}
}

func TestExampleBlueprintHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, plain := h.user("nexus-user")
	h.setModule(admin, "example", true)

	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/examples", plain, `{"title":"x"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/examples", admin, `{"title":""}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/examples", admin, `{"title":"   "}`) // passa no validate, o domínio recusa
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/examples", admin, `{"title":"x","campo_desconhecido":1}`)
	item := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/examples", admin, `{"title":"Blueprint","description":"d"}`))
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/examples/"+item.ID, plain, "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/examples/nao-uuid", plain, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/examples/"+uuid.NewString(), plain, "")
	if list := h.expect(http.StatusOK, http.MethodGet, "/api/v1/examples", plain, "").Body.String(); !strings.Contains(list, item.ID) {
		t.Fatal("listagem do blueprint")
	}
	if outboxCount(t, h, "example.item.created", item.ID) != 1 {
		t.Fatal("blueprint grava o evento no outbox na mesma transação")
	}
}

func TestOutboxMonitoringHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, plain := h.user("nexus-user")
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/monitoring/outbox-stats", plain, "")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/monitoring/outbox/requeue", plain, "")
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/monitoring/outbox-stats", admin, "").Body.String(); !strings.Contains(body, `"pending"`) {
		t.Fatalf("estatísticas do outbox: %s", body)
	}
	if body := h.expect(http.StatusOK, http.MethodPost, "/api/v1/monitoring/outbox/requeue", admin, "").Body.String(); !strings.Contains(body, `"requeued"`) {
		t.Fatalf("reprocessamento: %s", body)
	}
}
