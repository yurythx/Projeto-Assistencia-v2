package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
)

// Repository é a porta de persistência do SEMPRAS. Como em todos os
// plugins, cada método recebe o executor (o pool numa leitura, a transação
// de negócio numa escrita) — o repositório nunca abre transação própria.
type Repository interface {
	// Unidades
	GetUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) (Unidade, error)
	ListUnidades(ctx context.Context, db database.DBTX, area Area, entidadeSlug string) ([]Unidade, error)

	// Munícipe
	InsertCidadao(ctx context.Context, db database.DBTX, c Cidadao, autor uuid.UUID) error
	UpdateCidadao(ctx context.Context, db database.DBTX, c Cidadao) error
	GetCidadao(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (Cidadao, error)
	BuscarPorDocumento(ctx context.Context, db database.DBTX, digitos string) ([]Cidadao, error)
	BuscarPorNome(ctx context.Context, db database.DBTX, nome string, limit int) ([]Cidadao, error)
	HistoricoNaArea(ctx context.Context, db database.DBTX, ids []uuid.UUID, area Area) (map[uuid.UUID]int, error)

	// Fila e atendimento
	ProximaSenha(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string) (int, error)
	InsertAtendimento(ctx context.Context, db database.DBTX, a Atendimento) error
	GetAtendimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (Atendimento, error)
	ProximoAguardando(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string, tipos []string) (Atendimento, error)
	UpdateAtendimento(ctx context.Context, db database.DBTX, a Atendimento) error
	Fila(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string) ([]FilaItem, error)
	AtendimentosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area Area) ([]Atendimento, error)
	Painel(ctx context.Context, db database.DBTX, unidades []Unidade, dia string) ([]PainelUnidade, error)
	Rma(ctx context.Context, db database.DBTX, unidades []Unidade, inicio, fim time.Time) ([]RmaUnidade, error)

	// Prontuário
	InsertRegistro(ctx context.Context, db database.DBTX, r Registro) error
	RegistrosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area Area) ([]Registro, error)

	// Direitos do titular
	InsertPedido(ctx context.Context, db database.DBTX, p PedidoTitular) error
	GetPedido(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (PedidoTitular, error)
	ListPedidosPendentes(ctx context.Context, db database.DBTX) ([]PedidoTitular, error)
	DecidirPedido(ctx context.Context, db database.DBTX, id uuid.UUID, status string, decididoPor uuid.UUID, justificativa string, decididoEm time.Time) error
	AnonimizarCidadao(ctx context.Context, db database.DBTX, id uuid.UUID) error

	// Rede de apoio
	InsertContatoRedeApoio(ctx context.Context, db database.DBTX, c ContatoRedeApoio, autor uuid.UUID) error
	ListContatosRedeApoio(ctx context.Context, db database.DBTX, cidadao uuid.UUID) ([]ContatoRedeApoio, error)
	GetContatoRedeApoio(ctx context.Context, db database.DBTX, id uuid.UUID) (ContatoRedeApoio, error)
	DeleteContatoRedeApoio(ctx context.Context, db database.DBTX, id uuid.UUID) error

	// Benefícios eventuais
	InsertBeneficio(ctx context.Context, db database.DBTX, b Beneficio, autor uuid.UUID) error
	GetBeneficio(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (Beneficio, error)
	UpdateBeneficio(ctx context.Context, db database.DBTX, b Beneficio) error
	BeneficiosNaArea(ctx context.Context, db database.DBTX, beneficiario uuid.UUID, area Area) ([]Beneficio, error)

	// Encaminhamentos
	InsertEncaminhamento(ctx context.Context, db database.DBTX, e Encaminhamento, autor uuid.UUID) error
	GetEncaminhamento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (Encaminhamento, error)
	UpdateEncaminhamento(ctx context.Context, db database.DBTX, e Encaminhamento) error
	EncaminhamentosDoCidadaoNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area Area) ([]Encaminhamento, error)
	ListEncaminhamentosPendentes(ctx context.Context, db database.DBTX, unidadeDestino uuid.UUID) ([]Encaminhamento, error)

	// PAIF
	InsertPaifVinculo(ctx context.Context, db database.DBTX, p PaifVinculo, autor uuid.UUID) error
	GetPaifVinculo(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (PaifVinculo, error)
	UpdatePaifVinculo(ctx context.Context, db database.DBTX, p PaifVinculo) error
	PaifVinculosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area Area) ([]PaifVinculo, error)

	// Conselho Tutelar
	InsertConselhoAtendimento(ctx context.Context, db database.DBTX, c ConselhoAtendimento, autor uuid.UUID) error
	GetConselhoAtendimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (ConselhoAtendimento, error)
	UpdateConselhoAtendimento(ctx context.Context, db database.DBTX, c ConselhoAtendimento) error
	ConselhoAtendimentosNaArea(ctx context.Context, db database.DBTX, crianca uuid.UUID, area Area) ([]ConselhoAtendimento, error)

	// PAEFI (CREAS)
	InsertPaefiCaso(ctx context.Context, db database.DBTX, p PaefiCaso, autor uuid.UUID) error
	GetPaefiCaso(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (PaefiCaso, error)
	UpdatePaefiCaso(ctx context.Context, db database.DBTX, p PaefiCaso) error
	PaefiCasosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area Area) ([]PaefiCaso, error)

	// MSE (CREAS)
	InsertMseMedida(ctx context.Context, db database.DBTX, m MseMedida, autor uuid.UUID) error
	GetMseMedida(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (MseMedida, error)
	UpdateMseMedida(ctx context.Context, db database.DBTX, m MseMedida) error
	MseMedidasNaArea(ctx context.Context, db database.DBTX, adolescente uuid.UUID, area Area) ([]MseMedida, error)

	// Notificações (CREAS)
	InsertNotificacaoCreas(ctx context.Context, db database.DBTX, n NotificacaoCreas, autor uuid.UUID) error
	GetNotificacaoCreas(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (NotificacaoCreas, error)
	UpdateNotificacaoCreas(ctx context.Context, db database.DBTX, n NotificacaoCreas) error
	NotificacoesCreasNaArea(ctx context.Context, db database.DBTX, vitima uuid.UUID, area Area) ([]NotificacaoCreas, error)

	// Casa Abrigo
	InsertAcolhimento(ctx context.Context, db database.DBTX, a Acolhimento, autor uuid.UUID) error
	GetAcolhimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (Acolhimento, error)
	UpdateAcolhimento(ctx context.Context, db database.DBTX, a Acolhimento) error
	AcolhimentosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area Area) ([]Acolhimento, error)
	InsertIrmao(ctx context.Context, db database.DBTX, i Irmao) error
	ListIrmaos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]Irmao, error)
	GetIrmao(ctx context.Context, db database.DBTX, id uuid.UUID) (Irmao, error)
	DeleteIrmao(ctx context.Context, db database.DBTX, id uuid.UUID) error
	InsertDiagnostico(ctx context.Context, db database.DBTX, d Diagnostico) error
	ListDiagnosticos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]Diagnostico, error)
	GetDiagnostico(ctx context.Context, db database.DBTX, id uuid.UUID) (Diagnostico, error)
	DeleteDiagnostico(ctx context.Context, db database.DBTX, id uuid.UUID) error
	InsertVisitante(ctx context.Context, db database.DBTX, v Visitante, autor uuid.UUID) error
	ListVisitantes(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]Visitante, error)
	GetVisitante(ctx context.Context, db database.DBTX, id uuid.UUID) (Visitante, error)
	DeleteVisitante(ctx context.Context, db database.DBTX, id uuid.UUID) error
	InsertTermo(ctx context.Context, db database.DBTX, t Termo, autor uuid.UUID) error
	ListTermos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]Termo, error)
	GetTermo(ctx context.Context, db database.DBTX, id uuid.UUID) (Termo, error)
	DeleteTermo(ctx context.Context, db database.DBTX, id uuid.UUID) error
	UpsertPiaFicha(ctx context.Context, db database.DBTX, p PiaFicha, autor uuid.UUID) error
	GetPiaFicha(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) (PiaFicha, error)
	InsertPiaMeta(ctx context.Context, db database.DBTX, m PiaMeta, autor uuid.UUID) error
	ListPiaMetas(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]PiaMeta, error)
	GetPiaMeta(ctx context.Context, db database.DBTX, id uuid.UUID) (PiaMeta, error)
	UpdatePiaMeta(ctx context.Context, db database.DBTX, m PiaMeta) error
	DeletePiaMeta(ctx context.Context, db database.DBTX, id uuid.UUID) error
	InsertDocumento(ctx context.Context, db database.DBTX, d DocumentoCasaAbrigo, autor uuid.UUID) error
	GetDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) (DocumentoCasaAbrigo, error)
	UpdateDocumentoStatus(ctx context.Context, db database.DBTX, id uuid.UUID, status string, sizeBytes int64, contentType string) error
	ListDocumentos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]DocumentoCasaAbrigo, error)
	DeleteDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) error
}
