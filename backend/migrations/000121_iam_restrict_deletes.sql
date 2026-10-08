-- +goose Up
-- Estrutura organizacional e perfis passam a ON DELETE RESTRICT.
--
-- Com CASCADE, excluir uma entidade/unidade/departamento ou um perfil
-- apagava em silêncio a estrutura abaixo e TODAS as lotações e
-- mapeamentos do AD que o usavam — uma perda de concessões de acesso sem
-- aviso nem trilha individual. Agora a exclusão é recusada (409) enquanto
-- houver vínculos: o administrador remove/realoca antes, cada passo
-- auditado. (Remover o USUÁRIO continua levando as lotações dele.)
ALTER TABLE unidades DROP CONSTRAINT unidades_entidade_id_fkey,
    ADD CONSTRAINT unidades_entidade_id_fkey FOREIGN KEY (entidade_id) REFERENCES entidades (id) ON DELETE RESTRICT;
ALTER TABLE departamentos DROP CONSTRAINT departamentos_unidade_id_fkey,
    ADD CONSTRAINT departamentos_unidade_id_fkey FOREIGN KEY (unidade_id) REFERENCES unidades (id) ON DELETE RESTRICT;
ALTER TABLE ad_group_mappings DROP CONSTRAINT ad_group_mappings_perfil_id_fkey,
    ADD CONSTRAINT ad_group_mappings_perfil_id_fkey FOREIGN KEY (perfil_id) REFERENCES perfis (id) ON DELETE RESTRICT;
ALTER TABLE ad_group_mappings DROP CONSTRAINT ad_group_mappings_entidade_id_fkey,
    ADD CONSTRAINT ad_group_mappings_entidade_id_fkey FOREIGN KEY (entidade_id) REFERENCES entidades (id) ON DELETE RESTRICT;
ALTER TABLE ad_group_mappings DROP CONSTRAINT ad_group_mappings_unidade_id_fkey,
    ADD CONSTRAINT ad_group_mappings_unidade_id_fkey FOREIGN KEY (unidade_id) REFERENCES unidades (id) ON DELETE RESTRICT;
ALTER TABLE ad_group_mappings DROP CONSTRAINT ad_group_mappings_departamento_id_fkey,
    ADD CONSTRAINT ad_group_mappings_departamento_id_fkey FOREIGN KEY (departamento_id) REFERENCES departamentos (id) ON DELETE RESTRICT;
ALTER TABLE user_scopes DROP CONSTRAINT user_scopes_perfil_id_fkey,
    ADD CONSTRAINT user_scopes_perfil_id_fkey FOREIGN KEY (perfil_id) REFERENCES perfis (id) ON DELETE RESTRICT;
ALTER TABLE user_scopes DROP CONSTRAINT user_scopes_entidade_id_fkey,
    ADD CONSTRAINT user_scopes_entidade_id_fkey FOREIGN KEY (entidade_id) REFERENCES entidades (id) ON DELETE RESTRICT;
ALTER TABLE user_scopes DROP CONSTRAINT user_scopes_unidade_id_fkey,
    ADD CONSTRAINT user_scopes_unidade_id_fkey FOREIGN KEY (unidade_id) REFERENCES unidades (id) ON DELETE RESTRICT;
ALTER TABLE user_scopes DROP CONSTRAINT user_scopes_departamento_id_fkey,
    ADD CONSTRAINT user_scopes_departamento_id_fkey FOREIGN KEY (departamento_id) REFERENCES departamentos (id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE unidades DROP CONSTRAINT unidades_entidade_id_fkey,
    ADD CONSTRAINT unidades_entidade_id_fkey FOREIGN KEY (entidade_id) REFERENCES entidades (id) ON DELETE CASCADE;
ALTER TABLE departamentos DROP CONSTRAINT departamentos_unidade_id_fkey,
    ADD CONSTRAINT departamentos_unidade_id_fkey FOREIGN KEY (unidade_id) REFERENCES unidades (id) ON DELETE CASCADE;
ALTER TABLE ad_group_mappings DROP CONSTRAINT ad_group_mappings_perfil_id_fkey,
    ADD CONSTRAINT ad_group_mappings_perfil_id_fkey FOREIGN KEY (perfil_id) REFERENCES perfis (id) ON DELETE CASCADE;
ALTER TABLE ad_group_mappings DROP CONSTRAINT ad_group_mappings_entidade_id_fkey,
    ADD CONSTRAINT ad_group_mappings_entidade_id_fkey FOREIGN KEY (entidade_id) REFERENCES entidades (id) ON DELETE CASCADE;
ALTER TABLE ad_group_mappings DROP CONSTRAINT ad_group_mappings_unidade_id_fkey,
    ADD CONSTRAINT ad_group_mappings_unidade_id_fkey FOREIGN KEY (unidade_id) REFERENCES unidades (id) ON DELETE CASCADE;
ALTER TABLE ad_group_mappings DROP CONSTRAINT ad_group_mappings_departamento_id_fkey,
    ADD CONSTRAINT ad_group_mappings_departamento_id_fkey FOREIGN KEY (departamento_id) REFERENCES departamentos (id) ON DELETE CASCADE;
ALTER TABLE user_scopes DROP CONSTRAINT user_scopes_perfil_id_fkey,
    ADD CONSTRAINT user_scopes_perfil_id_fkey FOREIGN KEY (perfil_id) REFERENCES perfis (id) ON DELETE CASCADE;
ALTER TABLE user_scopes DROP CONSTRAINT user_scopes_entidade_id_fkey,
    ADD CONSTRAINT user_scopes_entidade_id_fkey FOREIGN KEY (entidade_id) REFERENCES entidades (id) ON DELETE CASCADE;
ALTER TABLE user_scopes DROP CONSTRAINT user_scopes_unidade_id_fkey,
    ADD CONSTRAINT user_scopes_unidade_id_fkey FOREIGN KEY (unidade_id) REFERENCES unidades (id) ON DELETE CASCADE;
ALTER TABLE user_scopes DROP CONSTRAINT user_scopes_departamento_id_fkey,
    ADD CONSTRAINT user_scopes_departamento_id_fkey FOREIGN KEY (departamento_id) REFERENCES departamentos (id) ON DELETE CASCADE;
