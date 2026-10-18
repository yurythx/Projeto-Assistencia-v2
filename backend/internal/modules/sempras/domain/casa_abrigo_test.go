package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// dia é uma data "pura", como o navegador manda (meia-noite UTC).
func dia(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

func ptr[T any](v T) *T { return &v }

// dadosValidos é o mínimo que o assistente aceita.
func dadosValidos() AcolhimentoDados {
	return AcolhimentoDados{Sexo: "feminino", Motivos: []string{"negligencia"}}
}

func TestAcolhimentoDadosNormalizar(t *testing.T) {
	L := strings.Repeat("x", 5000)
	recusa := map[string]func(*AcolhimentoDados){
		"texto longo":              func(d *AcolhimentoDados) { d.Apelido = L },
		"sexo":                     func(d *AcolhimentoDados) { d.Sexo = "" },
		"raça/cor":                 func(d *AcolhimentoDados) { d.RacaCor = "azul" },
		"histórico":                func(d *AcolhimentoDados) { d.HistoricoIrmaosNestaUnidade = "talvez" },
		"sem motivo":               func(d *AcolhimentoDados) { d.Motivos = nil },
		"motivo inválido":          func(d *AcolhimentoDados) { d.Motivos = []string{"motivo_invalido"} },
		"outro sem detalhe":        func(d *AcolhimentoDados) { d.Motivos = []string{"outro"} },
		"detalhe sem outro":        func(d *AcolhimentoDados) { d.OutroMotivo = "algo" },
		"CPF do responsável":       func(d *AcolhimentoDados) { d.ResponsavelCPF = "111.111.111-11" },
		"NIS do responsável":       func(d *AcolhimentoDados) { d.ResponsavelNIS = "123" },
		"CEP":                      func(d *AcolhimentoDados) { d.ResponsavelCEP = "7870" },
		"UF com 3 letras":          func(d *AcolhimentoDados) { d.ResponsavelUF = "mtx" },
		"UF com dígito":            func(d *AcolhimentoDados) { d.ResponsavelUF = "M1" },
		"e-mail inválido":          func(d *AcolhimentoDados) { d.ResponsavelEmail = "sem-arroba" },
		"e-mail com nome exibido":  func(d *AcolhimentoDados) { d.ResponsavelEmail = "Ana <ana@ex.com>" },
		"relação de trabalho":      func(d *AcolhimentoDados) { d.RelacaoTrabalho = "estagio" },
		"situação jurídica livre":  func(d *AcolhimentoDados) { d.SituacaoJuridica = "texto livre" },
		"vínculo fora da lista":    func(d *AcolhimentoDados) { d.ResponsavelVinculo = "vizinho" },
		"identidade de gênero":     func(d *AcolhimentoDados) { d.IdentidadeGenero = "x" },
		"orientação sexual":        func(d *AcolhimentoDados) { d.OrientacaoSexual = "x" },
		"laudo com resposta livre": func(d *AcolhimentoDados) { d.LaudoMedico = "talvez" },
	}
	for nome, mudar := range recusa {
		d := dadosValidos()
		mudar(&d)
		var v *ValidationError
		if _, err := d.Normalizar(); !errors.As(err, &v) {
			t.Errorf("%s: deveria recusar, veio %v", nome, err)
		}
	}

	d := dadosValidos()
	d.Motivos = []string{"violencia_fisica", "outro", "negligencia", "outro"}
	d.OutroMotivo = "  motivo   específico "
	d.ResponsavelCPF, d.ResponsavelNIS, d.ResponsavelCEP = "529.982.247-25", "123.45678.90-1", "78700-000"
	d.ResponsavelUF, d.ResponsavelEmail = " mt ", " mae@exemplo.com "
	d.LaudoMedico, d.LaudoMedicoDescricao = "nao", "apagado"
	d.PossuiTranstorno, d.TranstornoDescricao = "", "apagado"
	d.BeneficioGoverno, d.BeneficioQual = "nao", "apagado"
	n, err := d.Normalizar()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(n.Motivos, ",") != "negligencia,outro,violencia_fisica" || n.OutroMotivo != "motivo específico" {
		t.Errorf("motivos: %v / %q", n.Motivos, n.OutroMotivo)
	}
	if n.ResponsavelCPF != "52998224725" || n.ResponsavelNIS != "12345678901" || n.ResponsavelCEP != "78700000" ||
		n.ResponsavelUF != "MT" || n.ResponsavelEmail != "mae@exemplo.com" {
		t.Errorf("responsável: %+v", n)
	}
	if n.LaudoMedicoDescricao != "" || n.TranstornoDescricao != "" || n.BeneficioQual != "" {
		t.Errorf("R4: descrição sem \"sim\" deveria sair: %+v", n)
	}
	if &n.Motivos[0] == &d.Motivos[0] {
		t.Error("Normalizar não pode alterar o slice de quem chamou")
	}

	// "sim" guarda a descrição; "não possui responsável" limpa o responsável (R4)
	d = dadosValidos()
	d.LaudoMedico, d.LaudoMedicoDescricao = "sim", "laudo de TEA"
	d.PossuiTranstorno, d.TranstornoDescricao = "sim", "TEA nível 1"
	d.BeneficioGoverno, d.BeneficioQual = "sim", "BPC"
	d.ResponsavelNaoPossui, d.ResponsavelNome, d.ResponsavelCPF = true, "Maria", "111"
	d.ResponsavelNascimento, d.ResponsavelUF, d.ResponsavelEmail = ptr(dia("1980-01-01")), "XX9", "não é e-mail"
	if n, err = d.Normalizar(); err != nil {
		t.Fatal(err)
	}
	if n.LaudoMedicoDescricao == "" || n.TranstornoDescricao == "" || n.BeneficioQual != "BPC" {
		t.Errorf("com \"sim\" a descrição fica: %+v", n)
	}
	if n.ResponsavelNome != "" || n.ResponsavelCPF != "" || n.ResponsavelNascimento != nil || n.ResponsavelUF != "" {
		t.Errorf("sem responsável, os campos saem: %+v", n)
	}
}

func TestIdadeEm(t *testing.T) {
	if _, ok := IdadeEm("", dia("2026-10-17")); ok {
		t.Error("sem nascimento não há idade")
	}
	if i, _ := IdadeEm("2014-10-18", dia("2026-10-17")); i != 11 {
		t.Errorf("véspera do aniversário: %d", i)
	}
	if i, _ := IdadeEm("2014-10-17", dia("2026-10-17")); i != 12 {
		t.Errorf("no aniversário: %d", i)
	}
	if i, _ := IdadeEm("2014-11-01", dia("2026-10-17")); i != 11 {
		t.Errorf("mês do aniversário ainda não chegou: %d", i)
	}
	if i, _ := IdadeEm("2030-01-01", dia("2026-10-17")); i != 0 {
		t.Errorf("nunca negativa: %d", i)
	}
}

func TestAcolhimentoCicloDeVida(t *testing.T) {
	crianca, unidade, tecnico := uuid.New(), uuid.New(), uuid.New()
	hoje := dia("2026-10-17").Add(15 * time.Hour)

	// R1/R2
	casos := map[string]struct {
		nascimento string
		entrada    time.Time
	}{
		"entrada futura":             {"2015-01-01", dia("2026-10-30")},
		"sem nascimento no cadastro": {"", dia("2026-10-01")},
		"entrada antes de nascer":    {"2015-01-01", dia("2014-12-31")},
		"maior de idade na entrada":  {"2008-01-01", dia("2026-10-01")},
	}
	for nome, c := range casos {
		if _, err := NovoAcolhimento(crianca, unidade, c.nascimento, dadosValidos(), c.entrada, hoje); err == nil {
			t.Errorf("%s: deveria recusar", nome)
		}
	}
	if _, err := NovoAcolhimento(crianca, unidade, "2015-01-01", AcolhimentoDados{}, dia("2026-10-01"), hoje); err == nil {
		t.Error("assistente inválido")
	}
	a, err := NovoAcolhimento(crianca, unidade, "2015-01-01", dadosValidos(), dia("2026-10-01"), hoje)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != AcolhimentoStatusAcolhido || a.Sexo != "feminino" {
		t.Errorf("acolhimento: %+v", a)
	}

	// edição: assistente, data e a mesma regra de entrada
	if err := a.Atualizar("2015-01-01", AcolhimentoDados{}, dia("2026-10-01"), hoje); err == nil {
		t.Error("edição com assistente inválido")
	}
	if err := a.Atualizar("2015-01-01", dadosValidos(), dia("2027-01-01"), hoje); err == nil {
		t.Error("edição com entrada futura")
	}
	d := dadosValidos()
	d.Observacoes = "atualizado"
	if err := a.Atualizar("2015-01-01", d, dia("2026-09-20"), hoje); err != nil {
		t.Fatal(err)
	}
	if a.Observacoes != "atualizado" || !a.DataEntrada.Equal(dia("2026-09-20")) || a.Status != AcolhimentoStatusAcolhido {
		t.Errorf("atualizado: %+v", a)
	}

	// R5: desligamento
	desl := Desligamento{Tipo: "adocao", Motivo: "sentença de adoção", DataSaida: dia("2026-10-10"), Guia: "G-1"}
	for nome, mudar := range map[string]func(*Desligamento){
		"tipo":             func(d *Desligamento) { d.Tipo = "fuga" },
		"motivo":           func(d *Desligamento) { d.Motivo = "x" },
		"guia":             func(d *Desligamento) { d.Guia = strings.Repeat("g", 201) },
		"saída futura":     func(d *Desligamento) { d.DataSaida = dia("2026-12-01") },
		"saída na entrada": func(d *Desligamento) { d.DataSaida = dia("2026-09-19") },
	} {
		x := desl
		mudar(&x)
		if err := a.Desligar(x, tecnico, hoje); err == nil {
			t.Errorf("desligamento (%s): deveria recusar", nome)
		}
	}
	if err := a.Desligar(desl, tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if a.Status != AcolhimentoStatusDesligado || a.DesligamentoTipo != "adocao" || a.GuiaDesligamento != "G-1" ||
		a.DesligadoPor == nil || !a.DataDesligamento.Equal(dia("2026-10-10")) {
		t.Errorf("desligado: %+v", a)
	}
	if err := a.Desligar(desl, tecnico, hoje); !errors.Is(err, ErrAcolhimentoEstado) {
		t.Errorf("desligar de novo: %v", err)
	}
	if err := a.Atualizar("2015-01-01", dadosValidos(), dia("2026-10-15"), hoje); err == nil {
		t.Error("entrada depois do desligamento")
	}
	if err := a.Atualizar("2015-01-01", dadosValidos(), dia("2026-10-05"), hoje); err != nil || a.Status != AcolhimentoStatusDesligado {
		t.Errorf("editar o desligado não reabre: %v %s", err, a.Status)
	}
}

func TestIrmaoEDiagnostico(t *testing.T) {
	ac := uuid.New()
	if _, err := NovoIrmao(ac, "A", nil, false, "", ""); err == nil {
		t.Error("nome curto")
	}
	i, err := NovoIrmao(ac, " Irmão  da Criança ", nil, true, "Abrigo X", "")
	if err != nil || i.Nome != "Irmão da Criança" || !i.EstaAcolhido {
		t.Errorf("irmão: %+v %v", i, err)
	}
	if _, err := NovoDiagnostico(ac, "categoria", "Diabetes", ""); err == nil {
		t.Error("categoria inválida")
	}
	if _, err := NovoDiagnostico(ac, "doenca", "x", ""); err == nil {
		t.Error("diagnóstico curto")
	}
	if d, err := NovoDiagnostico(ac, "transtorno", "TEA - Autismo (Grau 1)", "suporte leve"); err != nil || d.Categoria != "transtorno" {
		t.Errorf("diagnóstico: %+v %v", d, err)
	}
}

func TestVisitante(t *testing.T) {
	unidade, ac := uuid.New(), uuid.New()
	base := VisitanteDados{AcolhimentoID: &ac, NomeVisitante: "Maria Visitante", DataVisita: dia("2026-10-17"), HoraEntrada: "14:00"}
	for nome, mudar := range map[string]func(*VisitanteDados){
		"tipo":                        func(d *VisitanteDados) { d.TipoVisita = "festa" },
		"visita a acolhido sem ele":   func(d *VisitanteDados) { d.AcolhimentoID = nil },
		"nome":                        func(d *VisitanteDados) { d.NomeVisitante = "A" },
		"hora de entrada":             func(d *VisitanteDados) { d.HoraEntrada = "1400" },
		"hora de saída":               func(d *VisitanteDados) { d.HoraSaida = "24:00" },
		"saída antes da entrada":      func(d *VisitanteDados) { d.HoraSaida = "13:00" },
		"minutos":                     func(d *VisitanteDados) { d.HoraEntrada = "10:60" },
		"sem dois-pontos":             func(d *VisitanteDados) { d.HoraEntrada = "10-00" },
		"letra na hora":               func(d *VisitanteDados) { d.HoraEntrada = "a0:00" },
		"letra na hora (2º dígito)":   func(d *VisitanteDados) { d.HoraEntrada = "1a:00" },
		"letra no minuto":             func(d *VisitanteDados) { d.HoraEntrada = "10:a0" },
		"letra no minuto (2º dígito)": func(d *VisitanteDados) { d.HoraEntrada = "10:0a" },
	} {
		d := base
		mudar(&d)
		if _, err := NovoVisitante(unidade, d); err == nil {
			t.Errorf("visitante (%s): deveria recusar", nome)
		}
	}
	v, err := NovoVisitante(unidade, base)
	if err != nil || v.TipoVisita != "acolhido" || v.UnidadeID != unidade {
		t.Fatalf("visita: %+v %v", v, err)
	}
	// doação: sem acolhido
	if d, err := NovoVisitante(unidade, VisitanteDados{TipoVisita: "doacao", NomeVisitante: "Igreja", DataVisita: dia("2026-10-17")}); err != nil || d.AcolhimentoID != nil {
		t.Errorf("doação: %+v %v", d, err)
	}

	if err := v.RegistrarSaida("1600"); err == nil {
		t.Error("saída mal formatada")
	}
	if err := v.RegistrarSaida(""); err == nil {
		t.Error("saída vazia")
	}
	if err := v.RegistrarSaida("13:59"); err == nil {
		t.Error("saída antes da entrada")
	}
	if err := v.RegistrarSaida("16:30"); err != nil || v.HoraSaida != "16:30" {
		t.Fatalf("saída: %v %+v", err, v)
	}
	if err := v.RegistrarSaida("17:00"); !errors.Is(err, ErrVisitaEncerrada) {
		t.Errorf("saída de novo: %v", err)
	}
	semEntrada := Visitante{}
	if err := semEntrada.RegistrarSaida("08:00"); err != nil {
		t.Errorf("sem hora de entrada, qualquer saída: %v", err)
	}
}

func TestTermo(t *testing.T) {
	ac, hoje := uuid.New(), dia("2026-10-17")
	if _, err := NovoTermo(ac, "tipo", "", "", "", hoje); err == nil {
		t.Error("tipo inválido")
	}
	if _, err := NovoTermo(ac, "passeio", strings.Repeat("x", 201), "", "", hoje); err == nil {
		t.Error("responsável longo")
	}
	if tm, err := NovoTermo(ac, "passeio", "Tia Maria", "Tia", "parque", hoje); err != nil || tm.TipoTermo != "passeio" {
		t.Errorf("termo: %+v %v", tm, err)
	}
}

func TestPia(t *testing.T) {
	ac := uuid.New()
	if _, err := NovaPiaFicha(ac, PiaFichaDados{Escola: strings.Repeat("x", 201)}); err == nil {
		t.Error("escola longa")
	}
	if _, err := NovaPiaFicha(ac, PiaFichaDados{PeriodoEscolar: "madrugada"}); err == nil {
		t.Error("período inválido")
	}
	p, err := NovaPiaFicha(ac, PiaFichaDados{Escola: " Escola  Municipal ", PeriodoEscolar: "matutino", MoradiaAnterior: "cedida",
		DocNIS: true, IntervencaoObjetivos: "fortalecer vínculos"})
	if err != nil || p.Escola != "Escola Municipal" || !p.DocNIS {
		t.Fatalf("PIA: %+v %v", p, err)
	}

	// R7: próxima reavaliação
	entrada := dia("2026-07-01")
	if got := ProximaReavaliacaoPia(entrada, nil); !got.Equal(time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)) {
		t.Errorf("sem PIA conta da entrada: %v", got)
	}
	gravado := time.Date(2026, 8, 10, 15, 0, 0, 0, time.Local)
	if got := ProximaReavaliacaoPia(entrada, &gravado); !got.Equal(time.Date(2026, 11, 8, 0, 0, 0, 0, time.Local)) {
		t.Errorf("com PIA conta da gravação: %v", got)
	}

	// metas
	if _, err := NovaPiaMeta(ac, PiaMetaDados{Tipo: "x", Descricao: "documentação"}); err == nil {
		t.Error("tipo de meta")
	}
	if _, err := NovaPiaMeta(ac, PiaMetaDados{Tipo: PiaMetaPlano, Descricao: "x"}); err == nil {
		t.Error("descrição curta")
	}
	m, err := NovaPiaMeta(ac, PiaMetaDados{Tipo: PiaMetaPlano, Descricao: "Documentação (RG/CPF)", Esportes: "futebol", Lazer: "parque"})
	if err != nil || m.Esportes != "futebol" || m.Lazer != "parque" {
		t.Fatalf("meta: %+v %v", m, err)
	}
	if err := m.Editar(PiaMetaDados{Tipo: PiaMetaPlano, Descricao: "x"}); err == nil {
		t.Error("edição inválida")
	}
	if err := m.Editar(PiaMetaDados{Tipo: PiaMetaIntervencao, Descricao: "visitas quinzenais", Cultura: "teatro"}); err != nil ||
		m.Tipo != PiaMetaIntervencao || m.Cultura != "teatro" || m.Esportes != "" {
		t.Errorf("meta editada: %+v %v", m, err)
	}
}

func TestDocumentoCasaAbrigo(t *testing.T) {
	ac := uuid.New()
	if _, err := NovoDocumentoCasaAbrigo(ac, "tipo", "doc.pdf", "k", "application/pdf"); err == nil {
		t.Error("tipo inválido")
	}
	if _, err := NovoDocumentoCasaAbrigo(ac, DocumentoTipoAcolhido, "", "k", "application/pdf"); err == nil {
		t.Error("nome vazio")
	}
	if d, err := NovoDocumentoCasaAbrigo(ac, DocumentoTipoResponsavel, "doc.pdf", "k", "application/pdf"); err != nil || d.Status != DocumentoStatusPending {
		t.Errorf("documento: %+v %v", d, err)
	}
}

func TestDiario(t *testing.T) {
	u := uuid.New()
	if _, err := NovaAnotacaoDiario(u, nil, dia("2026-10-17"), "ok", "Ana"); err == nil {
		t.Error("anotação curta")
	}
	if a, err := NovaAnotacaoDiario(u, nil, dia("2026-10-20"), "consulta médica às 9h", "Ana"); err != nil || a.AutorNome != "Ana" {
		t.Errorf("anotação: %+v %v", a, err)
	}
}

func TestRelatorio(t *testing.T) {
	u := uuid.New()
	base := RelatorioDados{Tipo: "mensal", Status: "em_elaboracao", DataEntrega: dia("2026-10-30")}
	for nome, mudar := range map[string]func(*RelatorioDados){
		"tipo":             func(d *RelatorioDados) { d.Tipo = "semanal" },
		"etapa":            func(d *RelatorioDados) { d.Status = "arquivado" },
		"outro sem nome":   func(d *RelatorioDados) { d.Tipo = "outro" },
		"nome longo":       func(d *RelatorioDados) { d.Tipo, d.TipoOutro = "outro", strings.Repeat("n", 201) },
		"conteúdo longo":   func(d *RelatorioDados) { d.Conteudo = strings.Repeat("c", 100001) },
		"sem data entrega": func(d *RelatorioDados) { d.DataEntrega = time.Time{} },
	} {
		d := base
		mudar(&d)
		if _, err := NovoRelatorio(u, d); err == nil {
			t.Errorf("relatório (%s): deveria recusar", nome)
		}
	}
	d := base
	d.TipoOutro = "ignorado fora de \"outro\""
	r, err := NovoRelatorio(u, d)
	if err != nil || r.TipoOutro != "" {
		t.Fatalf("relatório: %+v %v", r, err)
	}
	if err := r.PodeExcluir(); err != nil {
		t.Error(err)
	}
	if err := r.Atualizar(RelatorioDados{}); err == nil {
		t.Error("edição inválida")
	}
	d = base
	d.Tipo, d.TipoOutro, d.Status, d.Conteudo = "outro", "Relatório ao MP", RelatorioEntregue, "texto"
	if err := r.Atualizar(d); err != nil || r.Status != RelatorioEntregue || r.TipoOutro != "Relatório ao MP" {
		t.Fatalf("entregue: %+v %v", r, err)
	}
	// R8: o entregue não muda mais
	if err := r.Atualizar(base); !errors.Is(err, ErrRelatorioEntregue) {
		t.Errorf("editar entregue: %v", err)
	}
	if err := r.PodeExcluir(); !errors.Is(err, ErrRelatorioEntregue) {
		t.Errorf("excluir entregue: %v", err)
	}
}

func TestTarefa(t *testing.T) {
	u := uuid.New()
	for nome, d := range map[string]TarefaDados{
		"coluna":     {Titulo: "Relatório", Status: "arquivo"},
		"prioridade": {Titulo: "Relatório", Prioridade: "urgente"},
		"título":     {Titulo: "R"},
		"descrição":  {Titulo: "Relatório", Descricao: strings.Repeat("d", 4001)},
		"ordem":      {Titulo: "Relatório", Ordem: -1},
	} {
		if _, err := NovaTarefa(u, d); err == nil {
			t.Errorf("tarefa (%s): deveria recusar", nome)
		}
	}
	tf, err := NovaTarefa(u, TarefaDados{Titulo: " Finalizar  relatório ", Descricao: "  detalhes  "})
	if err != nil || tf.Status != "backlog" || tf.Prioridade != "media" || tf.Titulo != "Finalizar relatório" || tf.Descricao != "detalhes" {
		t.Fatalf("tarefa: %+v %v", tf, err)
	}
	if err := tf.Atualizar(TarefaDados{Titulo: "x"}); err == nil {
		t.Error("edição inválida")
	}
	if err := tf.Atualizar(TarefaDados{Titulo: "Finalizar relatório", Status: "doing", Prioridade: "alta", Ordem: 2}); err != nil ||
		tf.Status != "doing" || tf.Ordem != 2 {
		t.Errorf("tarefa movida: %+v %v", tf, err)
	}
}

func TestIniciais(t *testing.T) {
	if got := Iniciais("ana maria da silva e souza"); got != "A. M. S. S." {
		t.Errorf("iniciais: %q", got)
	}
	if got := Iniciais("  "); got != "N/I" {
		t.Errorf("vazio: %q", got)
	}
	if got := (AcolhidoResumo{Nome: "Ana", NomeSocial: "Aninha"}).NomeExibicao(); got != "Aninha" {
		t.Errorf("nome social: %q", got)
	}
	if got := (AcolhidoResumo{Nome: "Ana"}).NomeExibicao(); got != "Ana" {
		t.Errorf("nome civil: %q", got)
	}
}

func TestMontarPainel(t *testing.T) {
	hoje := time.Date(2026, 10, 17, 10, 0, 0, 0, time.Local)
	pia := time.Date(2026, 7, 20, 0, 0, 0, 0, time.Local) // reavaliação em 18/10: vence amanhã
	adolescente := AcolhidoResumo{AcolhimentoID: uuid.New(), Nome: "Bruno Lima", DataNascimento: "2012-01-01", Status: "acolhido",
		DataEntrada: dia("2026-07-01"), PiaAtualizadoEm: &pia}
	crianca := AcolhidoResumo{AcolhimentoID: uuid.New(), Nome: "Ana Souza", NomeSocial: "Aninha", DataNascimento: "2020-05-05",
		Status: "acolhido", DataEntrada: dia("2026-10-01")} // reavaliação em 30/12: fora da janela
	desligado := AcolhidoResumo{AcolhimentoID: uuid.New(), Nome: "Caio", DataNascimento: "2015-01-01", Status: "desligado",
		DataEntrada: dia("2026-01-01")}
	vencida := dia("2026-10-10")
	logo := dia("2026-10-20")
	longe := dia("2026-12-20")
	metas := []MetaPendente{
		{PiaMeta: PiaMeta{ID: uuid.New(), AcolhimentoID: adolescente.AcolhimentoID, Descricao: "matrícula", Prazo: &vencida}, AcolhidoNome: "Bruno Lima"},
		{PiaMeta: PiaMeta{ID: uuid.New(), Descricao: "sem prazo"}},
		{PiaMeta: PiaMeta{ID: uuid.New(), Descricao: "concluída", Prazo: &vencida, Concluida: true}},
		{PiaMeta: PiaMeta{ID: uuid.New(), Descricao: "longe", Prazo: &longe}},
	}
	relatorios := []RelatorioCasaAbrigo{
		{ID: uuid.New(), Tipo: "mensal", Status: "revisao", DataEntrega: logo},
		{ID: uuid.New(), Tipo: "outro", TipoOutro: "ao MP", Status: "em_elaboracao", DataEntrega: vencida},
		{ID: uuid.New(), Tipo: "anual", Status: RelatorioEntregue, DataEntrega: vencida},
	}
	tarefas := []TarefaCasaAbrigo{
		{ID: uuid.New(), Titulo: "comprar material", Status: "todo", Prazo: &logo},
		{ID: uuid.New(), Titulo: "feita", Status: TarefaConcluida, Prazo: &vencida},
		{ID: uuid.New(), Titulo: "sem prazo", Status: "todo"},
		{ID: uuid.New(), Titulo: "longe", Status: "todo", Prazo: &longe},
	}
	p := MontarPainel(hoje, []AcolhidoResumo{adolescente, crianca, desligado}, metas, relatorios, tarefas)
	if p.Ativos != 2 || p.Criancas != 1 || p.Adolescentes != 1 || p.Desacolhidos != 1 || p.PiasVencendo != 1 {
		t.Errorf("cartões: %+v", p)
	}
	if len(p.Acolhidos) != 2 || len(p.Desligados) != 1 || p.Acolhidos[0].Idade != 14 {
		t.Errorf("listas: %+v", p.Acolhidos)
	}
	tipos := []string{}
	for _, a := range p.Alertas {
		tipos = append(tipos, a.Tipo+"/"+a.Nivel)
	}
	// ordenados pelo prazo: atrasados primeiro
	if strings.Join(tipos, ",") != "meta/atraso,relatorio/atraso,pia/aviso,relatorio/aviso,tarefa/aviso" {
		t.Errorf("alertas: %v", tipos)
	}
	if !strings.Contains(p.Alertas[0].Mensagem, "B. L.") || !strings.Contains(p.Alertas[1].Mensagem, "ao MP") ||
		p.Alertas[3].Mensagem != "Relatório mensal a entregar" {
		t.Errorf("mensagens: %+v", p.Alertas)
	}
	vazio := MontarPainel(hoje, nil, nil, nil, nil)
	if vazio.Acolhidos == nil || vazio.Desligados == nil || vazio.Alertas == nil {
		t.Error("listas vazias saem como [] no JSON")
	}
}

func TestEstatisticas(t *testing.T) {
	saida := func(s string) *time.Time { d := dia(s); return &d }
	todos := []AcolhidoResumo{
		{DataNascimento: "2020-01-01", Sexo: "feminino", DataEntrada: dia("2026-07-10"), Motivos: []string{"negligencia", "abandono"}},
		{DataNascimento: "2012-01-01", Sexo: "masculino", DataEntrada: dia("2026-08-01"), Motivos: []string{"negligencia"},
			DataDesligamento: saida("2026-09-01"), DesligamentoTipo: "reintegracao_familia_natural"},
		{DataNascimento: "2012-01-01", DataEntrada: dia("2026-01-01"), DataDesligamento: saida("2026-08-15"), DesligamentoTipo: "adocao"},
		{DataNascimento: "2012-01-01", DataEntrada: dia("2026-01-01"), DataDesligamento: saida("2026-08-16"), DesligamentoTipo: "transferencia"},
		{DataNascimento: "2012-01-01", DataEntrada: dia("2026-01-01"), DataDesligamento: saida("2026-08-17"), DesligamentoTipo: "evasao"},
		{DataNascimento: "2010-01-01", Sexo: "masculino", DataEntrada: dia("2026-01-01")},
		{DataNascimento: "2019-01-01", DataEntrada: dia("2026-10-01")}, // entrou depois do período
	}
	if _, err := Estatisticas(todos, dia("2026-09-30"), dia("2026-07-01")); err == nil {
		t.Error("fim antes do início")
	}
	if _, err := Estatisticas(todos, dia("2025-01-01"), dia("2026-09-30")); err == nil {
		t.Error("período maior que 1 ano")
	}
	e, err := Estatisticas(todos, dia("2026-07-01"), dia("2026-09-30"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Entradas != 2 || e.Saidas != 4 || e.Reintegracoes != 1 || e.Adocoes != 1 || e.Transferencias != 1 ||
		e.SaidasPorTipo["evasao"] != 1 || e.Motivos["negligencia"] != 2 || e.Motivos["abandono"] != 1 {
		t.Errorf("movimento: %+v", e)
	}
	if e.AtivosNoFim != 2 || e.Criancas != 1 || e.Adolescentes != 1 || e.Meninas != 1 || e.Meninos != 1 {
		t.Errorf("no fim do período: %+v", e)
	}
	if e.De != "2026-07-01" || e.Ate != "2026-09-30" {
		t.Errorf("período: %s a %s", e.De, e.Ate)
	}
}

func TestLinhaDoTempo(t *testing.T) {
	d := func(s string) time.Time { return dia(s) }
	saida := d("2026-10-10")
	a := Acolhimento{DataEntrada: d("2026-09-01"), AcolhimentoDados: AcolhimentoDados{ProcessoJudicial: "123"},
		DataDesligamento: &saida, DesligamentoTipo: "adocao", MotivoDesligamento: "sentença",
		Pia: &PiaFicha{AtualizadoEm: d("2026-09-05")}, PiaMetas: []PiaMeta{{CreatedAt: d("2026-09-06"), Descricao: "RG"}},
		Visitantes: []Visitante{{DataVisita: d("2026-09-07"), NomeVisitante: "Avó", ParentescoVinculo: "avó", HoraEntrada: "14:00"}},
		Termos:     []Termo{{DataTermo: d("2026-09-08"), TipoTermo: "passeio"}},
		Documentos: []DocumentoCasaAbrigo{{CreatedAt: d("2026-09-02"), NomeOriginal: "rg.pdf"}}}
	evolucoes := []Registro{{CreatedAt: d("2026-09-03"), Tipo: TipoSocial, Texto: "chegou bem"},
		{CreatedAt: d("2026-09-04"), Tipo: TipoPsicologico, Sigiloso: true}}
	diario := []AnotacaoDiario{{DataReferencia: d("2026-09-09"), Texto: "consulta"}}
	relatorios := []RelatorioCasaAbrigo{{DataEntrega: d("2026-09-20"), Tipo: "psicossocial", Status: "entregue"}}
	ev := LinhaDoTempo(a, evolucoes, diario, relatorios)
	tipos := []string{}
	for _, e := range ev {
		tipos = append(tipos, e.Tipo)
	}
	if strings.Join(tipos, ",") != "desligamento,relatorio,diario,termo,visita,meta,pia,evolucao,evolucao,documento,entrada" {
		t.Fatalf("ordem: %v", tipos)
	}
	if ev[0].Titulo != "Desligamento (adoção)" || ev[1].Titulo != "Relatório psicossocial" || ev[3].Titulo != "Termo de passeio" ||
		ev[4].Detalhe != "avó 14:00" || ev[7].Detalhe != "Registro sigiloso da psicologia" || ev[7].Titulo != "Evolução psicológica" ||
		ev[8].Titulo != "Evolução social" || ev[10].Detalhe != "Processo 123." {
		t.Errorf("eventos: %+v", ev)
	}
	// sem desligamento, PIA, processo nem guia
	so := LinhaDoTempo(Acolhimento{DataEntrada: d("2026-09-01")}, nil, nil, nil)
	if len(so) != 1 || so[0].Detalhe != "" {
		t.Errorf("só a entrada: %+v", so)
	}
}
