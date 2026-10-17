# 032 — SEMPRAS: Casa Abrigo com paridade ao sistema da unidade

- **Status:** Aceito e implementado
- **Data:** 2026-10-17
- **Complementa:** [ADR 031](031-sempras-atendimento-por-tipo-de-unidade.md) (a Casa Abrigo é um dos tipos de unidade)
- **Referência:** `casa_abrigo/` na raiz (sistema "Casa Abrigo 4.0" que a unidade usa hoje: `server.js`, `public/index.html`; fora do git)

## Contexto

A Casa Abrigo já tinha, no SEMPRAS, o acolhimento institucional com
irmãos, diagnósticos, visitantes, termos, PIA e documentos (migrations
153 a 155). Essa primeira versão partiu de um legado mais antigo. O
sistema que a unidade usa hoje (4.0) foi além. A comparação tela a tela
encontrou o seguinte:

| Área | O 4.0 tem | Nós tínhamos |
|---|---|---|
| Cadastro | Assistente em 6 etapas: criança (situação especial, PCD, apelido, identidade de gênero, orientação sexual, raça/cor, filiação), responsável (não possui, CPF, nascimento, sexo, vínculo em lista, endereço por CEP, celular, e-mail), acolhimento (data na unidade, 2 processos, guia e data, histórico com "não sei", 18 motivos e "outro"), jurídico (10 situações, data, CNA, CNCAD), saúde (laudo, medicação, dependência química, checklist de doenças, deficiências e transtornos), social (estuda, ameaça de morte, visita, benefício, programa social, trabalho) e desligamento (guia e datas) | Formulário único com 9 motivos, responsável com 5 campos, situação jurídica em texto livre, desligamento só com motivo |
| PIA | 6 abas: identificação e documentos, saúde e escola, situação familiar, plano de ação (com esportes, cultura e lazer), rotina (com observações de comportamento), intervenção familiar (objetivos, estratégias, metas) | Ficha com 14 campos e metas sem esportes/cultura/lazer |
| Evolução técnica | Tela própria com todos os acolhidos e busca por nome | Só dentro da ficha de cada munícipe |
| Termos | Documento oficial impresso com o cabeçalho da Prefeitura e assinatura feita na tela | Só o registro |
| Visitantes | Visita ao acolhido, à instituição, doação e voluntariado, com entrada automática | Só visita a um acolhido |
| Diário | Calendário com anotações por dia (geral ou de um acolhido) | Não tinha |
| Relatórios | Institucionalizados (lista, totais por faixa, impressão, CSV), estatístico mensal/trimestral/anual (reintegrações, adoções, transferências, motivos, entradas e saídas) e relatórios escritos com prazo, etapa e editor | Não tinha |
| Kanban | Tarefas da equipe em 5 colunas | Não tinha |
| Monitor | Avisos de prazo do PIA, de relatórios e de tarefas | Não tinha |

O 4.0 também tem fragilidades que não vamos copiar:

- O motivo do acolhimento e o do desligamento são texto livre, e o
  relatório conta reintegrações e adoções procurando palavras nesse
  texto.
- O "PIA vencendo" dispara para todo acolhido com 80 dias ou mais, para
  sempre, porque não sabe quando o PIA foi reavaliado.
- O administrador pode excluir acolhido, apagar evolução e limpar o
  diário inteiro.
- O PIA de saúde e família é gravado com "N/I" fixo, sem ler a tela.

## Decisão

Decisões tomadas com a gestão em 2026-10-17: manter o histórico (nada de
excluir acolhido, evolução ou diário), trazer o Kanban para a unidade e
organizar as telas em abas no espaço da unidade.

### 1. Cadastro completo, com listas fechadas

O acolhimento ganha os campos do assistente do 4.0. Tudo o que lá é
`<select>` vira enum checado no banco e no domínio:

- 19 motivos de acolhimento, mais o detalhe obrigatório de "outro";
- 10 situações jurídicas, com a data;
- 16 vínculos do responsável;
- relação de trabalho, identidade de gênero, orientação sexual e
  raça/cor;
- perguntas de sim, não e "não sei" (histórico, CNA, CNCAD) e de sim e
  não (saúde, contexto social).

Os dados de identificação da criança (nome, nome social, CPF, NIS,
nascimento, sexo) continuam no cadastro único do munícipe. O responsável
ganha CPF validado, CEP de 8 dígitos e UF.

Regras:

- **R1.** A data de entrada não pode ser futura nem anterior ao
  nascimento.
- **R2.** O acolhido precisa ter data de nascimento e ser menor de 18
  anos na entrada (ECA art. 101, VII). A faixa é de 0 a 11 anos para
  criança e de 12 a 17 para adolescente (ECA art. 2º).
- **R3.** Uma criança tem no máximo um acolhimento ativo, em qualquer
  unidade (índice único parcial).
- **R4.** "Não possui responsável" limpa os campos do responsável.
  Benefício, laudo e transtorno só guardam a descrição quando a resposta
  é "sim".
- **R5.** O desligamento (só pela ação própria, como antes) exige:
  - o tipo: reintegração à família natural, reintegração à família
    extensa, adoção, guarda, transferência, maioridade, evasão ou outro;
  - o motivo;
  - a data da saída, que não pode ser futura nem anterior à entrada.

  A guia de desligamento e a data da guia são opcionais. As estatísticas
  contam pelo tipo, não pelo texto.
- **R6.** Irmãos e diagnósticos marcados no assistente são gravados na
  mesma transação do cadastro. O checklist de saúde do 4.0 vira
  diagnósticos com nome padronizado.

### 2. PIA com as 6 abas

A ficha do PIA ganha os campos do 4.0:

- naturalidade, nacionalidade e religião;
- documentos que a criança tem;
- sinais de violência, tratamento odontológico e médico/psicológico;
- deficiências e restrições;
- período, frequência e reforço escolar;
- benefícios da família e moradia anterior;
- observações da família e observações de comportamento;
- objetivos, estratégias e metas da intervenção familiar.

As metas do plano de ação ganham esportes, cultura e lazer, e podem ser
editadas.

- **R7.** O PIA é reavaliado no máximo a cada 3 meses (ECA art. 19,
  §1º). A próxima reavaliação é a última gravação do PIA mais 90 dias.
  Sem PIA gravado, conta a data de entrada mais 90 dias. O monitor avisa
  10 dias antes e aponta o atraso.

### 3. Telas da unidade

O espaço `/sempras/u/<Casa Abrigo>` passa a ter abas que espelham o menu
do 4.0:

- **Painel:** os 5 cartões e o monitor de prazos.
- **Acolhidos:** relatório de institucionalizados e assistente de
  cadastro e edição.
- **PIA**.
- **Evolução:** todos os acolhidos, com busca.
- **Termos:** documento impresso e assinatura na tela. A assinatura não
  é guardada, só impressa.
- **Visitantes**.
- **Diário:** calendário e anotações.
- **Relatórios:** estatísticos e escritos.
- **Tarefas:** Kanban.

A ficha do munícipe continua mostrando o acolhimento.

### 4. Novos registros da unidade

- **Visitas:** passam a ser da unidade, com o acolhido opcional. Os
  tipos são acolhido, instituição, doação e voluntariado. A visita a um
  acolhido exige o acolhido. A saída é registrada depois.
- **Diário de bordo** (`sempras_casa_abrigo_diario`): anotação de um dia
  (pode ser futura, como agenda), geral ou de um acolhido. Só de
  inclusão.
- **Relatórios escritos** (`sempras_casa_abrigo_relatorios`):
  - têm tipo da lista do 4.0 (ou "outro" com nome), etapa (em
    elaboração, revisão, aprovado, entregue), prazo, conteúdo e,
    opcionalmente, o acolhido;
  - **R8.** O entregue não muda mais e não pode ser excluído: é o que
    foi mandado ao Judiciário.
  - O monitor avisa 7 dias antes do prazo.
- **Tarefas** (`sempras_casa_abrigo_tarefas`):
  - Kanban com backlog, a fazer, em andamento, revisão e concluído;
  - prioridade baixa, média, alta ou crítica;
  - responsável, sprint, prazo e ordem;
  - o monitor avisa 3 dias antes do prazo;
  - podem ser excluídas.

### 5. O que não muda

- **Exclusões:**
  - O acolhido é desligado, nunca apagado.
  - A evolução é do prontuário, que é só de inclusão (ADR 030 §5), e
    segue o sigilo psicológico.
  - O diário é só de inclusão.
  - As ações do sistema, que o 4.0 registra no diário, já estão na
    trilha de auditoria da plataforma.
- **Login e criptografia:** o login (AD/Keycloak), a sessão e o modo
  escuro são da plataforma. O 4.0 criptografa campos com uma chave no
  `.env` do servidor da aplicação. Aqui a proteção é a do banco e do
  backup (RIPD do módulo), sem chave junto da aplicação.
- **Leituras auditadas:** listas com nome de criança (painel, evoluções,
  visitas, termos, relatórios) gravam a leitura na auditoria, como a
  ficha.
