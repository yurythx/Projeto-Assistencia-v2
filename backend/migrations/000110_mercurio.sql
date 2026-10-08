-- +goose Up
-- Plugin Mercúrio — chat em tempo real (canais globais, salas
-- departamentais mapeadas a grupos do AD e mensagens diretas). A entrega
-- em tempo real passa pelo Hub WebSocket com backplane Redis; esta é a
-- persistência durável.

CREATE TABLE mercurio_rooms (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind             TEXT NOT NULL CONSTRAINT mercurio_rooms_kind_check
                         CHECK (kind IN ('global', 'department', 'direct')),
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    ad_group         TEXT NOT NULL DEFAULT '',
    departamento_id  UUID REFERENCES departamentos (id) ON DELETE SET NULL,
    -- "uuidMenor:uuidMaior" — garante uma única sala direta por par.
    dm_key           TEXT UNIQUE,
    archived         BOOLEAN NOT NULL DEFAULT false,
    created_by       UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT mercurio_rooms_direct_has_key CHECK (kind <> 'direct' OR dm_key IS NOT NULL),
    CONSTRAINT mercurio_rooms_department_scope CHECK (
        kind <> 'department' OR ad_group <> '' OR departamento_id IS NOT NULL
    )
);
CREATE INDEX idx_mercurio_rooms_kind ON mercurio_rooms (kind) WHERE NOT archived;
CREATE INDEX idx_mercurio_rooms_ad_group ON mercurio_rooms (lower(ad_group)) WHERE kind = 'department';

-- +goose StatementBegin
CREATE TRIGGER trg_mercurio_rooms_set_updated_at BEFORE UPDATE ON mercurio_rooms
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- Participantes explícitos (salas diretas). Salas globais/departamentais
-- resolvem a participação dinamicamente (grupos do AD / lotação).
CREATE TABLE mercurio_room_members (
    room_id    UUID NOT NULL REFERENCES mercurio_rooms (id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);
CREATE INDEX idx_mercurio_room_members_user ON mercurio_room_members (user_id);

CREATE TABLE mercurio_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id     UUID NOT NULL REFERENCES mercurio_rooms (id) ON DELETE CASCADE,
    author_id   UUID NOT NULL REFERENCES users (id),
    body        TEXT NOT NULL CONSTRAINT mercurio_messages_body_len CHECK (char_length(body) BETWEEN 1 AND 4000),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at   TIMESTAMPTZ,
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX idx_mercurio_messages_room_created ON mercurio_messages (room_id, created_at DESC);

CREATE TABLE mercurio_reads (
    room_id       UUID NOT NULL REFERENCES mercurio_rooms (id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    last_read_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);

INSERT INTO mercurio_rooms (kind, name, description)
VALUES ('global', 'Geral', 'Canal aberto a todos os colaboradores autenticados.');

-- +goose Down
DROP TABLE IF EXISTS mercurio_reads;
DROP TABLE IF EXISTS mercurio_messages;
DROP TABLE IF EXISTS mercurio_room_members;
DROP TABLE IF EXISTS mercurio_rooms;
