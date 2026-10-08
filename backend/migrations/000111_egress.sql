-- +goose Up
-- Plugin Egress — webhooks de saída assíncronos. O dispatcher consome o
-- barramento de eventos (RabbitMQ, fila ligada a "#") e materializa uma
-- entrega por destino compatível; o worker de entrega faz o POST
-- assinado (HMAC-SHA256) através do cliente HTTP anti-SSRF.

CREATE TABLE egress_targets (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    kind              TEXT NOT NULL DEFAULT 'webhook' CONSTRAINT egress_targets_kind_check
                          CHECK (kind IN ('webhook', 'n8n', 'zabbix', 'grafana')),
    url               TEXT NOT NULL,
    -- segredo HMAC cifrado com AES-256-GCM (CONFIG_ENCRYPTION_KEY)
    secret_encrypted  TEXT NOT NULL DEFAULT '',
    -- padrões no formato de routing key do RabbitMQ ("*" = uma palavra, "#" = zero ou mais)
    event_patterns    TEXT[] NOT NULL DEFAULT ARRAY['#'],
    active            BOOLEAN NOT NULL DEFAULT true,
    created_by        TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose StatementBegin
CREATE TRIGGER trg_egress_targets_set_updated_at BEFORE UPDATE ON egress_targets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

CREATE TABLE egress_deliveries (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id         UUID NOT NULL REFERENCES egress_targets (id) ON DELETE CASCADE,
    event_id          UUID NOT NULL,
    event_type        TEXT NOT NULL,
    payload           JSONB NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending' CONSTRAINT egress_deliveries_status_check
                          CHECK (status IN ('pending', 'delivered', 'failed', 'dead')),
    attempts          INTEGER NOT NULL DEFAULT 0,
    next_attempt_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_status_code  INTEGER,
    last_error        TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at      TIMESTAMPTZ,
    -- idempotência do dispatcher: um evento reentregue não duplica a entrega
    CONSTRAINT uq_egress_deliveries_target_event UNIQUE (target_id, event_id)
);
CREATE INDEX idx_egress_deliveries_due ON egress_deliveries (next_attempt_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX idx_egress_deliveries_target_created ON egress_deliveries (target_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS egress_deliveries;
DROP TABLE IF EXISTS egress_targets;
