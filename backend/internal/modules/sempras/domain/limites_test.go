package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestLimitesDeTexto: cada campo de texto livre tem limite de tamanho (o
// banco tem o mesmo CHECK) — passar dele é erro de validação, não 500.
func TestLimitesDeTexto(t *testing.T) {
	id, agora := uuid.New(), time.Now()
	L := strings.Repeat("x", 5000)
	motivos := []string{MotivosAcolhimento[0]}
	acolhimento := func(nome, nis, tel, vinculo, endereco, processo, guia, consNome, consTel, situacao, obs string) error {
		_, err := NovoAcolhimento(id, id, "feminino", motivos, nome, nis, tel, vinculo, endereco, processo, guia, consNome, consTel,
			situacao, false, false, obs)
		return err
	}
	casos := map[string]func() error{
		"rede: parentesco": func() error { _, err := NovoContatoRedeApoio(id, "Maria", L, ""); return err },
		"rede: telefone":   func() error { _, err := NovoContatoRedeApoio(id, "Maria", "", L); return err },
		"visitante: documento": func() error {
			_, err := NovoVisitante(id, "Visita", L, "", agora, "", "", "")
			return err
		},
		"visitante: parentesco": func() error {
			_, err := NovoVisitante(id, "Visita", "", L, agora, "", "", "")
			return err
		},
		"visitante: hora de saída": func() error {
			_, err := NovoVisitante(id, "Visita", "", "", agora, "10:00", "25:99", "")
			return err
		},
		"visitante: observações": func() error {
			_, err := NovoVisitante(id, "Visita", "", "", agora, "", "", L)
			return err
		},
		"termo: responsável": func() error { _, err := NovoTermo(id, TiposTermo[0], L, "", "", agora); return err },
		"termo: parentesco":  func() error { _, err := NovoTermo(id, TiposTermo[0], "", L, "", agora); return err },
		"termo: motivo":      func() error { _, err := NovoTermo(id, TiposTermo[0], "", "", L, agora); return err },
		"PIA: escola": func() error {
			_, err := NovaPiaFicha(id, "", "", L, "", "", false, "", "", "", "", "", "", "", "")
			return err
		},
		"meta do PIA: providência": func() error {
			_, err := NovaPiaMeta(id, PiaMetaPlano, "meta válida", L, "", nil)
			return err
		},
		"meta do PIA: responsável": func() error {
			_, err := NovaPiaMeta(id, PiaMetaPlano, "meta válida", "", L, nil)
			return err
		},
		"benefício: observação": func() error {
			_, err := NovoBeneficio(id, nil, id, BeneficioCestaBasica, FormaAcessoDemandaEspontanea, 1, L)
			return err
		},
		"encaminhamento: observação da decisão": func() error {
			e := Encaminhamento{Status: EncaminhamentoStatusPendente}
			return e.Decidir(true, L, id, agora)
		},
		"coletivo: descrição": func() error {
			_, err := NovoAtendimentoColetivo(id, agora, CategoriasColetivo[0], 1, L)
			return err
		},
		"conselho: nome do responsável": func() error {
			_, err := NovoConselhoAtendimento(id, id, L, "", "", "escola", "evasao_escolar", "")
			return err
		},
		"conselho: telefone do responsável": func() error {
			_, err := NovoConselhoAtendimento(id, id, "", "", L, "escola", "evasao_escolar", "")
			return err
		},
		"conselho: observações": func() error {
			_, err := NovoConselhoAtendimento(id, id, "", "", "", "escola", "evasao_escolar", L)
			return err
		},
		"PAEFI: membros": func() error { _, err := NovoPaefiCaso(id, id, []string{"negligencia"}, L, ""); return err },
		"PAEFI: observações": func() error {
			_, err := NovoPaefiCaso(id, id, []string{"negligencia"}, "", L)
			return err
		},
		"MSE: processo": func() error {
			_, err := NovaMseMedida(id, id, L, "", "furto", MseTipoLiberdadeAssistida, "")
			return err
		},
		"MSE: vara": func() error {
			_, err := NovaMseMedida(id, id, "", L, "furto", MseTipoLiberdadeAssistida, "")
			return err
		},
		"MSE: instituição": func() error {
			_, err := NovaMseMedida(id, id, "", "", "furto", MseTipoLiberdadeAssistida, L)
			return err
		},
		"notificação: assunto": func() error {
			_, err := NovaNotificacaoCreas(id, id, OrigensNotificacaoCreas[0], L, nil)
			return err
		},
		"ofício: assunto": func() error { _, err := NovoOficioCras(id, id, "MPE", "1/2026", nil, L); return err },
		"acolhimento: nome do responsável": func() error {
			return acolhimento(L, "", "", "", "", "", "", "", "", "", "")
		},
		"acolhimento: NIS do responsável": func() error {
			return acolhimento("", "123", "", "", "", "", "", "", "", "", "")
		},
		"acolhimento: telefone": func() error { return acolhimento("", "", L, "", "", "", "", "", "", "", "") },
		"acolhimento: vínculo":  func() error { return acolhimento("", "", "", L, "", "", "", "", "", "", "") },
		"acolhimento: endereço": func() error { return acolhimento("", "", "", "", L, "", "", "", "", "", "") },
		"acolhimento: processo": func() error { return acolhimento("", "", "", "", "", L, "", "", "", "", "") },
		"acolhimento: guia":     func() error { return acolhimento("", "", "", "", "", "", L, "", "", "", "") },
		"acolhimento: nome do conselheiro": func() error {
			return acolhimento("", "", "", "", "", "", "", L, "", "", "")
		},
		"acolhimento: telefone do conselheiro": func() error {
			return acolhimento("", "", "", "", "", "", "", "", L, "", "")
		},
		"acolhimento: situação jurídica": func() error {
			return acolhimento("", "", "", "", "", "", "", "", "", L, "")
		},
		"acolhimento: observações": func() error { return acolhimento("", "", "", "", "", "", "", "", "", "", L) },
		"acolhimento: reavaliação inválida": func() error {
			a := Acolhimento{CidadaoID: id, UnidadeID: id}
			return a.Atualizar("x", motivos, "", "", "", "", "", "", "", "", "", "", false, false, "")
		},
		"irmão: unidade": func() error { _, err := NovoIrmao(id, "Irmão", nil, false, L, ""); return err },
		"irmão: observação": func() error {
			_, err := NovoIrmao(id, "Irmão", nil, false, "", L)
			return err
		},
		"diagnóstico: descrição": func() error {
			_, err := NovoDiagnostico(id, CategoriasDiagnostico[0], "diagnóstico", L)
			return err
		},
		"documento do acolhido: nome do arquivo": func() error {
			_, err := NovoDocumentoCasaAbrigo(id, DocumentoTipoAcolhido, L, "k", "application/pdf")
			return err
		},
		"perfil de rua: doença": func() error {
			_, err := NovoCentroPopPerfil(id, SituacoesRua[0], nil, "outro", true, L, false, "", false, "")
			return err
		},
		"perfil de rua: medicação": func() error {
			_, err := NovoCentroPopPerfil(id, SituacoesRua[0], nil, "outro", false, "", true, L, false, "")
			return err
		},
		"perfil de rua: drogas": func() error {
			_, err := NovoCentroPopPerfil(id, SituacoesRua[0], nil, "outro", false, "", false, "", true, L)
			return err
		},
		"encaminhamento externo: observações": func() error {
			e := EncaminhamentoExterno{}
			return e.Transitar(EncaminhamentoExternoStatusConcluido, L, id, agora)
		},
		"documento do Centro POP: nome do arquivo": func() error {
			_, err := NovoDocumentoCentroPop(id, id, DocumentoCentroPopLaudoSaude, L, "k", "application/pdf")
			return err
		},
	}
	for nome, chamar := range casos {
		if err := chamar(); err == nil {
			t.Errorf("%s: deveria recusar", nome)
		}
	}
}
