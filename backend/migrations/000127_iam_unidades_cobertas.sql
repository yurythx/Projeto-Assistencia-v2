-- +goose Up
-- Permissão com escopo e herança (ADR 013): as unidades que uma concessão
-- cobre. Unidade = ela e as subunidades (árvore parent_id); entidade =
-- todas as unidades dela; departamento ou global = nenhuma (o departamento
-- casa pelo próprio id; o global cobre tudo). O resolvedor do IAM guarda o
-- resultado no cache da identidade — checar uma permissão não consulta o
-- banco.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION nexus_unidades_cobertas(e UUID, u UUID, d UUID)
RETURNS UUID[]
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT CASE
        WHEN d IS NOT NULL THEN '{}'::UUID[]
        WHEN u IS NOT NULL THEN (
            WITH RECURSIVE arvore(id) AS (
                SELECT u
                UNION
                SELECT x.id FROM unidades x JOIN arvore a ON x.parent_id = a.id
            )
            SELECT array_agg(id) FROM arvore)
        WHEN e IS NOT NULL THEN COALESCE((SELECT array_agg(id) FROM unidades WHERE entidade_id = e), '{}'::UUID[])
        ELSE '{}'::UUID[]
    END
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS nexus_unidades_cobertas(UUID, UUID, UUID);
