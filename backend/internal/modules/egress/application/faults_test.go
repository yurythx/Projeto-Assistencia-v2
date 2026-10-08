package application_test

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/egress"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/application"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/netguard"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
)

// fakeDeliverer responde com o status configurado por destino.
type fakeDeliverer struct {
	mu     sync.Mutex
	status map[uuid.UUID]int
	err    map[uuid.UUID]error
	calls  []uuid.UUID
}

func (f *fakeDeliverer) Policy() netguard.Policy {
	return netguard.Policy{AllowPrivate: true, AllowHTTP: true}
}
func (f *fakeDeliverer) Deliver(_ context.Context, t domain.Target, _ domain.Delivery) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, t.ID)
	if err := f.err[t.ID]; err != nil {
		return f.status[t.ID], err
	}
	if s, ok := f.status[t.ID]; ok {
		return s, nil
	}
	return http.StatusOK, nil
}

type env struct {
	t      *testing.T
	pool   *pgxpool.Pool
	cipher *secretcrypto.Cipher
	dlv    *fakeDeliverer
}

func newEnv(t *testing.T) *env {
	pool := dbtest.Pool(t)
	cipher, err := secretcrypto.NewFromBase64Key(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	// Isola das entregas deixadas por outros testes no mesmo banco.
	if _, err := pool.Exec(context.Background(), `UPDATE egress_targets SET active = false`); err != nil {
		t.Fatal(err)
	}
	return &env{t: t, pool: pool, cipher: cipher, dlv: &fakeDeliverer{status: map[uuid.UUID]int{}, err: map[uuid.UUID]error{}}}
}

func (e *env) svc(repo domain.Repository, maxAttempts int) *application.Service {
	return application.NewService(e.pool, repo, e.dlv, e.cipher, application.Config{MaxAttempts: maxAttempts, BatchSize: 50, PollInterval: 10 * time.Millisecond},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (e *env) real() *application.Service { return e.svc(infrastructure.NewRepository(), 3) }

func (e *env) target(patterns ...string) domain.Target {
	e.t.Helper()
	secret := "s3cr3t"
	tg, err := e.real().SaveTarget(context.Background(), uuid.Nil, application.TargetInput{Name: "Destino", Kind: "webhook",
		URL: "http://127.0.0.1:9/hook?token=abc", Secret: &secret, EventPatterns: patterns, Active: true, Actor: "teste"})
	if err != nil {
		e.t.Fatal(err)
	}
	return tg
}

func (e *env) event(typ string) events.Event {
	ev, err := events.New(typ, "teste", uuid.Nil, map[string]string{"k": "v"})
	if err != nil {
		e.t.Fatal(err)
	}
	return ev
}

func (e *env) status(targetID uuid.UUID) (string, int) {
	e.t.Helper()
	var st string
	var attempts int
	if err := e.pool.QueryRow(context.Background(), `SELECT status, attempts FROM egress_deliveries WHERE target_id = $1`, targetID).Scan(&st, &attempts); err != nil {
		e.t.Fatal(err)
	}
	return st, attempts
}

// runOnce roda o worker de entrega por uma rodada.
// runOnce roda uma rodada completa do worker (o laço do RunDeliveries em si
// é coberto por TestWorkerStopsMidBatch).
func (e *env) runOnce(s *application.Service) {
	s.DeliverBatch(context.Background())
}

// Ciclo de vida de uma entrega: dispatcher por padrão de evento, entrega,
// retentativa com backoff, "dead" no limite ou por bloqueio anti-SSRF,
// reprocessamento manual e destino desativado em espera.
func TestDeliveryLifecycle(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.real()
	ok := e.target("blog.#")
	flaky := e.target("blog.post.*")
	outro := e.target("files.#")

	ev := e.event("blog.post.published")
	if err := s.Dispatch(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if err := s.Dispatch(ctx, ev); err != nil { // reentrega do barramento: idempotente
		t.Fatal(err)
	}
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM egress_deliveries WHERE event_id = $1`, ev.ID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("uma entrega por destino compatível, sem duplicar: %d %v", n, err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM egress_deliveries WHERE target_id = $1`, outro.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("destino de outro padrão não recebe: %d", n)
	}

	e.dlv.status[flaky.ID] = http.StatusBadGateway
	e.dlv.err[flaky.ID] = errors.New("destino respondeu HTTP 502")
	e.runOnce(s)
	if st, _ := e.status(ok.ID); st != "delivered" {
		t.Fatalf("entrega feita: %s", st)
	}
	if st, att := e.status(flaky.ID); st != "failed" || att != 1 {
		t.Fatalf("falha agenda retentativa: %s %d", st, att)
	}
	// Vencida de novo até o limite de tentativas: vira "dead".
	for i := 0; i < 3; i++ {
		if _, err := e.pool.Exec(ctx, `UPDATE egress_deliveries SET next_attempt_at = now() WHERE target_id = $1`, flaky.ID); err != nil {
			t.Fatal(err)
		}
		e.runOnce(s)
	}
	if st, att := e.status(flaky.ID); st != "dead" || att != 3 {
		t.Fatalf("no limite de tentativas a entrega morre (DLQ lógica): %s %d", st, att)
	}
	// Reprocessamento manual volta para a fila e entrega.
	var delID uuid.UUID
	if err := e.pool.QueryRow(ctx, `SELECT id FROM egress_deliveries WHERE target_id = $1`, flaky.ID).Scan(&delID); err != nil {
		t.Fatal(err)
	}
	delete(e.dlv.err, flaky.ID)
	delete(e.dlv.status, flaky.ID)
	if err := s.Redeliver(ctx, delID); err != nil {
		t.Fatal(err)
	}
	if err := s.Redeliver(ctx, delID); err == nil {
		t.Fatal("só entrega falha ou morta é reprocessada")
	}
	e.runOnce(s)
	if st, _ := e.status(flaky.ID); st != "delivered" {
		t.Fatalf("reprocessada e entregue: %s", st)
	}

	// Bloqueio anti-SSRF na entrega: morre na hora (não adianta insistir).
	blocked := e.target("files.#")
	e.dlv.err[blocked.ID] = netguard.ErrBlockedDestination
	if err := s.Dispatch(ctx, e.event("files.object.uploaded")); err != nil {
		t.Fatal(err)
	}
	e.runOnce(s)
	if st, att := e.status(blocked.ID); st != "dead" || att != 1 {
		t.Fatalf("destino bloqueado morre na primeira tentativa: %s %d", st, att)
	}

	// Destino desativado: a entrega espera (sem tentar), e sai ao reativar.
	pausado := e.target("wiki.#")
	if err := s.Dispatch(ctx, e.event("wiki.page.updated")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveTarget(ctx, pausado.ID, application.TargetInput{Name: "Destino", Kind: "webhook", URL: "http://127.0.0.1:9/hook",
		EventPatterns: []string{"wiki.#"}, Active: false}); err != nil {
		t.Fatal(err)
	}
	e.dlv.calls = nil
	e.runOnce(s)
	if st, att := e.status(pausado.ID); st != "pending" || att != 0 || len(e.dlv.calls) != 0 {
		t.Fatalf("destino desativado não recebe nem gasta tentativa: %s %d %v", st, att, e.dlv.calls)
	}
	if _, err := s.SaveTarget(ctx, pausado.ID, application.TargetInput{Name: "Destino", Kind: "webhook", URL: "http://127.0.0.1:9/hook",
		EventPatterns: []string{"wiki.#"}, Active: true}); err != nil {
		t.Fatal(err)
	}
	e.runOnce(s)
	if st, _ := e.status(pausado.ID); st != "delivered" {
		t.Fatalf("reativado, entrega: %s", st)
	}

	// Segredo que não decifra (chave trocada): tentativa falha registrada.
	quebrado := e.target("tramite.#")
	if _, err := e.pool.Exec(ctx, `UPDATE egress_targets SET secret_encrypted = 'lixo' WHERE id = $1`, quebrado.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Dispatch(ctx, e.event("tramite.processo.aberto")); err != nil {
		t.Fatal(err)
	}
	e.runOnce(s)
	if st, att := e.status(quebrado.ID); st != "failed" || att != 1 {
		t.Fatalf("segredo indecifrável aparece como falha: %s %d", st, att)
	}
	if _, err := s.Test(ctx, quebrado.ID); err == nil {
		t.Fatal("teste com segredo indecifrável falha")
	}
}

// Cadastro: anti-SSRF, padrões de evento, segredo cifrado e auditoria sem
// o token da URL; teste de conectividade.
func TestTargetsManagement(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.real()
	strict := application.NewService(e.pool, infrastructure.NewRepository(), infrastructure.NewDeliverer(time.Second, netguard.Policy{}),
		e.cipher, application.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for name, url := range map[string]string{"loopback": "https://127.0.0.1/x", "http sem TLS": "http://hooks.example.org/x", "rede privada": "https://10.0.0.5/x",
		"host que não resolve": "https://nao-existe.invalid/x"} {
		if _, err := strict.SaveTarget(ctx, uuid.Nil, application.TargetInput{Name: "x", Kind: "webhook", URL: url}); err == nil {
			t.Errorf("%s deveria ser recusado", name)
		}
	}
	if _, err := s.SaveTarget(ctx, uuid.Nil, application.TargetInput{Name: "x", Kind: "webhook", URL: "http://127.0.0.1/x", EventPatterns: []string{"Blog Post"}}); err == nil {
		t.Error("padrão de evento inválido")
	}
	tg, err := s.SaveTarget(ctx, uuid.Nil, application.TargetInput{Name: " n8n ", Kind: "n8n", URL: "http://127.0.0.1/x?token=segredo", EventPatterns: []string{" ", "BLOG.#"}, Active: true})
	if err != nil || strings.Join(tg.EventPatterns, ",") != "blog.#" || tg.HasSecret || tg.Name != "n8n" {
		t.Fatalf("padrões normalizados, sem segredo: %+v %v", tg, err)
	}
	all, err := s.SaveTarget(ctx, tg.ID, application.TargetInput{Name: "n8n", Kind: "n8n", URL: "http://127.0.0.1/x", Active: true})
	if err != nil || strings.Join(all.EventPatterns, ",") != "#" {
		t.Fatalf("sem padrão: recebe tudo: %+v %v", all, err)
	}
	secret := "novo"
	if tg, err = s.SaveTarget(ctx, tg.ID, application.TargetInput{Name: "n8n", Kind: "n8n", URL: "http://127.0.0.1/x", Secret: &secret, Active: true}); err != nil || !tg.HasSecret {
		t.Fatalf("segredo gravado: %+v %v", tg, err)
	}
	if tg, err = s.SaveTarget(ctx, tg.ID, application.TargetInput{Name: "n8n", Kind: "n8n", URL: "http://127.0.0.1/x", Active: true}); err != nil || !tg.HasSecret {
		t.Fatalf("segredo nil mantém o atual: %+v %v", tg, err)
	}
	empty := ""
	if tg, err = s.SaveTarget(ctx, tg.ID, application.TargetInput{Name: "n8n", Kind: "n8n", URL: "http://127.0.0.1/x", Secret: &empty, Active: true}); err != nil || tg.HasSecret {
		t.Fatalf("segredo vazio remove: %+v %v", tg, err)
	}
	var audit string
	if err := e.pool.QueryRow(ctx, `SELECT diff_after::text FROM audit_logs WHERE action = 'egress.target.saved' AND resource_id = $1 ORDER BY created_at LIMIT 1`,
		tg.ID.String()).Scan(&audit); err != nil || strings.Contains(audit, "token=segredo") {
		t.Fatalf("a auditoria não guarda o token da URL: %q %v", audit, err)
	}
	if _, err := s.SaveTarget(ctx, uuid.New(), application.TargetInput{Name: "x", Kind: "webhook", URL: "http://127.0.0.1/x"}); err == nil {
		t.Error("editar destino inexistente")
	}

	res, err := s.Test(ctx, tg.ID)
	if err != nil || !res.OK || res.StatusCode != http.StatusOK {
		t.Fatalf("teste de conectividade: %+v %v", res, err)
	}
	e.dlv.err[tg.ID] = errors.New("timeout")
	if res, err = s.Test(ctx, tg.ID); err != nil || res.OK || res.Error == "" {
		t.Fatalf("teste com destino fora: %+v %v", res, err)
	}
	if _, err := s.Test(ctx, uuid.New()); err == nil {
		t.Error("testar destino inexistente")
	}
	if list, err := s.Targets(ctx); err != nil || len(list) == 0 {
		t.Fatalf("lista de destinos: %v", err)
	}
	if items, total, err := s.Deliveries(ctx, &tg.ID, "", pagination.New(1, 10, 10)); err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("entregas por destino: %v %d %v", items, total, err)
	}
	if err := s.DeleteTarget(ctx, tg.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTarget(ctx, tg.ID); err == nil {
		t.Error("excluir destino inexistente")
	}
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"Targets": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Targets(ctx); return err }
		},
		"SaveTarget": func() func(*application.Service) error {
			tg := e.target("#")
			return func(s *application.Service) error {
				_, err := s.SaveTarget(ctx, tg.ID, application.TargetInput{Name: "x", Kind: "webhook", URL: "http://127.0.0.1/x", Active: true})
				return err
			}
		},
		"DeleteTarget": func() func(*application.Service) error {
			tg := e.target("#")
			return func(s *application.Service) error { return s.DeleteTarget(ctx, tg.ID) }
		},
		"Test": func() func(*application.Service) error {
			tg := e.target("#")
			return func(s *application.Service) error { _, err := s.Test(ctx, tg.ID); return err }
		},
		"Deliveries": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, _, err := s.Deliveries(ctx, nil, "", pagination.New(1, 5, 5))
				return err
			}
		},
		"Redeliver": func() func(*application.Service) error {
			tg := e.target("x.#")
			if err := e.real().Dispatch(ctx, e.event("x.y")); err != nil {
				t.Fatal(err)
			}
			var id uuid.UUID
			if _, err := e.pool.Exec(ctx, `UPDATE egress_deliveries SET status = 'dead' WHERE target_id = $1`, tg.ID); err != nil {
				t.Fatal(err)
			}
			if err := e.pool.QueryRow(ctx, `SELECT id FROM egress_deliveries WHERE target_id = $1`, tg.ID).Scan(&id); err != nil {
				t.Fatal(err)
			}
			return func(s *application.Service) error { return s.Redeliver(ctx, id) }
		},
		"Dispatch": func() func(*application.Service) error {
			e.target("z.#")
			return func(s *application.Service) error { return s.Dispatch(ctx, e.event("z.w")) }
		},
	}
	for name, o := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := o()(e.svc(probe, 3)); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			for _, poison := range []bool{false, true} {
				r := &faultRepo{inner: infrastructure.NewRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := o()(e.svc(r, 3)); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
}

// O worker não para por falhas do banco: registra e segue.
func TestWorkerSurvivesRepositoryFailures(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, failAt := range []int{1, 2, 3, 4} { // ClaimDue, Target, EncryptedSecret, MarkDelivered/MarkFailed
		tg := e.target("w.#")
		if err := e.real().Dispatch(ctx, e.event("w.x")); err != nil {
			t.Fatal(err)
		}
		e.runOnce(e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: failAt}, 3))
		e.dlv.err[tg.ID] = errors.New("falha")
		if _, err := e.pool.Exec(ctx, `UPDATE egress_deliveries SET next_attempt_at = now() WHERE target_id = $1`, tg.ID); err != nil {
			t.Fatal(err)
		}
		e.runOnce(e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: failAt}, 3))
		if _, err := e.pool.Exec(ctx, `UPDATE egress_targets SET active = false WHERE id = $1`, tg.ID); err != nil {
			t.Fatal(err)
		}
	}
	// Mensagem de erro longa é truncada.
	tg := e.target("long.#")
	e.dlv.err[tg.ID] = errors.New(strings.Repeat("x", 600))
	if err := e.real().Dispatch(ctx, e.event("long.y")); err != nil {
		t.Fatal(err)
	}
	e.runOnce(e.real())
	var msg string
	if err := e.pool.QueryRow(ctx, `SELECT last_error FROM egress_deliveries WHERE target_id = $1`, tg.ID).Scan(&msg); err != nil || len(msg) != 500 {
		t.Fatalf("erro registrado limitado a 500 caracteres: %d %v", len(msg), err)
	}
	// Destino desativado entre a reserva e a entrega (corrida): não entrega.
	racing := e.target("race.#")
	if err := e.real().Dispatch(ctx, e.event("race.x")); err != nil {
		t.Fatal(err)
	}
	// As entregas que sobraram dos passos anteriores ficam para depois: a
	// reserva tem de trazer justamente a do destino que será desativado.
	if _, err := e.pool.Exec(ctx, `UPDATE egress_deliveries SET next_attempt_at = now() + interval '1 hour'
		WHERE target_id <> $1 AND status IN ('pending', 'failed')`, racing.ID); err != nil {
		t.Fatal(err)
	}
	flip := &flipOnClaim{faultRepo: faultRepo{inner: infrastructure.NewRepository()}, pool: e.pool, target: racing.ID}
	e.dlv.calls = nil
	e.runOnce(e.svc(flip, 3))
	if !flip.claimed {
		t.Fatal("a reserva não trouxe a entrega do destino (o teste não exercitaria a corrida)")
	}
	for _, c := range e.dlv.calls {
		if c == racing.ID {
			t.Fatal("destino desativado depois da reserva não recebe a entrega")
		}
	}
	// Sem intervalo configurado, o worker usa o padrão e para com o contexto.
	m := egress.New(modkit.Deps{Pool: e.pool, Cipher: e.cipher, Config: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	wctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := m.Workers()[0].Run(wctx); err != nil || len(m.Consumers()) != 1 || m.Manifest().Key != egress.Key {
		t.Fatalf("módulo: %v", err)
	}
}

// flipOnClaim desativa o destino logo depois da reserva.
type flipOnClaim struct {
	faultRepo
	pool    *pgxpool.Pool
	target  uuid.UUID
	claimed bool // a reserva trouxe uma entrega do destino
}

func (f *flipOnClaim) ClaimDue(ctx context.Context, db database.DBTX, limit int) ([]domain.Delivery, error) {
	out, err := f.faultRepo.ClaimDue(ctx, db, limit)
	for _, d := range out {
		f.claimed = f.claimed || d.TargetID == f.target
	}
	if _, uerr := f.pool.Exec(ctx, `UPDATE egress_targets SET active = false WHERE id = $1`, f.target); uerr != nil {
		return nil, uerr
	}
	return out, err
}

func TestHandlersReportServiceFailures(t *testing.T) {
	e := newEnv(t)
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down, 3), slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Permissions: []string{"*"}})))
		})
	})
	h.RegisterRoutes(r)
	for _, path := range []string{"/egress/targets", "/egress/deliveries"} {
		down.calls = 0
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("GET %s com o banco fora: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Error("erro desconhecido passa adiante")
	}
}

// cancelOnDeliver encerra o contexto do worker na primeira entrega.
type cancelOnDeliver struct {
	fakeDeliverer
	cancel context.CancelFunc
}

func (c *cancelOnDeliver) Deliver(ctx context.Context, t domain.Target, d domain.Delivery) (int, error) {
	c.cancel()
	return c.fakeDeliverer.Deliver(ctx, t, d)
}

// Shutdown no meio do lote: o worker para depois da entrega em curso, e as
// reservadas voltam para a fila quando a reserva expira.
func TestWorkerStopsMidBatch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, b := e.target("mid.#"), e.target("mid.#")
	if err := e.real().Dispatch(ctx, e.event("mid.x")); err != nil {
		t.Fatal(err)
	}
	wctx, cancel := context.WithCancel(ctx)
	dlv := &cancelOnDeliver{fakeDeliverer: fakeDeliverer{status: map[uuid.UUID]int{}, err: map[uuid.UUID]error{}}, cancel: cancel}
	s := application.NewService(e.pool, infrastructure.NewRepository(), dlv, e.cipher, application.Config{MaxAttempts: 3, BatchSize: 10, PollInterval: time.Hour},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.RunDeliveries(wctx); err != nil {
		t.Fatal(err)
	}
	if len(dlv.calls) != 1 {
		t.Fatalf("uma entrega em curso termina; a seguinte não começa: %v", dlv.calls)
	}
	_, _ = a, b
}
