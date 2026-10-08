package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
)

// Validação dos procedimentos (ADR 028): o fluxo nasce RASCUNHO (ex.: pela
// importação), fica EM_VALIDACAO enquanto os departamentos são entrevistados
// e só é publicado ao ser HOMOLOGADO. Antes disso não aparece na consulta,
// na busca nem no assistente, não substitui a versão em vigor e não avisa
// ninguém.

// Eventos de auditoria da validação.
const (
	EventSituacao     = "atlas.workflow.situacao"
	EventHomologado   = "atlas.workflow.homologado"
	EventEntrevistado = "atlas.workflow.validacao"
)

// MudarSituacao alterna um fluxo não homologado entre rascunho e em
// validação (o homologado não volta: a revisão é uma nova versão).
func (s *Service) MudarSituacao(ctx context.Context, id uuid.UUID, situacao string) (domain.Workflow, error) {
	if situacao != domain.SituacaoRascunho && situacao != domain.SituacaoEmValidacao {
		return domain.Workflow{}, MapError(domain.InvalidError{Msg: "situação deve ser RASCUNHO ou EM_VALIDACAO (para publicar, homologue)"})
	}
	var out domain.Workflow
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		w, err := s.repo.Get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if w.Situacao == domain.SituacaoHomologado {
			return domain.InvalidError{Msg: "procedimento homologado não volta a rascunho: publique uma nova versão para revisá-lo"}
		}
		if err := s.repo.SetSituacao(ctx, tx, id, situacao, false); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, id, false); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventSituacao, "atlas_workflow", id.String(),
			map[string]any{"situacao": w.Situacao}, map[string]any{"situacao": situacao}))
	})
	return out, MapError(err)
}

// Homologar publica o fluxo validado: fica ativo, as demais versões do
// mesmo código são desativadas (substituídas) e os interessados são avisados.
func (s *Service) Homologar(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.Workflow, error) {
	var (
		out      domain.Workflow
		enviadas []notificacoes.Enviada
	)
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		w, err := s.repo.Get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if w.Situacao == domain.SituacaoHomologado {
			return domain.InvalidError{Msg: "procedimento já homologado"}
		}
		// A série tem de estar vigente na publicação (pode ter sido revogada
		// durante a validação).
		situacao, err := s.repo.LockTTDD(ctx, tx, w.CodigoTTDD)
		if err != nil {
			return err
		}
		if situacao != domain.TTDDVigente {
			return domain.InvalidError{Msg: "a série " + w.CodigoTTDD + " não está vigente na TTDD: enquadre o procedimento numa série vigente antes de homologar"}
		}
		if err := s.repo.SetSituacao(ctx, tx, id, domain.SituacaoHomologado, true); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, id, false); err != nil {
			return err
		}
		ids, err := s.repo.DesativarVersoes(ctx, tx, out.CodigoProcessual, out.ID)
		if err != nil {
			return err
		}
		var anterior *domain.Workflow
		for _, vid := range ids {
			antiga, err := s.repo.Get(ctx, tx, vid, false)
			if err != nil {
				return err
			}
			if err := s.substituida(ctx, tx, antiga, out); err != nil {
				return err
			}
			anterior = &antiga
		}
		if err := s.event(ctx, tx, EventWorkflowActivated, out); err != nil {
			return err
		}
		if anterior != nil {
			if enviadas, err = s.avisarNovaVersao(ctx, tx, identity, *anterior, out); err != nil {
				return err
			}
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventHomologado, "atlas_workflow", id.String(),
			map[string]any{"situacao": w.Situacao, "ativo": false}, map[string]any{"situacao": domain.SituacaoHomologado, "ativo": true}))
	})
	if err == nil {
		s.entregar(ctx, enviadas)
	}
	return out, MapError(err)
}

// Validacoes devolve o registro das entrevistas do procedimento (pelo
// código: inclui as das versões anteriores).
func (s *Service) Validacoes(ctx context.Context, id uuid.UUID) ([]domain.Validacao, error) {
	w, err := s.repo.Get(ctx, s.pool, id, false)
	if err != nil {
		return nil, MapError(err)
	}
	v, err := s.repo.Validacoes(ctx, s.pool, w.CodigoProcessual)
	return v, MapError(err)
}

// RegistrarValidacao grava uma entrevista; o rascunho passa a em validação.
func (s *Service) RegistrarValidacao(ctx context.Context, identity auth.Identity, id uuid.UUID, v domain.Validacao) (domain.Validacao, error) {
	v.Normalizar()
	if err := v.Validar(s.now()); err != nil {
		return v, MapError(err)
	}
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		w, err := s.repo.Get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		v.ID, v.CodigoProcessual, v.CreatedBy = uuid.New(), w.CodigoProcessual, autor(identity)
		if err := s.repo.InsertValidacao(ctx, tx, v); err != nil {
			return err
		}
		if w.Situacao == domain.SituacaoRascunho {
			if err := s.repo.SetSituacao(ctx, tx, id, domain.SituacaoEmValidacao, false); err != nil {
				return err
			}
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventEntrevistado, "atlas_workflow", id.String(), nil,
			map[string]any{"unidade": v.Unidade, "realizada_em": v.RealizadaEm.Format("2006-01-02"), "pendencias": v.Pendencias != ""}))
	})
	return v, MapError(err)
}
