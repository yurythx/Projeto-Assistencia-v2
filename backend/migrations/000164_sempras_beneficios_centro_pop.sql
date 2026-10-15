-- +goose Up
-- SEMPRAS — Centro POP: dois tipos de benefício eventual que faltavam
-- (auxílio-gás e cobertor/agasalho), comuns no atendimento à população
-- em situação de rua.

ALTER TABLE sempras_beneficios DROP CONSTRAINT sempras_beneficios_tipo_check;
ALTER TABLE sempras_beneficios ADD CONSTRAINT sempras_beneficios_tipo_check
    CHECK (tipo IN ('natalidade', 'funeral', 'cesta_basica', 'passagem', 'aluguel_social', 'outro', 'auxilio_gas', 'cobertor_agasalho'));

-- +goose Down
ALTER TABLE sempras_beneficios DROP CONSTRAINT sempras_beneficios_tipo_check;
ALTER TABLE sempras_beneficios ADD CONSTRAINT sempras_beneficios_tipo_check
    CHECK (tipo IN ('natalidade', 'funeral', 'cesta_basica', 'passagem', 'aluguel_social', 'outro'));
