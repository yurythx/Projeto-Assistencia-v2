package localauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

var errDown = errors.New("fora do ar")

// storeFaults faz operações do fakeStore falharem.
type storeFaults struct {
	*fakeStore
	get, register, reset, touch, rehash bool
}

func (s *storeFaults) GetByUsername(ctx context.Context, u string) (*Account, error) {
	if s.get {
		return nil, errDown
	}
	return s.fakeStore.GetByUsername(ctx, u)
}
func (s *storeFaults) RegisterFailedAttempt(ctx context.Context, id uuid.UUID) error {
	if s.register {
		return errDown
	}
	return s.fakeStore.RegisterFailedAttempt(ctx, id)
}
func (s *storeFaults) ResetFailedAttempts(ctx context.Context, id uuid.UUID) error {
	if s.reset {
		return errDown
	}
	return s.fakeStore.ResetFailedAttempts(ctx, id)
}
func (s *storeFaults) TouchLastSeen(ctx context.Context, id uuid.UUID) error {
	if s.touch {
		return errDown
	}
	return s.fakeStore.TouchLastSeen(ctx, id)
}
func (s *storeFaults) UpdatePasswordHash(ctx context.Context, id uuid.UUID, h string) error {
	if s.rehash {
		return errDown
	}
	return s.fakeStore.UpdatePasswordHash(ctx, id, h)
}

// fakeThrottle simula o lockout distribuído.
type fakeThrottle struct {
	locked   time.Duration
	lockErr  error
	applied  time.Duration
	resets   int
	failures int
}

func (f *fakeThrottle) LockedFor(context.Context, string) (time.Duration, error) {
	return f.locked, f.lockErr
}
func (f *fakeThrottle) RegisterFailure(context.Context, string) (time.Duration, error) {
	f.failures++
	return f.applied, nil
}
func (f *fakeThrottle) Reset(context.Context, string) error { f.resets++; return nil }

type fakePenalizer struct{ penalized, forgiven int }

func (f *fakePenalizer) Penalize(context.Context, string) error { f.penalized++; return nil }
func (f *fakePenalizer) Forgive(context.Context, string) error  { f.forgiven++; return nil }

type failingIssuer struct{}

func (failingIssuer) IssueToken(auth.LocalAccount) (string, time.Time, error) {
	return "", time.Time{}, errDown
}

func account(t *testing.T) *Account {
	return &Account{ID: uuid.New(), Username: "ana", PasswordHash: mustHash(t, "Senha-Forte-123!"), Active: true}
}

func TestLoginWithThrottleAndPenalizer(t *testing.T) {
	th := &fakeThrottle{locked: 30 * time.Second}
	pen := &fakePenalizer{}
	h := NewHandlers(newFakeStore(account(t)), testSigner(t), nil, th, pen, testLogger())
	rec := doLogin(h, "ana", "Senha-Forte-123!")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "31" {
		t.Fatalf("bloqueado pelo lockout distribuído: %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	// Lockout indisponível: segue só com o bloqueio da conta.
	th.locked, th.lockErr = 0, errDown
	if rec := doLogin(h, "ana", "Senha-Forte-123!"); rec.Code != http.StatusOK || th.resets != 1 || pen.forgiven != 1 {
		t.Fatalf("login com o lockout fora do ar: %d resets=%d forgiven=%d", rec.Code, th.resets, pen.forgiven)
	}
	// Falha: registra no lockout (IP e usuário) e penaliza o IP.
	th.lockErr, th.applied = nil, time.Minute
	if rec := doLogin(h, "ana", "errada"); rec.Code != http.StatusUnauthorized || th.failures != 2 || pen.penalized != 1 {
		t.Fatalf("falha registrada: %d failures=%d penalized=%d", rec.Code, th.failures, pen.penalized)
	}
}

func TestLoginStoreAndSignerFailures(t *testing.T) {
	a := account(t)
	// Banco fora ao buscar a conta: 500 (não "credencial inválida").
	h := NewHandlers(&storeFaults{fakeStore: newFakeStore(a), get: true}, testSigner(t), nil, nil, nil, testLogger())
	if rec := doLogin(h, "ana", "x"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("banco fora: %d", rec.Code)
	}
	// Falhas secundárias (registrar tentativa, zerar, last_seen, rehash) só avisam.
	legacy := account(t) // bcrypt: precisa de rehash
	s := &storeFaults{fakeStore: newFakeStore(legacy), register: true, reset: true, touch: true, rehash: true}
	h = NewHandlers(s, testSigner(t), nil, nil, nil, testLogger())
	if rec := doLogin(h, "ana", "errada"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("senha errada com o registro fora: %d", rec.Code)
	}
	if rec := doLogin(h, "ana", "Senha-Forte-123!"); rec.Code != http.StatusOK {
		t.Fatalf("login com falhas secundárias: %d", rec.Code)
	}
	// Emissão do token falhando: 500.
	h = NewHandlers(newFakeStore(account(t)), nil, nil, nil, nil, testLogger())
	h.signer = failingIssuer{}
	if rec := doLogin(h, "ana", "Senha-Forte-123!"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("token não emitido: %d", rec.Code)
	}
	if rec := doLogin(NewHandlers(newFakeStore(a), testSigner(t), nil, nil, nil, testLogger()), "ana", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("senha vazia: %d", rec.Code)
	}
}

func TestPostgresStoreLockoutAndFailures(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	s := NewPostgresStore(pool)
	uid := dbtest.User(t, pool)
	var username string
	_ = pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, uid).Scan(&username)
	if _, err := pool.Exec(ctx, `UPDATE users SET password_hash = 'x' WHERE id = $1`, uid); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxFailedAttempts; i++ {
		if err := s.RegisterFailedAttempt(ctx, uid); err != nil {
			t.Fatal(err)
		}
	}
	a, err := s.GetByUsername(ctx, username)
	if err != nil || a.FailedLoginAttempts != maxFailedAttempts || !a.Locked() {
		t.Fatalf("bloqueio no limite: %+v %v", a, err)
	}
	if left := time.Until(*a.LockedUntil); left < 50*time.Second || left > 70*time.Second {
		t.Fatalf("primeiro bloqueio de ~1 min: %v", left)
	}
	for i := 0; i < 30; i++ { // o bloqueio cresce, mas nunca passa do teto
		_ = s.RegisterFailedAttempt(ctx, uid)
	}
	a, _ = s.GetByUsername(ctx, username)
	if left := time.Until(*a.LockedUntil); left > maxLockout+time.Minute {
		t.Fatalf("teto do bloqueio: %v", left)
	}
	if err := s.ResetFailedAttempts(ctx, uid); err != nil {
		t.Fatal(err)
	}
	if err := s.TouchLastSeen(ctx, uid); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePasswordHash(ctx, uid, "novo"); err != nil {
		t.Fatal(err)
	}
	a, _ = s.GetByUsername(ctx, username)
	if a.Locked() || a.FailedLoginAttempts != 0 || a.PasswordHash != "novo" {
		t.Fatalf("zerado e senha trocada: %+v", a)
	}
	if _, err := s.GetByUsername(ctx, "ninguem-"+uuid.NewString()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("inexistente: %v", err)
	}

	down := &PostgresStore{db: dbtest.Fail{}}
	if _, err := down.GetByUsername(ctx, "x"); err == nil || errors.Is(err, pgx.ErrNoRows) {
		t.Error("get com o banco fora")
	}
	for name, err := range map[string]error{
		"touch":    down.TouchLastSeen(ctx, uid),
		"register": down.RegisterFailedAttempt(ctx, uid),
		"reset":    down.ResetFailedAttempts(ctx, uid),
		"rehash":   down.UpdatePasswordHash(ctx, uid, "h"),
	} {
		if err == nil || !strings.Contains(err.Error(), "localauth") {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
}

type allow struct{}

func (allow) Allow(context.Context, string) (bool, error) { return true, nil }

func TestLoginAuditAndRoutes(t *testing.T) {
	pool := dbtest.Pool(t)
	a := account(t)
	a.Username = "audit-" + uuid.NewString()[:8]
	h := NewHandlers(newFakeStore(a), testSigner(t), audit.NewWriter(pool), nil, nil, testLogger())
	r := chi.NewRouter()
	RegisterRoutes(r, h, testLogger(), allow{})
	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post(`{`); code != http.StatusBadRequest {
		t.Fatalf("JSON malformado: %d", code)
	}
	if code := post(`{"username":"` + a.Username + `","password":"errada"}`); code != http.StatusUnauthorized {
		t.Fatalf("falha: %d", code)
	}
	if code := post(`{"username":"` + a.Username + `","password":"Senha-Forte-123!"}`); code != http.StatusOK {
		t.Fatalf("sucesso: %d", code)
	}
	var ok, failed int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE action = 'login' AND resource_id = $2),
		count(*) FILTER (WHERE action = $1 AND resource_id = $3) FROM audit_logs`, ActionLoginFailed, a.ID.String(), a.Username).Scan(&ok, &failed)
	if ok != 1 || failed != 1 {
		t.Fatalf("login e falha auditados: %d %d", ok, failed)
	}
	RegisterAuthedRoutes(chi.NewRouter(), h)
}
