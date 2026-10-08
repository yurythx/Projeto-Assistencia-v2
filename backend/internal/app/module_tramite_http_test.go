package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// tramiteFixture monta a estrutura usada pelos testes do Trâmite: duas
// unidades (Protocolo e Jurídico) e servidores lotados com o perfil
// "protocolo" (tramite:create + tramite:route).
type tramiteFixture struct {
	h                  *apiHarness
	admin, tipo, ent   string
	unA, unB           string
	anaA, carlosA      string
	betoB, semLotacao  string
	anaID, carlosID    uuid.UUID
	betoID, semLotacID uuid.UUID
}

func newTramiteFixture(t *testing.T) *tramiteFixture {
	t.Helper()
	h := newHarness(t)
	f := &tramiteFixture{h: h, admin: h.admin()}
	ctx := context.Background()
	sfx := uuid.NewString()[:6]
	ent := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", f.admin, `{"nome":"Órgão `+sfx+`"}`))
	f.ent = ent.ID
	f.unA = data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", f.admin,
		`{"entidade_id":"`+ent.ID+`","nome":"Protocolo `+sfx+`","sigla":"PA`+sfx+`"}`)).ID
	f.unB = data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", f.admin,
		`{"entidade_id":"`+ent.ID+`","nome":"Jurídico `+sfx+`","sigla":"JU`+sfx+`"}`)).ID
	var protocolo string
	if err := h.d.DB.QueryRow(ctx, `SELECT id FROM perfis WHERE slug = 'protocolo'`).Scan(&protocolo); err != nil {
		t.Fatal(err)
	}
	if err := h.d.DB.QueryRow(ctx, `SELECT id FROM tramite_tipos WHERE ativo ORDER BY slug LIMIT 1`).Scan(&f.tipo); err != nil {
		t.Fatal(err)
	}
	lotar := func(unidade string) (uuid.UUID, string) {
		id, tok := h.user("nexus-user")
		h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+id.String()+"/lotacoes", f.admin,
			`{"perfil_id":"`+protocolo+`","unidade_id":"`+unidade+`"}`)
		return id, tok
	}
	f.anaID, f.anaA = lotar(f.unA)
	f.carlosID, f.carlosA = lotar(f.unA)
	f.betoID, f.betoB = lotar(f.unB)
	f.semLotacID, f.semLotacao = h.user("nexus-user")
	return f
}

type procResp struct {
	ID       string `json:"id"`
	Numero   string `json:"numero"`
	Status   string `json:"status"`
	Sigilo   string `json:"sigilo"`
	Assunto  string `json:"assunto"`
	CanAct   bool   `json:"can_act"`
	CanRoute bool   `json:"can_route"`
}

type docResp struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Origem      string  `json:"origem"`
	SHA256      *string `json:"sha256"`
	EnvelopeID  *string `json:"envelope_id"`
	Conteudo    string  `json:"conteudo"`
	DownloadURL string  `json:"download_url"`
}

func (f *tramiteFixture) abrir(t *testing.T, token, sigilo, unidade, assunto string) procResp {
	t.Helper()
	return data[procResp](t, f.h.expect(http.StatusCreated, http.MethodPost, "/api/v1/tramite/processos", token,
		`{"tipo_id":"`+f.tipo+`","assunto":"`+assunto+`","interessado":"Fulano","sigilo":"`+sigilo+`","unidade_origem_id":"`+unidade+`"}`))
}

func (f *tramiteFixture) redigir(t *testing.T, token, procID, titulo, conteudo string) docResp {
	t.Helper()
	return data[docResp](t, f.h.expect(http.StatusCreated, http.MethodPost, "/api/v1/tramite/processos/"+procID+"/documentos", token,
		`{"tipo":"Despacho","titulo":"`+titulo+`","conteudo":"`+conteudo+`"}`))
}

func movimentos(t *testing.T, h *apiHarness, procID string) []string {
	t.Helper()
	rows, err := h.d.DB.Query(context.Background(), `SELECT acao FROM tramite_movimentos WHERE processo_id = $1 ORDER BY created_at`, procID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	return out
}

// Abertura: quem pode abrir, validação de entrada e numeração sequencial.
func TestTramiteAbertura(t *testing.T) {
	f := newTramiteFixture(t)
	h := f.h
	ctx := context.Background()

	tipos := h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/tipos", f.anaA, "").Body.String()
	if !strings.Contains(tipos, f.tipo) {
		t.Fatalf("tipos ativos listados: %s", tipos)
	}

	body := func(tipo, sigilo, unidade string) string {
		return `{"tipo_id":"` + tipo + `","assunto":"Pedido","sigilo":"` + sigilo + `","unidade_origem_id":"` + unidade + `"}`
	}
	// Só abre quem tem tramite:create, e só na própria unidade.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/tramite/processos", f.semLotacao, body(f.tipo, "publico", f.unA))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/tramite/processos", f.anaA, body(f.tipo, "publico", f.unB))
	// Entrada inválida.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos", f.anaA, body(f.tipo, "secreto", f.unA))
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos", f.anaA, `{"sigilo":"publico"}`)
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/tramite/processos", f.anaA, `{`)
	// Tipo inexistente ou desativado não abre processo.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos", f.anaA, body(uuid.NewString(), "publico", f.unA))
	var inativo string
	if err := h.d.DB.QueryRow(ctx, `INSERT INTO tramite_tipos (slug, nome, ativo) VALUES ($1, 'Tipo antigo', false) RETURNING id`,
		"inativo-"+uuid.NewString()[:8]).Scan(&inativo); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/tipos", f.anaA, "").Body.String(), inativo) {
		t.Fatal("tipo desativado não aparece para escolha")
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos", f.anaA, body(inativo, "publico", f.unA))
	// tramite:manage abre em qualquer unidade — mas ela precisa existir.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos", f.admin, body(f.tipo, "publico", uuid.NewString()))

	p1 := f.abrir(t, f.anaA, "publico", f.unA, "Primeiro")
	p2 := f.abrir(t, f.anaA, "publico", f.unA, "Segundo")
	if p1.Status != "aberto" || len(p1.Numero) != len("000001/2026") || p1.Numero >= p2.Numero {
		t.Fatalf("numeração NNNNNN/AAAA sequencial: %s, %s", p1.Numero, p2.Numero)
	}
	if got := movimentos(t, h, p1.ID); len(got) != 1 || got[0] != "abertura" {
		t.Fatalf("abertura registrada no histórico: %v", got)
	}
	if outboxCount(t, h, "tramite.processo.aberto", p1.ID) != 1 {
		t.Fatal("abertura emite tramite.processo.aberto")
	}
	gestao := f.abrir(t, f.admin, "publico", f.unB, "Aberto pela gestão")
	if gestao.Numero == "" {
		t.Fatal("tramite:manage abre em qualquer unidade existente")
	}
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/tramite/processos/nao-e-uuid", f.anaA, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/processos/"+uuid.NewString(), f.anaA, "")
}

// Sigilo: público, restrito e sigiloso, com credencial nominal concedida e
// revogada, na leitura, na listagem, na busca e nos eventos.
func TestTramiteSigiloECredenciais(t *testing.T) {
	f := newTramiteFixture(t)
	h := f.h
	sfx := uuid.NewString()[:6]

	pub := f.abrir(t, f.anaA, "publico", f.unA, "Obra pública "+sfx)
	rest := f.abrir(t, f.anaA, "restrito", f.unA, "Sindicância "+sfx)
	sig := f.abrir(t, f.anaA, "sigiloso", f.unA, "Denúncia "+sfx)

	// Público: qualquer autenticado lê, mas só a unidade atual age.
	v := data[procResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+pub.ID, f.semLotacao, ""))
	if v.CanAct {
		t.Fatal("ler um processo público não dá direito de movimentá-lo")
	}
	if v = data[procResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+pub.ID, f.carlosA, "")); !v.CanAct || !v.CanRoute {
		t.Fatalf("colega da unidade atual movimenta e tramita: %+v", v)
	}
	// Restrito: unidades envolvidas e tramite:manage; outras unidades não.
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+rest.ID, f.carlosA, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+rest.ID, f.admin, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/processos/"+rest.ID, f.betoB, "")
	// Sigiloso: nem colega, nem tramite:manage — e a resposta é 404.
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/processos/"+sig.ID, f.carlosA, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/processos/"+sig.ID, f.admin, "")

	list := func(token, query string) string {
		return h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos?page_size=100"+query, token, "").Body.String()
	}
	if l := list(f.betoB, ""); !strings.Contains(l, pub.ID) || strings.Contains(l, rest.ID) || strings.Contains(l, sig.ID) {
		t.Fatal("listagem aplica o mesmo sigilo da leitura (outra unidade vê só o público)")
	}
	if l := list(f.carlosA, ""); !strings.Contains(l, rest.ID) || strings.Contains(l, sig.ID) {
		t.Fatal("colega vê o restrito da unidade, nunca o sigiloso sem credencial")
	}
	if l := list(f.admin, ""); !strings.Contains(l, rest.ID) || strings.Contains(l, sig.ID) {
		t.Fatal("tramite:manage lista restritos de qualquer unidade, mas não sigilosos")
	}

	// Evento de processo não público não carrega o assunto.
	var payload string
	if err := h.d.DB.QueryRow(context.Background(), `SELECT payload::text FROM outbox_events WHERE event_type = 'tramite.processo.aberto' AND aggregate_id = $1`,
		sig.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "Denúncia") {
		t.Fatalf("o sigilo protege o assunto também nos eventos: %s", payload)
	}

	// Credencial: não existe em processo público; concedida, dá acesso nominal.
	grant := func(want int, token, procID string, user uuid.UUID) {
		h.expect(want, http.MethodPost, "/api/v1/tramite/processos/"+procID+"/acessos", token, `{"user_id":"`+user.String()+`"}`)
	}
	grant(http.StatusUnprocessableEntity, f.anaA, pub.ID, f.betoID)
	grant(http.StatusUnprocessableEntity, f.anaA, sig.ID, uuid.New())
	grant(http.StatusNotFound, f.carlosA, sig.ID, f.carlosID) // sem acesso, nem sabe que existe
	grant(http.StatusNoContent, f.anaA, sig.ID, f.carlosID)
	grant(http.StatusNoContent, f.anaA, sig.ID, f.carlosID) // idempotente
	view := h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+sig.ID, f.carlosA, "").Body.String()
	if !strings.Contains(view, f.carlosID.String()) {
		t.Fatalf("a credencial aparece na lista de acessos: %s", view)
	}
	if !strings.Contains(list(f.carlosA, ""), sig.ID) {
		t.Fatal("credenciado passa a ver o sigiloso na listagem")
	}
	// Leitura de processo não público é auditada.
	var acessos int
	if err := h.d.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = 'tramite.processo.acessado' AND resource_id = $1`,
		sig.ID).Scan(&acessos); err != nil {
		t.Fatal(err)
	}
	if acessos == 0 {
		t.Fatal("acesso a processo sigiloso fica na auditoria")
	}

	// Revogação: some a leitura e a revogação fica no histórico.
	h.expect(http.StatusBadRequest, http.MethodDelete, "/api/v1/tramite/processos/"+sig.ID+"/acessos/xyz", f.anaA, "")
	h.expect(http.StatusBadRequest, http.MethodDelete, "/api/v1/tramite/processos/xyz/acessos/"+f.carlosID.String(), f.anaA, "")
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/tramite/processos/"+sig.ID+"/acessos/"+f.betoID.String(), f.anaA, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/tramite/processos/"+sig.ID+"/acessos/"+f.carlosID.String(), f.anaA, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/processos/"+sig.ID, f.carlosA, "")
	if got := strings.Join(movimentos(t, h, sig.ID), ","); !strings.Contains(got, "acesso_concedido") || !strings.Contains(got, "acesso_revogado") {
		t.Fatalf("concessão e revogação no histórico: %s", got)
	}
	// Quem não atua no processo não revoga.
	grant(http.StatusNoContent, f.anaA, sig.ID, f.betoID)
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/tramite/processos/"+sig.ID+"/acessos/"+f.anaID.String(), f.betoB, "")

	// Busca global: título do sigiloso sem o assunto; público com assunto.
	res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+sfx+"&module=tramite", f.anaA, "").Body.String()
	if !strings.Contains(res, "Obra pública "+sfx) {
		t.Fatalf("processo público aparece na busca com o assunto: %s", res)
	}
	if strings.Contains(res, "Denúncia") {
		t.Fatalf("a busca nunca expõe o assunto de processo sigiloso: %s", res)
	}
	if res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q="+sfx+"&module=tramite", f.betoB, "").Body.String(); strings.Contains(res, rest.ID) {
		t.Fatal("a busca respeita o sigilo")
	}
}

// Fluxo completo: documentos redigidos e anexados, assinatura via Signum
// (concluída e recusada), tramitação, conclusão, arquivamento e reabertura.
func TestTramiteFluxoDeTrabalho(t *testing.T) {
	f := newTramiteFixture(t)
	h := f.h
	proc := f.abrir(t, f.anaA, "restrito", f.unA, "Aquisição de equipamentos")

	// Documento redigido: criar, editar e ler.
	doc := f.redigir(t, f.anaA, proc.ID, "Parecer", "Rascunho")
	if doc.Status != "rascunho" || doc.Origem != "redigido" {
		t.Fatalf("documento redigido nasce em rascunho: %+v", doc)
	}
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/tramite/documentos/"+doc.ID, f.betoB, `{"titulo":"x","conteudo":"y"}`)
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/tramite/documentos/"+doc.ID, f.carlosA, `{"titulo":"Parecer técnico","conteudo":"Deferido."}`)
	lido := data[docResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/documentos/"+doc.ID, f.anaA, ""))
	if lido.Conteudo != "Deferido." {
		t.Fatalf("leitura traz o conteúdo editado: %+v", lido)
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/documentos/"+doc.ID, f.betoB, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/documentos/"+uuid.NewString(), f.anaA, "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/tramite/documentos/x", f.anaA, "")

	// Anexo: ticket de upload, envio direto e confirmação com hash.
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/uploads", f.betoB, `{"filename":"a.pdf","content_type":"application/pdf"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/uploads", f.anaA, `{"filename":"a.exe","content_type":"application/x-msdownload"}`)
	ticket := data[struct {
		ObjectKey string `json:"object_key"`
	}](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/uploads", f.anaA,
		`{"filename":"orcamento.pdf","content_type":"application/pdf"}`))
	pdf := []byte("%PDF-1.4 orçamento")
	h.upload(ticket.ObjectKey, "application/pdf", pdf)
	anexo := data[docResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/documentos", f.anaA,
		`{"tipo":"Orçamento","titulo":"Orçamento","object_key":"`+ticket.ObjectKey+`"}`))
	if anexo.Origem != "anexo" || anexo.SHA256 == nil || *anexo.SHA256 != sha(string(pdf)) {
		t.Fatalf("anexo confirmado com o SHA-256 do arquivo: %+v", anexo)
	}
	if a := data[docResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/documentos/"+anexo.ID, f.anaA, "")); a.DownloadURL == "" {
		t.Fatal("anexo é lido por URL temporária")
	}
	// Anexo não é editável como texto; chave de outro processo é recusada.
	h.expect(http.StatusConflict, http.MethodPut, "/api/v1/tramite/documentos/"+anexo.ID, f.anaA, `{"titulo":"x","conteudo":"y"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/documentos", f.anaA,
		`{"titulo":"Intruso","object_key":"tramite/`+uuid.NewString()+`/x.pdf"}`)

	// Assinatura concluída: o documento congela, o processo não conclui com
	// assinatura pendente, e o evento do Signum marca o documento assinado.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/documentos/"+doc.ID+"/assinatura", f.anaA, `{"signer_ids":[]}`)
	pedido := data[docResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/documentos/"+doc.ID+"/assinatura", f.anaA,
		`{"signer_ids":["`+f.carlosID.String()+`"]}`))
	if pedido.Status != "aguardando_assinatura" || pedido.EnvelopeID == nil || pedido.SHA256 == nil || *pedido.SHA256 != sha("Deferido.") {
		t.Fatalf("documento congelado com o hash do conteúdo e envelope aberto: %+v", pedido)
	}
	h.expect(http.StatusConflict, http.MethodPut, "/api/v1/tramite/documentos/"+doc.ID, f.anaA, `{"titulo":"x","conteudo":"y"}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/documentos/"+doc.ID+"/assinatura", f.anaA, `{"signer_ids":["`+f.carlosID.String()+`"]}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/concluir", f.anaA, `{"despacho":"Cedo demais"}`)
	if r := h.sign(f.carlosA, *pedido.EnvelopeID, *pedido.SHA256, userPassword); r.code != http.StatusOK {
		t.Fatalf("assinatura no Signum: %d %s", r.code, r.body)
	}
	if n := h.deliver("signum.envelope.completed", *pedido.EnvelopeID); n != 1 {
		t.Fatalf("o Trâmite consome signum.envelope.completed: %d entregas", n)
	}
	if d := data[docResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/documentos/"+doc.ID, f.anaA, "")); d.Status != "assinado" {
		t.Fatalf("documento assinado após o evento: %+v", d)
	}
	h.deliver("signum.envelope.completed", *pedido.EnvelopeID) // reentrega é inofensiva (idempotente)

	// Assinatura recusada: o documento volta a rascunho e pode ser corrigido.
	minuta := f.redigir(t, f.anaA, proc.ID, "Minuta", "Versão 1")
	pedido2 := data[docResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/documentos/"+minuta.ID+"/assinatura", f.anaA,
		`{"signer_ids":["`+f.carlosID.String()+`"],"sequential":true}`))
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/signum/envelopes/"+*pedido2.EnvelopeID+"/refuse", f.carlosA, `{"reason":"falta a cláusula de garantia"}`)
	h.deliver("signum.envelope.refused", *pedido2.EnvelopeID)
	if d := data[docResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/documentos/"+minuta.ID, f.anaA, "")); d.Status != "rascunho" || d.EnvelopeID != nil {
		t.Fatalf("recusa devolve o documento a rascunho, sem envelope: %+v", d)
	}
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/tramite/documentos/"+minuta.ID, f.anaA, `{"titulo":"Minuta","conteudo":"Versão 2"}`)

	// Tramitação: exige tramite:route, despacho e outra unidade.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.anaA, `{"despacho":"Sem destino"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.anaA, `{"para_unidade_id":"`+f.unB+`","despacho":"x"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.anaA, `{"para_unidade_id":"`+f.unA+`","despacho":"Mesma unidade"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.anaA, `{"para_unidade_id":"`+uuid.NewString()+`","despacho":"Unidade fantasma"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.semLotacao, `{"para_unidade_id":"`+f.unB+`","despacho":"Sem permissão"}`)
	tram := data[procResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.anaA,
		`{"para_unidade_id":"`+f.unB+`","despacho":"Encaminho ao jurídico"}`))
	if tram.Status != "em_tramitacao" {
		t.Fatalf("processo em tramitação: %+v", tram)
	}
	if outboxCount(t, h, "tramite.processo.tramitado", proc.ID) != 1 {
		t.Fatal("tramitação emite tramite.processo.tramitado")
	}
	// Restrito: a origem continua lendo, mas só a unidade atual age.
	if v := data[procResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+proc.ID, f.anaA, "")); v.CanAct {
		t.Fatal("a origem não movimenta o processo depois de tramitar")
	}
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/documentos", f.anaA, `{"titulo":"Tarde","conteudo":"x"}`)
	caixa := h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos?minha_caixa=true&page_size=100", f.betoB, "").Body.String()
	if !strings.Contains(caixa, proc.ID) {
		t.Fatal("o processo chega à caixa da unidade de destino")
	}
	if l := h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos?minha_caixa=true&page_size=100", f.anaA, "").Body.String(); strings.Contains(l, proc.ID) {
		t.Fatal("e sai da caixa da origem")
	}
	filtro := h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos?status=em_tramitacao&unidade_id="+f.unB+"&q="+tram.Numero[:6], f.admin, "").Body.String()
	if !strings.Contains(filtro, proc.ID) {
		t.Fatalf("filtros por status, unidade e prefixo do número: %s", filtro)
	}
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/tramite/processos?unidade_id=x", f.admin, "")

	// Conclusão encerra o processo: nada mais entra sem reabrir.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/concluir", f.betoB, `{}`)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/concluir", f.betoB, `{"despacho":"Aquisição concluída"}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/concluir", f.betoB, `{"despacho":"De novo"}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/documentos", f.betoB, `{"titulo":"Depois","conteudo":"x"}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/uploads", f.betoB, `{"filename":"a.pdf","content_type":"application/pdf"}`)
	h.expect(http.StatusConflict, http.MethodPut, "/api/v1/tramite/documentos/"+minuta.ID, f.betoB, `{"titulo":"x","conteudo":"y"}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/documentos/"+minuta.ID+"/assinatura", f.betoB, `{"signer_ids":["`+f.betoID.String()+`"]}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.betoB, `{"para_unidade_id":"`+f.unA+`","despacho":"Devolver"}`)

	// Arquivar só depois de concluir; reabrir exige tramite:manage.
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/arquivar", f.betoB, `{"despacho":"Arquivo"}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/arquivar", f.betoB, `{"despacho":"De novo"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/reabrir", f.betoB, `{"despacho":"Reabrir"}`)
	reab := data[procResp](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/reabrir", f.admin, `{"despacho":"Reaberto para complementação"}`))
	if reab.Status != "em_tramitacao" {
		t.Fatalf("reaberto volta a tramitar: %+v", reab)
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/reabrir", f.admin, `{"despacho":"De novo"}`)
	f.redigir(t, f.betoB, proc.ID, "Complemento", "Novo documento após a reabertura")

	want := []string{"abertura", "documento", "documento", "assinatura_solicitada", "assinatura_concluida", "documento",
		"assinatura_solicitada", "documento", "tramitacao", "conclusao", "arquivamento", "reabertura", "documento"}
	if got := movimentos(t, h, proc.ID); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("histórico completo e em ordem:\n got %v\nwant %v", got, want)
	}
	for _, ev := range []string{"tramite.processo.concluido", "tramite.processo.arquivado"} {
		if outboxCount(t, h, ev, proc.ID) != 1 {
			t.Fatalf("%s emitido", ev)
		}
	}
	var trilha int
	if err := h.d.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE resource_id = $1 AND action LIKE 'tramite.processo.%'`,
		proc.ID).Scan(&trilha); err != nil {
		t.Fatal(err)
	}
	if trilha < 6 {
		t.Fatalf("cada transição fica na auditoria (%d entradas)", trilha)
	}

	// O histórico é append-only também no banco.
	if _, err := h.d.DB.Exec(context.Background(), `UPDATE tramite_movimentos SET despacho = 'adulterado' WHERE processo_id = $1`, proc.ID); err == nil {
		t.Fatal("o banco recusa alterar o histórico")
	}
	raw := h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+proc.ID, f.betoB, "").Body.String()
	var view struct {
		Data struct {
			Movimentos []struct {
				Acao string `json:"acao"`
			} `json:"movimentos"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &view); err != nil || len(view.Data.Movimentos) != len(want) {
		t.Fatalf("a visão do processo traz o histórico: %v %s", err, raw)
	}
}

// Identificadores malformados na URL são recusados antes de qualquer
// acesso ao banco, e a busca respeita a indisponibilidade do banco.
// Unidade (ou entidade) desativada não abre nem recebe processos — nem por
// quem tem tramite:manage — e quem só tinha lotação nela perde o perfil.
func TestTramiteUnidadeDesativada(t *testing.T) {
	f := newTramiteFixture(t)
	h := f.h
	proc := f.abrir(t, f.anaA, "publico", f.unA, "Processo para unidade desativada")
	unidade := func(nome string, ativo bool) string {
		return fmt.Sprintf(`{"entidade_id":"%s","nome":"%s","ativo":%t}`, f.ent, nome, ativo)
	}
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/unidades/"+f.unB, f.admin, unidade("Jurídico desativado", false))

	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.anaA,
		`{"para_unidade_id":"`+f.unB+`","despacho":"Para unidade desativada"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos", f.admin,
		`{"tipo_id":"`+f.tipo+`","assunto":"x","sigilo":"publico","unidade_origem_id":"`+f.unB+`"}`)
	// Beto só tinha lotação no Jurídico: sem ela, nem protocolo ele é mais.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/tramite/processos", f.betoB,
		`{"tipo_id":"`+f.tipo+`","assunto":"x","sigilo":"publico","unidade_origem_id":"`+f.unB+`"}`)

	h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/unidades/"+f.unB, f.admin, unidade("Jurídico", true))
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/tramitar", f.anaA,
		`{"para_unidade_id":"`+f.unB+`","despacho":"Reativada"}`)

	// Entidade desativada: as unidades dela também não servem.
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/entidades/"+f.ent, f.admin, `{"nome":"Órgão desativado","ativo":false}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/tramite/processos", f.admin,
		`{"tipo_id":"`+f.tipo+`","assunto":"x","sigilo":"publico","unidade_origem_id":"`+f.unA+`"}`)
}

// Permissão com escopo (ADR 013): tramite:manage e tramite:create valem na
// unidade em que foram concedidos e nas subunidades dela — não nas irmãs.
func TestTramiteGestaoComEscopo(t *testing.T) {
	f := newTramiteFixture(t)
	h := f.h
	ctx := context.Background()
	sub := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", f.admin,
		`{"entidade_id":"`+f.ent+`","parent_id":"`+f.unA+`","nome":"Subunidade de A"}`)).ID
	var protocolo string
	if err := h.d.DB.QueryRow(ctx, `SELECT id FROM perfis WHERE slug = 'protocolo'`).Scan(&protocolo); err != nil {
		t.Fatal(err)
	}
	gestorPerfil := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/perfis", f.admin,
		`{"nome":"Gestor de trâmite `+uuid.NewString()[:6]+`","permissoes":["tramite:manage"]}`)).ID
	lotar := func(perfil, unidade string) string {
		id, tok := h.user("nexus-user")
		h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+id.String()+"/lotacoes", f.admin,
			`{"perfil_id":"`+perfil+`","unidade_id":"`+unidade+`"}`)
		return tok
	}
	gestorA := lotar(gestorPerfil, f.unA)
	protoSub := lotar(protocolo, sub)

	emA := f.abrir(t, f.anaA, "restrito", f.unA, "Restrito de A")
	emSub := f.abrir(t, protoSub, "restrito", sub, "Restrito da subunidade")
	emB := f.abrir(t, f.betoB, "restrito", f.unB, "Restrito de B")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+emA.ID, gestorA, "")
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos/"+emSub.ID, gestorA, "") // herança
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/tramite/processos/"+emB.ID, gestorA, "")
	lista := h.expect(http.StatusOK, http.MethodGet, "/api/v1/tramite/processos", gestorA, "").Body.String()
	if !strings.Contains(lista, emA.ID) || !strings.Contains(lista, emSub.ID) || strings.Contains(lista, emB.ID) {
		t.Fatalf("listagem do gestor de A: só A e a subunidade: %s", lista)
	}

	// Reabrir: só onde tem tramite:manage; o global reabre em qualquer unidade.
	pubB := f.abrir(t, f.betoB, "publico", f.unB, "Público de B")
	for _, proc := range []procResp{emA, pubB} {
		dono := f.anaA
		if proc.ID == pubB.ID {
			dono = f.betoB
		}
		h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/processos/"+proc.ID+"/concluir", dono, `{"despacho":"Concluído"}`)
	}
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/processos/"+emA.ID+"/reabrir", gestorA, `{"despacho":"Reaberto pela gestão de A"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/tramite/processos/"+pubB.ID+"/reabrir", gestorA, `{"despacho":"Fora do escopo"}`)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/tramite/processos/"+pubB.ID+"/reabrir", f.admin, `{"despacho":"Gestão global"}`)

	// tramite:create do Protocolo de A vale na subunidade, não na irmã B.
	f.abrir(t, f.anaA, "publico", sub, "Aberto pelo Protocolo de A na subunidade")
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/tramite/processos", f.anaA,
		`{"tipo_id":"`+f.tipo+`","assunto":"x","sigilo":"publico","unidade_origem_id":"`+f.unB+`"}`)
}

func TestTramiteRejectsMalformedIDs(t *testing.T) {
	f := newTramiteFixture(t)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/tramite/processos/x/tramitar", `{"para_unidade_id":"` + f.unB + `","despacho":"abc"}`},
		{http.MethodPost, "/api/v1/tramite/processos/x/concluir", `{"despacho":"abc"}`},
		{http.MethodPost, "/api/v1/tramite/processos/x/acessos", `{"user_id":"` + f.betoID.String() + `"}`},
		{http.MethodPost, "/api/v1/tramite/processos/x/uploads", `{"filename":"a.pdf","content_type":"application/pdf"}`},
		{http.MethodPost, "/api/v1/tramite/processos/x/documentos", `{"titulo":"t"}`},
		{http.MethodPut, "/api/v1/tramite/documentos/x", `{"titulo":"t"}`},
		{http.MethodPost, "/api/v1/tramite/documentos/x/assinatura", `{"signer_ids":["` + f.betoID.String() + `"]}`},
	} {
		f.h.expect(http.StatusBadRequest, c.method, c.path, f.anaA, c.body)
	}
	p := f.abrir(t, f.anaA, "restrito", f.unA, "Corpo inválido")
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/tramite/processos/" + p.ID + "/concluir"},
		{http.MethodPost, "/api/v1/tramite/processos/" + p.ID + "/acessos"},
		{http.MethodPost, "/api/v1/tramite/processos/" + p.ID + "/uploads"},
		{http.MethodPost, "/api/v1/tramite/processos/" + p.ID + "/documentos"},
		{http.MethodPut, "/api/v1/tramite/documentos/" + uuid.NewString()},
		{http.MethodPost, "/api/v1/tramite/documentos/" + uuid.NewString() + "/assinatura"},
	} {
		f.h.expect(http.StatusBadRequest, c.method, c.path, f.anaA, `{`)
	}
}
