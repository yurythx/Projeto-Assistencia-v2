-- +goose Up
-- Plugin Agenda — eventos corporativos e reserva de salas. A restrição de
-- exclusão (btree_gist) impede no próprio banco duas reservas confirmadas
-- sobrepostas na mesma sala, mesmo sob concorrência.

CREATE TABLE calendar_rooms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    location    TEXT NOT NULL DEFAULT '',
    capacity    INTEGER NOT NULL DEFAULT 0 CONSTRAINT calendar_rooms_capacity_check CHECK (capacity >= 0),
    resources   TEXT[] NOT NULL DEFAULT '{}',
    active      BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose StatementBegin
CREATE TRIGGER trg_calendar_rooms_set_updated_at BEFORE UPDATE ON calendar_rooms
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

CREATE TABLE calendar_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    location      TEXT NOT NULL DEFAULT '',
    room_id       UUID REFERENCES calendar_rooms (id) ON DELETE SET NULL,
    starts_at     TIMESTAMPTZ NOT NULL,
    ends_at       TIMESTAMPTZ NOT NULL,
    all_day       BOOLEAN NOT NULL DEFAULT false,
    visibility    TEXT NOT NULL DEFAULT 'internal' CONSTRAINT calendar_events_visibility_check
                      CHECK (visibility IN ('public', 'internal', 'private')),
    status        TEXT NOT NULL DEFAULT 'confirmed' CONSTRAINT calendar_events_status_check
                      CHECK (status IN ('confirmed', 'cancelled')),
    organizer_id  UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT calendar_events_range_check CHECK (ends_at > starts_at),
    CONSTRAINT calendar_events_no_room_overlap EXCLUDE USING gist (
        room_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    ) WHERE (room_id IS NOT NULL AND status = 'confirmed')
);
CREATE INDEX idx_calendar_events_range ON calendar_events USING gist (tstzrange(starts_at, ends_at, '[)'));
CREATE INDEX idx_calendar_events_organizer ON calendar_events (organizer_id);

-- +goose StatementBegin
CREATE TRIGGER trg_calendar_events_set_updated_at BEFORE UPDATE ON calendar_events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS calendar_events;
DROP TABLE IF EXISTS calendar_rooms;
