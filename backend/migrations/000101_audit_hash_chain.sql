-- +goose Up
-- Auditoria imutável com proveniência completa e encadeamento SHA-256
-- (LGPD art. 37 / skill §5):
--   actor_id, actor_subject, actor_roles, ip_address, user_agent,
--   entity_context, action, resource_type/resource_id, diff_before,
--   diff_after, created_at (UTC), prev_hash, hash.
--
-- O hash é calculado DENTRO do Postgres (trigger BEFORE INSERT), não na
-- aplicação: assim toda escrita — inclusive as feitas por outros
-- triggers (consentimento LGPD, solicitações do titular) — entra na
-- cadeia, e a fórmula existe em um único lugar (audit_row_digest), usada
-- tanto para gravar quanto para verificar (audit_verify_chain).
--
-- Serialização: pg_advisory_xact_lock garante que só uma transação por
-- vez estenda a cadeia; o lock é liberado no COMMIT/ROLLBACK. chain_pos é
-- atribuído depois do lock (não por sequence), então a ordem da cadeia é
-- exatamente a ordem de commit.

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_user_id_fkey;
ALTER TABLE audit_logs RENAME COLUMN user_id TO actor_id;
ALTER INDEX IF EXISTS idx_audit_logs_user_id RENAME TO idx_audit_logs_actor_id;

ALTER TABLE audit_logs
    ADD COLUMN actor_subject  TEXT,
    ADD COLUMN actor_roles    TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN user_agent     TEXT,
    ADD COLUMN entity_context JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN diff_before    JSONB,
    ADD COLUMN diff_after     JSONB,
    ADD COLUMN chain_pos      BIGINT,
    ADD COLUMN prev_hash      TEXT,
    ADD COLUMN hash           TEXT;

-- Serialização canônica e inequívoca da linha (array JSON: um separador
-- dentro de um campo nunca se confunde com a fronteira entre campos).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_row_digest(p audit_logs)
RETURNS TEXT
LANGUAGE sql STABLE AS $$
    SELECT encode(digest(jsonb_build_array(
        p.chain_pos,
        p.id,
        p.actor_id,
        p.actor_subject,
        to_jsonb(p.actor_roles),
        host(p.ip_address),
        p.user_agent,
        p.entity_context,
        p.action,
        p.resource_type,
        p.resource_id,
        p.diff_before,
        p.diff_after,
        p.metadata,
        p.correlation_id,
        to_char(p.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
        p.prev_hash
    )::text, 'sha256'), 'hex')
$$;
-- +goose StatementEnd

-- Backfill: encadeia as linhas já existentes na ordem em que foram
-- gravadas. É a ÚNICA vez em que audit_logs sofre UPDATE — com o gatilho
-- de imutabilidade desligado só dentro desta migration.
ALTER TABLE audit_logs DISABLE TRIGGER trg_audit_logs_immutable;

-- +goose StatementBegin
DO $$
DECLARE
    r        audit_logs;
    v_pos    BIGINT := 0;
    v_prev   TEXT := repeat('0', 64);
BEGIN
    FOR r IN SELECT * FROM audit_logs ORDER BY created_at, id LOOP
        v_pos := v_pos + 1;
        r.chain_pos := v_pos;
        r.prev_hash := v_prev;
        r.hash := audit_row_digest(r);
        UPDATE audit_logs SET chain_pos = r.chain_pos, prev_hash = r.prev_hash, hash = r.hash WHERE id = r.id;
        v_prev := r.hash;
    END LOOP;
END $$;
-- +goose StatementEnd

ALTER TABLE audit_logs ENABLE TRIGGER trg_audit_logs_immutable;

ALTER TABLE audit_logs
    ALTER COLUMN chain_pos SET NOT NULL,
    ALTER COLUMN prev_hash SET NOT NULL,
    ALTER COLUMN hash SET NOT NULL;
CREATE UNIQUE INDEX idx_audit_logs_chain_pos ON audit_logs (chain_pos);
CREATE INDEX idx_audit_logs_resource ON audit_logs (resource_type, resource_id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_logs_chain()
RETURNS TRIGGER AS $$
DECLARE
    v_pos  BIGINT;
    v_hash TEXT;
BEGIN
    -- Chave fixa do lock da cadeia de auditoria do Nexus.
    PERFORM pg_advisory_xact_lock(7300101);
    SELECT chain_pos, hash INTO v_pos, v_hash
      FROM audit_logs ORDER BY chain_pos DESC LIMIT 1;

    NEW.chain_pos      := COALESCE(v_pos, 0) + 1;
    NEW.prev_hash      := COALESCE(v_hash, repeat('0', 64));
    -- Carimbo de tempo do SERVIDOR de banco, nunca o do cliente.
    NEW.created_at     := clock_timestamp();
    NEW.actor_roles    := COALESCE(NEW.actor_roles, '{}');
    NEW.entity_context := COALESCE(NEW.entity_context, '{}'::jsonb);
    NEW.metadata       := COALESCE(NEW.metadata, '{}'::jsonb);
    NEW.hash           := audit_row_digest(NEW);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_audit_logs_chain
    BEFORE INSERT ON audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION audit_logs_chain();
-- +goose StatementEnd

-- Verificação de integridade: recalcula cada hash e confere o elo com o
-- anterior. Devolve na primeira quebra encontrada (valid = false).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_verify_chain(p_from BIGINT DEFAULT 1, p_limit BIGINT DEFAULT NULL)
RETURNS TABLE (checked BIGINT, valid BOOLEAN, first_invalid_pos BIGINT, reason TEXT)
LANGUAGE plpgsql STABLE AS $$
DECLARE
    r       audit_logs;
    v_prev  TEXT;
    v_count BIGINT := 0;
BEGIN
    IF p_from <= 1 THEN
        v_prev := repeat('0', 64);
    ELSE
        SELECT hash INTO v_prev FROM audit_logs WHERE chain_pos = p_from - 1;
        IF v_prev IS NULL THEN
            RETURN QUERY SELECT 0::BIGINT, false, p_from - 1, 'elo anterior ausente'::TEXT;
            RETURN;
        END IF;
    END IF;

    FOR r IN
        SELECT * FROM audit_logs WHERE chain_pos >= GREATEST(p_from, 1)
        ORDER BY chain_pos
        LIMIT p_limit
    LOOP
        v_count := v_count + 1;
        IF r.chain_pos <> GREATEST(p_from, 1) + v_count - 1 THEN
            RETURN QUERY SELECT v_count, false, r.chain_pos, 'lacuna na sequência da cadeia'::TEXT;
            RETURN;
        END IF;
        IF r.prev_hash <> v_prev THEN
            RETURN QUERY SELECT v_count, false, r.chain_pos, 'prev_hash não corresponde ao hash anterior'::TEXT;
            RETURN;
        END IF;
        IF r.hash <> audit_row_digest(r) THEN
            RETURN QUERY SELECT v_count, false, r.chain_pos, 'conteúdo alterado (hash divergente)'::TEXT;
            RETURN;
        END IF;
        v_prev := r.hash;
    END LOOP;

    RETURN QUERY SELECT v_count, true, NULL::BIGINT, NULL::TEXT;
END;
$$;
-- +goose StatementEnd

-- Os gatilhos de LGPD do baseline gravavam em audit_logs.user_id —
-- redefinidos para a coluna renomeada.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_lgpd_consent_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, metadata, ip_address, user_agent)
    VALUES (NEW.user_id, 'lgpd_consent_given', 'user_consents', NEW.id::text,
            jsonb_build_object('term_version', NEW.term_version),
            NEW.ip_address, NEW.user_agent);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_anonymous_consent_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, metadata, ip_address, user_agent)
    VALUES (NULL, 'lgpd_consent_given_anon', 'anonymous_consents', NEW.device_hash,
            jsonb_build_object('term_version', NEW.term_version),
            NEW.ip_address, NEW.user_agent);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_data_subject_request_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, metadata, ip_address)
    VALUES (NEW.user_id, 'lgpd.data_subject_request.' || NEW.kind, 'data_subject_request', NEW.id::text,
            jsonb_build_object('kind', NEW.kind, 'status', NEW.status),
            NEW.requested_ip);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS audit_verify_chain(BIGINT, BIGINT);
DROP TRIGGER IF EXISTS trg_audit_logs_chain ON audit_logs;
DROP FUNCTION IF EXISTS audit_logs_chain();
DROP INDEX IF EXISTS idx_audit_logs_resource;
DROP INDEX IF EXISTS idx_audit_logs_chain_pos;
DROP FUNCTION IF EXISTS audit_row_digest(audit_logs);
ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS hash,
    DROP COLUMN IF EXISTS prev_hash,
    DROP COLUMN IF EXISTS chain_pos,
    DROP COLUMN IF EXISTS diff_after,
    DROP COLUMN IF EXISTS diff_before,
    DROP COLUMN IF EXISTS entity_context,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS actor_roles,
    DROP COLUMN IF EXISTS actor_subject;
ALTER INDEX IF EXISTS idx_audit_logs_actor_id RENAME TO idx_audit_logs_user_id;
ALTER TABLE audit_logs RENAME COLUMN actor_id TO user_id;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_lgpd_consent_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata, ip_address)
    VALUES (NEW.user_id, 'lgpd_consent_given', 'user_consents', NEW.id::text,
            jsonb_build_object('term_version', NEW.term_version, 'user_agent', COALESCE(NEW.user_agent, 'unknown')),
            NEW.ip_address);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_anonymous_consent_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata, ip_address)
    VALUES (NULL, 'lgpd_consent_given_anon', 'anonymous_consents', NEW.device_hash,
            jsonb_build_object('term_version', NEW.term_version, 'user_agent', COALESCE(NEW.user_agent, 'unknown')),
            NEW.ip_address);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_data_subject_request_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata, ip_address)
    VALUES (NEW.user_id, 'lgpd.data_subject_request.' || NEW.kind, 'data_subject_request', NEW.id::text,
            jsonb_build_object('kind', NEW.kind, 'status', NEW.status), NEW.requested_ip);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
