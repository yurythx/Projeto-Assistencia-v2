-- Mapeamentos dos grupos do AD do SEMPRAS (ADR 030 §3, docs/ROADMAP_SEMPRAS.md).
--
-- Convenção: SEMPRAS_<UNIDADE>_<PERFIL> (RECEPCIONISTA, TECNICO, PSICOLOGO) por
-- unidade, e SEMPRAS_SUPERINTENDENCIA na entidade SEMPRAS. 16 × 3 + 1 = 49.
--
-- Pré-requisitos: a estrutura real (make estrutura-aplicar) e as migrations
-- (os perfis sempras-* nascem na 000143). Idempotente: rodar de novo não
-- duplica. O cache de permissões do IAM expira em poucos segundos — o
-- mapeamento passa a valer logo em seguida, sem reiniciar nada.
--
-- Aplicar: make sempras-mapeamentos
BEGIN;

WITH ent AS (
    SELECT id FROM entidades WHERE slug = 'sempras'
),
un AS (
    SELECT u.id, u.entidade_id, u.slug, u.nome
      FROM unidades u JOIN ent ON ent.id = u.entidade_id
     WHERE u.slug IN ('casa-abrigo', 'casa-da-mulher', 'centro-pop', 'ceu',
                      'conselho-tutelar-central', 'conselho-tutelar-vila-operaria',
                      'cras-alfredo-de-castro', 'cras-ana-carla', 'cras-central', 'cras-conjunto-sao-jose',
                      'cras-iguacu', 'cras-luz-dyara', 'cras-padre-lothar', 'cras-rio-vermelho',
                      'cras-sagrada-familia', 'creas')
),
pf AS (
    SELECT id, slug, nome,
           CASE slug WHEN 'sempras-recepcionista' THEN 'RECEPCIONISTA'
                     WHEN 'sempras-tecnico' THEN 'TECNICO'
                     ELSE 'PSICOLOGO' END AS sufixo
      FROM perfis WHERE slug IN ('sempras-recepcionista', 'sempras-tecnico', 'sempras-psicologo')
)
INSERT INTO ad_group_mappings (ad_group, perfil_id, entidade_id, unidade_id, descricao, created_by)
SELECT 'SEMPRAS_' || upper(replace(un.slug, '-', '_')) || '_' || pf.sufixo,
       pf.id, un.entidade_id, un.id, pf.nome || ' — ' || un.nome, 'deploy/sempras'
  FROM un CROSS JOIN pf
ON CONFLICT DO NOTHING;

INSERT INTO ad_group_mappings (ad_group, perfil_id, entidade_id, descricao, created_by)
SELECT 'SEMPRAS_SUPERINTENDENCIA', p.id, e.id, 'Superintendência — todas as unidades da SEMPRAS', 'deploy/sempras'
  FROM perfis p, entidades e
 WHERE p.slug = 'sempras-superintendente' AND e.slug = 'sempras'
ON CONFLICT DO NOTHING;

-- Conferência: devem ser 49 (falta unidade ou perfil = menos linhas).
SELECT count(*) AS mapeamentos_sempras FROM ad_group_mappings WHERE ad_group LIKE 'SEMPRAS\_%';

COMMIT;
