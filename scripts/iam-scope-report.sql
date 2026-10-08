-- Relatório de impacto da permissão com escopo (ADR 013) — `make iam-scope-report`.
--
-- Lista cada lotação e mapeamento de grupo COM escopo e, para cada
-- permissão do perfil, o que acontece com a regra nova:
--   * "deixa de valer"      — permissão de plataforma (ou de módulo ainda sem
--                             dono organizacional) só vale com concessão
--                             global; com escopo, não concede nada;
--   * "vale só no escopo"   — permissão de módulo com dono organizacional
--                             (antes valia na plataforma toda).
-- Concessões globais (sem escopo) não mudam e não aparecem.
--
-- A lista de permissões com escopo espelha auth.ScopedPermissions
-- (backend/internal/platform/auth/scope.go) — mantenha as duas iguais.
WITH com_escopo AS (
    SELECT 'lotação'::text AS origem, u.username AS quem, p.slug AS perfil, p.permissoes,
           s.entidade_id, s.unidade_id, s.departamento_id
      FROM user_scopes s JOIN perfis p ON p.id = s.perfil_id AND p.ativo JOIN users u ON u.id = s.user_id
     WHERE num_nonnulls(s.entidade_id, s.unidade_id, s.departamento_id) > 0
    UNION ALL
    SELECT 'grupo', m.ad_group, p.slug, p.permissoes, m.entidade_id, m.unidade_id, m.departamento_id
      FROM ad_group_mappings m JOIN perfis p ON p.id = m.perfil_id AND p.ativo
     WHERE num_nonnulls(m.entidade_id, m.unidade_id, m.departamento_id) > 0
),
escopo AS (
    SELECT perm FROM unnest(ARRAY[
        'tramite:create',
        'tramite:route',
        'tramite:manage',
        'catalog:manage',
        'blog:manage',
        'wiki:manage',
        'calendar:manage',
        'mercurio:manage',
        'directory:manage',
        'contact:read',
        'contact:manage',
        'audit:read',
        'files:manage',
        'iam:manage',
        'users:read',
        'users:manage'
    ]) AS perm
)
SELECT c.origem, c.quem, c.perfil,
       concat_ws(' > ', e.sigla, un.sigla, d.sigla) AS escopo,
       perm,
       CASE
           WHEN perm IN (SELECT perm FROM escopo) THEN 'vale só no escopo'
           WHEN perm = '*' OR perm LIKE '%:*' THEN 'só as de módulo com dono valem no escopo; o resto deixa de valer'
           ELSE 'deixa de valer'
       END AS efeito
  FROM com_escopo c
  CROSS JOIN LATERAL unnest(c.permissoes) AS perm
  LEFT JOIN entidades e ON e.id = c.entidade_id
  LEFT JOIN unidades un ON un.id = c.unidade_id
  LEFT JOIN departamentos d ON d.id = c.departamento_id
 ORDER BY efeito, c.perfil, c.quem, perm;
