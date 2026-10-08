-- +goose Up
-- Modelos de documento ligados a uma série da TTDD (ADR 024): o tipo de
-- documento ganha os modelos para baixar na página da série. Um modelo
-- pode servir a várias séries, e uma série a vários modelos.

CREATE TABLE atlas_ttdd_modelos (
    codigo      TEXT NOT NULL REFERENCES atlas_classificacao_ttdd (codigo) ON UPDATE CASCADE ON DELETE CASCADE,
    modelo_id   UUID NOT NULL REFERENCES atlas_modelos (id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (codigo, modelo_id)
);
CREATE INDEX idx_atlas_ttdd_modelos_modelo ON atlas_ttdd_modelos (modelo_id);

-- +goose Down
DROP TABLE IF EXISTS atlas_ttdd_modelos;
