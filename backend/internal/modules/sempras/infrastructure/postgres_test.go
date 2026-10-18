package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
)

// Toda falha do banco sobe como erro (nunca como sucesso vazio), e
// "nenhuma linha" vira o erro de domínio.
func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	area := domain.Area{Unidades: []uuid.UUID{id}, Entidades: []uuid.UUID{id}}
	unidades := []domain.Unidade{{ID: id}}

	calls := map[string]func(db database.DBTX) error{
		"GetUnidade":   func(db database.DBTX) error { _, err := r.GetUnidade(ctx, db, id); return err },
		"ListUnidades": func(db database.DBTX) error { _, err := r.ListUnidades(ctx, db, area, "sempras"); return err },
		"InsertCidadao": func(db database.DBTX) error {
			return r.InsertCidadao(ctx, db, domain.Cidadao{ID: id}, id)
		},
		"UpdateCidadao":      func(db database.DBTX) error { return r.UpdateCidadao(ctx, db, domain.Cidadao{ID: id}) },
		"GetCidadao":         func(db database.DBTX) error { _, err := r.GetCidadao(ctx, db, id, true); return err },
		"BuscarPorDocumento": func(db database.DBTX) error { _, err := r.BuscarPorDocumento(ctx, db, "1"); return err },
		"BuscarPorNome":      func(db database.DBTX) error { _, err := r.BuscarPorNome(ctx, db, "ma_ria%", 5); return err },
		"HistoricoNaArea": func(db database.DBTX) error {
			_, err := r.HistoricoNaArea(ctx, db, []uuid.UUID{id}, area)
			return err
		},
		"ProximaSenha":      func(db database.DBTX) error { _, err := r.ProximaSenha(ctx, db, id, "2026-01-01"); return err },
		"InsertAtendimento": func(db database.DBTX) error { return r.InsertAtendimento(ctx, db, domain.Atendimento{ID: id}) },
		"GetAtendimento":    func(db database.DBTX) error { _, err := r.GetAtendimento(ctx, db, id, true); return err },
		"ProximoAguardando": func(db database.DBTX) error {
			_, err := r.ProximoAguardando(ctx, db, id, "2026-01-01", []string{"social"})
			return err
		},
		"UpdateAtendimento": func(db database.DBTX) error { return r.UpdateAtendimento(ctx, db, domain.Atendimento{ID: id}) },
		"Fila":              func(db database.DBTX) error { _, err := r.Fila(ctx, db, id, "2026-01-01"); return err },
		"AtendimentosNaArea": func(db database.DBTX) error {
			_, err := r.AtendimentosNaArea(ctx, db, id, area)
			return err
		},
		"Painel": func(db database.DBTX) error { _, err := r.Painel(ctx, db, unidades, "2026-01-01"); return err },
		"Rma": func(db database.DBTX) error {
			_, err := r.Rma(ctx, db, unidades, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
			return err
		},
		"InsertRegistro":    func(db database.DBTX) error { return r.InsertRegistro(ctx, db, domain.Registro{ID: id}) },
		"RegistrosNaArea":   func(db database.DBTX) error { _, err := r.RegistrosNaArea(ctx, db, id, area); return err },
		"RegistrosDeOrigem": func(db database.DBTX) error { _, err := r.RegistrosDeOrigem(ctx, db, id, area); return err },
		"AcompanhamentosForaDaArea": func(db database.DBTX) error {
			_, err := r.AcompanhamentosForaDaArea(ctx, db, id, area)
			return err
		},
		"InsertPedido": func(db database.DBTX) error {
			return r.InsertPedido(ctx, db, domain.PedidoTitular{ID: id})
		},
		"GetPedido":            func(db database.DBTX) error { _, err := r.GetPedido(ctx, db, id, true); return err },
		"ListPedidosPendentes": func(db database.DBTX) error { _, err := r.ListPedidosPendentes(ctx, db); return err },
		"DecidirPedido": func(db database.DBTX) error {
			return r.DecidirPedido(ctx, db, id, domain.PedidoStatusConcluido, id, "x", time.Now())
		},
		"AnonimizarCidadao": func(db database.DBTX) error { return r.AnonimizarCidadao(ctx, db, id) },
		"InsertContatoRedeApoio": func(db database.DBTX) error {
			return r.InsertContatoRedeApoio(ctx, db, domain.ContatoRedeApoio{ID: id}, id)
		},
		"ListContatosRedeApoio": func(db database.DBTX) error { _, err := r.ListContatosRedeApoio(ctx, db, id); return err },
		"GetContatoRedeApoio":   func(db database.DBTX) error { _, err := r.GetContatoRedeApoio(ctx, db, id); return err },
		"DeleteContatoRedeApoio": func(db database.DBTX) error {
			return r.DeleteContatoRedeApoio(ctx, db, id)
		},
		"InsertBeneficio": func(db database.DBTX) error {
			return r.InsertBeneficio(ctx, db, domain.Beneficio{ID: id}, id)
		},
		"GetBeneficio":    func(db database.DBTX) error { _, err := r.GetBeneficio(ctx, db, id, true); return err },
		"UpdateBeneficio": func(db database.DBTX) error { return r.UpdateBeneficio(ctx, db, domain.Beneficio{ID: id}) },
		"BeneficiosNaArea": func(db database.DBTX) error {
			_, err := r.BeneficiosNaArea(ctx, db, id, area)
			return err
		},
		"InsertEncaminhamento": func(db database.DBTX) error {
			return r.InsertEncaminhamento(ctx, db, domain.Encaminhamento{ID: id}, id)
		},
		"GetEncaminhamento":    func(db database.DBTX) error { _, err := r.GetEncaminhamento(ctx, db, id, true); return err },
		"UpdateEncaminhamento": func(db database.DBTX) error { return r.UpdateEncaminhamento(ctx, db, domain.Encaminhamento{ID: id}) },
		"EncaminhamentosDoCidadaoNaArea": func(db database.DBTX) error {
			_, err := r.EncaminhamentosDoCidadaoNaArea(ctx, db, id, area)
			return err
		},
		"ListEncaminhamentosPendentes": func(db database.DBTX) error {
			_, err := r.ListEncaminhamentosPendentes(ctx, db, id)
			return err
		},
		"InsertPaifVinculo": func(db database.DBTX) error {
			return r.InsertPaifVinculo(ctx, db, domain.PaifVinculo{ID: id}, id)
		},
		"GetPaifVinculo":    func(db database.DBTX) error { _, err := r.GetPaifVinculo(ctx, db, id, true); return err },
		"UpdatePaifVinculo": func(db database.DBTX) error { return r.UpdatePaifVinculo(ctx, db, domain.PaifVinculo{ID: id}) },
		"PaifVinculosNaArea": func(db database.DBTX) error {
			_, err := r.PaifVinculosNaArea(ctx, db, id, area)
			return err
		},
		"InsertConselhoAtendimento": func(db database.DBTX) error {
			return r.InsertConselhoAtendimento(ctx, db, domain.ConselhoAtendimento{ID: id, Protocolo: "CT-" + id.String()[:8]}, id)
		},
		"GetConselhoAtendimento": func(db database.DBTX) error {
			_, err := r.GetConselhoAtendimento(ctx, db, id, true)
			return err
		},
		"UpdateConselhoAtendimento": func(db database.DBTX) error {
			return r.UpdateConselhoAtendimento(ctx, db, domain.ConselhoAtendimento{ID: id})
		},
		"ConselhoAtendimentosNaArea": func(db database.DBTX) error {
			_, err := r.ConselhoAtendimentosNaArea(ctx, db, id, area)
			return err
		},
		"InsertPaefiCaso": func(db database.DBTX) error {
			return r.InsertPaefiCaso(ctx, db, domain.PaefiCaso{ID: id, CasoCodigo: "PAEFI-" + id.String()[:8], Violacoes: []string{"violencia_fisica"}}, id)
		},
		"GetPaefiCaso": func(db database.DBTX) error { _, err := r.GetPaefiCaso(ctx, db, id, true); return err },
		"UpdatePaefiCaso": func(db database.DBTX) error {
			return r.UpdatePaefiCaso(ctx, db, domain.PaefiCaso{ID: id})
		},
		"PaefiCasosNaArea": func(db database.DBTX) error {
			_, err := r.PaefiCasosNaArea(ctx, db, id, area)
			return err
		},
		// Ofícios do CRAS, coletivos e Centro POP (fases 8.1 e 9).
		"InsertOficioCras": func(db database.DBTX) error { return r.InsertOficioCras(ctx, db, domain.OficioCras{ID: id}, id) },
		"GetOficioCras":    func(db database.DBTX) error { _, err := r.GetOficioCras(ctx, db, id, true); return err },
		"UpdateOficioCras": func(db database.DBTX) error { return r.UpdateOficioCras(ctx, db, domain.OficioCras{ID: id}) },
		"OficiosCrasNaArea": func(db database.DBTX) error {
			_, err := r.OficiosCrasNaArea(ctx, db, id, area)
			return err
		},
		"InsertAtendimentoColetivo": func(db database.DBTX) error {
			return r.InsertAtendimentoColetivo(ctx, db, domain.AtendimentoColetivo{ID: id}, id)
		},
		"ListAtendimentosColetivos": func(db database.DBTX) error {
			_, err := r.ListAtendimentosColetivos(ctx, db, id, time.Now(), time.Now())
			return err
		},
		"UpsertCentroPopPerfil": func(db database.DBTX) error {
			return r.UpsertCentroPopPerfil(ctx, db, domain.CentroPopPerfil{CidadaoID: id}, id)
		},
		"GetCentroPopPerfil": func(db database.DBTX) error { _, err := r.GetCentroPopPerfil(ctx, db, id); return err },
		"InsertEncaminhamentoExterno": func(db database.DBTX) error {
			return r.InsertEncaminhamentoExterno(ctx, db, domain.EncaminhamentoExterno{ID: id}, id)
		},
		"GetEncaminhamentoExterno": func(db database.DBTX) error {
			_, err := r.GetEncaminhamentoExterno(ctx, db, id, true)
			return err
		},
		"UpdateEncaminhamentoExterno": func(db database.DBTX) error {
			return r.UpdateEncaminhamentoExterno(ctx, db, domain.EncaminhamentoExterno{ID: id})
		},
		"EncaminhamentosExternosNaArea": func(db database.DBTX) error {
			_, err := r.EncaminhamentosExternosNaArea(ctx, db, id, area)
			return err
		},
		"InsertDocumentoCentroPop": func(db database.DBTX) error {
			return r.InsertDocumentoCentroPop(ctx, db, domain.DocumentoCentroPop{ID: id}, id)
		},
		"GetDocumentoCentroPop": func(db database.DBTX) error { _, err := r.GetDocumentoCentroPop(ctx, db, id); return err },
		"UpdateDocumentoCentroPopStatus": func(db database.DBTX) error {
			return r.UpdateDocumentoCentroPopStatus(ctx, db, id, domain.DocumentoStatusReady, 8, "application/pdf")
		},
		"ListDocumentosCentroPop": func(db database.DBTX) error {
			_, err := r.ListDocumentosCentroPop(ctx, db, id)
			return err
		},
		"DeleteDocumentoCentroPop": func(db database.DBTX) error { return r.DeleteDocumentoCentroPop(ctx, db, id) },
		"InsertAbordagem":          func(db database.DBTX) error { return r.InsertAbordagem(ctx, db, domain.Abordagem{ID: id}, id) },
		"ListAbordagens": func(db database.DBTX) error {
			_, err := r.ListAbordagens(ctx, db, id, time.Now(), time.Now())
			return err
		},
		"AbordagensNaArea": func(db database.DBTX) error {
			_, err := r.AbordagensNaArea(ctx, db, id, area)
			return err
		},
		"ConfigurarUnidade": func(db database.DBTX) error { return r.ConfigurarUnidade(ctx, db, id, domain.UnidadeCras, true) },
		"ConfigurarUnidade (tirar)": func(db database.DBTX) error {
			return r.ConfigurarUnidade(ctx, db, id, "", false)
		},
		"InsertCasoCasaMulher": func(db database.DBTX) error {
			return r.InsertCasoCasaMulher(ctx, db, domain.CasoCasaMulher{ID: id}, id)
		},
		"GetCasoCasaMulher": func(db database.DBTX) error { _, err := r.GetCasoCasaMulher(ctx, db, id, true); return err },
		"UpdateCasoCasaMulher": func(db database.DBTX) error {
			return r.UpdateCasoCasaMulher(ctx, db, domain.CasoCasaMulher{ID: id})
		},
		"CasosCasaMulherNaArea": func(db database.DBTX) error {
			_, err := r.CasosCasaMulherNaArea(ctx, db, id, area)
			return err
		},
		"InsertMseMedida": func(db database.DBTX) error {
			return r.InsertMseMedida(ctx, db, domain.MseMedida{ID: id, AtoInfracional: "furto", TipoMedida: domain.MseTipoLiberdadeAssistida}, id)
		},
		"GetMseMedida": func(db database.DBTX) error { _, err := r.GetMseMedida(ctx, db, id, true); return err },
		"UpdateMseMedida": func(db database.DBTX) error {
			return r.UpdateMseMedida(ctx, db, domain.MseMedida{ID: id})
		},
		"MseMedidasNaArea": func(db database.DBTX) error {
			_, err := r.MseMedidasNaArea(ctx, db, id, area)
			return err
		},
		"InsertNotificacaoCreas": func(db database.DBTX) error {
			return r.InsertNotificacaoCreas(ctx, db, domain.NotificacaoCreas{ID: id, Protocolo: "OF-" + id.String()[:8], OrigemEntidade: "ministerio_publico", Assunto: "assunto"}, id)
		},
		"GetNotificacaoCreas": func(db database.DBTX) error { _, err := r.GetNotificacaoCreas(ctx, db, id, true); return err },
		"UpdateNotificacaoCreas": func(db database.DBTX) error {
			return r.UpdateNotificacaoCreas(ctx, db, domain.NotificacaoCreas{ID: id})
		},
		"NotificacoesCreasNaArea": func(db database.DBTX) error {
			_, err := r.NotificacoesCreasNaArea(ctx, db, id, area)
			return err
		},
		"InsertAcolhimento": func(db database.DBTX) error {
			return r.InsertAcolhimento(ctx, db, domain.Acolhimento{ID: id, AcolhimentoDados: domain.AcolhimentoDados{Sexo: "masculino", Motivos: []string{"outro"}}}, id)
		},
		"GetAcolhimento": func(db database.DBTX) error { _, err := r.GetAcolhimento(ctx, db, id, true); return err },
		"UpdateAcolhimento": func(db database.DBTX) error {
			return r.UpdateAcolhimento(ctx, db, domain.Acolhimento{ID: id, AcolhimentoDados: domain.AcolhimentoDados{Motivos: []string{"outro"}}})
		},
		"AcolhimentosNaArea": func(db database.DBTX) error {
			_, err := r.AcolhimentosNaArea(ctx, db, id, area)
			return err
		},
		"InsertIrmao": func(db database.DBTX) error {
			return r.InsertIrmao(ctx, db, domain.Irmao{ID: id, AcolhimentoID: id, Nome: "Irmão Teste"})
		},
		"ListIrmaos": func(db database.DBTX) error { _, err := r.ListIrmaos(ctx, db, id); return err },
		"GetIrmao":   func(db database.DBTX) error { _, err := r.GetIrmao(ctx, db, id); return err },
		"DeleteIrmao": func(db database.DBTX) error {
			return r.DeleteIrmao(ctx, db, id)
		},
		"InsertDiagnostico": func(db database.DBTX) error {
			return r.InsertDiagnostico(ctx, db, domain.Diagnostico{ID: id, AcolhimentoID: id, Categoria: "doenca", Diagnostico: "diagnóstico teste"})
		},
		"ListDiagnosticos": func(db database.DBTX) error { _, err := r.ListDiagnosticos(ctx, db, id); return err },
		"GetDiagnostico":   func(db database.DBTX) error { _, err := r.GetDiagnostico(ctx, db, id); return err },
		"DeleteDiagnostico": func(db database.DBTX) error {
			return r.DeleteDiagnostico(ctx, db, id)
		},
		"InsertVisitante": func(db database.DBTX) error {
			return r.InsertVisitante(ctx, db, domain.Visitante{ID: id, UnidadeID: id, AcolhimentoID: &id, TipoVisita: "acolhido", NomeVisitante: "Visitante Teste", DataVisita: time.Now()}, id)
		},
		"ListVisitantes": func(db database.DBTX) error { _, err := r.ListVisitantes(ctx, db, id); return err },
		"GetVisitante":   func(db database.DBTX) error { _, err := r.GetVisitante(ctx, db, id); return err },
		"DeleteVisitante": func(db database.DBTX) error {
			return r.DeleteVisitante(ctx, db, id)
		},
		"InsertTermo": func(db database.DBTX) error {
			return r.InsertTermo(ctx, db, domain.Termo{ID: id, AcolhimentoID: id, TipoTermo: "passeio", DataTermo: time.Now()}, id)
		},
		"ListTermos": func(db database.DBTX) error { _, err := r.ListTermos(ctx, db, id); return err },
		"GetTermo":   func(db database.DBTX) error { _, err := r.GetTermo(ctx, db, id); return err },
		"DeleteTermo": func(db database.DBTX) error {
			return r.DeleteTermo(ctx, db, id)
		},
		"UpsertPiaFicha": func(db database.DBTX) error {
			return r.UpsertPiaFicha(ctx, db, domain.PiaFicha{AcolhimentoID: id}, id)
		},
		"GetPiaFicha": func(db database.DBTX) error { _, err := r.GetPiaFicha(ctx, db, id); return err },
		"InsertPiaMeta": func(db database.DBTX) error {
			return r.InsertPiaMeta(ctx, db, domain.PiaMeta{ID: id, AcolhimentoID: id, Tipo: domain.PiaMetaPlano, Descricao: "descrição teste"}, id)
		},
		"ListPiaMetas": func(db database.DBTX) error { _, err := r.ListPiaMetas(ctx, db, id); return err },
		"GetPiaMeta":   func(db database.DBTX) error { _, err := r.GetPiaMeta(ctx, db, id); return err },
		"UpdatePiaMeta": func(db database.DBTX) error {
			return r.UpdatePiaMeta(ctx, db, domain.PiaMeta{ID: id})
		},
		"DeletePiaMeta": func(db database.DBTX) error {
			return r.DeletePiaMeta(ctx, db, id)
		},
		"InsertDocumento": func(db database.DBTX) error {
			return r.InsertDocumento(ctx, db, domain.DocumentoCasaAbrigo{ID: id, AcolhimentoID: id, Tipo: "acolhido", NomeOriginal: "doc.pdf", ObjectKey: "key-" + id.String()}, id)
		},
		"GetDocumento": func(db database.DBTX) error { _, err := r.GetDocumento(ctx, db, id); return err },
		"UpdateDocumentoStatus": func(db database.DBTX) error {
			return r.UpdateDocumentoStatus(ctx, db, id, "ready", 8, "application/pdf")
		},
		"ListDocumentos": func(db database.DBTX) error { _, err := r.ListDocumentos(ctx, db, id); return err },
		"DeleteDocumento": func(db database.DBTX) error {
			return r.DeleteDocumento(ctx, db, id)
		},
		"UpdateVisitante": func(db database.DBTX) error { return r.UpdateVisitante(ctx, db, domain.Visitante{ID: id}) },
		"ListVisitantesDaUnidade": func(db database.DBTX) error {
			_, err := r.ListVisitantesDaUnidade(ctx, db, id)
			return err
		},
		"ListTermosDaUnidade": func(db database.DBTX) error { _, err := r.ListTermosDaUnidade(ctx, db, id); return err },
		"AcolhidosDaUnidade":  func(db database.DBTX) error { _, err := r.AcolhidosDaUnidade(ctx, db, id); return err },
		"MetasPendentesDaUnidade": func(db database.DBTX) error {
			_, err := r.MetasPendentesDaUnidade(ctx, db, id)
			return err
		},
		"EvolucoesDaUnidade": func(db database.DBTX) error { _, err := r.EvolucoesDaUnidade(ctx, db, id); return err },
		"InsertAnotacaoDiario": func(db database.DBTX) error {
			return r.InsertAnotacaoDiario(ctx, db, domain.AnotacaoDiario{ID: id, UnidadeID: id, Texto: "anotação"}, id)
		},
		"ListDiario": func(db database.DBTX) error { _, err := r.ListDiario(ctx, db, id, time.Now(), time.Now()); return err },
		"InsertRelatorio": func(db database.DBTX) error {
			return r.InsertRelatorio(ctx, db, domain.RelatorioCasaAbrigo{ID: id, UnidadeID: id, Tipo: "mensal", Status: "em_elaboracao"}, id)
		},
		"GetRelatorio":    func(db database.DBTX) error { _, err := r.GetRelatorio(ctx, db, id, true); return err },
		"UpdateRelatorio": func(db database.DBTX) error { return r.UpdateRelatorio(ctx, db, domain.RelatorioCasaAbrigo{ID: id}) },
		"DeleteRelatorio": func(db database.DBTX) error { return r.DeleteRelatorio(ctx, db, id) },
		"ListRelatorios":  func(db database.DBTX) error { _, err := r.ListRelatorios(ctx, db, id); return err },
		"InsertTarefa": func(db database.DBTX) error {
			return r.InsertTarefa(ctx, db, domain.TarefaCasaAbrigo{ID: id, UnidadeID: id, Titulo: "tarefa"}, id)
		},
		"GetTarefa":               func(db database.DBTX) error { _, err := r.GetTarefa(ctx, db, id, true); return err },
		"UpdateTarefa":            func(db database.DBTX) error { return r.UpdateTarefa(ctx, db, domain.TarefaCasaAbrigo{ID: id}) },
		"DeleteTarefa":            func(db database.DBTX) error { return r.DeleteTarefa(ctx, db, id) },
		"ListTarefas":             func(db database.DBTX) error { _, err := r.ListTarefas(ctx, db, id); return err },
		"RegistrosDoAcolhido":     func(db database.DBTX) error { _, err := r.RegistrosDoAcolhido(ctx, db, id, id); return err },
		"DiarioDoAcolhimento":     func(db database.DBTX) error { _, err := r.DiarioDoAcolhimento(ctx, db, id); return err },
		"RelatoriosDoAcolhimento": func(db database.DBTX) error { _, err := r.RelatoriosDoAcolhimento(ctx, db, id); return err },
		"UnidadesDoTipo":          func(db database.DBTX) error { _, err := r.UnidadesDoTipo(ctx, db, "abrigo"); return err },
		"EquipeDaUnidade":         func(db database.DBTX) error { _, err := r.EquipeDaUnidade(ctx, db, id); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: esperado ErrInjected, veio %v", name, err)
		}
	}
	// Linhas ilegíveis e conexão perdida no meio da leitura.
	for _, name := range []string{"GetUnidade", "ListUnidades", "GetCidadao", "BuscarPorDocumento", "BuscarPorNome",
		"HistoricoNaArea", "ProximaSenha", "GetAtendimento", "ProximoAguardando", "Fila", "AtendimentosNaArea", "Painel", "Rma",
		"RegistrosNaArea", "GetPedido", "ListPedidosPendentes", "GetContatoRedeApoio", "ListContatosRedeApoio",
		"GetBeneficio", "BeneficiosNaArea", "GetEncaminhamento", "EncaminhamentosDoCidadaoNaArea", "ListEncaminhamentosPendentes",
		"GetPaifVinculo", "PaifVinculosNaArea", "GetConselhoAtendimento", "ConselhoAtendimentosNaArea",
		"GetPaefiCaso", "PaefiCasosNaArea", "GetMseMedida", "MseMedidasNaArea", "GetNotificacaoCreas", "NotificacoesCreasNaArea",
		"GetAcolhimento", "AcolhimentosNaArea", "ListIrmaos", "GetIrmao", "ListDiagnosticos", "GetDiagnostico",
		"ListVisitantes", "GetVisitante", "ListTermos", "GetTermo", "GetPiaFicha", "ListPiaMetas", "GetPiaMeta",
		"GetDocumento", "ListDocumentos", "RegistrosDeOrigem", "AcompanhamentosForaDaArea", "GetCasoCasaMulher",
		"CasosCasaMulherNaArea", "GetOficioCras", "OficiosCrasNaArea", "ListAtendimentosColetivos", "GetCentroPopPerfil",
		"GetEncaminhamentoExterno", "EncaminhamentosExternosNaArea", "GetDocumentoCentroPop", "ListDocumentosCentroPop",
		"ListAbordagens", "AbordagensNaArea", "ListVisitantesDaUnidade", "ListTermosDaUnidade", "AcolhidosDaUnidade",
		"MetasPendentesDaUnidade", "EvolucoesDaUnidade", "ListDiario", "GetRelatorio", "ListRelatorios", "GetTarefa", "ListTarefas", "RegistrosDoAcolhido", "DiarioDoAcolhimento", "RelatoriosDoAcolhimento", "UnidadesDoTipo", "EquipeDaUnidade"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
	}
	for _, name := range []string{"ListUnidades", "BuscarPorDocumento", "BuscarPorNome", "HistoricoNaArea", "Fila",
		"AtendimentosNaArea", "Painel", "Rma", "RegistrosNaArea", "ListPedidosPendentes", "ListContatosRedeApoio", "BeneficiosNaArea",
		"EncaminhamentosDoCidadaoNaArea", "ListEncaminhamentosPendentes", "PaifVinculosNaArea", "ConselhoAtendimentosNaArea",
		"PaefiCasosNaArea", "MseMedidasNaArea", "NotificacoesCreasNaArea", "AcolhimentosNaArea", "ListIrmaos", "ListDiagnosticos",
		"ListVisitantes", "ListTermos", "ListPiaMetas", "ListDocumentos", "RegistrosDeOrigem", "AcompanhamentosForaDaArea",
		"CasosCasaMulherNaArea", "OficiosCrasNaArea", "ListAtendimentosColetivos", "EncaminhamentosExternosNaArea",
		"ListDocumentosCentroPop", "ListAbordagens", "AbordagensNaArea", "ListVisitantesDaUnidade", "ListTermosDaUnidade",
		"AcolhidosDaUnidade", "MetasPendentesDaUnidade", "EvolucoesDaUnidade", "ListDiario", "ListRelatorios", "ListTarefas", "RegistrosDoAcolhido", "DiarioDoAcolhimento", "RelatoriosDoAcolhimento", "UnidadesDoTipo", "EquipeDaUnidade"} {
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com a conexão perdida na leitura: %v", name, err)
		}
	}
}

// "Nenhuma linha" vira o erro de domínio (banco de verdade).
func TestRepositoryNotFound(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	if _, err := r.GetUnidade(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetUnidade: %v", err)
	}
	if _, err := r.GetCidadao(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetCidadao: %v", err)
	}
	if _, err := r.GetAtendimento(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetAtendimento: %v", err)
	}
	if _, err := r.ProximoAguardando(ctx, pool, id, "2026-01-01", []string{"social"}); !errors.Is(err, domain.ErrFilaVazia) {
		t.Errorf("ProximoAguardando: %v", err)
	}
	if _, err := r.GetPedido(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetPedido: %v", err)
	}
	if _, err := r.GetContatoRedeApoio(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetContatoRedeApoio: %v", err)
	}
	if _, err := r.GetBeneficio(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetBeneficio: %v", err)
	}
	if _, err := r.GetEncaminhamento(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetEncaminhamento: %v", err)
	}
	if _, err := r.GetPaifVinculo(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetPaifVinculo: %v", err)
	}
	if _, err := r.GetConselhoAtendimento(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetConselhoAtendimento: %v", err)
	}
	if _, err := r.GetPaefiCaso(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetPaefiCaso: %v", err)
	}
	if _, err := r.GetMseMedida(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetMseMedida: %v", err)
	}
	if _, err := r.GetNotificacaoCreas(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetNotificacaoCreas: %v", err)
	}
	if _, err := r.GetAcolhimento(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetAcolhimento: %v", err)
	}
	if _, err := r.GetIrmao(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetIrmao: %v", err)
	}
	if _, err := r.GetDiagnostico(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetDiagnostico: %v", err)
	}
	if _, err := r.GetVisitante(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetVisitante: %v", err)
	}
	if _, err := r.GetTermo(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetTermo: %v", err)
	}
	if _, err := r.GetPiaFicha(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetPiaFicha: %v", err)
	}
	if _, err := r.GetPiaMeta(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetPiaMeta: %v", err)
	}
	if _, err := r.GetRelatorio(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetRelatorio: %v", err)
	}
	if _, err := r.GetTarefa(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetTarefa: %v", err)
	}
	if _, err := r.GetDocumento(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetDocumento: %v", err)
	}
	if _, err := r.GetOficioCras(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetOficioCras: %v", err)
	}
	if _, err := r.GetEncaminhamentoExterno(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetEncaminhamentoExterno: %v", err)
	}
	if _, err := r.GetDocumentoCentroPop(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetDocumentoCentroPop: %v", err)
	}
	if _, err := r.GetCentroPopPerfil(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetCentroPopPerfil: %v", err)
	}
}

// Round-trip dos pedidos do titular: grava, lista como pendente, decide, e
// a decisão fica congelada (o trigger de imutabilidade bloqueia decidir
// de novo).
func TestPedidosTitularRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)

	// Sem CPF/NIS (só nascimento + mãe): evita colidir com o índice único de
	// outro teste do pacote — mesmo truque de cenario.cadastrar.
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Pessoa Teste",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	p := domain.PedidoTitular{ID: uuid.New(), CidadaoID: cid, Tipo: domain.PedidoAnonimizacao,
		Status: domain.PedidoStatusPendente, JustificativaSolicitacao: "titular pediu para ser esquecido",
		SolicitadoPor: autor, CreatedAt: time.Now()}
	if err := r.InsertPedido(ctx, pool, p); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetPedido(ctx, pool, p.ID, false)
	if err != nil || got.Status != domain.PedidoStatusPendente || got.CidadaoID != cid || got.SolicitadoPorNome == "" ||
		got.CidadaoNome != "Pessoa Teste" {
		t.Fatalf("GetPedido: %+v, %v", got, err)
	}

	pend, err := r.ListPedidosPendentes(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	achou := false
	for _, x := range pend {
		achou = achou || x.ID == p.ID
	}
	if !achou {
		t.Error("pedido pendente não apareceu na lista")
	}

	if err := r.DecidirPedido(ctx, pool, p.ID, domain.PedidoStatusConcluido, autor, "aprovado", time.Now()); err != nil {
		t.Fatal(err)
	}
	decidido, err := r.GetPedido(ctx, pool, p.ID, false)
	if err != nil || decidido.Status != domain.PedidoStatusConcluido || decidido.JustificativaDecisao != "aprovado" || decidido.DecididoPor == nil {
		t.Fatalf("GetPedido após decisão: %+v, %v", decidido, err)
	}
	if err := r.DecidirPedido(ctx, pool, p.ID, domain.PedidoStatusNegado, autor, "tentando mudar de novo", time.Now()); err == nil {
		t.Error("pedido já decidido deveria ser imutável (trigger)")
	}
}

// A anonimização zera os campos identificadores sem tocar no histórico do
// serviço (sempras_atendimentos/sempras_registros são o registro do
// atendimento prestado, não "dado do titular") nem violar a CHECK de
// identificação da tabela (data_nascimento e nome_mae ficam preenchidos).
func TestAnonimizarCidadao(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}

	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Pessoa Anonimizável", CPF: "52998224725",
		NIS: "12345678901", DataNascimento: "1990-01-01", NomeMae: "Mãe Original", Telefone: "66999990000",
		Endereco: "Rua X", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}
	atend := domain.Atendimento{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Dia: "2026-01-01", Senha: 1,
		Tipo: domain.TipoSocial, Status: domain.StatusAguardando, CreatedAt: time.Now()}
	if err := r.InsertAtendimento(ctx, pool, atend); err != nil {
		t.Fatal(err)
	}
	reg := domain.Registro{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Tipo: domain.TipoSocial,
		Texto: "evolução antes da anonimização", AutorID: &autor, CreatedAt: time.Now()}
	if err := r.InsertRegistro(ctx, pool, reg); err != nil {
		t.Fatal(err)
	}

	if err := r.AnonimizarCidadao(ctx, pool, cid); err != nil {
		t.Fatal(err)
	}

	depois, err := r.GetCidadao(ctx, pool, cid, false)
	if err != nil {
		t.Fatal(err)
	}
	if depois.Nome != "Munícipe anonimizado" || depois.NomeSocial != "" || depois.CPF != "" || depois.NIS != "" ||
		depois.Telefone != "" || depois.Endereco != "" {
		t.Errorf("campos não zerados: %+v", depois)
	}
	// Preservados pela CHECK de identificação da tabela (cpf/nis nulos exigem
	// data_nascimento + nome_mae não vazio).
	if depois.DataNascimento != "1990-01-01" || depois.NomeMae == "" {
		t.Errorf("identificação mínima deveria sobreviver: %+v", depois)
	}

	// Histórico do serviço intacto.
	area := domain.Area{All: true}
	atendimentos, err := r.AtendimentosNaArea(ctx, pool, cid, area)
	if err != nil || len(atendimentos) != 1 || atendimentos[0].ID != atend.ID || atendimentos[0].Status != domain.StatusAguardando {
		t.Errorf("atendimento mudou: %+v, %v", atendimentos, err)
	}
	registros, err := r.RegistrosNaArea(ctx, pool, cid, area)
	if err != nil || len(registros) != 1 || registros[0].Texto != reg.Texto {
		t.Errorf("registro mudou: %+v, %v", registros, err)
	}
}

// Round-trip da rede de apoio: grava, lista, remove.
func TestRedeApoioRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)

	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Pessoa Rede de Apoio",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	c := domain.ContatoRedeApoio{ID: uuid.New(), CidadaoID: cid, Nome: "Maria", Parentesco: "Irmã",
		Telefone: "66999990000", CreatedAt: time.Now()}
	if err := r.InsertContatoRedeApoio(ctx, pool, c, autor); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetContatoRedeApoio(ctx, pool, c.ID)
	if err != nil || got.Nome != "Maria" || got.CidadaoID != cid {
		t.Fatalf("GetContatoRedeApoio: %+v, %v", got, err)
	}
	lista, err := r.ListContatosRedeApoio(ctx, pool, cid)
	if err != nil || len(lista) != 1 || lista[0].ID != c.ID {
		t.Fatalf("ListContatosRedeApoio: %+v, %v", lista, err)
	}

	if err := r.DeleteContatoRedeApoio(ctx, pool, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetContatoRedeApoio(ctx, pool, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("deveria ter sumido: %v", err)
	}
}

// Round-trip do benefício eventual: grava, lista na área, decide.
func TestBeneficioRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Pessoa Benefício",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	b := domain.Beneficio{ID: uuid.New(), BeneficiarioID: cid, UnidadeID: un, Tipo: domain.BeneficioCestaBasica,
		FormaAcesso: domain.FormaAcessoDemandaEspontanea, Quantidade: 2, Status: domain.BeneficioStatusSolicitado, DataSolicitacao: time.Now()}
	if err := r.InsertBeneficio(ctx, pool, b, autor); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetBeneficio(ctx, pool, b.ID, false)
	if err != nil || got.Status != domain.BeneficioStatusSolicitado || got.Quantidade != 2 {
		t.Fatalf("GetBeneficio: %+v, %v", got, err)
	}

	area := domain.Area{Unidades: []uuid.UUID{un}}
	lista, err := r.BeneficiosNaArea(ctx, pool, cid, area)
	if err != nil || len(lista) != 1 || lista[0].ID != b.ID {
		t.Fatalf("BeneficiosNaArea: %+v, %v", lista, err)
	}
	if _, err := r.BeneficiosNaArea(ctx, pool, cid, domain.Area{}); err != nil {
		t.Fatal(err)
	} else if lista, _ := r.BeneficiosNaArea(ctx, pool, cid, domain.Area{}); len(lista) != 0 {
		t.Errorf("fora da área não deveria aparecer: %+v", lista)
	}

	now := time.Now()
	got.Status, got.MotivoDecisao, got.TecnicoDecisaoID, got.DataDecisao = domain.BeneficioStatusConcedido, "", &autor, &now
	if err := r.UpdateBeneficio(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	decidido, err := r.GetBeneficio(ctx, pool, b.ID, false)
	if err != nil || decidido.Status != domain.BeneficioStatusConcedido || decidido.TecnicoDecisaoID == nil {
		t.Fatalf("GetBeneficio após decisão: %+v, %v", decidido, err)
	}
}

// Round-trip do encaminhamento: grava, aparece na fila do destino e na
// área de quem vê qualquer uma das duas unidades, decide.
func TestEncaminhamentoRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, origem, destino uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "origem-"+sfx).Scan(&origem); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "destino-"+sfx).Scan(&destino); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Pessoa Encaminhada",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	e := domain.Encaminhamento{ID: uuid.New(), CidadaoID: cid, UnidadeOrigemID: origem, UnidadeDestinoID: destino,
		Motivo: "precisa de acompanhamento especializado", Status: domain.EncaminhamentoStatusPendente, CreatedAt: time.Now()}
	if err := r.InsertEncaminhamento(ctx, pool, e, autor); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetEncaminhamento(ctx, pool, e.ID, false)
	if err != nil || got.Status != domain.EncaminhamentoStatusPendente {
		t.Fatalf("GetEncaminhamento: %+v, %v", got, err)
	}

	pendentes, err := r.ListEncaminhamentosPendentes(ctx, pool, destino)
	if err != nil || len(pendentes) != 1 || pendentes[0].ID != e.ID {
		t.Fatalf("ListEncaminhamentosPendentes: %+v, %v", pendentes, err)
	}

	// quem só vê a origem, ou só o destino, enxerga o mesmo encaminhamento.
	for _, un := range []uuid.UUID{origem, destino} {
		lista, err := r.EncaminhamentosDoCidadaoNaArea(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{un}})
		if err != nil || len(lista) != 1 || lista[0].ID != e.ID {
			t.Errorf("EncaminhamentosDoCidadaoNaArea(%s): %+v, %v", un, lista, err)
		}
	}
	if lista, err := r.EncaminhamentosDoCidadaoNaArea(ctx, pool, cid, domain.Area{}); err != nil || len(lista) != 0 {
		t.Errorf("fora da área não deveria aparecer: %+v, %v", lista, err)
	}

	now := time.Now()
	got.Status, got.ObservacaoDecisao, got.DecididoPor, got.DecididoEm = domain.EncaminhamentoStatusAceito, "vamos acompanhar", &autor, &now
	if err := r.UpdateEncaminhamento(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	decidido, err := r.GetEncaminhamento(ctx, pool, e.ID, false)
	if err != nil || decidido.Status != domain.EncaminhamentoStatusAceito || decidido.DecididoPor == nil {
		t.Fatalf("GetEncaminhamento após decisão: %+v, %v", decidido, err)
	}
	if pendentes, err := r.ListEncaminhamentosPendentes(ctx, pool, destino); err != nil || len(pendentes) != 0 {
		t.Errorf("decidido não deveria ficar na fila: %+v, %v", pendentes, err)
	}

	// Visão cruzada (ADR 031 §4): o destino lê a evolução SOCIAL da origem
	// (nunca a psicológica) e vê o resumo do que corre lá, sem conteúdo.
	for _, tipo := range []string{domain.TipoSocial, domain.TipoPsicologico} {
		reg, err := domain.NovoRegistro(cid, origem, nil, tipo, "evolução "+tipo, autor)
		if err != nil {
			t.Fatal(err)
		}
		reg.CreatedAt = time.Now()
		if err := r.InsertRegistro(ctx, pool, reg); err != nil {
			t.Fatal(err)
		}
	}
	paif, err := domain.NovoPaifVinculo(cid, origem, domain.PaifAtendimentoParticularizado, false, false, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	paif.DataInclusao = time.Now()
	if err := r.InsertPaifVinculo(ctx, pool, paif, autor); err != nil {
		t.Fatal(err)
	}
	noDestino := domain.Area{Unidades: []uuid.UUID{destino}}
	origemRegs, err := r.RegistrosDeOrigem(ctx, pool, cid, noDestino)
	if err != nil || len(origemRegs) != 1 || origemRegs[0].Tipo != domain.TipoSocial || origemRegs[0].UnidadeID != origem {
		t.Errorf("RegistrosDeOrigem: %+v, %v", origemRegs, err)
	}
	// Na própria origem, a evolução já vem por RegistrosNaArea — não repete.
	if regs, err := r.RegistrosDeOrigem(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{origem}}); err != nil || len(regs) != 0 {
		t.Errorf("RegistrosDeOrigem na origem: %+v, %v", regs, err)
	}
	outras, err := r.AcompanhamentosForaDaArea(ctx, pool, cid, noDestino)
	if err != nil || len(outras) != 1 || outras[0].UnidadeID != origem || len(outras[0].Ativos) != 1 || outras[0].Ativos[0] != "paif" {
		t.Errorf("AcompanhamentosForaDaArea: %+v, %v", outras, err)
	}
	if outras, err := r.AcompanhamentosForaDaArea(ctx, pool, cid, domain.Area{All: true}); err != nil || len(outras) != 0 {
		t.Errorf("quem vê tudo não tem 'outras unidades': %+v, %v", outras, err)
	}
}

// Round-trip do PAIF: grava, lista na área, a constraint única barra um
// segundo vínculo ativo, exclui, reinclui (um novo vínculo, agora permitido).
func TestPaifVinculoRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Família PAIF",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	v := domain.PaifVinculo{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Status: domain.PaifStatusAtivo,
		TipoAtendimento: domain.PaifAtendimentoParticularizado, ExtremaPobreza: true, SCFVParticipantes: 2, DataInclusao: time.Now()}
	if err := r.InsertPaifVinculo(ctx, pool, v, autor); err != nil {
		t.Fatal(err)
	}

	// um segundo vínculo ativo pra mesma família+unidade é rejeitado.
	v2 := domain.PaifVinculo{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Status: domain.PaifStatusAtivo,
		TipoAtendimento: domain.PaifAtendimentoParticularizado, DataInclusao: time.Now()}
	if err := r.InsertPaifVinculo(ctx, pool, v2, autor); !errors.Is(err, domain.ErrPaifJaAtivo) {
		t.Errorf("segundo vínculo ativo deveria ser rejeitado: %v", err)
	}

	got, err := r.GetPaifVinculo(ctx, pool, v.ID, false)
	if err != nil || got.Status != domain.PaifStatusAtivo || !got.ExtremaPobreza || got.SCFVParticipantes != 2 {
		t.Fatalf("GetPaifVinculo: %+v, %v", got, err)
	}
	lista, err := r.PaifVinculosNaArea(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{un}})
	if err != nil || len(lista) != 1 || lista[0].ID != v.ID {
		t.Fatalf("PaifVinculosNaArea: %+v, %v", lista, err)
	}

	now := time.Now()
	got.Status, got.MotivoExclusao, got.ExcluidoPor, got.DataExclusao = domain.PaifStatusExcluido, "família se mudou", &autor, &now
	if err := r.UpdatePaifVinculo(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	excluido, err := r.GetPaifVinculo(ctx, pool, v.ID, false)
	if err != nil || excluido.Status != domain.PaifStatusExcluido {
		t.Fatalf("GetPaifVinculo após exclusão: %+v, %v", excluido, err)
	}

	// depois de excluído, um novo vínculo ativo é aceito (reinclusão).
	v3 := domain.PaifVinculo{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Status: domain.PaifStatusAtivo,
		TipoAtendimento: domain.PaifAtendimentoParticularizado, DataInclusao: time.Now()}
	if err := r.InsertPaifVinculo(ctx, pool, v3, autor); err != nil {
		t.Errorf("reinclusão após exclusão deveria ser aceita: %v", err)
	}
}

// Round-trip do Conselho Tutelar: grava, lista na área da criança,
// transita (aplicar_medida), confere a persistência, e o protocolo é
// único (um segundo atendimento com o mesmo protocolo é rejeitado).
func TestConselhoAtendimentoRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un, unOutra uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-outra-"+sfx).Scan(&unOutra); err != nil {
		t.Fatal(err)
	}
	crianca := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: crianca, Nome: "Criança Teste",
		DataNascimento: "2015-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	c, err := domain.NovoConselhoAtendimento(crianca, un, "Responsável Teste", "", "", "escola", "evasao_escolar", "")
	if err != nil {
		t.Fatal(err)
	}
	c.CreatedAt = time.Now()
	if err := r.InsertConselhoAtendimento(ctx, pool, c, autor); err != nil {
		t.Fatal(err)
	}

	// protocolo é único: um segundo atendimento com o mesmo protocolo é rejeitado.
	dup, err := domain.NovoConselhoAtendimento(crianca, un, "", "", "", "escola", "evasao_escolar", "")
	if err != nil {
		t.Fatal(err)
	}
	dup.Protocolo, dup.CreatedAt = c.Protocolo, time.Now()
	if err := r.InsertConselhoAtendimento(ctx, pool, dup, autor); err == nil {
		t.Error("protocolo duplicado deveria ser rejeitado")
	}

	got, err := r.GetConselhoAtendimento(ctx, pool, c.ID, false)
	if err != nil || got.Status != domain.ConselhoStatusEmAndamento || got.OrigemDemanda != "escola" {
		t.Fatalf("GetConselhoAtendimento: %+v, %v", got, err)
	}

	lista, err := r.ConselhoAtendimentosNaArea(ctx, pool, crianca, domain.Area{Unidades: []uuid.UUID{un}})
	if err != nil || len(lista) != 1 || lista[0].ID != c.ID {
		t.Fatalf("ConselhoAtendimentosNaArea: %+v, %v", lista, err)
	}
	// unidade diferente da área não vê o atendimento.
	foraDaArea, err := r.ConselhoAtendimentosNaArea(ctx, pool, crianca, domain.Area{Unidades: []uuid.UUID{unOutra}})
	if err != nil || len(foraDaArea) != 0 {
		t.Fatalf("ConselhoAtendimentosNaArea fora da área: %+v, %v", foraDaArea, err)
	}

	conselheiro := autor
	if err := got.Transitar("aplicar_medida", "matricula_escolar_obrigatoria", conselheiro, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateConselhoAtendimento(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	atualizado, err := r.GetConselhoAtendimento(ctx, pool, c.ID, false)
	if err != nil || atualizado.Status != domain.ConselhoStatusMedidaAplicada ||
		atualizado.MedidaProtetiva != "matricula_escolar_obrigatoria" || atualizado.ConselheiroID == nil {
		t.Fatalf("GetConselhoAtendimento após transição: %+v, %v", atualizado, err)
	}
}

// Round-trip do PAEFI: grava, lista na área do munícipe, marca o PIA
// elaborado e encerra o caso.
func TestPaefiCasoRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un, unOutra uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-outra-"+sfx).Scan(&unOutra); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Família PAEFI",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	p, err := domain.NovoPaefiCaso(cid, un, []string{"violencia_fisica", "negligencia"}, "dois filhos menores", "")
	if err != nil {
		t.Fatal(err)
	}
	p.DataAbertura = time.Now()
	if err := r.InsertPaefiCaso(ctx, pool, p, autor); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetPaefiCaso(ctx, pool, p.ID, false)
	if err != nil || got.Status != domain.PaefiStatusEmAcompanhamento || len(got.Violacoes) != 2 || got.PiaElaborado {
		t.Fatalf("GetPaefiCaso: %+v, %v", got, err)
	}

	lista, err := r.PaefiCasosNaArea(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{un}})
	if err != nil || len(lista) != 1 || lista[0].ID != p.ID {
		t.Fatalf("PaefiCasosNaArea: %+v, %v", lista, err)
	}
	foraDaArea, err := r.PaefiCasosNaArea(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{unOutra}})
	if err != nil || len(foraDaArea) != 0 {
		t.Fatalf("PaefiCasosNaArea fora da área: %+v, %v", foraDaArea, err)
	}

	if err := got.MarcarPiaElaborado(autor); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePaefiCaso(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	comPia, err := r.GetPaefiCaso(ctx, pool, p.ID, false)
	if err != nil || !comPia.PiaElaborado {
		t.Fatalf("GetPaefiCaso após PIA: %+v, %v", comPia, err)
	}

	if err := comPia.Encerrar("objetivos_alcancados", autor, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePaefiCaso(ctx, pool, comPia); err != nil {
		t.Fatal(err)
	}
	encerrado, err := r.GetPaefiCaso(ctx, pool, p.ID, false)
	if err != nil || encerrado.Status != domain.PaefiStatusDesligado || encerrado.MotivoEncerramento != "objetivos_alcancados" || encerrado.DataEncerramento == nil {
		t.Fatalf("GetPaefiCaso após encerrar: %+v, %v", encerrado, err)
	}
}

// Round-trip da MSE: grava, lista na área do adolescente e conclui o
// acompanhamento.
func TestMseMedidaRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un, unOutra uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-outra-"+sfx).Scan(&unOutra); err != nil {
		t.Fatal(err)
	}
	adolescente := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: adolescente, Nome: "Adolescente MSE",
		DataNascimento: "2009-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	m, err := domain.NovaMseMedida(adolescente, un, "123/2026", "Vara da Infância", "furto (art. 155 CP)", domain.MseTipoLiberdadeAssistida, "")
	if err != nil {
		t.Fatal(err)
	}
	m.DataInicio = time.Now()
	if err := r.InsertMseMedida(ctx, pool, m, autor); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetMseMedida(ctx, pool, m.ID, false)
	if err != nil || got.Status != domain.MseStatusEmCumprimento || got.TipoMedida != domain.MseTipoLiberdadeAssistida {
		t.Fatalf("GetMseMedida: %+v, %v", got, err)
	}

	lista, err := r.MseMedidasNaArea(ctx, pool, adolescente, domain.Area{Unidades: []uuid.UUID{un}})
	if err != nil || len(lista) != 1 || lista[0].ID != m.ID {
		t.Fatalf("MseMedidasNaArea: %+v, %v", lista, err)
	}
	foraDaArea, err := r.MseMedidasNaArea(ctx, pool, adolescente, domain.Area{Unidades: []uuid.UUID{unOutra}})
	if err != nil || len(foraDaArea) != 0 {
		t.Fatalf("MseMedidasNaArea fora da área: %+v, %v", foraDaArea, err)
	}

	if err := got.Concluir(domain.MseStatusCumprida, autor, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateMseMedida(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	concluida, err := r.GetMseMedida(ctx, pool, m.ID, false)
	if err != nil || concluida.Status != domain.MseStatusCumprida || concluida.DataConclusao == nil {
		t.Fatalf("GetMseMedida após conclusão: %+v, %v", concluida, err)
	}
}

// Round-trip das notificações do CREAS: grava, lista na área da vítima e
// transita (iniciar diligência, responder).
func TestNotificacaoCreasRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un, unOutra uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-outra-"+sfx).Scan(&unOutra); err != nil {
		t.Fatal(err)
	}
	vitima := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: vitima, Nome: "Vítima Notificação",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	n, err := domain.NovaNotificacaoCreas(vitima, un, "ministerio_publico", "requisição de informações sobre acompanhamento", nil)
	if err != nil {
		t.Fatal(err)
	}
	n.DataEntrada = time.Now()
	if err := r.InsertNotificacaoCreas(ctx, pool, n, autor); err != nil {
		t.Fatal(err)
	}

	// protocolo é único: uma segunda notificação com o mesmo protocolo é rejeitada.
	dup := n
	dup.ID = uuid.New()
	if err := r.InsertNotificacaoCreas(ctx, pool, dup, autor); err == nil {
		t.Error("protocolo duplicado deveria ser rejeitado")
	}

	got, err := r.GetNotificacaoCreas(ctx, pool, n.ID, false)
	if err != nil || got.Status != domain.NotificacaoStatusRecebida || got.OrigemEntidade != "ministerio_publico" {
		t.Fatalf("GetNotificacaoCreas: %+v, %v", got, err)
	}

	lista, err := r.NotificacoesCreasNaArea(ctx, pool, vitima, domain.Area{Unidades: []uuid.UUID{un}})
	if err != nil || len(lista) != 1 || lista[0].ID != n.ID {
		t.Fatalf("NotificacoesCreasNaArea: %+v, %v", lista, err)
	}
	foraDaArea, err := r.NotificacoesCreasNaArea(ctx, pool, vitima, domain.Area{Unidades: []uuid.UUID{unOutra}})
	if err != nil || len(foraDaArea) != 0 {
		t.Fatalf("NotificacoesCreasNaArea fora da área: %+v, %v", foraDaArea, err)
	}

	if err := got.Transitar("responder", "família está em acompanhamento ativo pelo PAEFI", autor, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateNotificacaoCreas(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	respondida, err := r.GetNotificacaoCreas(ctx, pool, n.ID, false)
	if err != nil || respondida.Status != domain.NotificacaoStatusRespondida || respondida.DataResposta == nil {
		t.Fatalf("GetNotificacaoCreas após responder: %+v, %v", respondida, err)
	}
}

// Round-trip dos Ofícios do CRAS: grava, lista na área da família e
// responde.
func TestOficioCrasRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un, unOutra uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-outra-"+sfx).Scan(&unOutra); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Família Ofício",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	o, err := domain.NovoOficioCras(cid, un, "Conselho Tutelar", "123/2026", nil, "acompanhamento da família")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.InsertOficioCras(ctx, pool, o, autor); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetOficioCras(ctx, pool, o.ID, false)
	if err != nil || got.Status != domain.OficioStatusPendente || got.OrgaoEmissor != "Conselho Tutelar" {
		t.Fatalf("GetOficioCras: %+v, %v", got, err)
	}

	lista, err := r.OficiosCrasNaArea(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{un}})
	if err != nil || len(lista) != 1 || lista[0].ID != o.ID {
		t.Fatalf("OficiosCrasNaArea: %+v, %v", lista, err)
	}
	foraDaArea, err := r.OficiosCrasNaArea(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{unOutra}})
	if err != nil || len(foraDaArea) != 0 {
		t.Fatalf("OficiosCrasNaArea fora da área: %+v, %v", foraDaArea, err)
	}

	if err := got.Responder("456/2026", time.Now(), autor); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateOficioCras(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	respondido, err := r.GetOficioCras(ctx, pool, o.ID, false)
	if err != nil || respondido.Status != domain.OficioStatusRespondido || respondido.NumeroResposta != "456/2026" {
		t.Fatalf("GetOficioCras após responder: %+v, %v", respondido, err)
	}
}

// Round-trip dos Atendimentos Coletivos: grava eventos de categorias
// diferentes e lista por unidade/mês.
func TestAtendimentoColetivoRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}

	dentro := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	fora := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	a1, err := domain.NovoAtendimentoColetivo(un, dentro, domain.ColetivoSCFV0a6, 10, "turma manhã")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.InsertAtendimentoColetivo(ctx, pool, a1, autor); err != nil {
		t.Fatal(err)
	}
	a2, err := domain.NovoAtendimentoColetivo(un, fora, domain.ColetivoSCFV0a6, 99, "fora do mês")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.InsertAtendimentoColetivo(ctx, pool, a2, autor); err != nil {
		t.Fatal(err)
	}

	inicio, fim := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	lista, err := r.ListAtendimentosColetivos(ctx, pool, un, inicio, fim)
	if err != nil || len(lista) != 1 || lista[0].ID != a1.ID || lista[0].Participantes != 10 {
		t.Fatalf("ListAtendimentosColetivos: %+v, %v", lista, err)
	}
}

// Round-trip do RMA/MDS: grava um pouco de cada entidade do módulo
// dentro e fora da competência, e confirma que o relatório soma certo
// — "ativos" é fotografia (não some fora da competência), os demais só
// contam dentro do intervalo [início, fim).
func TestRmaRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un, unOutra uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-outra-"+sfx).Scan(&unOutra); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Família RMA",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	inicio := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	fim := inicio.AddDate(0, 1, 0)
	dentro := inicio.AddDate(0, 0, 10)
	fora := inicio.AddDate(0, -1, -5) // mês anterior

	// Atendimentos: um dentro (social) e um fora (psicológico) da competência.
	if err := r.InsertAtendimento(ctx, pool, domain.Atendimento{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Dia: "2026-10-10",
		Senha: 1, Tipo: domain.TipoSocial, Status: domain.StatusAguardando, CreatedAt: dentro}); err != nil {
		t.Fatal(err)
	}
	if err := r.InsertAtendimento(ctx, pool, domain.Atendimento{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Dia: "2026-08-10",
		Senha: 1, Tipo: domain.TipoPsicologico, Status: domain.StatusAguardando, CreatedAt: fora}); err != nil {
		t.Fatal(err)
	}

	// PAIF: vínculo ativo (fotografia — entra mesmo tendo sido incluído fora da competência).
	if err := r.InsertPaifVinculo(ctx, pool, domain.PaifVinculo{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Status: domain.PaifStatusAtivo,
		TipoAtendimento: domain.PaifAtendimentoParticularizado, ExtremaPobreza: true, BolsaFamilia: true, SCFVParticipantes: 3, DataInclusao: fora}, autor); err != nil {
		t.Fatal(err)
	}

	// Benefício concedido dentro da competência.
	if err := r.InsertBeneficio(ctx, pool, domain.Beneficio{ID: uuid.New(), BeneficiarioID: cid, UnidadeID: un,
		Tipo: domain.BeneficioCestaBasica, FormaAcesso: domain.FormaAcessoDemandaEspontanea, Quantidade: 1,
		Status: domain.BeneficioStatusSolicitado, DataSolicitacao: dentro}, autor); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `UPDATE sempras_beneficios SET status = 'concedido', data_decisao = $2 WHERE unidade_id = $1 RETURNING id`,
		un, dentro).Scan(new(uuid.UUID)); err != nil {
		t.Fatal(err)
	}

	// PAEFI: caso aberto dentro da competência, em acompanhamento (ativo).
	paefi, err := domain.NovoPaefiCaso(cid, un, []string{"negligencia"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	paefi.DataAbertura = dentro
	if err := r.InsertPaefiCaso(ctx, pool, paefi, autor); err != nil {
		t.Fatal(err)
	}

	// MSE: medida concluída dentro da competência.
	mse, err := domain.NovaMseMedida(cid, un, "", "", "ato teste", domain.MseTipoLiberdadeAssistida, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.InsertMseMedida(ctx, pool, mse, autor); err != nil {
		t.Fatal(err)
	}
	if err := mse.Concluir(domain.MseStatusCumprida, autor, dentro); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateMseMedida(ctx, pool, mse); err != nil {
		t.Fatal(err)
	}

	// Conselho Tutelar: atendimento registrado dentro da competência.
	conselho, err := domain.NovoConselhoAtendimento(cid, un, "", "", "", "escola", "evasao_escolar", "")
	if err != nil {
		t.Fatal(err)
	}
	conselho.CreatedAt = dentro
	if err := r.InsertConselhoAtendimento(ctx, pool, conselho, autor); err != nil {
		t.Fatal(err)
	}

	// Notificação do CREAS: recebida e respondida dentro da competência.
	notif, err := domain.NovaNotificacaoCreas(cid, un, "ministerio_publico", "requisição teste", nil)
	if err != nil {
		t.Fatal(err)
	}
	notif.DataEntrada = dentro
	if err := r.InsertNotificacaoCreas(ctx, pool, notif, autor); err != nil {
		t.Fatal(err)
	}
	if err := notif.Transitar("responder", "resposta teste", autor, dentro); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateNotificacaoCreas(ctx, pool, notif); err != nil {
		t.Fatal(err)
	}

	// Casa Abrigo: acolhimento ativo com entrada dentro da competência.
	acolh := domain.Acolhimento{ID: uuid.New(), CidadaoID: cid, UnidadeID: un, Status: domain.AcolhimentoStatusAcolhido,
		AcolhimentoDados: domain.AcolhimentoDados{Sexo: "masculino", Motivos: []string{"outro"}, OutroMotivo: "teste"}, DataEntrada: dentro}
	if err := r.InsertAcolhimento(ctx, pool, acolh, autor); err != nil {
		t.Fatal(err)
	}

	// Encaminhamento enviado dentro da competência.
	if err := r.InsertEncaminhamento(ctx, pool, domain.Encaminhamento{ID: uuid.New(), CidadaoID: cid, UnidadeOrigemID: un,
		UnidadeDestinoID: unOutra, Motivo: "motivo teste", Status: domain.EncaminhamentoStatusPendente, CreatedAt: dentro}, autor); err != nil {
		t.Fatal(err)
	}

	rma, err := r.Rma(ctx, pool, []domain.Unidade{{ID: un}}, inicio, fim)
	if err != nil || len(rma) != 1 {
		t.Fatalf("Rma: %+v, %v", rma, err)
	}
	m := rma[0]
	switch {
	case m.Atendimentos != 1 || m.AtendimentosSociais != 1 || m.AtendimentosPsicologicos != 0:
		t.Errorf("atendimentos (só o de dentro da competência): %+v", m)
	case m.PaifAtivos != 1 || m.PaifExtremaPobreza != 1 || m.PaifBolsaFamilia != 1 || m.PaifSCFVParticipantes != 3:
		t.Errorf("PAIF (fotografia, ignora a competência): %+v", m)
	case m.BeneficiosConcedidosNoMes != 1:
		t.Errorf("benefícios concedidos no mês: %+v", m)
	case m.PaefiCasosAtivos != 1 || m.PaefiCasosAbertosNoMes != 1:
		t.Errorf("PAEFI: %+v", m)
	case m.MseAtivas != 0 || m.MseConcluidasNoMes != 1:
		t.Errorf("MSE: %+v", m)
	case m.ConselhoAtendimentosNoMes != 1:
		t.Errorf("Conselho Tutelar: %+v", m)
	case m.CreasNotificacoesRecebidasNoMes != 1 || m.CreasNotificacoesRespondidasNoMes != 1:
		t.Errorf("notificações do CREAS: %+v", m)
	case m.CasaAbrigoAcolhidosAtivos != 1 || m.CasaAbrigoEntradasNoMes != 1 || m.CasaAbrigoDesligamentosNoMes != 0:
		t.Errorf("Casa Abrigo: %+v", m)
	case m.EncaminhamentosEnviadosNoMes != 1:
		t.Errorf("encaminhamentos: %+v", m)
	}

	// Unidade sem nenhum movimento sai zerada, não ausente.
	semMovimento, err := r.Rma(ctx, pool, []domain.Unidade{{ID: unOutra}}, inicio, fim)
	if err != nil || len(semMovimento) != 1 || semMovimento[0].Atendimentos != 0 {
		t.Fatalf("Rma unidade sem movimento: %+v, %v", semMovimento, err)
	}
}

// Round-trip do perfil de rua do Centro POP: ainda não preenchido vira
// ErrNotFound; upsert grava e atualiza (um só por cidadão).
func TestCentroPopPerfilRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)

	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Cidadão não identificado", Anonimo: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	if _, err := r.GetCentroPopPerfil(ctx, pool, cid); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("perfil ainda não preenchido deveria ser ErrNotFound: %v", err)
	}

	tempo := 1.5
	p, err := domain.NovoCentroPopPerfil(cid, "situacao_rua", &tempo, "desemprego", false, "", false, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	p.AtualizadoEm = time.Now()
	if err := r.UpsertCentroPopPerfil(ctx, pool, p, autor); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetCentroPopPerfil(ctx, pool, cid)
	if err != nil || got.Situacao != "situacao_rua" || *got.TempoRuaAnos != 1.5 {
		t.Fatalf("GetCentroPopPerfil: %+v, %v", got, err)
	}

	// upsert de novo (atualização) continua um só registro.
	p2, err := domain.NovoCentroPopPerfil(cid, "albergado", nil, "desemprego", true, "diabetes", false, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	p2.AtualizadoEm = time.Now()
	if err := r.UpsertCentroPopPerfil(ctx, pool, p2, autor); err != nil {
		t.Fatal(err)
	}
	atualizado, err := r.GetCentroPopPerfil(ctx, pool, cid)
	if err != nil || atualizado.Situacao != "albergado" || atualizado.TempoRuaAnos != nil || !atualizado.PossuiDoenca {
		t.Fatalf("GetCentroPopPerfil após atualizar: %+v, %v", atualizado, err)
	}
}

// Round-trip do encaminhamento externo: grava, lista na área do
// cidadão e muda de status (sem trava de "já decidido").
func TestEncaminhamentoExternoRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un, unOutra uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-outra-"+sfx).Scan(&unOutra); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Cidadão da Rua",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	e, err := domain.NovoEncaminhamentoExterno(cid, un, "caps_ad", "alta", "uso abusivo de substâncias")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.InsertEncaminhamentoExterno(ctx, pool, e, autor); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetEncaminhamentoExterno(ctx, pool, e.ID, false)
	if err != nil || got.Status != domain.EncaminhamentoExternoStatusPendente || got.Destino != "caps_ad" {
		t.Fatalf("GetEncaminhamentoExterno: %+v, %v", got, err)
	}

	lista, err := r.EncaminhamentosExternosNaArea(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{un}})
	if err != nil || len(lista) != 1 || lista[0].ID != e.ID {
		t.Fatalf("EncaminhamentosExternosNaArea: %+v, %v", lista, err)
	}
	foraDaArea, err := r.EncaminhamentosExternosNaArea(ctx, pool, cid, domain.Area{Unidades: []uuid.UUID{unOutra}})
	if err != nil || len(foraDaArea) != 0 {
		t.Fatalf("EncaminhamentosExternosNaArea fora da área: %+v, %v", foraDaArea, err)
	}

	if err := got.Transitar(domain.EncaminhamentoExternoStatusConcluido, "atendido", autor, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateEncaminhamentoExterno(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	concluido, err := r.GetEncaminhamentoExterno(ctx, pool, e.ID, false)
	if err != nil || concluido.Status != domain.EncaminhamentoExternoStatusConcluido {
		t.Fatalf("GetEncaminhamentoExterno após concluir: %+v, %v", concluido, err)
	}
}

// Round-trip dos documentos do Centro POP: mesmo padrão dos documentos
// da Casa Abrigo, mas chaveado por cidadão (não por acolhimento).
func TestDocumentoCentroPopRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	cid := uuid.New()
	if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: "Cidadão da Rua",
		DataNascimento: "1990-01-01", NomeMae: "Mãe Teste", CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
		t.Fatal(err)
	}

	d, err := domain.NovoDocumentoCentroPop(cid, un, domain.DocumentoCentroPopLaudoSaude, "laudo.pdf",
		"sempras/centro-pop/"+cid.String()+"/2026/10/abc.pdf", "application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	d.CreatedAt = time.Now()
	if err := r.InsertDocumentoCentroPop(ctx, pool, d, autor); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetDocumentoCentroPop(ctx, pool, d.ID)
	if err != nil || got.Status != domain.DocumentoStatusPending {
		t.Fatalf("GetDocumentoCentroPop: %+v, %v", got, err)
	}

	if err := r.UpdateDocumentoCentroPopStatus(ctx, pool, d.ID, domain.DocumentoStatusReady, 2048, "application/pdf"); err != nil {
		t.Fatal(err)
	}
	confirmado, err := r.GetDocumentoCentroPop(ctx, pool, d.ID)
	if err != nil || confirmado.Status != domain.DocumentoStatusReady || confirmado.SizeBytes != 2048 {
		t.Fatalf("GetDocumentoCentroPop após confirmar: %+v, %v", confirmado, err)
	}

	documentos, err := r.ListDocumentosCentroPop(ctx, pool, cid)
	if err != nil || len(documentos) != 1 || documentos[0].ID != d.ID {
		t.Fatalf("ListDocumentosCentroPop: %+v, %v", documentos, err)
	}

	if err := r.DeleteDocumentoCentroPop(ctx, pool, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetDocumentoCentroPop(ctx, pool, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("documento deveria ter sido removido: %v", err)
	}
}

// rmaUnidade cria uma entidade com uma unidade e um munícipe nela.
func rmaUnidade(t *testing.T, pool *pgxpool.Pool, r *Repository, autor uuid.UUID, nomes ...string) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	sfx := uuid.NewString()[:8]
	var ent, un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{}
	for _, nome := range nomes {
		cid := uuid.New()
		if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: cid, Nome: nome, DataNascimento: "1990-01-01", NomeMae: "Mãe",
			CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, cid)
	}
	return un, ids
}

// TestRmaPorTipo cobre os indicadores do RMA do CREAS, do Centro POP e da
// Casa da Mulher (ADR 031, fase E), as demandas da recepção e a regressão
// dos ofícios e coletivos (o acumulador antigo os zerava).
func TestRmaPorTipo(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	un, cids := rmaUnidade(t, pool, r, autor, "Antiga", "Nova")
	antiga, nova := cids[0], cids[1]

	inicio := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	fim := inicio.AddDate(0, 1, 0)
	dentro := inicio.AddDate(0, 0, 10)
	antes := inicio.AddDate(0, -2, 0)
	falhar := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Recepção / Centro POP: a "antiga" já veio antes; a "nova", só neste mês.
	for i, a := range []struct {
		cid     uuid.UUID
		dia     string
		quando  time.Time
		demanda string
	}{
		{antiga, "2026-08-01", antes, "alimentacao_higiene"},
		{antiga, "2026-10-11", dentro, "alimentacao_higiene"},
		{nova, "2026-10-11", dentro, "banho_lavanderia"},
	} {
		falhar(r.InsertAtendimento(ctx, pool, domain.Atendimento{ID: uuid.New(), CidadaoID: a.cid, UnidadeID: un, Dia: a.dia,
			Senha: i + 1, Tipo: domain.TipoSocial, Demanda: a.demanda, Status: domain.StatusAguardando, CreatedAt: a.quando}))
	}
	perfil, err := domain.NovoCentroPopPerfil(antiga, "trecheiro", nil, "outro", false, "", false, "", true, "álcool")
	falhar(err)
	falhar(r.UpsertCentroPopPerfil(ctx, pool, perfil, autor))
	perfil, err = domain.NovoCentroPopPerfil(nova, "situacao_rua", nil, "outro", true, "diabetes", false, "", false, "")
	falhar(err)
	falhar(r.UpsertCentroPopPerfil(ctx, pool, perfil, autor))
	ext, err := domain.NovoEncaminhamentoExterno(nova, un, "caps_ad", "alta", "acompanhamento em saúde mental")
	falhar(err)
	ext.CreatedAt = dentro
	falhar(r.InsertEncaminhamentoExterno(ctx, pool, ext, autor))

	// PAEFI: um caso em acompanhamento desde antes; um aberto e desligado no mês.
	velho, err := domain.NovoPaefiCaso(antiga, un, []string{"negligencia"}, "", "")
	falhar(err)
	velho.DataAbertura = antes
	falhar(r.InsertPaefiCaso(ctx, pool, velho, autor))
	novo, err := domain.NovoPaefiCaso(nova, un, []string{"violencia_fisica", "violencia_sexual"}, "", "")
	falhar(err)
	novo.DataAbertura = dentro
	falhar(r.InsertPaefiCaso(ctx, pool, novo, autor))
	falhar(novo.Encerrar("objetivos_alcancados", autor, dentro.Add(time.Hour)))
	falhar(r.UpdatePaefiCaso(ctx, pool, novo))

	// MSE: LA nova no mês; PSC iniciada antes, ainda em cumprimento.
	la, err := domain.NovaMseMedida(nova, un, "1/2026", "Vara", "furto", domain.MseTipoLiberdadeAssistida, "")
	falhar(err)
	la.DataInicio = dentro
	falhar(r.InsertMseMedida(ctx, pool, la, autor))
	psc, err := domain.NovaMseMedida(antiga, un, "2/2026", "Vara", "dano", domain.MseTipoPrestacaoServicoComunidade, "")
	falhar(err)
	psc.DataInicio = antes
	falhar(r.InsertMseMedida(ctx, pool, psc, autor))

	// Casa da Mulher: caso aberto no mês, risco extremo, abrigada, medida deferida.
	caso, err := domain.NovoCasoCasaMulher(nova, un, domain.CasoCasaMulherInput{Violencias: []string{"fisica"},
		RelacaoAgressor: "conjuge_companheiro", NivelRisco: "extremo", MedidaProtetivaStatus: "deferida", Abrigada: true}, dentro)
	falhar(err)
	falhar(r.InsertCasoCasaMulher(ctx, pool, caso, autor))

	// Sexo e idade (fim do mês) das duas pessoas atendidas; abordagens.
	if _, err := pool.Exec(ctx, `UPDATE sempras_cidadaos SET sexo = 'feminino', data_nascimento = '1990-01-01' WHERE id = $1`, antiga); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sempras_cidadaos SET sexo = '', data_nascimento = NULL, nis = lpad((random() * 1e11)::bigint::text, 11, '0') WHERE id = $1`, nova); err != nil {
		t.Fatal(err)
	}
	for _, ab := range []domain.AbordagemInput{
		{CidadaoID: &antiga, Local: "Praça Brasil", Resultado: "aceitou_acolhimento", DataAbordagem: dentro},
		{DescricaoPessoa: "homem, uns 40 anos", Local: "Rodoviária", Resultado: "recusou", DataAbordagem: dentro},
		{CidadaoID: &antiga, Local: "Praça Brasil", Resultado: "orientado", DataAbordagem: antes},
	} {
		a, err := domain.NovaAbordagem(un, ab, time.Now())
		falhar(err)
		falhar(r.InsertAbordagem(ctx, pool, a, autor))
	}
	abordagens, err := r.ListAbordagens(ctx, pool, un, inicio, fim)
	if err != nil || len(abordagens) != 2 {
		t.Fatalf("ListAbordagens (só as do mês): %+v, %v", abordagens, err)
	}
	if lista, err := r.AbordagensNaArea(ctx, pool, antiga, domain.Area{Unidades: []uuid.UUID{un}}); err != nil || len(lista) != 2 {
		t.Fatalf("AbordagensNaArea: %+v, %v", lista, err)
	}

	// Regressão: ofício recebido e atendimento coletivo no mês.
	oficio, err := domain.NovoOficioCras(antiga, un, "Ministério Público", "10/2026", nil, "requisição de relatório")
	falhar(err)
	oficio.DataRecebimento = dentro
	falhar(r.InsertOficioCras(ctx, pool, oficio, autor))
	coletivo, err := domain.NovoAtendimentoColetivo(un, dentro, domain.CategoriasColetivo[0], 7, "")
	falhar(err)
	falhar(r.InsertAtendimentoColetivo(ctx, pool, coletivo, autor))

	rma, err := r.Rma(ctx, pool, []domain.Unidade{{ID: un}}, inicio, fim)
	if err != nil || len(rma) != 1 {
		t.Fatalf("Rma: %+v, %v", rma, err)
	}
	m := rma[0]
	switch {
	case m.OficiosCrasRecebidosNoMes != 1 || m.ColetivoTotalNoMes != 7:
		t.Errorf("ofícios e coletivos não podem sair zerados: %+v", m)
	case m.AtendimentosPorDemanda["alimentacao_higiene"] != 1 || m.AtendimentosPorDemanda["banho_lavanderia"] != 1 || len(m.AtendimentosPorDemanda) != 2:
		t.Errorf("demandas do mês: %+v", m.AtendimentosPorDemanda)
	case m.PopPessoasAtendidasNoMes != 2 || m.PopPessoasNovasNoMes != 1 || m.PopPessoasUsoDrogas != 1 ||
		m.PopPessoasComDoenca != 1 || m.PopPessoasTrecheiras != 1 || m.EncaminhamentosExternosNoMes != 1:
		t.Errorf("Centro POP: %+v", m)
	case m.PaefiCasosInicioMes != 1 || m.PaefiDesligadosNoMes != 1 || m.PaefiCasosAbertosNoMes != 1 ||
		m.PaefiNovosPorViolacao["violencia_fisica"] != 1 || m.PaefiNovosPorViolacao["violencia_sexual"] != 1 ||
		m.PaefiNovosPorViolacao["negligencia"] != 0:
		t.Errorf("PAEFI: %+v", m)
	case m.MseLaAtivas != 1 || m.MsePscAtivas != 1 || m.MseNovasLaNoMes != 1 || m.MseNovasPscNoMes != 0:
		t.Errorf("MSE: %+v", m)
	case m.CasaMulherCasosAtivos != 1 || m.CasaMulherCasosNovosNoMes != 1 || m.CasaMulherRiscoAltoExtremo != 1 ||
		m.CasaMulherAbrigadas != 1 || m.CasaMulherMedidasDeferidas != 1:
		t.Errorf("Casa da Mulher: %+v", m)
	case m.AbordagensNoMes != 2 || m.AbordagensAceitouAcolhimento != 1 || m.AbordagensRecusas != 1 || m.AbordagensPessoasIdentific != 1:
		t.Errorf("abordagens: %+v", m)
	case m.PessoasPorSexoFaixa["feminino|30-59"] != 1 || m.PessoasPorSexoFaixa["|"] != 1 || len(m.PessoasPorSexoFaixa) != 2:
		t.Errorf("pessoas por sexo e faixa: %+v", m.PessoasPorSexoFaixa)
	}

	// Sem movimento: mapas vazios, não nulos.
	vazia, _ := rmaUnidade(t, pool, r, autor)
	sem, err := r.Rma(ctx, pool, []domain.Unidade{{ID: vazia}}, inicio, fim)
	if err != nil || sem[0].AtendimentosPorDemanda == nil || sem[0].PaefiNovosPorViolacao == nil || sem[0].PessoasPorSexoFaixa == nil {
		t.Fatalf("Rma sem movimento: %+v, %v", sem, err)
	}
}

// TestRmaFalhaEmCadaConsulta: o RMA é uma sequência de consultas; a falha
// (consulta, linha ilegível ou conexão perdida) em qualquer uma delas sobe.
func TestRmaFalhaEmCadaConsulta(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	unidades := []domain.Unidade{{ID: uuid.New()}}
	falhas := []dbtest.QueryResult{{Err: dbtest.ErrInjected}, {Rows: dbtest.BadRows()}, {Rows: dbtest.ErrRows()}}
	for _, falha := range falhas {
		n := 0
		for ; ; n++ {
			seq := &dbtest.Seq{}
			for range n {
				seq.Queries = append(seq.Queries, dbtest.QueryResult{Rows: dbtest.NoRows()})
			}
			// A falha se repete dali em diante (Seq repete a última resposta) —
			// recria as linhas a cada consulta para não reaproveitar o cursor.
			seq.Queries = append(seq.Queries, falha)
			if _, err := r.Rma(ctx, seq, unidades, time.Now(), time.Now()); err == nil {
				break // passou de todas as consultas sem falhar: acabaram
			}
			if falha.Rows != nil {
				falha = dbtest.QueryResult{Rows: novaLinhaRuim(falha)}
			}
		}
		// 17 consultas de contadores + 3 por chave (demandas, sexo/faixa, violações).
		if n != 20 {
			t.Errorf("o RMA falhou em %d consultas; esperava 20 (%+v)", n, falha)
		}
	}
}

// novaLinhaRuim recria o resultado com defeito (cada cursor só é lido uma vez).
func novaLinhaRuim(f dbtest.QueryResult) pgx.Rows {
	if f.Rows.Err() != nil {
		return dbtest.ErrRows()
	}
	return dbtest.BadRows()
}

// Round-trip do caso da Casa da Mulher: grava, um ativo por mulher na
// unidade, reavalia, lista na área, encerra.
func TestCasoCasaMulherRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	un, cids := rmaUnidade(t, pool, r, autor, "Maria")
	agora := time.Now()
	in := domain.CasoCasaMulherInput{Violencias: []string{"psicologica", "moral"}, RelacaoAgressor: "ex_namorado",
		NivelRisco: "medio", MedidaProtetivaStatus: "nao_solicitada", Filhos: 1}
	c, err := domain.NovoCasoCasaMulher(cids[0], un, in, agora)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.InsertCasoCasaMulher(ctx, pool, c, autor); err != nil {
		t.Fatal(err)
	}
	outro, _ := domain.NovoCasoCasaMulher(cids[0], un, in, agora)
	if err := r.InsertCasoCasaMulher(ctx, pool, outro, autor); !errors.Is(err, domain.ErrCasaMulherJaAtivo) {
		t.Fatalf("segundo caso ativo: %v", err)
	}
	got, err := r.GetCasoCasaMulher(ctx, pool, c.ID, false)
	if err != nil || got.CasoCodigo != c.CasoCodigo || len(got.Violencias) != 2 || got.Filhos != 1 {
		t.Fatalf("GetCasoCasaMulher: %+v, %v", got, err)
	}
	if _, err := r.GetCasoCasaMulher(ctx, pool, uuid.New(), false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("inexistente: %v", err)
	}
	in.Abrigada, in.NivelRisco = true, "alto"
	if err := got.Atualizar(in, autor, agora); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateCasoCasaMulher(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	lista, err := r.CasosCasaMulherNaArea(ctx, pool, cids[0], domain.Area{Unidades: []uuid.UUID{un}})
	if err != nil || len(lista) != 1 || !lista[0].Abrigada || lista[0].DataAbrigamento == nil || lista[0].NivelRisco != "alto" {
		t.Fatalf("CasosCasaMulherNaArea: %+v, %v", lista, err)
	}
	if lista, err := r.CasosCasaMulherNaArea(ctx, pool, cids[0], domain.Area{}); err != nil || len(lista) != 0 {
		t.Fatalf("fora da área: %+v, %v", lista, err)
	}
}

// O tipo é gravado, trocado e tirado (a unidade volta a não atender).
func TestConfigurarUnidadeRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	un, _ := rmaUnidade(t, pool, r, dbtest.User(t, pool))
	for _, c := range []struct {
		tipo   string
		painel bool
	}{{domain.UnidadeCras, true}, {domain.UnidadeCreas, false}, {"", false}} {
		if err := r.ConfigurarUnidade(ctx, pool, un, c.tipo, c.painel); err != nil {
			t.Fatal(err)
		}
		u, err := r.GetUnidade(ctx, pool, un)
		if err != nil || u.Tipo != c.tipo || u.PainelExibeNome != c.painel {
			t.Fatalf("depois de configurar %q: %+v, %v", c.tipo, u, err)
		}
	}
}
