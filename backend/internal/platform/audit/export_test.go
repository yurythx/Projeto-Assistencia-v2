package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestExportPaginatesWithCursor(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	action := "export.page." + uuid.NewString()[:8]
	w := NewWriter(pool)
	for i := 0; i < 3; i++ {
		if err := w.Record(ctx, Entry{Action: action, IPAddress: "192.168.0.9"}); err != nil {
			t.Fatal(err)
		}
	}
	exp := NewExporter(pool, quietLogger())
	exp.maxRows = 2
	h := router(NewReader(pool), exp)
	var seen []string
	cursor := ""
	for page := 0; page < 3; page++ {
		rec := get(h, "/audit/export?format=json&action="+action+"&cursor="+cursor)
		var env struct {
			Dataset struct {
				Rows       int    `json:"rows"`
				NextCursor string `json:"next_cursor"`
			} `json:"dataset"`
			Rows []exportRow `json:"rows"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &env) != nil {
			t.Fatalf("página %d: %d %s", page, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Next-Cursor") != env.Dataset.NextCursor {
			t.Fatal("cursor no cabeçalho e no corpo")
		}
		for _, r := range env.Rows {
			if r.IPAddress != "192.168.0.9" {
				t.Fatalf("IP sai sem máscara: %q", r.IPAddress)
			}
			seen = append(seen, r.ID)
		}
		cursor = env.Dataset.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 3 || seen[0] == seen[2] || cursor != "" {
		t.Fatalf("3 linhas em 2 páginas, sem repetir: %v (cursor %q)", seen, cursor)
	}
}

func TestExportFailures(t *testing.T) {
	for path, want := range map[string]int{
		"/audit/export?from=ontem":  http.StatusBadRequest,
		"/audit/export?cursor=xyz":  http.StatusBadRequest,
		"/audit/export?format=json": http.StatusInternalServerError, // consulta falha
	} {
		if rec := get(router(NewReader(dbtest.Fail{}), NewExporter(dbtest.Fail{}, quietLogger())), path); rec.Code != want {
			t.Errorf("%s: %d, esperado %d", path, rec.Code, want)
		}
	}
	for name, rows := range map[string]dbtest.QueryResult{
		"linha ilegível":       {Rows: dbtest.BadRows()},
		"leitura interrompida": {Rows: dbtest.ErrRows()},
	} {
		exp := NewExporter(&dbtest.Seq{Queries: []dbtest.QueryResult{rows}}, quietLogger())
		if rec := get(router(NewReader(dbtest.Fail{}), exp), "/audit/export"); rec.Code != http.StatusInternalServerError {
			t.Errorf("%s: relatório incompleto não pode sair (%d)", name, rec.Code)
		}
	}
	// A trilha da própria exportação falhar não muda a resposta já escrita.
	exp := NewExporter(&dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}}}, quietLogger())
	if rec := get(router(NewReader(dbtest.Fail{}), exp), "/audit/export?format=csv"); rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "ID,") {
		t.Errorf("exportação sem trilha: %d %s", rec.Code, rec.Body.String())
	}
	// Defesa em profundidade: sem identidade, 401 mesmo fora do middleware.
	rec := httptest.NewRecorder()
	exp.handleExport(rec, httptest.NewRequest(http.MethodGet, "/audit/export", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("sem identidade: %d", rec.Code)
	}
}
