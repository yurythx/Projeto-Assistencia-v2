// Package domain define o Trâmite: processos administrativos numerados,
// com controle de sigilo, documentos e tramitação entre unidades.
package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

var (
	ErrNotFound      = errors.New("tramite: registro não encontrado")
	ErrForbidden     = errors.New("tramite: sem acesso a este processo")
	ErrInvalidState  = errors.New("tramite: operação inválida no estado atual")
	ErrSignatureOff  = errors.New("tramite: assinatura eletrônica indisponível (Signum desativado)")
	ErrDocumentState = errors.New("tramite: o documento não está em rascunho")
	// ErrClosed: processo concluído/arquivado não recebe documentos nem
	// assinaturas — é preciso reabri-lo antes.
	ErrClosed = errors.New("tramite: processo concluído ou arquivado; reabra-o para alterar")
	// ErrPendingSignature: não se conclui um processo com documento
	// aguardando assinatura (o resultado ficaria fora do processo encerrado).
	ErrPendingSignature = errors.New("tramite: há documento aguardando assinatura; aguarde ou cancele o envelope antes de concluir")
	// ErrPublicGrant: credencial só faz sentido em processo restrito ou
	// sigiloso (o público já é legível por qualquer autenticado).
	ErrPublicGrant = errors.New("tramite: processo público não usa credencial de acesso")
	// ErrInactiveTipo: tipos desativados não aceitam novos processos.
	ErrInactiveTipo = errors.New("tramite: tipo de processo inexistente ou desativado")
	// ErrSerieInvalida: o código da série da TTDD não tem o formato 2.0.02.00.07.
	ErrSerieInvalida = errors.New("tramite: código de série da TTDD inválido")
	// ErrInactiveUnidade: unidade (ou a entidade dela) inexistente ou
	// desativada não abre nem recebe processos.
	ErrInactiveUnidade = errors.New("tramite: unidade inexistente ou desativada")
)

// Níveis de sigilo.
const (
	SigiloPublico  = "publico"
	SigiloRestrito = "restrito"
	SigiloSigiloso = "sigiloso"
)

// Estados do processo.
const (
	StatusAberto       = "aberto"
	StatusEmTramitacao = "em_tramitacao"
	StatusConcluido    = "concluido"
	StatusArquivado    = "arquivado"
)

// Aberto reporta se o processo ainda está em curso (aberto ou em
// tramitação) — só então aceita documentos e pedidos de assinatura.
func Aberto(status string) bool { return status == StatusAberto || status == StatusEmTramitacao }

// Encerrado reporta se o processo está concluído ou arquivado.
func Encerrado(status string) bool { return status == StatusConcluido || status == StatusArquivado }

// FormatNumero monta "NNNNNN/AAAA".
func FormatNumero(seq, ano int) string { return fmt.Sprintf("%06d/%04d", seq, ano) }

// Tipo é um tipo de processo.
type Tipo struct {
	ID        uuid.UUID `json:"id"`
	Slug      string    `json:"slug"`
	Nome      string    `json:"nome"`
	Descricao string    `json:"descricao"`
	Ativo     bool      `json:"ativo"`
}

// Processo é um processo administrativo.
type Processo struct {
	ID              uuid.UUID  `json:"id"`
	Numero          string     `json:"numero"`
	TipoID          uuid.UUID  `json:"tipo_id"`
	Tipo            string     `json:"tipo"`
	Assunto         string     `json:"assunto"`
	Interessado     string     `json:"interessado"`
	Descricao       string     `json:"descricao"`
	Sigilo          string     `json:"sigilo"`
	Status          string     `json:"status"`
	UnidadeOrigemID uuid.UUID  `json:"unidade_origem_id"`
	UnidadeOrigem   string     `json:"unidade_origem"`
	UnidadeAtualID  uuid.UUID  `json:"unidade_atual_id"`
	UnidadeAtual    string     `json:"unidade_atual"`
	CreatedBy       uuid.UUID  `json:"created_by"`
	CreatedByName   string     `json:"created_by_name"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ConcluidoAt     *time.Time `json:"concluido_at,omitempty"`
	// Classificação pelo Atlas (ADR 026): o procedimento que o processo segue
	// e a série da TTDD que define a guarda.
	AtlasProcedimentoID *uuid.UUID `json:"atlas_procedimento_id"`
	CodigoTTDD          string     `json:"codigo_ttdd"`
}

// codigoTTDD: código de série da TTDD ("2.0.02.00.07").
var codigoTTDD = regexp.MustCompile(`^[0-9]{1,2}\.0\.[0-9]{2}\.[0-9]{2}\.[0-9]{2}$`)

// ValidarClassificacao confere o código da série (vazio = sem série).
func ValidarClassificacao(codigo string) error {
	if codigo != "" && !codigoTTDD.MatchString(codigo) {
		return ErrSerieInvalida
	}
	return nil
}

// Documento é uma peça do processo.
type Documento struct {
	ID          uuid.UUID  `json:"id"`
	ProcessoID  uuid.UUID  `json:"processo_id"`
	Tipo        string     `json:"tipo"`
	Titulo      string     `json:"titulo"`
	Origem      string     `json:"origem"`
	Conteudo    string     `json:"conteudo,omitempty"`
	ObjectKey   string     `json:"-"`
	ContentType string     `json:"content_type,omitempty"`
	SizeBytes   int64      `json:"size_bytes"`
	SHA256      *string    `json:"sha256,omitempty"`
	Status      string     `json:"status"`
	EnvelopeID  *uuid.UUID `json:"envelope_id,omitempty"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Movimento é um registro imutável do histórico.
type Movimento struct {
	ID            uuid.UUID  `json:"id"`
	ProcessoID    uuid.UUID  `json:"processo_id"`
	Acao          string     `json:"acao"`
	DeUnidadeID   *uuid.UUID `json:"de_unidade_id,omitempty"`
	DeUnidade     string     `json:"de_unidade,omitempty"`
	ParaUnidadeID *uuid.UUID `json:"para_unidade_id,omitempty"`
	ParaUnidade   string     `json:"para_unidade,omitempty"`
	Despacho      string     `json:"despacho"`
	ActorID       *uuid.UUID `json:"actor_id,omitempty"`
	ActorName     string     `json:"actor_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// AccessInfo é o que a regra de sigilo precisa saber sobre o processo.
type AccessInfo struct {
	Sigilo          string
	CreatedBy       uuid.UUID
	UnidadeOrigemID uuid.UUID
	UnidadeAtualID  uuid.UUID
	ExplicitGrant   bool
}

// CanRead aplica o controle de sigilo:
//   - público: qualquer autenticado;
//   - restrito: lotados na unidade atual ou de origem, credenciados, autor
//     e tramite:manage;
//   - sigiloso: SOMENTE credenciados explícitos e o autor (nem a unidade
//     atual vê sem credencial; tramite:manage também precisa de credencial).
func CanRead(identity auth.Identity, a AccessInfo) bool {
	if a.CreatedBy == identity.UserID || a.ExplicitGrant {
		return true
	}
	switch a.Sigilo {
	case SigiloPublico:
		return true
	case SigiloRestrito:
		if Gere(identity, a) {
			return true
		}
		return InUnidade(identity, a.UnidadeAtualID) || InUnidade(identity, a.UnidadeOrigemID)
	default:
		return false
	}
}

// CanAct reporta se identity pode movimentar o processo (tramitar,
// juntar documento, concluir): precisa estar lotado na unidade ATUAL, ou
// ter tramite:manage (e acesso de leitura).
func CanAct(identity auth.Identity, a AccessInfo) bool {
	if !CanRead(identity, a) {
		return false
	}
	return InUnidade(identity, a.UnidadeAtualID) || auth.Can(identity, auth.PermTramiteManage, auth.InUnidade(a.UnidadeAtualID))
}

// Gere reporta se identity tem tramite:manage cobrindo a unidade atual ou
// a de origem do processo (ADR 013: vale onde foi concedida, com herança
// para as subunidades).
func Gere(identity auth.Identity, a AccessInfo) bool {
	return auth.Can(identity, auth.PermTramiteManage, auth.InUnidade(a.UnidadeAtualID)) ||
		auth.Can(identity, auth.PermTramiteManage, auth.InUnidade(a.UnidadeOrigemID))
}

// PodeTramitar reporta se identity tem tramite:route (ou tramite:manage)
// cobrindo a unidade em que o processo está.
func PodeTramitar(identity auth.Identity, unidadeAtual uuid.UUID) bool {
	alvo := auth.InUnidade(unidadeAtual)
	return auth.Can(identity, auth.PermTramiteRoute, alvo) || auth.Can(identity, auth.PermTramiteManage, alvo)
}

// InUnidade reporta se identity tem lotação na unidade (ou em departamento
// dela — o IAM completa unidade_id a partir do departamento).
func InUnidade(identity auth.Identity, unidadeID uuid.UUID) bool {
	for _, s := range identity.Scopes {
		if s.UnidadeID != nil && *s.UnidadeID == unidadeID {
			return true
		}
	}
	return false
}

// Filter restringe a listagem.
type Filter struct {
	Query     string
	Status    string
	UnidadeID *uuid.UUID
	// Mine: só processos cuja unidade ATUAL é uma das lotações do usuário
	// (a "caixa" da unidade — o processo sai dela ao ser tramitado).
	Mine bool
}

// Repository é a porta de persistência.
type Repository interface {
	Tipos(ctx context.Context, db database.DBTX) ([]Tipo, error)
	TipoAtivo(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error)
	// UnidadeAtiva reporta se a unidade existe e ela e a entidade estão
	// ativas, travando as duas linhas até o fim da transação.
	UnidadeAtiva(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error)
	NextNumero(ctx context.Context, db database.DBTX, ano int) (int, error)
	Insert(ctx context.Context, db database.DBTX, p Processo) error
	Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (Processo, error)
	HasGrant(ctx context.Context, db database.DBTX, processoID, userID uuid.UUID) (bool, error)
	// ListVisible aplica o sigilo no próprio SQL (para paginar corretamente).
	ListVisible(ctx context.Context, db database.DBTX, identity auth.Identity, f Filter, p pagination.Params) ([]Processo, int64, error)
	Update(ctx context.Context, db database.DBTX, p Processo) error
	Grant(ctx context.Context, db database.DBTX, processoID, userID, grantedBy uuid.UUID) error
	// Revoke remove a credencial; reporta se ela existia.
	Revoke(ctx context.Context, db database.DBTX, processoID, userID uuid.UUID) (bool, error)
	Grants(ctx context.Context, db database.DBTX, processoID uuid.UUID) ([]Grant, error)

	Documentos(ctx context.Context, db database.DBTX, processoID uuid.UUID) ([]Documento, error)
	GetDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) (Documento, error)
	InsertDocumento(ctx context.Context, db database.DBTX, d Documento) error
	UpdateDocumento(ctx context.Context, db database.DBTX, d Documento) error
	DocumentoByEnvelope(ctx context.Context, db database.DBTX, envelopeID uuid.UUID) (Documento, error)
	// PendingSignatures conta documentos aguardando assinatura.
	PendingSignatures(ctx context.Context, db database.DBTX, processoID uuid.UUID) (int, error)

	AddMovimento(ctx context.Context, db database.DBTX, m Movimento) error
	Movimentos(ctx context.Context, db database.DBTX, processoID uuid.UUID) ([]Movimento, error)

	// ProcessosDoProcedimento: os processos em andamento (aberto ou em
	// tramitação) que seguem o procedimento do Atlas (ADR 027).
	ProcessosDoProcedimento(ctx context.Context, db database.DBTX, atlasProcedimentoID uuid.UUID) ([]Processo, error)
}

// Grant é uma credencial de acesso.
type Grant struct {
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	GrantedBy uuid.UUID `json:"granted_by"`
	GrantedAt time.Time `json:"granted_at"`
}

// SignaturePort é a porta para o motor de assinatura (implementada em
// internal/app com o Signum — o Trâmite nunca importa o Signum).
type SignaturePort interface {
	Available() bool
	OpenEnvelope(ctx context.Context, tx pgx.Tx, req SignatureRequest) (uuid.UUID, error)
}

// SignatureRequest pede a abertura de um envelope.
type SignatureRequest struct {
	Title, Description, DocumentSHA256, SourceRef string
	SignerIDs                                     []uuid.UUID
	Sequential                                    bool
}
