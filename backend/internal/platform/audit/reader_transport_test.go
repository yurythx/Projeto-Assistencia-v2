package audit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func TestReaderFiltersAndEscapesWildcards(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	actor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]
	action := "esc." + sfx + ".a_b"
	if err := NewWriter(pool).Record(ctx, Entry{ActorID: &actor, Action: action, ResourceType: "rt", ResourceID: sfx}); err != nil {
		t.Fatal(err)
	}
	r := NewReader(pool)
	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	p := pagination.New(1, 10, 10)
	recs, total, err := r.List(ctx, Filter{Action: "esc." + sfx + ".*", ActorID: &actor, ResourceType: "rt", ResourceID: sfx, From: &from, To: &to}, p)
	if err != nil || total != 1 || len(recs) != 1 || recs[0].Action != action {
		t.Fatalf("todos os filtros: %d %v %v", total, recs, err)
	}
	for _, f := range []string{"esc." + sfx + ".a%", "esc." + sfx + ".__b", "esc." + sfx + "_a_b"} {
		if _, n, err := r.List(ctx, Filter{Action: f}, p); err != nil || n != 0 {
			t.Errorf("%q: %% e _ digitados valem como texto, não como curinga (%d, %v)", f, n, err)
		}
	}
	got, err := r.Get(ctx, recs[0].ID, nil)
	if err != nil || got.ResourceID != sfx || got.Hash == "" {
		t.Fatalf("Get: %+v %v", got, err)
	}
	if _, err := r.Get(ctx, uuid.New(), nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("Get inexistente: %v", err)
	}
	if res, err := r.Verify(ctx, 1, 5); err != nil || res.Checked > 5 {
		t.Fatalf("Verify limitado: %+v %v", res, err)
	}
}

func TestReaderPropagatesDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	p := pagination.New(1, 10, 10)
	count := scanRow(func(dest ...any) error { *(dest[0].(*int64)) = 1; return nil })
	for name, db := range map[string]database.DBTX{
		"contagem":             dbtest.Fail{},
		"página":               &dbtest.Seq{Row: count, Queries: []dbtest.QueryResult{{Err: dbtest.ErrInjected}}},
		"linha ilegível":       &dbtest.Seq{Row: count, Queries: []dbtest.QueryResult{{Rows: dbtest.BadRows()}}},
		"leitura interrompida": &dbtest.Seq{Row: count, Queries: []dbtest.QueryResult{{Rows: dbtest.ErrRows()}}},
	} {
		if _, _, err := NewReader(db).List(ctx, Filter{}, p); err == nil {
			t.Errorf("List (%s) deveria falhar", name)
		}
	}
	if _, err := NewReader(dbtest.Fail{}).Verify(ctx, 1, 0); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("Verify com o banco fora: %v", err)
	}
	invalid := scanRow(func(dest ...any) error {
		*(dest[0].(*int64)) = 7
		pos := int64(3)
		*(dest[2].(**int64)) = &pos
		reason := "hash divergente"
		*(dest[3].(**string)) = &reason
		return nil
	})
	res, err := NewReader(&dbtest.Seq{Row: invalid}).Verify(ctx, 1, 0)
	if err != nil || res.Reason != "hash divergente" || *res.FirstInvalidPos != 3 {
		t.Fatalf("motivo da quebra da cadeia: %+v %v", res, err)
	}
}

// router monta as rotas de auditoria com um usuário de permissão total.
func router(reader *Reader, exp *Exporter) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Permissions: []string{"*"}, Roles: []string{auth.RoleAdmin}})))
		})
	})
	RegisterRoutes(r, NewHandlers(reader, exp, quietLogger(), 100))
	return r
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHandlers(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	actor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]
	if err := NewWriter(pool).Record(ctx, Entry{ActorID: &actor, Action: "handlers." + sfx, ResourceID: sfx}); err != nil {
		t.Fatal(err)
	}
	h := router(NewReader(pool), NewExporter(pool, quietLogger()))
	rec := get(h, "/audit/logs?actor_id="+actor.String()+"&from=2000-01-01&to="+time.Now().Add(time.Hour).UTC().Format(time.RFC3339)+"&action=handlers."+sfx)
	var list struct {
		Data []Record `json:"data"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Data) != 1 {
		t.Fatalf("lista filtrada: %d %s", rec.Code, rec.Body.String())
	}
	id := list.Data[0].ID.String()
	for path, want := range map[string]int{
		"/audit/logs/" + id:               http.StatusOK,
		"/audit/logs/" + uuid.NewString(): http.StatusNotFound,
		"/audit/logs/nao-uuid":            http.StatusBadRequest,
		"/audit/logs?to=amanha":           http.StatusBadRequest,
		"/audit/verify?from=0&limit=3":    http.StatusOK,
	} {
		if rec := get(h, path); rec.Code != want {
			t.Errorf("%s: %d, esperado %d (%s)", path, rec.Code, want, rec.Body.String())
		}
	}

	// Banco fora: 500 sem vazar o erro interno.
	down := router(NewReader(dbtest.Fail{}), NewExporter(dbtest.Fail{}, quietLogger()))
	for _, path := range []string{"/audit/logs", "/audit/logs/" + id, "/audit/verify"} {
		if rec := get(down, path); rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), dbtest.ErrInjected.Error()) {
			t.Errorf("%s com o banco fora: %d %s", path, rec.Code, rec.Body.String())
		}
	}

	// A verificação responde mesmo se a trilha do próprio ato falhar.
	ok := scanRow(func(dest ...any) error { *(dest[0].(*int64)) = 1; *(dest[1].(*bool)) = true; return nil })
	noAudit := router(NewReader(&dbtest.Seq{Row: ok}), NewExporter(dbtest.Fail{}, quietLogger()))
	if rec := get(noAudit, "/audit/verify"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"valid":true`) {
		t.Errorf("verificação sem trilha: %d %s", rec.Code, rec.Body.String())
	}
}

// Área (ADR 013): o registro entra na área pela lotação do autor gravada
// nele — unidade, departamento ou entidade; sem lotação, só a trilha toda.
func TestReaderRestritoAArea(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	sfx := uuid.NewString()[:8]
	ent, un, dep := uuid.New(), uuid.New(), uuid.New()
	ids := map[string]uuid.UUID{}
	for nome, scopes := range map[string]any{
		"unidade":      []map[string]any{{"entidade_id": ent, "unidade_id": un}},
		"departamento": []map[string]any{{"departamento_id": dep}},
		"entidade":     []map[string]any{{"entidade_id": ent}},
		"sem-lotacao":  nil,
		"invalido":     "x",
	} {
		e := Entry{Action: "area." + sfx, ResourceID: nome}
		if scopes != nil {
			e.EntityContext = map[string]any{"scopes": scopes}
		}
		if err := NewWriter(pool).Record(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	r := NewReader(pool)
	p := pagination.New(1, 10, 10)
	all, _, err := r.List(ctx, Filter{Action: "area." + sfx}, p)
	if err != nil || len(all) != 5 {
		t.Fatalf("trilha toda: %d %v", len(all), err)
	}
	for _, rec := range all {
		ids[rec.ResourceID] = rec.ID
	}
	veem := func(area Area) []string {
		recs, _, err := r.List(ctx, Filter{Action: "area." + sfx, Area: &area}, p)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, rec := range recs {
			out = append(out, rec.ResourceID)
		}
		slices.Sort(out)
		return out
	}
	if got := veem(Area{Unidades: []uuid.UUID{un}}); !slices.Equal(got, []string{"unidade"}) {
		t.Errorf("área da unidade: %v", got)
	}
	if got := veem(Area{Departamentos: []uuid.UUID{dep}}); !slices.Equal(got, []string{"departamento"}) {
		t.Errorf("área do departamento: %v", got)
	}
	if got := veem(Area{Entidades: []uuid.UUID{ent}}); !slices.Equal(got, []string{"entidade", "unidade"}) {
		t.Errorf("área da entidade: %v", got)
	}
	if got := veem(Area{}); len(got) != 0 {
		t.Errorf("área vazia: %v", got)
	}
	if _, err := r.Get(ctx, ids["unidade"], &Area{Unidades: []uuid.UUID{un}}); err != nil {
		t.Errorf("detalhe dentro da área: %v", err)
	}
	if _, err := r.Get(ctx, ids["sem-lotacao"], &Area{Unidades: []uuid.UUID{un}}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("detalhe fora da área: %v", err)
	}
	if AreaOf(auth.Identity{Roles: []string{auth.RoleAdmin}}) != nil {
		t.Error("gestão global lê a trilha toda")
	}
	escopo := auth.Identity{Scopes: []auth.Scope{{Permissions: []string{"audit:read"}, UnidadeID: &un, Unidades: []uuid.UUID{un}}}}
	if a := AreaOf(escopo); a == nil || !slices.Equal(a.Unidades, []uuid.UUID{un}) {
		t.Errorf("audit:read com escopo: %+v", a)
	}
}
