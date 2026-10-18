# 033 — SEMPRAS: Casa Abrigo — histórico do 4.0, avisos no sino e linha do tempo

- **Status:** Aceito e implementado
- **Data:** 2026-10-18
- **Complementa:** [ADR 032](032-sempras-casa-abrigo-paridade-legado.md) (as telas da Casa Abrigo) e [ADR 027](027-notificacoes.md) (o sino)

## Contexto

O ADR 032 trouxe as telas do Casa Abrigo 4.0 para o SEMPRAS. Para a
unidade deixar o 4.0, faltavam três coisas:

1. **O histórico.**
   - O 4.0 tem anos de acolhidos, evoluções, visitas, termos, P.I.A.,
     diário, relatórios e tarefas.
   - Começar do zero obrigaria a equipe a consultar dois sistemas.
   - Alguns campos estão cifrados com uma chave que fica no `.env` do
     servidor do 4.0.
   - O texto livre não segue as listas fechadas daqui.
2. **Os avisos fora da tela.** O monitor de prazos só aparece para quem
   abre o painel da unidade, e o 4.0 também só avisava assim.
3. **A história de cada criança.** Audiência concentrada e reavaliação
   pedem a trajetória completa do acolhido: entrada, evoluções, visitas,
   termos, P.I.A., relatórios e desligamento. Hoje ela está espalhada em
   oito abas.

## Decisão

### 1. Importação do histórico (`importar-casa-abrigo`)

O comando importa o histórico em duas etapas.

**Simulação (o padrão):**

- roda a importação inteira numa transação e a desfaz no fim;
- passa pelos mesmos construtores do domínio de um cadastro na tela, com as
  restrições do banco (R2, R3, R8);
- devolve um relatório com três partes:
  - o que entraria;
  - o que entra com ajuste a revisar;
  - o que fica de fora, e por quê.

**Aplicação (`-aplicar`):**

- grava tudo numa transação só;
- usa um savepoint por registro: o que o domínio recusa fica de fora sem
  desfazer o resto, e um erro do banco desfaz tudo.

As regras abaixo valem para as duas etapas.

- **I1. Nada duplica.** Cada registro importado deixa em
  `sempras_casa_abrigo_importacao` a origem (tabela e id no 4.0), o
  destino e quem importou. Rodar de novo só traz o que faltou, e se sabe de
  onde veio cada linha.
- **I2. Nada some.** Um rótulo fora das listas fica em branco no campo e
  vai, com o texto original, para as observações do acolhimento. RG e
  certidão também vão para as observações, porque não têm campo próprio.
  O motivo fora da lista vira "outro" com o texto como detalhe.
- **I3. Nada se inventa em silêncio.**
  - Fica marcado como "a revisar":
    - o tipo de desligamento, deduzido do texto do motivo;
    - o P.I.A. sem data, cuja reavaliação passa a contar da entrada, então
      o monitor avisa antes, nunca depois;
    - o munícipe que já tinha cadastro, achado pelo CPF ou NIS, ou pelo
      nome e nascimento. O cadastro daqui é mantido.
  - Um CPF, NIS, CEP, UF ou e-mail inválido do responsável não é
    importado, também com aviso.
- **I4. O que não fecha fica de fora**, com o id do 4.0 e o motivo:
  - acolhido sem nascimento ou sem entrada;
  - acolhido maior de idade na entrada, ou com um segundo acolhimento
    ativo;
  - desligado sem data da saída;
  - registro de um acolhido que não entrou.

  A equipe corrige no 4.0 e simula de novo.
- **I5. Sem nome de criança no relatório.** O relatório aponta só o id do
  4.0, porque pode ser impresso e circular.
- **I6. A chave do 4.0 só existe em memória.**
  - Ela vem do ambiente (`LEGADO_ENCRYPTION_KEY`), nunca de argumento,
    para não ficar no histórico do shell.
  - Uma chave errada para tudo antes de começar.
  - Os dados entram em claro no banco daqui, sob a proteção do banco e do
    backup (ADR 032 §5).
- **I7. Leitura consistente e só de leitura.** O 4.0 é lido numa
  transação `REPEATABLE READ READ ONLY`. A importação nunca escreve nele.

**Mapeamento:**

| Tabela do 4.0 | Destino |
|---|---|
| acolhidos | munícipe (achado ou criado), acolhimento com o assistente completo, diagnósticos do checklist, desligamento |
| pia_saude_escola, pia_situacao_familiar, pia_rotina, pia_intervencao_familiar | a ficha do P.I.A. (a última gravação vira a última reavaliação) |
| pia_plano_acao | as metas do plano |
| evolucoes_tecnicas | o prontuário, com a nota "registrado no sistema anterior por <técnico>"; o que for psicologia vai para o registro psicológico, com o sigilo daqui |
| visitantes_voluntarios | as visitas (o tipo sai do sufixo "(Tipo: …)" das observações) |
| documentos_termos | os termos |
| diario_logs | só as anotações da equipe; os registros automáticos do 4.0 já têm equivalente na auditoria |
| relatorios_gerais, scrum_tarefas | relatórios escritos e Kanban da unidade |
| documentos_acolhido | não importado: os PDFs estão em `uploads/` no servidor do 4.0, e o relatório lista quais anexar |

### 2. Avisos de prazo no sino

- **Varredura:** um worker passa pelas Casas Abrigo ativas ao subir e a
  cada 6 horas. Monta o mesmo monitor do painel e avisa a equipe lotada
  na unidade:
  - a lotação pode ser manual ou por grupo do AD;
  - o perfil tem de atender ou coordenar, então a recepção não recebe.
- **Chave do aviso:** `sempras:casa-abrigo:<tipo>:<id>:<prazo>:<nível>`.
  - O mesmo prazo avisa uma vez quando se aproxima e outra quando atrasa.
  - Um prazo remarcado avisa de novo.
  - Repetir a varredura não duplica nada.
- **Conteúdo:**
  - O texto leva só as iniciais da criança.
  - O link abre a aba certa; no caso do P.I.A., já no acolhido.

### 3. Linha do tempo do acolhido

- `GET /sempras/casa-abrigo/{id}/linha-do-tempo` reúne, do mais recente ao
  mais antigo:
  - entrada;
  - evoluções da unidade;
  - P.I.A. e metas;
  - visitas;
  - termos;
  - diário;
  - relatórios;
  - documentos;
  - desligamento.
- O texto da evolução psicológica não aparece para quem não tem o sigilo
  (ADR 030 §5).
- A leitura é auditada, como a ficha.
- Na tela de institucionalizados, cada acolhido tem o botão **Linha do
  tempo**, com impressão.

### 4. RMA

O formulário "Casa Abrigo (indicadores municipais)" do RMA conta:

- crianças e adolescentes ativos, pela faixa do ECA;
- reintegrações, adoções e transferências do mês, pelo tipo do
  desligamento;
- P.I.A. sem reavaliação há mais de 90 dias.

## Consequências

- A unidade pode deixar o 4.0 depois de uma simulação limpa e da
  aplicação. O roteiro está no GUIA_SEMPRAS, em "Importar o histórico do
  4.0".
- Depois disso o 4.0 deve ser desligado e as credenciais do `.env` dele
  trocadas. A chave de cifra, o banco e o AD estão lá em claro.
- Os PDFs ficam no servidor do 4.0 até a equipe anexá-los. O relatório diz
  quais são.
- Os avisos dependem do worker (`backend-worker`). Sem lotação na unidade,
  ninguém recebe, e o monitor do painel continua valendo.
