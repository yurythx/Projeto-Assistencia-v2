package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/application"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
)

// semArea é uma identidade sem nenhuma permissão do módulo.
func semArea(c cenario) auth.Identity { return auth.Identity{UserID: c.identity.UserID} }

// TestUnidadeDesativadaNaoRegistra: nenhum serviço novo é aberto numa
// unidade desativada (ela continua visível para quem atua nela).
func TestUnidadeDesativadaNaoRegistra(t *testing.T) {
	c := novoCenario(t)
	s := c.limpo()
	ctx := context.Background()
	cid := c.cadastrar(t).ID
	desativar := func(ativo bool, uns ...uuid.UUID) {
		t.Helper()
		for _, un := range uns {
			if _, err := c.pool.Exec(ctx, `UPDATE unidades SET ativo = $2 WHERE id = $1`, un, ativo); err != nil {
				t.Fatal(err)
			}
		}
	}
	desativar(false, c.unidade, c.unidade2, c.unCreas, c.unPop, c.unAbrigo, c.unConselho, c.unMulher)
	inativa := func(nome string, err error) {
		t.Helper()
		if !errors.Is(err, domain.ErrUnidadeInativa) {
			t.Errorf("%s numa unidade desativada: %v", nome, err)
		}
	}
	_, err := s.AtualizarCentroPopPerfil(ctx, c.identity, cid, c.unPop, domain.SituacoesRua[0], nil, "outro", false, "", false, "", false, "")
	inativa("perfil de rua", err)
	_, err = s.RegistrarEncaminhamentoExterno(ctx, c.identity, cid, c.unPop, "caps_ad", "media", "motivo válido")
	inativa("encaminhamento externo", err)
	_, err = s.RegistrarAtendimentoColetivo(ctx, c.identity, c.unidade, time.Now(), domain.CategoriasColetivo[0], 3, "")
	inativa("coletivo", err)
	_, err = s.SolicitarBeneficio(ctx, c.identity, cid, nil, c.unidade, domain.BeneficioCestaBasica, domain.FormaAcessoDemandaEspontanea, 1, "")
	inativa("benefício", err)
	_, err = s.Encaminhar(ctx, c.identity, cid, c.unidade, c.unidade2, "motivo do encaminhamento")
	inativa("encaminhamento (origem)", err)
	_, err = s.IncluirPaif(ctx, c.identity, cid, c.unidade, domain.PaifAtendimentoParticularizado, false, false, false, 0)
	inativa("PAIF", err)
	_, err = s.RegistrarConselhoAtendimento(ctx, c.identity, cid, c.unConselho, "Mãe", "", "", "escola", "evasao_escolar", "")
	inativa("Conselho", err)
	_, err = s.RegistrarPaefiCaso(ctx, c.identity, cid, c.unCreas, []string{"negligencia"}, "", "")
	inativa("PAEFI", err)
	_, err = s.RegistrarCasoCasaMulher(ctx, c.identity, cid, c.unMulher, casoMulherInput())
	inativa("Casa da Mulher", err)
	_, err = s.RegistrarMseMedida(ctx, c.identity, cid, c.unCreas, "1/2026", "Vara", "furto", domain.MseTipoLiberdadeAssistida, "")
	inativa("MSE", err)
	_, err = s.RegistrarNotificacaoCreas(ctx, c.identity, cid, c.unCreas, "ministerio_publico", "assunto da notificação", nil)
	inativa("notificação", err)
	_, err = s.RegistrarOficioCras(ctx, c.identity, cid, c.unidade, "MPE", "1/2026", nil, "assunto do ofício")
	inativa("ofício", err)
	_, err = s.RegistrarAcolhimento(ctx, c.identity, cid, c.unAbrigo,
		application.AcolhimentoInput{Sexo: "masculino", Motivos: []string{"negligencia_abandono"}})
	inativa("acolhimento", err)

	// Origem ativa, destino desativado.
	desativar(true, c.unidade)
	_, err = s.Encaminhar(ctx, c.identity, cid, c.unidade, c.unidade2, "motivo do encaminhamento")
	inativa("encaminhamento (destino)", err)
}

// TestEntradasInvalidasNosCasosDeUso: a validação de cada caso de uso
// recusa antes de tocar no banco (ou no armazenamento).
func TestEntradasInvalidasNosCasosDeUso(t *testing.T) {
	c := novoCenario(t)
	s := c.limpo()
	ctx := context.Background()
	cid := c.cadastrar(t).ID
	longo := strings.Repeat("x", 5000)
	invalido := func(nome string, err error) {
		t.Helper()
		var v *domain.ValidationError
		if !errors.As(err, &v) {
			t.Errorf("%s: esperava erro de validação, veio %v", nome, err)
		}
	}
	acolh := c.acolhimento(t)

	_, err := s.SalvarPiaFicha(ctx, c.identity, acolh.ID, application.PiaFichaInput{Escola: longo})
	invalido("ficha do PIA", err)
	_, err = s.StartUploadDocumento(ctx, c.identity, acolh.ID, "x", "doc.pdf", "application/pdf", 8)
	invalido("tipo de documento do acolhido", err)
	_, err = s.RegistrarEncaminhamentoExterno(ctx, c.identity, cid, c.unPop, "lugar_nenhum", "media", "motivo válido")
	invalido("destino externo", err)
	_, err = s.RegistrarAtendimentoColetivo(ctx, c.identity, c.unidade, time.Now(), "x", 3, "")
	invalido("categoria do coletivo", err)
	_, err = s.ListarAtendimentosColetivos(ctx, c.identity, c.unidade, "outubro")
	invalido("competência", err)
	_, err = s.IncluirPaif(ctx, c.identity, cid, c.unidade, "x", false, false, false, 0)
	invalido("tipo de atendimento do PAIF", err)
	_, err = s.RegistrarMseMedida(ctx, c.identity, cid, c.unCreas, "1/2026", "Vara", "furto", "x", "")
	invalido("tipo de MSE", err)
	_, err = s.RegistrarNotificacaoCreas(ctx, c.identity, cid, c.unCreas, "x", "assunto da notificação", nil)
	invalido("origem da notificação", err)
	_, err = s.RegistrarOficioCras(ctx, c.identity, cid, c.unidade, "", "1/2026", nil, "assunto do ofício")
	invalido("órgão do ofício", err)
	_, err = s.AdicionarIrmao(ctx, c.identity, acolh.ID, "", nil, false, "", "")
	invalido("nome do irmão", err)
	_, err = s.AtualizarAcolhimento(ctx, c.identity, acolh.ID, application.AcolhimentoInput{Sexo: "x", Motivos: []string{"negligencia_abandono"}})
	invalido("sexo do acolhido", err)
	_, err = s.TransitarEncaminhamentoExterno(ctx, c.identity, c.encaminhamentoExterno(t).ID, "x", "")
	invalido("status do encaminhamento externo", err)
	_, err = s.SolicitarPedido(ctx, c.identity, cid, domain.PedidoCorrecao, "")
	invalido("justificativa do pedido", err)

	// Ofício: responder sem número; responder duas vezes.
	of, err := s.RegistrarOficioCras(ctx, c.identity, cid, c.unidade, "MPE", "1/2026", nil, "assunto do ofício")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ResponderOficioCras(ctx, c.identity, of.ID, "")
	invalido("número da resposta", err)
	if _, err := s.ResponderOficioCras(ctx, c.identity, of.ID, "R-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResponderOficioCras(ctx, c.identity, of.ID, "R-2"); !errors.Is(err, domain.ErrOficioRespondido) {
		t.Errorf("ofício já respondido: %v", err)
	}

	// Documentos do Centro POP: tamanho, munícipe, tipo de arquivo e tipo de documento.
	_, err = s.StartUploadDocumentoCentroPop(ctx, c.identity, cid, c.unPop, domain.DocumentoCentroPopLaudoSaude, "doc.pdf", "application/pdf", 0)
	invalido("tamanho do documento", err)
	if _, err := s.StartUploadDocumentoCentroPop(ctx, c.identity, uuid.New(), c.unPop, domain.DocumentoCentroPopLaudoSaude, "doc.pdf", "application/pdf", 8); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("documento de munícipe inexistente: %v", err)
	}
	if _, err := s.StartUploadDocumentoCentroPop(ctx, c.identity, cid, c.unPop, domain.DocumentoCentroPopLaudoSaude, "nota.txt", "text/plain", 8); err == nil {
		t.Error("tipo de arquivo não aceito deveria falhar")
	}
	_, err = s.StartUploadDocumentoCentroPop(ctx, c.identity, cid, c.unPop, "x", "doc.pdf", "application/pdf", 8)
	invalido("tipo de documento do Centro POP", err)
	if _, err := s.StartUploadDocumento(ctx, c.identity, acolh.ID, domain.DocumentoTipoAcolhido, "nota.txt", "text/plain", 8); err == nil {
		t.Error("tipo de arquivo não aceito (acolhido) deveria falhar")
	}
}

// TestPermissaoESolicitante: quem não atua no módulo não edita cadastro nem
// exerce direitos do titular; o solicitante do benefício tem de existir.
func TestPermissaoESolicitante(t *testing.T) {
	c := novoCenario(t)
	s := c.limpo()
	ctx := context.Background()
	cid := c.cadastrar(t).ID
	fora := semArea(c)
	if _, err := s.Atualizar(ctx, fora, cid, domain.CidadaoInput{Nome: "X", NIS: "12345678901"}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("editar sem área: %v", err)
	}
	if _, err := s.SolicitarAcesso(ctx, fora, cid, "titular pediu os dados"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("acesso sem área: %v", err)
	}
	if _, err := s.SolicitarPedido(ctx, fora, cid, domain.PedidoCorrecao, "corrigir o nome"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("pedido sem área: %v", err)
	}
	ninguem := uuid.New()
	if _, err := s.SolicitarBeneficio(ctx, c.identity, cid, &ninguem, c.unidade, domain.BeneficioCestaBasica,
		domain.FormaAcessoDemandaEspontanea, 1, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("solicitante inexistente: %v", err)
	}
	solicitante := c.cadastrar(t).ID
	b, err := s.SolicitarBeneficio(ctx, c.identity, cid, &solicitante, c.unidade, domain.BeneficioCestaBasica,
		domain.FormaAcessoDemandaEspontanea, 1, "")
	if err != nil || b.SolicitanteID == nil || *b.SolicitanteID != solicitante {
		t.Errorf("benefício com solicitante: %+v, %v", b, err)
	}
}

// TestDocumentosEArmazenamento: documento pendente não baixa, confirmar de
// novo é idempotente, e o armazenamento fora do ar vira erro de dependência
// (a URL) ou só um aviso (o objeto órfão na exclusão).
func TestDocumentosEArmazenamento(t *testing.T) {
	c := novoCenario(t)
	s := c.limpo()
	ctx := context.Background()

	pendente, err := s.StartUploadDocumento(ctx, c.identity, c.acolhimento(t).ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.DownloadURLDocumento(ctx, c.identity, pendente.Documento.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("documento pendente não baixa: %v", err)
	}
	pop, err := s.StartUploadDocumentoCentroPop(ctx, c.identity, c.cadastrar(t).ID, c.unPop, domain.DocumentoCentroPopLaudoSaude, "doc.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.DownloadURLDocumentoCentroPop(ctx, c.identity, pop.Documento.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("documento pendente do Centro POP não baixa: %v", err)
	}

	docCp := c.documentoCentroPop(t)
	if d, err := s.ConfirmUploadDocumentoCentroPop(ctx, c.identity, docCp.ID); err != nil || d.Status != domain.DocumentoStatusReady {
		t.Errorf("confirmar de novo é idempotente: %+v, %v", d, err)
	}
	doc := c.documento(t)

	c.store.FailPresign = true
	if _, _, err := s.DownloadURLDocumento(ctx, c.identity, doc.ID); err == nil {
		t.Error("armazenamento fora: a URL do documento do acolhido deveria falhar")
	}
	if _, _, err := s.DownloadURLDocumentoCentroPop(ctx, c.identity, docCp.ID); err == nil {
		t.Error("armazenamento fora: a URL do documento do Centro POP deveria falhar")
	}
	c.store.FailPresign = false

	c.store.FailDelete = true
	if err := s.RemoverDocumento(ctx, c.identity, doc.ID); err != nil {
		t.Errorf("objeto órfão não bloqueia a exclusão do metadado: %v", err)
	}
	c.store.FailDelete = false
}

// TestProntuarioComAcolhimentoPropagaFalhas: a ficha com acolhimento (e o
// PIA preenchido) percorre as listas do acolhimento — a falha em qualquer
// leitura sobe.
func TestProntuarioComAcolhimentoPropagaFalhas(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	acolh := c.acolhimento(t)
	if _, err := c.limpo().SalvarPiaFicha(ctx, c.identity, acolh.ID, application.PiaFichaInput{Escola: "EMEB"}); err != nil {
		t.Fatal(err)
	}
	p, err := c.limpo().Prontuario(ctx, c.identity, acolh.CidadaoID)
	if err != nil || len(p.CasaAbrigoAcolhimentos) != 1 || p.CasaAbrigoAcolhimentos[0].Pia == nil {
		t.Fatalf("prontuário com acolhimento e PIA: %+v, %v", p.CasaAbrigoAcolhimentos, err)
	}
	for failAt := 1; ; failAt++ {
		fr := &faultRepo{inner: infrastructure.NewRepository(), failAt: failAt}
		if _, err := c.svc(fr, publisher{}).Prontuario(ctx, c.identity, acolh.CidadaoID); err == nil {
			if failAt <= len(fr.trace) {
				t.Fatalf("falha na chamada %d (%s) não subiu", failAt, fr.trace[failAt-1])
			}
			break
		}
	}
}
