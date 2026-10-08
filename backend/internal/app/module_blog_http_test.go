package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Capa: trocar ou excluir remove a imagem antiga do armazenamento; texto
// vazio não se publica; estados e entrada malformada.
func TestBlogCoverAndPublicationRules(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	ctx := context.Background()
	bucket := h.d.Config.MinIO.Bucket
	cover := func() string {
		tk := data[struct {
			ObjectKey string `json:"object_key"`
		}](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/uploads", admin, `{"filename":"capa.png","content_type":"image/png"}`))
		h.upload(tk.ObjectKey, "image/png", []byte("\x89PNG capa"))
		return tk.ObjectKey
	}
	c1, c2 := cover(), cover()
	title := "Aviso " + uuid.NewString()[:6]
	p := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":"`+title+`","cover_object_key":"`+c1+`"}`))
	if got := h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts/"+p.ID, admin, "").Body.String(); !strings.Contains(got, `"cover_url":"https://minio.test/`) {
		t.Fatalf("capa servida por URL temporária: %s", got)
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/blog/posts/"+p.ID+"/publish", admin, "")
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/blog/posts/"+p.ID, admin, `{"title":"`+title+`","body":"Texto","cover_object_key":"`+c2+`"}`)
	if _, err := h.d.Storage.Stat(ctx, bucket, c1); err == nil {
		t.Fatal("a capa substituída sai do armazenamento")
	}
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/blog/posts/"+p.ID+"/publish", admin, "")
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/blog/posts/"+p.ID+"/unpublish", admin, "")
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/blog/posts/"+p.ID+"/unpublish", admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/blog/posts/"+p.ID, admin, "")
	if _, err := h.d.Storage.Stat(ctx, bucket, c2); err == nil {
		t.Fatal("excluir a publicação remove a capa")
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/blog/posts/x", `{"title":"x"}`},
		{http.MethodPost, "/api/v1/blog/posts/x/publish", ""},
		{http.MethodDelete, "/api/v1/blog/posts/x", ""},
		{http.MethodPost, "/api/v1/blog/posts", `{`},
		{http.MethodPut, "/api/v1/blog/posts/" + p.ID, `{`},
		{http.MethodPost, "/api/v1/blog/uploads", `{`},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, admin, c.body)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/blog/posts/" + uuid.NewString(), `{"title":"Fantasma"}`},
		{http.MethodPost, "/api/v1/blog/posts/" + uuid.NewString() + "/publish", ""},
		{http.MethodDelete, "/api/v1/blog/posts/" + uuid.NewString(), ""},
		{http.MethodGet, "/api/v1/blog/posts/nao-existe-" + uuid.NewString()[:6], ""},
	} {
		h.expect(http.StatusNotFound, c.method, c.path, admin, c.body)
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/blog/posts", admin, `{"title":"???"}`)
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts?status=archived", admin, "")
}

// blog:manage com escopo (ADR 013): a gestão de A publica pela unidade A e
// subunidades; não por B nem institucional; rascunhos só no escopo.
func TestBlogGestaoComEscopo(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	o := h.orgEscopo(t)
	gestorA := h.gestorEm(t, o.a, "blog:manage")
	post := func(unidade string) string {
		body := `{"title":"Post ` + uuid.NewString()[:8] + `","summary":"s","body":"texto"`
		if unidade != "" {
			body += `,"unidade_id":"` + unidade + `"`
		}
		return body + `}`
	}
	deA := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", gestorA, post(o.a)))
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", gestorA, post(o.sub)) // herança
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/blog/posts", gestorA, post(o.b))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/blog/posts", gestorA, post(""))
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/blog/posts/"+deA.ID+"/publish", gestorA, "")
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/blog/posts/"+deA.ID, gestorA, post(o.b)) // não move para B

	institucional := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin, post("")))
	rascunhoB := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin, post(o.b)))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/blog/posts/"+institucional.ID+"/publish", gestorA, "")
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/blog/posts/"+rascunhoB.ID, gestorA, "")
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/blog/posts/"+rascunhoB.ID, gestorA, post(o.a)) // não traz de B para A
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/blog/posts/"+rascunhoB.ID, gestorA, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts/"+rascunhoB.ID, admin, "")
	if lista := h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts?status=draft&page_size=100", gestorA, "").Body.String(); strings.Contains(lista, rascunhoB.ID) || strings.Contains(lista, institucional.ID) {
		t.Fatalf("rascunhos fora do escopo não aparecem para a gestão de A: %s", lista)
	}
}

// Público-alvo (ADR 014): o post para a unidade A é lido por quem está em A
// ou abaixo; para a secretaria, por quem está em qualquer unidade dela.
// Fora do público: some da lista, do detalhe e da busca.
func TestBlogPublicoAlvo(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	o := h.orgEscopo(t)
	naSub := h.gestorEm(t, o.sub, "contact:read")
	emB := h.gestorEm(t, o.b, "contact:read")
	_, semLotacao := h.user("nexus-user")
	sfx := uuid.NewString()[:8]
	publicar := func(titulo, publico string) string {
		id := data[postResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/blog/posts", admin,
			`{"title":"`+titulo+` `+sfx+`","summary":"s","body":"texto `+sfx+`","publico":`+publico+`}`)).ID
		h.expect(http.StatusOK, http.MethodPost, "/api/v1/blog/posts/"+id+"/publish", admin, "")
		return id
	}
	deA := publicar("Para A", `{"unidades":["`+o.a+`"]}`)
	daSecretaria := publicar("Para a secretaria", `{"entidades":["`+o.ent+`"]}`)
	// Unidade que é público de algum conteúdo não é excluída (ADR 014).
	avulsa := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin, `{"entidade_id":"`+o.ent+`","nome":"Avulsa `+sfx+`"}`)).ID
	publicar("Para a avulsa", `{"unidades":["`+avulsa+`"]}`)
	h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/iam/unidades/"+avulsa, admin, "")
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/blog/posts", admin,
		`{"title":"Fantasma","summary":"s","body":"b","publico":{"unidades":["`+uuid.NewString()+`"]}}`)

	lista := func(tok string) string {
		return h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts?page_size=100&q="+sfx, tok, "").Body.String()
	}
	if l := lista(naSub); !strings.Contains(l, deA) || !strings.Contains(l, daSecretaria) {
		t.Fatalf("lotado na subunidade de A lê o de A e o da secretaria: %s", l)
	}
	if l := lista(emB); strings.Contains(l, deA) || !strings.Contains(l, daSecretaria) {
		t.Fatalf("lotado em B lê só o da secretaria: %s", l)
	}
	if l := lista(semLotacao); strings.Contains(l, deA) || strings.Contains(l, daSecretaria) {
		t.Fatalf("sem lotação não lê conteúdo com público: %s", l)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts/"+deA, naSub, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/blog/posts/"+deA, emB, "")
	if busca := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+sfx, emB, "").Body.String(); strings.Contains(busca, deA) || !strings.Contains(busca, daSecretaria) {
		t.Fatalf("busca global respeita o público: %s", busca)
	}
	detalhe := data[struct {
		Publico struct {
			Unidades []string `json:"unidades"`
		} `json:"publico"`
	}](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/blog/posts/"+deA, admin, ""))
	if len(detalhe.Publico.Unidades) != 1 || detalhe.Publico.Unidades[0] != o.a {
		t.Fatalf("detalhe traz o público: %+v", detalhe)
	}
}
