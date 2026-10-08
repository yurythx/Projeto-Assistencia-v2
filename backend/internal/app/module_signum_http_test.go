package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const userPassword = "Senha-Forte-123!"

type envelopeResp struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Signers []struct {
		UserID string `json:"user_id"`
		Status string `json:"status"`
	} `json:"signers"`
}

type challengeResp struct {
	ChallengeID string `json:"challenge_id"`
	Nonce       string `json:"nonce"`
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// sign executa a cerimônia completa (desafio + reautenticação).
func (h *apiHarness) sign(token, envID, docHash, password string) *httpRec {
	h.t.Helper()
	c := data[challengeResp](h.t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/signum/envelopes/"+envID+"/challenge", token, ""))
	rec := h.do(http.MethodPost, "/api/v1/signum/envelopes/"+envID+"/sign", token,
		`{"challenge_id":"`+c.ChallengeID+`","nonce":"`+c.Nonce+`","password":"`+password+`","confirm_document_sha256":"`+docHash+`"}`)
	return &httpRec{code: rec.Code, body: rec.Body.String()}
}

type httpRec struct {
	code int
	body string
}

func (h *apiHarness) envelope(token, body string) envelopeResp {
	h.t.Helper()
	return data[envelopeResp](h.t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/signum/envelopes", token, body))
}

func signersJSON(ids ...uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = `"` + id.String() + `"`
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func auditCount(t *testing.T, h *apiHarness, action, resourceID string) int {
	t.Helper()
	var n int
	if err := h.d.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = $1 AND resource_id = $2`,
		action, resourceID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Abertura: validação do hash e dos signatários, e quem vê o envelope.
func TestSignumAberturaEVisibilidade(t *testing.T) {
	h := newHarness(t)
	authorID, author := h.user("nexus-user")
	anaID, ana := h.user("nexus-user")
	betoID, _ := h.user("nexus-user")
	_, stranger := h.user("nexus-user")
	admin := h.admin()
	doc := sha("contrato " + uuid.NewString())
	ctx := context.Background()

	open := func(want int, body string) {
		h.expect(want, http.MethodPost, "/api/v1/signum/envelopes", author, body)
	}
	open(http.StatusUnprocessableEntity, `{"title":"x","document_sha256":"curto","signer_ids":`+signersJSON(anaID)+`}`)
	open(http.StatusUnprocessableEntity, `{"title":"x","document_sha256":"`+strings.Repeat("z", 64)+`","signer_ids":`+signersJSON(anaID)+`}`)
	open(http.StatusUnprocessableEntity, `{"title":"x","document_sha256":"`+doc+`","signer_ids":[]}`)
	open(http.StatusUnprocessableEntity, `{"document_sha256":"`+doc+`","signer_ids":`+signersJSON(anaID)+`}`)
	open(http.StatusBadRequest, `{`)
	many := make([]uuid.UUID, 51)
	for i := range many {
		many[i] = uuid.New()
	}
	open(http.StatusUnprocessableEntity, `{"title":"x","document_sha256":"`+doc+`","signer_ids":`+signersJSON(many...)+`}`)
	// Signatário inexistente ou desativado nunca poderia assinar.
	open(http.StatusUnprocessableEntity, `{"title":"x","document_sha256":"`+doc+`","signer_ids":`+signersJSON(anaID, uuid.New())+`}`)
	inativoID, _ := h.user("nexus-user")
	if _, err := h.d.DB.Exec(ctx, `UPDATE users SET active = false WHERE id = $1`, inativoID); err != nil {
		t.Fatal(err)
	}
	open(http.StatusUnprocessableEntity, `{"title":"x","document_sha256":"`+doc+`","signer_ids":`+signersJSON(inativoID)+`}`)

	// Hash em maiúsculas é normalizado; signatário repetido entra uma vez.
	env := h.envelope(author, `{"title":"Contrato","document_sha256":"`+strings.ToUpper(doc)+`","signer_ids":`+signersJSON(anaID, betoID, anaID)+`}`)
	full := data[struct {
		DocumentSHA256 string `json:"document_sha256"`
		Signers        []struct {
			UserID string `json:"user_id"`
		} `json:"signers"`
		CreatedBy string `json:"created_by"`
	}](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/signum/envelopes/"+env.ID, author, ""))
	if full.DocumentSHA256 != doc || len(full.Signers) != 2 || full.CreatedBy != authorID.String() {
		t.Fatalf("envelope normalizado: %+v", full)
	}
	if auditCount(t, h, "signum.envelope.opened", env.ID) != 1 {
		t.Fatal("abertura auditada")
	}

	// Quem vê: autor, signatários e signum:manage. Estranho recebe 404.
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/signum/envelopes/"+env.ID, ana, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/signum/envelopes/"+env.ID, admin, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/signum/envelopes/"+env.ID, stranger, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/signum/envelopes/"+uuid.NewString(), author, "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/signum/envelopes/x", author, "")

	// Caixas: a assinar, criados, todos e "any" (só signum:manage).
	list := func(token, query string) string {
		return h.expect(http.StatusOK, http.MethodGet, "/api/v1/signum/envelopes"+query, token, "").Body.String()
	}
	if !strings.Contains(list(ana, "?role=to_sign"), env.ID) || strings.Contains(list(ana, "?role=created"), env.ID) {
		t.Fatal("o envelope está na caixa de quem assina, não na de criados")
	}
	if !strings.Contains(list(author, "?role=created&status=pending"), env.ID) || strings.Contains(list(author, "?role=to_sign"), env.ID) {
		t.Fatal("o autor vê o envelope entre os criados")
	}
	if !strings.Contains(list(ana, ""), env.ID) || strings.Contains(list(author, "?status=completed"), env.ID) {
		t.Fatal("filtro padrão (todos do usuário) e por status")
	}
	if strings.Contains(list(stranger, "?role=any"), env.ID) {
		t.Fatal(`"any" sem signum:manage vira "all" (só os do usuário)`)
	}
	if !strings.Contains(list(admin, "?role=any"), env.ID) {
		t.Fatal(`signum:manage lista todos com "any"`)
	}
}

// Cerimônia: ordem, desafio de uso único, reautenticação, bloqueio por
// tentativas, selo e verificação pública.
func TestSignumCerimonia(t *testing.T) {
	h := newHarness(t)
	_, author := h.user("nexus-user")
	anaID, ana := h.user("nexus-user")
	betoID, beto := h.user("nexus-user")
	_, stranger := h.user("nexus-user")
	doc := sha("contrato " + uuid.NewString())
	ctx := context.Background()

	env := h.envelope(author, `{"title":"Contrato","document_sha256":"`+doc+`","signer_ids":`+signersJSON(anaID, betoID)+`,"sequential":true}`)
	if env.Status != "pending" || len(env.Signers) != 2 {
		t.Fatalf("envelope: %+v", env)
	}

	// Desafio: só o signatário da vez.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/challenge", stranger, "")
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/challenge", beto, "")
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/signum/envelopes/"+uuid.NewString()+"/challenge", ana, "")
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/signum/envelopes/x/challenge", ana, "")
	c := data[challengeResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/challenge", ana, ""))
	signBody := func(ch challengeResp, password, docHash string) string {
		return `{"challenge_id":"` + ch.ChallengeID + `","nonce":"` + ch.Nonce + `","password":"` + password + `","confirm_document_sha256":"` + docHash + `"}`
	}

	// Documento divergente é recusado ANTES da senha (não conta tentativa).
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", ana, signBody(c, "qualquer", sha("outro")))
	if auditCount(t, h, "signum.reauth.failed", env.ID) != 0 {
		t.Fatal("documento divergente não chega a testar a senha")
	}
	// Quem não é da vez também não chega à senha.
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", beto, signBody(c, userPassword, doc))
	// Senha errada: 401, auditada, e o desafio continua válido.
	rec := h.do(http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", ana, signBody(c, "senha-errada", doc))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "SIGNUM_REAUTH_FAILED") {
		t.Fatalf("senha errada: %d %s", rec.Code, rec.Body.String())
	}
	if auditCount(t, h, "signum.reauth.failed", env.ID) != 1 {
		t.Fatal("falha de reautenticação fica na auditoria")
	}
	// Desafio de outro signatário, nonce errado e desafio expirado.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", ana,
		signBody(challengeResp{ChallengeID: c.ChallengeID, Nonce: sha("nonce-forjado")}, userPassword, doc))
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", ana,
		signBody(challengeResp{ChallengeID: uuid.NewString(), Nonce: c.Nonce}, userPassword, doc))
	exp := data[challengeResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/challenge", ana, ""))
	if _, err := h.d.DB.Exec(ctx, `UPDATE signum_challenges SET expires_at = now() - interval '1 second' WHERE id = $1`, exp.ChallengeID); err != nil {
		t.Fatal(err)
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", ana, signBody(exp, userPassword, doc))
	// Assinatura válida; o mesmo desafio não serve duas vezes.
	signed := data[envelopeResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", ana, signBody(c, userPassword, doc)))
	if signed.Status != "pending" || signed.Signers[0].Status != "signed" {
		t.Fatalf("primeira assinatura registrada, envelope segue pendente: %+v", signed)
	}
	// Quem já assinou não é mais signatário pendente.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", ana, signBody(c, userPassword, doc))
	if outboxCount(t, h, "signum.envelope.signed", env.ID) != 1 || outboxCount(t, h, "signum.envelope.completed", env.ID) != 0 {
		t.Fatal("cada assinatura emite signum.envelope.signed; completed só no fim")
	}

	// Bloqueio progressivo: 5 senhas erradas travam a cerimônia do usuário.
	cb := data[challengeResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/challenge", beto, ""))
	for i := 0; i < 5; i++ {
		h.expect(http.StatusUnauthorized, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", beto, signBody(cb, "errada", doc))
	}
	h.expect(http.StatusTooManyRequests, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/sign", beto, signBody(cb, userPassword, doc))
	if err := h.d.RateLimiters.Lockout.Reset(ctx, "signum:"+betoID.String()); err != nil {
		t.Fatal(err)
	}
	if r := h.sign(beto, env.ID, doc, userPassword); r.code != http.StatusOK || !strings.Contains(r.body, `"status":"completed"`) {
		t.Fatalf("última assinatura conclui o envelope: %d %s", r.code, r.body)
	}
	if outboxCount(t, h, "signum.envelope.completed", env.ID) != 1 {
		t.Fatal("conclusão emite signum.envelope.completed")
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/challenge", ana, "")
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/signum/envelopes/"+env.ID+"/cancel", author, "")

	// Verificação pública: selos válidos e conferência do arquivo.
	pub := h.expect(http.StatusOK, http.MethodGet, "/api/v1/signum/verify/"+env.ID+"?document_sha256="+strings.ToUpper(doc), "", "").Body.String()
	if !strings.Contains(pub, `"document_match":true`) || strings.Contains(pub, `"valid":false`) || strings.Contains(pub, "ip_address") {
		t.Fatalf("verificação pública (sem dados técnicos do signatário): %s", pub)
	}
	if bad := h.expect(http.StatusOK, http.MethodGet, "/api/v1/signum/verify/"+env.ID+"?document_sha256="+sha("x"), "", "").Body.String(); !strings.Contains(bad, `"document_match":false`) {
		t.Fatalf("arquivo diferente não confere: %s", bad)
	}
	if noDoc := h.expect(http.StatusOK, http.MethodGet, "/api/v1/signum/verify/"+env.ID, "", "").Body.String(); strings.Contains(noDoc, "document_match") {
		t.Fatalf("sem arquivo apresentado, não há conferência: %s", noDoc)
	}
	// Adulterar o banco invalida o selo (a chave do selo não está no banco).
	if _, err := h.d.DB.Exec(ctx, `UPDATE signum_signers SET signed_at = signed_at + interval '1 second' WHERE envelope_id = $1 AND user_id = $2`,
		env.ID, anaID); err != nil {
		t.Fatal(err)
	}
	if tampered := h.expect(http.StatusOK, http.MethodGet, "/api/v1/signum/verify/"+env.ID, "", "").Body.String(); !strings.Contains(tampered, `"valid":false`) {
		t.Fatalf("assinatura adulterada no banco deixa de ser válida: %s", tampered)
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/signum/verify/"+uuid.NewString(), "", "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/signum/verify/x", "", "")
	if auditCount(t, h, "signum.envelope.signed", env.ID) != 2 {
		t.Fatal("cada assinatura auditada com método e selo")
	}
}

// Assinatura em paralelo, recusa e cancelamento encerram o envelope.
func TestSignumRecusaECancelamento(t *testing.T) {
	h := newHarness(t)
	authorID, author := h.user("nexus-user")
	anaID, ana := h.user("nexus-user")
	betoID, beto := h.user("nexus-user")
	_, stranger := h.user("nexus-user")
	admin := h.admin()
	doc := sha("aditivo " + uuid.NewString())

	// Paralelo: qualquer pendente assina, em qualquer ordem.
	par := h.envelope(author, `{"title":"Ata","document_sha256":"`+doc+`","signer_ids":`+signersJSON(anaID, betoID)+`}`)
	if r := h.sign(beto, par.ID, doc, userPassword); r.code != http.StatusOK {
		t.Fatalf("em paralelo, o segundo assina primeiro: %d %s", r.code, r.body)
	}

	// Recusa: exige motivo, vale para quem ainda não chegou a vez, encerra.
	seq := h.envelope(author, `{"title":"Aditivo","document_sha256":"`+doc+`","signer_ids":`+signersJSON(anaID, betoID)+`,"sequential":true}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/signum/envelopes/"+seq.ID+"/refuse", beto, `{"reason":"x"}`)
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/signum/envelopes/"+seq.ID+"/refuse", beto, `{`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/signum/envelopes/"+seq.ID+"/refuse", stranger, `{"reason":"não sou parte"}`)
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/signum/envelopes/x/refuse", beto, `{"reason":"motivo"}`)
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/signum/envelopes/"+uuid.NewString()+"/refuse", beto, `{"reason":"motivo"}`)
	ref := data[envelopeResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/signum/envelopes/"+seq.ID+"/refuse", beto, `{"reason":"cláusula errada"}`))
	if ref.Status != "refused" || ref.Signers[1].Status != "refused" || ref.Signers[0].Status != "pending" {
		t.Fatalf("recusa registrada no signatário e encerra o envelope: %+v", ref)
	}
	if outboxCount(t, h, "signum.envelope.refused", seq.ID) != 1 {
		t.Fatal("recusa emite signum.envelope.refused")
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/signum/envelopes/"+seq.ID+"/challenge", ana, "")
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/signum/envelopes/"+seq.ID+"/refuse", ana, `{"reason":"tarde demais"}`)

	// Cancelamento: autor ou signum:manage; signatário não.
	env3 := h.envelope(author, `{"title":"Termo","document_sha256":"`+doc+`","signer_ids":`+signersJSON(anaID)+`}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/signum/envelopes/"+env3.ID+"/cancel", ana, "")
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/signum/envelopes/"+uuid.NewString()+"/cancel", author, "")
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/signum/envelopes/x/cancel", author, "")
	cancelled := data[envelopeResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/signum/envelopes/"+env3.ID+"/cancel", author, ""))
	if cancelled.Status != "cancelled" || outboxCount(t, h, "signum.envelope.cancelled", env3.ID) != 1 {
		t.Fatalf("cancelado com evento: %+v", cancelled)
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/signum/envelopes/"+env3.ID+"/challenge", ana, "")
	env4 := h.envelope(author, `{"title":"Termo 2","document_sha256":"`+doc+`","signer_ids":`+signersJSON(anaID)+`}`)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/signum/envelopes/"+env4.ID+"/cancel", admin, "")
	_ = authorID

	// Corpo inválido na assinatura.
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/signum/envelopes/x/sign", ana, `{}`)
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/signum/envelopes/"+par.ID+"/sign", ana, `{`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/signum/envelopes/"+par.ID+"/sign", ana, `{"nonce":"curto"}`)
}
