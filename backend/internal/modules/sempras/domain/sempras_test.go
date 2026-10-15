package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
)

var hoje = time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)

func TestCPFValido(t *testing.T) {
	for cpf, want := range map[string]bool{
		"52998224725": true,
		"11111111111": false, // todos iguais
		"52998224715": false, // 1º dígito verificador
		"52998224726": false, // 2º dígito verificador
		"5299822472":  false, // tamanho
	} {
		if got := CPFValido(cpf); got != want {
			t.Errorf("CPFValido(%s) = %v", cpf, got)
		}
	}
	if MascararCPF("") != "" || MascararCPF("52998224725") != "***.982.247-**" {
		t.Errorf("máscara do CPF: %q", MascararCPF("52998224725"))
	}
	if MascararNIS("12") != "" || MascararNIS("12345678901") != "*******8901" {
		t.Error("máscara do NIS")
	}
	if Digitos("529.982.247-25") != "52998224725" {
		t.Error("Digitos")
	}
}

func TestNormalizarCadastro(t *testing.T) {
	ok := CidadaoInput{Nome: "  Maria   da Silva ", CPF: "529.982.247-25", Telefone: "(66) 9999"}
	c, err := ok.Normalizar(hoje)
	if err != nil || c.Nome != "Maria da Silva" || c.CPF != "52998224725" {
		t.Fatalf("cadastro válido: %+v %v", c, err)
	}
	if c, err := (CidadaoInput{Nome: "João", DataNascimento: "1990-05-01", NomeMae: "Ana"}).Normalizar(hoje); err != nil || c.DataNascimento != "1990-05-01" {
		t.Fatalf("sem documento, com nascimento e mãe: %+v %v", c, err)
	}
	if _, err := (CidadaoInput{Nome: "Zé", NIS: "123.45678.90-1"}).Normalizar(hoje); err != nil {
		t.Fatalf("NIS válido: %v", err)
	}
	if c, err := (CidadaoInput{Nome: "Cidadão não identificado", Anonimo: true}).Normalizar(hoje); err != nil || !c.Anonimo {
		t.Fatalf("cadastro anônimo (Centro POP), sem nenhuma identificação: %+v %v", c, err)
	}
	for nome, in := range map[string]CidadaoInput{
		"nome curto":        {Nome: "A", CPF: "52998224725"},
		"nome social longo": {Nome: "Maria", NomeSocial: strings.Repeat("x", 201), CPF: "52998224725"},
		"CPF inválido":      {Nome: "Maria", CPF: "123"},
		"NIS curto":         {Nome: "Maria", NIS: "123"},
		"data ilegível":     {Nome: "Maria", CPF: "52998224725", DataNascimento: "01/05/1990"},
		"data futura":       {Nome: "Maria", CPF: "52998224725", DataNascimento: "2027-01-01"},
		"data antiga":       {Nome: "Maria", CPF: "52998224725", DataNascimento: "1899-12-31"},
		"sem identificação": {Nome: "Maria", DataNascimento: "1990-05-01"},
	} {
		var v *ValidationError
		if _, err := in.Normalizar(hoje); !errors.As(err, &v) || v.Error() == "" {
			t.Errorf("%s: esperado erro de validação, veio %v", nome, err)
		}
	}
}

func TestResumoENomeSocial(t *testing.T) {
	c := Cidadao{ID: uuid.New(), Nome: "Mariana", CPF: "52998224725", NIS: "12345678901"}
	if c.NomeExibicao() != "Mariana" {
		t.Error("sem nome social, o nome civil")
	}
	c.NomeSocial = "Mari"
	if c.NomeExibicao() != "Mari" {
		t.Error("nome social tem precedência")
	}
	r := c.Resumir(3)
	if r.CPF != "***.982.247-**" || r.NIS != "*******8901" || r.AtendimentosNaArea != 3 {
		t.Errorf("resumo: %+v", r)
	}
}

func TestFilaEstados(t *testing.T) {
	autor := uuid.New()
	if _, err := NovaEntrada(uuid.New(), uuid.New(), "juridico", "", autor, "2026-10-08"); err == nil {
		t.Error("tipo inválido")
	}
	if _, err := NovaEntrada(uuid.New(), uuid.New(), TipoSocial, "vip", autor, "2026-10-08"); err == nil {
		t.Error("prioridade inválida")
	}
	a, err := NovaEntrada(uuid.New(), uuid.New(), TipoSocial, "", autor, "2026-10-08")
	if err != nil || a.Status != StatusAguardando {
		t.Fatal(err)
	}
	a.Senha = 7
	if a.SenhaExibida() != "N007" {
		t.Errorf("senha: %s", a.SenhaExibida())
	}
	a.Prioridade = "gestante"
	if a.SenhaExibida() != "P007" {
		t.Errorf("senha prioritária: %s", a.SenhaExibida())
	}

	if err := a.Transitar("iniciar", autor, "", hoje); !errors.Is(err, ErrInvalidState) {
		t.Error("não se inicia quem não foi chamado")
	}
	if err := a.Transitar("chamar", autor, " Sala 1 ", hoje); err != nil || a.Local != "Sala 1" || a.ChamadoEm == nil {
		t.Fatalf("chamar: %+v %v", a, err)
	}
	if err := a.Transitar("chamar", autor, "", hoje); err != nil || a.Local != "Sala 1" {
		t.Fatal("rechamar mantém o local")
	}
	if err := a.Transitar("iniciar", autor, "", hoje); err != nil || a.IniciadoEm == nil || a.Status != StatusEmAtendimento {
		t.Fatal("iniciar")
	}
	if err := a.Transitar("cancelar", autor, "", hoje); !errors.Is(err, ErrInvalidState) {
		t.Error("em atendimento não se cancela")
	}
	if err := a.Transitar("concluir", autor, "", hoje); err != nil || a.EncerradoEm == nil || a.Status != StatusConcluido {
		t.Fatal("concluir")
	}
	if err := a.Transitar("desconhecida", autor, "", hoje); !errors.Is(err, ErrInvalidState) {
		t.Error("ação desconhecida")
	}
}

func TestRegistroESigilo(t *testing.T) {
	autor := uuid.New()
	if _, err := NovoRegistro(uuid.New(), uuid.New(), nil, "outro", "texto", autor); err == nil {
		t.Error("tipo inválido")
	}
	if _, err := NovoRegistro(uuid.New(), uuid.New(), nil, TipoSocial, "   ", autor); err == nil {
		t.Error("texto vazio")
	}
	r, err := NovoRegistro(uuid.New(), uuid.New(), nil, TipoPsicologico, " escuta ", autor)
	if err != nil || r.Texto != "escuta" {
		t.Fatal(err)
	}
	if o := r.Ocultar(); o.Texto != "" || !o.Sigiloso || r.Texto != "escuta" {
		t.Error("ocultar não altera o original e esconde o texto")
	}
}

func TestEscopoPorUnidade(t *testing.T) {
	ent, un, outra := uuid.New(), uuid.New(), uuid.New()
	u := Unidade{ID: un, EntidadeID: ent}
	tecnico := auth.Identity{Scopes: []auth.Scope{{UnidadeID: &un, Permissions: []string{"sempras:recepcao", "sempras:atendimento"}}}}
	psicologo := auth.Identity{Scopes: []auth.Scope{{UnidadeID: &un, Permissions: []string{"sempras:*"}}}}
	superint := auth.Identity{Scopes: []auth.Scope{{EntidadeID: &ent, Permissions: []string{"sempras:atendimento", "sempras:gestao"}}}}
	admin := auth.Identity{Roles: []string{auth.RoleAdmin}}
	alheio := auth.Identity{Scopes: []auth.Scope{{UnidadeID: &outra, Permissions: []string{"sempras:*"}}}}

	if !u.Pode(tecnico, auth.PermSemprasAtendimento) || u.Pode(alheio, auth.PermSemprasAtendimento) {
		t.Error("técnico na unidade sim, de outra unidade não")
	}
	if !u.Pode(superint, auth.PermSemprasAtendimento) || !u.PodeAlguma(superint, TodasAsPermissoes...) {
		t.Error("concessão na entidade cobre a unidade sem a árvore expandida")
	}
	if !u.PodeTipo(tecnico, TipoSocial) || u.PodeTipo(tecnico, TipoPsicologico) || !u.PodeTipo(psicologo, TipoPsicologico) {
		t.Error("psicológico só com o sigilo")
	}
	if u.PodePsicologia(admin) || !u.Pode(admin, auth.PermSemprasAtendimento) {
		t.Error("administrador passa no atendimento, mas não no sigilo psicológico")
	}
	if a := AreaDe(tecnico, TodasAsPermissoes...); a.Vazia() || a.All || len(a.Unidades) == 0 {
		t.Errorf("área do técnico: %+v", a)
	}
	if a := AreaDe(superint, auth.PermSemprasGestao); len(a.Entidades) != 1 {
		t.Errorf("área do superintendente: %+v", a)
	}
	if !AreaDe(admin, auth.PermSemprasGestao).All || !AreaDe(auth.Identity{}, TodasAsPermissoes...).Vazia() {
		t.Error("admin alcança tudo; sem concessão, nada")
	}
}

func TestNovoPedido(t *testing.T) {
	cidadao, solicitante := uuid.New(), uuid.New()
	p, err := NovoPedido(cidadao, solicitante, PedidoAnonimizacao, "titular pediu para ser esquecido")
	if err != nil {
		t.Fatal(err)
	}
	if p.CidadaoID != cidadao || p.SolicitadoPor != solicitante || p.Status != PedidoStatusPendente {
		t.Errorf("pedido: %+v", p)
	}
	if _, err := NovoPedido(cidadao, solicitante, "exclusao", "justificativa válida"); err == nil {
		t.Error("tipo inválido")
	}
	if _, err := NovoPedido(cidadao, solicitante, PedidoAcesso, "oi"); err == nil {
		t.Error("justificativa curta demais")
	}
	if _, err := NovoPedido(cidadao, solicitante, PedidoAcesso, strings.Repeat("x", 1001)); err == nil {
		t.Error("justificativa longa demais")
	}
}

func TestJustificativaDecisaoValida(t *testing.T) {
	if just, err := JustificativaDecisaoValida("", false); err != nil || just != "" {
		t.Errorf("aprovar sem justificativa: %q, %v", just, err)
	}
	if _, err := JustificativaDecisaoValida("", true); err == nil {
		t.Error("negar exige justificativa")
	}
	if _, err := JustificativaDecisaoValida("oi", true); err == nil {
		t.Error("justificativa curta demais")
	}
	if just, err := JustificativaDecisaoValida("  sem   elementos  ", true); err != nil || just != "sem elementos" {
		t.Errorf("normaliza espaços: %q, %v", just, err)
	}
}

func TestNovoContatoRedeApoio(t *testing.T) {
	cidadao := uuid.New()
	c, err := NovoContatoRedeApoio(cidadao, "  Maria   da Silva ", "Irmã", "(66) 99999-0000")
	if err != nil {
		t.Fatal(err)
	}
	if c.CidadaoID != cidadao || c.Nome != "Maria da Silva" || c.Parentesco != "Irmã" || c.Telefone != "(66) 99999-0000" {
		t.Errorf("contato: %+v", c)
	}
	if _, err := NovoContatoRedeApoio(cidadao, "A", "", ""); err == nil {
		t.Error("nome curto demais")
	}
	// nome é o único obrigatório — parentesco e telefone podem faltar
	if _, err := NovoContatoRedeApoio(cidadao, "Maria", "", ""); err != nil {
		t.Errorf("parentesco e telefone opcionais: %v", err)
	}
}

func TestBeneficioEventual(t *testing.T) {
	beneficiario, solicitante, unidade, tecnico := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	if _, err := NovoBeneficio(beneficiario, nil, unidade, "casa", FormaAcessoDemandaEspontanea, 1, ""); err == nil {
		t.Error("tipo inválido")
	}
	if _, err := NovoBeneficio(beneficiario, nil, unidade, BeneficioCestaBasica, "invalida", 1, ""); err == nil {
		t.Error("forma de acesso inválida")
	}
	if _, err := NovoBeneficio(beneficiario, nil, unidade, BeneficioCestaBasica, FormaAcessoDemandaEspontanea, 0, ""); err == nil {
		t.Error("quantidade zero")
	}
	b, err := NovoBeneficio(beneficiario, &solicitante, unidade, BeneficioFuneral, FormaAcessoBuscaAtiva, 1, "urgente")
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != BeneficioStatusSolicitado || b.SolicitanteID == nil || *b.SolicitanteID != solicitante || b.FormaAcesso != FormaAcessoBuscaAtiva {
		t.Errorf("benefício: %+v", b)
	}

	// negar exige motivo
	negar := b
	if err := negar.Decidir("negar", "", tecnico, hoje); err == nil {
		t.Error("negar sem motivo")
	}
	if err := negar.Decidir("negar", "família já recebeu o auxílio este mês", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if negar.Status != BeneficioStatusNegado || negar.TecnicoDecisaoID == nil || negar.DataDecisao == nil {
		t.Errorf("negado: %+v", negar)
	}
	// decidido não decide de novo
	if err := negar.Decidir("conceder", "", tecnico, hoje); !errors.Is(err, ErrBeneficioEstado) {
		t.Errorf("já decidido: %v", err)
	}

	// conceder não exige motivo; suspender (só a partir de concedido) exige
	conceder := b
	if err := conceder.Decidir("conceder", "", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if conceder.Status != BeneficioStatusConcedido {
		t.Errorf("concedido: %+v", conceder)
	}
	if err := conceder.Decidir("suspender", "", tecnico, hoje); err == nil {
		t.Error("suspender sem motivo")
	}
	if err := conceder.Decidir("suspender", "a condição que justificava deixou de valer", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if conceder.Status != BeneficioStatusSuspenso {
		t.Errorf("suspenso: %+v", conceder)
	}
	// solicitado não suspende direto
	if err := b.Decidir("suspender", "motivo qualquer aqui", tecnico, hoje); !errors.Is(err, ErrBeneficioEstado) {
		t.Errorf("solicitado não suspende: %v", err)
	}
}

func TestEncaminhamento(t *testing.T) {
	cidadao, origem, destino, tecnico := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	if _, err := NovoEncaminhamento(cidadao, origem, origem, "motivo válido"); !errors.Is(err, ErrMesmaUnidade) {
		t.Errorf("mesma unidade: %v", err)
	}
	if _, err := NovoEncaminhamento(cidadao, origem, destino, "x"); err == nil {
		t.Error("motivo curto demais")
	}
	e, err := NovoEncaminhamento(cidadao, origem, destino, "precisa de acompanhamento especializado")
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != EncaminhamentoStatusPendente || e.UnidadeOrigemID != origem || e.UnidadeDestinoID != destino {
		t.Errorf("encaminhamento: %+v", e)
	}

	recusado := e
	if err := recusado.Decidir(false, "", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if recusado.Status != EncaminhamentoStatusRecusado || recusado.DecididoPor == nil {
		t.Errorf("recusado: %+v", recusado)
	}
	if err := recusado.Decidir(true, "", tecnico, hoje); !errors.Is(err, ErrEncaminhamentoDecidido) {
		t.Errorf("já decidido: %v", err)
	}

	aceito := e
	if err := aceito.Decidir(true, "vamos acompanhar", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if aceito.Status != EncaminhamentoStatusAceito || aceito.ObservacaoDecisao != "vamos acompanhar" {
		t.Errorf("aceito: %+v", aceito)
	}
}

func TestPaifVinculo(t *testing.T) {
	cidadao, unidade, tecnico := uuid.New(), uuid.New(), uuid.New()

	if _, err := NovoPaifVinculo(cidadao, unidade, "invalido", true, false, false, 3); err == nil {
		t.Error("tipo de atendimento inválido")
	}
	if _, err := NovoPaifVinculo(cidadao, unidade, PaifAtendimentoParticularizado, true, false, false, 51); err == nil {
		t.Error("scfv fora do intervalo")
	}
	v, err := NovoPaifVinculo(cidadao, unidade, PaifAtendimentoColetivo, true, true, false, 3)
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != PaifStatusAtivo || v.TipoAtendimento != PaifAtendimentoColetivo || !v.ExtremaPobreza || !v.BolsaFamilia || v.BPC || v.SCFVParticipantes != 3 {
		t.Errorf("vínculo: %+v", v)
	}

	if err := v.Excluir("", tecnico, hoje); err == nil {
		t.Error("motivo obrigatório")
	}
	if err := v.Excluir("família se mudou do território", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if v.Status != PaifStatusExcluido || v.ExcluidoPor == nil || v.DataExclusao == nil {
		t.Errorf("excluído: %+v", v)
	}
	if err := v.Excluir("de novo", tecnico, hoje); !errors.Is(err, ErrPaifJaExcluido) {
		t.Errorf("já excluído: %v", err)
	}
}

func TestConselhoAtendimento(t *testing.T) {
	crianca, unidade, conselheiro := uuid.New(), uuid.New(), uuid.New()

	if _, err := NovoConselhoAtendimento(crianca, unidade, "Mãe", "", "", "origem_invalida", "conflito_familiar", ""); err == nil {
		t.Error("origem inválida")
	}
	if _, err := NovoConselhoAtendimento(crianca, unidade, "Mãe", "", "", "escola", "motivo_invalido", ""); err == nil {
		t.Error("motivo inválido")
	}
	c, err := NovoConselhoAtendimento(crianca, unidade, "Mãe da Criança", "52998224725", "66999990000", "escola", "evasao_escolar", "faltou 20 dias seguidos")
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != ConselhoStatusEmAndamento || c.MedidaProtetiva != "" || !strings.HasPrefix(c.Protocolo, "CT-") {
		t.Errorf("atendimento: %+v", c)
	}
	if _, err := NovoConselhoAtendimento(crianca, unidade, "Mãe", "11111111111", "", "escola", "evasao_escolar", ""); err == nil {
		t.Error("CPF do responsável inválido")
	}

	// aplicar medida exige uma medida válida (Art. 101 ECA).
	comMedida := c
	if err := comMedida.Transitar("aplicar_medida", "medida_invalida", conselheiro, hoje); err == nil {
		t.Error("medida inválida")
	}
	if err := comMedida.Transitar("aplicar_medida", "matricula_escolar_obrigatoria", conselheiro, hoje); err != nil {
		t.Fatal(err)
	}
	if comMedida.Status != ConselhoStatusMedidaAplicada || comMedida.MedidaProtetiva != "matricula_escolar_obrigatoria" {
		t.Errorf("medida aplicada: %+v", comMedida)
	}
	// arquiva depois de aplicada a medida.
	if err := comMedida.Transitar("arquivar", "", conselheiro, hoje); err != nil {
		t.Fatal(err)
	}
	if comMedida.Status != ConselhoStatusArquivado {
		t.Errorf("arquivado: %+v", comMedida)
	}
	// não decide de novo um caso já arquivado.
	if err := comMedida.Transitar("encaminhar_mp", "", conselheiro, hoje); !errors.Is(err, ErrConselhoEstado) {
		t.Errorf("já arquivado: %v", err)
	}

	// encaminhar ao MP não exige medida.
	paraMP := c
	if err := paraMP.Transitar("encaminhar_mp", "", conselheiro, hoje); err != nil {
		t.Fatal(err)
	}
	if paraMP.Status != ConselhoStatusEncaminhadoMP || paraMP.MedidaProtetiva != "" {
		t.Errorf("encaminhado ao MP: %+v", paraMP)
	}
}

func TestPaefiCaso(t *testing.T) {
	cidadao, unidade, tecnico := uuid.New(), uuid.New(), uuid.New()

	if _, err := NovoPaefiCaso(cidadao, unidade, nil, "", ""); err == nil {
		t.Error("sem violações")
	}
	if _, err := NovoPaefiCaso(cidadao, unidade, []string{"violencia_fisica", "tipo_invalido"}, "", ""); err == nil {
		t.Error("tipo de violação inválido")
	}
	p, err := NovoPaefiCaso(cidadao, unidade, []string{"violencia_fisica", "negligencia"}, "dois filhos menores", "família acompanhada pela rede")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != PaefiStatusEmAcompanhamento || p.PiaElaborado || !strings.HasPrefix(p.CasoCodigo, "PAEFI-") {
		t.Errorf("caso: %+v", p)
	}

	if err := p.MarcarPiaElaborado(tecnico); err != nil {
		t.Fatal(err)
	}
	if !p.PiaElaborado || p.TecnicoReferenciaID == nil {
		t.Errorf("PIA elaborado: %+v", p)
	}

	if err := p.Encerrar("motivo_invalido", tecnico, hoje); err == nil {
		t.Error("motivo de encerramento inválido")
	}
	if err := p.Encerrar("objetivos_alcancados", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if p.Status != PaefiStatusDesligado || p.DataEncerramento == nil {
		t.Errorf("encerrado: %+v", p)
	}
	// não reabre nem reencerra um caso já desligado.
	if err := p.MarcarPiaElaborado(tecnico); !errors.Is(err, ErrPaefiEstado) {
		t.Errorf("já desligado (PIA): %v", err)
	}
	if err := p.Encerrar("outro", tecnico, hoje); !errors.Is(err, ErrPaefiEstado) {
		t.Errorf("já desligado (encerrar de novo): %v", err)
	}
}

func TestMseMedida(t *testing.T) {
	adolescente, unidade, tecnico := uuid.New(), uuid.New(), uuid.New()

	if _, err := NovaMseMedida(adolescente, unidade, "123/2026", "", "furto (art. 155 CP)", "tipo_invalido", ""); err == nil {
		t.Error("tipo de medida inválido")
	}
	if _, err := NovaMseMedida(adolescente, unidade, "", "", "", MseTipoLiberdadeAssistida, ""); err == nil {
		t.Error("ato infracional obrigatório")
	}
	m, err := NovaMseMedida(adolescente, unidade, "123/2026", "Vara da Infância", "furto (art. 155 CP)", MseTipoLiberdadeAssistida, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != MseStatusEmCumprimento {
		t.Errorf("medida: %+v", m)
	}

	if err := m.Concluir("resultado_invalido", tecnico, hoje); err == nil {
		t.Error("resultado inválido")
	}
	if err := m.Concluir(MseStatusCumprida, tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if m.Status != MseStatusCumprida || m.DataConclusao == nil {
		t.Errorf("concluída: %+v", m)
	}
	// não conclui de novo uma medida já concluída.
	if err := m.Concluir(MseStatusExtinta, tecnico, hoje); !errors.Is(err, ErrMseEstado) {
		t.Errorf("já concluída: %v", err)
	}
}

func TestNotificacaoCreas(t *testing.T) {
	vitima, unidade, tecnico := uuid.New(), uuid.New(), uuid.New()

	if _, err := NovaNotificacaoCreas(vitima, unidade, "origem_invalida", "ofício nº 123", nil); err == nil {
		t.Error("origem inválida")
	}
	n, err := NovaNotificacaoCreas(vitima, unidade, "ministerio_publico", "requisição de informações sobre acompanhamento", nil)
	if err != nil {
		t.Fatal(err)
	}
	if n.Status != NotificacaoStatusRecebida || !strings.HasPrefix(n.Protocolo, "OF-") {
		t.Errorf("notificação: %+v", n)
	}

	// responder exige o texto da resposta.
	comResposta := n
	if err := comResposta.Transitar("responder", "", tecnico, hoje); err == nil {
		t.Error("resposta obrigatória")
	}
	if err := comResposta.Transitar("responder", "família está em acompanhamento ativo pelo PAEFI", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if comResposta.Status != NotificacaoStatusRespondida || comResposta.DataResposta == nil {
		t.Errorf("respondida: %+v", comResposta)
	}
	// arquiva depois de respondida.
	if err := comResposta.Transitar("arquivar", "", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if comResposta.Status != NotificacaoStatusArquivada {
		t.Errorf("arquivada: %+v", comResposta)
	}
	// não decide de novo uma notificação já arquivada.
	if err := comResposta.Transitar("iniciar_diligencia", "", tecnico, hoje); !errors.Is(err, ErrNotificacaoEstado) {
		t.Errorf("já arquivada: %v", err)
	}

	// iniciar diligência não exige resposta.
	emDiligencia := n
	if err := emDiligencia.Transitar("iniciar_diligencia", "", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if emDiligencia.Status != NotificacaoStatusEmDiligencia || emDiligencia.Resposta != "" {
		t.Errorf("em diligência: %+v", emDiligencia)
	}
}

func TestAcolhimento(t *testing.T) {
	crianca, unidade, tecnico := uuid.New(), uuid.New(), uuid.New()

	if _, err := NovoAcolhimento(crianca, unidade, "outro", []string{"violencia_fisica"}, "", "", "", "", "", "", "", "", "", "", false, false, ""); err == nil {
		t.Error("sexo inválido")
	}
	if _, err := NovoAcolhimento(crianca, unidade, "masculino", nil, "", "", "", "", "", "", "", "", "", "", false, false, ""); err == nil {
		t.Error("sem motivos")
	}
	if _, err := NovoAcolhimento(crianca, unidade, "masculino", []string{"motivo_invalido"}, "", "", "", "", "", "", "", "", "", "", false, false, ""); err == nil {
		t.Error("motivo inválido")
	}
	a, err := NovoAcolhimento(crianca, unidade, "masculino", []string{"negligencia_abandono", "situacao_rua"},
		"Mãe da Criança", "", "66999990000", "mãe", "", "", "", "Conselheiro", "", "", false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != AcolhimentoStatusAcolhido || len(a.Motivos) != 2 {
		t.Errorf("acolhimento: %+v", a)
	}

	if err := a.Desligar("", tecnico, hoje); err == nil {
		t.Error("motivo de desligamento obrigatório")
	}
	if err := a.Atualizar("masculino", []string{"outro"}, "", "", "", "", "", "", "", "", "", "", false, false, "atualizado"); err != nil {
		t.Fatal(err)
	}
	if a.Observacoes != "atualizado" || a.Status != AcolhimentoStatusAcolhido {
		t.Errorf("atualizar não deve tocar status: %+v", a)
	}
	if err := a.Desligar("família reestruturada, retorno ao convívio", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if a.Status != AcolhimentoStatusDesligado || a.DataDesligamento == nil {
		t.Errorf("desligado: %+v", a)
	}
	// não desliga de novo um acolhimento já desligado.
	if err := a.Desligar("de novo", tecnico, hoje); !errors.Is(err, ErrAcolhimentoEstado) {
		t.Errorf("já desligado: %v", err)
	}
}

func TestIrmao(t *testing.T) {
	acolhimento := uuid.New()
	if _, err := NovoIrmao(acolhimento, "A", nil, false, "", ""); err == nil {
		t.Error("nome curto demais")
	}
	i, err := NovoIrmao(acolhimento, "Irmão da Criança", nil, true, "", "mesma unidade")
	if err != nil {
		t.Fatal(err)
	}
	if i.Nome != "Irmão da Criança" || !i.EstaAcolhido {
		t.Errorf("irmão: %+v", i)
	}
}

func TestDiagnostico(t *testing.T) {
	acolhimento := uuid.New()
	if _, err := NovoDiagnostico(acolhimento, "categoria_invalida", "diagnóstico", ""); err == nil {
		t.Error("categoria inválida")
	}
	if _, err := NovoDiagnostico(acolhimento, "doenca", "x", ""); err == nil {
		t.Error("diagnóstico curto demais")
	}
	d, err := NovoDiagnostico(acolhimento, "transtorno", "TEA nível 1", "suporte leve")
	if err != nil {
		t.Fatal(err)
	}
	if d.Categoria != "transtorno" || d.Diagnostico != "TEA nível 1" {
		t.Errorf("diagnóstico: %+v", d)
	}
}

func TestVisitante(t *testing.T) {
	acolhimento := uuid.New()
	if _, err := NovoVisitante(acolhimento, "A", "", "", hoje, "", "", ""); err == nil {
		t.Error("nome curto demais")
	}
	if _, err := NovoVisitante(acolhimento, "Maria Visitante", "", "", hoje, "25:00", "", ""); err == nil {
		t.Error("hora de entrada inválida")
	}
	v, err := NovoVisitante(acolhimento, "Maria Visitante", "12345678900", "Tia", hoje, "14:00", "16:30", "visita tranquila")
	if err != nil {
		t.Fatal(err)
	}
	if v.HoraEntrada != "14:00" || v.HoraSaida != "16:30" {
		t.Errorf("visitante: %+v", v)
	}
	// hora vazia é aceita (ainda não saiu).
	if _, err := NovoVisitante(acolhimento, "Maria Visitante", "", "", hoje, "14:00", "", ""); err != nil {
		t.Errorf("hora de saída vazia: %v", err)
	}
}

func TestTermo(t *testing.T) {
	acolhimento := uuid.New()
	if _, err := NovoTermo(acolhimento, "tipo_invalido", "", "", "", hoje); err == nil {
		t.Error("tipo de termo inválido")
	}
	termo, err := NovoTermo(acolhimento, "passeio", "Tia Maria", "Tia", "passeio ao parque", hoje)
	if err != nil {
		t.Fatal(err)
	}
	if termo.TipoTermo != "passeio" || termo.ResponsavelRetirada != "Tia Maria" {
		t.Errorf("termo: %+v", termo)
	}
}

func TestPiaFicha(t *testing.T) {
	acolhimento := uuid.New()
	p, err := NovaPiaFicha(acolhimento, "12345", "5º ano", "Escola Municipal", "", "", true, "123456789012345",
		"Tia Maria — 66999990000", "fins de semana", "", "café e escola", "almoço e tarefas", "jantar e TV", "passeio no parque")
	if err != nil {
		t.Fatal(err)
	}
	if !p.VacinacaoAtualizada || p.Escola != "Escola Municipal" || p.AtividadesFimSemana != "passeio no parque" {
		t.Errorf("ficha do PIA: %+v", p)
	}
}

func TestPiaMeta(t *testing.T) {
	acolhimento := uuid.New()
	if _, err := NovaPiaMeta(acolhimento, "tipo_invalido", "descrição válida", "", "", nil); err == nil {
		t.Error("tipo de meta inválido")
	}
	if _, err := NovaPiaMeta(acolhimento, PiaMetaPlano, "x", "", "", nil); err == nil {
		t.Error("descrição curta demais")
	}
	m, err := NovaPiaMeta(acolhimento, PiaMetaIntervencao, "reduzir episódios de agressividade", "atendimento psicológico semanal", "Psicóloga", nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Tipo != PiaMetaIntervencao || m.Concluida {
		t.Errorf("meta do PIA: %+v", m)
	}
}

func TestDocumentoCasaAbrigo(t *testing.T) {
	acolhimento := uuid.New()
	if _, err := NovoDocumentoCasaAbrigo(acolhimento, "tipo_invalido", "doc.pdf", "key", "application/pdf"); err == nil {
		t.Error("tipo de documento inválido")
	}
	d, err := NovoDocumentoCasaAbrigo(acolhimento, DocumentoTipoResponsavel, "doc.pdf", "sempras/casa-abrigo/x/2026/10/abc.pdf", "application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != DocumentoStatusPending || d.Tipo != DocumentoTipoResponsavel {
		t.Errorf("documento: %+v", d)
	}
}

func TestOficioCras(t *testing.T) {
	cidadao, unidade, tecnico := uuid.New(), uuid.New(), uuid.New()

	if _, err := NovoOficioCras(cidadao, unidade, "CT", "123/2026", nil, ""); err == nil {
		t.Error("órgão emissor curto demais")
	}
	if _, err := NovoOficioCras(cidadao, unidade, "Conselho Tutelar", "", nil, ""); err == nil {
		t.Error("número do ofício obrigatório")
	}
	o, err := NovoOficioCras(cidadao, unidade, "Conselho Tutelar", "123/2026", nil, "acompanhamento da família")
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != OficioStatusPendente {
		t.Errorf("ofício: %+v", o)
	}

	if err := o.Responder("", hoje, tecnico); err == nil {
		t.Error("número da resposta obrigatório")
	}
	if err := o.Responder("456/2026", hoje, tecnico); err != nil {
		t.Fatal(err)
	}
	if o.Status != OficioStatusRespondido || o.NumeroResposta != "456/2026" || o.TecnicoResponsavelID == nil {
		t.Errorf("ofício respondido: %+v", o)
	}
	if err := o.Responder("outra", hoje, tecnico); !errors.Is(err, ErrOficioRespondido) {
		t.Errorf("já respondido: %v", err)
	}
}

func TestAtendimentoColetivo(t *testing.T) {
	unidade := uuid.New()

	if _, err := NovoAtendimentoColetivo(unidade, hoje, "invalida", 10, ""); err == nil {
		t.Error("categoria inválida")
	}
	if _, err := NovoAtendimentoColetivo(unidade, hoje, ColetivoSCFV0a6, -1, ""); err == nil {
		t.Error("participantes negativo")
	}
	a, err := NovoAtendimentoColetivo(unidade, hoje, ColetivoSCFV0a6, 12, "turma da tarde")
	if err != nil {
		t.Fatal(err)
	}
	if a.Categoria != ColetivoSCFV0a6 || a.Participantes != 12 {
		t.Errorf("atendimento coletivo: %+v", a)
	}
}

func TestCentroPopPerfil(t *testing.T) {
	cidadao := uuid.New()
	tempo := 2.5

	if _, err := NovoCentroPopPerfil(cidadao, "invalida", nil, "outro", false, "", false, "", false, ""); err == nil {
		t.Error("situação inválida")
	}
	if _, err := NovoCentroPopPerfil(cidadao, SituacoesRua[0], nil, "invalido", false, "", false, "", false, ""); err == nil {
		t.Error("motivo de vulnerabilidade inválido")
	}
	menosUm := -1.0
	if _, err := NovoCentroPopPerfil(cidadao, SituacoesRua[0], &menosUm, "outro", false, "", false, "", false, ""); err == nil {
		t.Error("tempo de rua negativo")
	}
	p, err := NovoCentroPopPerfil(cidadao, "situacao_rua", &tempo, "uso_de_drogas", true, "hipertensão", true, "losartana", true, "uso de crack")
	if err != nil {
		t.Fatal(err)
	}
	if p.Situacao != "situacao_rua" || *p.TempoRuaAnos != 2.5 || !p.PossuiDoenca || !p.UsoDrogas || p.DrogasDescricao != "uso de crack" {
		t.Errorf("perfil de rua: %+v", p)
	}
}

func TestEncaminhamentoExterno(t *testing.T) {
	cidadao, unidade, tecnico := uuid.New(), uuid.New(), uuid.New()

	if _, err := NovoEncaminhamentoExterno(cidadao, unidade, "invalido", "media", "motivo válido"); err == nil {
		t.Error("destino inválido")
	}
	if _, err := NovoEncaminhamentoExterno(cidadao, unidade, "caps_ad", "invalida", "motivo válido"); err == nil {
		t.Error("prioridade inválida")
	}
	if _, err := NovoEncaminhamentoExterno(cidadao, unidade, "caps_ad", "alta", "x"); err == nil {
		t.Error("motivo curto demais")
	}
	e, err := NovoEncaminhamentoExterno(cidadao, unidade, "caps_ad", "alta", "uso abusivo de substâncias")
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != EncaminhamentoExternoStatusPendente {
		t.Errorf("encaminhamento externo: %+v", e)
	}

	if err := e.Transitar("invalido", "", tecnico, hoje); err == nil {
		t.Error("status inválido")
	}
	if err := e.Transitar(EncaminhamentoExternoStatusConcluido, "atendido pelo CAPS AD", tecnico, hoje); err != nil {
		t.Fatal(err)
	}
	if e.Status != EncaminhamentoExternoStatusConcluido || e.Observacoes != "atendido pelo CAPS AD" || e.TecnicoID == nil {
		t.Errorf("encaminhamento externo concluído: %+v", e)
	}
	// diferente de Encaminhamento (entre unidades), aqui não há trava de
	// "já decidido" — quem atende pode reabrir se precisar.
	if err := e.Transitar(EncaminhamentoExternoStatusEmAtendimento, "reaberto", tecnico, hoje); err != nil {
		t.Errorf("deveria aceitar nova transição: %v", err)
	}
}

func TestDocumentoCentroPop(t *testing.T) {
	cidadao, unidade := uuid.New(), uuid.New()

	if _, err := NovoDocumentoCentroPop(cidadao, unidade, "tipo_invalido", "doc.pdf", "key", "application/pdf"); err == nil {
		t.Error("tipo de documento inválido")
	}
	d, err := NovoDocumentoCentroPop(cidadao, unidade, DocumentoCentroPopLaudoSaude, "laudo.pdf", "sempras/centro-pop/x/2026/10/abc.pdf", "application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != DocumentoStatusPending || d.Tipo != DocumentoCentroPopLaudoSaude || d.UnidadeID != unidade {
		t.Errorf("documento centro pop: %+v", d)
	}
}
