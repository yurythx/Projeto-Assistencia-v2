package lgpd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakePD é um plugin com dados pessoais.
type fakePD struct {
	data                any
	exportErr, eraseErr error
	poison              bool // envenena a transação e não reporta (simula bug do plugin)
	erased              []uuid.UUID
}

func (f *fakePD) ExportPersonalData(context.Context, database.DBTX, uuid.UUID) (any, error) {
	return f.data, f.exportErr
}

func (f *fakePD) ErasePersonalData(ctx context.Context, tx database.DBTX, id uuid.UUID) error {
	if f.poison {
		_, _ = tx.Exec(ctx, `SELECT 1/0`)
		return nil
	}
	f.erased = append(f.erased, id)
	return f.eraseErr
}

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	s    *Service
	pds  map[string]PersonalData
}

func newEnv(t *testing.T) *env {
	pool := dbtest.Pool(t)
	e := &env{t: t, pool: pool, s: NewService(pool, quiet(), nil), pds: map[string]PersonalData{}}
	e.s.SetPersonalDataSource(func() map[string]PersonalData { return e.pds })
	// O processor varre TODAS as solicitações pendentes: as deixadas por
	// outros testes no banco compartilhado sairiam do lote de 20.
	if _, err := pool.Exec(context.Background(), `UPDATE data_subject_requests SET status = 'rejected'
		WHERE kind = 'erasure' AND status = 'pending'`); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) router(identity *auth.Identity) http.Handler {
	r := chi.NewRouter()
	if identity != nil {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), *identity)))
			})
		})
	}
	e.s.RegisterRoutes(r)
	e.s.RegisterDSRRoutes(r)
	return r
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func (e *env) requestErasure(uid uuid.UUID) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	if err := e.pool.QueryRow(context.Background(),
		`INSERT INTO data_subject_requests (user_id, kind, status) VALUES ($1, 'erasure', 'pending') RETURNING id`, uid).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) status(req uuid.UUID) (string, int, string) {
	var st, detail string
	var attempts int
	_ = e.pool.QueryRow(context.Background(), `SELECT status, attempts, detail FROM data_subject_requests WHERE id = $1`, req).Scan(&st, &attempts, &detail)
	return st, attempts, detail
}

func TestExportIncludesEveryModuleOrFails(t *testing.T) {
	e := newEnv(t)
	uid := dbtest.User(t, e.pool)
	id := auth.Identity{UserID: uid, Source: auth.SourceLocal}
	_, _ = e.pool.Exec(context.Background(), `UPDATE users SET last_seen_at = now() WHERE id = $1`, uid)
	e.pds["directory"] = &fakePD{data: map[string]string{"phone": "61 3333"}}
	e.pds["vazio"] = &fakePD{} // nada guardado: fica fora do pacote
	rec := do(e.router(&id), http.MethodGet, "/lgpd/meus-dados", "")
	var env struct {
		Data myDataPackage `json:"data"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &env) != nil {
		t.Fatalf("exportação: %d %s", rec.Code, rec.Body.String())
	}
	pkg := env.Data
	if pkg.Account["last_seen_at"] == nil {
		t.Fatal("último acesso entra no pacote")
	}
	if _, ok := pkg.Modules["vazio"]; ok || pkg.Modules["directory"] == nil || pkg.Account["id"] != uid.String() {
		t.Fatalf("pacote deveria trazer só os módulos com dados: %+v", pkg.Modules)
	}

	// Um módulo que falha derruba a exportação: nada de pacote incompleto.
	e.pds["quebrado"] = &fakePD{exportErr: errors.New("fora")}
	if rec := do(e.router(&id), http.MethodGet, "/lgpd/meus-dados", ""); rec.Code != http.StatusInternalServerError {
		t.Fatalf("módulo com falha: %d", rec.Code)
	}
	delete(e.pds, "quebrado")

	// Cada seção do núcleo também: conta, consentimentos, trilha, solicitações.
	for n := 1; n <= 3; n++ {
		e.s.db = &hookDB{DBTX: e.pool, failQueryAt: n}
		if rec := do(e.router(&id), http.MethodGet, "/lgpd/meus-dados", ""); rec.Code != http.StatusInternalServerError {
			t.Errorf("seção %d com falha: %d", n, rec.Code)
		}
	}
	e.s.db = &hookDB{DBTX: e.pool, badRowsAt: 1}
	if rec := do(e.router(&id), http.MethodGet, "/lgpd/meus-dados", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("linha ilegível: %d", rec.Code)
	}
	e.s.db = &hookDB{DBTX: e.pool, failRowAt: 1} // conta
	if rec := do(e.router(&id), http.MethodGet, "/lgpd/meus-dados", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("conta ilegível: %d", rec.Code)
	}
	e.s.db = e.pool
	// Trilha do próprio ato falhando não muda a resposta.
	e.s.db = &hookDB{DBTX: e.pool, failExecAt: 1}
	if rec := do(e.router(&id), http.MethodGet, "/lgpd/meus-dados", ""); rec.Code != http.StatusOK {
		t.Errorf("exportação com a auditoria fora: %d", rec.Code)
	}
}

func TestErasureAnonymizesAccountAndModulesAtomically(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	uid := dbtest.User(t, e.pool)
	dir := &fakePD{}
	e.pds["directory"] = dir
	id := auth.Identity{UserID: uid, Source: auth.SourceLocal}
	h := e.router(&id)

	rec := do(h, http.MethodPost, "/lgpd/solicitar-exclusao", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("solicitação: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPost, "/lgpd/solicitar-exclusao", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "em andamento") {
		t.Fatalf("segunda solicitação devolve a aberta: %d", rec.Code)
	}
	e.s.processErasureBatch(ctx)
	var username, email string
	var active bool
	_ = e.pool.QueryRow(ctx, `SELECT username, email, active FROM users WHERE id = $1`, uid).Scan(&username, &email, &active)
	want := "anon_" + strings.ReplaceAll(uid.String(), "-", "")
	if username != want || active || !strings.HasSuffix(email, "@anonimizado.invalid") || len(dir.erased) != 1 {
		t.Fatalf("conta anonimizada com o id inteiro e módulo apagado: %s %v erased=%v", username, active, dir.erased)
	}
	if list := do(h, http.MethodGet, "/lgpd/minhas-solicitacoes", ""); !strings.Contains(list.Body.String(), "completed") {
		t.Fatalf("acompanhamento: %s", list.Body.String())
	}
	var audited int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'lgpd.erasure.completed' AND resource_id = $1
		AND metadata->'modules' ? 'directory'`, uid.String()).Scan(&audited)
	if audited != 1 {
		t.Fatal("conclusão auditada com os módulos eliminados")
	}
	// Já concluída: outra réplica não reprocessa.
	if err := e.s.processOneErasure(ctx, e.requestErasureDone(uid), uid); err != nil {
		t.Fatal(err)
	}
}

func (e *env) requestErasureDone(uid uuid.UUID) uuid.UUID {
	id := e.requestErasure(uid)
	_, _ = e.pool.Exec(context.Background(), `UPDATE data_subject_requests SET status = 'completed' WHERE id = $1`, id)
	return id
}

func TestErasureRetriesThenFails(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.s.maxErasureAttempts = 2
	for name, pd := range map[string]*fakePD{
		"módulo recusa":        {eraseErr: errors.New("fora")},
		"transação envenenada": {poison: true}, // "marcar concluída" falha
	} {
		uid := dbtest.User(t, e.pool)
		e.pds = map[string]PersonalData{"m": pd}
		req := e.requestErasure(uid)
		e.s.processErasureBatch(ctx)
		if st, n, _ := e.status(req); st != "pending" || n != 1 {
			t.Fatalf("%s: falha transitória fica pendente (%s, %d)", name, st, n)
		}
		e.s.processErasureBatch(ctx)
		if st, n, detail := e.status(req); st != "failed" || n != 2 || detail != erasureFailedDetail {
			t.Fatalf("%s: no limite vira failed (%s, %d, %q)", name, st, n, detail)
		}
		var active bool
		_ = e.pool.QueryRow(ctx, `SELECT active FROM users WHERE id = $1`, uid).Scan(&active)
		if !active {
			t.Fatalf("%s: nada é anonimizado pela metade", name)
		}
	}
	e.pds = map[string]PersonalData{}

	// Colisão de username na anonimização também é falha (e é retentada).
	uid := dbtest.User(t, e.pool)
	if _, err := e.pool.Exec(ctx, `INSERT INTO users (username, email, password_hash) VALUES ($1, $1 || '@x', 'x')`,
		"anon_"+strings.ReplaceAll(uid.String(), "-", "")); err != nil {
		t.Fatal(err)
	}
	req := e.requestErasure(uid)
	e.s.processErasureBatch(ctx)
	if st, n, _ := e.status(req); st != "pending" || n != 1 {
		t.Fatalf("colisão: %s %d", st, n)
	}
	if err := e.s.processOneErasure(ctx, uuid.New(), uid); err == nil {
		t.Fatal("solicitação inexistente")
	}
}

func TestErasureBatchSurvivesDatabaseFailures(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.s.db = dbtest.Fail{}
	e.s.processErasureBatch(ctx) // varredura falha: só registra
	e.s.db = &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.BadRows()}}}
	e.s.processErasureBatch(ctx)

	// Falha ao registrar a falha: só registra também (a solicitação segue pendente).
	uid := dbtest.User(t, e.pool)
	req := e.requestErasure(uid)
	e.pds["m"] = &fakePD{eraseErr: errors.New("fora")}
	e.s.db = &hookDB{DBTX: e.pool, failExecAt: 1}
	e.s.processErasureBatch(ctx)
	if st, n, _ := e.status(req); st != "pending" || n != 0 {
		t.Fatalf("sem registro da tentativa: %s %d", st, n)
	}
	e.s.db = e.pool
	e.pds = map[string]PersonalData{}

	// O processor roda uma vez no boot e depois a cada intervalo.
	e.s.erasureInterval = 5 * time.Millisecond
	c, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { _ = e.s.ErasureProcessor()(c); close(done) }()
	waitDone(t, func() bool { st, _, _ := e.status(req); return st == "completed" })
	later := e.requestErasure(dbtest.User(t, e.pool)) // só o tick seguinte pega
	waitDone(t, func() bool { st, _, _ := e.status(later); return st == "completed" })
	cancel()
	<-done
}

func TestDSRHandlersEdgeCases(t *testing.T) {
	e := newEnv(t)
	uid := dbtest.User(t, e.pool)
	id := auth.Identity{UserID: uid, Source: auth.SourceLocal}
	anon := e.router(nil)
	for _, p := range []string{"GET /lgpd/meus-dados", "POST /lgpd/solicitar-exclusao", "GET /lgpd/minhas-solicitacoes", "GET /lgpd/status", "POST /lgpd/accept"} {
		parts := strings.SplitN(p, " ", 2)
		if rec := do(anon, parts[0], parts[1], ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s sem identidade: %d", p, rec.Code)
		}
	}
	ghost := auth.Identity{Subject: "kc-" + uuid.NewString(), Source: auth.SourceKeycloak}
	for _, p := range []string{"GET /lgpd/meus-dados", "POST /lgpd/solicitar-exclusao", "GET /lgpd/minhas-solicitacoes", "GET /lgpd/status", "POST /lgpd/accept"} {
		parts := strings.SplitN(p, " ", 2)
		if rec := do(e.router(&ghost), parts[0], parts[1], ""); rec.Code != http.StatusNotFound {
			t.Errorf("%s sem conta local: %d", p, rec.Code)
		}
	}
	// Federado sem o id resolvido pelo IAM: busca por keycloak_subject.
	sub := "kc-" + uuid.NewString()
	if _, err := e.pool.Exec(context.Background(), `INSERT INTO users (keycloak_subject, username, email) VALUES ($1, $1, $1 || '@ad.test')`, sub); err != nil {
		t.Fatal(err)
	}
	fed := auth.Identity{Subject: sub, Source: auth.SourceKeycloak}
	if rec := do(e.router(&fed), http.MethodGet, "/lgpd/status", ""); rec.Code != http.StatusOK {
		t.Errorf("federado resolvido pelo subject: %d", rec.Code)
	}
	if keys, m := NewService(e.pool, quiet(), nil).providers(); keys != nil || m != nil {
		t.Error("sem fonte ligada: nenhum provedor")
	}
	// Token local: o subject é o id interno.
	local := auth.Identity{Subject: uid.String(), Source: auth.SourceLocal}
	if rec := do(e.router(&local), http.MethodGet, "/lgpd/status", ""); rec.Code != http.StatusOK {
		t.Errorf("subject local: %d", rec.Code)
	}
	// Banco fora em cada etapa.
	e.s.db = dbtest.Fail{}
	h := e.router(&id)
	for _, p := range []string{"POST /lgpd/solicitar-exclusao", "GET /lgpd/minhas-solicitacoes", "GET /lgpd/status", "POST /lgpd/accept"} {
		parts := strings.SplitN(p, " ", 2)
		if rec := do(h, parts[0], parts[1], ""); rec.Code != http.StatusInternalServerError {
			t.Errorf("%s com o banco fora: %d", p, rec.Code)
		}
	}
	if rec := do(e.router(&ghost), http.MethodGet, "/lgpd/status", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("resolver federado com o banco fora: %d", rec.Code)
	}
	// Checagem ok, criação falha.
	e.s.db = &hookDB{DBTX: e.pool, failRowAt: 2}
	if rec := do(h, http.MethodPost, "/lgpd/solicitar-exclusao", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("criação falha: %d", rec.Code)
	}
	e.s.db = e.pool
	if rec := do(h, http.MethodPost, "/lgpd/accept", "{"); rec.Code != http.StatusBadRequest {
		t.Errorf("JSON malformado: %d", rec.Code)
	}
}

func TestAnonymousConsent(t *testing.T) {
	e := newEnv(t)
	r := chi.NewRouter()
	RegisterPublicRoutes(r, e.s, allowAll{})
	if rec := do(r, http.MethodPost, "/lgpd/accept-anon", `{"device_hash":"`+uuid.NewString()+`"}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), CurrentTermVersion) {
		t.Fatalf("aceite anônimo: %d %s", rec.Code, rec.Body.String())
	}
	for body, want := range map[string]int{`{`: http.StatusBadRequest, `{"device_hash":"curto"}`: http.StatusUnprocessableEntity} {
		if rec := do(r, http.MethodPost, "/lgpd/accept-anon", body); rec.Code != want {
			t.Errorf("%s: %d", body, rec.Code)
		}
	}
	e.s.db = dbtest.Fail{}
	if rec := do(r, http.MethodPost, "/lgpd/accept-anon", `{"device_hash":"`+uuid.NewString()+`"}`); rec.Code != http.StatusInternalServerError {
		t.Errorf("banco fora: %d", rec.Code)
	}
}

type allowAll struct{}

func (allowAll) Allow(context.Context, string) (bool, error) { return true, nil }

// hookDB envolve o banco real falhando a n-ésima chamada de cada tipo.
type hookDB struct {
	database.DBTX
	failQueryAt, badRowsAt, failRowAt, failExecAt int
	nq, nr, ne                                    int
}

func (h *hookDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	h.nq++
	if h.nq == h.failQueryAt {
		return nil, dbtest.ErrInjected
	}
	if h.nq == h.badRowsAt {
		return dbtest.BadRows(), nil
	}
	return h.DBTX.Query(ctx, sql, args...)
}

func (h *hookDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	h.nr++
	if h.nr == h.failRowAt {
		return dbtest.Fail{}.QueryRow(ctx, sql)
	}
	return h.DBTX.QueryRow(ctx, sql, args...)
}

func (h *hookDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	h.ne++
	if h.ne == h.failExecAt {
		return pgconn.CommandTag{}, dbtest.ErrInjected
	}
	return h.DBTX.Exec(ctx, sql, args...)
}

func waitDone(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condição não atingida a tempo")
}
