-- +goose Up
-- SEMPRAS — perfil do coordenador de unidade (grupos "Grupo <Unidade>
-- Coordenação" do AD). Mesmas permissões do superintendente, mas concedido
-- NA UNIDADE: fila, atendimento e gestão (painel, cancelar entradas) só da
-- unidade que coordena. Sem o sigilo da psicologia.
INSERT INTO perfis (slug, nome, descricao, permissoes) VALUES
    ('sempras-coordenador', 'SEMPRAS — Coordenador',
     'Conceda na unidade: atendimento e gestão (painel, cancelar entradas) da unidade que coordena.',
     ARRAY['sempras:recepcao', 'sempras:atendimento', 'sempras:gestao'])
ON CONFLICT (slug) DO NOTHING;

-- +goose Down
DELETE FROM ad_group_mappings WHERE perfil_id IN (SELECT id FROM perfis WHERE slug = 'sempras-coordenador');
DELETE FROM user_scopes WHERE perfil_id IN (SELECT id FROM perfis WHERE slug = 'sempras-coordenador');
DELETE FROM perfis WHERE slug = 'sempras-coordenador';
