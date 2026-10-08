-- +goose Up
-- Baseline da plataforma Projeto Nexus (base genérica).
-- Consolida as migrations 000001–000042 no estado final: só as tabelas do
-- núcleo — identidade (users + login local + lockout), jobs, outbox
-- transacional, auditoria imutável, integrações, rate limiting distribuído,
-- chaves de idempotência e feature flags. Sem nenhuma tabela de produto
-- derivado (contratos, demandas/kanban, diário oficial, scanning): um
-- projeto que parte desta base adiciona as suas em migrations próprias.

-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- +goose StatementEnd

-- Trigger compartilhado: toda tabela com updated_at usa esta função para
-- mantê-la atualizada em todo UPDATE.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- ============================================================
-- users — identidade. keycloak_subject (nulo para conta de login local),
-- password_hash/roles (só usados por conta local; contas do Keycloak têm
-- roles do próprio token), lockout por tentativas.
-- ============================================================
CREATE TABLE users (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    keycloak_subject      TEXT,
    username              TEXT NOT NULL,
    email                 TEXT NOT NULL,
    display_name          TEXT NOT NULL DEFAULT '',
    active                BOOLEAN NOT NULL DEFAULT true,
    password_hash         TEXT,
    roles                 TEXT[] NOT NULL DEFAULT '{}',
    failed_login_attempts INT NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at          TIMESTAMPTZ,
    CONSTRAINT users_has_auth_method CHECK (keycloak_subject IS NOT NULL OR password_hash IS NOT NULL)
);

CREATE UNIQUE INDEX idx_users_keycloak_subject ON users (keycloak_subject);
CREATE UNIQUE INDEX idx_users_email ON users (email);
CREATE UNIQUE INDEX idx_users_username ON users (username);
CREATE INDEX idx_users_active ON users (active);

-- +goose StatementBegin
CREATE TRIGGER trg_users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- Nenhum usuário admin é semeado — um hash de senha conhecido num script
-- versionado é vulnerabilidade. Use `make seed-admin` (cmd/seedadmin).

-- ============================================================
-- jobs — rastreamento de processamento assíncrono de longa duração.
-- ============================================================
CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'queued'
                        CONSTRAINT jobs_status_check
                        CHECK (status IN ('queued', 'processing', 'completed', 'failed', 'dead_letter')),
    attempts        INTEGER NOT NULL DEFAULT 0,
    payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
    result          JSONB,
    error           TEXT,
    correlation_id  UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ
);

CREATE INDEX idx_jobs_type ON jobs (type);
CREATE INDEX idx_jobs_status ON jobs (status);
CREATE INDEX idx_jobs_correlation_id ON jobs (correlation_id);
CREATE INDEX idx_jobs_created_at ON jobs (created_at DESC);
CREATE INDEX idx_jobs_type_status ON jobs (type, status);

-- ============================================================
-- outbox_events — Transactional Outbox: todo evento de domínio nasce na
-- mesma transação do dado de negócio que o originou.
-- ============================================================
CREATE TABLE outbox_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type      TEXT NOT NULL,
    aggregate_type  TEXT NOT NULL,
    aggregate_id    TEXT NOT NULL,
    payload         JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending'
                        CONSTRAINT outbox_events_status_check
                        CHECK (status IN ('pending', 'published', 'failed')),
    attempts        INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ,
    last_error      TEXT
);

CREATE INDEX idx_outbox_events_pending ON outbox_events (created_at) WHERE status = 'pending';
CREATE INDEX idx_outbox_events_aggregate ON outbox_events (aggregate_type, aggregate_id);
CREATE INDEX idx_outbox_events_correlation_id ON outbox_events (((payload ->> 'correlation_id')));

-- ============================================================
-- audit_logs — trilha imutável (append-only via os gatilhos abaixo).
-- ============================================================
CREATE TABLE audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users (id) ON DELETE SET NULL,
    action          TEXT NOT NULL,
    resource_type   TEXT,
    resource_id     TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    correlation_id  UUID,
    ip_address      INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs (user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs (action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_logs_correlation_id ON audit_logs (correlation_id);

-- Recusa UPDATE/DELETE/TRUNCATE em audit_logs, não importa a role.
-- Ressalva: um superusuário do Postgres ainda pode remover o próprio
-- gatilho — para garantia mais forte, exportar para storage WORM.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_audit_logs_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only: % is not allowed (attempted on id=%)',
        TG_OP,
        COALESCE(OLD.id, NEW.id);
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_audit_logs_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_logs_mutation();
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_audit_logs_truncate()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only: TRUNCATE is not allowed';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_audit_logs_immutable_truncate
    BEFORE TRUNCATE ON audit_logs
    FOR EACH STATEMENT
    EXECUTE FUNCTION prevent_audit_logs_truncate();
-- +goose StatementEnd

-- ============================================================
-- integrations — registro genérico de integrações externas (começa vazio;
-- um projeto derivado semeia as suas).
-- ============================================================
CREATE TABLE integrations (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key              TEXT NOT NULL,
    name             TEXT NOT NULL,
    type             TEXT NOT NULL,
    enabled          BOOLEAN NOT NULL DEFAULT true,
    status           TEXT NOT NULL DEFAULT 'unknown'
                         CONSTRAINT integrations_status_check
                         CHECK (status IN ('unknown', 'online', 'offline', 'degraded', 'disabled')),
    last_check_at    TIMESTAMPTZ,
    last_success_at  TIMESTAMPTZ,
    last_error       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_integrations_key ON integrations (key);
CREATE INDEX idx_integrations_type ON integrations (type);

-- +goose StatementBegin
CREATE TRIGGER trg_integrations_set_updated_at
    BEFORE UPDATE ON integrations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- ============================================================
-- rate_limit_buckets — rate limiting distribuído (janela fixa),
-- compartilhado por todas as réplicas da API. `bucket` namespaceia cada
-- PostgresLimiter (evita dois limiters com o mesmo `key` se anularem).
-- ============================================================
CREATE TABLE rate_limit_buckets (
    bucket       TEXT NOT NULL,
    key          TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    count        INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket, key)
);

CREATE INDEX idx_rate_limit_buckets_window_start ON rate_limit_buckets (window_start);

-- ============================================================
-- idempotency_keys — Idempotency-Key Middleware. `key` é
-- "<subject>:<header>", escopada por usuário.
-- ============================================================
CREATE TABLE idempotency_keys (
    key             TEXT PRIMARY KEY,
    request_hash    TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'processing'
                        CONSTRAINT idempotency_keys_status_check
                        CHECK (status IN ('processing', 'completed', 'failed')),
    response_status INTEGER,
    response_body   BYTEA,
    content_type    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_idempotency_keys_updated_at ON idempotency_keys (updated_at);

-- ============================================================
-- feature_flags — alteráveis em runtime, sem reiniciar api/worker.
-- ============================================================
CREATE TABLE feature_flags (
    key         TEXT PRIMARY KEY,
    enabled     BOOLEAN NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  TEXT
);

INSERT INTO feature_flags (key, enabled, description) VALUES
    ('module_exemplos_enabled', true, 'Habilita a exibição e uso do Módulo Modelo (Exemplo Blueprint).');

-- +goose Down
DROP TABLE IF EXISTS feature_flags;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS rate_limit_buckets;
DROP TRIGGER IF EXISTS trg_integrations_set_updated_at ON integrations;
DROP TABLE IF EXISTS integrations;
DROP TRIGGER IF EXISTS trg_audit_logs_immutable_truncate ON audit_logs;
DROP TRIGGER IF EXISTS trg_audit_logs_immutable ON audit_logs;
DROP FUNCTION IF EXISTS prevent_audit_logs_truncate();
DROP FUNCTION IF EXISTS prevent_audit_logs_mutation();
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS jobs;
DROP TRIGGER IF EXISTS trg_users_set_updated_at ON users;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
DROP EXTENSION IF EXISTS pgcrypto;
