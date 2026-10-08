-- +goose Up
-- Plugin Signum — motor de assinatura eletrônica. Um envelope fixa o hash
-- SHA-256 do documento no momento da abertura; cada assinatura exige uma
-- cerimônia de reautenticação (desafio de uso único + senha) e grava um
-- hash de integridade que amarra documento, signatário, instante e desafio.

CREATE TABLE signum_envelopes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    document_sha256  CHAR(64) NOT NULL CONSTRAINT signum_envelopes_hash_format CHECK (document_sha256 ~ '^[0-9a-f]{64}$'),
    source_module    TEXT NOT NULL DEFAULT '',
    source_ref       TEXT NOT NULL DEFAULT '',
    sequential       BOOLEAN NOT NULL DEFAULT false,
    status           TEXT NOT NULL DEFAULT 'pending' CONSTRAINT signum_envelopes_status_check
                         CHECK (status IN ('pending', 'completed', 'refused', 'cancelled')),
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ
);
CREATE INDEX idx_signum_envelopes_source ON signum_envelopes (source_module, source_ref);

CREATE TABLE signum_signers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    envelope_id     UUID NOT NULL REFERENCES signum_envelopes (id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users (id),
    position        INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'pending' CONSTRAINT signum_signers_status_check
                        CHECK (status IN ('pending', 'signed', 'refused')),
    signed_at       TIMESTAMPTZ,
    signature_hash  CHAR(64),
    method          TEXT NOT NULL DEFAULT '',
    ip_address      INET,
    user_agent      TEXT NOT NULL DEFAULT '',
    reason          TEXT NOT NULL DEFAULT '',
    CONSTRAINT uq_signum_signers_envelope_user UNIQUE (envelope_id, user_id)
);
CREATE INDEX idx_signum_signers_user_status ON signum_signers (user_id, status);

-- Desafio de uso único da cerimônia: só o hash do nonce é persistido.
CREATE TABLE signum_challenges (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    envelope_id  UUID NOT NULL REFERENCES signum_envelopes (id) ON DELETE CASCADE,
    user_id      UUID NOT NULL,
    nonce_hash   CHAR(64) NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    used_at      TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_signum_challenges_lookup ON signum_challenges (envelope_id, user_id) WHERE used_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS signum_challenges;
DROP TABLE IF EXISTS signum_signers;
DROP TABLE IF EXISTS signum_envelopes;
