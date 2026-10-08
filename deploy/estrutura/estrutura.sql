-- GERADO por scripts/estrutura/ad-para-estrutura.mjs a partir do export do AD — não edite à mão.
-- Estrutura organizacional real (entidades, unidades, departamentos). Idempotente.
-- Aplicar: make estrutura-aplicar
BEGIN;

-- Administração
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('2913c7f3-b37c-517c-b698-17f40dc95c32', 'Administração', 'SEMAD', 'administracao')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('d3557ec1-90a0-56d8-a80c-8232e2b5d6eb', '2913c7f3-b37c-517c-b698-17f40dc95c32', NULL, 'Sede', 'SEMAD', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Agricultura e Pecuária
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('dd679d31-4a1e-53bb-bf5e-afa1c5d3b14e', 'Agricultura e Pecuária', 'SEMAGRI', 'agricultura-e-pecuaria')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a7ffaa84-f21d-5e0c-9091-9b19de711661', 'dd679d31-4a1e-53bb-bf5e-afa1c5d3b14e', NULL, 'Sede', 'SEMAGRI', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Ciência, Tecnologia e Inovação
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('0cde932c-a8d0-5bbb-9f49-46486ad9ccdb', 'Ciência, Tecnologia e Inovação', 'SECITI', 'ciencia-tecnologia-e-inovacao')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5ed40e60-26e2-57e0-9698-e267515cf348', '0cde932c-a8d0-5bbb-9f49-46486ad9ccdb', NULL, 'Sede', 'SECITI', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Cultura
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('18e3b74d-2495-5d5b-b412-7afff14515b0', 'Cultura', 'SECULT', 'cultura')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5126015b-16ec-5c0f-9294-2ad00df7b58a', '18e3b74d-2495-5d5b-b412-7afff14515b0', NULL, 'Sede', 'SECULT', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Desenvolvimento Econômico
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('a7bbbdf0-dc9e-54c5-987a-830c73e7fef0', 'Desenvolvimento Econômico', 'SEDEC', 'desenvolvimento-economico')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('e8d0f774-f182-5e4f-b0ab-c69177a9ee17', 'a7bbbdf0-dc9e-54c5-987a-830c73e7fef0', NULL, 'Sede', 'SEDEC', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Esporte e Lazer
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('227ab076-cca5-5d6b-a862-9aa50c7bcbc3', 'Esporte e Lazer', 'SEMEL', 'esporte-e-lazer')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b76abef2-585c-5d6e-8fd7-cf9cb3686c3a', '227ab076-cca5-5d6b-a862-9aa50c7bcbc3', NULL, 'Sede', 'SEMEL', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Finanças
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('434b8b7a-9ad2-5a32-adab-37201cd561cc', 'Finanças', 'SEFIN', 'financas')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f6495e52-92a9-535d-b7e5-a2fc80b4d297', '434b8b7a-9ad2-5a32-adab-37201cd561cc', NULL, 'Sede', 'SEFIN', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Gabinete Comunicação
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('213b8f30-6543-5255-866d-876578a4d373', 'Gabinete Comunicação', 'GAB', 'gabinete-comunicacao')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('85156e0f-8178-5a9b-9987-6daf65418817', '213b8f30-6543-5255-866d-876578a4d373', NULL, 'Sede', 'GAB', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Gestão de Pessoas
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('da126408-3525-516f-acd6-4e1c312a339e', 'Gestão de Pessoas', 'SEGEP', 'gestao-de-pessoas')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('d84bcbce-54ec-5f3c-93c0-69b5f9a50360', 'da126408-3525-516f-acd6-4e1c312a339e', NULL, 'Sede', 'SEGEP', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Governo
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('f9e2aeb0-fb45-55cc-b10a-7a4066e7e147', 'Governo', 'SEGOV', 'governo')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('696e621e-a985-5f8e-8975-8084d4ea0775', 'f9e2aeb0-fb45-55cc-b10a-7a4066e7e147', NULL, 'Sede', 'SEGOV', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Habitação
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('0e13489f-62e0-581e-b6e6-da6fd0363771', 'Habitação', 'SEHAB', 'habitacao')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('7926ff91-d8b0-5211-8f16-bc37ea1695f9', '0e13489f-62e0-581e-b6e6-da6fd0363771', NULL, 'Sede', 'SEHAB', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- IPPUR - SINFRA
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('128199a5-98aa-5d88-b2ee-f4ae54ef2b52', 'IPPUR - SINFRA', 'IPPUR', 'ippur-sinfra')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('28df788d-5712-5436-a934-6be00ef2df01', '128199a5-98aa-5d88-b2ee-f4ae54ef2b52', NULL, 'Sede', 'IPPUR', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Meio Ambiente
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('5033a2f8-0aba-58d3-a29e-b755119ae6fe', 'Meio Ambiente', 'SEMMA', 'meio-ambiente')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('25f7c11a-5e22-5c95-b3c7-daffdb82f0e6', '5033a2f8-0aba-58d3-a29e-b755119ae6fe', NULL, 'Sede', 'SEMMA', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Pesquisa e Planejamento Urbano
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('063ca411-e0b6-53c7-8b72-912714f8e290', 'Pesquisa e Planejamento Urbano', 'SEPPU', 'pesquisa-e-planejamento-urbano')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('ff426485-c747-586a-bc34-00aaa8889ff0', '063ca411-e0b6-53c7-8b72-912714f8e290', NULL, 'Sede', 'SEPPU', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Planejamento
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('232984a5-59a9-5865-afff-a17b7b335c4f', 'Planejamento', 'SEPLAN', 'planejamento')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('7e64cca2-cb6d-57cc-93c5-ec0e15493480', '232984a5-59a9-5865-afff-a17b7b335c4f', NULL, 'Sede', 'SEPLAN', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- PROCON
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('d7963656-57f8-55e8-864c-8c42151c29b1', 'PROCON', 'PROCON', 'procon')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('57a1246f-e74e-5015-b60a-55f26a8e0b46', 'd7963656-57f8-55e8-864c-8c42151c29b1', NULL, 'Sede', 'PROCON', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Procuradoria Geral
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('e0cff087-9697-524c-8ac4-60412b104db1', 'Procuradoria Geral', 'PGM', 'procuradoria-geral')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('d19cc18b-ca2e-5a82-9031-a9d0cf3b6413', 'e0cff087-9697-524c-8ac4-60412b104db1', NULL, 'Sede', 'PGM', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Receita
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('9c2ff196-6de2-5314-903d-1011a1d8990a', 'Receita', 'SEREC', 'receita')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f7d2f509-c78a-5fc0-88f5-0514f4970736', '9c2ff196-6de2-5314-903d-1011a1d8990a', NULL, 'Sede', 'SEREC', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Segurança Pública
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('7a9a657c-9160-5180-8e48-08072827efbf', 'Segurança Pública', 'SESP', 'seguranca-publica')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a1b3ead5-5b2a-5619-9391-36bad2d9f4c9', '7a9a657c-9160-5180-8e48-08072827efbf', NULL, 'Sede', 'SESP', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- SINFRA
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('1f4990ea-8b47-5dcc-87dc-5c349088e67d', 'SINFRA', 'SINFRA', 'sinfra')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8cb3a7f2-9002-5a97-a133-358a29a0ff97', '1f4990ea-8b47-5dcc-87dc-5c349088e67d', NULL, 'Sede', 'SINFRA', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Transportes e Trânsito
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('5bf1edc1-bd04-5b96-acb9-1462c481a2e9', 'Transportes e Trânsito', 'SETRAT', 'transportes-e-transito')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('883b3bae-b1d2-5337-a833-bd52c94a3e0d', '5bf1edc1-bd04-5b96-acb9-1462c481a2e9', NULL, 'Sede', 'SETRAT', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Unidade Central de Controle Interno
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('353d0ae0-ed0d-5160-a981-96f8ba56e557', 'Unidade Central de Controle Interno', 'UCCI', 'unidade-central-de-controle-interno')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5b50f4f5-658a-5b24-962b-4a242888de40', '353d0ae0-ed0d-5160-a981-96f8ba56e557', NULL, 'Sede', 'UCCI', 'sede')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Secretaria Municipal de Educação
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('494a38e0-eb83-5695-a0a5-ed9a793fd1c0', 'Secretaria Municipal de Educação', 'SEMED', 'semed')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0007e74f-60b3-5eef-a8be-2d706a7e6589', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', NULL, 'Secretaria Executiva', 'SEMED', 'secretaria-executiva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('aef0fcc4-35b4-5861-93b3-de33b1da59a8', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Almoxarifado', 'almoxarifado')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('808a710d-b5fd-584d-a3a2-bf868a9e8ccd', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Avaliação e Monitoramento da Aprendizagem', 'avaliacao-e-monitoramento-da-aprendizagem')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('0933a6f3-22d9-5331-afb7-75652de409f8', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Comunicação', 'comunicacao')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('4dba8ce9-29d8-5f1e-a956-7bb9eb12eb39', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Conselho de Educação', 'conselho-de-educacao')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('31d5ea03-8b6d-59c3-a77e-39588ac1274d', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'DTI', 'dti')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('9d67b940-a330-5a08-a03b-e203aa87ead3', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Educação Inclusiva', 'educacao-inclusiva')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('2ddc4a60-52df-58e5-a56a-b373e74ea0dc', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Educação Infantil', 'educacao-infantil')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('606dffb7-4a41-56fc-a097-7d190bd13a87', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Engenharia Arquitetura', 'engenharia-arquitetura')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('4b1c9626-d46c-5de8-807e-1adb944e57cd', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Ensino Fundamental', 'ensino-fundamental')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('79be632a-23b1-51fb-9950-c6e037a5e7f4', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Financeiro SEMED', 'financeiro-semed')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('86002909-b472-5349-9e71-bc68ff9b855d', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Formação Profissional', 'formacao-profissional')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('743674b9-e6f4-5a61-b9a7-a312d2646efc', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Gabinete', 'gabinete')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('756242e3-b089-5cc0-bf91-f85873a733c9', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Gestão Escolar', 'gestao-escolar')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('b01ed610-aa35-5c5e-b0ed-2ca91b3fc677', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Gestão Infraestrutura', 'gestao-infraestrutura')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('300d5206-ce88-5a9c-ab9c-d90e80530cff', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Jurídico', 'juridico')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('b68b90f5-d782-538c-824a-a157f07154ef', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Merenda', 'merenda')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('be827edd-4ce2-5be2-90fb-7439bd7402c8', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Planejamento Finanças', 'planejamento-financas')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('5d967e1f-a052-5df3-b0ab-530ca17ded36', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Programas e Projetos', 'programas-e-projetos')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('070c42c5-09f1-54d0-8a3a-bdf3a8ca023e', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Projetos Educacionais', 'projetos-educacionais')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('07b60658-c1e4-5e82-ab69-cf76de827854', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Recursos Humanos', 'recursos-humanos')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('bb8d1e28-f77d-599b-9bc5-e1d63e644daf', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Serviços Gerais', 'servicos-gerais')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('040c42ff-8c58-5667-9e0a-a5daefd28cc1', '0007e74f-60b3-5eef-a8be-2d706a7e6589', 'Transporte Escolar', 'transporte-escolar')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('30543421-239a-5e5b-8a70-f19c6a1d8f4b', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', NULL, 'Escola de Música', '', 'escola-de-musica')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('7c080d7a-0a82-59a9-b646-d7ab32d788b0', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', NULL, 'Educação Infantil', '', 'educacao-infantil')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0b8f3782-7440-559d-8c13-ddb4539d6740', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Agton Kayro Leite dos Santos', '', 'cmei-agton-kayro-leite-dos-santos')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('de6c946d-ade9-5a10-9ba2-62d5631db2e2', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Anterina Miranda de Moraes', '', 'cmei-anterina-miranda-de-moraes')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('835a8128-803b-5ca6-ae13-af30b2d9f80b', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Antônio Vânier de Oliveira', '', 'cmei-antonio-vanier-de-oliveira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('e72aefa3-fe96-544d-99a2-40a8d9632657', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Aparecida de Souza Vetorasso', '', 'cmei-aparecida-de-souza-vetorasso')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('04b4a279-1b26-5122-ab03-5387e5de9233', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Arthur Araujo Lula da Silva', '', 'cmei-arthur-araujo-lula-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('3a7a950a-ff00-5fdd-9498-794ebd4ed413', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Augustim Alves de Oliveira', '', 'cmei-augustim-alves-de-oliveira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('e388439c-bacb-54b6-83a6-7620c8a3b8b1', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Bruna Cristina da Silva Santos', '', 'cmei-bruna-cristina-da-silva-santos')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0278f382-7481-5499-9cad-37991691fd55', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Carlos Alberto de Carvalho', '', 'cmei-carlos-alberto-de-carvalho')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('35808c7f-e213-5bad-a17d-d75ecf0855eb', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Celina Fialho Bezerra', '', 'cmei-celina-fialho-bezerra')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('16925d66-2792-51ec-8fb7-f6c7bc8955c7', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Ely Carlos', '', 'cmei-ely-carlos')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('46cccf55-16ec-5ec9-82c3-29b22abc4557', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Enezio Machado Vieira', '', 'cmei-enezio-machado-vieira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('78cf38a3-8dc2-5a65-9aea-53a9fc3ca4b7', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Giovanni Gomes Moreira', '', 'cmei-giovanni-gomes-moreira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('841bafb4-721d-5648-b172-10e86d8fdf14', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Hildegard Erika Bauchrowitz', '', 'cmei-hildegard-erika-bauchrowitz')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('15afb325-ab4e-5f30-90d2-267864069771', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Iracy Pereira da Conceicao Araujo', '', 'cmei-iracy-pereira-da-conceicao-araujo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4aa88bc2-52e0-5fac-a9d2-13bdc4192288', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Jessica Adriana Lima Ferreira', '', 'cmei-jessica-adriana-lima-ferreira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8ecd933b-876c-589d-909a-655f4827ae6b', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Joana Maria dos Anjos Meireles', '', 'cmei-joana-maria-dos-anjos-meireles')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('cc61a7c2-c226-5d9e-9051-383bedf82f37', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI João Cesar Domingos da Silva', '', 'cmei-joao-cesar-domingos-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8d893305-ac1e-5603-841a-37323d213cb3', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI João Lopes da Silva', '', 'cmei-joao-lopes-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4599aa41-355e-5dd0-a963-bdde42eb9f5c', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Jonas Nunes Cavalcante', '', 'cmei-jonas-nunes-cavalcante')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('60b74878-7287-5101-956b-eaa85cce93a5', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Jose Antonio de Oliveira', '', 'cmei-jose-antonio-de-oliveira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b014f02b-1451-572c-a314-55de3c3a04ba', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Leonese de Pinho Carvalho', '', 'cmei-leonese-de-pinho-carvalho')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('d3dbd428-a447-5e32-9fd8-cb3702551f84', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Mãe Margarida', '', 'cmei-mae-margarida')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('3f9de743-b222-5810-8238-70db872be141', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Magnólia Angélica Araújo', '', 'cmei-magnolia-angelica-araujo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f79ef6b5-1c25-5a2b-8405-6f6c9b7633ea', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Marcia Gleide Ribeiro Clara Souto', '', 'cmei-marcia-gleide-ribeiro-clara-souto')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('946ccd7f-84c8-5291-8690-d5d0e02136a1', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Maria Amelia de Araujo', '', 'cmei-maria-amelia-de-araujo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('ebbb0e44-b0d7-5e1a-a018-6a2a34467414', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Maria das Graças', '', 'cmei-maria-das-gracas')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('18332bda-fed9-574c-a565-abf650484508', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Maria Severina da Silva', '', 'cmei-maria-severina-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1ca4e6a3-0dde-5d4d-b736-64578da0790d', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Natalia Maximo Lima', '', 'cmei-natalia-maximo-lima')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f72deeb8-fed4-58ae-b207-47aaae2416a8', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Prof. Alessandro Gomes de Jesus', '', 'cmei-prof-alessandro-gomes-de-jesus')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('df07d2b1-cdf1-54ec-b545-8fac4502f838', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Prof. Geraldo Jose de Oliveira', '', 'cmei-prof-geraldo-jose-de-oliveira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('2e38397b-f837-526f-b776-fb07184ad044', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Prof. Vilma Moreira dos Santos', '', 'cmei-prof-vilma-moreira-dos-santos')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f7c4bd8c-d326-5d98-8db7-9a237ae2f1f7', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Professora Ivan Santos Arruda', '', 'cmei-professora-ivan-santos-arruda')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('fe9c5b40-0f59-55f3-8e7b-09410f6de79a', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Professora Liege Santos Pereira', '', 'cmei-professora-liege-santos-pereira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('396a614d-2007-5990-ad04-e2e9161e5ed8', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Rita Maria Correia', '', 'cmei-rita-maria-correia')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f1665688-395d-5c1f-885b-b54f7c0ef80b', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEI Widisney Aparecido Pereira Rodrigues', '', 'cmei-widisney-aparecido-pereira-rodrigues')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('cfe028a2-c575-5017-83bd-c1eeffb9142a', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEITI Engenheiro Nafes Antonio Daud', '', 'cmeiti-engenheiro-nafes-antonio-daud')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('99298d08-80f4-5b19-84c5-aab0050dc920', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEITI Maria de Souza Miranda', '', 'cmeiti-maria-de-souza-miranda')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4425300f-81d1-5f3a-acf0-75cdb3ae991f', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'CMEITI Natália Junqueira Botelho de Azevedo', '', 'cmeiti-natalia-junqueira-botelho-de-azevedo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('00e97f79-98ac-513e-b596-14121526a086', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'EMEI Cora Coralina', '', 'emei-cora-coralina')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1140dda5-cc8f-5708-b8cc-6006513ddd27', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'EMEI Elaine Aparecida de Oliveira Lopes', '', 'emei-elaine-aparecida-de-oliveira-lopes')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5c6bcebd-4c66-5296-9baa-f82737f4d845', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'EMEI Machado de Assis', '', 'emei-machado-de-assis')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('6c9217e6-936f-5a6f-ae18-92f101d5c6c9', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'EMEI Mateus Vinicius', '', 'emei-mateus-vinicius')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('31c07d59-2f4a-555e-9413-908a58c72486', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'EMEI Rubens Alves de Souza', '', 'emei-rubens-alves-de-souza')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('e7ab06a6-0658-5f3b-87ac-69573495833c', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'EMEI Selma Doho', '', 'emei-selma-doho')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('7115d427-1311-5d25-9fa4-add797080f1d', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI Charmene Rosa da Silva', '', 'umei-charmene-rosa-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('3f917b3c-5294-568d-b016-f98b136a0566', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI Gabriel de Oliveira Dias', '', 'umei-gabriel-de-oliveira-dias')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('556794fd-2a90-5807-bc63-1597889b32f3', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI João de Paula Mendonça de Souza', '', 'umei-joao-de-paula-mendonca-de-souza')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b65a2f74-dfc0-51a8-97e9-a8dac274b03d', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI José dos Reis Sales', '', 'umei-jose-dos-reis-sales')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0e875a44-21e4-57af-933f-826d821bad8d', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI Luiz Henrique Dias Bulhões', '', 'umei-luiz-henrique-dias-bulhoes')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('54b08dce-446f-5a7c-a30d-64dc08ae4c2c', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI Mateus Vinicius Braz', '', 'umei-mateus-vinicius-braz')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0ccd899d-b24d-508e-b359-015acbd9c616', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI Monteiro Lobato', '', 'umei-monteiro-lobato')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('6e1d50ae-9144-58a3-a651-ea3fe9f1619e', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI Natalia Maximo Lima', '', 'umei-natalia-maximo-lima')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('019b69c8-c827-5cde-9731-6dd86413e591', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '7c080d7a-0a82-59a9-b646-d7ab32d788b0', 'UMEI Pequenos Brilhantes', '', 'umei-pequenos-brilhantes')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0b665951-5ab4-55ce-bd9a-50d7537f7bab', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', NULL, 'Ensino Fundamental', '', 'ensino-fundamental')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('19f8a8e3-bf42-50d3-bcf1-ab0302871c9d', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Aparecida de Souza Vetorasso', '', 'emeb-aparecida-de-souza-vetorasso')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a26850fa-d8e1-5578-a995-d327ee70bb87', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Bernardo Venancio de Carvalho', '', 'emeb-bernardo-venancio-de-carvalho')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b7a592ef-99ee-5009-8e22-08d1b0dca352', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Celson Antonio de Carvalho', '', 'emeb-celson-antonio-de-carvalho')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4f2ed1d1-17ad-55d2-a059-a1e7da2f9d50', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Dom Wunibaldo Talleur', '', 'emeb-dom-wunibaldo-talleur')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a821339f-9634-5d08-9e51-7491726b4a0b', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Evania Rodrigues da Silva', '', 'emeb-evania-rodrigues-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('425b9909-bc9a-5766-86a0-0a3214eda6de', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Giselio da Nobrega', '', 'emeb-giselio-da-nobrega')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1798eed8-5c3d-5838-a4d5-09b1c0b6cc8e', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Gleba Dom Bosco', '', 'emeb-gleba-dom-bosco')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('617fd537-3140-56c5-9a7f-d75279c53559', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Irmã Elza Geovanella', '', 'emeb-irma-elza-geovanella')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('cc9bfa2f-2722-5d88-a7e1-8b7bd6261eff', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB João Evangelista Porto Fernandes', '', 'emeb-joao-evangelista-porto-fernandes')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('14fedb84-c0f6-5ec3-a203-2ad369059d39', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Mario de Andrade', '', 'emeb-mario-de-andrade')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b9292963-505b-5ae9-a108-e93484007eaa', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Melchiades Figueiredo Miranda', '', 'emeb-melchiades-figueiredo-miranda')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('9b339586-dd40-509f-9bab-c736bf980b05', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Nossa Senhora Aparecida', '', 'emeb-nossa-senhora-aparecida')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('7a3987a3-7bb0-5806-854c-79faddb109ec', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Odorico Leocadio da Rosa', '', 'emeb-odorico-leocadio-da-rosa')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('493ac6cc-743b-5b92-8815-7bdd3b3f9e35', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Padre Joao Paulo Nolli', '', 'emeb-padre-joao-paulo-nolli')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('fe32aadc-e9bc-522a-9474-316e303b5499', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Prefeito Fausto de Souza Faria', '', 'emeb-prefeito-fausto-de-souza-faria')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('cc438bc8-99a1-54bd-8f95-48a4aba6130e', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Prof. Maria Aparecida de Oliveira', '', 'emeb-prof-maria-aparecida-de-oliveira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a3950155-0444-5141-bb29-ef08f2ec9df6', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Prof. Evania Rodrigues da Silva', '', 'emeb-prof-evania-rodrigues-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('cd26d383-a96d-51a3-a3b4-ea29b7b2b43b', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Prof. Gildazia Souza Pirozzi', '', 'emeb-prof-gildazia-souza-pirozzi')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b907c005-3d55-58ab-a1c9-7e281b51b852', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Prof. Sebastiana R. de Souza', '', 'emeb-prof-sebastiana-r-de-souza')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1dd1e197-fe59-5ff5-b4b5-d6c2820d4ce4', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Professor Carlos Pereira Barbosa', '', 'emeb-professor-carlos-pereira-barbosa')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('06bd8c92-8080-58c7-aae5-949e25d27614', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Professora Dulcineia Cascão Barbosa', '', 'emeb-professora-dulcineia-cascao-barbosa')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('aaf64c69-7ae4-5bd6-8140-648d85c4c027', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Professora Renilda Silva Moraes', '', 'emeb-professora-renilda-silva-moraes')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('825f5a4b-06de-57c2-8b6e-fb9e2d15cf3e', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Professora Sebastiana Rodrigues de Souza', '', 'emeb-professora-sebastiana-rodrigues-de-souza')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0edfea6e-cafa-5f5b-818d-8e0ee79a8e54', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Terezinha Silva de Souza', '', 'emeb-terezinha-silva-de-souza')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('bb463681-c4dd-5f83-8664-61f0c3dab1df', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEB Vereador Rozendo Ferreira de Souza', '', 'emeb-vereador-rozendo-ferreira-de-souza')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('37b0f58c-b087-547f-b137-e5b76f5c3f22', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEBTI Altamirando de Araújo Miranda', '', 'emebti-altamirando-de-araujo-miranda')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f2c51d63-8dbf-5dfd-ba98-7d84adae8c03', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEBTI Prof. Virgilina de Melo Ferreira', '', 'emebti-prof-virgilina-de-melo-ferreira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('d9ca755c-4dae-5e6c-8df7-42ad0173fbc1', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Albino Saldanha Dantas', '', 'emef-albino-saldanha-dantas')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('adecd591-44fa-511a-87c1-7cc297db3d5a', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Alcides Pereira dos Santos', '', 'emef-alcides-pereira-dos-santos')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('79d740fd-3e2f-5bf6-bf6b-9f55bc5f0e6c', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Alfredo de Castro Araujo', '', 'emef-alfredo-de-castro-araujo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('00d90528-04e6-5699-a950-398930a3a9f6', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Antonio Guimaraes Balbino', '', 'emef-antonio-guimaraes-balbino')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('9e1e9236-3aaf-5794-a7b1-b6a3fc7d61a4', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Arão Gomes Bezerra', '', 'emef-arao-gomes-bezerra')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('053450c4-50d7-5105-b3bc-87bae2b8a880', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Bonifácio Sachetti', '', 'emef-bonifacio-sachetti')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('60b98dd3-f247-5d8b-843e-c7f9383bd236', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF CPAC São José', '', 'emef-cpac-sao-jose')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('688568d3-18fa-5091-8af7-0e4a21f38b1d', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Daniel Paulista Campos', '', 'emef-daniel-paulista-campos')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('575b26e8-8c15-55ed-9a4a-cb01dd213cbf', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Edivaldo Zuliano Belo', '', 'emef-edivaldo-zuliano-belo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4a40c7b5-05a8-5ea4-8e5d-55c02f71fb69', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Firmicio Alves Barreto', '', 'emef-firmicio-alves-barreto')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b966643b-323b-58d2-b7d5-437e78597a21', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Frei Milton', '', 'emef-frei-milton')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('3f69ddbb-7476-5ce6-bf40-84f4e9db8f2d', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Jose Antonio da Silva', '', 'emef-jose-antonio-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('db41f532-d0e9-527e-9473-568d2c841119', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Primeiro de Maio', '', 'emef-primeiro-de-maio')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('2e938f08-148e-55db-9fbe-81a96713aba2', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Princesa Isabel', '', 'emef-princesa-isabel')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4c64bf4a-ad98-585f-91a3-16163999dcd8', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Rosalino Antonio da Silva', '', 'emef-rosalino-antonio-da-silva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('56e5d67f-3a59-55a8-9e61-bc2e8e021a26', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Tancredo Neves', '', 'emef-tancredo-neves')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('41198fbd-c77d-5b75-b7ed-cecf6c439542', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '0b665951-5ab4-55ce-bd9a-50d7537f7bab', 'EMEF Vila Paulista', '', 'emef-vila-paulista')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5de7d01d-78aa-52ee-8847-29f95a38b013', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', NULL, 'Escolas Rurais', '', 'escolas-rurais')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('45924e27-ee04-56f8-bcbb-cc3d6345eab6', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '5de7d01d-78aa-52ee-8847-29f95a38b013', 'EIMEB Leosidio Fermau', '', 'eimeb-leosidio-fermau')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c8f90c3b-8707-55d9-bea6-c24777c83200', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '5de7d01d-78aa-52ee-8847-29f95a38b013', 'EMCEB Fazenda Carima', '', 'emceb-fazenda-carima')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('94622141-1054-53f0-997c-5621d5830682', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '5de7d01d-78aa-52ee-8847-29f95a38b013', 'EMCEB Maraja', '', 'emceb-maraja')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('bef800bb-8728-5dc7-9f03-4170bc9059aa', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '5de7d01d-78aa-52ee-8847-29f95a38b013', 'EMCEB Padre Dionisio Kuduavizcz', '', 'emceb-padre-dionisio-kuduavizcz')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b8c1ca83-4c56-5df6-91a9-03b1492afc5a', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '5de7d01d-78aa-52ee-8847-29f95a38b013', 'EMCEB Rui Barbosa', '', 'emceb-rui-barbosa')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1dd0562b-ea39-5011-b31f-3e02bdc61512', '494a38e0-eb83-5695-a0a5-ed9a793fd1c0', '5de7d01d-78aa-52ee-8847-29f95a38b013', 'EMCEB São Domingos Sávio', '', 'emceb-sao-domingos-savio')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Secretaria Municipal de Saúde
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('9a74ba6a-74e1-51eb-be89-4ab4fd59af84', 'Secretaria Municipal de Saúde', 'SMS', 'sms')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('9b67e596-ca02-5710-bcbe-4364e012a65c', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Sede da SMS', 'SMS', 'sede-da-sms')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('c3793892-8de6-5954-8d41-8357f1c1fb9e', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Ações Programáticas de Saúde', 'acoes-programaticas-de-saude')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('42477167-aef1-50e3-8fe1-52d2e9988839', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Administração e Finanças', 'administracao-e-financas')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('d7574406-593a-5d39-9a96-b4b9e1378eb7', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Arquivo da Saúde', 'arquivo-da-saude')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('62e3a15d-23cd-5938-8e58-599010a0a6a5', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Assessoria Jurídica', 'assessoria-juridica')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('b1ba41b3-e6cd-5297-b2b4-3c7b50fb09df', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Atenção à Saúde', 'atencao-a-saude')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('046d3d7b-b5ba-549f-9bc6-e0c51fa73063', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Auditor SUS', 'auditor-sus')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('8b3e7341-e1aa-5b43-9408-a41272f8fd94', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Comunicação', 'comunicacao')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('25a26908-05f3-534a-bd01-47f88e7918c3', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Educação Permanente', 'educacao-permanente')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('5469c076-541c-500f-a415-de4cf6186369', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Engenharia Arquitetura', 'engenharia-arquitetura')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('a33d7754-c8e8-504d-9bb4-e3ea4aef8358', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Gabinete', 'gabinete')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('fc9350f8-5561-5545-a9cc-169ead974954', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Gestão do SUS', 'gestao-do-sus')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('a72150a3-9dee-5d36-872a-6d5fe97bebac', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Manutenção', 'manutencao')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('6332d6bd-5073-5616-b601-79ef67136854', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Ouvidoria', 'ouvidoria')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('150f6c1f-837e-5efa-8f06-35ecc7b210b6', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Planejamento', 'planejamento')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('e6f98e87-db91-53dc-86b7-5308e8ebc26f', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'RH em Saúde', 'rh-em-saude')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('195e2df3-ad22-5d16-901c-8d5ff33fc742', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Saúde Bucal', 'saude-bucal')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('b34d66e5-fb3b-5baa-b0ad-7b6ac75fb384', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Saúde do Trabalhador', 'saude-do-trabalhador')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('9edd7727-9913-5612-a3a6-81c2912fb86a', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Transporte', 'transporte')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('70b84fa0-995c-5efb-b014-2bc62a46b9a9', '9b67e596-ca02-5710-bcbe-4364e012a65c', 'Vigilância Epidemiológica', 'vigilancia-epidemiologica')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('2564e3ac-02d5-58f6-ad77-b644a3dec9bc', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Almoxarifado', '', 'almoxarifado')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1e4014e8-a20b-52d1-99f2-f4296e67fbc4', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Assistência Farmacêutica', '', 'assistencia-farmaceutica')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('9633ceb5-7540-5b73-a53a-f88a22c5457e', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Central Regulação', '', 'central-regulacao')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('aa120749-a4df-58a3-98f4-651c81d66a76', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Conselho Municipal', '', 'conselho-municipal')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('ffe74075-ff8f-5544-8e8d-39c636a6b630', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Laboratório Central', '', 'laboratorio-central')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('ba386f7c-b0dd-55c2-b531-cc8482637a5a', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'SAMU', '', 'samu')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('e031cc2d-1d98-596b-a065-a1b473dd02d0', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Tecnologia', '', 'tecnologia')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a748ea4f-85b3-5ffd-8eca-6eb220f20b67', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Vigilância Sanitária', '', 'vigilancia-sanitaria')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4c54d23d-59dc-56f0-b417-31075ac48bdc', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Centros de Especialidades Odontológicas', 'CEO', 'centros-de-especialidades-odontologicas')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('482ae2d3-d6ba-5e2d-a3ed-6d4b0f3a7809', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '4c54d23d-59dc-56f0-b417-31075ac48bdc', 'CEO Itamaraty', '', 'ceo-itamaraty')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('34da87db-f1ea-55ca-9bc1-4aab4235fc5b', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '4c54d23d-59dc-56f0-b417-31075ac48bdc', 'CEO MAMED', '', 'ceo-mamed')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('ef3f4528-00c0-558c-8c94-287978d55a15', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '4c54d23d-59dc-56f0-b417-31075ac48bdc', 'CEO N.S. do Amparo', '', 'ceo-n-s-do-amparo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('111db73a-4884-5528-8cf0-af4a25bc6531', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Centros de Saúde', 'CS', 'centros-de-saude')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b8904f46-8f5b-5f91-b593-48deabfc59d0', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '111db73a-4884-5528-8cf0-af4a25bc6531', 'CS COHAB', '', 'cs-cohab')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('e5bd3f2a-f81b-50cc-ad55-578f57799c59', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '111db73a-4884-5528-8cf0-af4a25bc6531', 'CS Conjunto São José', '', 'cs-conjunto-sao-jose')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('d3b2895e-97be-5bac-a5c7-d95410b1a713', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '111db73a-4884-5528-8cf0-af4a25bc6531', 'CS N.S. do Amparo', '', 'cs-n-s-do-amparo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('ce6f8f11-8ff7-5cab-a27e-96d475e5b46f', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '111db73a-4884-5528-8cf0-af4a25bc6531', 'CS São Francisco', '', 'cs-sao-francisco')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1265f9fc-8ab0-5dda-96f9-b17a0ec06144', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Estratégia Saúde da Família', 'ESF', 'estrategia-saude-da-familia')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('7fec846b-0f4c-538d-92ae-9cf393679a86', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Alfredo de Castro I', '', 'esf-alfredo-de-castro-i')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('2178ca3c-1f27-572e-9341-7f5536a0e751', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Alfredo de Castro II', '', 'esf-alfredo-de-castro-ii')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1ae328f4-ddd7-5d9e-89ee-38cce8fd1ead', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF André Maggi', '', 'esf-andre-maggi')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('ba136561-9986-5f7b-8ed6-9da1bfe30566', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Assunção', '', 'esf-assuncao')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('769260f3-edc5-55d4-9ade-743b97285611', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Belo Horizonte', '', 'esf-belo-horizonte')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('38f2197b-9223-5547-81fd-f3faf1abe77e', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Bispo Pedro Casaldaliga', '', 'esf-bispo-pedro-casaldaliga')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5845a945-9595-5a44-b90a-5d8d56e429d6', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Bom Pastor', '', 'esf-bom-pastor')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('9c1d1ec5-b726-5da8-83a7-95e6c63a9e79', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF CAIC', '', 'esf-caic')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('70d5750a-7d78-5d0a-89ac-54bd21553791', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Canaã', '', 'esf-canaa')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('02efb58d-9dc7-5850-a5b2-18b5b6a8b074', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Cidade Alta', '', 'esf-cidade-alta')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('49dc6dbd-2133-5873-acc4-e107205eba0c', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Cidade de Deus', '', 'esf-cidade-de-deus')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('1a11073e-203c-5f4a-8457-886b1da1e48e', '49dc6dbd-2133-5873-acc4-e107205eba0c', '3º Turno', '3-turno')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('2a6c183b-50f4-5a97-9b72-51b379993609', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Conjunto São José I', '', 'esf-conjunto-sao-jose-i')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c7fd1358-b828-56d3-8931-3e88c50eea42', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Conjunto São José II', '', 'esf-conjunto-sao-jose-ii')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('2283e518-8e78-564f-b851-596b07a9f6d1', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Dom Osório', '', 'esf-dom-osorio')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('491b4bdd-ae34-53bd-9e95-2f86ebc05d51', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Itamaraty', '', 'esf-itamaraty')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('3a1d38f1-a1b4-5e30-aa84-3123487eb499', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jambrapi', '', 'esf-jambrapi')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('3aebcd07-634f-5488-bf52-3f01f9368e6c', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Adriana', '', 'esf-jardim-adriana')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('db28ec72-5efc-5b6d-9bc3-c636fbc8c6c3', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Atlântico', '', 'esf-jardim-atlantico')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('217ffa84-e051-5b6c-b315-3616eac964b4', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Europa', '', 'esf-jardim-europa')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8fcc4605-a01e-5d3a-8999-8a235d609fda', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Iguaçu', '', 'esf-jardim-iguacu')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c88a361b-856b-58a4-ab6b-10cfc4a0c56a', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Ipiranga', '', 'esf-jardim-ipiranga')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8c8a5b34-2260-567a-8e9f-0c0131da4828', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Itapuã', '', 'esf-jardim-itapua')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b3d57726-740b-56ae-a5c1-cb6dea099da8', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Liberdade', '', 'esf-jardim-liberdade')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('9300e284-21be-5a9d-9d6b-9510e68a9ec4', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Luz D''Yara', '', 'esf-jardim-luz-d-yara')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('18aedb39-846b-5467-a63e-53036c39f58a', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Morumbi', '', 'esf-jardim-morumbi')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('99f22cce-800a-5d2c-8a39-1e6b9d0732de', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Paineiras', '', 'esf-jardim-paineiras')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b50ed0dd-1dfc-5dfc-a998-de368d0c1c23', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Parque Industrial', '', 'esf-jardim-parque-industrial')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('3ef29cad-526c-514a-8dd5-734c1c133ff9', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Primavera', '', 'esf-jardim-primavera')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('10bd69aa-8f54-5c43-b2cd-463894d1a5b8', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Jardim Ypê', '', 'esf-jardim-ype')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a9a6e091-d6f3-53be-bd82-0b98e6ca082e', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF João de Barro', '', 'esf-joao-de-barro')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('837cb683-2081-5650-bc86-fc6545eae504', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF MAMED', '', 'esf-mamed')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5f689d7f-387c-53e7-9f48-1f2fc45893ad', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Marechal Rondon', '', 'esf-marechal-rondon')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f5107e4f-9a2b-5a0c-8127-7abc861301d1', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Margarida', '', 'esf-margarida')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c978ddd9-9a3d-5afb-9586-562dce4b2507', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Maria Amélia', '', 'esf-maria-amelia')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b4fe798e-05a6-50ab-b5b0-53578e131cfe', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Mathias Neves I', '', 'esf-mathias-neves-i')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1ad363a4-4fa5-56d8-9e39-1cede34a80fe', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Mathias Neves II', '', 'esf-mathias-neves-ii')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('fe5fb8a2-b6ad-59b7-b888-683634710ed1', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Monte Libano', '', 'esf-monte-libano')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5f039d47-baeb-5080-a7fe-da5f77571d6d', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Padre Miguel', '', 'esf-padre-miguel')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('d5056707-a23c-53d3-b52c-0440fcaa8519', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Padre Rodolfo', '', 'esf-padre-rodolfo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('80b9cc9c-5825-5d38-a143-c1ff5bda3550', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Parque das Rosas', '', 'esf-parque-das-rosas')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0f3acc13-e2b8-5458-8d93-0e9e19163792', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Parque São Jorge', '', 'esf-parque-sao-jorge')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('70f66684-4fe9-56d5-b7ad-cf4897643312', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Parque Universitário', '', 'esf-parque-universitario')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('52c86fc5-12df-5335-904e-6846810904e0', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Paulista', '', 'esf-paulista')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('35b3cbda-8e6b-5922-a34a-eb2b55ab0111', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Pedra 90', '', 'esf-pedra-90')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('abd63bf8-c13e-5eba-969f-e9d81ff4f6c3', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Pindorama', '', 'esf-pindorama')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('6db8fe94-be08-500c-bf5c-6b9b1c1f7b33', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Santa Clara', '', 'esf-santa-clara')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('d361aca4-ff9d-5f29-a618-90fe642a7457', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Serra Dourada', '', 'esf-serra-dourada')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8ac30dc8-a705-52d1-bb9e-6e05e524a152', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Sumaré', '', 'esf-sumare')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5d3b4f59-eb7b-5fc9-bc5e-d9fcc81d5dc7', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Verde Teto', '', 'esf-verde-teto')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c5601de0-fd73-55f0-8291-d1f583a9e581', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Vila Cardoso', '', 'esf-vila-cardoso')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('b2233431-d069-5c90-8d56-695a7d44ccae', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Vila Goulart', '', 'esf-vila-goulart')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0edfeda4-3635-5349-9d8c-c93b47af4b37', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Vila Mineira', '', 'esf-vila-mineira')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('95b0f400-8f2d-5eb5-8e02-6012d790860f', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Vila Olinda', '', 'esf-vila-olinda')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('a83ae368-eeb9-5f76-ae19-7cd74471354f', '95b0f400-8f2d-5eb5-8e02-6012d790860f', '3º Turno', '3-turno')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('914d57c0-346f-5c3b-8b3f-9ea3d113a240', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Vila Operária', '', 'esf-vila-operaria')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('2c597508-ae40-50ca-ac30-d94e19fd89e3', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Vila Rica', '', 'esf-vila-rica')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('6418d5bc-79ba-5ec8-9122-eabad8083c3b', '2c597508-ae40-50ca-ac30-d94e19fd89e3', '3º Turno', '3-turno')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f687ad0e-79a5-5c7e-9eb0-312d2757aa48', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '1265f9fc-8ab0-5dda-96f9-b17a0ec06144', 'ESF Vila Verde', '', 'esf-vila-verde')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('01941e66-20ff-5249-bfd3-f458aa6e825a', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Especialidades', '', 'especialidades')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('9e23f37e-03db-56a6-9e86-ca47ee2f8c1e', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'Abrigo Municipal de Animais', '', 'abrigo-municipal-de-animais')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c3ca1c59-a725-5228-af3f-1dfb7070b765', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'Ambulatório de Saúde Mental', '', 'ambulatorio-de-saude-mental')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('6a45d18f-3684-57a7-aa98-6b14a1dd062c', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'Ambulatório VIVA', '', 'ambulatorio-viva')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('9116e9ec-8acb-5cd0-ab3e-547cabd72523', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'CAISM', '', 'caism')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('10cdd764-43d3-5bf9-9143-7c84dd01c9fc', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'CAPS AD', '', 'caps-ad')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('285ab451-1006-5301-a9d2-8590e18b2c1c', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'CAPS Infantil', '', 'caps-infantil')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('6b4a1060-9498-58e3-b4ae-7cd5ace936bc', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'CEADAS', '', 'ceadas')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c3ca8cf2-1144-516b-aff7-edf17f6d1193', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'Cedero', '', 'cedero')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5c4315cb-8f7d-5836-9af9-6de041081f28', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'CERARO', '', 'ceraro')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0032efe4-9a03-5a1b-afc3-b689c6f3fce9', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'Nefrologia', '', 'nefrologia')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f4a8ea7e-62dd-5bed-ad92-beaffe46df63', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'Nilmo Junior', '', 'nilmo-junior')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a35f9678-531c-5885-97a6-89c8d5eb8e19', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'SAE', '', 'sae')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('afb27181-a3f6-5fb1-acd0-846157e607da', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '01941e66-20ff-5249-bfd3-f458aa6e825a', 'UVZ', '', 'uvz')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('dec56165-f39b-5c53-89ae-77a30726e9a0', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Farmácias', '', 'farmacias')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0a05c7f3-4afa-5a84-a4ad-8df5f0ddf157', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', 'dec56165-f39b-5c53-89ae-77a30726e9a0', 'Farmácia Administração', '', 'farmacia-administracao')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('481ca52c-8dae-5b02-86c0-2da9903f8ee0', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', 'dec56165-f39b-5c53-89ae-77a30726e9a0', 'Farmácia Alto Custo', '', 'farmacia-alto-custo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8ee39195-431f-5f6c-94c7-332d3adc6000', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', 'dec56165-f39b-5c53-89ae-77a30726e9a0', 'Farmácia Central', '', 'farmacia-central')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('af90f30f-156a-53ee-a25f-aee3a8bf6c29', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', 'dec56165-f39b-5c53-89ae-77a30726e9a0', 'Farmácia Judicial', '', 'farmacia-judicial')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('65cbed6b-7f60-50c8-aaf8-b89b856c0a55', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Hospitais e Pronto Atendimento', '', 'hospitais-e-pronto-atendimento')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c2900743-0ec1-59ae-aac6-4f883de67f72', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '65cbed6b-7f60-50c8-aaf8-b89b856c0a55', 'Hospital Cristyan Mary (lions)', '', 'hospital-cristyan-mary-lions')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8d6ef1b9-8067-5934-a174-a64bccdd281c', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '65cbed6b-7f60-50c8-aaf8-b89b856c0a55', 'Hospital Municipal Antônio Muniz', '', 'hospital-municipal-antonio-muniz')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5e6f1833-24f0-5674-b9b8-e27518831b51', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '65cbed6b-7f60-50c8-aaf8-b89b856c0a55', 'Hospital Retaguarda - UTI', '', 'hospital-retaguarda-uti')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('579a587c-29b5-567c-9ce9-21e4b563b0d3', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '65cbed6b-7f60-50c8-aaf8-b89b856c0a55', 'PA Infantil', '', 'pa-infantil')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a280f690-5398-5dac-ba25-12f3b571ba9f', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '65cbed6b-7f60-50c8-aaf8-b89b856c0a55', 'UPA', '', 'upa')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('218553e2-61c3-5182-842f-749cf8551a00', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Policlínicas', '', 'policlinicas')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('83d80c46-4c0d-5eb1-bd63-3205a650d109', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '218553e2-61c3-5182-842f-749cf8551a00', 'Policlínica Central', '', 'policlinica-central')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('118bb869-a798-5c89-9fd3-5400a95df507', '83d80c46-4c0d-5eb1-bd63-3205a650d109', '3º Turno', '3-turno')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('56c20dd8-73ad-5f2d-99a8-08dddd2b1d1d', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '218553e2-61c3-5182-842f-749cf8551a00', 'Policlínica Itamaraty', '', 'policlinica-itamaraty')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('d417f2cf-8abf-5c42-a3d6-058c37f0b7d3', '56c20dd8-73ad-5f2d-99a8-08dddd2b1d1d', '3º Turno', '3-turno')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('891dca97-cf3e-577f-b42a-95c611f8dd47', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', NULL, 'Zona Rural', '', 'zona-rural')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('8da1146c-1d79-5be2-a7f8-aadeca11fac4', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '891dca97-cf3e-577f-b42a-95c611f8dd47', 'ESF Boa Vista', '', 'esf-boa-vista')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5b602afc-b17c-572e-9a34-0b4e3581a758', '9a74ba6a-74e1-51eb-be89-4ab4fd59af84', '891dca97-cf3e-577f-b42a-95c611f8dd47', 'ESF Nova Galiléia', '', 'esf-nova-galileia')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

-- Secretaria Municipal de Promoção e Assistência Social
INSERT INTO entidades (id, nome, sigla, slug) VALUES ('57a95844-3d87-5f0a-95ef-34349a9d7474', 'Secretaria Municipal de Promoção e Assistência Social', 'SEMPRAS', 'sempras')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('3c1286a9-ef0e-5418-b029-fc7d77b1be17', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'Sede da SEMPRAS', 'SEMPRAS', 'sede-da-sempras')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('78855ca1-2688-56f2-bf09-57d3da0c29b7', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Administrativo', 'administrativo')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('e2973d79-7ef6-58e0-9bad-7e24cce094f7', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Cadastro Único', 'cadastro-unico')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('04d482ed-c757-5d81-a153-1c32c01ca314', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Compras', 'compras')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('270bccf9-8c08-56a7-8485-51fe7060d1b7', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Engenharia', 'engenharia')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('2ac5160a-57a9-588a-a8c4-348103c096a1', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Financeiro', 'financeiro')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('eabd7556-b408-5806-9083-d566232599f9', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Jurídico', 'juridico')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('a65ce0da-9fcc-53f0-a7eb-987318542b21', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Núcleo de Conselhos', 'nucleo-de-conselhos')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('f18ae9f1-74be-5ebb-9811-e671f6e62ce7', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Proteção Social Básica', 'protecao-social-basica')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES ('85f80cc2-6bac-58cd-b843-64a23493ffd3', '3c1286a9-ef0e-5418-b029-fc7d77b1be17', 'Proteção Social Especial', 'protecao-social-especial')
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('e0922b5d-a26b-574c-add0-19bdd6634de1', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'Casa Abrigo', '', 'casa-abrigo')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('29aea463-8905-55e3-9b66-fe99139daa6e', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'Casa da Mulher', '', 'casa-da-mulher')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('bbb21012-14ff-52e9-a247-57cf192c8939', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'Centro POP', '', 'centro-pop')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('77b5598f-d57c-5f6b-9a3b-ee7e2e70b274', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CEU', '', 'ceu')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4c230512-83c3-5662-8438-8052d42ab419', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'Conselho Tutelar Central', '', 'conselho-tutelar-central')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('24bc0f79-ce38-5e20-8643-4590f94e3639', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'Conselho Tutelar Vila Operária', '', 'conselho-tutelar-vila-operaria')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0d57a5cb-ec89-5853-b19f-e5c69fdfde69', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Alfredo de Castro', '', 'cras-alfredo-de-castro')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5ae7ab4f-d688-5e91-a9cc-83b6f9f96887', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Ana Carla', '', 'cras-ana-carla')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('a1eb429d-5735-590d-9471-b6e535d7c84f', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Central', '', 'cras-central')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('0fa8c1a9-d1d3-58ce-aa2c-b5275be2c8c3', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Conjunto São José', '', 'cras-conjunto-sao-jose')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('1d5e8e7d-28c3-5546-84b7-10af8cad2212', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Iguaçu', '', 'cras-iguacu')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('c154eaeb-f0ee-5b74-8808-c90419afa0ef', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Luz Dyara', '', 'cras-luz-dyara')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('cba8fa0e-208c-54d2-a9f3-34b03900f527', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Padre Lothar', '', 'cras-padre-lothar')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('7b1f1203-3eb4-5ab7-a9ad-a2b1f0eea1cf', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Rio Vermelho', '', 'cras-rio-vermelho')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('f1dfb434-5ee6-5d6d-bbdc-6ba5644a86de', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CRAS Sagrada Família', '', 'cras-sagrada-familia')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('4736ed15-a205-5557-a88b-4f65324528ec', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'CREAS', '', 'creas')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('5ec31521-7938-5e0f-b04d-366894c462f8', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'SCFV Padre Lothar', '', 'scfv-padre-lothar')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;
INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES ('902b0ad1-38a5-5b97-9727-8e196984346c', '57a95844-3d87-5f0a-95ef-34349a9d7474', NULL, 'Vila Olímpica', '', 'vila-olimpica')
  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;

COMMIT;
