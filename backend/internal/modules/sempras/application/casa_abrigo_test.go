package application_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/application"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/transport"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
)

// relatorio cria um relatório escrito na Casa Abrigo (fora da transação sob teste).
func (c cenario) relatorio(t *testing.T, acolhimento *uuid.UUID) domain.RelatorioCasaAbrigo {
	t.Helper()
	r, err := c.limpo().RegistrarRelatorio(context.Background(), c.identity, c.unAbrigo, domain.RelatorioDados{
		AcolhimentoID: acolhimento, Tipo: "mensal", Status: "em_elaboracao", DataEntrega: time.Now().AddDate(0, 0, 5)})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// tarefa cria uma tarefa no Kanban da Casa Abrigo.
func (c cenario) tarefa(t *testing.T) domain.TarefaCasaAbrigo {
	t.Helper()
	tf, err := c.limpo().CriarTarefa(context.Background(), c.identity, c.unAbrigo, domain.TarefaDados{Titulo: "Comprar material"})
	if err != nil {
		t.Fatal(err)
	}
	return tf
}

// comPia cria um acolhimento já com o PIA gravado.
func (c cenario) comPia(t *testing.T) domain.Acolhimento {
	t.Helper()
	a := c.acolhimento(t)
	if _, err := c.limpo().SalvarPiaFicha(context.Background(), c.identity, a.ID, domain.PiaFichaDados{Escola: "EMEB"}); err != nil {
		t.Fatal(err)
	}
	return a
}

// TestCasaAbrigoFalhasPropagadas: a matriz de falhas da Casa Abrigo (ADR
// 032) — cada chamada ao repositório falha (ou envenena a transação logo
// depois) e o caso de uso propaga o erro; as leituras auditadas não saem
// sem a auditoria.
func TestCasaAbrigoFalhasPropagadas(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	id := c.identity
	agora := time.Now()
	ops := map[string]func(t *testing.T, s *application.Service) error{
		"RegistrarAcolhimento": func(t *testing.T, s *application.Service) error {
			in := acolhimentoInput()
			in.Irmaos = []application.IrmaoInput{{Nome: "Irmã"}}
			in.Diagnosticos = []application.DiagnosticoInput{{Categoria: "doenca", Diagnostico: "Asma"}}
			_, err := s.RegistrarAcolhimento(ctx, id, c.crianca(t).ID, c.unAbrigo, in)
			return err
		},
		"AtualizarAcolhimento": func(t *testing.T, s *application.Service) error {
			_, err := s.AtualizarAcolhimento(ctx, id, c.acolhimento(t).ID, acolhimentoInput().Dados, agora.Add(-48*time.Hour))
			return err
		},
		"DesligarAcolhimento": func(t *testing.T, s *application.Service) error {
			_, err := s.DesligarAcolhimento(ctx, id, c.acolhimento(t).ID, domain.Desligamento{Tipo: "adocao", Motivo: "sentença", DataSaida: agora})
			return err
		},
		"ObterAcolhimento": func(t *testing.T, s *application.Service) error {
			_, err := s.ObterAcolhimento(ctx, id, c.comPia(t).ID)
			return err
		},
		"AdicionarIrmao": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarIrmao(ctx, id, c.acolhimento(t).ID, application.IrmaoInput{Nome: "Irmão"})
			return err
		},
		"RemoverIrmao": func(t *testing.T, s *application.Service) error { return s.RemoverIrmao(ctx, id, c.irmao(t).ID) },
		"AdicionarDiagnostico": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarDiagnostico(ctx, id, c.acolhimento(t).ID, application.DiagnosticoInput{Categoria: "doenca", Diagnostico: "Asma"})
			return err
		},
		"RemoverDiagnostico": func(t *testing.T, s *application.Service) error {
			return s.RemoverDiagnostico(ctx, id, c.diagnosticoAcolhido(t).ID)
		},
		"AdicionarVisitante": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarVisitante(ctx, id, c.acolhimento(t).ID, domain.VisitanteDados{NomeVisitante: "Avó", DataVisita: agora})
			return err
		},
		"RegistrarVisita": func(t *testing.T, s *application.Service) error {
			ac := c.acolhimento(t).ID
			_, err := s.RegistrarVisita(ctx, id, c.unAbrigo, domain.VisitanteDados{AcolhimentoID: &ac, NomeVisitante: "Avó", DataVisita: agora})
			return err
		},
		"RegistrarSaidaVisitante": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarSaidaVisitante(ctx, id, c.visitante(t).ID, "10:00")
			return err
		},
		"RemoverVisitante": func(t *testing.T, s *application.Service) error {
			return s.RemoverVisitante(ctx, id, c.visitante(t).ID)
		},
		"VisitantesDaUnidade": func(_ *testing.T, s *application.Service) error {
			_, err := s.VisitantesDaUnidade(ctx, id, c.unAbrigo)
			return err
		},
		"RegistrarTermo": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarTermo(ctx, id, c.acolhimento(t).ID, "advertencia", "", "", "atraso", agora)
			return err
		},
		"RemoverTermo": func(t *testing.T, s *application.Service) error { return s.RemoverTermo(ctx, id, c.termo(t).ID) },
		"TermosDaUnidade": func(_ *testing.T, s *application.Service) error {
			_, err := s.TermosDaUnidade(ctx, id, c.unAbrigo)
			return err
		},
		"SalvarPiaFicha": func(t *testing.T, s *application.Service) error {
			_, err := s.SalvarPiaFicha(ctx, id, c.acolhimento(t).ID, domain.PiaFichaDados{Escola: "EMEB"})
			return err
		},
		"AdicionarPiaMeta": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarPiaMeta(ctx, id, c.acolhimento(t).ID, domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: "matrícula"})
			return err
		},
		"EditarPiaMeta": func(t *testing.T, s *application.Service) error {
			_, err := s.EditarPiaMeta(ctx, id, c.piaMeta(t).ID, domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: "matrícula escolar"})
			return err
		},
		"ConcluirPiaMeta": func(t *testing.T, s *application.Service) error {
			_, err := s.ConcluirPiaMeta(ctx, id, c.piaMeta(t).ID, true)
			return err
		},
		"RemoverPiaMeta": func(t *testing.T, s *application.Service) error { return s.RemoverPiaMeta(ctx, id, c.piaMeta(t).ID) },
		"StartUploadDocumento": func(t *testing.T, s *application.Service) error {
			_, err := s.StartUploadDocumento(ctx, id, c.acolhimento(t).ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
			return err
		},
		"ConfirmUploadDocumento": func(t *testing.T, s *application.Service) error {
			res, err := c.limpo().StartUploadDocumento(ctx, c.identity, c.acolhimento(t).ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.store.Put(ctx, bucket, res.Upload.ObjectKey, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
				t.Fatal(err)
			}
			_, err = s.ConfirmUploadDocumento(ctx, id, res.Documento.ID)
			return err
		},
		"DownloadURLDocumento": func(t *testing.T, s *application.Service) error {
			_, _, err := s.DownloadURLDocumento(ctx, id, c.documento(t).ID)
			return err
		},
		"RemoverDocumento": func(t *testing.T, s *application.Service) error {
			return s.RemoverDocumento(ctx, id, c.documento(t).ID)
		},
		"PainelCasaAbrigo": func(_ *testing.T, s *application.Service) error {
			_, err := s.PainelCasaAbrigo(ctx, id, c.unAbrigo)
			return err
		},
		"EstatisticasCasaAbrigo": func(_ *testing.T, s *application.Service) error {
			_, err := s.EstatisticasCasaAbrigo(ctx, id, c.unAbrigo, agora.AddDate(0, -1, 0), agora)
			return err
		},
		"EvolucoesCasaAbrigo": func(_ *testing.T, s *application.Service) error {
			_, err := s.EvolucoesCasaAbrigo(ctx, id, c.unAbrigo)
			return err
		},
		"RegistrarAnotacaoDiario": func(t *testing.T, s *application.Service) error {
			ac := c.acolhimento(t).ID
			_, err := s.RegistrarAnotacaoDiario(ctx, id, c.unAbrigo, &ac, agora, "consulta médica")
			return err
		},
		"DiarioCasaAbrigo": func(_ *testing.T, s *application.Service) error {
			_, err := s.DiarioCasaAbrigo(ctx, id, c.unAbrigo, agora.Format("2006-01"))
			return err
		},
		"RegistrarRelatorio": func(t *testing.T, s *application.Service) error {
			ac := c.acolhimento(t).ID
			_, err := s.RegistrarRelatorio(ctx, id, c.unAbrigo, domain.RelatorioDados{AcolhimentoID: &ac, Tipo: "mensal",
				Status: "em_elaboracao", DataEntrega: agora})
			return err
		},
		"AtualizarRelatorio": func(t *testing.T, s *application.Service) error {
			_, err := s.AtualizarRelatorio(ctx, id, c.relatorio(t, nil).ID, domain.RelatorioDados{Tipo: "mensal", Status: "revisao",
				DataEntrega: agora, Conteudo: "texto"})
			return err
		},
		"RemoverRelatorio": func(t *testing.T, s *application.Service) error {
			return s.RemoverRelatorio(ctx, id, c.relatorio(t, nil).ID)
		},
		"RelatoriosCasaAbrigo": func(_ *testing.T, s *application.Service) error {
			_, err := s.RelatoriosCasaAbrigo(ctx, id, c.unAbrigo)
			return err
		},
		"CriarTarefa": func(_ *testing.T, s *application.Service) error {
			_, err := s.CriarTarefa(ctx, id, c.unAbrigo, domain.TarefaDados{Titulo: "Relatório"})
			return err
		},
		"AtualizarTarefa": func(t *testing.T, s *application.Service) error {
			_, err := s.AtualizarTarefa(ctx, id, c.tarefa(t).ID, domain.TarefaDados{Titulo: "Relatório", Status: "done"})
			return err
		},
		"RemoverTarefa": func(t *testing.T, s *application.Service) error { return s.RemoverTarefa(ctx, id, c.tarefa(t).ID) },
		"TarefasCasaAbrigo": func(_ *testing.T, s *application.Service) error {
			_, err := s.TarefasCasaAbrigo(ctx, id, c.unAbrigo)
			return err
		},
	}
	for name, op := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := op(t, c.svc(probe, publisher{})); err != nil {
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
				if err := op(t, c.svc(r, publisher{})); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
}

// TestCasaAbrigoRegras: as regras do ADR 032 que não dependem de falha do banco.
func TestCasaAbrigoRegras(t *testing.T) {
	c := novoCenario(t)
	s := c.limpo()
	ctx := context.Background()
	agora := time.Now()
	recepcao := c.so("sempras:recepcao")
	tecnico := c.so("sempras:atendimento") // sem o sigilo da psicologia
	invalido := func(nome string, err error) {
		t.Helper()
		var v *domain.ValidationError
		if !errors.As(err, &v) {
			t.Errorf("%s: esperava erro de validação, veio %v", nome, err)
		}
	}
	proibido := func(nome string, err error) {
		t.Helper()
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("%s: esperava sem permissão, veio %v", nome, err)
		}
	}

	// R2: só menores de 18; irmão e diagnóstico do assistente validados
	_, err := s.RegistrarAcolhimento(ctx, c.identity, c.cadastrar(t).ID, c.unAbrigo, acolhimentoInput())
	invalido("adulto", err)
	in := acolhimentoInput()
	in.Irmaos = []application.IrmaoInput{{Nome: "I"}}
	_, err = s.RegistrarAcolhimento(ctx, c.identity, c.crianca(t).ID, c.unAbrigo, in)
	invalido("irmão do assistente", err)
	in = acolhimentoInput()
	in.Diagnosticos = []application.DiagnosticoInput{{Categoria: "x", Diagnostico: "Asma"}}
	_, err = s.RegistrarAcolhimento(ctx, c.identity, c.crianca(t).ID, c.unAbrigo, in)
	invalido("diagnóstico do assistente", err)
	_, err = s.RegistrarAcolhimento(ctx, recepcao, c.crianca(t).ID, c.unAbrigo, acolhimentoInput())
	proibido("recepção acolhe", err)

	// R3 e R6: um ativo por criança; irmãos e diagnósticos na mesma transação
	crianca := c.crianca(t).ID
	in = acolhimentoInput()
	in.Irmaos = []application.IrmaoInput{{Nome: "Irmã da Bruna", EstaAcolhido: true}}
	in.Diagnosticos = []application.DiagnosticoInput{{Categoria: "deficiencia", Diagnostico: "Deficiência visual"}}
	a, err := s.RegistrarAcolhimento(ctx, c.identity, crianca, c.unAbrigo, in)
	if err != nil || len(a.Irmaos) != 1 || len(a.Diagnosticos) != 1 {
		t.Fatalf("acolhimento com irmã e diagnóstico: %+v %v", a, err)
	}
	if _, err := s.RegistrarAcolhimento(ctx, c.identity, crianca, c.unAbrigo, acolhimentoInput()); !errors.Is(err, domain.ErrAcolhimentoAtivo) {
		t.Errorf("segundo acolhimento ativo: %v", err)
	}
	det, err := s.ObterAcolhimento(ctx, c.identity, a.ID)
	if err != nil || det.Cidadao.ID != crianca || det.Pia != nil || len(det.Irmaos) != 1 {
		t.Errorf("detalhe: %+v %v", det, err)
	}
	if _, err := s.ObterAcolhimento(ctx, recepcao, a.ID); err == nil {
		t.Error("recepção não lê o acolhimento")
	}
	if _, err := s.AtualizarAcolhimento(ctx, c.identity, a.ID, acolhimentoInput().Dados, agora.AddDate(0, 0, 3)); err == nil {
		t.Error("edição com entrada futura")
	}

	// visitas: da unidade, com acolhido opcional (que tem de ser dela)
	var ent uuid.UUID
	if err := c.pool.QueryRow(ctx, `SELECT entidade_id FROM unidades WHERE id = $1`, c.unAbrigo).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	outraCasa := novaUnidade(t, c.pool, ent, domain.UnidadeCasaAbrigo)
	gestor := auth.Identity{UserID: c.identity.UserID, Name: "Gestor", Scopes: []auth.Scope{{EntidadeID: &ent, Permissions: []string{"sempras:*"}}}}
	gestor.Permissions = auth.EffectivePermissions(gestor.Scopes)
	_, err = s.RegistrarVisita(ctx, gestor, outraCasa, domain.VisitanteDados{AcolhimentoID: &a.ID, NomeVisitante: "Avó", DataVisita: agora})
	invalido("visita a acolhido de outra unidade", err)
	ninguem := uuid.New()
	if _, err := s.RegistrarVisita(ctx, c.identity, c.unAbrigo, domain.VisitanteDados{AcolhimentoID: &ninguem, NomeVisitante: "Avó",
		DataVisita: agora}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("acolhido inexistente: %v", err)
	}
	_, err = s.RegistrarVisita(ctx, c.identity, c.unAbrigo, domain.VisitanteDados{TipoVisita: "festa", NomeVisitante: "Avó", DataVisita: agora})
	invalido("tipo de visita", err)
	_, err = s.RegistrarVisita(ctx, recepcao, c.unAbrigo, domain.VisitanteDados{TipoVisita: "doacao", NomeVisitante: "Igreja", DataVisita: agora})
	proibido("recepção registra visita", err)
	doacao, err := s.RegistrarVisita(ctx, c.identity, c.unAbrigo, domain.VisitanteDados{TipoVisita: "doacao", NomeVisitante: "Igreja",
		DataVisita: agora, HoraEntrada: "09:00"})
	if err != nil || doacao.AcolhimentoID != nil {
		t.Fatalf("doação: %+v %v", doacao, err)
	}
	_, err = s.RegistrarSaidaVisitante(ctx, c.identity, doacao.ID, "08:00")
	invalido("saída antes da entrada", err)
	if _, err := s.RegistrarSaidaVisitante(ctx, c.identity, doacao.ID, "10:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RegistrarSaidaVisitante(ctx, c.identity, doacao.ID, "11:00"); !errors.Is(err, domain.ErrVisitaEncerrada) {
		t.Errorf("saída de novo: %v", err)
	}
	if _, err := s.AdicionarVisitante(ctx, c.identity, a.ID, domain.VisitanteDados{NomeVisitante: "A", DataVisita: agora}); err == nil {
		t.Error("visitante pela ficha com nome curto")
	}
	if l, err := s.VisitantesDaUnidade(ctx, c.identity, c.unAbrigo); err != nil || len(l) != 1 {
		t.Errorf("visitas da unidade: %+v %v", l, err)
	}
	_, err = s.VisitantesDaUnidade(ctx, recepcao, c.unAbrigo)
	proibido("recepção lê visitas", err)
	_, err = s.TermosDaUnidade(ctx, recepcao, c.unAbrigo)
	proibido("recepção lê termos", err)

	// diagnóstico, termo e documentos
	_, err = s.AdicionarDiagnostico(ctx, c.identity, a.ID, application.DiagnosticoInput{Categoria: "x", Diagnostico: "Asma"})
	invalido("diagnóstico", err)
	_, err = s.RegistrarTermo(ctx, c.identity, a.ID, "x", "", "", "", agora)
	invalido("tipo de termo", err)
	_, err = s.StartUploadDocumento(ctx, c.identity, a.ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 200<<20)
	invalido("documento acima do limite", err)
	_, err = s.StartUploadDocumento(ctx, recepcao, a.ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
	proibido("recepção envia documento", err)
	up, err := s.StartUploadDocumento(ctx, c.identity, a.ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	// o tipo real é conferido no armazenamento: texto não passa por PDF
	if err := c.store.Put(ctx, bucket, up.Upload.ObjectKey, strings.NewReader("não é pdf"), 9, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmUploadDocumento(ctx, c.identity, up.Documento.ID); err == nil {
		t.Error("arquivo que não é PDF deveria ser recusado")
	}
	if _, err := s.ConfirmUploadDocumento(ctx, recepcao, up.Documento.ID); err == nil {
		t.Error("recepção não confirma documento")
	}
	doc := c.documento(t)
	if d, err := s.ConfirmUploadDocumento(ctx, c.identity, doc.ID); err != nil || d.Status != domain.DocumentoStatusReady {
		t.Errorf("confirmar de novo é idempotente: %+v %v", d, err)
	}

	// PIA e metas
	_, err = s.AdicionarPiaMeta(ctx, c.identity, a.ID, domain.PiaMetaDados{Tipo: "x", Descricao: "matrícula"})
	invalido("meta inválida", err)
	meta, err := s.AdicionarPiaMeta(ctx, c.identity, a.ID, domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: "matrícula"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.EditarPiaMeta(ctx, c.identity, meta.ID, domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: "x"})
	invalido("edição da meta", err)
	if m, err := s.EditarPiaMeta(ctx, c.identity, meta.ID, domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: "matrícula",
		Esportes: "natação"}); err != nil || m.Esportes != "natação" {
		t.Errorf("meta editada: %+v %v", m, err)
	}
	if p, err := s.SalvarPiaFicha(ctx, c.identity, a.ID, domain.PiaFichaDados{Escola: "EMEB"}); err != nil || p.AtualizadoPor == nil {
		t.Errorf("PIA: %+v %v", p, err)
	}

	// painel, estatísticas e evoluções (o psicológico fica sem texto para quem não tem o sigilo)
	// "a" e o acolhimento criado por c.documento
	if p, err := s.PainelCasaAbrigo(ctx, c.identity, c.unAbrigo); err != nil || p.Ativos != 2 {
		t.Errorf("painel: %+v %v", p, err)
	}
	_, err = s.PainelCasaAbrigo(ctx, recepcao, c.unAbrigo)
	proibido("recepção vê o painel", err)
	_, err = s.EstatisticasCasaAbrigo(ctx, c.identity, c.unAbrigo, agora, agora.AddDate(0, 0, -1))
	invalido("período invertido", err)
	_, err = s.EstatisticasCasaAbrigo(ctx, recepcao, c.unAbrigo, agora, agora)
	proibido("recepção vê estatísticas", err)
	if _, err := s.EstatisticasCasaAbrigo(ctx, c.identity, c.unidade, agora, agora); !errors.Is(err, domain.ErrServicoIndisponivel) {
		t.Errorf("estatísticas da Casa Abrigo num CRAS: %v", err)
	}
	for _, tipo := range []string{domain.TipoSocial, domain.TipoPsicologico} {
		if _, err := s.RegistrarEvolucao(ctx, c.identity, crianca, application.EvolucaoInput{UnidadeID: c.unAbrigo, Tipo: tipo,
			Texto: "evolução " + tipo}); err != nil {
			t.Fatal(err)
		}
	}
	evo, err := s.EvolucoesCasaAbrigo(ctx, tecnico, c.unAbrigo)
	if err != nil || len(evo) != 2 {
		t.Fatalf("evoluções: %+v %v", evo, err)
	}
	for _, e := range evo {
		if (e.Tipo == domain.TipoPsicologico) != (e.Texto == "") || e.AcolhidoNome == "" {
			t.Errorf("sigilo psicológico na lista: %+v", e)
		}
	}
	if evo, _ := s.EvolucoesCasaAbrigo(ctx, c.identity, c.unAbrigo); evo[0].Texto == "" || evo[1].Texto == "" {
		t.Errorf("com o sigilo, a psicologia lê: %+v", evo)
	}
	_, err = s.EvolucoesCasaAbrigo(ctx, recepcao, c.unAbrigo)
	proibido("recepção lê evoluções", err)

	// diário
	_, err = s.RegistrarAnotacaoDiario(ctx, c.identity, c.unAbrigo, nil, agora, "ok")
	invalido("anotação curta", err)
	_, err = s.RegistrarAnotacaoDiario(ctx, recepcao, c.unAbrigo, nil, agora, "rotina da casa")
	proibido("recepção anota", err)
	_, err = s.RegistrarAnotacaoDiario(ctx, gestor, outraCasa, &a.ID, agora, "acolhido de outra casa")
	invalido("anotação de acolhido de outra unidade", err)
	if an, err := s.RegistrarAnotacaoDiario(ctx, c.identity, c.unAbrigo, nil, agora, "rotina da casa"); err != nil || an.AutorNome != "Profissional" {
		t.Errorf("anotação geral: %+v %v", an, err)
	}
	_, err = s.DiarioCasaAbrigo(ctx, c.identity, c.unAbrigo, "outubro")
	invalido("mês do diário", err)
	_, err = s.DiarioCasaAbrigo(ctx, recepcao, c.unAbrigo, agora.Format("2006-01"))
	proibido("recepção lê o diário", err)

	// relatórios (R8)
	_, err = s.RegistrarRelatorio(ctx, c.identity, c.unAbrigo, domain.RelatorioDados{Tipo: "semanal", Status: "revisao", DataEntrega: agora})
	invalido("tipo de relatório", err)
	_, err = s.RegistrarRelatorio(ctx, recepcao, c.unAbrigo, domain.RelatorioDados{Tipo: "mensal", Status: "revisao", DataEntrega: agora})
	proibido("recepção cria relatório", err)
	_, err = s.RegistrarRelatorio(ctx, gestor, outraCasa, domain.RelatorioDados{AcolhimentoID: &a.ID, Tipo: "mensal", Status: "revisao", DataEntrega: agora})
	invalido("relatório de acolhido de outra unidade", err)
	rel := c.relatorio(t, &a.ID)
	_, err = s.AtualizarRelatorio(ctx, c.identity, rel.ID, domain.RelatorioDados{Tipo: "mensal", Status: "x", DataEntrega: agora})
	invalido("etapa inválida", err)
	daOutra, err := s.RegistrarAcolhimento(ctx, gestor, c.crianca(t).ID, outraCasa, acolhimentoInput())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AtualizarRelatorio(ctx, c.identity, rel.ID, domain.RelatorioDados{AcolhimentoID: &daOutra.ID, Tipo: "mensal",
		Status: "revisao", DataEntrega: agora})
	invalido("relatório trocado para acolhido de outra unidade", err)
	entregue, err := s.AtualizarRelatorio(ctx, c.identity, rel.ID, domain.RelatorioDados{AcolhimentoID: &a.ID, Tipo: "mensal",
		Status: domain.RelatorioEntregue, DataEntrega: agora, Conteudo: "texto final"})
	if err != nil || entregue.Status != domain.RelatorioEntregue {
		t.Fatalf("entregar: %+v %v", entregue, err)
	}
	if _, err := s.AtualizarRelatorio(ctx, c.identity, rel.ID, domain.RelatorioDados{Tipo: "mensal", Status: "revisao",
		DataEntrega: agora}); !errors.Is(err, domain.ErrRelatorioEntregue) {
		t.Errorf("editar entregue: %v", err)
	}
	if err := s.RemoverRelatorio(ctx, c.identity, rel.ID); !errors.Is(err, domain.ErrRelatorioEntregue) {
		t.Errorf("excluir entregue: %v", err)
	}
	_, err = s.RelatoriosCasaAbrigo(ctx, recepcao, c.unAbrigo)
	proibido("recepção lê relatórios", err)

	// tarefas
	_, err = s.CriarTarefa(ctx, c.identity, c.unAbrigo, domain.TarefaDados{Titulo: "R"})
	invalido("título da tarefa", err)
	if _, err := s.CriarTarefa(ctx, c.identity, c.unidade, domain.TarefaDados{Titulo: "Relatório"}); !errors.Is(err, domain.ErrServicoIndisponivel) {
		t.Errorf("Kanban da Casa Abrigo num CRAS: %v", err)
	}
	tf := c.tarefa(t)
	_, err = s.AtualizarTarefa(ctx, c.identity, tf.ID, domain.TarefaDados{Titulo: "Relatório", Prioridade: "urgente"})
	invalido("prioridade", err)
	_, err = s.TarefasCasaAbrigo(ctx, recepcao, c.unAbrigo)
	proibido("recepção vê o Kanban", err)
	if l, err := s.TarefasCasaAbrigo(ctx, c.identity, c.unAbrigo); err != nil || len(l) != 1 {
		t.Errorf("Kanban: %+v %v", l, err)
	}

	// desligamento (R5)
	_, err = s.DesligarAcolhimento(ctx, c.identity, a.ID, domain.Desligamento{Tipo: "fuga", Motivo: "evadiu", DataSaida: agora})
	invalido("tipo de desligamento", err)
	if d, err := s.DesligarAcolhimento(ctx, c.identity, a.ID, domain.Desligamento{Tipo: "evasao", Motivo: "evadiu à noite",
		DataSaida: agora}); err != nil || d.Status != domain.AcolhimentoStatusDesligado {
		t.Fatalf("desligar: %+v %v", d, err)
	}
	if _, err := s.RegistrarAcolhimento(ctx, c.identity, crianca, c.unAbrigo, acolhimentoInput()); err != nil {
		t.Errorf("desligada, a criança pode ser acolhida de novo: %v", err)
	}
}

// TestCasaAbrigoHandlers percorre as rotas da Casa Abrigo com sucesso e
// confere 4xx na entrada ruim e 500 (sem vazar o erro) com o banco fora.
func TestCasaAbrigoHandlers(t *testing.T) {
	c := novoCenario(t)
	a := novaAPI(t, c)
	un := c.unAbrigo.String()
	base := "/sempras/unidades/" + un + "/casa-abrigo"
	hoje := time.Now().Format("2006-01-02")
	ontem := time.Now().AddDate(0, 0, -1).Format(time.RFC3339)
	assistente := `"sexo":"feminino","motivos":["negligencia","outro"],"outro_motivo":"detalhe","responsavel_nome":"Tia",` +
		`"situacao_juridica":"processo_guarda","cna":"nao_sei","laudo_medico":"sim","laudo_medico_descricao":"TEA","data_entrada":"` + ontem + `"`

	ac := a.id("POST", "/sempras/cidadaos/"+c.crianca(t).ID.String()+"/casa-abrigo", `{"unidade_id":"`+un+`",`+assistente+
		`,"irmaos":[{"nome":"Irmã"}],"diagnosticos":[{"categoria":"doenca","diagnostico":"Asma"}]}`)
	a.do(200, "GET", "/sempras/casa-abrigo/"+ac, "")
	a.do(200, "PUT", "/sempras/casa-abrigo/"+ac, `{`+assistente+`,"observacoes_gerais":"editado"}`)
	irmao := a.id("POST", "/sempras/casa-abrigo/"+ac+"/irmaos", `{"nome":"Irmão"}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-irmaos/"+irmao, "")
	diag := a.id("POST", "/sempras/casa-abrigo/"+ac+"/diagnosticos", `{"categoria":"transtorno","diagnostico":"Síndrome de Down"}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-diagnosticos/"+diag, "")
	vis := a.id("POST", "/sempras/casa-abrigo/"+ac+"/visitantes", `{"nome_visitante":"Avó","data_visita":"`+ontem+`","hora_entrada":"14:00"}`)
	a.do(200, "POST", "/sempras/casa-abrigo-visitantes/"+vis+"/saida", `{"hora_saida":"15:00"}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-visitantes/"+vis, "")
	a.id("POST", base+"/visitantes", `{"tipo_visita":"voluntariado","nome_visitante":"Faculdade","data_visita":"`+ontem+`"}`)
	a.do(200, "GET", base+"/visitantes", "")
	termo := a.id("POST", "/sempras/casa-abrigo/"+ac+"/termos", `{"tipo_termo":"passeio","data_termo":"`+ontem+`"}`)
	a.do(200, "GET", base+"/termos", "")
	a.do(200, "DELETE", "/sempras/casa-abrigo-termos/"+termo, "")
	a.do(200, "PUT", "/sempras/casa-abrigo/"+ac+"/pia", `{"escola":"EMEB","periodo_escolar":"vespertino","doc_nis":true}`)
	meta := a.id("POST", "/sempras/casa-abrigo/"+ac+"/pia-metas", `{"tipo":"plano","descricao":"retomar a escola","esportes":"futebol"}`)
	a.do(200, "PUT", "/sempras/casa-abrigo-pia-metas/"+meta, `{"tipo":"plano","descricao":"retomar a escola","lazer":"praça"}`)
	a.do(200, "POST", "/sempras/casa-abrigo-pia-metas/"+meta+"/concluir", `{"concluida":true}`)
	a.do(200, "DELETE", "/sempras/casa-abrigo-pia-metas/"+meta, "")
	up := a.do(201, "POST", "/sempras/casa-abrigo/"+ac+"/documentos/uploads",
		`{"tipo":"responsavel","filename":"doc.pdf","content_type":"application/pdf","size":8}`)
	doc := enviar(t, c, up)
	a.do(200, "POST", "/sempras/casa-abrigo-documentos/"+doc+"/confirm", "")
	a.do(200, "GET", "/sempras/casa-abrigo-documentos/"+doc+"/download", "")
	a.do(200, "DELETE", "/sempras/casa-abrigo-documentos/"+doc, "")
	a.do(200, "GET", base, "")
	a.do(200, "GET", base+"/estatisticas?de="+hoje+"&ate="+hoje, "")
	a.do(422, "GET", base+"/estatisticas?de=ontem&ate="+hoje, "")
	a.do(422, "GET", base+"/estatisticas?de="+hoje+"&ate=amanha", "")
	a.do(200, "GET", base+"/evolucoes", "")
	a.id("POST", base+"/diario", `{"acolhimento_id":"`+ac+`","data_referencia":"`+ontem+`","texto":"consulta médica"}`)
	a.do(200, "GET", base+"/diario", "")
	a.do(200, "GET", base+"/diario?mes="+hoje[:7], "")
	rel := a.id("POST", base+"/relatorios", `{"tipo":"psicossocial","status":"em_elaboracao","data_entrega":"`+ontem+`"}`)
	a.do(200, "PUT", "/sempras/casa-abrigo-relatorios/"+rel, `{"tipo":"psicossocial","status":"revisao","data_entrega":"`+ontem+`","conteudo":"texto"}`)
	a.do(200, "GET", base+"/relatorios", "")
	a.do(200, "DELETE", "/sempras/casa-abrigo-relatorios/"+rel, "")
	tf := a.id("POST", base+"/tarefas", `{"titulo":"Comprar material","prioridade":"alta"}`)
	a.do(200, "PUT", "/sempras/casa-abrigo-tarefas/"+tf, `{"titulo":"Comprar material","status":"doing","ordem":1}`)
	a.do(200, "GET", base+"/tarefas", "")
	a.do(200, "DELETE", "/sempras/casa-abrigo-tarefas/"+tf, "")
	a.do(200, "POST", "/sempras/casa-abrigo/"+ac+"/desligar", `{"tipo":"reintegracao_familia_natural","motivo":"retorno à família","data_saida":"`+ontem+`"}`)

	// com o banco fora: 500 sem vazar; id ou corpo ruins: 400
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(c.svc(down, publisher{}), testLogger())
	r := chi.NewRouter()
	identidade := c.global()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), identidade)))
		})
	})
	h.RegisterRoutes(r)
	x := uuid.NewString()
	ub := "/sempras/unidades/" + x + "/casa-abrigo"
	casos := []struct {
		method, path, body string
		want               int
	}{
		{http.MethodGet, ub, "", 500},
		{http.MethodGet, ub + "/estatisticas?de=" + hoje + "&ate=" + hoje, "", 500},
		{http.MethodGet, ub + "/evolucoes", "", 500},
		{http.MethodGet, ub + "/visitantes", "", 500},
		{http.MethodPost, ub + "/visitantes", `{"tipo_visita":"doacao","nome_visitante":"Igreja","data_visita":"` + ontem + `"}`, 500},
		{http.MethodGet, ub + "/termos", "", 500},
		{http.MethodGet, ub + "/diario?mes=2026-10", "", 500},
		{http.MethodPost, ub + "/diario", `{"data_referencia":"` + ontem + `","texto":"rotina da casa"}`, 500},
		{http.MethodGet, ub + "/relatorios", "", 500},
		{http.MethodPost, ub + "/relatorios", `{"tipo":"mensal","status":"revisao","data_entrega":"` + ontem + `"}`, 500},
		{http.MethodGet, ub + "/tarefas", "", 500},
		{http.MethodPost, ub + "/tarefas", `{"titulo":"Relatório"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/casa-abrigo", `{"unidade_id":"` + x + `",` + assistente + `}`, 500},
		{http.MethodGet, "/sempras/casa-abrigo/" + x, "", 500},
		{http.MethodPut, "/sempras/casa-abrigo/" + x, `{` + assistente + `}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/desligar", `{"tipo":"adocao","motivo":"sentença","data_saida":"` + ontem + `"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/irmaos", `{"nome":"Irmão"}`, 500},
		{http.MethodDelete, "/sempras/casa-abrigo-irmaos/" + x, "", 500},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/diagnosticos", `{"categoria":"doenca","diagnostico":"Asma"}`, 500},
		{http.MethodDelete, "/sempras/casa-abrigo-diagnosticos/" + x, "", 500},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/visitantes", `{"nome_visitante":"Avó","data_visita":"` + ontem + `"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo-visitantes/" + x + "/saida", `{"hora_saida":"10:00"}`, 500},
		{http.MethodDelete, "/sempras/casa-abrigo-visitantes/" + x, "", 500},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/termos", `{"tipo_termo":"passeio","data_termo":"` + ontem + `"}`, 500},
		{http.MethodDelete, "/sempras/casa-abrigo-termos/" + x, "", 500},
		{http.MethodPut, "/sempras/casa-abrigo/" + x + "/pia", `{"escola":"EMEB"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/pia-metas", `{"tipo":"plano","descricao":"matrícula"}`, 500},
		{http.MethodPut, "/sempras/casa-abrigo-pia-metas/" + x, `{"tipo":"plano","descricao":"matrícula"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo-pia-metas/" + x + "/concluir", `{"concluida":true}`, 500},
		{http.MethodDelete, "/sempras/casa-abrigo-pia-metas/" + x, "", 500},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/documentos/uploads", `{"tipo":"acolhido","filename":"doc.pdf","content_type":"application/pdf","size":8}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo-documentos/" + x + "/confirm", "", 500},
		{http.MethodGet, "/sempras/casa-abrigo-documentos/" + x + "/download", "", 500},
		{http.MethodDelete, "/sempras/casa-abrigo-documentos/" + x, "", 500},
		{http.MethodPut, "/sempras/casa-abrigo-relatorios/" + x, `{"tipo":"mensal","status":"revisao","data_entrega":"` + ontem + `"}`, 500},
		{http.MethodDelete, "/sempras/casa-abrigo-relatorios/" + x, "", 500},
		{http.MethodPut, "/sempras/casa-abrigo-tarefas/" + x, `{"titulo":"Relatório"}`, 500},
		{http.MethodDelete, "/sempras/casa-abrigo-tarefas/" + x, "", 500},
	}
	// cada rota com {id} recusa o id ruim (400) e, se tem corpo, o corpo ruim (400)
	for _, cs := range casos[:len(casos):len(casos)] {
		ruim := strings.Replace(cs.path, x, "zz", 1)
		casos = append(casos, struct {
			method, path, body string
			want               int
		}{cs.method, ruim, cs.body, 400})
		if cs.body != "" {
			casos = append(casos, struct {
				method, path, body string
				want               int
			}{cs.method, cs.path, `{`, 400})
		}
	}
	for _, tc := range casos {
		down.calls = 0
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		if rec.Code != tc.want || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("%s %s: esperado %d, veio %d %s", tc.method, tc.path, tc.want, rec.Code, rec.Body.String())
		}
	}
}
