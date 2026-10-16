-- +goose Up
-- SEMPRAS — tipo de cada unidade (ADR 031). As unidades continuam sendo do
-- IAM; o que é de assistência social (o tipo, que decide os serviços
-- ofertados, e o comportamento do painel de chamada) fica aqui, 1:1. Uma
-- unidade sem linha nesta tabela não atende.

CREATE TABLE sempras_unidades (
    unidade_id        UUID PRIMARY KEY REFERENCES unidades (id) ON DELETE CASCADE,
    tipo              TEXT NOT NULL CHECK (tipo IN ('cras', 'scfv', 'creas', 'centro_pop', 'casa_mulher', 'casa_abrigo', 'conselho_tutelar')),
    -- A TV da sala de espera mostra e fala o nome de quem é chamado? Nas
    -- unidades de proteção especial e de garantia de direitos, só a senha:
    -- violência doméstica (Lei 11.340/2006), crianças (ECA arts. 17 e 143).
    painel_exibe_nome BOOLEAN NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed pela estrutura real (deploy/estrutura). Numa instalação nova, em que
-- a estrutura é aplicada depois das migrations, o mesmo seed roda por
-- deploy/sempras/tipos-unidades.sql (make sempras-mapeamentos).
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

-- +goose Down
DROP TABLE sempras_unidades;
