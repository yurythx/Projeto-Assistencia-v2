-- +goose Up
-- SEMPRAS — PAIF: se o acompanhamento da família é particularizado
-- (atendimento individual), em grupo/coletivo (SCFV) ou os dois — o
-- legado de referência guarda isso junto da inclusão no PAIF.

ALTER TABLE sempras_paif_vinculos ADD COLUMN tipo_atendimento TEXT NOT NULL DEFAULT 'particularizado'
    CHECK (tipo_atendimento IN ('particularizado', 'coletivo', 'ambos'));

-- +goose Down
ALTER TABLE sempras_paif_vinculos DROP COLUMN tipo_atendimento;
