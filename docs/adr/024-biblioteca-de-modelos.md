# 024 — Biblioteca de modelos de documento do Atlas

- **Status:** Aceito e implementado
- **Data:** 2026-10-04
- **Relacionado:** [ADR 018](018-atlas-paginas-e-busca-unificada.md) (páginas do Atlas), [ADR 019](019-vigencia-da-ttdd-e-versoes.md) (versões de procedimento), [ADR 023](023-assistente-fluxos-e-ttdd.md) (assistente sobre os fluxos)

## Contexto

Cada etapa de um procedimento lista as **peças exigidas** (DFD, termo de
referência, ofício…). Até aqui a peça só aceitava um link externo opcional
para a minuta (`modelo_minuta_padrao_url`). Esse link quebra quando o site
de origem muda, ninguém sabe qual versão foi publicada, e a mesma minuta
(um ofício, um requerimento) é usada por vários fluxos: atualizar o arquivo
significava procurar e editar procedimento por procedimento.

## Decisão

1. **Catálogo reutilizável** (`atlas_modelos`): um modelo tem nome (único,
   sem diferenciar maiúsculas), descrição e situação (ativo/desativado). As
   peças **apontam** para um modelo (`atlas_etapa_documentos.modelo_id`), e
   não guardam uma cópia: publicar uma nova versão do modelo vale para todos
   os fluxos que o usam.
2. **Versões imutáveis** (`atlas_modelo_versoes`): cada versão guarda o nome
   do arquivo, o tipo, o tamanho, o SHA-256, a nota ("o que mudou"), quem
   publicou e quando. A "versão atual" é a de maior número; as anteriores
   continuam baixáveis (`?versao=N`). Nada é apagado: **desativar** tira o
   modelo da escolha de novas peças, mas as peças já ligadas continuam
   baixando, e a nova versão de um procedimento que já o usava continua
   possível.
3. **Arquivo no armazenamento de objetos** (MinIO, bucket da aplicação,
   chave `atlas/modelos/{modelo}/{uuid}{ext}`). O arquivo vai para o
   armazenamento **antes** da transação que registra a versão; se ela
   falha, o objeto é apagado (sem órfão no bucket).
4. **Upload e download pela API**, na mesma origem do sistema (multipart na
   ida, stream na volta, com `Content-Disposition` RFC 2231 para nomes com
   acento, `X-Content-Type-Options: nosniff` e `ETag` = SHA-256). Não há URL
   pré-assinada do MinIO: o navegador não precisa alcançar o MinIO nem
   confiar no certificado dele.
5. **Arquivo conferido pelo conteúdo:** .docx, .odt, .pdf, .doc, .rtf, .xlsx
   e .ods, até 10 MB; a assinatura dos bytes iniciais tem de corresponder à
   extensão (um executável renomeado para .pdf é recusado).
6. **Acesso:** consulta e download públicos, como o resto do Atlas
   (`GET /atlas/modelos`, `/atlas/modelos/{id}`, `/atlas/modelos/{id}/arquivo`),
   sem a autoria nas respostas públicas (minimização, LGPD art. 6º III);
   gestão com `atlas:manage` (`/atlas/admin/modelos`: listar com os
   desativados, cadastrar, publicar versão, alterar). Cadastro, versão e
   alteração entram na trilha de auditoria (`atlas.modelo.criado`,
   `atlas.modelo.versao`, `atlas.modelo.alterado`).
7. **Interface:** página **Atlas → Modelos de documento** (busca, baixar,
   histórico de versões; para a gestão, novo modelo, nova versão, editar e
   desativar); no procedimento, a peça ligada mostra **Baixar modelo**
   (versão atual) e cada peça tem **Detalhes**: o modelo dela para baixar
   e, para a gestão, **ligar/trocar o modelo** ou **enviar o arquivo** ali
   mesmo (nova versão do modelo ligado, ou um modelo novo com o nome da
   peça) — direto na versão em vigor, sem publicar nova versão do
   procedimento: o modelo é material de apoio, não muda o fluxo
   (`PUT /atlas/admin/workflows/{id}/pecas/{peca}/modelo`, auditado como
   `atlas.workflow.modelo_peca`); no cadastro e na nova versão do procedimento, a seção
   **Modelos das peças** liga cada peça a um modelo — a peça com o mesmo
   nome de um modelo ativo já vem ligada.
8. **Modelos da série da TTDD** (`atlas_ttdd_modelos`): a página de cada
   série documental tem a seção **Modelos de documento**, com os modelos
   daquele tipo de documento para baixar (um modelo serve a várias séries
   e uma série a vários modelos). A gestão liga um modelo ativo da
   biblioteca ou envia um arquivo, que vira um modelo novo (nome sugerido:
   o descritor da série) já ligado; retirar desfaz só a ligação. Série
   revogada não recebe modelos novos. **A peça sem modelo próprio oferece
   os modelos ativos da série do procedimento** (no checklist e nos
   Detalhes): quem acompanha o fluxo encontra o modelo onde quer que a
   gestão o tenha anexado; o modelo próprio da peça tem precedência. `GET /atlas/ttdd/{codigo}/modelos`
   (público, sem autoria); `POST /atlas/admin/ttdd/{codigo}/modelos` e
   `DELETE …/modelos/{modelo}`, auditados (`atlas.ttdd.modelo_ligado`,
   `atlas.ttdd.modelo_desligado`).
9. **Assistente:** a síntese do fluxo (ADR 023) cita o modelo da peça na
   biblioteca ("modelo na biblioteca do Atlas: Requerimento (versão 2)").

## Consequências

- O link externo (`modelo_minuta_padrao_url`) continua aceito para os
  procedimentos já cadastrados; o caminho recomendado passa a ser a
  biblioteca.
- O backup do MinIO passa a incluir os modelos (mesmo bucket da aplicação).
- Modelo não é apagado pela aplicação (`ON DELETE RESTRICT`); a limpeza de
  versões antigas, se um dia for necessária, é decisão de gestão documental.
