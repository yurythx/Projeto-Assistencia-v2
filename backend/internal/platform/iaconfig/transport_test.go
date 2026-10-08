package iaconfig

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

type api struct {
	t     *testing.T
	store *memStore
	r     chi.Router
}

func novaAPI(t *testing.T, store *memStore, identidade auth.Identity, ambiente *Conexao) *api {
	h := NewHandlers(store, NovoCliente(), ambiente, audit.NewWriter(dbtest.Fail{}), logger)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), identidade)))
		})
	})
	RegisterRoutes(r, h, logger)
	return &api{t: t, store: store, r: r}
}

func (a *api) req(method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	a.r.ServeHTTP(rec, req)
	return rec
}

func (a *api) expect(code int, method, path, body string) string {
	a.t.Helper()
	rec := a.req(method, path, body)
	if rec.Code != code {
		a.t.Fatalf("%s %s: %d (quero %d) %s", method, path, rec.Code, code, rec.Body.String())
	}
	return rec.Body.String()
}

var admin = auth.Identity{Subject: "sub-1", Username: "admin", Permissions: []string{"ia:manage"},
	Scopes: []auth.Scope{{Perfil: "p", Permissions: []string{"ia:manage"}}}}

func corpo(endpoint, extra string) string {
	return `{"nome":"Local","provedor":"ollama","endpoint":"` + endpoint + `","modelo":"qwen2.5:1.5b"` + extra + `}`
}

func TestRotasExigemPermissao(t *testing.T) {
	a := novaAPI(t, novoMem(), auth.Identity{Subject: "x"}, nil)
	a.expect(http.StatusForbidden, http.MethodGet, "/ia/conexoes", "")
}

func TestConexoesCRUD(t *testing.T) {
	ok := fornecedor(t, responde("OK"))
	outro := fornecedor(t, responde("OK"))
	m := novoMem()
	a := novaAPI(t, m, admin, nil)

	if body := a.expect(http.StatusOK, http.MethodGet, "/ia/provedores", ""); !strings.Contains(body, `"anthropic"`) {
		t.Fatalf("catálogo: %s", body)
	}
	// Criar: valida, testa e só grava se o teste passar.
	a.expect(http.StatusBadRequest, http.MethodPost, "/ia/conexoes", `{`)
	a.expect(http.StatusUnprocessableEntity, http.MethodPost, "/ia/conexoes", corpo("ftp://x", ""))
	if body := a.expect(http.StatusUnprocessableEntity, http.MethodPost, "/ia/conexoes", corpo("http://127.0.0.1:1", "")); !strings.Contains(body, "nada foi salvo") {
		t.Fatalf("teste falho: %s", body)
	}
	if len(m.conexoes) != 0 {
		t.Fatal("gravou sem o teste passar")
	}
	var criada ConexaoPublica
	_ = json.Unmarshal([]byte(a.expect(http.StatusCreated, http.MethodPost, "/ia/conexoes",
		`{"nome":"Nuvem","provedor":"openai","endpoint":"`+ok.URL+`","modelo":"gpt","chave":"sk-1234567890abcd"}`)), &struct {
		Data *ConexaoPublica `json:"data"`
	}{&criada})
	if !criada.Externo || !criada.TemChave || criada.ChaveFinal != "abcd" || criada.UltimoTeste == nil || !criada.UltimoTeste.OK ||
		m.testes != 1 || m.conexoes[criada.ID].UpdatedBy != "admin" {
		t.Fatalf("criada: %+v", criada)
	}
	if body := a.expect(http.StatusOK, http.MethodGet, "/ia/conexoes", ""); strings.Contains(body, "sk-1234") || !strings.Contains(body, criada.ID.String()) {
		t.Fatalf("lista sem a chave: %s", body)
	}
	id := "/ia/conexoes/" + criada.ID.String()

	// Alterar mantendo a chave (mesmo endereço); trocar o endereço exige a chave.
	a.expect(http.StatusOK, http.MethodPut, id, `{"nome":"Nuvem 2","provedor":"openai","endpoint":"`+ok.URL+`/","modelo":"gpt"}`)
	if m.conexoes[criada.ID].Chave != "sk-1234567890abcd" || m.conexoes[criada.ID].Nome != "Nuvem 2" {
		t.Fatalf("chave mantida: %+v", m.conexoes[criada.ID])
	}
	if body := a.expect(http.StatusUnprocessableEntity, http.MethodPut, id, `{"nome":"N","provedor":"openai","endpoint":"`+outro.URL+`","modelo":"gpt"}`); !strings.Contains(body, "informe a chave") {
		t.Fatalf("troca de endereço: %s", body)
	}
	a.expect(http.StatusOK, http.MethodPut, id, `{"nome":"N","provedor":"openai","endpoint":"`+outro.URL+`","modelo":"gpt","chave":"nova-chave-123456"}`)
	// Remover a chave de quem exige: 422.
	a.expect(http.StatusUnprocessableEntity, http.MethodPut, id, `{"nome":"N","provedor":"openai","endpoint":"`+outro.URL+`","modelo":"gpt","remover_chave":true}`)
	a.expect(http.StatusBadRequest, http.MethodPut, "/ia/conexoes/x", `{}`)
	a.expect(http.StatusBadRequest, http.MethodPut, id, `{`)
	a.expect(http.StatusNotFound, http.MethodPut, "/ia/conexoes/"+uuid.NewString(), corpo(ok.URL, ""))

	// Testar a conexão salva: 200 com o resultado, mesmo se falhar.
	if body := a.expect(http.StatusOK, http.MethodPost, id+"/testar", ""); !strings.Contains(body, `"ok":true`) {
		t.Fatalf("testar: %s", body)
	}
	a.expect(http.StatusBadRequest, http.MethodPost, "/ia/conexoes/x/testar", "")
	a.expect(http.StatusNotFound, http.MethodPost, "/ia/conexoes/"+uuid.NewString()+"/testar", "")
	m.errTeste = errors.New("db")
	a.expect(http.StatusInternalServerError, http.MethodPost, id+"/testar", "")
	a.expect(http.StatusInternalServerError, http.MethodPost, "/ia/conexoes", corpo(ok.URL, ""))
	m.errTeste = nil
	m.errSalvar = ErrNomeRepetido
	a.expect(http.StatusConflict, http.MethodPost, "/ia/conexoes", corpo(ok.URL, ""))
	m.errSalvar = nil

	// Excluir: em uso = 409; inexistente = 404.
	m.errExcluir = ErrEmUso
	a.expect(http.StatusConflict, http.MethodDelete, id, "")
	m.errExcluir = nil
	a.expect(http.StatusBadRequest, http.MethodDelete, "/ia/conexoes/x", "")
	a.expect(http.StatusNotFound, http.MethodDelete, "/ia/conexoes/"+uuid.NewString(), "")
	a.expect(http.StatusNoContent, http.MethodDelete, id, "")

	m.errListar = errors.New("db")
	a.expect(http.StatusInternalServerError, http.MethodGet, "/ia/conexoes", "")
}

func TestUso(t *testing.T) {
	local := Conexao{ID: uuid.New(), Nome: "Local", Provedor: ProvedorLocal}
	nuvem := Conexao{ID: uuid.New(), Nome: "Nuvem", Provedor: ProvedorOpenAI, Externo: true}
	m := novoMem(local, nuvem)
	ambiente := ConexaoDoAmbiente("http://ia-local:11434", "", "qwen", 60)
	a := novaAPI(t, m, auth.Identity{Subject: "sub-9", Permissions: []string{"*"}}, ambiente)
	const uso = "/ia/uso/atlas.assistente"

	if body := a.expect(http.StatusOK, http.MethodGet, uso, ""); !strings.Contains(body, `"configurado":false`) ||
		!strings.Contains(body, `"ambiente":{`) || !strings.Contains(body, `"mascarar_dados_pessoais":true`) {
		t.Fatalf("sem configuração: %s", body)
	}
	a.expect(http.StatusNotFound, http.MethodGet, "/ia/uso/outra", "")
	a.expect(http.StatusNotFound, http.MethodPut, "/ia/uso/outra", `{}`)
	a.expect(http.StatusBadRequest, http.MethodPut, uso, `{`)

	put := func(code int, body string) string { return a.expect(code, http.MethodPut, uso, body) }
	put(http.StatusUnprocessableEntity, `{"reserva_id":"`+local.ID.String()+`"}`)
	put(http.StatusUnprocessableEntity, `{"principal_id":"`+local.ID.String()+`","reserva_id":"`+local.ID.String()+`"}`)
	if body := put(http.StatusUnprocessableEntity, `{"principal_id":"`+uuid.NewString()+`"}`); !strings.Contains(body, "inexistente") {
		t.Fatalf("conexão inexistente: %s", body)
	}
	// Externa exige autorização explícita; com ela, registra quem autorizou.
	if body := put(http.StatusUnprocessableEntity, `{"principal_id":"`+local.ID.String()+`","reserva_id":"`+nuvem.ID.String()+`"}`); !strings.Contains(body, "Nuvem") {
		t.Fatalf("externa sem autorização: %s", body)
	}
	body := put(http.StatusOK, `{"principal_id":"`+local.ID.String()+`","reserva_id":"`+nuvem.ID.String()+`","mascarar_dados_pessoais":true,"autorizo_envio_externo":true}`)
	if !strings.Contains(body, `"externo_autorizado_por":"sub-9"`) || m.uso.ExternoAutorizadoEm == nil || m.uso.UpdatedBy != "sub-9" {
		t.Fatalf("autorizada: %s", body)
	}
	// Só local: sem autorização registrada. Desligar: principal nula.
	if body := put(http.StatusOK, `{"principal_id":"`+local.ID.String()+`"}`); !strings.Contains(body, `"externo_autorizado_por":""`) {
		t.Fatalf("só local: %s", body)
	}
	if put(http.StatusOK, `{}`); m.uso.PrincipalID != nil || !m.uso.Configurado {
		t.Fatalf("desligada: %+v", m.uso)
	}

	// Falhas do armazenamento.
	m.errDefinir = errors.New("db")
	put(http.StatusInternalServerError, `{}`)
	m.errObter = errors.New("db")
	put(http.StatusInternalServerError, `{"principal_id":"`+local.ID.String()+`"}`)
	m.errUso = errors.New("db")
	a.expect(http.StatusInternalServerError, http.MethodGet, uso, "")
}

func TestMapErrorEMesmoEndpoint(t *testing.T) {
	outro := errors.New("x")
	if MapError(outro) != outro {
		t.Fatal("erro genérico passa direto")
	}
	if !mesmoEndpoint("HTTPS://API.x/v1/", " https://api.x/v1") || mesmoEndpoint("https://a", "https://b") {
		t.Fatal("comparação de endereços")
	}
}
