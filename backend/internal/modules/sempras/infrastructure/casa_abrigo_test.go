package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
)

func diaTeste(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d
}

func pdia(s string) *time.Time { d := diaTeste(s); return &d }

// Round-trip da Casa Abrigo (ADR 032) no banco de verdade: o assistente
// inteiro ida e volta, R3 (um acolhimento ativo por criança), as listas da
// unidade (institucionalizados, visitas, termos, evoluções, metas), o
// diário, os relatórios e as tarefas.
func TestCasaAbrigoRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	autor := dbtest.User(t, pool)
	sfx := uuid.NewString()[:8]

	var ent, un, unOutra uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	for _, u := range []*uuid.UUID{&un, &unOutra} {
		if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`,
			ent, "un-"+uuid.NewString()[:8]).Scan(u); err != nil {
			t.Fatal(err)
		}
	}
	cidadao := func(nome, nasc string) uuid.UUID {
		id := uuid.New()
		if err := r.InsertCidadao(ctx, pool, domain.Cidadao{ID: id, Nome: nome, DataNascimento: nasc, NomeMae: "Mãe",
			CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
			t.Fatal(err)
		}
		return id
	}
	bruna, caio, adulto := cidadao("Bruna Acolhida", "2015-03-01"), cidadao("Caio Acolhido", "2012-06-10"), cidadao("Adulto", "1980-01-01")

	// o assistente inteiro ida e volta
	dados := domain.AcolhimentoDados{Sexo: "feminino", SituacaoEspecial: true, PCD: true, Apelido: "Bru", IdentidadeGenero: "cisgenero",
		RacaCor: "parda", Filiacao1: "Mãe", ResponsavelNome: "Tia", ResponsavelCPF: "52998224725",
		ResponsavelNascimento: pdia("1985-02-03"), ResponsavelSexo: "feminino", ResponsavelVinculo: "tia",
		ResponsavelCEP: "78700000", ResponsavelLogradouro: "Rua A", ResponsavelUF: "MT", ResponsavelEmail: "tia@ex.com",
		ProcessoJudicial: "123", DataGuia: pdia("2026-09-01"), HistoricoAcolhidoAnteriormente: "nao_sei",
		Motivos: []string{"negligencia", "outro"}, OutroMotivo: "detalhe", SituacaoJuridica: "processo_guarda",
		DataSituacaoJuridica: pdia("2026-09-02"), CNA: "nao", LaudoMedico: "sim", LaudoMedicoDescricao: "laudo",
		Estuda: "sim", RelacaoTrabalho: "nao", Observacoes: "obs"}
	a, err := domain.NovoAcolhimento(bruna, un, "2015-03-01", dados, diaTeste("2026-09-01"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.InsertAcolhimento(ctx, pool, a, autor); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetAcolhimento(ctx, pool, a.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Apelido != "Bru" || got.ResponsavelCPF != "52998224725" || !got.ResponsavelNascimento.Equal(diaTeste("1985-02-03")) ||
		got.HistoricoAcolhidoAnteriormente != "nao_sei" || got.SituacaoJuridica != "processo_guarda" || len(got.Motivos) != 2 ||
		got.OutroMotivo != "detalhe" || !got.DataGuia.Equal(diaTeste("2026-09-01")) || !got.PCD || got.Status != domain.AcolhimentoStatusAcolhido {
		t.Fatalf("assistente ida e volta: %+v", got)
	}

	// R3: um acolhimento ativo por criança, em qualquer unidade
	outro, _ := domain.NovoAcolhimento(bruna, unOutra, "2015-03-01", dados, diaTeste("2026-09-05"), time.Now())
	if err := r.InsertAcolhimento(ctx, pool, outro, autor); !errors.Is(err, domain.ErrAcolhimentoAtivo) {
		t.Fatalf("segundo acolhimento ativo: %v", err)
	}

	// desliga e reabre noutro acolhimento (o índice só conta os ativos)
	if err := got.Desligar(domain.Desligamento{Tipo: "reintegracao_familia_extensa", Motivo: "retorno à tia", Guia: "GD-1",
		DataSaida: diaTeste("2026-10-01"), DataGuia: pdia("2026-10-02")}, autor, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateAcolhimento(ctx, pool, got); err != nil {
		t.Fatal(err)
	}
	desligado, err := r.GetAcolhimento(ctx, pool, a.ID, false)
	if err != nil || desligado.DesligamentoTipo != "reintegracao_familia_extensa" || desligado.GuiaDesligamento != "GD-1" ||
		!desligado.DataGuiaDesligamento.Equal(diaTeste("2026-10-02")) || desligado.DesligadoPor == nil {
		t.Fatalf("desligado: %+v %v", desligado, err)
	}
	ativo, err := domain.NovoAcolhimento(caio, un, "2012-06-10", domain.AcolhimentoDados{Sexo: "masculino", Motivos: []string{"abandono"}},
		diaTeste("2026-08-01"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.InsertAcolhimento(ctx, pool, ativo, autor); err != nil {
		t.Fatal(err)
	}
	if lista, err := r.AcolhimentosNaArea(ctx, pool, bruna, domain.Area{Unidades: []uuid.UUID{un}}); err != nil || len(lista) != 1 {
		t.Fatalf("AcolhimentosNaArea: %+v %v", lista, err)
	}

	// irmãos e diagnósticos
	irmao, _ := domain.NovoIrmao(ativo.ID, "Irmã do Caio", nil, false, "", "")
	if err := r.InsertIrmao(ctx, pool, irmao); err != nil {
		t.Fatal(err)
	}
	if l, err := r.ListIrmaos(ctx, pool, ativo.ID); err != nil || len(l) != 1 {
		t.Fatalf("ListIrmaos: %+v %v", l, err)
	}
	if i, err := r.GetIrmao(ctx, pool, irmao.ID); err != nil || i.Nome != "Irmã do Caio" {
		t.Fatalf("GetIrmao: %+v %v", i, err)
	}
	if err := r.DeleteIrmao(ctx, pool, irmao.ID); err != nil {
		t.Fatal(err)
	}
	diag, _ := domain.NovoDiagnostico(ativo.ID, "doenca", "Diabetes", "")
	if err := r.InsertDiagnostico(ctx, pool, diag); err != nil {
		t.Fatal(err)
	}
	if l, err := r.ListDiagnosticos(ctx, pool, ativo.ID); err != nil || len(l) != 1 {
		t.Fatalf("ListDiagnosticos: %+v %v", l, err)
	}
	if d, err := r.GetDiagnostico(ctx, pool, diag.ID); err != nil || d.Diagnostico != "Diabetes" {
		t.Fatalf("GetDiagnostico: %+v %v", d, err)
	}
	if err := r.DeleteDiagnostico(ctx, pool, diag.ID); err != nil {
		t.Fatal(err)
	}

	// visitas: ao acolhido e à instituição (sem acolhido); saída depois
	visita, _ := domain.NovoVisitante(un, domain.VisitanteDados{AcolhimentoID: &ativo.ID, NomeVisitante: "Avó", DataVisita: diaTeste("2026-10-10"),
		HoraEntrada: "14:00"})
	doacao, _ := domain.NovoVisitante(un, domain.VisitanteDados{TipoVisita: "doacao", NomeVisitante: "Igreja", DataVisita: diaTeste("2026-10-11")})
	for _, v := range []domain.Visitante{visita, doacao} {
		v.CreatedAt = time.Now()
		if err := r.InsertVisitante(ctx, pool, v, autor); err != nil {
			t.Fatal(err)
		}
	}
	if err := visita.RegistrarSaida("15:30"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateVisitante(ctx, pool, visita); err != nil {
		t.Fatal(err)
	}
	if v, err := r.GetVisitante(ctx, pool, visita.ID); err != nil || v.HoraSaida != "15:30" || v.HoraEntrada != "14:00" || *v.AcolhimentoID != ativo.ID {
		t.Fatalf("GetVisitante: %+v %v", v, err)
	}
	if l, err := r.ListVisitantes(ctx, pool, ativo.ID); err != nil || len(l) != 1 {
		t.Fatalf("ListVisitantes: %+v %v", l, err)
	}
	vis, err := r.ListVisitantesDaUnidade(ctx, pool, un)
	if err != nil || len(vis) != 2 || vis[0].AcolhidoNome != "" || vis[1].AcolhidoNome != "Caio Acolhido" {
		t.Fatalf("ListVisitantesDaUnidade: %+v %v", vis, err)
	}
	if err := r.DeleteVisitante(ctx, pool, doacao.ID); err != nil {
		t.Fatal(err)
	}

	// termos
	termo, _ := domain.NovoTermo(ativo.ID, "passeio", "Avó", "avó", "parque", diaTeste("2026-10-12"))
	termo.CreatedAt = time.Now()
	if err := r.InsertTermo(ctx, pool, termo, autor); err != nil {
		t.Fatal(err)
	}
	if l, err := r.ListTermos(ctx, pool, ativo.ID); err != nil || len(l) != 1 {
		t.Fatalf("ListTermos: %+v %v", l, err)
	}
	if l, err := r.ListTermosDaUnidade(ctx, pool, un); err != nil || len(l) != 1 || l[0].AcolhidoNome != "Caio Acolhido" {
		t.Fatalf("ListTermosDaUnidade: %+v %v", l, err)
	}
	if tm, err := r.GetTermo(ctx, pool, termo.ID); err != nil || tm.TipoTermo != "passeio" {
		t.Fatalf("GetTermo: %+v %v", tm, err)
	}
	if err := r.DeleteTermo(ctx, pool, termo.ID); err != nil {
		t.Fatal(err)
	}

	// PIA: grava, regrava (upsert) e lê as 6 abas
	pia, _ := domain.NovaPiaFicha(ativo.ID, domain.PiaFichaDados{Escola: "EMEB", PeriodoEscolar: "matutino", DocNIS: true})
	pia.AtualizadoEm = time.Now()
	if err := r.UpsertPiaFicha(ctx, pool, pia, autor); err != nil {
		t.Fatal(err)
	}
	pia.MoradiaAnterior, pia.IntervencaoObjetivos = "cedida", "vínculos"
	if err := r.UpsertPiaFicha(ctx, pool, pia, autor); err != nil {
		t.Fatal(err)
	}
	if p, err := r.GetPiaFicha(ctx, pool, ativo.ID); err != nil || p.Escola != "EMEB" || !p.DocNIS || p.MoradiaAnterior != "cedida" ||
		p.IntervencaoObjetivos != "vínculos" || p.AtualizadoPor == nil {
		t.Fatalf("GetPiaFicha: %+v %v", p, err)
	}

	// metas: só as abertas, com prazo, de acolhidos ativos vão para o monitor
	prazo := diaTeste("2026-10-20")
	meta, _ := domain.NovaPiaMeta(ativo.ID, domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: "matrícula", Prazo: &prazo, Esportes: "futebol"})
	metaDesligado, _ := domain.NovaPiaMeta(a.ID, domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: "do desligado", Prazo: &prazo})
	for _, m := range []domain.PiaMeta{meta, metaDesligado} {
		m.CreatedAt = time.Now()
		if err := r.InsertPiaMeta(ctx, pool, m, autor); err != nil {
			t.Fatal(err)
		}
	}
	if err := meta.Editar(domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: "matrícula escolar", Prazo: &prazo, Lazer: "praça"}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePiaMeta(ctx, pool, meta); err != nil {
		t.Fatal(err)
	}
	if m, err := r.GetPiaMeta(ctx, pool, meta.ID); err != nil || m.Descricao != "matrícula escolar" || m.Lazer != "praça" || m.Esportes != "" {
		t.Fatalf("GetPiaMeta: %+v %v", m, err)
	}
	if l, err := r.ListPiaMetas(ctx, pool, ativo.ID); err != nil || len(l) != 1 {
		t.Fatalf("ListPiaMetas: %+v %v", l, err)
	}
	pend, err := r.MetasPendentesDaUnidade(ctx, pool, un)
	if err != nil || len(pend) != 1 || pend[0].ID != meta.ID || pend[0].AcolhidoNome != "Caio Acolhido" {
		t.Fatalf("MetasPendentesDaUnidade: %+v %v", pend, err)
	}
	if err := r.DeletePiaMeta(ctx, pool, metaDesligado.ID); err != nil {
		t.Fatal(err)
	}

	// documentos: o pronto acende o indicador do relatório de institucionalizados
	doc, _ := domain.NovoDocumentoCasaAbrigo(ativo.ID, domain.DocumentoTipoAcolhido, "certidao.pdf", "k-"+sfx, "application/pdf")
	doc.CreatedAt = time.Now()
	if err := r.InsertDocumento(ctx, pool, doc, autor); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateDocumentoStatus(ctx, pool, doc.ID, domain.DocumentoStatusReady, 10, "application/pdf"); err != nil {
		t.Fatal(err)
	}
	if d, err := r.GetDocumento(ctx, pool, doc.ID); err != nil || d.Status != domain.DocumentoStatusReady {
		t.Fatalf("GetDocumento: %+v %v", d, err)
	}
	if l, err := r.ListDocumentos(ctx, pool, ativo.ID); err != nil || len(l) != 1 {
		t.Fatalf("ListDocumentos: %+v %v", l, err)
	}

	todos, err := r.AcolhidosDaUnidade(ctx, pool, un)
	if err != nil || len(todos) != 2 {
		t.Fatalf("AcolhidosDaUnidade: %+v %v", todos, err)
	}
	if todos[0].AcolhimentoID != ativo.ID || todos[0].Nome != "Caio Acolhido" || todos[0].DataNascimento != "2012-06-10" ||
		!todos[0].DocumentoAcolhido || todos[0].DocumentoResponsavel || todos[0].PiaAtualizadoEm == nil {
		t.Errorf("ativo primeiro: %+v", todos[0])
	}
	if todos[1].Status != domain.AcolhimentoStatusDesligado || todos[1].DesligamentoTipo != "reintegracao_familia_extensa" ||
		todos[1].DataDesligamento == nil {
		t.Errorf("desligado depois: %+v", todos[1])
	}
	if err := r.DeleteDocumento(ctx, pool, doc.ID); err != nil {
		t.Fatal(err)
	}

	// evoluções: só as dos acolhidos da unidade, registradas nela
	for _, c := range []uuid.UUID{caio, adulto} {
		reg, _ := domain.NovoRegistro(c, un, nil, domain.TipoSocial, "evolução de "+c.String()[:4], autor)
		reg.CreatedAt = time.Now()
		if err := r.InsertRegistro(ctx, pool, reg); err != nil {
			t.Fatal(err)
		}
	}
	evo, err := r.EvolucoesDaUnidade(ctx, pool, un)
	if err != nil || len(evo) != 1 || evo[0].AcolhidoNome != "Caio Acolhido" || evo[0].CidadaoID != caio {
		t.Fatalf("EvolucoesDaUnidade: %+v %v", evo, err)
	}

	// diário: geral e de um acolhido, filtrado pelo mês
	for _, an := range []domain.AnotacaoDiario{
		{ID: uuid.New(), UnidadeID: un, DataReferencia: diaTeste("2026-10-05"), Texto: "rotina da casa", AutorNome: "Ana"},
		{ID: uuid.New(), UnidadeID: un, AcolhimentoID: &ativo.ID, DataReferencia: diaTeste("2026-10-06"), Texto: "consulta", AutorNome: "Ana"},
		{ID: uuid.New(), UnidadeID: un, DataReferencia: diaTeste("2026-11-01"), Texto: "fora do mês", AutorNome: "Ana"},
	} {
		an.CreatedAt = time.Now()
		if err := r.InsertAnotacaoDiario(ctx, pool, an, autor); err != nil {
			t.Fatal(err)
		}
	}
	diario, err := r.ListDiario(ctx, pool, un, diaTeste("2026-10-01"), diaTeste("2026-11-01"))
	if err != nil || len(diario) != 2 || diario[0].AcolhidoNome != "Caio Acolhido" || diario[1].AcolhidoNome != "" {
		t.Fatalf("ListDiario: %+v %v", diario, err)
	}

	// relatórios escritos
	rel, _ := domain.NovoRelatorio(un, domain.RelatorioDados{AcolhimentoID: &ativo.ID, Tipo: "psicossocial", Status: "em_elaboracao",
		DataEntrega: diaTeste("2026-10-25")})
	rel.CreatedAt, rel.AtualizadoEm = time.Now(), time.Now()
	if err := r.InsertRelatorio(ctx, pool, rel, autor); err != nil {
		t.Fatal(err)
	}
	if err := rel.Atualizar(domain.RelatorioDados{Tipo: "outro", TipoOutro: "ao MP", Status: "revisao", DataEntrega: diaTeste("2026-10-26"),
		Conteudo: "texto"}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateRelatorio(ctx, pool, rel); err != nil {
		t.Fatal(err)
	}
	if g, err := r.GetRelatorio(ctx, pool, rel.ID, true); err != nil || g.TipoOutro != "ao MP" || g.Conteudo != "texto" || g.AcolhimentoID != nil {
		t.Fatalf("GetRelatorio: %+v %v", g, err)
	}
	if l, err := r.ListRelatorios(ctx, pool, un); err != nil || len(l) != 1 {
		t.Fatalf("ListRelatorios: %+v %v", l, err)
	}
	if err := r.DeleteRelatorio(ctx, pool, rel.ID); err != nil {
		t.Fatal(err)
	}

	// tarefas
	tf, _ := domain.NovaTarefa(un, domain.TarefaDados{Titulo: "Comprar material", Prazo: &prazo})
	tf.CreatedAt, tf.AtualizadoEm = time.Now(), time.Now()
	if err := r.InsertTarefa(ctx, pool, tf, autor); err != nil {
		t.Fatal(err)
	}
	if err := tf.Atualizar(domain.TarefaDados{Titulo: "Comprar material", Status: "doing", Prioridade: "alta", Ordem: 3}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateTarefa(ctx, pool, tf); err != nil {
		t.Fatal(err)
	}
	if g, err := r.GetTarefa(ctx, pool, tf.ID, true); err != nil || g.Status != "doing" || g.Ordem != 3 || g.Prazo != nil {
		t.Fatalf("GetTarefa: %+v %v", g, err)
	}
	if l, err := r.ListTarefas(ctx, pool, un); err != nil || len(l) != 1 {
		t.Fatalf("ListTarefas: %+v %v", l, err)
	}
	if err := r.DeleteTarefa(ctx, pool, tf.ID); err != nil {
		t.Fatal(err)
	}
}
