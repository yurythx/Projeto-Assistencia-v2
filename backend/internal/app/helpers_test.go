package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/passwords"
	"github.com/yurythx/projeto-nexus/internal/platform/storage/storagetest"
)

// apiHarness é o sistema completo (router real + Postgres de teste) com
// helpers para autenticar como diferentes perfis.
type apiHarness struct {
	t      *testing.T
	d      *Dependencies
	router http.Handler
}

func newHarness(t *testing.T) *apiHarness {
	t.Helper()
	d := testDeps(t)
	return &apiHarness{t: t, d: d, router: NewRouter(d)}
}

// user cria uma conta local com os papéis dados e devolve (id, token).
func (h *apiHarness) user(roles ...string) (uuid.UUID, string) {
	h.t.Helper()
	username := "t_" + strings.ReplaceAll(uuid.NewString()[:10], "-", "")
	hash, err := passwords.Hash("Senha-Forte-123!")
	if err != nil {
		h.t.Fatal(err)
	}
	var id uuid.UUID
	if err := h.d.DB.QueryRow(context.Background(),
		`INSERT INTO users (username, email, display_name, password_hash, roles)
		 VALUES ($1, $1 || '@nexus.test', 'Usuário ' || $1, $2, $3) RETURNING id`,
		username, hash, roles).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	rec := h.do(http.MethodPost, "/api/v1/auth/login", "", `{"username":"`+username+`","password":"Senha-Forte-123!"}`)
	if rec.Code != http.StatusOK {
		h.t.Fatalf("login %s: %d %s", username, rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return id, out.Data.AccessToken
}

// upload simula o PUT direto do navegador no MinIO (URL pré-assinada).
func (h *apiHarness) upload(objectKey, contentType string, body []byte) {
	h.t.Helper()
	st := h.d.Storage.(*storagetest.Memory)
	if err := st.Put(context.Background(), h.d.Config.MinIO.Bucket, objectKey, bytes.NewReader(body), int64(len(body)), contentType); err != nil {
		h.t.Fatal(err)
	}
}

func (h *apiHarness) admin() string {
	_, tok := h.user("nexus-admin")
	return tok
}

func (h *apiHarness) do(method, path, token, body string) *httptest.ResponseRecorder {
	return h.doFrom("203.0.113.10:1234", method, path, token, body)
}

// doFrom faz a requisição a partir de um IP de origem específico.
func (h *apiHarness) doFrom(remote, method, path, token, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remote
	req.Header.Set("User-Agent", "nexus-test/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// expect executa a requisição e falha se o status não for o esperado.
func (h *apiHarness) expect(want int, method, path, token, body string) *httptest.ResponseRecorder {
	h.t.Helper()
	rec := h.do(method, path, token, body)
	if rec.Code != want {
		h.t.Fatalf("%s %s: esperado %d, veio %d %s", method, path, want, rec.Code, rec.Body.String())
	}
	return rec
}

// data decodifica o campo "data" do envelope padrão.
func data[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var env struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("resposta ilegível: %v — %s", err, rec.Body.String())
	}
	return env.Data
}

// setModule liga/desliga um módulo pela API respeitando o grafo:
// desligar desliga antes os dependentes; ligar liga antes as dependências.
// Registra a restauração do estado original ao fim do teste.
func (h *apiHarness) setModule(token, key string, enabled bool) {
	h.t.Helper()
	status := map[string]struct {
		DependsOn  []string `json:"depends_on"`
		Dependents []string `json:"dependents"`
		Enabled    bool     `json:"enabled"`
	}{}
	for _, s := range data[[]struct {
		Key        string   `json:"key"`
		DependsOn  []string `json:"depends_on"`
		Dependents []string `json:"dependents"`
		Enabled    bool     `json:"enabled"`
	}](h.t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/system/modules", token, "")) {
		status[s.Key] = struct {
			DependsOn  []string `json:"depends_on"`
			Dependents []string `json:"dependents"`
			Enabled    bool     `json:"enabled"`
		}{s.DependsOn, s.Dependents, s.Enabled}
	}
	st := status[key]
	if st.Enabled == enabled {
		return
	}
	if enabled {
		for _, dep := range st.DependsOn {
			h.setModule(token, dep, true)
		}
	} else {
		for _, dep := range st.Dependents {
			h.setModule(token, dep, false)
		}
	}
	h.expect(http.StatusOK, http.MethodPatch, "/api/v1/admin/modules/"+key, token, `{"enabled":`+boolStr(enabled)+`}`)
	h.t.Cleanup(func() {
		ctx := context.Background()
		if enabled {
			_ = h.d.Kernel.SetEnabled(ctx, key, false, "test-restore")
		} else {
			_ = h.d.Kernel.SetEnabled(ctx, key, true, "test-restore")
		}
	})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// deliver faz o papel do worker: lê do outbox os eventos eventType do
// agregado e os entrega a todo consumidor de plug-in inscrito naquela
// routing key. Devolve quantas entregas foram feitas.
func (h *apiHarness) deliver(eventType, aggregateID string) int {
	h.t.Helper()
	ctx := context.Background()
	rows, err := h.d.DB.Query(ctx, `SELECT payload FROM outbox_events WHERE event_type = $1 AND aggregate_id = $2 ORDER BY created_at`,
		eventType, aggregateID)
	if err != nil {
		h.t.Fatal(err)
	}
	var envelopes [][]byte
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			h.t.Fatal(err)
		}
		envelopes = append(envelopes, raw)
	}
	rows.Close()
	n := 0
	for _, raw := range envelopes {
		var ev events.Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			h.t.Fatal(err)
		}
		for _, p := range h.d.Kernel.Plugins() {
			cp, ok := p.(kernel.ConsumerProvider)
			if !ok {
				continue
			}
			for _, c := range cp.Consumers() {
				if !slices.Contains(c.Queue.RoutingKeys, eventType) {
					continue
				}
				if err := c.Handler(ctx, ev); err != nil {
					h.t.Fatalf("consumidor %s falhou em %s: %v", c.Queue.Name, eventType, err)
				}
				n++
			}
		}
	}
	return n
}
