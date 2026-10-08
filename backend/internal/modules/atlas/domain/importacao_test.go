package domain

import "testing"

func TestAssinatura(t *testing.T) {
	w := validWorkflow()
	w.Normalize()
	w.Etapas[0].Documentos = append(w.Etapas[0].Documentos, EtapaDocumento{NomeDocumento: "Anexo", Formato: FormatoNatoDigital,
		TipoAssinatura: AssinaturaIndividual})
	w.Etapas[0].Transicoes = append(w.Etapas[0].Transicoes, EtapaTransicao{DestinoOrdem: 2, CondicaoTransicao: "Aprovado com ressalva"},
		EtapaTransicao{DestinoOrdem: 1, CondicaoTransicao: "Refazer"})
	// Mesma coisa em outra ordem (etapas, peças, transições): mesma assinatura.
	r := validWorkflow()
	r.Normalize()
	r.Etapas[0].Documentos = append([]EtapaDocumento{{NomeDocumento: "Anexo", Formato: FormatoNatoDigital,
		TipoAssinatura: AssinaturaIndividual}}, r.Etapas[0].Documentos...)
	r.Etapas[0].Transicoes = append([]EtapaTransicao{{DestinoOrdem: 1, CondicaoTransicao: "Refazer"},
		{DestinoOrdem: 2, CondicaoTransicao: "Aprovado com ressalva"}}, r.Etapas[0].Transicoes...)
	r.Etapas[0], r.Etapas[1] = r.Etapas[1], r.Etapas[0]
	r.Versao, r.CodigoProcessual = 9, "OUTRO.CODIGO"
	if Assinatura(w) != Assinatura(r) {
		t.Fatalf("mesmo conteúdo, assinaturas diferentes:\n%s\n%s", Assinatura(w), Assinatura(r))
	}
	r.Etapas[0].PrazoSLAEmDias++
	if Assinatura(w) == Assinatura(r) {
		t.Fatal("prazo diferente, mesma assinatura")
	}
}
