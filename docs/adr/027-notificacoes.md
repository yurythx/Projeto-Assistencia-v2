# 027 — Notificações persistentes e avisos de nova versão

- **Status:** Aceito e implementado
- **Data:** 2026-10-04
- **Relacionado:** [ADR 004](004-outbox-listen-notify-e-rbac-modular.md) (outbox e barramento), [ADR 024](024-biblioteca-de-modelos.md) (modelos), [ADR 026](026-tramite-e-atlas-publico.md) (Trâmite ligado ao Atlas)

## Contexto

Quem acompanha um fluxo precisa saber quando o procedimento ou o modelo
dele muda. O sino da plataforma só guardava, em memória, os toasts da
sessão: recarregar a página apagava tudo. Não havia caixa de entrada,
leitura, preferência nem garantia contra aviso repetido. Avisar "todo
mundo" a cada mudança geraria ruído e seria ignorado.

## Decisão

1. **Caixa de notificações da plataforma** (`platform/notificacoes`,
   migração 000140), disponível a todos os módulos:
   - **Persistência:** cada aviso é gravado para o usuário, com título,
     mensagem, link e lido ou não.
   - **Idempotência:** a chave é única por usuário (`UNIQUE (user_id,
     chave)`). Repetir o envio, ou a reentrega de um evento, não duplica
     o aviso.
   - **Consistência:** o aviso é gravado **na mesma transação** da
     mudança (`Registrar`), então só existe se a mudança existir. Ele é
     **entregue em tempo real depois do commit** (`Entregar`, WebSocket
     no tópico pessoal `user:<id>`). A entrega é de melhor esforço: quem
     está fora vê o aviso ao abrir o sino.
   - **Opt-out por módulo:** em `/notificacoes`, a pessoa desliga os
     avisos de um módulo, e a escolha vale no envio.
   - **Segurança:** o link só aceita caminho interno (CHECK no banco e
     validação no código), sem redirecionamento externo. Cada pessoa só lê
     e marca a própria caixa (`GET /notificacoes`,
     `POST /notificacoes/{id}/lida`, `POST /notificacoes/lidas`,
     `GET`/`PUT /notificacoes/preferencias`).
   - **Retenção (LGPD, minimização):** um aviso lido há mais de 180 dias é
     apagado quando chega um aviso novo para a mesma pessoa.
2. **Quem recebe o aviso de nova versão de procedimento** (Atlas):
   - **Quem segue o fluxo:** o botão "Seguir este fluxo" (opt-in) segue
     pelo código processual, então vale para as versões seguintes.
   - **Quem está lotado nas unidades que participam do fluxo:** a lotação
     manual ou o grupo do AD mapeado, nas unidades ativas cuja sigla é a
     da etapa ou o primeiro segmento dela (`SEMAD/LIC` → `SEMAD`). Conta
     a versão antiga e a nova.
   - **Quem publicou não recebe o próprio aviso.**
   - A importação em lote avisa cada versão nova que aplicou. A simulação
     não avisa.
3. **Nova versão de modelo:** recebe quem segue um procedimento em vigor
   que usa o modelo, na peça ou pela série.
4. **Processos do Trâmite:** quando uma versão de procedimento é
   substituída ou desativada, o Atlas publica `atlas.workflow.deactivated`.
   Esse evento agora inclui `substituido_por` e `versao_nova`.
   - O Trâmite consome o evento (fila `nexus.tramite.atlas`, com DLQ e
     retry do barramento) e avisa **quem abriu** cada processo em
     andamento (aberto ou em tramitação) que segue aquela versão.
   - Os módulos continuam independentes: só se falam por evento.
   - O aviso é idempotente por processo e versão.

## Consequências

- O sino mostra a caixa persistida e os avisos novos na hora.
  `/notificacoes` traz a lista completa e as preferências.
- Novos módulos podem avisar com `notificacoes.Registrar` dentro da
  própria transação, sem infraestrutura nova.
- As notificações são dado pessoal (destinatário e conteúdo): ficam
  sujeitas à retenção acima e são apagadas com o usuário
  (`ON DELETE CASCADE`).
- **Fora do escopo:** e-mail e resumo diário. Os avisos ficam no sino e na
  caixa, e a entrega por e-mail pode ser acrescentada depois sobre a mesma
  tabela.
