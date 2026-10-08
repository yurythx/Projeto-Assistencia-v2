-- +goose Up
-- Classificação do processo pelo Atlas (ADR 026): o procedimento homologado
-- que o processo segue e a série da TTDD que define a guarda dele. São
-- referências por valor (sem chave estrangeira): o Trâmite não depende do
-- plugin Atlas estar ativo, e a série guarda o código vigente na abertura.

ALTER TABLE tramite_processos
    ADD COLUMN atlas_procedimento_id UUID,
    ADD COLUMN codigo_ttdd TEXT CONSTRAINT tramite_processos_codigo_ttdd_check
        CHECK (codigo_ttdd IS NULL OR codigo_ttdd ~ '^[0-9]{1,2}\.0\.[0-9]{2}\.[0-9]{2}\.[0-9]{2}$');
CREATE INDEX idx_tramite_processos_ttdd ON tramite_processos (codigo_ttdd) WHERE codigo_ttdd IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_tramite_processos_ttdd;
ALTER TABLE tramite_processos DROP COLUMN IF EXISTS codigo_ttdd, DROP COLUMN IF EXISTS atlas_procedimento_id;
