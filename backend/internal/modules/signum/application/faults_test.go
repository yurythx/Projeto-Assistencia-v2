package application_test

import (
	"context"
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
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/signum/application"
	"github.com/yurythx/projeto-nexus/internal/modules/signum/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/signum/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/signum/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

var errBoom = errors.New("falha simulada no repositório")

// faultRepo: ver o equivalente no Trâmite (erro na chamada failAt;
// transação envenenada depois da chamada poisonAt).
type faultRepo struct {
	domain.Repository
	calls, failAt, poisonAt int
	trace                   []string
	noTx                    bool
}

func (f *faultRepo) hook(name string) error {
	f.calls++
	f.trace = append(f.trace, name)
	if f.calls == f.failAt {
		return errBoom
	}
	return nil
}

func (f *faultRepo) post(ctx context.Context, db database.DBTX) {
	if f.calls != f.poisonAt {
		return
	}
	if _, inTx := db.(pgx.Tx); !inTx {
		f.noTx = true
		return
	}
	_, _ = db.Exec(ctx, `SELECT 1/0`)
}

func (f *faultRepo) Insert(ctx context.Context, db database.DBTX, e domain.Envelope) error {
	if err := f.hook("Insert"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.Insert(ctx, db, e)
}
func (f *faultRepo) UnavailableSigners(ctx context.Context, db database.DBTX, ids []uuid.UUID) ([]uuid.UUID, error) {
	if err := f.hook("UnavailableSigners"); err != nil {
		return nil, err
	}
	defer f.post(ctx, db)
	return f.Repository.UnavailableSigners(ctx, db, ids)
}
func (f *faultRepo) Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Envelope, error) {
	if err := f.hook("Get"); err != nil {
		return domain.Envelope{}, err
	}
	defer f.post(ctx, db)
	return f.Repository.Get(ctx, db, id, forUpdate)
}
func (f *faultRepo) List(ctx context.Context, db database.DBTX, fl domain.Filter, limit int) ([]domain.Envelope, error) {
	if err := f.hook("List"); err != nil {
		return nil, err
	}
	defer f.post(ctx, db)
	return f.Repository.List(ctx, db, fl, limit)
}
func (f *faultRepo) SetEnvelopeStatus(ctx context.Context, db database.DBTX, id uuid.UUID, status string) error {
	if err := f.hook("SetEnvelopeStatus"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.SetEnvelopeStatus(ctx, db, id, status)
}
func (f *faultRepo) UpdateSigner(ctx context.Context, db database.DBTX, s domain.Signer) error {
	if err := f.hook("UpdateSigner"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.UpdateSigner(ctx, db, s)
}
func (f *faultRepo) InsertChallenge(ctx context.Context, db database.DBTX, c domain.Challenge) error {
	if err := f.hook("InsertChallenge"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.InsertChallenge(ctx, db, c)
}
func (f *faultRepo) ConsumeChallenge(ctx context.Context, db database.DBTX, id, env, user uuid.UUID, nonce string) error {
	if err := f.hook("ConsumeChallenge"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.ConsumeChallenge(ctx, db, id, env, user, nonce)
}

// okReauth aceita qualquer senha exceto "errada"; err força outro erro.
type okReauth struct{ err error }

func (r okReauth) Reauthenticate(_ context.Context, _ uuid.UUID, _ string, _ bool, password string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if password == "errada" {
		return "", domain.ErrReauth
	}
	return "password_reauth:local", nil
}

// fakeThrottle registra chamadas e simula bloqueio/indisponibilidade.
type fakeThrottle struct {
	locked   time.Duration
	lockErr  error
	failures int
	resets   int
}

func (f *fakeThrottle) LockedFor(context.Context, string) (time.Duration, error) {
	return f.locked, f.lockErr
}
func (f *fakeThrottle) RegisterFailure(context.Context, string) (time.Duration, error) {
	f.failures++
	return 0, nil
}
func (f *fakeThrottle) Reset(context.Context, string) error { f.resets++; return nil }

type env struct {
	t             *testing.T
	pool          *pgxpool.Pool
	author, alice auth.Identity
	doc           string
}

func newEnv(t *testing.T) *env {
	pool := dbtest.Pool(t)
	return &env{t: t, pool: pool,
		author: auth.Identity{UserID: dbtest.User(t, pool), Username: "autor"},
		alice:  auth.Identity{UserID: dbtest.User(t, pool), Username: "alice"},
		doc:    strings.Repeat("ab", 32)}
}

func (e *env) svc(repo domain.Repository, reauth domain.Reauthenticator, throttle application.Throttle) *application.Service {
	return application.NewService(e.pool, repo, reauth, throttle, outbox.NewWriter("test"), "segredo", time.Minute,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (e *env) real() *application.Service {
	return e.svc(infrastructure.NewRepository(), okReauth{}, nil)
}

func (e *env) ctx(id auth.Identity) context.Context {
	return auth.WithIdentity(context.Background(), id)
}

func (e *env) envelope(signers ...uuid.UUID) domain.Envelope {
	e.t.Helper()
	env, err := e.real().Open(e.ctx(e.author), application.OpenRequest{Title: "Contrato", DocumentSHA256: e.doc, SignerIDs: signers, SourceModule: "signum"})
	if err != nil {
		e.t.Fatal(err)
	}
	return env
}

func (e *env) challenge(envID uuid.UUID) application.ChallengeResponse {
	e.t.Helper()
	c, err := e.real().Challenge(context.Background(), e.alice, envID)
	if err != nil {
		e.t.Fatal(err)
	}
	return c
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := newEnv(t)
	ctx := e.ctx(e.author)
	type setup func() func(s *application.Service) error
	ops := map[string]setup{
		"Open": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.Open(ctx, application.OpenRequest{Title: "x", DocumentSHA256: e.doc, SignerIDs: []uuid.UUID{e.alice.UserID}})
				return err
			}
		},
		"Get": func() func(*application.Service) error {
			env := e.envelope(e.alice.UserID)
			return func(s *application.Service) error { _, err := s.Get(ctx, e.author, env.ID); return err }
		},
		"List": func() func(*application.Service) error {
			e.envelope(e.alice.UserID)
			return func(s *application.Service) error { _, err := s.List(ctx, e.author, "created", ""); return err }
		},
		"Challenge": func() func(*application.Service) error {
			env := e.envelope(e.alice.UserID)
			return func(s *application.Service) error { _, err := s.Challenge(ctx, e.alice, env.ID); return err }
		},
		"Sign": func() func(*application.Service) error {
			env := e.envelope(e.alice.UserID)
			c := e.challenge(env.ID)
			return func(s *application.Service) error {
				_, err := s.Sign(ctx, e.alice, env.ID, application.SignInput{ChallengeID: c.ChallengeID, Nonce: c.Nonce, Password: "ok", ConfirmSHA256: e.doc})
				return err
			}
		},
		"Refuse": func() func(*application.Service) error {
			env := e.envelope(e.alice.UserID)
			return func(s *application.Service) error { _, err := s.Refuse(ctx, e.alice, env.ID, "motivo"); return err }
		},
		"Cancel": func() func(*application.Service) error {
			env := e.envelope(e.alice.UserID)
			return func(s *application.Service) error { _, err := s.Cancel(ctx, e.author, env.ID); return err }
		},
		"Verify": func() func(*application.Service) error {
			env := e.envelope(e.alice.UserID)
			return func(s *application.Service) error { _, err := s.Verify(ctx, env.ID, e.doc); return err }
		},
	}
	for name, o := range ops {
		probe := &faultRepo{Repository: infrastructure.NewRepository()}
		if err := o()(e.svc(probe, okReauth{}, nil)); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			for _, poison := range []bool{false, true} {
				r := &faultRepo{Repository: infrastructure.NewRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := o()(e.svc(r, okReauth{}, nil)); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
}

func TestOpenTxGuards(t *testing.T) {
	e := newEnv(t)
	s := e.real()
	req := application.OpenRequest{Title: "x", DocumentSHA256: e.doc, SignerIDs: []uuid.UUID{e.alice.UserID}}
	if _, err := s.Open(context.Background(), req); status(err) != http.StatusUnauthorized {
		t.Errorf("abrir envelope exige identidade: %v", err)
	}
	for _, n := range []int{0, 51} {
		ids := make([]uuid.UUID, n)
		for i := range ids {
			ids[i] = uuid.New()
		}
		r := req
		r.SignerIDs = ids
		if _, err := s.Open(e.ctx(e.author), r); status(err) != http.StatusUnprocessableEntity {
			t.Errorf("%d signatários: %v", n, err)
		}
	}
}

// A cerimônia com o throttle: bloqueio, indisponibilidade do Redis
// (fail-open), contagem de falhas e reset após sucesso; user agent longo.
func TestSignThrottleAndReauthErrors(t *testing.T) {
	e := newEnv(t)
	ctx := e.ctx(e.alice)
	repo := infrastructure.NewRepository()
	sign := func(s *application.Service, env domain.Envelope, password string) (domain.Envelope, error) {
		c := e.challenge(env.ID)
		return s.Sign(ctx, e.alice, env.ID, application.SignInput{ChallengeID: c.ChallengeID, Nonce: c.Nonce, Password: password,
			ConfirmSHA256: e.doc, IP: "203.0.113.7", UserAgent: strings.Repeat("u", 500)})
	}

	th := &fakeThrottle{locked: time.Minute}
	if _, err := sign(e.svc(repo, okReauth{}, th), e.envelope(e.alice.UserID), "ok"); status(err) != http.StatusTooManyRequests {
		t.Errorf("usuário bloqueado: esperado 429, veio %v", err)
	}
	th = &fakeThrottle{}
	if _, err := sign(e.svc(repo, okReauth{}, th), e.envelope(e.alice.UserID), "errada"); status(err) != http.StatusUnauthorized || th.failures != 1 {
		t.Errorf("senha errada conta uma falha: %v (%d)", err, th.failures)
	}
	unavail := apperrors.DependencyUnavailable("Keycloak fora")
	if _, err := sign(e.svc(repo, okReauth{err: unavail}, th), e.envelope(e.alice.UserID), "ok"); !errors.Is(err, unavail) || th.failures != 1 {
		t.Errorf("Keycloak fora não conta como senha errada: %v (%d)", err, th.failures)
	}
	th = &fakeThrottle{lockErr: errors.New("redis fora")}
	signed, err := sign(e.svc(repo, okReauth{}, th), e.envelope(e.alice.UserID), "ok")
	if err != nil || signed.Status != domain.StatusCompleted || th.resets != 1 {
		t.Fatalf("Redis fora não impede a assinatura (fail-open) e o sucesso zera o contador: %v %+v", err, th)
	}
	var ua string
	if err := e.pool.QueryRow(context.Background(), `SELECT user_agent FROM signum_signers WHERE envelope_id = $1`, signed.ID).Scan(&ua); err != nil || len(ua) != 400 {
		t.Errorf("user agent limitado a 400 caracteres: %d %v", len(ua), err)
	}
	// Sem throttle configurado, a falha de senha ainda é auditada.
	env := e.envelope(e.alice.UserID)
	if _, err := sign(e.real(), env, "errada"); status(err) != http.StatusUnauthorized {
		t.Fatalf("senha errada sem throttle: %v", err)
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = 'signum.reauth.failed' AND resource_id = $1`,
		env.ID.String()).Scan(&n); err != nil || n != 1 {
		t.Errorf("falha de reautenticação auditada mesmo sem throttle: %d %v", n, err)
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Errorf("erro desconhecido passa adiante: %v", err)
	}
}

func TestHandlersReportServiceFailures(t *testing.T) {
	e := newEnv(t)
	h := transport.NewHandlers(e.svc(&faultRepo{Repository: infrastructure.NewRepository(), failAt: 1}, okReauth{}, nil),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), e.author)))
		})
	})
	h.RegisterAuthedRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/signum/envelopes", nil))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
		t.Errorf("listagem com o banco fora: %d %s", rec.Code, rec.Body.String())
	}
}

func status(err error) int {
	if appErr, ok := apperrors.As(err); ok {
		return appErr.Status
	}
	return 0
}
