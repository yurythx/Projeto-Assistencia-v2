-- Mapeamentos dos grupos do AD (via Keycloak) para o SEMPRAS — ADR 030 §3.
--
-- Gerado a partir dos grupos REAIS do realm rondonopolis (lidos em 2026-10-19),
-- com as decisões da gestão:
--   "Grupo <Unidade>"              -> SEMPRAS — Técnico, na unidade
--   "Grupo <Unidade> Administração" -> SEMPRAS — Recepcionista, na unidade
--   "Grupo <Unidade> Coordenação"   -> SEMPRAS — Coordenador, na unidade
--   SEMPRAS, Sempras_Gabinete e Sempras_Sup_* -> SEMPRAS — Superintendente,
--     na entidade (todas as unidades)
-- Psicólogos e ajustes de função são feitos na tela de lotação (o AD não
-- distingue). Unidades sem grupo no AD (Conselhos Tutelares, SCFV, Vila
-- Olímpica) ficam por lotação manual até a TI confirmar o grupo.
--
-- Idempotente. Remove os mapeamentos de exemplo SEMPRAS_<UNIDADE>_<PERFIL>
-- (nomes que não existem no AD). O cache de permissões expira em segundos.
--
-- Aplicar: make sempras-mapeamentos
BEGIN;

DELETE FROM ad_group_mappings WHERE created_by = 'deploy/sempras' AND ad_group LIKE 'SEMPRAS\_%';

CREATE TEMP TABLE m (ad_group text, perfil text, unidade text) ON COMMIT DROP;
INSERT INTO m VALUES
    ('Grupo Casa Abrigo', 'sempras-tecnico', 'casa-abrigo'),
    ('Grupo Casa Abrigo Administração', 'sempras-recepcionista', 'casa-abrigo'),
    ('Grupo Casa Abrigo Coordenação', 'sempras-coordenador', 'casa-abrigo'),
    ('Grupo Centro Pop', 'sempras-tecnico', 'centro-pop'),
    ('Grupo Centro Pop Administração', 'sempras-recepcionista', 'centro-pop'),
    ('Grupo Centro Pop Coordenação', 'sempras-coordenador', 'centro-pop'),
    ('Grupo Ceu', 'sempras-tecnico', 'ceu'),
    ('Grupo Ceu Administração', 'sempras-recepcionista', 'ceu'),
    ('Grupo Ceu Coordenação', 'sempras-coordenador', 'ceu'),
    ('Grupo Cras Alfredo de Castro', 'sempras-tecnico', 'cras-alfredo-de-castro'),
    ('Grupo Cras Alfredo de Castro Administração', 'sempras-recepcionista', 'cras-alfredo-de-castro'),
    ('Grupo Cras Alfredo de Castro Coordenação', 'sempras-coordenador', 'cras-alfredo-de-castro'),
    ('Grupo Cras Ana Carla', 'sempras-tecnico', 'cras-ana-carla'),
    ('Grupo Cras Ana Carla Administração', 'sempras-recepcionista', 'cras-ana-carla'),
    ('Grupo Cras Ana Carla Coordenação', 'sempras-coordenador', 'cras-ana-carla'),
    ('Grupo Cras Central', 'sempras-tecnico', 'cras-central'),
    ('Grupo Cras Central Administração', 'sempras-recepcionista', 'cras-central'),
    ('Grupo Cras Conjunto São José', 'sempras-tecnico', 'cras-conjunto-sao-jose'),
    ('Grupo Cras Conjunto São José Administração', 'sempras-recepcionista', 'cras-conjunto-sao-jose'),
    ('Grupo Cras Conjunto São José Coordenação', 'sempras-coordenador', 'cras-conjunto-sao-jose'),
    ('Grupo Cras Iguaçu', 'sempras-tecnico', 'cras-iguacu'),
    ('Grupo Cras Iguaçu Administração', 'sempras-recepcionista', 'cras-iguacu'),
    ('Grupo Cras Iguaçu Coordenação', 'sempras-coordenador', 'cras-iguacu'),
    ('Grupo Cras Rio Vermelho', 'sempras-tecnico', 'cras-rio-vermelho'),
    ('Grupo Cras Rio Vermelho Administração', 'sempras-recepcionista', 'cras-rio-vermelho'),
    ('Grupo Cras Rio Vermelho Coordenação', 'sempras-coordenador', 'cras-rio-vermelho'),
    ('Grupo Cras Padre Lothar', 'sempras-tecnico', 'cras-padre-lothar'),
    ('Grupo Cras Sagrada Familia', 'sempras-tecnico', 'cras-sagrada-familia'),
    ('Grupo Creas', 'sempras-tecnico', 'creas'),
    ('Grupo Creas Administração', 'sempras-recepcionista', 'creas'),
    ('Grupo Creas Coordenação', 'sempras-coordenador', 'creas'),
    ('Grupo Cras Luz Dyara', 'sempras-tecnico', 'cras-luz-dyara'),
    ('Grupo Cras Luz D''Yara Administração', 'sempras-recepcionista', 'cras-luz-dyara'),
    ('Grupo Cras Luz D''Yara Coordenação', 'sempras-coordenador', 'cras-luz-dyara'),
    ('Sempras_Casa_da_Mulher', 'sempras-tecnico', 'casa-da-mulher'),
    ('SEMPRAS', 'sempras-superintendente', NULL),
    ('Sempras_Gabinete', 'sempras-superintendente', NULL),
    ('Sempras_Sup_Beneficios_Proj_SUAS', 'sempras-superintendente', NULL),
    ('Sempras_Sup_Coord_Jur', 'sempras-superintendente', NULL),
    ('Sempras_Sup_Engenharia_Arquitetura', 'sempras-superintendente', NULL),
    ('Sempras_Sup_Gestao', 'sempras-superintendente', NULL),
    ('Sempras_Sup_Politicas_Publicas_Mulheres', 'sempras-superintendente', NULL),
    ('Sempras_Sup_Politicas_Trabalho', 'sempras-superintendente', NULL),
    ('Sempras_Sup_Protec_Social_Basica', 'sempras-superintendente', NULL),
    ('Sempras_Sup_Protec_Social_Especial', 'sempras-superintendente', NULL);

INSERT INTO ad_group_mappings (ad_group, perfil_id, entidade_id, unidade_id, descricao, created_by)
SELECT m.ad_group, p.id, e.id, u.id,
       p.nome || ' — ' || COALESCE(u.nome, 'todas as unidades da SEMPRAS'), 'deploy/sempras'
  FROM m
  JOIN perfis p ON p.slug = m.perfil
  JOIN entidades e ON e.slug = 'sempras'
  LEFT JOIN unidades u ON u.slug = m.unidade AND u.entidade_id = e.id
 WHERE m.unidade IS NULL OR u.id IS NOT NULL
ON CONFLICT DO NOTHING;

-- Conferência: devem ser 45 menos os grupos listados abaixo.
SELECT count(*) AS mapeamentos_sempras FROM ad_group_mappings WHERE created_by = 'deploy/sempras';
-- Grupos sem unidade na estrutura (não mapeados: crie a unidade e rode de novo)
SELECT m.ad_group AS sem_unidade, m.unidade
  FROM m LEFT JOIN unidades u ON u.slug = m.unidade
 WHERE m.unidade IS NOT NULL AND u.id IS NULL;

COMMIT;
