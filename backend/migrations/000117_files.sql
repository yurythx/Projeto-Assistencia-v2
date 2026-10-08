-- +goose Up
-- Plugin Arquivos — pastas e arquivos sobre o MinIO, com permissões por
-- grupo do AD / perfil / lotação / usuário. A ACL de uma pasta vale para
-- as subpastas (herança resolvida por CTE recursiva na aplicação).

CREATE TABLE files_folders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id   UUID REFERENCES files_folders (id) ON DELETE CASCADE,
    name        TEXT NOT NULL CONSTRAINT files_folders_name_check
                    CHECK (char_length(name) BETWEEN 1 AND 200 AND name !~ '[/\\]'),
    owner_id    UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT files_folders_parent_not_self CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE UNIQUE INDEX uq_files_folders_name ON files_folders (
    COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name)
);

-- +goose StatementBegin
CREATE TRIGGER trg_files_folders_set_updated_at BEFORE UPDATE ON files_folders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

CREATE TABLE files_folder_acl (
    folder_id     UUID NOT NULL REFERENCES files_folders (id) ON DELETE CASCADE,
    subject_type  TEXT NOT NULL CONSTRAINT files_folder_acl_subject_type_check
                      CHECK (subject_type IN ('everyone', 'user', 'perfil', 'ad_group', 'unidade', 'departamento')),
    subject       TEXT NOT NULL,
    can_write     BOOLEAN NOT NULL DEFAULT false,
    granted_by    TEXT NOT NULL DEFAULT '',
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (folder_id, subject_type, subject)
);

CREATE TABLE files_objects (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    folder_id     UUID NOT NULL REFERENCES files_folders (id) ON DELETE CASCADE,
    name          TEXT NOT NULL CONSTRAINT files_objects_name_check
                      CHECK (char_length(name) BETWEEN 1 AND 255 AND name !~ '[/\\]'),
    object_key    TEXT NOT NULL UNIQUE,
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    content_type  TEXT NOT NULL DEFAULT 'application/octet-stream',
    status        TEXT NOT NULL DEFAULT 'pending' CONSTRAINT files_objects_status_check
                      CHECK (status IN ('pending', 'ready')),
    owner_id      UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    search        TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple', nexus_unaccent(name))) STORED
);
CREATE INDEX idx_files_objects_folder ON files_objects (folder_id, name);
CREATE INDEX idx_files_objects_pending ON files_objects (created_at) WHERE status = 'pending';
CREATE INDEX idx_files_objects_search ON files_objects USING gin (search);

-- +goose StatementBegin
CREATE TRIGGER trg_files_objects_set_updated_at BEFORE UPDATE ON files_objects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS files_objects;
DROP TABLE IF EXISTS files_folder_acl;
DROP TABLE IF EXISTS files_folders;
