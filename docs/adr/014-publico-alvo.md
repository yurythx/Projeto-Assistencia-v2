# 014 — Público-alvo: publicação por secretaria

- **Status:** Aceito
- **Data:** 2026-09-27
- **Relacionado:** [ADR 013](013-permissao-com-escopo-e-heranca.md) (o escopo restringe a gestão; este ADR restringe a **leitura**)

## Contexto

O ADR 013 decidiu que a leitura continuaria aberta: um post, uma página da
Wiki ou um evento interno eram vistos por todo autenticado. Numa prefeitura
com várias secretarias, boa parte do conteúdo interno interessa a uma
secretaria só (o comunicado do RH da SEMSA, o manual das escolas da SEMED,
a reunião dos CRAS). Publicar para todos polui a leitura de todos e expõe o
que não precisa ser exposto.

## Decisão

Blog, Wiki e Agenda ganham um **público-alvo** opcional: uma ou mais
**entidades** (secretarias) e/ou **unidades**.

- **Vazio = todos**, como antes. Nada que já existe muda.
- **Quem pertence ao público:** quem tem lotação (manual ou por grupo do AD,
  de qualquer perfil) numa entidade do público, ou numa unidade do público
  **ou abaixo dela**. A lotação num departamento conta na unidade dele.
  Concessão só global não é pertencimento.
- **Quem mais vê:** o autor (organizador, no evento) e a gestão que cobre o
  conteúdo — `blog:manage` na unidade dona do post, `wiki:manage` na dona
  da página, `calendar:manage` na dona da sala do evento — e a gestão
  global.
- **Nunca público:** conteúdo com público-alvo não aparece no site público
  nem para anônimos; um evento com público não pode ser "público".
- **Wiki herda:** a página sem público próprio vale o da página-mãe mais
  próxima que tem um. Subpáginas de uma página restrita são restritas.
- **Busca Global** respeita a mesma regra.
- **Avisos em tempo real** (publicação de post, criação de evento) de
  conteúdo com público não são difundidos a todos: o payload marca
  `restrito` e a difusão geral o descarta.

## Consequências

- Uma tabela de público por módulo (`blog_post_publico`,
  `wiki_page_publico`, `calendar_event_publico`), com chave estrangeira
  para entidade/unidade em **RESTRICT**: excluir uma secretaria ou unidade
  que é público de algum conteúdo é recusado (senão o conteúdo ficaria
  visível a todos em silêncio).
- O resolvedor do IAM calcula, junto com as concessões, as unidades a que
  a pessoa pertence (a da lotação e as de cima — `nexus_unidades_acima`),
  no cache da identidade: checar o público não consulta o banco.
- Público-alvo **não é sigilo**: serve para direcionar a leitura, não para
  proteger informação classificada (que tem regra própria no Trâmite).
