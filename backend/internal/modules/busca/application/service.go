// Package application implementa a Busca Global: consulta em paralelo os
// providers dos módulos ATIVOS (o Kernel só entrega os ativos), cada um
// com seu próprio prazo. Um provider lento ou com erro não derruba a busca
// — o resultado sai parcial e informa quais módulos falharam.
package application

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// ProviderSource devolve os providers dos módulos ativos neste instante.
type ProviderSource func() []search.Provider

// Response é o resultado agregado.
type Response struct {
	Query    string          `json:"query"`
	Results  []search.Result `json:"results"`
	Modules  []string        `json:"modules"`
	Degraded []string        `json:"degraded"`
	TookMS   int64           `json:"took_ms"`
}

// Service agrega os providers.
type Service struct {
	source  ProviderSource
	timeout time.Duration
	logger  *slog.Logger
}

// NewService cria o serviço.
func NewService(source ProviderSource, logger *slog.Logger) *Service {
	return &Service{source: source, timeout: 2 * time.Second, logger: logger}
}

// Search executa a busca global.
func (s *Service) Search(ctx context.Context, identity auth.Identity, query string, module string, limit int) (Response, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return Response{}, apperrors.Validation("digite ao menos 2 caracteres")
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	start := time.Now()
	providers := s.source()

	type outcome struct {
		module  string
		results []search.Result
		err     error
	}
	ch := make(chan outcome, len(providers))
	var wg sync.WaitGroup
	modules := []string{}
	for _, p := range providers {
		if module != "" && p.Module() != module {
			continue
		}
		modules = append(modules, p.Module())
		wg.Add(1)
		go func(p search.Provider) {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()
			res, err := p.Search(pctx, identity, query, limit)
			ch <- outcome{module: p.Module(), results: res, err: err}
		}(p)
	}
	wg.Wait()
	close(ch)

	resp := Response{Query: query, Results: []search.Result{}, Modules: modules, Degraded: []string{}}
	for o := range ch {
		if o.err != nil {
			s.logger.Warn("busca: provider falhou — resultado parcial", slog.String("module", o.module), slog.Any("error", o.err))
			resp.Degraded = append(resp.Degraded, o.module)
			continue
		}
		resp.Results = append(resp.Results, o.results...)
	}
	sort.SliceStable(resp.Results, func(i, j int) bool { return resp.Results[i].Score > resp.Results[j].Score })
	if len(resp.Results) > limit*2 {
		resp.Results = resp.Results[:limit*2]
	}
	sort.Strings(resp.Degraded)
	resp.TookMS = time.Since(start).Milliseconds()
	return resp, nil
}
