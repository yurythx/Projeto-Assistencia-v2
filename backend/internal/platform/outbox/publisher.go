package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Publisher faz polling de outbox_events buscando linhas pendentes e as
// encaminha ao RabbitMQ via o events.EventPublisher injetado, marcando
// cada linha como "published" somente depois que o broker a confirma
// (§14/§16). Nunca importa o pacote messaging diretamente — só a
// interface de domínio — o que permite testá-lo com um publisher falso
// (fake), sem precisar de um RabbitMQ real no teste.
type Publisher struct {
	pool           *pgxpool.Pool
	eventPublisher events.EventPublisher
	logger         *slog.Logger

	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	// retryBase/retryMax: backoff exponencial entre tentativas de UM evento
	// (base, 2×base, 4×base… até retryMax).
	retryBase, retryMax time.Duration
	// listenMin/listenMax: reconexão do LISTEN.
	listenMin, listenMax time.Duration
	channel              string
	begin                func(context.Context) (pgx.Tx, error)
}

// NewPublisher constrói um Publisher de outbox com padrões razoáveis: é
// dirigido por LISTEN/NOTIFY no canal outbox.Channel (latência ~zero) com
// um polling de segurança a cada 15s como rede contra um NOTIFY perdido
// (ex.: janela de reconexão do listener). Até 20 linhas por lote. Uma
// publicação que falha reagenda o evento com backoff exponencial (1s, 2s,
// 4s… até 1h) e só o marca "failed" depois de 20 tentativas — cerca de 9h
// de broker fora do ar. Eventos "failed" voltam à fila por Requeue.
func NewPublisher(pool *pgxpool.Pool, eventPublisher events.EventPublisher, logger *slog.Logger) *Publisher {
	return &Publisher{
		pool:           pool,
		eventPublisher: eventPublisher,
		logger:         logger,
		pollInterval:   15 * time.Second,
		batchSize:      20,
		maxAttempts:    20,
		retryBase:      time.Second,
		retryMax:       time.Hour,
		listenMin:      time.Second,
		listenMax:      30 * time.Second,
		channel:        Channel,
		begin:          pool.Begin,
	}
}

// Run despacha o outbox até ctx ser cancelado. Três gatilhos alimentam um
// único loop de despacho: (1) um poll imediato no start, para drenar o que
// já estava pendente; (2) NOTIFY no canal outbox.Channel, entregue no
// commit da transação que gravou o evento; (3) um ticker de segurança.
// Registrado como um dos processadores de segundo plano do cmd/worker.
func (p *Publisher) Run(ctx context.Context) error {
	wake := make(chan struct{}, 1)
	signal := func() {
		select {
		case wake <- struct{}{}:
		default: // já há um wake pendente — coalesce
		}
	}

	go p.listenLoop(ctx, signal)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	signal() // drena o backlog no boot

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			signal()
		case <-wake:
			if err := p.publishPendingBatch(ctx); err != nil {
				p.logger.Error("outbox: publish pending batch failed", slog.Any("error", err))
			}
		}
	}
}

// listenLoop mantém um LISTEN outbox.Channel numa conexão dedicada,
// reconectando com backoff se a conexão cair. Cada notificação chama
// signal() para o loop de Run() despachar. Retorna só quando ctx é
// cancelado.
func (p *Publisher) listenLoop(ctx context.Context, signal func()) {
	backoff := p.listenMin
	for {
		listening, err := p.listenOnce(ctx, signal)
		if ctx.Err() != nil {
			return
		}
		if listening {
			backoff = p.listenMin // caiu depois de conectado: reconecta rápido
		}
		p.logger.Warn("outbox: LISTEN encerrou, reconectando",
			slog.Any("error", err), slog.Duration("retry_in", backoff))
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, p.listenMax)
	}
}

// listenOnce devolve listening=true se o LISTEN chegou a ser estabelecido.
func (p *Publisher) listenOnce(ctx context.Context, signal func()) (bool, error) {
	poolConn, err := p.pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("acquire listen conn: %w", err)
	}
	// Hijack: tira a conexão do pool em definitivo — uma conexão com LISTEN
	// pendente não pode voltar ao pool para servir queries comuns. Nós
	// gerenciamos o Close.
	conn := poolConn.Hijack()
	defer conn.Close(context.Background())

	if _, err := conn.Exec(ctx, "LISTEN "+p.channel); err != nil {
		return false, fmt.Errorf("LISTEN %s: %w", p.channel, err)
	}
	p.logger.Info("outbox: publisher escutando LISTEN/NOTIFY", slog.String("channel", p.channel))

	// Após (re)conectar, força um poll — um NOTIFY pode ter ocorrido
	// enquanto não havia listener.
	signal()

	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			return true, err
		}
		signal()
	}
}

type outboxRow struct {
	id       uuid.UUID
	payload  []byte
	attempts int
}

// publishPendingBatch trava até batchSize linhas pendentes com SELECT ...
// FOR UPDATE SKIP LOCKED (seguro para múltiplas réplicas do worker fazendo
// polling ao mesmo tempo — cada uma pega um conjunto disjunto de linhas,
// sem duas réplicas processarem a mesma linha) e, para cada uma, publica
// e atualiza seu status dentro da mesma transação.
func (p *Publisher) publishPendingBatch(ctx context.Context) error {
	tx, err := p.begin(ctx)
	if err != nil {
		return fmt.Errorf("outbox: begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // vira no-op depois de um commit bem-sucedido

	const selectQ = `
		SELECT id, payload, attempts
		FROM outbox_events
		WHERE status = 'pending' AND next_attempt_at <= now()
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, selectQ, p.batchSize)
	if err != nil {
		return fmt.Errorf("outbox: select pending: %w", err)
	}

	var pending []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.id, &r.payload, &r.attempts); err != nil {
			rows.Close()
			return fmt.Errorf("outbox: scan pending row: %w", err)
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("outbox: iterate pending rows: %w", err)
	}

	for _, row := range pending {
		stop, err := p.publishRow(ctx, tx, row)
		if err != nil {
			// A transação ficou inutilizável: reverte o lote inteiro (as
			// linhas já publicadas voltam a pendentes e são reenviadas —
			// os consumidores deduplicam por event_id).
			return err
		}
		if stop {
			break
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("outbox: commit batch: %w", err)
	}
	return nil
}

// publishRow tenta publicar uma linha e ajusta seu status:
//   - envelope ilegível vai direto para "failed" (nunca teria sucesso);
//   - sucesso marca "published";
//   - falha de publicação soma uma tentativa e reagenda com backoff; no
//     limite, "failed". Devolve stop=true: se o broker recusou este
//     evento, provavelmente recusará os próximos — o lote para em vez de
//     queimar uma tentativa de cada linha pendente.
//
// Um erro devolvido é do banco (UPDATE do status) e aborta o lote.
func (p *Publisher) publishRow(ctx context.Context, tx database.DBTX, row outboxRow) (stop bool, err error) {
	var event events.Event
	if err := json.Unmarshal(row.payload, &event); err != nil {
		p.logger.Error("outbox: undecodable envelope, marking failed", slog.String("outbox_id", row.id.String()), slog.Any("error", err))
		return false, p.markFailed(ctx, tx, row.id, "undecodable envelope: "+err.Error())
	}

	if err := p.eventPublisher.Publish(ctx, event); err != nil {
		attempts := row.attempts + 1
		if attempts >= p.maxAttempts {
			p.logger.Error("outbox: exceeded max publish attempts, marking failed",
				slog.String("outbox_id", row.id.String()), slog.String("event_type", event.Type), slog.Any("error", err))
			return true, p.markFailed(ctx, tx, row.id, err.Error())
		}
		retryIn := p.retryDelay(attempts)
		p.logger.Warn("outbox: publish attempt failed, rescheduled",
			slog.String("outbox_id", row.id.String()), slog.String("event_type", event.Type),
			slog.Int("attempts", attempts), slog.Duration("retry_in", retryIn), slog.Any("error", err))
		return true, p.markAttemptFailed(ctx, tx, row.id, attempts, retryIn, err.Error())
	}

	return false, p.markPublished(ctx, tx, row.id)
}

// retryDelay é o backoff da n-ésima tentativa falha: base·2^(n-1), até retryMax.
func (p *Publisher) retryDelay(attempts int) time.Duration {
	d := p.retryBase
	for i := 1; i < attempts && d < p.retryMax; i++ {
		d *= 2
	}
	return min(d, p.retryMax)
}

func (p *Publisher) markPublished(ctx context.Context, tx database.DBTX, id uuid.UUID) error {
	const q = `UPDATE outbox_events SET status = 'published', published_at = now() WHERE id = $1`
	if _, err := tx.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("outbox: mark %s published: %w", id, err)
	}
	return nil
}

func (p *Publisher) markAttemptFailed(ctx context.Context, tx database.DBTX, id uuid.UUID, attempts int, retryIn time.Duration, lastError string) error {
	const q = `UPDATE outbox_events SET attempts = $2, last_error = $3,
		next_attempt_at = now() + $4 * interval '1 millisecond' WHERE id = $1`
	if _, err := tx.Exec(ctx, q, id, attempts, lastError, retryIn.Milliseconds()); err != nil {
		return fmt.Errorf("outbox: record attempt for %s: %w", id, err)
	}
	return nil
}

func (p *Publisher) markFailed(ctx context.Context, tx database.DBTX, id uuid.UUID, lastError string) error {
	const q = `UPDATE outbox_events SET status = 'failed', attempts = attempts + 1, last_error = $2 WHERE id = $1`
	if _, err := tx.Exec(ctx, q, id, lastError); err != nil {
		return fmt.Errorf("outbox: mark %s failed: %w", id, err)
	}
	return nil
}
