package application

import (
	"context"
	"net/url"
	"strings"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

// BuscaGlobal alimenta a Busca Global do sistema: procedimentos, séries
// vigentes da TTDD e modelos de documento ativos (ADR 024).
func (s *Service) BuscaGlobal(ctx context.Context, modulo, q string, limit int) ([]search.Result, error) {
	items, ranks, err := s.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]search.Result, 0, len(items))
	for i, w := range items {
		updated := w.UpdatedAt
		out = append(out, search.Result{
			Module: modulo, Type: "procedimento", ID: w.ID.String(), Title: w.CodigoProcessual + " — " + w.Titulo,
			Snippet: modkit.Snippet(w.Objetivo, 180), URL: "/atlas/procedimentos/" + w.ID.String(), Score: ranks[i], UpdatedAt: &updated,
		})
	}
	// Séries da TTDD e modelos de documento (ADR 024): pontuação na faixa
	// do ts_rank dos procedimentos, decrescente na ordem do Atlas.
	series, err := s.BuscarSeries(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	for i, c := range series {
		score := 0.05 / float64(i+1)
		if c.Codigo == strings.TrimSpace(q) {
			score = 1 // código exato: a própria série vem primeiro
		}
		out = append(out, search.Result{
			Module: modulo, Type: "serie_ttdd", ID: c.Codigo, Title: c.Codigo + " — " + c.Descritor,
			Snippet: modkit.Snippet(domain.Temporalidade(c), 180), URL: "/atlas/ttdd/" + c.Codigo, Score: score,
		})
	}
	modelos, err := s.BuscarModelos(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	for i, m := range modelos {
		updated := m.UpdatedAt
		out = append(out, search.Result{
			Module: modulo, Type: "modelo", ID: m.ID.String(), Title: "Modelo: " + m.Nome,
			Snippet: modkit.Snippet(m.Descricao, 180), URL: "/atlas/modelos?q=" + url.QueryEscape(m.Nome), Score: 0.08 / float64(i+1),
			UpdatedAt: &updated,
		})
	}
	return out, nil
}
