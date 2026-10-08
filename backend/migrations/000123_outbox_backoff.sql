-- +goose Up
-- outbox_events.next_attempt_at — backoff exponencial por evento no Relay.
-- Antes, cada lote tentava TODAS as linhas pendentes e queimava uma
-- tentativa por linha a cada poll (15s ou a cada NOTIFY): uma queda do
-- RabbitMQ de ~2,5 min marcava todo o backlog como 'failed', sem caminho
-- de volta. Agora uma linha só é tentada quando vence o seu próximo
-- horário, e o lote para na primeira falha de publicação.
ALTER TABLE outbox_events ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();

DROP INDEX IF EXISTS idx_outbox_events_pending;
CREATE INDEX idx_outbox_events_pending ON outbox_events (next_attempt_at, created_at) WHERE status = 'pending';

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_events_pending;
CREATE INDEX idx_outbox_events_pending ON outbox_events (created_at) WHERE status = 'pending';
ALTER TABLE outbox_events DROP COLUMN next_attempt_at;
