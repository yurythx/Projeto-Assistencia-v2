-- +goose Up
-- SEMPRAS — Benefícios Eventuais: forma de acesso (como a demanda chegou
-- até a unidade — LOAS/NOB-SUAS distingue demanda espontânea de busca
-- ativa e de encaminhamento da rede, usado pelo RMA/MDS) e dois tipos que
-- faltavam (aluguel social, muito comum em incêndio/enchente, e "outro").

ALTER TABLE sempras_beneficios DROP CONSTRAINT sempras_beneficios_tipo_check;
ALTER TABLE sempras_beneficios ADD CONSTRAINT sempras_beneficios_tipo_check
    CHECK (tipo IN ('natalidade', 'funeral', 'cesta_basica', 'passagem', 'aluguel_social', 'outro'));

ALTER TABLE sempras_beneficios ADD COLUMN forma_acesso TEXT NOT NULL DEFAULT 'demanda_espontanea' CHECK (forma_acesso IN
    ('demanda_espontanea', 'busca_ativa', 'encaminhamento_rede_socioassistencial', 'encaminhamento_sistema_garantia_direitos'));

-- +goose Down
ALTER TABLE sempras_beneficios DROP COLUMN forma_acesso;
ALTER TABLE sempras_beneficios DROP CONSTRAINT sempras_beneficios_tipo_check;
ALTER TABLE sempras_beneficios ADD CONSTRAINT sempras_beneficios_tipo_check
    CHECK (tipo IN ('natalidade', 'funeral', 'cesta_basica', 'passagem'));
