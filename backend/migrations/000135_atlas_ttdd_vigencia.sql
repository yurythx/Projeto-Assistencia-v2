-- +goose Up
-- Vigência e histórico da TTDD (ADR 019):
--   * série retirada de uma nova publicação não é apagada — fica REVOGADA
--     (data e edição do Diário Oficial), porque documentos já produzidos
--     continuam classificados nela e procedimentos podem apontar para ela;
--   * toda mudança de prazo, descritor, destinação ou vigência de uma série
--     guarda os valores anteriores em atlas_ttdd_historico (gatilho: vale
--     para a carga oficial e para qualquer outra escrita).

ALTER TABLE atlas_classificacao_ttdd ADD COLUMN revogada_em DATE;
ALTER TABLE atlas_classificacao_ttdd ADD COLUMN revogada_edicao TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_atlas_ttdd_vigentes ON atlas_classificacao_ttdd (codigo) WHERE revogada_em IS NULL;

CREATE TABLE atlas_ttdd_historico (
    id             BIGSERIAL PRIMARY KEY,
    codigo         TEXT NOT NULL REFERENCES atlas_classificacao_ttdd (codigo) ON UPDATE CASCADE ON DELETE CASCADE,
    evento         TEXT NOT NULL CHECK (evento IN ('ALTERADA', 'REVOGADA', 'RESTABELECIDA')),
    -- Valores da série ANTES da mudança.
    anterior       JSONB NOT NULL,
    -- Publicação que trouxe a mudança (a edição vigente do órgão).
    edicao_diario  TEXT NOT NULL DEFAULT '',
    registrado_em  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_atlas_ttdd_historico_codigo ON atlas_ttdd_historico (codigo, registrado_em DESC, id DESC);

-- +goose StatementBegin
CREATE FUNCTION atlas_ttdd_registrar_historico() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    v_evento TEXT := 'ALTERADA';
    v_edicao TEXT;
BEGIN
    IF OLD.revogada_em IS NULL AND NEW.revogada_em IS NOT NULL THEN
        v_evento := 'REVOGADA';
        v_edicao := NEW.revogada_edicao;
    ELSIF OLD.revogada_em IS NOT NULL AND NEW.revogada_em IS NULL THEN
        v_evento := 'RESTABELECIDA';
    END IF;
    IF v_edicao IS NULL THEN
        SELECT o.edicao_diario INTO v_edicao FROM atlas_ttdd_subfuncoes s
            JOIN atlas_ttdd_funcoes f ON f.codigo = s.funcao_codigo
            JOIN atlas_ttdd_orgaos o ON o.prefixo = f.orgao_prefixo
            WHERE s.codigo = NEW.subfuncao_codigo;
    END IF;
    INSERT INTO atlas_ttdd_historico (codigo, evento, anterior, edicao_diario)
    VALUES (NEW.codigo, v_evento, jsonb_build_object(
        'descritor', OLD.descritor,
        'fase_corrente_anos', OLD.fase_corrente_anos,
        'fase_corrente_condicao', OLD.fase_corrente_condicao,
        'fase_interm_anos', OLD.fase_interm_anos,
        'fase_interm_condicao', OLD.fase_interm_condicao,
        'destinacao_final', OLD.destinacao_final,
        'observacoes', OLD.observacoes), COALESCE(v_edicao, ''));
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- Só mudanças de conteúdo ou de vigência (a carga idempotente regrava as
-- mesmas linhas sem gerar histórico).
CREATE TRIGGER trg_atlas_ttdd_historico AFTER UPDATE ON atlas_classificacao_ttdd FOR EACH ROW
    WHEN (OLD.descritor IS DISTINCT FROM NEW.descritor
       OR OLD.fase_corrente_anos IS DISTINCT FROM NEW.fase_corrente_anos
       OR OLD.fase_corrente_condicao IS DISTINCT FROM NEW.fase_corrente_condicao
       OR OLD.fase_interm_anos IS DISTINCT FROM NEW.fase_interm_anos
       OR OLD.fase_interm_condicao IS DISTINCT FROM NEW.fase_interm_condicao
       OR OLD.destinacao_final IS DISTINCT FROM NEW.destinacao_final
       OR OLD.observacoes IS DISTINCT FROM NEW.observacoes
       OR (OLD.revogada_em IS NULL) <> (NEW.revogada_em IS NULL))
    EXECUTE FUNCTION atlas_ttdd_registrar_historico();

-- +goose Down
DROP TRIGGER IF EXISTS trg_atlas_ttdd_historico ON atlas_classificacao_ttdd;
DROP FUNCTION IF EXISTS atlas_ttdd_registrar_historico();
DROP TABLE IF EXISTS atlas_ttdd_historico;
DROP INDEX IF EXISTS idx_atlas_ttdd_vigentes;
ALTER TABLE atlas_classificacao_ttdd DROP COLUMN IF EXISTS revogada_edicao;
ALTER TABLE atlas_classificacao_ttdd DROP COLUMN IF EXISTS revogada_em;
