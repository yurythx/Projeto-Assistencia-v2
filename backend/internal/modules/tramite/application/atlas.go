package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
)

// Avisos do Atlas no Trâmite (ADR 027): quando um procedimento do Atlas é
// substituído por uma versão nova (ou desativado), quem abriu um processo
// em andamento que segue aquela versão é avisado. Os módulos só se falam
// pelo barramento de eventos.

// WithNotificacoes liga os avisos (sem ele, o evento é ignorado).
func (s *Service) WithNotificacoes(n *notificacoes.Service) *Service {
	s.avisos = n
	return s
}

// procedimentoAtlas é o payload de atlas.workflow.deactivated.
type procedimentoAtlas struct {
	ID               uuid.UUID  `json:"id"`
	CodigoProcessual string     `json:"codigo_processual"`
	Titulo           string     `json:"titulo"`
	Versao           int        `json:"versao"`
	SubstituidoPor   *uuid.UUID `json:"substituido_por"`
	VersaoNova       int        `json:"versao_nova"`
}

// HandleAtlasEvent avisa os processos abertos que seguem a versão
// desativada. Idempotente (a chave do aviso inclui o processo e a versão):
// a reentrega do evento não duplica o aviso.
func (s *Service) HandleAtlasEvent(ctx context.Context, ev events.Event) error {
	var p procedimentoAtlas
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.ID == uuid.Nil || s.avisos == nil {
		return nil // payload inválido ou avisos desligados: nada a reprocessar
	}
	var enviadas []notificacoes.Enviada
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		processos, err := s.repo.ProcessosDoProcedimento(ctx, tx, p.ID)
		if err != nil {
			return err
		}
		for _, proc := range processos {
			msg := fmt.Sprintf("O processo %s segue a versão %d do procedimento %s, que foi desativada no Atlas.",
				proc.Numero, p.Versao, p.CodigoProcessual)
			if p.SubstituidoPor != nil {
				msg = fmt.Sprintf("O processo %s segue a versão %d do procedimento %s; a versão %d entrou em vigor. Confira as peças exigidas.",
					proc.Numero, p.Versao, p.CodigoProcessual, p.VersaoNova)
			}
			e, err := notificacoes.Registrar(ctx, tx, []uuid.UUID{proc.CreatedBy}, notificacoes.Nova{
				Modulo: "tramite", Titulo: "Procedimento do processo mudou: " + p.CodigoProcessual, Mensagem: msg,
				Link: "/tramite/" + proc.ID.String(), Chave: fmt.Sprintf("tramite:atlas:%s:%s", proc.ID, p.ID),
			})
			if err != nil {
				return err
			}
			enviadas = append(enviadas, e...)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.avisos.Entregar(ctx, enviadas)
	return nil
}
