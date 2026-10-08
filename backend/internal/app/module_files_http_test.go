package app

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type fileTicket struct {
	File struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"file"`
	Upload struct {
		ObjectKey string `json:"object_key"`
	} `json:"upload"`
}

// uploadFile faz o fluxo completo: ticket, envio direto e confirmação.
func (h *apiHarness) uploadFile(token, folderID, name string, body []byte) string {
	h.t.Helper()
	tk := data[fileTicket](h.t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/files/uploads", token,
		`{"folder_id":"`+folderID+`","filename":"`+name+`","content_type":"application/pdf","size":`+strconv.Itoa(len(body))+`}`))
	h.upload(tk.Upload.ObjectKey, "application/pdf", body)
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/files/objects/"+tk.File.ID+"/confirm", token, "")
	return tk.File.ID
}

func (h *apiHarness) folder(token, name, parent string) string {
	h.t.Helper()
	body := `{"name":"` + name + `"`
	if parent != "" {
		body += `,"parent_id":"` + parent + `"`
	}
	return data[folderResp](h.t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/files/folders", token, body+`}`)).ID
}

// ACL por sujeito (pessoa, perfil, unidade, departamento, todos), herança
// pelas subpastas e escrita só quando concedida.
func TestFilesACLSubjects(t *testing.T) {
	h := newHarness(t)
	admin := h.admin()
	ctx := context.Background()
	sfx := uuid.NewString()[:6]
	_, owner := h.user("nexus-user")

	ent := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/entidades", admin, `{"nome":"Órgão `+sfx+`"}`))
	un := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", admin, `{"entidade_id":"`+ent.ID+`","nome":"Unidade `+sfx+`"}`))
	dep := data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/departamentos", admin, `{"unidade_id":"`+un.ID+`","nome":"Dep `+sfx+`"}`))
	var servidor string
	if err := h.d.DB.QueryRow(ctx, `SELECT id FROM perfis WHERE slug = 'servidor'`).Scan(&servidor); err != nil {
		t.Fatal(err)
	}
	lotado := func(scope string) (uuid.UUID, string) {
		id, tok := h.user("nexus-user")
		h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+id.String()+"/lotacoes", admin, `{"perfil_id":"`+servidor+`"`+scope+`}`)
		return id, tok
	}
	pessoaID, pessoa := h.user("nexus-user")
	_, daUnidade := lotado(`,"unidade_id":"` + un.ID + `"`)
	_, doDep := lotado(`,"departamento_id":"` + dep.ID + `"`)
	_, comPerfil := lotado("")
	_, estranho := h.user("nexus-user")

	root := h.folder(owner, "Projetos "+sfx, "")
	sub := h.folder(owner, "Contratos", root)
	arquivo := h.uploadFile(owner, sub, "contrato.pdf", []byte("%PDF-1.4 contrato"))

	browse := func(token, folder string) *httpRec {
		rec := h.do(http.MethodGet, "/api/v1/files/browse?folder_id="+folder, token, "")
		return &httpRec{code: rec.Code, body: rec.Body.String()}
	}
	for name, tok := range map[string]string{"pessoa": pessoa, "unidade": daUnidade, "departamento": doDep, "perfil": comPerfil, "estranho": estranho} {
		if r := browse(tok, sub); r.code != http.StatusForbidden {
			t.Fatalf("%s sem ACL não lê a pasta privada: %d", name, r.code)
		}
	}
	if !strings.Contains(h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/browse?folder_id="+sub, admin, "").Body.String(), arquivo) {
		t.Fatal("files:manage vê tudo")
	}

	acl := `{"entries":[
		{"subject_type":"user","subject":"` + pessoaID.String() + `","can_write":true},
		{"subject_type":"unidade","subject":"` + un.ID + `"},
		{"subject_type":"departamento","subject":"` + dep.ID + `"},
		{"subject_type":"perfil","subject":"SERVIDOR"},
		{"subject_type":"user","subject":"` + pessoaID.String() + `"}]}`
	saved := h.expect(http.StatusOK, http.MethodPut, "/api/v1/files/folders/"+root+"/acl", owner, acl).Body.String()
	if strings.Count(saved, pessoaID.String()) != 1 {
		t.Fatalf("entradas repetidas para o mesmo sujeito viram uma (com escrita): %s", saved)
	}
	got := h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/folders/"+root+"/acl", owner, "").Body.String()
	if !strings.Contains(got, `"can_write":true`) || strings.Count(got, `"subject_type"`) != 4 {
		t.Fatalf("ACL gravada: %s", got)
	}
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/files/folders/"+root+"/acl", pessoa, "")

	// Herança: todos com ACL leem a subpasta e baixam o arquivo.
	for name, tok := range map[string]string{"pessoa": pessoa, "unidade": daUnidade, "departamento": doDep, "perfil": comPerfil} {
		if r := browse(tok, sub); r.code != http.StatusOK || !strings.Contains(r.body, arquivo) {
			t.Fatalf("%s herda a leitura da pasta-mãe: %d %s", name, r.code, r.body)
		}
		h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/objects/"+arquivo+"/download", tok, "")
	}
	if r := browse(estranho, sub); r.code != http.StatusForbidden {
		t.Fatal("quem não está na ACL continua sem acesso")
	}
	// Escrita só para quem tem can_write.
	h.folder(pessoa, "Da pessoa", sub)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/files/folders", daUnidade, `{"name":"Sem escrita","parent_id":"`+sub+`"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/files/uploads", comPerfil,
		`{"folder_id":"`+sub+`","filename":"x.pdf","content_type":"application/pdf","size":10}`)

	// Na raiz, cada um vê só as pastas a que tem acesso.
	raiz := h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/browse", estranho, "").Body.String()
	if strings.Contains(raiz, root) || !strings.Contains(raiz, `"write":true`) {
		t.Fatalf("raiz: pastas visíveis e qualquer um cria a sua: %s", raiz)
	}
	if !strings.Contains(h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/browse", doDep, "").Body.String(), root) {
		t.Fatal("a pasta compartilhada aparece na raiz de quem tem acesso")
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/files/browse?folder_id="+uuid.NewString(), owner, "")
	h.expect(http.StatusBadRequest, http.MethodGet, "/api/v1/files/browse?folder_id=x", owner, "")

	// Busca global respeita a ACL.
	if res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q=contrato&module=files", estranho, "").Body.String(); strings.Contains(res, arquivo) {
		t.Fatal("a busca não mostra arquivo de pasta sem acesso")
	}
	if res := h.expect(http.StatusOK, http.MethodGet, "/api/v1/search?q=contrato&module=files", doDep, "").Body.String(); !strings.Contains(res, arquivo) {
		t.Fatalf("a busca mostra o arquivo a quem tem acesso: %s", res)
	}
}

// Mover e renomear: ciclos, destino sem escrita, raiz, nomes duplicados e
// o arquivo que o dono tentava levar para uma pasta somente leitura.
func TestFilesMoveRenameAndDelete(t *testing.T) {
	h := newHarness(t)
	sfx := uuid.NewString()[:6]
	ownerID, owner := h.user("nexus-user")
	colegaID, colega := h.user("nexus-user")
	_, leitor := h.user("nexus-user")

	a := h.folder(owner, "A "+sfx, "")
	b := h.folder(owner, "B", a)
	c := h.folder(owner, "C", b)
	patch := func(want int, token, id, body string) {
		h.expect(want, http.MethodPatch, "/api/v1/files/folders/"+id, token, body)
	}
	patch(http.StatusConflict, owner, a, `{"name":"A","parent_id":"`+c+`","move":true}`)
	patch(http.StatusConflict, owner, a, `{"name":"A","parent_id":"`+a+`","move":true}`)
	patch(http.StatusNotFound, owner, c, `{"name":"C","parent_id":"`+uuid.NewString()+`","move":true}`)
	patch(http.StatusUnprocessableEntity, owner, c, `{"name":"a/b"}`)
	h.folder(owner, "Irmã", b)
	patch(http.StatusConflict, owner, c, `{"name":"Irmã"}`)
	patch(http.StatusOK, owner, c, `{"name":"C renomeada"}`)
	patch(http.StatusNotFound, owner, uuid.NewString(), `{"name":"x"}`)

	// A colega recebe escrita em A; gerencia o que criar, não o resto.
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/files/folders/"+a+"/acl", owner,
		`{"entries":[{"subject_type":"user","subject":"`+colegaID.String()+`","can_write":true},{"subject_type":"everyone"}]}`)
	daColega := h.folder(colega, "Da colega", b)
	patch(http.StatusForbidden, colega, b, `{"name":"B renomeada"}`)
	// Dono de pasta acima gerencia a da colega, mas não a leva para a raiz
	// (ela perderia a ACL herdada e ele o controle); a própria colega leva.
	patch(http.StatusOK, owner, daColega, `{"name":"Da colega (revisada)"}`)
	patch(http.StatusForbidden, owner, daColega, `{"name":"Da colega","move":true}`)
	somenteLeitura := h.folder(leitor, "Leitura "+sfx, "")
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/files/folders/"+somenteLeitura+"/acl", leitor, `{"entries":[{"subject_type":"everyone"}]}`)
	patch(http.StatusForbidden, colega, daColega, `{"name":"Da colega","parent_id":"`+somenteLeitura+`","move":true}`)
	patch(http.StatusOK, colega, daColega, `{"name":"Da colega na raiz `+sfx+`","move":true}`)

	// Arquivo: o dono não coloca o arquivo numa pasta somente leitura.
	arq := h.uploadFile(colega, b, "planilha.pdf", []byte("%PDF-1.4 planilha"))
	mover := func(want int, token, folder, name string) {
		h.expect(want, http.MethodPatch, "/api/v1/files/objects/"+arq, token, `{"name":"`+name+`","folder_id":"`+folder+`"}`)
	}
	mover(http.StatusForbidden, colega, somenteLeitura, "planilha.pdf")
	mover(http.StatusNotFound, colega, uuid.NewString(), "planilha.pdf")
	mover(http.StatusForbidden, leitor, c, "planilha.pdf")
	mover(http.StatusUnprocessableEntity, colega, c, "../planilha.pdf")
	mover(http.StatusOK, colega, c, "planilha final.pdf")
	h.expect(http.StatusNotFound, http.MethodPatch, "/api/v1/files/objects/"+uuid.NewString(), colega, `{"name":"x","folder_id":"`+c+`"}`)

	// Exclusão: dono do arquivo ou quem gerencia a pasta; leitor não.
	h.expect(http.StatusForbidden, http.MethodDelete, "/api/v1/files/objects/"+arq, leitor, "")
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/files/objects/"+arq, colega, "")
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/files/objects/"+arq, colega, "")
	outro := h.uploadFile(colega, c, "outro.pdf", []byte("%PDF-1.4 outro"))
	h.expect(http.StatusNoContent, http.MethodDelete, "/api/v1/files/objects/"+outro, owner, "")
	h.expect(http.StatusNotFound, http.MethodDelete, "/api/v1/files/folders/"+uuid.NewString(), owner, "")
	_ = ownerID
}

// Upload: limites, tipos, confirmação só pelo dono e idempotente, arquivo
// recusado removido do armazenamento, download só de arquivo pronto.
func TestFilesUploadRules(t *testing.T) {
	h := newHarness(t)
	_, owner := h.user("nexus-user")
	_, other := h.user("nexus-user")
	folder := h.folder(owner, "Uploads "+uuid.NewString()[:6], "")
	start := func(want int, body string) {
		h.expect(want, http.MethodPost, "/api/v1/files/uploads", owner, body)
	}
	start(http.StatusUnprocessableEntity, `{"folder_id":"`+folder+`","filename":"a.pdf","content_type":"application/pdf","size":999999999999}`)
	start(http.StatusUnprocessableEntity, `{"folder_id":"`+folder+`","filename":"a/b.pdf","content_type":"application/pdf","size":10}`)
	start(http.StatusUnprocessableEntity, `{"folder_id":"`+folder+`","filename":"a.exe","content_type":"application/x-msdownload","size":10}`)
	start(http.StatusUnprocessableEntity, `{"folder_id":"`+folder+`","filename":"a.pdf","content_type":"application/pdf","size":0}`)
	start(http.StatusNotFound, `{"folder_id":"`+uuid.NewString()+`","filename":"a.pdf","content_type":"application/pdf","size":10}`)
	start(http.StatusBadRequest, `{`)

	tk := data[fileTicket](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/files/uploads", owner,
		`{"folder_id":"`+folder+`","filename":"falso.pdf","content_type":"application/pdf","size":10}`))
	if tk.File.Status != "pending" {
		t.Fatalf("registro nasce pendente: %+v", tk)
	}
	if strings.Contains(h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/browse?folder_id="+folder, owner, "").Body.String(), tk.File.ID) {
		t.Fatal("pendente não aparece na listagem")
	}
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/files/objects/"+tk.File.ID+"/download", owner, "")
	// O navegador enviou outra coisa (executável): recusado e apagado.
	h.upload(tk.Upload.ObjectKey, "application/x-msdownload", []byte("MZ executável"))
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/files/objects/"+tk.File.ID+"/confirm", other, "")
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/files/objects/"+tk.File.ID+"/confirm", owner, "")
	if _, err := h.d.Storage.Stat(context.Background(), h.d.Config.MinIO.Bucket, tk.Upload.ObjectKey); err == nil {
		t.Fatal("arquivo recusado sai do armazenamento")
	}
	id := h.uploadFile(owner, folder, "bom.pdf", []byte("%PDF-1.4 bom"))
	h.expect(http.StatusOK, http.MethodPost, "/api/v1/files/objects/"+id+"/confirm", owner, "") // idempotente
	if outboxCount(t, h, "files.object.uploaded", id) != 1 {
		t.Fatal("reconfirmar não emite o evento de novo")
	}
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/files/objects/"+uuid.NewString()+"/confirm", owner, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/files/objects/"+uuid.NewString()+"/download", owner, "")
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPatch, "/api/v1/files/folders/x", `{"name":"a"}`},
		{http.MethodDelete, "/api/v1/files/folders/x", ""},
		{http.MethodGet, "/api/v1/files/folders/x/acl", ""},
		{http.MethodPut, "/api/v1/files/folders/x/acl", `{"entries":[]}`},
		{http.MethodPost, "/api/v1/files/objects/x/confirm", ""},
		{http.MethodGet, "/api/v1/files/objects/x/download", ""},
		{http.MethodPatch, "/api/v1/files/objects/x", `{"name":"a","folder_id":"` + folder + `"}`},
		{http.MethodDelete, "/api/v1/files/objects/x", ""},
		{http.MethodPost, "/api/v1/files/folders", `{`},
		{http.MethodPatch, "/api/v1/files/folders/" + folder, `{`},
		{http.MethodPut, "/api/v1/files/folders/" + folder + "/acl", `{`},
		{http.MethodPatch, "/api/v1/files/objects/" + id, `{`},
	} {
		h.expect(http.StatusBadRequest, c.method, c.path, owner, c.body)
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/files/folders/"+folder+"/acl", owner, `{"entries":[{"subject_type":"perfil","subject":"  "}]}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/files/folders/"+folder+"/acl", owner, `{"entries":[{"subject_type":"grupo","subject":"x"}]}`)
}

// Pasta da unidade (ADR 013, fase 3): a pasta pode ter unidade dona,
// herdada pelas subpastas; files:manage com escopo tem acesso total às
// pastas da área que cobre. Marca a dona quem é lotado nela ou a gerencia.
func TestFilesPastaDaUnidade(t *testing.T) {
	h := newHarness(t)
	o := h.orgEscopo(t)
	sfx := uuid.NewString()[:6]
	gestorA := h.gestorEm(t, o.a, "files:manage")
	lotadoSub := h.gestorEm(t, o.sub, "contact:read")
	lotadoB := h.gestorEm(t, o.b, "contact:read")
	_, estranho := h.user("nexus-user")
	criar := func(want int, token, name, parent, unidade string) string {
		body := `{"name":"` + name + `"`
		if parent != "" {
			body += `,"parent_id":"` + parent + `"`
		}
		if unidade != "" {
			body += `,"unidade_id":"` + unidade + `"`
		}
		rec := h.expect(want, http.MethodPost, "/api/v1/files/folders", token, body+`}`)
		if want != http.StatusCreated {
			return ""
		}
		return data[folderResp](t, rec).ID
	}
	patch := func(want int, token, id, body string) {
		h.expect(want, http.MethodPatch, "/api/v1/files/folders/"+id, token, body)
	}

	criar(http.StatusForbidden, estranho, "Alheia "+sfx, "", o.b)
	deB := criar(http.StatusCreated, lotadoB, "Pasta B "+sfx, "", o.b)
	daSub := criar(http.StatusCreated, lotadoSub, "Pasta Sub "+sfx, "", o.sub)
	interna := criar(http.StatusCreated, lotadoSub, "Interna", daSub, "")

	// Herança: a gestão de A (cobre a subunidade) gerencia a subpasta.
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/files/browse?folder_id="+interna, gestorA, "")
	patch(http.StatusOK, gestorA, interna, `{"name":"Interna (revisada)"}`)
	patch(http.StatusForbidden, gestorA, deB, `{"name":"Pasta B tomada"}`)
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/files/browse?folder_id="+deB, gestorA, "")
	patch(http.StatusForbidden, gestorA, interna, `{"name":"Interna","set_unidade":true,"unidade_id":"`+o.b+`"}`)

	// Na raiz a subpasta perderia a dona herdada; marcada, a gestão a leva.
	patch(http.StatusForbidden, gestorA, interna, `{"name":"Interna","move":true}`)
	patch(http.StatusOK, gestorA, interna, `{"name":"Interna","set_unidade":true,"unidade_id":"`+o.sub+`"}`)
	patch(http.StatusOK, gestorA, interna, `{"name":"Interna","set_unidade":true,"unidade_id":"`+o.a+`"}`) // troca de dona dentro da área
	patch(http.StatusOK, gestorA, interna, `{"name":"Interna na raiz `+sfx+`","move":true}`)

	// Tirar a dona: quem é lotado nela pode; a gestão perde o acesso.
	patch(http.StatusForbidden, estranho, daSub, `{"name":"Pasta Sub","set_unidade":true}`)
	patch(http.StatusOK, lotadoSub, daSub, `{"name":"Pasta Sub `+sfx+`","set_unidade":true}`)
	patch(http.StatusForbidden, gestorA, daSub, `{"name":"Pasta Sub tomada"}`)
}
