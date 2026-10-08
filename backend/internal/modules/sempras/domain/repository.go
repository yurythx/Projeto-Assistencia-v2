package domain

import (
	"context"

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

	// Prontuário
	InsertRegistro(ctx context.Context, db database.DBTX, r Registro) error
	RegistrosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area Area) ([]Registro, error)
}
