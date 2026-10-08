package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

type fakeProvider struct {
	module string
	score  float64
	err    error
	delay  time.Duration
}

func (f fakeProvider) Module() string { return f.module }

func (f fakeProvider) Search(ctx context.Context, _ auth.Identity, q string, _ int) ([]search.Result, error) {
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if f.err != nil {
		return nil, f.err
	}
	return []search.Result{{Module: f.module, Title: q, Score: f.score}}, nil
}

func TestSearchDegradesGracefully(t *testing.T) {
	providers := []search.Provider{
		fakeProvider{module: "blog", score: 0.2},
		fakeProvider{module: "wiki", score: 0.9},
		fakeProvider{module: "files", err: errors.New("boom")},
		fakeProvider{module: "lento", delay: 5 * time.Second},
	}
	svc := NewService(func() []search.Provider { return providers }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.timeout = 100 * time.Millisecond

	resp, err := svc.Search(context.Background(), auth.Identity{}, "relatório", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 || resp.Results[0].Module != "wiki" {
		t.Fatalf("esperado wiki antes de blog (score), veio %+v", resp.Results)
	}
	if len(resp.Degraded) != 2 || resp.Degraded[0] != "files" || resp.Degraded[1] != "lento" {
		t.Fatalf("módulos degradados esperados [files lento], veio %v", resp.Degraded)
	}
}

func TestSearchRequiresMinimumQuery(t *testing.T) {
	svc := NewService(func() []search.Provider { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := svc.Search(context.Background(), auth.Identity{}, "a", "", 10); err == nil {
		t.Fatal("consulta de 1 caractere deveria ser recusada")
	}
}

// manyProvider devolve n resultados e registra o limite recebido.
type manyProvider struct {
	module string
	n      int
	got    *int
}

func (m manyProvider) Module() string { return m.module }

func (m manyProvider) Search(_ context.Context, _ auth.Identity, q string, limit int) ([]search.Result, error) {
	*m.got = limit
	out := make([]search.Result, m.n)
	for i := range out {
		out[i] = search.Result{Module: m.module, Title: q, Score: float64(i)}
	}
	return out, nil
}

func TestSearchTruncatesFiltersAndDefaultsLimit(t *testing.T) {
	var gotA, gotB int
	providers := []search.Provider{
		manyProvider{module: "blog", n: 30, got: &gotA},
		manyProvider{module: "wiki", n: 30, got: &gotB},
	}
	svc := NewService(func() []search.Provider { return providers }, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Limite fora da faixa cai para 20; o agregado corta em limit*2.
	for _, limit := range []int{0, 51} {
		resp, err := svc.Search(context.Background(), auth.Identity{}, "  ata  ", "", limit)
		if err != nil {
			t.Fatal(err)
		}
		if gotA != 20 || gotB != 20 {
			t.Fatalf("limit %d: providers deveriam receber 20, veio %d/%d", limit, gotA, gotB)
		}
		if len(resp.Results) != 40 || resp.Query != "ata" {
			t.Fatalf("limit %d: esperado 40 resultados e query aparada, veio %d %q", limit, len(resp.Results), resp.Query)
		}
		if resp.Results[0].Score < resp.Results[39].Score {
			t.Fatal("resultados deveriam vir por score decrescente")
		}
	}

	// Filtro por módulo consulta só o provider pedido.
	gotA, gotB = 0, 0
	resp, err := svc.Search(context.Background(), auth.Identity{}, "ata", "wiki", 50)
	if err != nil {
		t.Fatal(err)
	}
	if gotA != 0 || gotB != 50 || len(resp.Modules) != 1 || resp.Modules[0] != "wiki" || len(resp.Results) != 30 {
		t.Fatalf("filtro por módulo falhou: a=%d b=%d modules=%v n=%d", gotA, gotB, resp.Modules, len(resp.Results))
	}
}
