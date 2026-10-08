package kernel

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
)

type memStore struct {
	mu     sync.Mutex
	state  map[string]bool
	notify chan struct{}
}

func newMemStore() *memStore {
	return &memStore{state: map[string]bool{}, notify: make(chan struct{}, 8)}
}

func (s *memStore) Ensure(_ context.Context, key string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.state[key]; !ok {
		s.state[key] = enabled
	}
	return nil
}

func (s *memStore) LoadAll(context.Context) (map[string]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]bool{}
	for k, v := range s.state {
		out[k] = v
	}
	return out, nil
}

func (s *memStore) Set(_ context.Context, key string, enabled bool, _ string, _ audit.Entry) error {
	s.mu.Lock()
	s.state[key] = enabled
	s.mu.Unlock()
	return nil
}

func (s *memStore) Listen(ctx context.Context, onChange func()) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.notify:
			onChange()
		}
	}
}

type fakePlugin struct {
	m       Manifest
	workers []Worker
	routes  func(Routes)
}

func (p *fakePlugin) Manifest() Manifest { return p.m }
func (p *fakePlugin) Workers() []Worker  { return p.workers }
func (p *fakePlugin) RegisterRoutes(r Routes) {
	if p.routes != nil {
		p.routes(r)
	}
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newKernel(t *testing.T, plugins ...Plugin) (*Kernel, *memStore) {
	t.Helper()
	st := newMemStore()
	k := New(st, quiet())
	for _, p := range plugins {
		if err := k.RegisterModule(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return k, st
}

func TestRegisterModuleValidation(t *testing.T) {
	k := New(newMemStore(), quiet())
	if err := k.RegisterModule(&fakePlugin{m: Manifest{Key: "Inválido!"}}); err == nil {
		t.Fatal("chave inválida deveria falhar")
	}
	_ = k.RegisterModule(&fakePlugin{m: Manifest{Key: "blog"}})
	if err := k.RegisterModule(&fakePlugin{m: Manifest{Key: "blog"}}); err == nil {
		t.Fatal("chave duplicada deveria falhar")
	}
	k2 := New(newMemStore(), quiet())
	_ = k2.RegisterModule(&fakePlugin{m: Manifest{Key: "tramite", DependsOn: []string{"signum"}}})
	if err := k2.Start(context.Background()); err == nil {
		t.Fatal("dependência não registrada deveria falhar no Start")
	}
	k3 := New(newMemStore(), quiet())
	_ = k3.RegisterModule(&fakePlugin{m: Manifest{Key: "aa", DependsOn: []string{"bb"}}})
	_ = k3.RegisterModule(&fakePlugin{m: Manifest{Key: "bb", DependsOn: []string{"aa"}}})
	if err := k3.Start(context.Background()); err == nil {
		t.Fatal("ciclo de dependência deveria falhar")
	}
}

func TestCoreCannotBeDisabledAndDependencies(t *testing.T) {
	k, _ := newKernel(t,
		&fakePlugin{m: Manifest{Key: "iam", Core: true}},
		&fakePlugin{m: Manifest{Key: "signum", DefaultEnabled: true}},
		&fakePlugin{m: Manifest{Key: "tramite", DefaultEnabled: true, DependsOn: []string{"signum"}}},
	)
	ctx := context.Background()
	if err := k.SetEnabled(ctx, "iam", false, "t"); !errors.Is(err, ErrCoreModule) {
		t.Fatalf("núcleo: esperado ErrCoreModule, veio %v", err)
	}
	if err := k.SetEnabled(ctx, "signum", false, "t"); !errors.Is(err, ErrDependentActive) {
		t.Fatalf("esperado ErrDependentActive, veio %v", err)
	}
	if err := k.SetEnabled(ctx, "tramite", false, "t"); err != nil {
		t.Fatal(err)
	}
	if err := k.SetEnabled(ctx, "signum", false, "t"); err != nil {
		t.Fatal(err)
	}
	if err := k.SetEnabled(ctx, "tramite", true, "t"); !errors.Is(err, ErrDependencyMissing) {
		t.Fatalf("esperado ErrDependencyMissing, veio %v", err)
	}
	if err := k.SetEnabled(ctx, "nao_existe", true, "t"); !errors.Is(err, ErrUnknownModule) {
		t.Fatalf("esperado ErrUnknownModule, veio %v", err)
	}
}

func TestGuardReturns404WhenDisabledAtRuntime(t *testing.T) {
	p := &fakePlugin{m: Manifest{Key: "blog", DefaultEnabled: true}, routes: func(r Routes) {
		r.Authed.Get("/blog/posts", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	}}
	k, _ := newKernel(t, p)
	router := chi.NewRouter()
	pub, authed := chi.NewRouter(), chi.NewRouter()
	k.MountRoutes(pub, authed, quiet())
	router.Mount("/", authed)

	do := func() int {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/blog/posts", nil))
		return rec.Code
	}
	if code := do(); code != http.StatusOK {
		t.Fatalf("ativo: esperado 200, veio %d", code)
	}
	if err := k.SetEnabled(context.Background(), "blog", false, "t"); err != nil {
		t.Fatal(err)
	}
	if code := do(); code != http.StatusNotFound {
		t.Fatalf("desativado: esperado 404, veio %d", code)
	}
}

func TestSuperviseStopsAndRestartsWorkersOnToggle(t *testing.T) {
	var running atomic.Int32
	var starts atomic.Int32
	p := &fakePlugin{m: Manifest{Key: "egress", DefaultEnabled: true}, workers: []Worker{{
		Name: "delivery", Process: ProcessWorker,
		Run: func(ctx context.Context) error {
			starts.Add(1)
			running.Add(1)
			<-ctx.Done()
			running.Add(-1)
			return nil
		},
	}}}
	k, _ := newKernel(t, p)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = k.Supervise(ctx, ProcessWorker, nil, nil); close(done) }()

	waitFor(t, func() bool { return running.Load() == 1 })
	_ = k.SetEnabled(context.Background(), "egress", false, "t")
	waitFor(t, func() bool { return running.Load() == 0 })
	_ = k.SetEnabled(context.Background(), "egress", true, "t")
	waitFor(t, func() bool { return running.Load() == 1 && starts.Load() == 2 })

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Supervise não terminou após o cancelamento do context")
	}
	if running.Load() != 0 {
		t.Fatal("worker deveria ter parado no shutdown")
	}
}

func TestLifecycleHooksFireOnChange(t *testing.T) {
	k, _ := newKernel(t, &fakePlugin{m: Manifest{Key: "mercurio", DefaultEnabled: true}})
	var got []bool
	k.OnChange(func(_ context.Context, key string, enabled bool) {
		if key == "mercurio" {
			got = append(got, enabled)
		}
	})
	_ = k.SetEnabled(context.Background(), "mercurio", false, "t")
	_ = k.SetEnabled(context.Background(), "mercurio", true, "t")
	if len(got) != 2 || got[0] || !got[1] {
		t.Fatalf("hooks esperados [false true], veio %v", got)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condição não atingida a tempo")
}
