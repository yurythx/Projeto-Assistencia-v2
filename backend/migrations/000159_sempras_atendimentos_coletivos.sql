-- +goose Up
-- SEMPRAS — Atendimentos Coletivos (SCFV): evento de grupo da unidade,
-- não é por munícipe. categoria é um enum real (Bloco III do RMA/MDS) —
-- o legado de referência guarda isso como texto livre e soma por ILIKE
-- no relatório, o que falha silenciosamente com um erro de digitação.

CREATE TABLE sempras_atendimentos_coletivos (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unidade_id      UUID NOT NULL REFERENCES unidades (id),
    data_evento     DATE NOT NULL,
    categoria       TEXT NOT NULL CHECK (categoria IN
                        ('paif_familias', 'scfv_0_6', 'scfv_7_14', 'scfv_15_17', 'scfv_18_59', 'scfv_idosos',
                         'palestras_nao_continuado', 'pcd')),
    participantes   INT NOT NULL CHECK (participantes BETWEEN 0 AND 1000),
    descricao       TEXT NOT NULL DEFAULT '' CHECK (char_length(descricao) <= 2000),
    registrado_por  UUID REFERENCES users (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_coletivos_unidade ON sempras_atendimentos_coletivos (unidade_id, data_evento DESC);

-- +goose Down
DROP TABLE IF EXISTS sempras_atendimentos_coletivos;
