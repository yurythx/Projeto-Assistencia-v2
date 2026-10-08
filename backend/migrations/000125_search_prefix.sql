-- +goose Up
-- Busca por prefixo em todo o full-text (achado nos testes com os dados
-- fictícios): websearch_to_tsquery só casa palavras inteiras, então
-- "vacina" não encontrava "vacinação" nem "campan" encontrava "campanha" —
-- justamente o que se digita numa busca global. nexus_search_tsquery soma
-- (OR) à consulta websearch a versão com prefixo de cada termo
-- ('vacina':* & 'campan':*), sem acento. Consultas com operadores
-- (aspas, -exclusão, or) ficam só com o websearch, para não mudar o
-- sentido delas. O índice GIN dos tsvector já atende prefixos.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION nexus_search_tsquery(cfg regconfig, q TEXT)
RETURNS tsquery
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT websearch_to_tsquery(cfg, nexus_unaccent(q)) || COALESCE((
        SELECT to_tsquery(cfg, string_agg(quote_literal(t) || ':*', ' & '))
          FROM regexp_split_to_table(lower(nexus_unaccent(q)), '[^[:alnum:]]+') AS t
         WHERE length(t) >= 2
           AND q !~ '["-]'
           AND q !~* '\mor\M'
    ), ''::tsquery)
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS nexus_search_tsquery(regconfig, TEXT);
