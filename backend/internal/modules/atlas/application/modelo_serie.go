package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Modelos ligados às séries da TTDD (ADR 024): o tipo de documento ganha os
// modelos para baixar na página da série.

// Eventos de auditoria dos modelos da série.
const (
	EventSerieModeloLigado    = "atlas.ttdd.modelo_ligado"
	EventSerieModeloDesligado = "atlas.ttdd.modelo_desligado"
)

// ModelosDaSerie lista os modelos da série. Desativado continua listado
// (marcado): o que já estava ligado segue baixável.
func (s *Service) ModelosDaSerie(ctx context.Context, codigo string) ([]domain.Modelo, error) {
	if _, err := s.repo.GetTTDD(ctx, s.pool, codigo); err != nil {
		return nil, MapError(err)
	}
	m, err := s.repo.ModelosDaSerie(ctx, s.pool, codigo)
	return m, mapModeloError(err)
}

// LigarModeloSerie liga um modelo ativo a uma série vigente (atlas:manage
// na rota) e devolve os modelos da série.
func (s *Service) LigarModeloSerie(ctx context.Context, identity auth.Identity, codigo string, modeloID uuid.UUID) ([]domain.Modelo, error) {
	var out []domain.Modelo
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		situacao, err := s.repo.LockTTDD(ctx, tx, codigo)
		if err != nil {
			return err
		}
		switch situacao {
		case domain.TTDDInexistente:
			return domain.ErrTTDDNotFound
		case domain.TTDDRevogada:
			return domain.InvalidError{Msg: "a série " + codigo + " foi revogada na TTDD em vigor: não recebe modelos novos"}
		}
		n, err := s.repo.ModelosAtivos(ctx, tx, []uuid.UUID{modeloID})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.InvalidError{Msg: "modelo de documento inexistente ou desativado: escolha um modelo ativo da biblioteca"}
		}
		if err := s.repo.LigarModeloSerie(ctx, tx, codigo, modeloID, autor(identity)); err != nil {
			return err
		}
		if out, err = s.repo.ModelosDaSerie(ctx, tx, codigo); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventSerieModeloLigado, "atlas_ttdd", codigo, nil,
			map[string]any{"modelo": modeloID.String()}))
	})
	return out, mapModeloError(err)
}

// DesligarModeloSerie retira o modelo da série (o modelo continua na
// biblioteca) e devolve os modelos que ficaram.
func (s *Service) DesligarModeloSerie(ctx context.Context, codigo string, modeloID uuid.UUID) ([]domain.Modelo, error) {
	var out []domain.Modelo
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) (err error) {
		if err := s.repo.DesligarModeloSerie(ctx, tx, codigo, modeloID); err != nil {
			return err
		}
		if out, err = s.repo.ModelosDaSerie(ctx, tx, codigo); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventSerieModeloDesligado, "atlas_ttdd", codigo,
			map[string]any{"modelo": modeloID.String()}, nil))
	})
	return out, mapModeloError(err)
}

// BuscarSeries alimenta a Busca Global com as séries vigentes da TTDD.
func (s *Service) BuscarSeries(ctx context.Context, query string, limit int) ([]domain.ClassificacaoTTDD, error) {
	items, _, err := s.repo.ListTTDD(ctx, s.pool, domain.FiltroTTDD{Query: query}, pagination.New(1, limit, limit))
	return items, MapError(err)
}

// BuscarModelos alimenta a Busca Global com os modelos ativos cujo nome ou
// descrição contêm todos os termos (a biblioteca é pequena: filtro em memória).
func (s *Service) BuscarModelos(ctx context.Context, query string, limit int) ([]domain.Modelo, error) {
	todos, err := s.repo.ListModelos(ctx, s.pool, false)
	if err != nil {
		return nil, mapModeloError(err)
	}
	out := []domain.Modelo{}
	for _, m := range todos {
		if len(out) < limit && domain.CasaModelo(m, query) {
			out = append(out, m)
		}
	}
	return out, nil
}

// Cobertura devolve o painel de cobertura do Atlas (atlas:manage na rota).
func (s *Service) Cobertura(ctx context.Context) (domain.Cobertura, error) {
	c, err := s.repo.Cobertura(ctx, s.pool)
	return c, MapError(err)
}
