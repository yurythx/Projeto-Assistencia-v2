-- Administradores da plataforma pelo AD (via Keycloak, realm rondonopolis).
--
-- Decisão da gestão (2026-10-19): o grupo "Departamento de Tecnologia da
-- Informação" recebe o perfil "Administrador da Plataforma" (permissão *),
-- sem escopo, ou seja, acesso a tudo, inclusive prontuários e o sigilo
-- psicológico do SEMPRAS. Revise a composição do grupo no AD: quem entra
-- nele passa a ver esses dados.
--
-- Idempotente; a criação fica na trilha de auditoria. O cache de permissões
-- expira em segundos.
--
-- Aplicar: make iam-administradores
BEGIN;

WITH novo AS (
    INSERT INTO ad_group_mappings (ad_group, perfil_id, descricao, created_by)
    SELECT 'Departamento de Tecnologia da Informação', p.id,
           'Administrador da Plataforma — acesso total (decisão da gestão, 2026-10-19)', 'deploy/iam'
      FROM perfis p
     WHERE p.slug = 'administrador'
    ON CONFLICT DO NOTHING
    RETURNING id, ad_group
)
INSERT INTO audit_logs (action, resource_type, resource_id, actor_subject, metadata)
SELECT 'iam.ad_mapping.created', 'ad_group_mapping', novo.id::text, 'deploy/iam',
       jsonb_build_object('ad_group', novo.ad_group, 'perfil', 'administrador', 'escopo', 'global', 'origem', 'deploy/iam/administradores-ad.sql')
  FROM novo;

SELECT m.ad_group, p.slug AS perfil, (m.entidade_id IS NULL AND m.unidade_id IS NULL) AS global
  FROM ad_group_mappings m JOIN perfis p ON p.id = m.perfil_id
 WHERE p.slug = 'administrador';

COMMIT;
