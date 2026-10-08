package app

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDirectoryHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	aliceID, alice := h.user("nexus-user")
	_, bob := h.user("nexus-user")

	// Autoatendimento: a própria ficha.
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/me", alice,
		`{"job_title":"Analista","phone":"(65) 99999-0000","extension":"1234","bio":"olá","visible":true}`)
	me := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/me", alice, "").Body.String()
	if !strings.Contains(me, `"job_title":"Analista"`) {
		t.Fatalf("ficha própria: %s", me)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people/"+aliceID.String(), bob, "")
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/directory/me", alice, `{"extension":"1234567890123456789"}`)

	// Ficha oculta some para colegas, mas não para o dono nem para a gestão.
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/me", alice, `{"job_title":"Analista","visible":false}`)
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/directory/people/"+aliceID.String(), bob, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people/"+aliceID.String(), alice, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people/"+aliceID.String(), admin, "")
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/people?page_size=100&q=Usu", bob, "").Body.String(); strings.Contains(body, aliceID.String()) {
		t.Fatal("pessoa oculta não pode aparecer na listagem de colegas")
	}

	// Edição da ficha alheia exige directory:manage.
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/directory/people/"+aliceID.String(), bob, `{"job_title":"Hacker"}`)
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/directory/people/"+aliceID.String(), admin, `{"job_title":"Coordenadora","visible":true}`)

	// Setores: autenticado e público (anônimo).
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/sectors", bob, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/directory/public/sectors", "", "")
}

type eventResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func TestCalendarHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, ana := h.user("nexus-user")
	_, beto := h.user("nexus-user")
	base := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Hour)
	iso := func(d time.Duration) string { return base.Add(d).Format(time.RFC3339) }
	window := "from=" + base.Add(-time.Hour).Format(time.RFC3339) + "&to=" + base.Add(24*time.Hour).Format(time.RFC3339)
	window = strings.ReplaceAll(window, "+", "%2B")

	// Salas: gestão exige calendar:manage.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/calendar/rooms", ana, `{"name":"Sala"}`)
	room := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/calendar/rooms", admin,
		`{"name":"Sala `+uuid.NewString()[:5]+`","capacity":10,"resources":["projetor"]}`))

	// Intervalo inválido.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/calendar/events", ana,
		`{"title":"Ruim","starts_at":"`+iso(time.Hour)+`","ends_at":"`+iso(0)+`"}`)

	// Reserva e conflito de sala.
	ev := data[eventResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/calendar/events", ana,
		`{"title":"Reunião","starts_at":"`+iso(0)+`","ends_at":"`+iso(time.Hour)+`","room_id":"`+room.ID+`","visibility":"internal"}`))
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/calendar/events", beto,
		`{"title":"Choque","starts_at":"`+iso(30*time.Minute)+`","ends_at":"`+iso(90*time.Minute)+`","room_id":"`+room.ID+`"}`)
	busy := h.expect(http.StatusOK, http.MethodGet, "/api/v1/calendar/rooms/"+room.ID+"/busy?"+window, beto, "").Body.String()
	if !strings.Contains(busy, ev.ID) {
		t.Fatalf("ocupação da sala deveria listar a reserva: %s", busy)
	}

	// Só o organizador (ou a gestão) altera/cancela.
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/calendar/events/"+ev.ID, beto,
		`{"title":"Tomado","starts_at":"`+iso(0)+`","ends_at":"`+iso(time.Hour)+`"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/calendar/events/"+ev.ID+"/cancel", beto, "")
	c := data[eventResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/calendar/events/"+ev.ID+"/cancel", ana, ""))
	if c.Status != "cancelled" {
		t.Fatalf("cancelado: %+v", c)
	}
	// Cancelar libera a sala.
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/calendar/events", beto,
		`{"title":"Agora cabe","starts_at":"`+iso(30*time.Minute)+`","ends_at":"`+iso(90*time.Minute)+`","room_id":"`+room.ID+`"}`)

	// Visibilidade: privado só para o organizador; público aparece no site sem organizador.
	priv := data[eventResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/calendar/events", ana,
		`{"title":"Médico","starts_at":"`+iso(3*time.Hour)+`","ends_at":"`+iso(4*time.Hour)+`","visibility":"private"}`))
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/calendar/events/"+priv.ID, beto, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/calendar/events/"+priv.ID, ana, "")
	pub := data[eventResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/calendar/events", ana,
		`{"title":"Audiência pública","starts_at":"`+iso(5*time.Hour)+`","ends_at":"`+iso(6*time.Hour)+`","visibility":"public"}`))
	public := h.expect(http.StatusOK, http.MethodGet, "/api/v1/calendar/public/events?"+window, "", "").Body.String()
	if !strings.Contains(public, pub.ID) || strings.Contains(public, priv.ID) || strings.Contains(public, "organizer_name\":\"Usu") {
		t.Fatalf("agenda pública: só eventos públicos e sem dados do organizador: %s", public)
	}
	if list := h.expect(http.StatusOK, http.MethodGet, "/api/v1/calendar/events?"+window, beto, "").Body.String(); strings.Contains(list, priv.ID) {
		t.Fatal("evento privado de outra pessoa não pode aparecer na agenda")
	}
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/calendar/events/"+pub.ID, ana, "")
}

type folderResp struct {
	ID string `json:"id"`
}

func TestFilesHTTP(t *testing.T) {
	h := newHarness(t)
	ownerID, owner := h.user("nexus-user")
	_, other := h.user("nexus-user")
	_ = ownerID

	root := data[folderResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/files/folders", owner, `{"name":"Equipe `+uuid.NewString()[:5]+`"}`))
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/files/folders", owner, `{"name":"../etc"}`)

	// Pasta sem ACL: privada ao dono.
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/files/browse?folder_id="+root.ID, other, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/browse?folder_id="+root.ID, owner, "")

	// Upload direto: ticket -> PUT no storage -> confirmação (tamanho/tipo conferidos).
	pdf := []byte("%PDF-1.4\n% documento de teste\n")
	up := data[struct {
		File struct {
			ID string `json:"id"`
		} `json:"file"`
		Upload struct {
			ObjectKey string `json:"object_key"`
			UploadURL string `json:"upload_url"`
		} `json:"upload"`
	}](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/files/uploads", owner,
		`{"folder_id":"`+root.ID+`","filename":"relatorio.pdf","content_type":"application/pdf","size":`+strconv.Itoa(len(pdf))+`}`))
	if up.Upload.UploadURL == "" || !strings.Contains(up.Upload.ObjectKey, "relatorio") && up.Upload.ObjectKey == "" {
		t.Fatalf("ticket de upload: %+v", up)
	}
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/files/uploads", other,
		`{"folder_id":"`+root.ID+`","filename":"x.pdf","content_type":"application/pdf","size":10}`)
	if rec := h.do(http.MethodPost, "/api/v1/files/objects/"+up.File.ID+"/confirm", owner, ""); rec.Code < 400 {
		t.Fatalf("confirmar sem o objeto no storage deveria falhar: %d", rec.Code)
	}
	h.upload(up.Upload.ObjectKey, "application/pdf", pdf)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/files/objects/"+up.File.ID+"/confirm", owner, "")
	if outboxCount(t, h, "files.object.uploaded", up.File.ID) != 1 {
		t.Fatal("confirmação deveria emitir files.object.uploaded")
	}
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/files/objects/"+up.File.ID+"/download", other, "")
	dl := h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/objects/"+up.File.ID+"/download", owner, "").Body.String()
	if !strings.Contains(dl, `"url"`) {
		t.Fatalf("download devolve URL temporária: %s", dl)
	}

	// ACL: somente leitura para todos; escrita continua negada.
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/files/folders/"+root.ID+"/acl", other, `{"entries":[{"subject_type":"everyone"}]}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/files/folders/"+root.ID+"/acl", owner, `{"entries":[{"subject_type":"user","subject":"não-é-uuid"}]}`)
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/files/folders/"+root.ID+"/acl", owner, `{"entries":[{"subject_type":"everyone","can_write":false}]}`)
	listing := h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/browse?folder_id="+root.ID, other, "").Body.String()
	if !strings.Contains(listing, up.File.ID) || !strings.Contains(listing, `"write":false`) {
		t.Fatalf("leitura liberada, escrita não: %s", listing)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/objects/"+up.File.ID+"/download", other, "")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/files/folders", other, `{"name":"Intruso","parent_id":"`+root.ID+`"}`)

	// Herança: subpasta herda a ACL da mãe.
	sub := data[folderResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/files/folders", owner, `{"name":"Sub","parent_id":"`+root.ID+`"}`))
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/browse?folder_id="+sub.ID, other, "")

	// Pasta com conteúdo só sai com confirmação explícita (recursive=true),
	// e aí leva os objetos do storage junto.
	rec := h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/files/folders/"+root.ID, owner, "")
	if !strings.Contains(rec.Body.String(), "FOLDER_NOT_EMPTY") {
		t.Fatalf("código FOLDER_NOT_EMPTY esperado: %s", rec.Body.String())
	}
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/files/folders/"+sub.ID, owner, "")
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/files/objects/"+up.File.ID, other, "")
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/files/folders/"+root.ID+"?recursive=true", other, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/files/folders/"+root.ID+"?recursive=true", owner, "")
	if _, err := h.d.Storage.Stat(t.Context(), h.d.Config.MinIO.Bucket, up.Upload.ObjectKey); err == nil {
		t.Fatal("exclusão recursiva deveria remover o objeto do storage")
	}
}

type wikiResp struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Version int    `json:"version"`
}

func TestWikiHTTP(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	_, ana := h.user("nexus-user")
	_, beto := h.user("nexus-user")
	title := "Manual " + uuid.NewString()[:6]

	p := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", ana, `{"title":"`+title+`","body":"versão 1"}`))
	child := data[wikiResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/wiki/pages", beto, `{"title":"Capítulo","parent_id":"`+p.ID+`"}`))
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/wiki/pages/"+child.Slug, ana, "").Body.String(); !strings.Contains(body, `"breadcrumbs"`) || !strings.Contains(body, p.ID) {
		t.Fatalf("subpágina com trilha: %s", body)
	}

	// Concorrência otimista: dois editores abrem a v1; o segundo a salvar perde.
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/wiki/pages/"+p.ID, ana, `{"title":"`+title+`","body":"sem versão"}`)
	v2 := data[wikiResp](t, h.expect(http.StatusOK, http.MethodPut, "/api/v1/wiki/pages/"+p.ID, ana, `{"title":"`+title+`","body":"versão 2","version":1,"summary":"ana"}`))
	if v2.Version != 2 {
		t.Fatalf("versão incrementada: %+v", v2)
	}
	rec := h.expect(http.StatusConflict, http.MethodPut, "/api/v1/wiki/pages/"+p.ID, beto, `{"title":"`+title+`","body":"beto","version":1}`)
	if !strings.Contains(rec.Body.String(), "WIKI_STALE_VERSION") {
		t.Fatalf("conflito de versão identificado: %s", rec.Body.String())
	}

	// Histórico e restauração geram uma nova versão.
	revs := h.expect(http.StatusOK, http.MethodGet, "/api/v1/wiki/pages/"+p.ID+"/revisions", beto, "").Body.String()
	if strings.Count(revs, `"version"`) < 2 {
		t.Fatalf("duas revisões esperadas: %s", revs)
	}
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/wiki/pages/"+p.ID+"/revisions/1", beto, "")
	restored := data[wikiResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/wiki/pages/"+p.ID+"/revisions/1/restore", beto, ""))
	if restored.Version != 3 {
		t.Fatalf("restaurar cria a v3: %+v", restored)
	}
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/wiki/pages/"+p.ID, ana, "").Body.String(); !strings.Contains(body, "versão 1") {
		t.Fatalf("conteúdo restaurado: %s", body)
	}
	// Uma página não pode virar filha de si mesma.
	if rec := h.do(http.MethodPut, "/api/v1/wiki/pages/"+p.ID, ana, `{"title":"`+title+`","parent_id":"`+p.ID+`","version":3}`); rec.Code < 400 {
		t.Fatalf("ciclo na árvore deveria ser recusado: %d", rec.Code)
	}

	// Excluir exige wiki:manage e só sem subpáginas.
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/wiki/pages/"+child.ID, ana, "")
	h.expect(http.StatusConflict, http.MethodDelete, "/api/v1/wiki/pages/"+p.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/wiki/pages/"+child.ID, admin, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/wiki/pages/"+p.ID, admin, "")
	if tree := h.expect(http.StatusOK, http.MethodGet, "/api/v1/wiki/tree", ana, "").Body.String(); strings.Contains(tree, p.ID) {
		t.Fatal("página excluída não pode continuar na árvore")
	}
}
