-- +goose Up
-- Trâmite: credencial de acesso passa a ser revogável. A revogação fica no
-- histórico append-only como qualquer outra movimentação (quem, quando).
ALTER TABLE tramite_movimentos DROP CONSTRAINT tramite_movimentos_acao_check,
    ADD CONSTRAINT tramite_movimentos_acao_check
        CHECK (acao IN ('abertura', 'tramitacao', 'conclusao', 'arquivamento', 'reabertura',
                        'documento', 'assinatura_solicitada', 'assinatura_concluida', 'acesso_concedido',
                        'acesso_revogado'));

-- +goose Down
-- O histórico é append-only: revogações já registradas não podem ser
-- apagadas, então o Down só restaura a lista antiga se não houver nenhuma.
ALTER TABLE tramite_movimentos DROP CONSTRAINT tramite_movimentos_acao_check,
    ADD CONSTRAINT tramite_movimentos_acao_check
        CHECK (acao IN ('abertura', 'tramitacao', 'conclusao', 'arquivamento', 'reabertura',
                        'documento', 'assinatura_solicitada', 'assinatura_concluida', 'acesso_concedido'));
