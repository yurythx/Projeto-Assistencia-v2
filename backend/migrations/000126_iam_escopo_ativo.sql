-- +goose Up
-- Estrutura desativada deixa de dar acesso (achado ao documentar as regras
-- de negócio — docs/REGRAS_DE_NEGOCIO.md §8): o "ativo" de entidade,
-- unidade e departamento só afetava o Diretório; lotações e mapeamentos do
-- AD num escopo desativado seguiam concedendo perfis, e o Trâmite abria e
-- tramitava processos para unidade desativada.
--
-- nexus_scope_active é a regra ÚNICA, usada pelo resolvedor do IAM
-- (platform/iam) e pelo cálculo de permissões da tela de usuários
-- (modules/iam): um escopo vale só se cada nível informado existe e está
-- ativo. Nível vazio (NULL) não restringe — escopo todo vazio é a
-- plataforma inteira.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION nexus_scope_active(e UUID, u UUID, d UUID)
RETURNS BOOLEAN
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT (e IS NULL OR EXISTS (SELECT 1 FROM entidades WHERE id = e AND ativo))
       AND (u IS NULL OR EXISTS (SELECT 1 FROM unidades WHERE id = u AND ativo))
       AND (d IS NULL OR EXISTS (SELECT 1 FROM departamentos WHERE id = d AND ativo))
$$;
-- +goose StatementEnd

-- Excluir uma unidade-mãe deixava as subunidades sem mãe, em silêncio
-- (ON DELETE SET NULL). Agora, como o resto da estrutura (000121), a
-- exclusão é recusada enquanto houver subunidades.
ALTER TABLE unidades DROP CONSTRAINT unidades_parent_id_fkey,
    ADD CONSTRAINT unidades_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES unidades (id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE unidades DROP CONSTRAINT unidades_parent_id_fkey,
    ADD CONSTRAINT unidades_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES unidades (id) ON DELETE SET NULL;
DROP FUNCTION IF EXISTS nexus_scope_active(UUID, UUID, UUID);
