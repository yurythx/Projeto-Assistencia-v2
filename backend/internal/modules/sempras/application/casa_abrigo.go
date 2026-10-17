package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/yurythx/projeto-assistencia-v2/internal/domain/errors"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/audit"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/modkit"
)

// --------------------------------------------------------------- Casa Abrigo
//
// Acolhimento institucional com paridade ao sistema da unidade (ADR 032).
// Toda ação exige sempras:atendimento numa unidade que oferta o
// acolhimento; as listas com nome de criança gravam a leitura na
// auditoria, como a ficha.

// casaAbrigo confere a unidade: equipe técnica numa unidade que acolhe.
func (s *Service) casaAbrigo(ctx context.Context, db database.DBTX, identity auth.Identity, unidade uuid.UUID) (domain.Unidade, error) {
	return s.unidade(ctx, db, identity, unidade, domain.ServicoAcolhimento, auth.PermSemprasAtendimento)
}

// acolhimento carrega o acolhimento e confere a unidade dele.
func (s *Service) acolhimento(ctx context.Context, db database.DBTX, identity auth.Identity, id uuid.UUID, forUpdate bool) (domain.Acolhimento, error) {
	a, err := s.repo.GetAcolhimento(ctx, db, id, forUpdate)
	if err != nil {
		return domain.Acolhimento{}, err
	}
	if _, err := s.casaAbrigo(ctx, db, identity, a.UnidadeID); err != nil {
		return domain.Acolhimento{}, err
	}
	return a, nil
}

// doAcolhido confere que o acolhimento informado (opcional) é da unidade.
func (s *Service) doAcolhido(ctx context.Context, db database.DBTX, acolhimento *uuid.UUID, unidade uuid.UUID) error {
	if acolhimento == nil {
		return nil
	}
	a, err := s.repo.GetAcolhimento(ctx, db, *acolhimento, false)
	if err != nil {
		return err
	}
	if a.UnidadeID != unidade {
		return &domain.ValidationError{Msg: "o acolhido é de outra unidade"}
	}
	return nil
}

// lerAuditado roda a leitura numa transação e grava a leitura na
// auditoria — sem auditoria, sem lista (como a ficha, ADR 030 §6).
func (s *Service) lerAuditado(ctx context.Context, acao, entidade, id string, ler func(ctx context.Context, tx pgx.Tx) (int, error)) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		n, err := ler(ctx, tx)
		if err != nil {
			return err
		}
		e := audit.Meta(ctx, acao, entidade, id, nil, nil)
		e.Metadata = map[string]any{"itens": n}
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// completarAcolhimento carrega o que é aninhado ao acolhimento (irmãos,
// diagnósticos, visitas, termos, PIA, metas e documentos).
func (s *Service) completarAcolhimento(ctx context.Context, tx pgx.Tx, a *domain.Acolhimento) error {
	var err error
	if a.Irmaos, err = s.repo.ListIrmaos(ctx, tx, a.ID); err != nil {
		return err
	}
	if a.Diagnosticos, err = s.repo.ListDiagnosticos(ctx, tx, a.ID); err != nil {
		return err
	}
	if a.Visitantes, err = s.repo.ListVisitantes(ctx, tx, a.ID); err != nil {
		return err
	}
	if a.Termos, err = s.repo.ListTermos(ctx, tx, a.ID); err != nil {
		return err
	}
	if a.PiaMetas, err = s.repo.ListPiaMetas(ctx, tx, a.ID); err != nil {
		return err
	}
	if a.Documentos, err = s.repo.ListDocumentos(ctx, tx, a.ID); err != nil {
		return err
	}
	switch pia, err := s.repo.GetPiaFicha(ctx, tx, a.ID); {
	case errors.Is(err, domain.ErrNotFound):
		// PIA ainda não preenchido — fica nil, não é erro.
	case err != nil:
		return err
	default:
		a.Pia = &pia
	}
	return nil
}

// IrmaoInput é um irmão marcado no assistente (R6).
type IrmaoInput struct {
	Nome             string
	DataNascimento   *time.Time
	EstaAcolhido     bool
	UnidadeDescricao string
	Observacao       string
}

// DiagnosticoInput é um diagnóstico marcado no checklist do assistente (R6).
type DiagnosticoInput struct {
	Categoria, Diagnostico, Descricao string
}

// AcolhimentoInput é o assistente de 6 etapas: os dados, a data na
// unidade e os irmãos e diagnósticos marcados (gravados na mesma transação).
type AcolhimentoInput struct {
	Dados        domain.AcolhimentoDados
	DataEntrada  time.Time
	Irmaos       []IrmaoInput
	Diagnosticos []DiagnosticoInput
}

// RegistrarAcolhimento abre um acolhimento institucional (nasce
// 'acolhido'). R1 a R3 e R6 do ADR 032.
func (s *Service) RegistrarAcolhimento(ctx context.Context, identity auth.Identity, cidadao, unidade uuid.UUID, in AcolhimentoInput) (domain.Acolhimento, error) {
	var a domain.Acolhimento
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.casaAbrigo(ctx, tx, identity, unidade)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		c, err := s.repo.GetCidadao(ctx, tx, cidadao, false)
		if err != nil {
			return err
		}
		if a, err = domain.NovoAcolhimento(cidadao, unidade, c.DataNascimento, in.Dados, in.DataEntrada, now); err != nil {
			return err
		}
		irmaos := make([]domain.Irmao, 0, len(in.Irmaos))
		for _, i := range in.Irmaos {
			irmao, err := domain.NovoIrmao(a.ID, i.Nome, i.DataNascimento, i.EstaAcolhido, i.UnidadeDescricao, i.Observacao)
			if err != nil {
				return err
			}
			irmao.CreatedAt = now
			irmaos = append(irmaos, irmao)
		}
		diagnosticos := make([]domain.Diagnostico, 0, len(in.Diagnosticos))
		for _, d := range in.Diagnosticos {
			diag, err := domain.NovoDiagnostico(a.ID, d.Categoria, d.Diagnostico, d.Descricao)
			if err != nil {
				return err
			}
			diag.CreatedAt = now
			diagnosticos = append(diagnosticos, diag)
		}
		if err := s.repo.InsertAcolhimento(ctx, tx, a, identity.UserID); err != nil {
			return err
		}
		for _, i := range irmaos {
			if err := s.repo.InsertIrmao(ctx, tx, i); err != nil {
				return err
			}
		}
		for _, d := range diagnosticos {
			if err := s.repo.InsertDiagnostico(ctx, tx, d); err != nil {
				return err
			}
		}
		a.Irmaos, a.Diagnosticos = irmaos, diagnosticos
		e := audit.Meta(ctx, "sempras.casa_abrigo.acolhimento_registrado", "sempras_casa_abrigo_acolhimento", a.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "unidade_id": unidade, "irmaos": len(irmaos), "diagnosticos": len(diagnosticos)})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Acolhimento{}, err
	}
	return a, nil
}

// AtualizarAcolhimento regrava o assistente e a data de entrada — nunca o
// status nem o desligamento (só DesligarAcolhimento muda isso).
func (s *Service) AtualizarAcolhimento(ctx context.Context, identity auth.Identity, id uuid.UUID, dados domain.AcolhimentoDados, entrada time.Time) (domain.Acolhimento, error) {
	var a domain.Acolhimento
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if a, err = s.acolhimento(ctx, tx, identity, id, true); err != nil {
			return err
		}
		c, err := s.repo.GetCidadao(ctx, tx, a.CidadaoID, false)
		if err != nil {
			return err
		}
		if err := a.Atualizar(c.DataNascimento, dados, entrada, now); err != nil {
			return err
		}
		if err := s.repo.UpdateAcolhimento(ctx, tx, a); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.acolhimento_atualizado", "sempras_casa_abrigo_acolhimento", id.String(), nil, nil)
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Acolhimento{}, err
	}
	return a, nil
}

// DesligarAcolhimento encerra o acolhimento (R5) — ação restrita à equipe
// técnica, sem equivalente em AtualizarAcolhimento.
func (s *Service) DesligarAcolhimento(ctx context.Context, identity auth.Identity, id uuid.UUID, d domain.Desligamento) (domain.Acolhimento, error) {
	var a domain.Acolhimento
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if a, err = s.acolhimento(ctx, tx, identity, id, true); err != nil {
			return err
		}
		if err := a.Desligar(d, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateAcolhimento(ctx, tx, a); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.desligado", "sempras_casa_abrigo_acolhimento", id.String(), nil,
			map[string]any{"cidadao_id": a.CidadaoID, "tipo": a.DesligamentoTipo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Acolhimento{}, err
	}
	return a, nil
}

// AcolhimentoDetalhe é o acolhimento completo com o cadastro do munícipe
// (o assistente em modo de edição e a aba do PIA).
type AcolhimentoDetalhe struct {
	domain.Acolhimento
	Cidadao domain.Cidadao `json:"cidadao"`
}

// ObterAcolhimento devolve o acolhimento completo — leitura auditada.
func (s *Service) ObterAcolhimento(ctx context.Context, identity auth.Identity, id uuid.UUID) (AcolhimentoDetalhe, error) {
	var out AcolhimentoDetalhe
	err := s.lerAuditado(ctx, "sempras.casa_abrigo.acolhimento_visualizado", "sempras_casa_abrigo_acolhimento", id.String(),
		func(ctx context.Context, tx pgx.Tx) (int, error) {
			a, err := s.acolhimento(ctx, tx, identity, id, false)
			if err != nil {
				return 0, err
			}
			if err := s.completarAcolhimento(ctx, tx, &a); err != nil {
				return 0, err
			}
			c, err := s.repo.GetCidadao(ctx, tx, a.CidadaoID, false)
			if err != nil {
				return 0, err
			}
			out = AcolhimentoDetalhe{Acolhimento: a, Cidadao: c}
			return 1, nil
		})
	if err != nil {
		return AcolhimentoDetalhe{}, err
	}
	return out, nil
}

// AdicionarIrmao registra um irmão do acolhido.
func (s *Service) AdicionarIrmao(ctx context.Context, identity auth.Identity, acolhimento uuid.UUID, in IrmaoInput) (domain.Irmao, error) {
	i, err := domain.NovoIrmao(acolhimento, in.Nome, in.DataNascimento, in.EstaAcolhido, in.UnidadeDescricao, in.Observacao)
	if err != nil {
		return domain.Irmao{}, err
	}
	i.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.acolhimento(ctx, tx, identity, acolhimento, false); err != nil {
			return err
		}
		if err := s.repo.InsertIrmao(ctx, tx, i); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.irmao_adicionado", "sempras_casa_abrigo_irmao", i.ID.String(), nil,
			map[string]any{"acolhimento_id": acolhimento})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Irmao{}, err
	}
	return i, nil
}

// RemoverIrmao remove um irmão registrado por engano.
func (s *Service) RemoverIrmao(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		i, err := s.repo.GetIrmao(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := s.acolhimento(ctx, tx, identity, i.AcolhimentoID, false); err != nil {
			return err
		}
		if err := s.repo.DeleteIrmao(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.irmao_removido", "sempras_casa_abrigo_irmao", id.String(), nil,
			map[string]any{"acolhimento_id": i.AcolhimentoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// AdicionarDiagnostico registra um diagnóstico de saúde/deficiência/
// transtorno do acolhido (o checklist da etapa 5 marca e desmarca).
func (s *Service) AdicionarDiagnostico(ctx context.Context, identity auth.Identity, acolhimento uuid.UUID, in DiagnosticoInput) (domain.Diagnostico, error) {
	d, err := domain.NovoDiagnostico(acolhimento, in.Categoria, in.Diagnostico, in.Descricao)
	if err != nil {
		return domain.Diagnostico{}, err
	}
	d.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.acolhimento(ctx, tx, identity, acolhimento, false); err != nil {
			return err
		}
		if err := s.repo.InsertDiagnostico(ctx, tx, d); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.diagnostico_adicionado", "sempras_casa_abrigo_diagnostico", d.ID.String(), nil,
			map[string]any{"acolhimento_id": acolhimento, "categoria": d.Categoria})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Diagnostico{}, err
	}
	return d, nil
}

// RemoverDiagnostico remove um diagnóstico (desmarcado no checklist).
func (s *Service) RemoverDiagnostico(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		d, err := s.repo.GetDiagnostico(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := s.acolhimento(ctx, tx, identity, d.AcolhimentoID, false); err != nil {
			return err
		}
		if err := s.repo.DeleteDiagnostico(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.diagnostico_removido", "sempras_casa_abrigo_diagnostico", id.String(), nil,
			map[string]any{"acolhimento_id": d.AcolhimentoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// ------------------------------------------------------------ visitantes

// AdicionarVisitante registra uma visita a um acolhido (pela ficha).
func (s *Service) AdicionarVisitante(ctx context.Context, identity auth.Identity, acolhimento uuid.UUID, d domain.VisitanteDados) (domain.Visitante, error) {
	d.AcolhimentoID, d.TipoVisita = &acolhimento, "acolhido"
	var v domain.Visitante
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, err := s.acolhimento(ctx, tx, identity, acolhimento, false)
		if err != nil {
			return err
		}
		v, err = s.inserirVisitante(ctx, tx, identity, a.UnidadeID, d)
		return err
	})
	if err != nil {
		return domain.Visitante{}, err
	}
	return v, nil
}

// RegistrarVisita registra a entrada de um visitante na unidade (tela de
// visitantes) — a um acolhido, à instituição, doação ou voluntariado.
func (s *Service) RegistrarVisita(ctx context.Context, identity auth.Identity, unidade uuid.UUID, d domain.VisitanteDados) (domain.Visitante, error) {
	var v domain.Visitante
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
			return err
		}
		if err := s.doAcolhido(ctx, tx, d.AcolhimentoID, unidade); err != nil {
			return err
		}
		var err error
		v, err = s.inserirVisitante(ctx, tx, identity, unidade, d)
		return err
	})
	if err != nil {
		return domain.Visitante{}, err
	}
	return v, nil
}

func (s *Service) inserirVisitante(ctx context.Context, tx pgx.Tx, identity auth.Identity, unidade uuid.UUID, d domain.VisitanteDados) (domain.Visitante, error) {
	v, err := domain.NovoVisitante(unidade, d)
	if err != nil {
		return domain.Visitante{}, err
	}
	v.CreatedAt = s.now()
	if err := s.repo.InsertVisitante(ctx, tx, v, identity.UserID); err != nil {
		return domain.Visitante{}, err
	}
	e := audit.Meta(ctx, "sempras.casa_abrigo.visitante_adicionado", "sempras_casa_abrigo_visitante", v.ID.String(), nil,
		map[string]any{"unidade_id": unidade, "acolhimento_id": v.AcolhimentoID, "tipo_visita": v.TipoVisita})
	return v, audit.NewWriter(tx).Record(ctx, e)
}

// RegistrarSaidaVisitante anota a hora de saída do visitante.
func (s *Service) RegistrarSaidaVisitante(ctx context.Context, identity auth.Identity, id uuid.UUID, hora string) (domain.Visitante, error) {
	var v domain.Visitante
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if v, err = s.repo.GetVisitante(ctx, tx, id); err != nil {
			return err
		}
		if _, err := s.casaAbrigo(ctx, tx, identity, v.UnidadeID); err != nil {
			return err
		}
		if err := v.RegistrarSaida(hora); err != nil {
			return err
		}
		if err := s.repo.UpdateVisitante(ctx, tx, v); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.visitante_saida", "sempras_casa_abrigo_visitante", id.String(), nil,
			map[string]any{"hora_saida": v.HoraSaida})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Visitante{}, err
	}
	return v, nil
}

// RemoverVisitante remove uma visita registrada por engano.
func (s *Service) RemoverVisitante(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		v, err := s.repo.GetVisitante(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := s.casaAbrigo(ctx, tx, identity, v.UnidadeID); err != nil {
			return err
		}
		if err := s.repo.DeleteVisitante(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.visitante_removido", "sempras_casa_abrigo_visitante", id.String(), nil,
			map[string]any{"acolhimento_id": v.AcolhimentoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// VisitantesDaUnidade lista as visitas recentes da unidade — leitura auditada.
func (s *Service) VisitantesDaUnidade(ctx context.Context, identity auth.Identity, unidade uuid.UUID) ([]domain.Visitante, error) {
	var out []domain.Visitante
	err := s.lerAuditado(ctx, "sempras.casa_abrigo.visitantes_visualizados", "unidade", unidade.String(),
		func(ctx context.Context, tx pgx.Tx) (int, error) {
			if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
				return 0, err
			}
			var err error
			out, err = s.repo.ListVisitantesDaUnidade(ctx, tx, unidade)
			return len(out), err
		})
	return out, err
}

// ---------------------------------------------------------------- termos

// RegistrarTermo registra um termo de advertência, passeio ou saída.
func (s *Service) RegistrarTermo(ctx context.Context, identity auth.Identity, acolhimento uuid.UUID, tipoTermo, responsavelRetirada, parentescoResponsavel, motivoObservacao string, dataTermo time.Time) (domain.Termo, error) {
	t, err := domain.NovoTermo(acolhimento, tipoTermo, responsavelRetirada, parentescoResponsavel, motivoObservacao, dataTermo)
	if err != nil {
		return domain.Termo{}, err
	}
	t.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.acolhimento(ctx, tx, identity, acolhimento, false); err != nil {
			return err
		}
		if err := s.repo.InsertTermo(ctx, tx, t, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.termo_registrado", "sempras_casa_abrigo_termo", t.ID.String(), nil,
			map[string]any{"acolhimento_id": acolhimento, "tipo_termo": t.TipoTermo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Termo{}, err
	}
	return t, nil
}

// RemoverTermo remove um termo registrado por engano.
func (s *Service) RemoverTermo(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		t, err := s.repo.GetTermo(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := s.acolhimento(ctx, tx, identity, t.AcolhimentoID, false); err != nil {
			return err
		}
		if err := s.repo.DeleteTermo(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.termo_removido", "sempras_casa_abrigo_termo", id.String(), nil,
			map[string]any{"acolhimento_id": t.AcolhimentoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// TermosDaUnidade lista os termos recentes da unidade — leitura auditada.
func (s *Service) TermosDaUnidade(ctx context.Context, identity auth.Identity, unidade uuid.UUID) ([]domain.Termo, error) {
	var out []domain.Termo
	err := s.lerAuditado(ctx, "sempras.casa_abrigo.termos_visualizados", "unidade", unidade.String(),
		func(ctx context.Context, tx pgx.Tx) (int, error) {
			if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
				return 0, err
			}
			var err error
			out, err = s.repo.ListTermosDaUnidade(ctx, tx, unidade)
			return len(out), err
		})
	return out, err
}

// ------------------------------------------------------------------- PIA

// SalvarPiaFicha grava a ficha do PIA (as 6 abas). Gravar é reavaliar:
// o prazo da próxima reavaliação conta a partir daqui (R7).
func (s *Service) SalvarPiaFicha(ctx context.Context, identity auth.Identity, acolhimento uuid.UUID, d domain.PiaFichaDados) (domain.PiaFicha, error) {
	p, err := domain.NovaPiaFicha(acolhimento, d)
	if err != nil {
		return domain.PiaFicha{}, err
	}
	p.AtualizadoEm, p.AtualizadoPor = s.now(), &identity.UserID
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.acolhimento(ctx, tx, identity, acolhimento, false); err != nil {
			return err
		}
		if err := s.repo.UpsertPiaFicha(ctx, tx, p, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.pia_atualizado", "sempras_casa_abrigo_pia", acolhimento.String(), nil, nil)
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PiaFicha{}, err
	}
	return p, nil
}

// AdicionarPiaMeta registra uma meta do plano de ação ou da intervenção.
func (s *Service) AdicionarPiaMeta(ctx context.Context, identity auth.Identity, acolhimento uuid.UUID, d domain.PiaMetaDados) (domain.PiaMeta, error) {
	m, err := domain.NovaPiaMeta(acolhimento, d)
	if err != nil {
		return domain.PiaMeta{}, err
	}
	m.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.acolhimento(ctx, tx, identity, acolhimento, false); err != nil {
			return err
		}
		if err := s.repo.InsertPiaMeta(ctx, tx, m, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.pia_meta_adicionada", "sempras_casa_abrigo_pia_meta", m.ID.String(), nil,
			map[string]any{"acolhimento_id": acolhimento, "tipo": m.Tipo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PiaMeta{}, err
	}
	return m, nil
}

// alterarPiaMeta carrega a meta, confere a unidade, aplica a mudança e grava.
func (s *Service) alterarPiaMeta(ctx context.Context, identity auth.Identity, id uuid.UUID, mudar func(*domain.PiaMeta) error) (domain.PiaMeta, error) {
	var m domain.PiaMeta
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if m, err = s.repo.GetPiaMeta(ctx, tx, id); err != nil {
			return err
		}
		if _, err := s.acolhimento(ctx, tx, identity, m.AcolhimentoID, false); err != nil {
			return err
		}
		if err := mudar(&m); err != nil {
			return err
		}
		if err := s.repo.UpdatePiaMeta(ctx, tx, m); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.pia_meta_atualizada", "sempras_casa_abrigo_pia_meta", id.String(), nil,
			map[string]any{"concluida": m.Concluida})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PiaMeta{}, err
	}
	return m, nil
}

// EditarPiaMeta regrava uma meta (a tabela do plano de ação é editável).
func (s *Service) EditarPiaMeta(ctx context.Context, identity auth.Identity, id uuid.UUID, d domain.PiaMetaDados) (domain.PiaMeta, error) {
	return s.alterarPiaMeta(ctx, identity, id, func(m *domain.PiaMeta) error { return m.Editar(d) })
}

// ConcluirPiaMeta marca (ou desmarca) uma meta do PIA como concluída.
func (s *Service) ConcluirPiaMeta(ctx context.Context, identity auth.Identity, id uuid.UUID, concluida bool) (domain.PiaMeta, error) {
	return s.alterarPiaMeta(ctx, identity, id, func(m *domain.PiaMeta) error { m.Concluida = concluida; return nil })
}

// RemoverPiaMeta remove uma meta registrada por engano.
func (s *Service) RemoverPiaMeta(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		m, err := s.repo.GetPiaMeta(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := s.acolhimento(ctx, tx, identity, m.AcolhimentoID, false); err != nil {
			return err
		}
		if err := s.repo.DeletePiaMeta(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.pia_meta_removida", "sempras_casa_abrigo_pia_meta", id.String(), nil,
			map[string]any{"acolhimento_id": m.AcolhimentoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// ------------------------------------------------------------ documentos

// documentoPrefix é o prefixo dos objetos no bucket — um "subdiretório"
// por acolhimento.
func documentoPrefix(acolhimento uuid.UUID) string {
	return "sempras/casa-abrigo/" + acolhimento.String()
}

// documentoTypes: só PDF.
var documentoTypes = []string{"application/pdf"}

// UploadDocumentoResult é o ticket de upload + o metadado pendente.
type UploadDocumentoResult struct {
	Documento domain.DocumentoCasaAbrigo `json:"documento"`
	Upload    modkit.UploadTicket        `json:"upload"`
}

// StartUploadDocumento cria o metadado pendente e a URL de upload direto
// ao object storage (o conteúdo nunca atravessa a API).
func (s *Service) StartUploadDocumento(ctx context.Context, identity auth.Identity, acolhimento uuid.UUID, tipo, filename, contentType string, size int64) (UploadDocumentoResult, error) {
	if size <= 0 || size > s.maxDocBytes {
		return UploadDocumentoResult{}, &domain.ValidationError{Msg: "tamanho de arquivo acima do limite permitido"}
	}
	if _, err := s.acolhimento(ctx, s.pool, identity, acolhimento, false); err != nil {
		return UploadDocumentoResult{}, err
	}
	ticket, err := modkit.NewUpload(ctx, s.store, s.bucket, documentoPrefix(acolhimento), filename, contentType, documentoTypes, s.uploadExpiry)
	if err != nil {
		return UploadDocumentoResult{}, err
	}
	d, err := domain.NovoDocumentoCasaAbrigo(acolhimento, tipo, filename, ticket.ObjectKey, ticket.Headers["Content-Type"])
	if err != nil {
		return UploadDocumentoResult{}, err
	}
	d.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.InsertDocumento(ctx, tx, d, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.documento_upload_iniciado", "sempras_casa_abrigo_documento", d.ID.String(), nil,
			map[string]any{"acolhimento_id": acolhimento, "tipo": tipo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return UploadDocumentoResult{}, err
	}
	return UploadDocumentoResult{Documento: d, Upload: ticket}, nil
}

// ConfirmUploadDocumento confere o objeto enviado (tamanho e tipo reais —
// o que o navegador declarou não é confiável) e publica o documento.
func (s *Service) ConfirmUploadDocumento(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.DocumentoCasaAbrigo, error) {
	d, err := s.repo.GetDocumento(ctx, s.pool, id)
	if err != nil {
		return domain.DocumentoCasaAbrigo{}, err
	}
	if _, err := s.acolhimento(ctx, s.pool, identity, d.AcolhimentoID, false); err != nil {
		return domain.DocumentoCasaAbrigo{}, err
	}
	if d.Status == domain.DocumentoStatusReady {
		return d, nil
	}
	info, err := modkit.ConfirmUpload(ctx, s.store, s.bucket, d.ObjectKey, documentoPrefix(d.AcolhimentoID), s.maxDocBytes, documentoTypes)
	if err != nil {
		return domain.DocumentoCasaAbrigo{}, err
	}
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.UpdateDocumentoStatus(ctx, tx, id, domain.DocumentoStatusReady, info.Size, info.ContentType); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.documento_confirmado", "sempras_casa_abrigo_documento", id.String(), nil,
			map[string]any{"acolhimento_id": d.AcolhimentoID, "size_bytes": info.Size})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.DocumentoCasaAbrigo{}, err
	}
	d.Status, d.SizeBytes, d.ContentType = domain.DocumentoStatusReady, info.Size, info.ContentType
	return d, nil
}

// DownloadURLDocumento devolve uma URL pré-assinada de leitura (curta).
func (s *Service) DownloadURLDocumento(ctx context.Context, identity auth.Identity, id uuid.UUID) (string, domain.DocumentoCasaAbrigo, error) {
	d, err := s.repo.GetDocumento(ctx, s.pool, id)
	if err != nil {
		return "", domain.DocumentoCasaAbrigo{}, err
	}
	if d.Status != domain.DocumentoStatusReady {
		return "", domain.DocumentoCasaAbrigo{}, domain.ErrNotFound
	}
	if _, err := s.acolhimento(ctx, s.pool, identity, d.AcolhimentoID, false); err != nil {
		return "", domain.DocumentoCasaAbrigo{}, err
	}
	url, err := s.store.PresignedGetURL(ctx, s.bucket, d.ObjectKey, 5*time.Minute)
	if err != nil {
		return "", domain.DocumentoCasaAbrigo{}, apperrors.DependencyUnavailable("armazenamento indisponível").WithCause(err)
	}
	e := audit.Meta(ctx, "sempras.casa_abrigo.documento_baixado", "sempras_casa_abrigo_documento", id.String(), nil, nil)
	_ = audit.NewWriter(s.pool).Record(ctx, e)
	return url, d, nil
}

// RemoverDocumento remove o metadado e, fora da transação, o objeto no storage.
func (s *Service) RemoverDocumento(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	var key string
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		d, err := s.repo.GetDocumento(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := s.acolhimento(ctx, tx, identity, d.AcolhimentoID, false); err != nil {
			return err
		}
		key = d.ObjectKey
		if err := s.repo.DeleteDocumento(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.documento_removido", "sempras_casa_abrigo_documento", id.String(), nil,
			map[string]any{"acolhimento_id": d.AcolhimentoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err == nil {
		s.removeObjetos(ctx, []string{key})
	}
	return err
}

// ------------------------------------------------- painel e relatórios

// PainelCasaAbrigo é a tela inicial da unidade: os cartões, o relatório
// de institucionalizados e o monitor de prazos — leitura auditada.
func (s *Service) PainelCasaAbrigo(ctx context.Context, identity auth.Identity, unidade uuid.UUID) (domain.PainelCasaAbrigo, error) {
	var p domain.PainelCasaAbrigo
	err := s.lerAuditado(ctx, "sempras.casa_abrigo.painel_visualizado", "unidade", unidade.String(),
		func(ctx context.Context, tx pgx.Tx) (int, error) {
			if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
				return 0, err
			}
			todos, err := s.repo.AcolhidosDaUnidade(ctx, tx, unidade)
			if err != nil {
				return 0, err
			}
			metas, err := s.repo.MetasPendentesDaUnidade(ctx, tx, unidade)
			if err != nil {
				return 0, err
			}
			relatorios, err := s.repo.ListRelatorios(ctx, tx, unidade)
			if err != nil {
				return 0, err
			}
			tarefas, err := s.repo.ListTarefas(ctx, tx, unidade)
			if err != nil {
				return 0, err
			}
			p = domain.MontarPainel(s.now(), todos, metas, relatorios, tarefas)
			return len(todos), nil
		})
	return p, err
}

// EstatisticasCasaAbrigo é o relatório estatístico do período [de, ate].
// Só números — não há nome de criança, então não é leitura auditada.
func (s *Service) EstatisticasCasaAbrigo(ctx context.Context, identity auth.Identity, unidade uuid.UUID, de, ate time.Time) (domain.EstatisticasCasaAbrigo, error) {
	if _, err := s.casaAbrigo(ctx, s.pool, identity, unidade); err != nil {
		return domain.EstatisticasCasaAbrigo{}, err
	}
	todos, err := s.repo.AcolhidosDaUnidade(ctx, s.pool, unidade)
	if err != nil {
		return domain.EstatisticasCasaAbrigo{}, err
	}
	return domain.Estatisticas(todos, de, ate)
}

// EvolucoesCasaAbrigo lista as evoluções dos acolhidos da unidade (tela de
// evolução técnica). O psicológico sai sem o texto para quem não tem o
// sigilo (ADR 030 §5) — leitura auditada.
func (s *Service) EvolucoesCasaAbrigo(ctx context.Context, identity auth.Identity, unidade uuid.UUID) ([]domain.EvolucaoAcolhido, error) {
	var out []domain.EvolucaoAcolhido
	err := s.lerAuditado(ctx, "sempras.casa_abrigo.evolucoes_visualizadas", "unidade", unidade.String(),
		func(ctx context.Context, tx pgx.Tx) (int, error) {
			u, err := s.casaAbrigo(ctx, tx, identity, unidade)
			if err != nil {
				return 0, err
			}
			if out, err = s.repo.EvolucoesDaUnidade(ctx, tx, unidade); err != nil {
				return 0, err
			}
			if !u.PodePsicologia(identity) {
				for i, e := range out {
					if e.Tipo == domain.TipoPsicologico {
						out[i].Registro = e.Ocultar()
					}
				}
			}
			return len(out), nil
		})
	return out, err
}

// ---------------------------------------------------------------- diário

// RegistrarAnotacaoDiario inclui uma anotação no diário de bordo.
func (s *Service) RegistrarAnotacaoDiario(ctx context.Context, identity auth.Identity, unidade uuid.UUID, acolhimento *uuid.UUID, data time.Time, txt string) (domain.AnotacaoDiario, error) {
	a, err := domain.NovaAnotacaoDiario(unidade, acolhimento, data, txt, identity.Name)
	if err != nil {
		return domain.AnotacaoDiario{}, err
	}
	a.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
			return err
		}
		if err := s.doAcolhido(ctx, tx, acolhimento, unidade); err != nil {
			return err
		}
		if err := s.repo.InsertAnotacaoDiario(ctx, tx, a, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.diario_anotado", "sempras_casa_abrigo_diario", a.ID.String(), nil,
			map[string]any{"unidade_id": unidade, "acolhimento_id": acolhimento})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.AnotacaoDiario{}, err
	}
	return a, nil
}

// DiarioCasaAbrigo lista as anotações de um mês ("AAAA-MM") — leitura auditada.
func (s *Service) DiarioCasaAbrigo(ctx context.Context, identity auth.Identity, unidade uuid.UUID, mes string) ([]domain.AnotacaoDiario, error) {
	inicio, fim, err := domain.CompetenciaMes(mes)
	if err != nil {
		return nil, err
	}
	var out []domain.AnotacaoDiario
	err = s.lerAuditado(ctx, "sempras.casa_abrigo.diario_visualizado", "unidade", unidade.String(),
		func(ctx context.Context, tx pgx.Tx) (int, error) {
			if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
				return 0, err
			}
			var err error
			out, err = s.repo.ListDiario(ctx, tx, unidade, inicio, fim)
			return len(out), err
		})
	return out, err
}

// ------------------------------------------------------------ relatórios

// RegistrarRelatorio cria um relatório escrito.
func (s *Service) RegistrarRelatorio(ctx context.Context, identity auth.Identity, unidade uuid.UUID, d domain.RelatorioDados) (domain.RelatorioCasaAbrigo, error) {
	r, err := domain.NovoRelatorio(unidade, d)
	if err != nil {
		return domain.RelatorioCasaAbrigo{}, err
	}
	r.CreatedAt = s.now()
	r.AtualizadoEm = r.CreatedAt
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
			return err
		}
		if err := s.doAcolhido(ctx, tx, r.AcolhimentoID, unidade); err != nil {
			return err
		}
		if err := s.repo.InsertRelatorio(ctx, tx, r, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.relatorio_registrado", "sempras_casa_abrigo_relatorio", r.ID.String(), nil,
			map[string]any{"tipo": r.Tipo, "status": r.Status})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.RelatorioCasaAbrigo{}, err
	}
	return r, nil
}

// AtualizarRelatorio regrava o relatório (o entregue não muda mais, R8).
func (s *Service) AtualizarRelatorio(ctx context.Context, identity auth.Identity, id uuid.UUID, d domain.RelatorioDados) (domain.RelatorioCasaAbrigo, error) {
	var r domain.RelatorioCasaAbrigo
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if r, err = s.repo.GetRelatorio(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.casaAbrigo(ctx, tx, identity, r.UnidadeID); err != nil {
			return err
		}
		antes := r.Status
		if err := r.Atualizar(d); err != nil {
			return err
		}
		if err := s.doAcolhido(ctx, tx, r.AcolhimentoID, r.UnidadeID); err != nil {
			return err
		}
		r.AtualizadoEm = s.now()
		if err := s.repo.UpdateRelatorio(ctx, tx, r); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.relatorio_atualizado", "sempras_casa_abrigo_relatorio", id.String(),
			map[string]any{"status": antes}, map[string]any{"status": r.Status})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.RelatorioCasaAbrigo{}, err
	}
	return r, nil
}

// RemoverRelatorio exclui um relatório ainda não entregue (R8).
func (s *Service) RemoverRelatorio(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		r, err := s.repo.GetRelatorio(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if _, err := s.casaAbrigo(ctx, tx, identity, r.UnidadeID); err != nil {
			return err
		}
		if err := r.PodeExcluir(); err != nil {
			return err
		}
		if err := s.repo.DeleteRelatorio(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.relatorio_removido", "sempras_casa_abrigo_relatorio", id.String(), nil,
			map[string]any{"tipo": r.Tipo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// RelatoriosCasaAbrigo lista os relatórios escritos — leitura auditada.
func (s *Service) RelatoriosCasaAbrigo(ctx context.Context, identity auth.Identity, unidade uuid.UUID) ([]domain.RelatorioCasaAbrigo, error) {
	var out []domain.RelatorioCasaAbrigo
	err := s.lerAuditado(ctx, "sempras.casa_abrigo.relatorios_visualizados", "unidade", unidade.String(),
		func(ctx context.Context, tx pgx.Tx) (int, error) {
			if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
				return 0, err
			}
			var err error
			out, err = s.repo.ListRelatorios(ctx, tx, unidade)
			return len(out), err
		})
	return out, err
}

// --------------------------------------------------------------- tarefas

// CriarTarefa cria uma tarefa no Kanban da equipe.
func (s *Service) CriarTarefa(ctx context.Context, identity auth.Identity, unidade uuid.UUID, d domain.TarefaDados) (domain.TarefaCasaAbrigo, error) {
	t, err := domain.NovaTarefa(unidade, d)
	if err != nil {
		return domain.TarefaCasaAbrigo{}, err
	}
	t.CreatedAt = s.now()
	t.AtualizadoEm = t.CreatedAt
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.casaAbrigo(ctx, tx, identity, unidade); err != nil {
			return err
		}
		if err := s.repo.InsertTarefa(ctx, tx, t, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.tarefa_criada", "sempras_casa_abrigo_tarefa", t.ID.String(), nil,
			map[string]any{"status": t.Status, "prioridade": t.Prioridade})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.TarefaCasaAbrigo{}, err
	}
	return t, nil
}

// AtualizarTarefa regrava a tarefa (inclusive a coluna, ao arrastar).
func (s *Service) AtualizarTarefa(ctx context.Context, identity auth.Identity, id uuid.UUID, d domain.TarefaDados) (domain.TarefaCasaAbrigo, error) {
	var t domain.TarefaCasaAbrigo
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if t, err = s.repo.GetTarefa(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.casaAbrigo(ctx, tx, identity, t.UnidadeID); err != nil {
			return err
		}
		antes := t.Status
		if err := t.Atualizar(d); err != nil {
			return err
		}
		t.AtualizadoEm = s.now()
		if err := s.repo.UpdateTarefa(ctx, tx, t); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.tarefa_atualizada", "sempras_casa_abrigo_tarefa", id.String(),
			map[string]any{"status": antes}, map[string]any{"status": t.Status})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.TarefaCasaAbrigo{}, err
	}
	return t, nil
}

// RemoverTarefa exclui uma tarefa.
func (s *Service) RemoverTarefa(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		t, err := s.repo.GetTarefa(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if _, err := s.casaAbrigo(ctx, tx, identity, t.UnidadeID); err != nil {
			return err
		}
		if err := s.repo.DeleteTarefa(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_abrigo.tarefa_removida", "sempras_casa_abrigo_tarefa", id.String(), nil,
			map[string]any{"titulo": t.Titulo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// TarefasCasaAbrigo lista o quadro da equipe (sem dado de criança).
func (s *Service) TarefasCasaAbrigo(ctx context.Context, identity auth.Identity, unidade uuid.UUID) ([]domain.TarefaCasaAbrigo, error) {
	if _, err := s.casaAbrigo(ctx, s.pool, identity, unidade); err != nil {
		return nil, err
	}
	return s.repo.ListTarefas(ctx, s.pool, unidade)
}
