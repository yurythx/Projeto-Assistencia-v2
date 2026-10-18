package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/notificacoes"
)

// Avisos de prazo da Casa Abrigo no sino (ADR 027 e ADR 032 §2/§4): o
// worker percorre as Casas Abrigo, monta o mesmo monitor do painel e avisa
// a equipe lotada na unidade. A chave leva o prazo e o nível: o mesmo
// prazo avisa uma vez quando se aproxima e outra quando atrasa; um prazo
// remarcado avisa de novo. Os textos usam as iniciais da criança.

// IntervaloAvisosCasaAbrigo é a frequência da varredura (e ela roda uma vez
// ao subir o worker).
const IntervaloAvisosCasaAbrigo = 6 * time.Hour

// WithNotificacoes liga a entrega em tempo real dos avisos.
func (s *Service) WithNotificacoes(n *notificacoes.Service) *Service {
	s.avisos = n
	return s
}

// AvisarPrazosCasaAbrigo é o worker: varre agora e a cada intervalo, até o
// contexto acabar. Uma varredura que falha é registrada no log e tentada
// de novo na próxima.
func (s *Service) AvisarPrazosCasaAbrigo(ctx context.Context) error {
	return s.avisarACada(ctx, IntervaloAvisosCasaAbrigo)
}

func (s *Service) avisarACada(ctx context.Context, intervalo time.Duration) error {
	t := time.NewTicker(intervalo)
	defer t.Stop()
	for {
		if _, err := s.AvisarPrazosUmaVez(ctx); err != nil && ctx.Err() == nil {
			s.logger.Warn("sempras: avisos de prazo da Casa Abrigo falharam", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// AvisarPrazosUmaVez avisa os prazos de todas as Casas Abrigo e devolve
// quantos avisos novos gravou. Uma unidade com erro não impede as outras.
func (s *Service) AvisarPrazosUmaVez(ctx context.Context) (int, error) {
	unidades, err := s.repo.UnidadesDoTipo(ctx, s.pool, domain.UnidadeCasaAbrigo)
	if err != nil {
		return 0, err
	}
	total := 0
	var erros []error
	for _, u := range unidades {
		var enviadas []notificacoes.Enviada
		err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
			var err error
			enviadas, err = s.avisarUnidade(ctx, tx, u)
			return err
		})
		if err != nil {
			erros = append(erros, fmt.Errorf("unidade %s: %w", u, err))
			continue
		}
		if s.avisos != nil {
			s.avisos.Entregar(ctx, enviadas)
		}
		total += len(enviadas)
	}
	return total, errors.Join(erros...)
}

// avisarUnidade grava, na transação, um aviso por alerta do monitor para a
// equipe da unidade.
func (s *Service) avisarUnidade(ctx context.Context, tx pgx.Tx, unidade uuid.UUID) ([]notificacoes.Enviada, error) {
	equipe, err := s.repo.EquipeDaUnidade(ctx, tx, unidade)
	if err != nil || len(equipe) == 0 {
		return nil, err
	}
	todos, err := s.repo.AcolhidosDaUnidade(ctx, tx, unidade)
	if err != nil {
		return nil, err
	}
	metas, err := s.repo.MetasPendentesDaUnidade(ctx, tx, unidade)
	if err != nil {
		return nil, err
	}
	relatorios, err := s.repo.ListRelatorios(ctx, tx, unidade)
	if err != nil {
		return nil, err
	}
	tarefas, err := s.repo.ListTarefas(ctx, tx, unidade)
	if err != nil {
		return nil, err
	}
	var out []notificacoes.Enviada
	for _, a := range domain.MontarPainel(s.now(), todos, metas, relatorios, tarefas).Alertas {
		enviadas, err := notificacoes.Registrar(ctx, tx, equipe, AvisoDePrazo(unidade, a))
		if err != nil {
			return nil, err
		}
		out = append(out, enviadas...)
	}
	return out, nil
}

// AvisoDePrazo monta o aviso de um alerta do monitor: o link abre a aba
// certa do espaço da unidade (o P.I.A. já no acolhido).
func AvisoDePrazo(unidade uuid.UUID, a domain.Alerta) notificacoes.Nova {
	link := fmt.Sprintf("/sempras/u/%s?aba=", unidade)
	switch a.Tipo {
	case "pia", "meta":
		link += "pia"
		if a.AcolhimentoID != nil {
			link += "&acolhido=" + a.AcolhimentoID.String()
		}
	case "relatorio":
		link += "relatorios"
	default:
		link += "tarefas"
	}
	prazo := a.Prazo.Format("02/01/2006")
	msg := fmt.Sprintf("Vence em %d dia(s), em %s.", a.Dias, prazo)
	switch {
	case a.Dias < 0:
		msg = fmt.Sprintf("Atrasado há %d dia(s): o prazo era %s.", -a.Dias, prazo)
	case a.Dias == 0:
		msg = "Vence hoje (" + prazo + ")."
	}
	return notificacoes.Nova{
		Modulo: "sempras", Titulo: cortar(a.Mensagem, 200), Mensagem: "Casa Abrigo — " + msg, Link: link,
		Chave: fmt.Sprintf("sempras:casa-abrigo:%s:%s:%s:%s", a.Tipo, a.RefID, a.Prazo.Format(time.DateOnly), a.Nivel),
	}
}

// cortar limita o texto a n caracteres (a meta é texto livre).
func cortar(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
