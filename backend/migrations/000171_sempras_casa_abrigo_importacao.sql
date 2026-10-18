-- +goose Up
-- SEMPRAS — importação do histórico do Casa Abrigo 4.0 (ADR 033). Cada
-- registro importado deixa aqui a origem (tabela e id no 4.0) e o destino:
-- rodar a importação de novo não duplica nada, e se sabe de onde veio
-- cada acolhimento, evolução, visita etc.
CREATE TABLE sempras_casa_abrigo_importacao (
    unidade_id   UUID        NOT NULL REFERENCES unidades(id) ON DELETE RESTRICT,
    origem       TEXT        NOT NULL CHECK (origem IN ('acolhidos', 'evolucoes_tecnicas', 'visitantes_voluntarios',
                     'documentos_termos', 'pia', 'pia_plano_acao', 'diario_logs', 'relatorios_gerais', 'scrum_tarefas')),
    origem_id    BIGINT      NOT NULL,
    destino_id   UUID        NOT NULL,
    importado_em TIMESTAMPTZ NOT NULL DEFAULT now(),
    importado_por UUID       REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (unidade_id, origem, origem_id)
);

-- +goose Down
DROP TABLE sempras_casa_abrigo_importacao;
