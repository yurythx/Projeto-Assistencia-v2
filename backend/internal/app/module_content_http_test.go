package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type postResp struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	Status      string  `json:"status"`
	PublishedAt *string `json:"published_at"`
}

func outboxCount(t *testing.T, h *apiHarness, eventType, aggregateID string) int {
	t.Helper()
	var n int
	if err := h.d.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox_events WHERE event_type = $1 AND aggregate_id = $2`, eventType, aggregateID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestBlogHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, reader := h.user("nexus-user")
	title := "Comunicado " + uuid.NewString()[:8]

	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/blog/posts", reader, `{"title":"x"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":""}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":"x","kind":"fofoca"}`)

	p := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin,
		`{"title":"`+title+`","summary":"resumo","body":"# Olá","kind":"comunicado"}`))
	if p.Status != "draft" || p.Slug == "" {
		t.Fatalf("nasce rascunho com slug: %+v", p)
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":"outro","slug":"`+p.Slug+`"}`)

	// Rascunho é invisível para leitores — por id, por slug e na listagem.
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/blog/posts/"+p.Slug, reader, "")
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/blog/posts?status=draft", reader, "")
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts?page_size=100", reader, "").Body.String(); strings.Contains(body, p.ID) {
		t.Fatal("listagem de leitor não pode trazer rascunhos")
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts/"+p.ID, admin, "")

	// Publicar: estado, data e evento blog.post.published no outbox (mesma tx).
	pub := data[postResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/blog/posts/"+p.ID+"/publish", admin, ""))
	if pub.Status != "published" || pub.PublishedAt == nil {
		t.Fatalf("publicado: %+v", pub)
	}
	if outboxCount(t, h, "blog.post.published", p.ID) != 1 {
		t.Fatal("publicar deveria gravar exatamente um evento blog.post.published")
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/blog/posts/"+p.ID+"/publish", admin, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts/"+p.Slug, reader, "")
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts?q="+strings.ReplaceAll(title, " ", "+"), reader, "").Body.String(); !strings.Contains(body, p.ID) {
		t.Fatalf("busca textual deveria achar a publicação: %s", body)
	}

	// PUT substitui a publicação inteira: editar a publicada sem o texto
	// apagaria o conteúdo no ar — é recusado.
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/blog/posts/"+p.ID, admin, `{"title":"`+title+` (editado)","kind":"comunicado"}`)
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/blog/posts/"+p.ID, admin, `{"title":"`+title+` (editado)","body":"# Olá, editado","kind":"comunicado"}`)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/blog/posts/"+p.ID+"/archive", admin, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/blog/posts/"+p.Slug, reader, "")

	// Upload de capa: ticket só para imagens; capa não enviada é recusada.
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/uploads", admin, `{"filename":"capa.png","content_type":"image/png"}`)
	if rec := h.do(http.MethodPost, "/api/v1/blog/uploads", admin, `{"filename":"x.exe","content_type":"application/x-msdownload"}`); rec.Code < 400 {
		t.Fatalf("tipo não-imagem deveria ser recusado: %d", rec.Code)
	}
	if rec := h.do(http.MethodPut, "/api/v1/blog/posts/"+p.ID, admin, `{"title":"x","cover_object_key":"blog/covers/nao-existe.png"}`); rec.Code < 400 {
		t.Fatalf("capa inexistente no storage deveria ser recusada: %d", rec.Code)
	}

	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/blog/posts/"+p.ID, admin, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/blog/posts/"+p.ID, admin, "")
}

func TestCatalogHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, plain := h.user("nexus-user")
	title := "Emissão de certidão " + uuid.NewString()[:6]

	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/catalog/admin/services", plain, "")
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/catalog/admin/services", admin,
		`{"title":"x","channels":[{"type":"pombo","label":"a","value":"b"}]}`)
	svc := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/catalog/admin/services", admin,
		`{"title":"`+title+`","summary":"s","category":"Documentos","requirements":["RG"],"steps":["Pedir","Retirar"],
		  "channels":[{"type":"online","label":"Portal","value":"https://servicos.gov.br"}],"sla":"5 dias","cost":"Gratuito"}`))

	// Rascunho não aparece no site público (anônimo).
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/catalog/services/"+svc.Slug, "", "")
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/catalog/admin/services/"+svc.ID+"/publish", admin, "")
	pub := h.expect(http.StatusOK, http.MethodGet, "/api/v1/catalog/services/"+svc.Slug, "", "")
	if !strings.Contains(pub.Body.String(), `"requirements":["RG"]`) || pub.Header().Get("Cache-Control") == "" {
		t.Fatalf("serviço público com requisitos e cache: %s", pub.Body.String())
	}
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/catalog/categories", "", "").Body.String(); !strings.Contains(body, "Documentos") {
		t.Fatalf("categorias públicas: %s", body)
	}
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/catalog/services?category=Documentos", "", "").Body.String(); !strings.Contains(body, svc.ID) {
		t.Fatal("filtro por categoria no público")
	}
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/catalog/admin/services/"+svc.ID+"/archive", admin, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/catalog/services/"+svc.Slug, "", "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/catalog/admin/services/"+svc.ID, admin, "")
}

func TestContactHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, plain := h.user("nexus-user")
	msg := func(extra string) string {
		return `{"name":"Maria Silva","email":"maria@example.com","subject":"Dúvida sobre serviço","message":"Gostaria de saber o prazo.","category":"duvida"` + extra + `}`
	}

	// Anônimo: consentimento LGPD obrigatório.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/contact/messages", "", msg(`,"consent":false`))
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/contact/messages", "", `{"name":"x","email":"inválido","subject":"a","message":"b","consent":true}`)
	ok := data[struct {
		Protocol string `json:"protocol"`
	}](t, h.expect(http.StatusAccepted, http.MethodPost, "/api/v1/contact/messages", "", msg(`,"consent":true`)))
	if !strings.HasPrefix(ok.Protocol, "CT-") || ok.Protocol == "CT-RECEBIDO" {
		t.Fatalf("protocolo real esperado: %q", ok.Protocol)
	}
	// Honeypot: mesma resposta de sucesso, nada gravado.
	bot := data[struct {
		Protocol string `json:"protocol"`
	}](t, h.expect(http.StatusAccepted, http.MethodPost, "/api/v1/contact/messages", "", msg(`,"consent":true,"website":"http://spam"`)))
	var stored int
	_ = h.d.DB.QueryRow(context.Background(), `SELECT count(*) FROM contact_messages WHERE protocol = $1`, bot.Protocol).Scan(&stored)
	if stored != 0 {
		t.Fatal("mensagem de bot (honeypot) não pode ser gravada")
	}

	// Gestão exige contact:read / contact:manage.
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/contact/messages", plain, "")
	list := h.expect(http.StatusOK, http.MethodGet, "/api/v1/contact/messages?status=new", admin, "").Body.String()
	if !strings.Contains(list, ok.Protocol) {
		t.Fatalf("mensagem nova deveria estar na caixa: %s", list)
	}
	var id string
	if err := h.d.DB.QueryRow(context.Background(), `SELECT id FROM contact_messages WHERE protocol = $1`, ok.Protocol).Scan(&id); err != nil {
		t.Fatal(err)
	}
	detail := h.expect(http.StatusOK, http.MethodGet, "/api/v1/contact/messages/"+id, admin, "").Body.String()
	if !strings.Contains(detail, `"consent_at"`) {
		t.Fatalf("detalhe deveria registrar o consentimento: %s", detail)
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPatch, "/api/v1/contact/messages/"+id, admin, `{"status":"sumiu"}`)
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/contact/messages/"+id, admin, `{"status":"answered","notes":"respondido por e-mail"}`)
	h.expect(http.StatusForbidden, http.MethodPatch, "/api/v1/contact/messages/"+id, plain, `{"status":"archived"}`)

	// Encaminhado por setor (ADR 013, fase 3): a gestão da unidade A vê e
	// trata só o que a triagem encaminhou à área dela.
	o := h.orgEscopo(t)
	setorA := h.gestorEm(t, o.a, "contact:read", "contact:manage")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/contact/messages/"+id, setorA, "")
	h.expect(http.StatusUnprocessableEntity, http.MethodGet, "/api/v1/contact/messages?unidade_id=x", setorA, "")
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/contact/messages/"+id, admin, `{"status":"new","unidade_id":"`+o.sub+`"}`)
	if lista := h.expect(http.StatusOK, http.MethodGet, "/api/v1/contact/messages?unidade_id="+o.sub, setorA, "").Body.String(); !strings.Contains(lista, ok.Protocol) {
		t.Fatalf("encaminhada à subunidade de A aparece para A: %s", lista)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/contact/messages/"+id, setorA, "")
	h.expect(http.StatusForbidden, http.MethodPatch, "/api/v1/contact/messages/"+id, setorA, `{"status":"new","unidade_id":"`+o.b+`"}`)
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/contact/messages/"+id, setorA, `{"status":"answered"}`) // devolve à caixa geral
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/contact/messages/"+id, setorA, "")
	if outboxCount(t, h, "contact.message.submitted", id) != 1 {
		t.Fatal("envio deveria emitir contact.message.submitted no outbox")
	}
}
