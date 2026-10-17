-- +goose Up
-- SEMPRAS — sexo do munícipe. O RMA do MDS recorta pessoas atendidas e
-- vítimas por sexo e faixa etária; sem o campo, o relatório não sai no
-- formato oficial. '' = não informado (o cadastro antigo e quem não quis
-- declarar). Não é identificador: a anonimização (§8) o mantém, como mantém
-- as contagens.

ALTER TABLE sempras_cidadaos
    ADD COLUMN sexo TEXT NOT NULL DEFAULT '' CHECK (sexo IN ('', 'feminino', 'masculino'));

-- +goose Down
ALTER TABLE sempras_cidadaos DROP COLUMN sexo;
