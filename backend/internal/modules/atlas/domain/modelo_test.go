package domain

import (
	"bytes"
	"strings"
	"testing"
)

func TestModeloValidar(t *testing.T) {
	m := Modelo{Nome: "  Ofício padrão ", Descricao: " d "}
	m.Normalizar()
	if m.Nome != "Ofício padrão" || m.Descricao != "d" || m.Validar() != nil {
		t.Fatalf("válido: %+v %v", m, m.Validar())
	}
	for name, x := range map[string]Modelo{
		"sem nome":        {},
		"nome longo":      {Nome: strings.Repeat("a", 151)},
		"descrição longa": {Nome: "x", Descricao: strings.Repeat("a", 2001)},
	} {
		if x.Validar() == nil {
			t.Errorf("%s aceito", name)
		}
	}
}

func TestValidarArquivoModelo(t *testing.T) {
	for nome, conteudo := range map[string][]byte{
		"modelo.pdf": []byte("%PDF-1.7"), "MODELO.DOCX": []byte("PK\x03\x04a"), "planilha.xlsx": []byte("PK\x03\x04"),
		"texto.odt": []byte("PK\x03\x04"), "calc.ods": []byte("PK\x03\x04"), "antigo.doc": []byte("\xD0\xCF\x11\xE0x"),
		"rico.rtf": []byte("{\\rtf1"),
	} {
		if _, _, err := ValidarArquivoModelo(nome, conteudo); err != nil {
			t.Errorf("%s recusado: %v", nome, err)
		}
	}
	// O caminho enviado pelo navegador (Windows ou Unix) é descartado.
	if nome, tipo, err := ValidarArquivoModelo(`C:\Users\x\Ofício.pdf`, []byte("%PDF-")); err != nil || nome != "Ofício.pdf" ||
		tipo != "application/pdf" {
		t.Fatalf("caminho: %q %q %v", nome, tipo, err)
	}
	grande := append([]byte("%PDF-"), bytes.Repeat([]byte("a"), MaxModeloBytes)...)
	for name, c := range map[string]struct {
		nome     string
		conteudo []byte
	}{
		"sem nome":            {"", []byte("%PDF-")},
		"só pasta":            {"pasta/", []byte("%PDF-")},
		"nome longo":          {strings.Repeat("a", 252) + ".pdf", []byte("%PDF-")},
		"executável":          {"x.exe", []byte("MZ")},
		"vazio":               {"x.pdf", nil},
		"acima de 10 MB":      {"x.pdf", grande},
		"conteúdo disfarçado": {"x.pdf", []byte("PK\x03\x04")},
	} {
		if _, _, err := ValidarArquivoModelo(c.nome, c.conteudo); err == nil {
			t.Errorf("%s aceito", name)
		}
	}
}

func TestComModelosECasaModelo(t *testing.T) {
	ativo := Modelo{Nome: "Ofício padrão", Descricao: "Comunicação externa", Ativo: true, Atual: ModeloVersao{Versao: 3}}
	inativo := Modelo{Nome: "Antigo", Atual: ModeloVersao{Versao: 1}}
	if got := ComModelos("síntese\n", "2.0", []Modelo{inativo}); got != "síntese\n" {
		t.Fatalf("sem modelo ativo, a síntese não muda: %q", got)
	}
	if got := ComModelos("síntese\n", "2.0", []Modelo{ativo, inativo}); got !=
		"síntese\nModelos de documento para baixar (biblioteca do Atlas, série 2.0): Ofício padrão (versão 3)\n" {
		t.Fatalf("com modelo: %q", got)
	}
	for busca, want := range map[string]bool{"oficio": true, "comunicação ofício": true, "ofício interno": false, "de": false, "": false} {
		if CasaModelo(ativo, busca) != want {
			t.Errorf("CasaModelo(%q) != %v", busca, want)
		}
	}
}
