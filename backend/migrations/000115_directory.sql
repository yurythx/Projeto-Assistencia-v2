-- +goose Up
-- Plugin Diretório — consulta de pessoas e setores. Nome/e-mail/grupos vêm
-- da identidade sincronizada do AD (tabela users, via Keycloak); este
-- plugin guarda só o perfil estendido (cargo, ramal, lotação exibida).

CREATE TABLE directory_profiles (
    user_id          UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    job_title        TEXT NOT NULL DEFAULT '',
    phone            TEXT NOT NULL DEFAULT '',
    extension        TEXT NOT NULL DEFAULT '',
    unidade_id       UUID REFERENCES unidades (id) ON DELETE SET NULL,
    departamento_id  UUID REFERENCES departamentos (id) ON DELETE SET NULL,
    bio              TEXT NOT NULL DEFAULT '',
    visible          BOOLEAN NOT NULL DEFAULT true,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_directory_profiles_departamento ON directory_profiles (departamento_id);
CREATE INDEX idx_directory_profiles_unidade ON directory_profiles (unidade_id);

-- +goose StatementBegin
CREATE TRIGGER trg_directory_profiles_set_updated_at BEFORE UPDATE ON directory_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS directory_profiles;
