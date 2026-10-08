package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Triagem: responsável precisa ser conta ativa; a leitura (acesso a PII)
// é auditada; o evento não carrega dados pessoais.
func TestContactTriageRules(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	ctx := context.Background()
	ok := data[struct {
		Protocol string `json:"protocol"`
	}](t, h.expect(http.StatusAccepted, http.MethodPost, "/api/v1/contact/messages", "",
		`{"name":"João Souza","email":"JOAO@Example.com","phone":"61 99999-0000","subject":"Reclamação formal","message":"Atendimento demorado no posto.","category":"reclamacao","consent":true}`))
	var id string
	if err := h.d.DB.QueryRow(ctx, `SELECT id FROM contact_messages WHERE protocol = $1`, ok.Protocol).Scan(&id); err != nil {
		t.Fatal(err)
	}
	var payload string
	if err := h.d.DB.QueryRow(ctx, `SELECT payload::text FROM outbox_events WHERE event_type = 'contact.message.submitted' AND aggregate_id = $1`, id).Scan(&payload); err != nil ||
		strings.Contains(payload, "João") || strings.Contains(payload, "joao@example.com") || strings.Contains(payload, "99999") {
		t.Fatalf("o evento não leva dados pessoais: %q %v", payload, err)
	}
	detail := h.expect(http.StatusOK, http.MethodGet, "/api/v1/contact/messages/"+id, admin, "").Body.String()
	if !strings.Contains(detail, `"email":"joao@example.com"`) {
		t.Fatalf("e-mail normalizado: %s", detail)
	}
	var viewed int
	if err := h.d.DB.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'contact.message.viewed' AND resource_id = $1`, id).Scan(&viewed); err != nil || viewed == 0 {
		t.Fatalf("a leitura da mensagem (PII) é auditada: %d %v", viewed, err)
	}

	inativoID, _ := h.user("nexus-user")
	if _, err := h.d.DB.Exec(ctx, `UPDATE users SET active = false WHERE id = $1`, inativoID); err != nil {
		t.Fatal(err)
	}
	responsavelID, _ := h.user("nexus-user")
	triage := func(want int, body string) {
		h.expect(want, http.MethodPatch, "/api/v1/contact/messages/"+id, admin, body)
	}
	triage(http.StatusUnprocessableEntity, `{"status":"in_progress","assigned_to":"`+uuid.NewString()+`"}`)
	triage(http.StatusUnprocessableEntity, `{"status":"in_progress","assigned_to":"`+inativoID.String()+`"}`)
	triage(http.StatusUnprocessableEntity, `{"status":"resolvida"}`)
	triage(http.StatusBadRequest, `{`)
	triage(http.StatusOK, `{"status":"in_progress","notes":"Encaminhado ao posto","assigned_to":"`+responsavelID.String()+`"}`)
	triage(http.StatusOK, `{"status":"answered","notes":"Respondido por e-mail"}`)
	if got := h.expect(http.StatusOK, http.MethodGet, "/api/v1/contact/messages?status=answered", admin, "").Body.String(); !strings.Contains(got, ok.Protocol) {
		t.Fatalf("filtro por estado: %s", got)
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/contact/messages/"+uuid.NewString(), admin, "")
	h.expect(http.StatusNotFound, http.MethodPatch, "/api/v1/contact/messages/"+uuid.NewString(), admin, `{"status":"archived"}`)
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/contact/messages/x", admin, "")
	h.expect(http.StatusBadRequest, http.MethodPatch, "/api/v1/contact/messages/x", admin, `{"status":"archived"}`)
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/contact/messages", "", `{`)
}
