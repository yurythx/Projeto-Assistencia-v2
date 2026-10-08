-- +goose Up
-- IAM multi-escopo (Core imutável): estrutura organizacional
-- Entidade (tenant) > Unidade (hierárquica) > Departamento, Perfis com
-- permissões granulares "recurso:ação", e o mapeamento flexível
-- grupo do AD -> Perfil + escopo organizacional.
--
-- Um usuário pode ter N lotações (user_scopes), cada uma um perfil
-- aplicado a um escopo. Escopo nulo em todos os níveis = plataforma toda.

-- Permissão válida: "*" (tudo), "recurso:*" ou "recurso:acao".
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION nexus_valid_permissions(perms TEXT[])
RETURNS BOOLEAN
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT COALESCE(bool_and(p ~ '^(\*|[a-z_]+:(\*|[a-z_]+))$'), true) FROM unnest(perms) AS p
$$;
-- +goose StatementEnd

CREATE TABLE entidades (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome        TEXT NOT NULL,
    sigla       TEXT NOT NULL DEFAULT '',
    slug        TEXT NOT NULL UNIQUE,
    documento   TEXT NOT NULL DEFAULT '', -- CNPJ ou outro identificador institucional
    ativo       BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE unidades (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entidade_id  UUID NOT NULL REFERENCES entidades (id) ON DELETE CASCADE,
    parent_id    UUID REFERENCES unidades (id) ON DELETE SET NULL,
    nome         TEXT NOT NULL,
    sigla        TEXT NOT NULL DEFAULT '',
    slug         TEXT NOT NULL,
    ad_group     TEXT NOT NULL DEFAULT '',
    email        TEXT NOT NULL DEFAULT '',
    telefone     TEXT NOT NULL DEFAULT '',
    endereco     TEXT NOT NULL DEFAULT '',
    ativo        BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_unidades_entidade_slug UNIQUE (entidade_id, slug),
    CONSTRAINT ck_unidades_parent_not_self CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE INDEX idx_unidades_entidade ON unidades (entidade_id);
CREATE INDEX idx_unidades_parent ON unidades (parent_id);

CREATE TABLE departamentos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unidade_id  UUID NOT NULL REFERENCES unidades (id) ON DELETE CASCADE,
    nome        TEXT NOT NULL,
    sigla       TEXT NOT NULL DEFAULT '',
    slug        TEXT NOT NULL,
    ad_group    TEXT NOT NULL DEFAULT '',
    email       TEXT NOT NULL DEFAULT '',
    telefone    TEXT NOT NULL DEFAULT '',
    ativo       BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_departamentos_unidade_slug UNIQUE (unidade_id, slug)
);
CREATE INDEX idx_departamentos_unidade ON departamentos (unidade_id);

CREATE TABLE perfis (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE,
    nome        TEXT NOT NULL,
    descricao   TEXT NOT NULL DEFAULT '',
    permissoes  TEXT[] NOT NULL DEFAULT '{}',
    -- perfis de sistema não podem ser removidos pela API (podem ter as
    -- permissões ajustadas, exceto o administrador).
    sistema     BOOLEAN NOT NULL DEFAULT false,
    ativo       BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_perfis_permissoes_formato CHECK (nexus_valid_permissions(permissoes))
);

CREATE TABLE ad_group_mappings (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ad_group         TEXT NOT NULL,
    perfil_id        UUID NOT NULL REFERENCES perfis (id) ON DELETE CASCADE,
    entidade_id      UUID REFERENCES entidades (id) ON DELETE CASCADE,
    unidade_id       UUID REFERENCES unidades (id) ON DELETE CASCADE,
    departamento_id  UUID REFERENCES departamentos (id) ON DELETE CASCADE,
    descricao        TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by       TEXT NOT NULL DEFAULT ''
);
-- Comparação de grupo do AD é case-insensitive (CN vem com caixa variável).
CREATE UNIQUE INDEX uq_ad_group_mappings ON ad_group_mappings (
    lower(ad_group), perfil_id,
    COALESCE(entidade_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(unidade_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(departamento_id, '00000000-0000-0000-0000-000000000000'::uuid)
);
CREATE INDEX idx_ad_group_mappings_group ON ad_group_mappings (lower(ad_group));

CREATE TABLE user_scopes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    perfil_id        UUID NOT NULL REFERENCES perfis (id) ON DELETE CASCADE,
    entidade_id      UUID REFERENCES entidades (id) ON DELETE CASCADE,
    unidade_id       UUID REFERENCES unidades (id) ON DELETE CASCADE,
    departamento_id  UUID REFERENCES departamentos (id) ON DELETE CASCADE,
    principal        BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by       TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX uq_user_scopes ON user_scopes (
    user_id, perfil_id,
    COALESCE(entidade_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(unidade_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(departamento_id, '00000000-0000-0000-0000-000000000000'::uuid)
);
CREATE INDEX idx_user_scopes_user ON user_scopes (user_id);

-- +goose StatementBegin
CREATE TRIGGER trg_entidades_set_updated_at BEFORE UPDATE ON entidades
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_unidades_set_updated_at BEFORE UPDATE ON unidades
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_departamentos_set_updated_at BEFORE UPDATE ON departamentos
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_perfis_set_updated_at BEFORE UPDATE ON perfis
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- Perfis de sistema. Os slugs são referenciados pela documentação de
-- implantação (mapeamento inicial dos grupos do AD).
INSERT INTO perfis (slug, nome, descricao, permissoes, sistema) VALUES
    ('administrador', 'Administrador da Plataforma',
     'Acesso total: módulos, identidade, estrutura organizacional e todos os plugins.',
     ARRAY['*'], true),
    ('auditor', 'Auditor',
     'Leitura da trilha de auditoria, verificação da cadeia de integridade e consulta de usuários.',
     ARRAY['audit:read', 'audit:verify', 'users:read', 'monitoring:read'], true),
    ('gestor-iam', 'Gestor de Identidade',
     'Administra usuários, lotações, perfis e o mapeamento de grupos do AD.',
     ARRAY['users:read', 'users:manage', 'iam:manage'], true),
    ('gestor-conteudo', 'Gestor de Conteúdo',
     'Publica comunicados, mantém o catálogo de serviços e modera a wiki.',
     ARRAY['blog:manage', 'catalog:manage', 'wiki:manage', 'contact:read', 'contact:manage'], true),
    ('protocolo', 'Protocolo',
     'Abre e tramita processos administrativos.',
     ARRAY['tramite:create', 'tramite:route'], true),
    ('servidor', 'Servidor',
     'Perfil básico de colaborador autenticado (sem permissões administrativas).',
     ARRAY[]::TEXT[], true)
ON CONFLICT (slug) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS user_scopes;
DROP TABLE IF EXISTS ad_group_mappings;
DROP TABLE IF EXISTS perfis;
DROP TABLE IF EXISTS departamentos;
DROP TABLE IF EXISTS unidades;
DROP TABLE IF EXISTS entidades;
DROP FUNCTION IF EXISTS nexus_valid_permissions(TEXT[]);
