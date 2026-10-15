-- +goose Up
-- SEMPRAS — Centro POP (Fase 9): abordagem de rua às vezes não consegue
-- nem o fallback já previsto (nascimento + nome da mãe, migração 000143)
-- — a pessoa pode estar em crise e não informar nada. "anonimo" é uma
-- 4ª via de identificação, explícita e auditável (não um jeito de
-- burlar a exigência pras outras unidades).

ALTER TABLE sempras_cidadaos ADD COLUMN anonimo BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE sempras_cidadaos DROP CONSTRAINT sempras_cidadaos_identificacao_check;
ALTER TABLE sempras_cidadaos ADD CONSTRAINT sempras_cidadaos_identificacao_check
    CHECK (cpf IS NOT NULL OR nis IS NOT NULL OR (data_nascimento IS NOT NULL AND nome_mae <> '') OR anonimo);

-- +goose Down
ALTER TABLE sempras_cidadaos DROP CONSTRAINT sempras_cidadaos_identificacao_check;
ALTER TABLE sempras_cidadaos ADD CONSTRAINT sempras_cidadaos_identificacao_check
    CHECK (cpf IS NOT NULL OR nis IS NOT NULL OR (data_nascimento IS NOT NULL AND nome_mae <> ''));
ALTER TABLE sempras_cidadaos DROP COLUMN anonimo;
