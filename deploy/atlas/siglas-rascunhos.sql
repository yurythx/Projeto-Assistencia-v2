-- Alinha as siglas das etapas dos RASCUNHOS às siglas do cadastro de unidades
-- (deploy/estrutura; a sede de cada órgão leva a sigla do órgão). Os avisos
-- do Atlas chegam à unidade pela sigla da etapa ou pelo primeiro segmento
-- ("SEGEP/RH" → SEGEP). Procedimento homologado não é tocado: muda por nova
-- versão. Idempotente. Gerado junto com gerar-rascunhos*.mjs.
--   docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U <user> -d <db> < deploy/atlas/siglas-rascunhos.sql
BEGIN;
UPDATE atlas_etapas e SET unidade_administrativa = m.nova
  FROM (VALUES
  ('CAEP', 'SEGEP/CAEP'),
  ('CCPAD', 'SEMAD/CCPAD'),
  ('CMAS', 'SEMPRAS/CMAS'),
  ('CMS', 'SMS/CMS'),
  ('CODIP', 'SEDEC/CODIP'),
  ('CONSEMMA', 'SEMMA/CONSEMMA'),
  ('CONTROLADORIA', 'UCCI'),
  ('CPAD', 'SEGEP/CPAD'),
  ('GABINETE', 'GAB'),
  ('LACEN_AGUA', 'SMS/LACEN_AGUA'),
  ('OUVIDORIA', 'UCCI/OUVIDORIA'),
  ('PROTOCOLO', 'SEMAD/PROTOCOLO'),
  ('SEFAZ/ARRECADACAO', 'SEREC/ARRECADACAO'),
  ('SEFAZ/ATENDIMENTO', 'SEREC/ATENDIMENTO'),
  ('SEFAZ/CADASTRO', 'SEREC/CADASTRO'),
  ('SEFAZ/CONTABILIDADE', 'SEFIN/CONTABILIDADE'),
  ('SEFAZ/CONTROLE_URBANO', 'SEREC/CONTROLE_URBANO'),
  ('SEFAZ/DIVIDA', 'SEREC/DIVIDA'),
  ('SEFAZ/FISCALIZACAO', 'SEREC/FISCALIZACAO'),
  ('SEFAZ/LICENCIAMENTO', 'SEREC/LICENCIAMENTO'),
  ('SEFAZ/PATRIMONIO', 'SEFIN/PATRIMONIO'),
  ('SEFAZ/RECEITA', 'SEREC/RECEITA'),
  ('SEFAZ/TESOURARIA', 'SEFIN/TESOURARIA'),
  ('SEFAZ/TRIBUTARIO', 'SEREC/TRIBUTARIO'),
  ('SIC', 'SEMAD/SIC'),
  ('SMPAS/ATENDIMENTO', 'SEMPRAS/ATENDIMENTO'),
  ('SMPAS/AVALIACAO', 'SEMPRAS/AVALIACAO'),
  ('SMPAS/COMISSAO', 'SEMPRAS/COMISSAO'),
  ('SMPAS/GESTAO', 'SEMPRAS/GESTAO'),
  ('SMPAS/VIGILANCIA', 'SEMPRAS/VIGILANCIA'),
  ('TI', 'SEMAD/TI'),
  ('UBS', 'SMS/UBS'),
  ('VISA', 'SMS/VISA')
  ) AS m(antiga, nova), atlas_workflows w
 WHERE w.id = e.workflow_id AND w.situacao <> 'HOMOLOGADO' AND e.unidade_administrativa = m.antiga;
COMMIT;
