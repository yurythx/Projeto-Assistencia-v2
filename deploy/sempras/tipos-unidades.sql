-- SEMPRAS — tipo de cada unidade (ADR 031), pelo slug da estrutura real.
-- Idempotente: não muda o tipo de quem já tem. A migration 000165 faz o
-- mesmo seed; este arquivo cobre a instalação nova, em que a estrutura
-- (make estrutura-aplicar) entra depois das migrations.
INSERT INTO sempras_unidades (unidade_id, tipo, painel_exibe_nome)
SELECT u.id, t.tipo, t.tipo IN ('cras', 'scfv', 'centro_pop')
  FROM unidades u
  JOIN entidades e ON e.id = u.entidade_id AND e.slug = 'sempras'
  JOIN LATERAL (SELECT CASE
        WHEN u.slug LIKE 'cras-%'                                     THEN 'cras'
        WHEN u.slug IN ('scfv-padre-lothar', 'ceu', 'vila-olimpica')  THEN 'scfv'
        WHEN u.slug = 'creas'                                         THEN 'creas'
        WHEN u.slug = 'centro-pop'                                    THEN 'centro_pop'
        WHEN u.slug = 'casa-da-mulher'                                THEN 'casa_mulher'
        WHEN u.slug = 'casa-abrigo'                                   THEN 'casa_abrigo'
        WHEN u.slug LIKE 'conselho-tutelar-%'                         THEN 'conselho_tutelar'
       END AS tipo) t ON t.tipo IS NOT NULL
ON CONFLICT (unidade_id) DO NOTHING;
