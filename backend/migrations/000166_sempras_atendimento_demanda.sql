-- +goose Up
-- SEMPRAS — a demanda que trouxe o munícipe à recepção (ADR 031 §3). A
-- lista válida depende do tipo da unidade e é checada na aplicação; aqui só
-- o formato. Entradas anteriores ficam com demanda vazia ("não informada").

ALTER TABLE sempras_atendimentos
    ADD COLUMN demanda         TEXT NOT NULL DEFAULT '' CHECK (demanda ~ '^[a-z_]{0,40}$'),
    ADD COLUMN demanda_detalhe TEXT NOT NULL DEFAULT '' CHECK (char_length(demanda_detalhe) <= 200);

-- +goose Down
ALTER TABLE sempras_atendimentos DROP COLUMN demanda_detalhe, DROP COLUMN demanda;
