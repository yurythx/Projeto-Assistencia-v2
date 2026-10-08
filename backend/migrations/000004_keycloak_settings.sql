-- +goose Up
-- keycloak_settings — configuração OIDC do Keycloak editável em tempo de
-- execução pelo nexus-admin (menu Configurações > Keycloak / IAM),
-- persistida no Postgres em vez de só existir como variável de ambiente.
-- Linha única (id fixo 'default', como um singleton) — não há múltiplos
-- realms/clients configuráveis por esta tela, só o realm que o backend
-- de fato usa para validar token (§29: este projeto nunca cria/administra
-- o Keycloak em si, só consome um realm/client já existentes).
--
-- *_secret_encrypted guarda o Client Secret CIFRADO (AES-256-GCM, ver
-- internal/platform/secretcrypto) com a chave de CONFIG_ENCRYPTION_KEY —
-- nunca texto plano, para que um dump/réplica/backup deste banco não
-- exponha o segredo do client Keycloak. A decifragem só acontece em
-- memória, no processo do backend, nunca em uma query SQL.
--
-- Quando issuer_url está vazio, a plataforma continua usando a
-- configuração vinda de variável de ambiente (KEYCLOAK_ISSUER_URL etc.,
-- ver config.Load) — esta tabela é uma SOBRESCRITA opcional, não uma
-- exigência: um ambiente que já configura tudo via env (ex.: Docker
-- Swarm/Kubernetes secrets) nunca precisa desta tela.
CREATE TABLE keycloak_settings (
    id                                TEXT PRIMARY KEY DEFAULT 'default',
    issuer_url                        TEXT NOT NULL DEFAULT '',
    realm                             TEXT NOT NULL DEFAULT '',
    client_id                         TEXT NOT NULL DEFAULT '',
    client_secret_encrypted           TEXT NOT NULL DEFAULT '',
    audience                          TEXT NOT NULL DEFAULT '',
    frontend_client_id                TEXT NOT NULL DEFAULT '',
    frontend_client_secret_encrypted  TEXT NOT NULL DEFAULT '',
    updated_at                        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by                        TEXT NOT NULL DEFAULT '',
    CONSTRAINT keycloak_settings_singleton CHECK (id = 'default')
);

-- +goose Down
DROP TABLE IF EXISTS keycloak_settings;
