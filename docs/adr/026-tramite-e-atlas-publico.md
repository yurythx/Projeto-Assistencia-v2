# 026 — Trâmite ligado ao Atlas e Atlas público

- **Status:** Aceito e implementado
- **Data:** 2026-10-04
- **Relacionado:** [ADR 024](024-biblioteca-de-modelos.md) (modelos), [ADR 025](025-atlas-em-escala.md) (Atlas em escala), [ADR 019](019-vigencia-da-ttdd-e-versoes.md) (TTDD)

## Contexto

O Atlas descreve **como** cada processo tramita e **por quanto tempo**
guardar os documentos. O Trâmite é onde os processos andam de verdade. Os
dois não se falavam:

- O tipo de documento do Trâmite era um texto livre.
- O processo não sabia qual fluxo seguia nem qual série da TTDD define a
  guarda dele.
- O Atlas só era visível com login, embora a TTDD e os procedimentos sejam
  informação pública (LAI, transparência ativa).

## Decisão

1. **Classificação do processo** (migração 000139): `tramite_processos`
   ganha `atlas_procedimento_id` e `codigo_ttdd`.
   - São referências **por valor**, sem chave estrangeira: o Trâmite não
     depende do plugin Atlas, e os dois módulos continuam independentes no
     código. A composição acontece na tela.
   - A classificação é **opcional na abertura**: escolher o procedimento
     traz a série dele.
   - Pode ser **feita ou trocada depois**
     (`PUT /tramite/processos/{id}/classificacao`), também com o processo
     encerrado, porque é aí que a guarda conta. Exige poder movimentar o
     processo, e a troca é auditada com a classificação anterior
     (`tramite.processo.classificado`).
   - O código da série é conferido: o formato vale na API, e há um CHECK
     no banco.
2. **Cartão Atlas no processo:**
   - O procedimento seguido, com link.
   - O **checklist das peças exigidas**: uma peça está "no processo"
     quando algum documento tem o tipo ou o título igual ao nome dela, sem
     diferenciar acento e caixa. Documento cancelado não conta.
   - **"Modelo"** para baixar em cada peça: o modelo da própria peça ou o
     da série.
   - A **guarda**: os prazos da série e, para o processo encerrado, a
     situação. A contagem parte da data de encerramento.
3. **Novo documento:** o tipo sugere as peças do procedimento. Escolhida
   uma peça com modelo, o formulário oferece **"Baixar &lt;modelo&gt;"**.
4. **Guarda documental** (`/tramite/guarda`): os processos encerrados
   (concluídos e arquivados) aparecem agrupados por série, cada um com a
   situação:
   - na fase corrente ou no arquivo intermediário até tal data;
   - **pode ser eliminado, com a aprovação da CCPAD**;
   - **recolher ao arquivo permanente**;
   - destinação não definida.

   Os processos sem série aparecem numa lista à parte, para classificar.
   O cálculo é o mesmo da calculadora do Atlas. A listagem **orienta**, e
   a eliminação continua dependendo da CCPAD.
5. **Atlas público** (sem login, no site institucional):
   - **Páginas:**
     - `/procedimentos` e `/procedimentos/{id}`: etapas, documentos
       exigidos com o modelo para baixar e a guarda.
     - `/temporalidade` e `/temporalidade/{codigo}`: busca, filtro por
       secretaria, paginação, planilha CSV, prazos, fonte oficial, modelos
       e os procedimentos que produzem a série.
   - **Proxy público:** o proxy anônimo passa a liberar só a leitura do
     Atlas (`v1/atlas/ttdd`, `v1/atlas/workflows`, `v1/atlas/modelos`).
     A gestão e o assistente ficam de fora. Ele também repassa arquivos em
     bytes, com o nome (`Content-Disposition`): o CSV mantém o BOM e o
     modelo baixa íntegro.
   - **Menu e sitemap:** os links "Procedimentos" e "Temporalidade" só
     aparecem com o módulo ativo.
6. **Visualizar o PDF do modelo:**
   - `GET /atlas/modelos/{id}/arquivo?inline=1` abre **só PDF** no
     navegador; os outros formatos são sempre baixados.
   - Na biblioteca, os modelos em PDF têm o botão **"Visualizar"**.

## Consequências

- O processo passa a carregar a informação de guarda desde a abertura. A
  listagem de guarda substitui a planilha manual de quem administra o
  arquivo, mas não autoriza a eliminação.
- Desativar o plugin Atlas não quebra o Trâmite: o cartão e as sugestões
  simplesmente não aparecem.
- **Fica para depois:** avisar quem acompanha um fluxo quando sai uma nova
  versão do procedimento ou do modelo. Os eventos já são emitidos, mas
  falta definir **quem** recebe (assinatura do fluxo, unidade ou perfil)
  para não notificar todos os servidores.
