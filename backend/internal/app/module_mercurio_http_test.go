package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Salas departamentais (por departamento e por grupo do AD), janela de
// edição, sala arquivada somente leitura e quem perde acesso à sala.
func TestMercurioRoomsAndMessageRules(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	ctx := context.Background()
	sfx := uuid.NewString()[:6]
	ent := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"Órgão `+sfx+`"}`))
	un := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin, `{"entidade_id":"`+ent.ID+`","nome":"Unidade `+sfx+`"}`))
	dep := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", admin, `{"unidade_id":"`+un.ID+`","nome":"TI `+sfx+`"}`))
	var servidor string
	if err := h.d.DB.QueryRow(ctx, `SELECT id FROM perfis WHERE slug = 'servidor'`).Scan(&servidor); err != nil {
		t.Fatal(err)
	}
	anaID, ana := h.user("nexus-user")
	lotacao := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+anaID.String()+"/lotacoes", admin,
		`{"perfil_id":"`+servidor+`","departamento_id":"`+dep.ID+`"}`))
	grupoID, _ := h.user("nexus-user")
	if _, err := h.d.DB.Exec(ctx, `UPDATE users SET groups = ARRAY['grp_ti_`+sfx+`'] WHERE id = $1`, grupoID); err != nil {
		t.Fatal(err)
	}
	_, beto := h.user("nexus-user")

	// Validação na criação da sala.
	saveRoom := func(want int, body string) {
		h.expect(want, http.MethodPost, "/api/v1/mercurio/rooms", admin, body)
	}
	saveRoom(http.StatusUnprocessableEntity, `{"kind":"direct","name":"x"}`)
	saveRoom(http.StatusUnprocessableEntity, `{"kind":"department","name":"Sem vínculo"}`)
	saveRoom(http.StatusUnprocessableEntity, `{"kind":"department","name":"Dep fantasma","departamento_id":"`+uuid.NewString()+`"}`)
	saveRoom(http.StatusBadRequest, `{`)
	sala := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms", admin,
		`{"kind":"department","name":"TI `+sfx+`","departamento_id":"`+dep.ID+`","ad_group":"GRP_TI_`+sfx+`"}`))
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/mercurio/rooms/"+uuid.NewString(), admin, `{"kind":"global","name":"x"}`)
	h.expect(http.StatusBadRequest, http.MethodPut, "/api/v1/mercurio/rooms/x", admin, `{"kind":"global","name":"x"}`)

	// Acesso: lotado no departamento vê; outro não. Moderação vê tudo.
	rooms := func(tok string) string {
		return h.expect(http.StatusOK, http.MethodGet, "/api/v1/mercurio/rooms", tok, "").Body.String()
	}
	if !strings.Contains(rooms(ana), sala.ID) || strings.Contains(rooms(beto), sala.ID) || !strings.Contains(rooms(admin), sala.ID) {
		t.Fatal("sala departamental: só quem está no departamento (ou modera)")
	}
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/mercurio/rooms/"+sala.ID+"/messages", beto, "")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/mercurio/rooms/"+sala.ID+"/read", beto, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/mercurio/rooms/"+uuid.NewString()+"/messages", ana, "")

	// Mensagens: tamanho, cursor e limite.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/mercurio/rooms/"+sala.ID+"/messages", ana, `{"body":"`+strings.Repeat("a", 4001)+`"}`)
	var ids []string
	for i := 0; i < 3; i++ {
		m := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms/"+sala.ID+"/messages", ana, `{"body":"mensagem `+string(rune('a'+i))+`"}`))
		ids = append(ids, m.ID)
	}
	var created time.Time
	if err := h.d.DB.QueryRow(ctx, `SELECT created_at FROM mercurio_messages WHERE id = $1`, ids[2]).Scan(&created); err != nil {
		t.Fatal(err)
	}
	older := h.expect(http.StatusOK, http.MethodGet, "/api/v1/mercurio/rooms/"+sala.ID+"/messages?before="+created.Format(time.RFC3339Nano)+"&limit=1000", ana, "").Body.String()
	if strings.Contains(older, ids[2]) || !strings.Contains(older, ids[1]) {
		t.Fatalf("cursor before traz só as mais antigas: %s", older)
	}
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/mercurio/rooms/"+sala.ID+"/messages?before=ontem", ana, "")

	// Janela de edição de 15 minutos.
	if _, err := h.d.DB.Exec(ctx, `UPDATE mercurio_messages SET created_at = now() - interval '16 minutes' WHERE id = $1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	h.expect(http.StatusConflict, http.MethodPatch, "/api/v1/mercurio/messages/"+ids[0], ana, `{"body":"tarde demais"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPatch, "/api/v1/mercurio/messages/"+ids[1], ana, `{"body":"   "}`)
	h.expect(http.StatusNotFound, http.MethodPatch, "/api/v1/mercurio/messages/"+uuid.NewString(), ana, `{"body":"x"}`)
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/mercurio/messages/"+uuid.NewString(), ana, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/mercurio/messages/"+ids[2], ana, "")
	h.expect(http.StatusConflict, http.MethodPatch, "/api/v1/mercurio/messages/"+ids[2], ana, `{"body":"ressuscitar"}`)

	// Quem sai do departamento perde a sala — inclusive para editar ou
	// apagar o que escreveu.
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/users/"+anaID.String()+"/lotacoes/"+lotacao.ID, admin, "")
	h.expect(http.StatusForbidden, http.MethodPatch, "/api/v1/mercurio/messages/"+ids[1], ana, `{"body":"editado de fora"}`)
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/mercurio/messages/"+ids[1], ana, "")

	// Arquivada: somente leitura para todos; a moderação ainda remove.
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+anaID.String()+"/lotacoes", admin, `{"perfil_id":"`+servidor+`","departamento_id":"`+dep.ID+`"}`)
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/mercurio/rooms/"+sala.ID, admin,
		`{"kind":"department","name":"TI `+sfx+`","departamento_id":"`+dep.ID+`","archived":true}`)
	if strings.Contains(rooms(ana), sala.ID) {
		t.Fatal("sala arquivada sai da lista dos participantes")
	}
	h.expect(http.StatusForbidden, http.MethodPatch, "/api/v1/mercurio/messages/"+ids[1], ana, `{"body":"x"}`)
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/mercurio/messages/"+ids[1], admin, "")
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/mercurio/rooms/"+sala.ID+"/messages", admin, `{"body":"x"}`)
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/mercurio/rooms/"+sala.ID, admin,
		`{"kind":"department","name":"TI `+sfx+`","ad_group":"GRP_TI_`+sfx+`"}`)
	if strings.Contains(rooms(ana), sala.ID) {
		t.Fatal("sala passou a ser só do grupo do AD: a lotada no departamento sai")
	}

	// Conversa direta: consigo mesmo não; com conta desativada/inexistente não.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/mercurio/direct", ana, `{"user_id":"`+anaID.String()+`"}`)
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/mercurio/direct", ana, `{"user_id":"`+uuid.NewString()+`"}`)
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/mercurio/direct", ana, `{`)
	dm := data[idResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/mercurio/direct", ana, `{"user_id":"`+grupoID.String()+`"}`))
	h.expect(http.StatusConflict, http.MethodPut, "/api/v1/mercurio/rooms/"+dm.ID, admin, `{"kind":"global","name":"sequestro"}`)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/mercurio/rooms/x/messages", ""},
		{http.MethodPost, "/api/v1/mercurio/rooms/x/messages", `{"body":"a"}`},
		{http.MethodPost, "/api/v1/mercurio/rooms/x/read", ""},
		{http.MethodPatch, "/api/v1/mercurio/messages/x", `{"body":"a"}`},
		{http.MethodDelete, "/api/v1/mercurio/messages/x", ""},
		{http.MethodPost, "/api/v1/mercurio/rooms/" + dm.ID + "/messages", `{`},
		{http.MethodPatch, "/api/v1/mercurio/messages/" + ids[0], `{`},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, ana, c.body)
	}
}

// mercurio:manage com escopo (ADR 013): o moderador da unidade A cria e
// modera as salas dos departamentos de A — não as de B, nem as globais.
func TestMercurioModeracaoComEscopo(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	o := h.orgEscopo(t)
	dep := func(unidade string) string {
		return data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", admin,
			`{"unidade_id":"`+unidade+`","nome":"Depto `+uuid.NewString()[:6]+`"}`)).ID
	}
	depA, depSub, depB := dep(o.a), dep(o.sub), dep(o.b)
	modA := h.gestorEm(t, o.a, "mercurio:manage")
	sala := func(dep string) string {
		return `{"kind":"department","name":"Equipe ` + uuid.NewString()[:6] + `","departamento_id":"` + dep + `"}`
	}
	salaA := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms", modA, sala(depA)))
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms", modA, sala(depSub)) // herança
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/mercurio/rooms", modA, sala(depB))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/mercurio/rooms", modA, `{"kind":"global","name":"Geral paralelo"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/mercurio/rooms", modA, `{"kind":"department","name":"Só grupo","ad_group":"GRP_X"}`)
	// Não move a própria sala para o departamento de B.
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/mercurio/rooms/"+salaA.ID, modA, sala(depB))

	salaB := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms", admin, sala(depB)))
	h.expect(http.StatusForbidden, http.MethodPut, "/api/v1/mercurio/rooms/"+salaB.ID, modA, sala(depA))
	msgA := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms/"+salaA.ID+"/messages", admin, `{"body":"na sala de A"}`))
	msgB := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/mercurio/rooms/"+salaB.ID+"/messages", admin, `{"body":"na sala de B"}`))
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/mercurio/messages/"+msgB.ID, modA, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/mercurio/messages/"+msgA.ID, modA, "")
}
