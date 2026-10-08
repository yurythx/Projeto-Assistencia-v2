// Package outbox implementa o padrão Transactional Outbox (§16): um evento
// é gravado na tabela outbox_events dentro da mesma transação PostgreSQL
// que os dados de negócio que ele descreve, para que uma falha parcial
// (dado commitado mas RabbitMQ inalcançável, ou vice-versa) nunca possa
// acontecer. Um Publisher separado faz polling das linhas pendentes e as
// encaminha ao RabbitMQ, marcando cada uma como publicada somente depois
// que o broker confirma o recebimento.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
)

// Writer insere linhas no outbox. Não recebe um handle de banco próprio —
// quem chama passa o pgx.Tx da transação de negócio em que já está, para
// que o insert no outbox seja atômico com qualquer dado de negócio que
// acabou de ser gravado nessa mesma transação (ex.: criar um Job e seu
// evento "job.created" juntos, ou os dois ou nenhum dos dois).
type Writer struct {
	source string // events.Event.Source para todo evento que este writer constrói, ex.: "nexus.example"
}

// NewWriter constrói um Writer que carimba todo evento com source.
func NewWriter(source string) *Writer {
	return &Writer{source: source}
}

// Channel é o canal LISTEN/NOTIFY do PostgreSQL que o Writer sinaliza a
// cada evento gravado e que o Publisher escuta para despachar sem esperar
// o próximo tick do polling (latência ~zero, §16). O NOTIFY roda DENTRO da
// mesma transação do INSERT, então só dispara quando o evento de fato
// commita — nunca acorda o Publisher para uma linha que sofreu rollback.
const Channel = "nexus_outbox_channel"

// Write monta o envelope de evento padrão (§17), o valida contra o JSON
// Schema do contrato de evento (§ Schema Validator para Eventos do
// Outbox — ver schema.go) e o insere em outbox_events dentro de tx.
// Precisa ser chamado antes de tx.Commit(); o evento só se torna visível
// ao Publisher depois que a transação em volta é commitada — é essa
// visibilidade adiada que garante a atomicidade do padrão outbox.
func (w *Writer) Write(ctx context.Context, tx pgx.Tx, eventType, aggregateType, aggregateID string, correlationID uuid.UUID, payload any) error {
	event, err := events.New(eventType, w.source, correlationID, payload)
	if err != nil {
		return fmt.Errorf("outbox: build event envelope: %w", err)
	}

	// Não falha: events.New já serializou o payload (json.RawMessage válido).
	envelope, _ := json.Marshal(event)

	// Validado depois de serializar, contra o MESMO JSON que será
	// gravado — não contra o struct Go em memória — para pegar qualquer
	// divergência introduzida pela própria serialização (ex.: um futuro
	// campo com tag `json:"-"` que devesse ter sido incluído) e não só
	// erros de construção do struct events.Event.
	if err := validateEnvelope(envelope); err != nil {
		return fmt.Errorf("outbox: invalid event envelope for %s: %w", eventType, err)
	}

	const q = `
		INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload, status, attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6)
	`
	if _, err := tx.Exec(ctx, q, event.ID, eventType, aggregateType, aggregateID, envelope, event.OccurredAt); err != nil {
		return fmt.Errorf("outbox: insert event %s: %w", eventType, err)
	}

	// Acorda o Publisher assim que esta transação commitar (o NOTIFY do
	// PostgreSQL é entregue no commit, não no Exec). Falha aqui não é
	// fatal — o polling de segurança do Publisher pega a linha de qualquer
	// forma —, mas logar seria no nível do chamador; propagamos o erro
	// para manter a transação consistente (se o NOTIFY falhou, algo mais
	// grave está acontecendo na conexão).
	if _, err := tx.Exec(ctx, "SELECT pg_notify($1, '')", Channel); err != nil {
		return fmt.Errorf("outbox: notify %s: %w", Channel, err)
	}
	return nil
}
