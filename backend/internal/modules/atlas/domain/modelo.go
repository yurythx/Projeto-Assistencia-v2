package domain

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Biblioteca de modelos de documento (ADR 024): um catálogo reutilizável;
// cada peça dos fluxos aponta para um modelo, e o arquivo de cada versão
// fica no armazenamento de objetos.

// ErrModeloNaoEncontrado: modelo (ou versão) inexistente.
var ErrModeloNaoEncontrado = errors.New("atlas: modelo de documento não encontrado")

// ErrModeloRepetido: já existe modelo com o mesmo nome.
var ErrModeloRepetido = errors.New("atlas: já existe modelo com este nome")

// ErrPecaNaoEncontrada: a peça não existe ou não é do procedimento.
var ErrPecaNaoEncontrada = errors.New("atlas: peça não encontrada no procedimento")

// MaxModeloBytes limita o arquivo de um modelo.
const MaxModeloBytes = 10 << 20

// Modelo é um modelo de documento com a versão atual (e, no detalhe, o
// histórico de versões).
type Modelo struct {
	ID           uuid.UUID      `json:"id"`
	Nome         string         `json:"nome"`
	Descricao    string         `json:"descricao"`
	Ativo        bool           `json:"ativo"`
	Atual        ModeloVersao   `json:"atual"`
	Versoes      []ModeloVersao `json:"versoes,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at"`
	UpdatedBy    string         `json:"updated_by"`
	PecasLigadas int            `json:"pecas_ligadas"`
}

// ModeloVersao é um arquivo publicado do modelo.
type ModeloVersao struct {
	Versao      int       `json:"versao"`
	ArquivoNome string    `json:"arquivo_nome"`
	ContentType string    `json:"content_type"`
	Tamanho     int64     `json:"tamanho"`
	SHA256      string    `json:"sha256"`
	Nota        string    `json:"nota"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
	// Objeto é a chave no armazenamento (não sai pela API).
	Objeto string `json:"-"`
}

// ModeloResumo é o modelo ligado a uma peça (versão atual).
type ModeloResumo struct {
	ID          uuid.UUID `json:"id"`
	Nome        string    `json:"nome"`
	Versao      int       `json:"versao"`
	ArquivoNome string    `json:"arquivo_nome"`
}

// Normalizar apara nome e descrição.
func (m *Modelo) Normalizar() {
	m.Nome, m.Descricao = strings.TrimSpace(m.Nome), strings.TrimSpace(m.Descricao)
}

// Validar confere nome e descrição (chame Normalizar antes).
func (m Modelo) Validar() error {
	switch {
	case m.Nome == "" || utf8.RuneCountInString(m.Nome) > 150:
		return invalid("nome do modelo obrigatório (até 150 caracteres)")
	case utf8.RuneCountInString(m.Descricao) > 2000:
		return invalid("descrição do modelo acima de 2.000 caracteres")
	}
	return nil
}

// tiposModelo: extensão aceita → tipo de conteúdo e assinatura dos bytes
// iniciais (o nome do arquivo não basta: o conteúdo é conferido).
var tiposModelo = map[string]struct {
	tipo       string
	assinatura []byte
}{
	".pdf":  {"application/pdf", []byte("%PDF-")},
	".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", []byte("PK\x03\x04")},
	".xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("PK\x03\x04")},
	".odt":  {"application/vnd.oasis.opendocument.text", []byte("PK\x03\x04")},
	".ods":  {"application/vnd.oasis.opendocument.spreadsheet", []byte("PK\x03\x04")},
	".doc":  {"application/msword", []byte("\xD0\xCF\x11\xE0")},
	".rtf":  {"application/rtf", []byte("{\\rtf")},
}

// ValidarArquivoModelo confere o arquivo de uma versão (nome, tamanho e
// conteúdo) e devolve o nome limpo e o tipo de conteúdo.
func ValidarArquivoModelo(nome string, conteudo []byte) (string, string, error) {
	nome = strings.TrimSpace(filepath.Base(strings.ReplaceAll(nome, "\\", "/")))
	t, ok := tiposModelo[strings.ToLower(filepath.Ext(nome))]
	switch {
	case nome == "" || nome == "." || utf8.RuneCountInString(nome) > 255:
		return "", "", invalid("nome de arquivo inválido")
	case !ok:
		return "", "", invalid("formato não aceito: use .docx, .odt, .pdf, .doc, .rtf, .xlsx ou .ods")
	case len(conteudo) == 0:
		return "", "", invalid("o arquivo está vazio")
	case len(conteudo) > MaxModeloBytes:
		return "", "", invalid("o arquivo passa de 10 MB")
	case !bytes.HasPrefix(conteudo, t.assinatura):
		return "", "", invalid("o conteúdo do arquivo não corresponde à extensão %s", filepath.Ext(nome))
	}
	return nome, t.tipo, nil
}
