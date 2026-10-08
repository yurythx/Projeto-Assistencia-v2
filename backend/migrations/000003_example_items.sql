-- +goose Up
-- example_items — tabela do Módulo Modelo (internal/modules/example),
-- o blueprint de referência Clean Architecture pra novos módulos de
-- negócio. Achado de auditoria: esta tabela nunca tinha sido criada em
-- migration nenhuma — o módulo de exemplo, incluindo o endpoint
-- POST /api/v1/examples exposto no frontend (/exemplos), falhava com
-- "relation example_items does not exist" em qualquer banco criado do
-- baseline (000001) pra cá. Nenhum teste pegou isso porque o módulo não
-- tinha teste nenhum (application/domain/infrastructure/transport) até
-- este mesmo commit.
CREATE TABLE example_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_example_items_created_at ON example_items (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS example_items;
