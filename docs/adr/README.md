# Registros de Decisão de Arquitetura (ADR)

Toda mudança de contrato de API, RBAC, auditoria ou middleware exige um ADR
(ver checklist no template de PR).

| ADR | Decisão | Status |
|---|---|---|
| [002](002-enterprise-resilience-and-governance.md) | Resiliência: idempotência, circuit breaker, DLQ | Aceito (contexto histórico: parte dos módulos citados foi substituída pelo Microkernel) |
| [003](003-local-auth-rsa-hardening.md) | Login local RS256 e bloqueio de conta | Aceito (o bloqueio vigente é o progressivo no Redis — ver 011) |
| [004](004-outbox-listen-notify-e-rbac-modular.md) | Outbox por LISTEN/NOTIFY e RBAC por módulo | Aceito (o exemplo `demands` é histórico; o RBAC modular vive hoje nos Manifests do Kernel) |
| [005](005-roadmap-conformidade-governamental.md) | Roadmap de conformidade SGD/MGI | Em execução |
| [006](006-rfc7807-problem-details.md) | Erros RFC 7807 por negociação de conteúdo | Aceito |
| [007](007-excecao-csp-style-src-vlibras.md) | Exceção de CSP `style-src` (VLibras) | Aceito, revisão trimestral |
| [008](008-kernel-grafo-de-modulos-e-estrategia-de-testes.md) | Grafo de módulos, invariantes do IAM, proveniência da auditoria, LGPD federado e estratégia de testes | Aceito |
| [009](009-revisao-dos-plugins-e-meta-de-cobertura.md) | Revisão das regras de negócio de cada plug-in e meta de 100% de cobertura por módulo no CI | Aceito |
| [010](010-revisao-da-plataforma-e-cobertura-por-pacote.md) | Revisão de cada pacote da plataforma e da composição; RabbitMQ e MinIO reais no CI; meta de 100% por pacote | Aceito |
| [011](011-borda-unica-ip-confiavel-e-busca-por-prefixo.md) | Proxy HTTPS como borda única, IP do visitante confiável ponta a ponta, bloqueio de login por IP com limite próprio, busca por prefixo, reautenticação do Signum | Aceito |
| [012](012-estrutura-desativada-nao-concede.md) | Estrutura desativada deixa de conceder perfis e de receber processos; unidade-mãe com subunidades não é excluída | Aceito |
| [013](013-permissao-com-escopo-e-heranca.md) | Permissão com escopo e herança: as permissões de gestão valem onde foram concedidas (e abaixo); unidade dona do conteúdo; administração delegada do IAM | Aceito e implementado |
| [014](014-publico-alvo.md) | Público-alvo: Blog, Wiki e Agenda publicados só para secretarias e/ou unidades | Aceito |
| [015](015-atlas-padronizado.md) | Atlas alinhado ao padrão dos plug-ins: sem tenant, busca full-text nativa (sem Typesense), rotas pública/assistente/gestão, grounding determinístico e IA opcional | Aceito e implementado |
| [016](016-camada-de-aplicacao-e-transacoes.md) | A camada de aplicação delimita a transação com pgx (escrita + outbox + auditoria); domínio nunca importa o driver | Aceito |
| [017](017-ttdd-oficial.md) | TTDD do Atlas fiel ao documento oficial: hierarquia órgão > função > subfunção > série, prazo por anos ou condição, destinação indefinida, fonte e recomendação; 1.686 séries extraídas do PDF; assistente responde sobre temporalidade | Aceito e implementado |
| [018](018-atlas-paginas-e-busca-unificada.md) | Atlas em páginas com URL própria (procedimento, consulta e série da TTDD), busca unificada, assistente em gaveta, árvore da TTDD, calculadora de temporalidade e exportação CSV | Aceito e implementado |
| [019](019-vigencia-da-ttdd-e-versoes.md) | Vigência da TTDD: série retirada fica revogada (não apagada), histórico de prazos por gatilho, carga com relatório de impacto (`make ttdd-impacto`); nova versão de procedimento que desativa as anteriores na mesma transação | Aceito e implementado |
| [020](020-conexoes-de-ia.md) | Conexões de IA pela tela (Configurações → Inteligência artificial): fornecedores compatíveis com a API da OpenAI, chave cifrada e só de escrita, teste antes de salvar, principal e reserva, autorização e mascaramento para fornecedor externo (LGPD); IA local como serviço `ia-local` | Aceito e implementado |
| [021](021-assistente-restrito-a-ttdd.md) | Assistente do Atlas restrito à TTDD: só séries da tabela oficial sustentam a resposta; qualquer outro assunto (inclusive procedimentos) recebe "foge do objetivo da IA", decidido antes do modelo e reforçado no prompt | Aceito e implementado |
| [022](022-atualizar-ttdd-pela-tela.md) | Atualizar a TTDD pela tela (gestão do Atlas): CSV da exportação revisado no Excel ou o ttdd.json do PDF; simulação com o impacto e os procedimentos afetados; aplicação só do arquivo conferido (hash), com histórico, revogação e auditoria | Aceito e implementado |
| [023](023-assistente-fluxos-e-ttdd.md) | Assistente sobre o acervo do Atlas: explica os fluxos homologados do início ao fim (etapas, setores, prazos, peças, condição para seguir, diligências) e a TTDD; o resto continua recusado | Aceito e implementado |
| [024](024-biblioteca-de-modelos.md) | Biblioteca de modelos de documento do Atlas: catálogo reutilizável com versões imutáveis no MinIO; as peças dos fluxos apontam para o modelo; upload e download pela API, arquivo conferido pelo conteúdo; desativar sem quebrar as peças ligadas | Aceito e implementado |
| [025](025-atlas-em-escala.md) | Atlas em escala: importação/exportação de procedimentos em lote (simular → aplicar, savepoint por item), modelos em lote, painel de cobertura, Busca Global com séries e modelos, assistente citando os modelos | Aceito e implementado |
| [026](026-tramite-e-atlas-publico.md) | Trâmite ligado ao Atlas (classificação do processo, checklist de peças com modelo, guarda documental) e Atlas público sem login (procedimentos e TTDD); visualizar o PDF do modelo | Aceito e implementado |
| [027](027-notificacoes.md) | Notificações persistentes da plataforma (caixa, leitura, opt-out por módulo, idempotência, retenção) e avisos de nova versão: quem segue o fluxo, as unidades do fluxo e os processos do Trâmite (por evento) | Aceito e implementado |
| [028](028-validacao-dos-procedimentos.md) | Validação dos procedimentos: rascunho → em validação → homologado (só o homologado é publicado, garantido no banco); importação como rascunho; registro das entrevistas por departamento; homologação que publica e avisa; 92 rascunhos em duas levas e plano de validação | Aceito e implementado |
| [029](029-organograma-da-ttdd.md) | Organograma da TTDD no Atlas: secretaria → função → subfunção com séries, procedimentos e lacunas; quatro visualizações para imprimir (caixas, tópicos, blocos e fichas); público sem rascunhos | Aceito e implementado |
