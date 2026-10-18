# RIPD — SEMPRAS (Atendimento Socioassistencial)

> **Modelo / minuta técnica**, nos termos de [docs/RIPD_MODELO.md](RIPD_MODELO.md)
> §7: "sistemas derivados que tratem dados sensíveis fazem seu próprio
> RIPD". O SEMPRAS trata (art. 11): vulnerabilidade social, violência
> doméstica e infantojuvenil, dados de saúde (registros de atendimento
> psicológico e diagnósticos da Casa Abrigo) e, por natureza do público,
> dados de crianças e adolescentes (LGPD art. 14) em medida protetiva
> (ECA) ou medida socioeducativa. O RIPD **oficial** é elaborado e
> assinado pelo Encarregado de Dados (DPO) do órgão.

- **Sistema:** SEMPRAS — atendimento socioassistencial (ADR 030), módulo do Projeto-Assistencia-v2
- **Controlador:** Secretaria Municipal de Promoção e Assistência Social de Rondonópolis/MT
- **Base legal principal:** LOAS (Lei 8.742/1993) e Tipificação Nacional dos Serviços Socioassistenciais (SUAS) — execução de política pública (LGPD art. 7º, III); ECA (Lei 8.069/1990) para os atendimentos do Conselho Tutelar e das medidas socioeducativas
- **Data desta minuta:** 2026-10-08 (revisada em 2026-10-14 — Fase 8: PAIF, benefícios eventuais, encaminhamentos, Conselho Tutelar, CREAS, Casa Abrigo, RMA/MDS, painel de chamada; revisada em 2026-10-15 — Fase 9: Centro POP — cadastro anônimo, perfil de rua com dado de saúde/drogas, encaminhamento externo, documentos; revisada em 2026-10-16 — ADR 031: atendimento por tipo de unidade, perfil de rua só para o Centro POP, painel de chamada só com a senha nas unidades sigilosas, caso da Casa da Mulher; revisada em 2026-10-17 — ADR 032: cadastro completo da Casa Abrigo, P.I.A. em 6 partes, diário, relatórios escritos, visitas da unidade e tarefas; revisada em 2026-10-18 — ADR 033: importação do histórico do Casa Abrigo 4.0, avisos de prazo no sino e linha do tempo do acolhido)

---

## 1. Descrição do tratamento

| Aspecto | Descrição |
|---|---|
| Natureza | Busca e cadastro do munícipe, fila de atendimento por unidade, atendimento técnico/psicológico, prontuário (evolução), direitos do titular, rede de apoio, benefícios eventuais, encaminhamento entre unidades, vínculo PAIF, atendimento do Conselho Tutelar, acompanhamento do CREAS (PAEFI/MSE/notificações), acolhimento institucional (Casa Abrigo) e o relatório RMA/MDS |
| Escopo | Munícipes atendidos nas 16 unidades da SEMPRAS (CRAS, CREAS, Conselho Tutelar, Casa Abrigo, Casa da Mulher, Centro POP, CEU) — **o titular não tem conta na plataforma** |
| Contexto | Atendimento presencial nas unidades; cada servidor só vê as unidades onde atua (ADR 013) |
| Finalidades | Execução do SUAS e do ECA: acolhimento, triagem, atendimento social e psicológico, acompanhamento familiar (PAIF/PAEFI), proteção de direitos da criança/adolescente (Conselho Tutelar, medida socioeducativa), acolhimento institucional (Casa Abrigo) e relato estatístico obrigatório ao MDS (RMA) |

## 2. Dados pessoais tratados e base legal

| Categoria | Dados | Base legal (LGPD) | Retenção |
|---|---|---|---|
| Identificação do munícipe | nome, nome social, CPF, NIS, data de nascimento, nome da mãe, telefone, endereço | art. 7º III (execução de política pública — LOAS) | enquanto o cadastro não for anonimizado a pedido do titular (§6) ou houver obrigação de guarda (RMA/MDS, processo judicial) |
| Fila e atendimento | tipo (social/psicológico), prioridade legal (Lei 10.048/2000, 12.764/2012, 13.466/2017), status, horários, unidade, profissional | art. 7º III | histórico de serviço — não é "dado do titular" no sentido do art. 18 (ver §6) |
| Prontuário (evolução) | texto do atendimento, autor, unidade, tipo | art. 7º III; **dado sensível** (art. 11) quando psicológico | registro só de inclusão (nunca editado/apagado) — valor de prova do serviço prestado |
| Sigilo psicológico | registros de atendimento psicológico só com o texto visível para quem tem `sempras:psicologia` na unidade | art. 11 (dado sensível de saúde) + Código de Ética do CFP | mesma do prontuário; mascarado (não exportado nem exibido) para quem não tem o sigilo |
| Pedidos do titular | tipo (acesso/correção/anonimização), justificativa, quem solicitou/decidiu, quando | art. 7º II (cumprimento de obrigação legal — art. 18) | permanente (prova do exercício do direito); decisão imutável (trigger de banco) |
| Rede de apoio | nome, parentesco/vínculo, telefone de pessoas de referência do munícipe (família, vizinhos) | art. 7º III | enquanto o cadastro do munícipe existir; exclusão simples (contato cadastrado errado ou que pediu para saser) |
| Benefícios eventuais | tipo (natalidade/funeral/cesta básica/passagem), quantidade, motivo da decisão, quem decidiu | art. 7º III (LOAS art. 22) | permanente — valor de prova do benefício concedido/negado |
| Encaminhamentos entre unidades | motivo do encaminhamento, observação da decisão, unidades de origem/destino | art. 7º III | permanente |
| PAIF (CRAS) | indicadores de vulnerabilidade (extrema pobreza, Bolsa Família, BPC), participação no SCFV, motivo de exclusão do vínculo | art. 7º III | histórico mantido mesmo após exclusão (reinclusão é um novo vínculo) |
| Conselho Tutelar | dados da criança/adolescente (via cadastro), nome/CPF/telefone do responsável, origem da demanda, motivo da violação, medida protetiva aplicada (Art. 101 do ECA) | art. 7º III + ECA; **dado sensível de criança/adolescente** (LGPD art. 14) | permanente — registro de medida protetiva tem valor legal |
| CREAS — PAEFI | violações de direitos (Resolução CNAS nº 109/2009), membros vulneráveis da família (texto livre), situação do PIA | art. 7º III; **dado sensível** (violência, vulnerabilidade) | permanente enquanto caso ativo; sem prazo definido após desligamento (ver §7) |
| CREAS — medida socioeducativa (MSE) | ato infracional (texto livre), número de processo, vara de origem, tipo de medida (LA/PSC), instituição parceira | art. 7º III + ECA; **dado sensível** — ato infracional de adolescente tem sigilo próprio no ECA (art. 143) | permanente — registro de cumprimento de medida tem valor legal/judicial |
| CREAS — notificações | origem (MP/Judiciário/Conselho Tutelar/Disque 100-180/Delegacia), assunto, resposta do CREAS | art. 7º III | permanente — prova de resposta a ofício/requisição |
| Casa Abrigo — acolhimento | sexo, situação especial/PCD, apelido, filiação; responsável antes da medida (nome, CPF, NIS, nascimento, vínculo, endereço por CEP, telefones, e-mail); data na unidade, processos, guia; histórico familiar; 19 motivos do acolhimento; situação jurídica, conselheiro, CNA/CNCAD; contexto social (estuda, ameaça de morte, visitas, benefício, programa social, trabalho); tipo, motivo, data e guia do desligamento | art. 7º III + ECA; **dado sensível de criança/adolescente** (LGPD art. 14) | permanente — registro de custódia sob medida protetiva |
| Casa Abrigo — identidade | identidade de gênero, orientação sexual e raça/cor da criança (o 4.0 já coletava) | **art. 11 — dado sensível** (origem racial, vida sexual) + art. 14 | permanente, vinculado ao acolhimento; opcional ("não informado" é aceito) |
| Casa Abrigo — saúde | checklist de diagnósticos (doenças, deficiências, transtornos), laudo médico, medicação contínua, dependência química | **art. 11 — dado sensível de saúde** | permanente, vinculado ao acolhimento |
| Casa Abrigo — visitantes/termos | nome/documento/vínculo do visitante, tipo de visita (acolhido, instituição, doação, voluntariado), entrada e saída; tipo de termo, responsável pela retirada (a assinatura feita na tela vai só para a impressão, não é guardada) | art. 7º III | permanente |
| Casa Abrigo — diário e relatórios | anotações do diário de bordo (geral ou de um acolhido); relatórios escritos (tipo, etapa, prazo, conteúdo, acolhido opcional) | art. 7º III + ECA; **pode conter dado sensível no texto** | permanente; o diário é só de inclusão; o relatório entregue não muda nem é excluído |
| Casa Abrigo — tarefas da equipe | título, descrição, responsável e prazo das tarefas internas (sem campo de criança) | art. 7º III | até exclusão pela equipe |
| Casa Abrigo — PIA | naturalidade, religião, documentos que possui; saúde (sinais de violência, medicamentos, alergias, vacinação, cartão SUS, tratamentos, deficiências); escola; família (relato, benefícios, moradia anterior, contato, visitas); rotina e comportamento; intervenção familiar; metas do plano de ação | **art. 11 — dado sensível de saúde e de convicção religiosa** + art. 7º III (demais campos) | permanente, vinculado ao acolhimento; ficha é 1:1 (sobrescrita; a data da gravação é a da última reavaliação) |
| Casa Abrigo — documentos | arquivos PDF do acolhido ou do responsável (certidões, laudos, documentos de identidade etc.) | art. 7º III; **dado sensível por natureza do conteúdo** (pode conter CPF, filiação, laudo de saúde) | até exclusão pela equipe técnica; arquivo é removido do object storage (MinIO) junto com o metadado |
| RMA/MDS | contagens agregadas por unidade e competência (não há dado pessoal no relatório — só números) | art. 7º III (obrigação estatística do SUAS/MDS) | permanente — obrigação de prestação de contas |
| Painel de chamada (TV) | nome e senha do munícipe exibidos em tempo real na tela da sala de espera | art. 7º III | nenhuma retenção — exibição efêmera, não grava nada novo (lê a fila já existente) |
| Casa da Mulher — caso | formas de violência doméstica (art. 7º da Lei 11.340/2006), vínculo com o agressor, avaliação de risco, medida protetiva (situação e processo), Patrulha Maria da Penha, filhos, abrigamento sigiloso | **art. 11 — dado sensível** (violência, vida íntima) + Lei 11.340/2006 | permanente, vinculado ao caso; não aparece no resumo das outras unidades; painel de chamada só com a senha (ADR 031) |
| Centro POP — abordagem social | local e data da abordagem, munícipe cadastrado ou descrição física da pessoa não identificada, resultado, uso aparente de drogas, demanda de saúde | art. 7º III; **art. 11** (indício de saúde e drogas) | permanente; só a unidade que abordou vê |
| Cadastro — sexo | feminino, masculino ou não informado | art. 7º III | junto do cadastro; mantido na anonimização (não identifica, entra nas contagens do RMA) |
| Centro POP — cadastro anônimo | flag `anonimo` para munícipe sem nenhuma identificação confiável (abordagem de rua); quando ativo, nome fica só com o placeholder "Cidadão não identificado" e os demais campos de identificação ficam vazios de propósito | art. 7º III — a ausência de identificação é deliberada, não uma falha de coleta | mesma do cadastro comum (§2 acima); `anonimo` em si não é dado pessoal, é a ausência dele |
| Centro POP — perfil de rua | situação habitacional, tempo de rua, motivo da vulnerabilidade, possui doença/descrição, usa medicação/descrição, uso de drogas/descrição | art. 7º III; **art. 11 — dado sensível de saúde** (doença, medicação, uso de drogas) | permanente, vinculado ao cadastro; visível só a quem tem `sempras:atendimento` na unidade (mesmo gate do prontuário — não é mascaramento de campo) |
| Centro POP — encaminhamento externo | destino (CAPS AD, albergue, saúde, polícia etc.), prioridade, motivo, status, observações, técnico responsável | art. 7º III; destino pode revelar indiretamente condição de saúde (CAPS AD) ou situação de risco (DEAM) | permanente — log de encaminhamento, sem fluxo de aceite da outra ponta (ela não é um sistema nosso) |
| Centro POP — documentos | arquivos do munícipe em situação de rua (documentos de identidade, laudos etc.), mesmo padrão dos documentos da Casa Abrigo | art. 7º III; **dado sensível por natureza do conteúdo**, como os documentos da Casa Abrigo | até exclusão pela equipe técnica; arquivo removido do MinIO junto com o metadado |

**Dados sensíveis (art. 11):** sim — registros de atendimento psicológico, diagnósticos e PIA da Casa Abrigo, e agora também o perfil de rua do Centro POP (saúde, medicação, uso de drogas), e, por natureza do público, dados de violência doméstica/infantojuvenil e de medida protetiva/socioeducativa envolvendo crianças e adolescentes (Conselho Tutelar, CREAS, Casa Abrigo) — sujeitos também ao regime de proteção reforçada do **LGPD art. 14** (dados de crianças e adolescentes) e ao sigilo do ato infracional previsto no **ECA art. 143**.

## 3. Fluxo de dados

```
Munícipe (SEM conta na plataforma)
  → atendido presencialmente por Recepcionista/Técnico/Psicólogo/Superintendente
  → Frontend (sessão do SERVIDOR que atende) ──BFF──► API Go
       ├─ sempras_cidadaos (Postgres)                      — identificação
       ├─ sempras_atendimentos / sempras_senhas             — fila do dia
       ├─ sempras_registros (append-only)                   — prontuário
       ├─ sempras_pedidos_titular                           — pedidos do titular (§6)
       ├─ sempras_rede_apoio / sempras_beneficios /
       │  sempras_encaminhamentos / sempras_paif_vinculos    — rede de apoio, benefícios, encaminhamentos, PAIF
       ├─ sempras_conselho_atendimentos                     — Conselho Tutelar (Art. 101 ECA)
       ├─ sempras_paefi_casos / sempras_mse_medidas /
       │  sempras_creas_notificacoes                        — CREAS
       ├─ sempras_casa_abrigo_* (acolhimentos, irmãos,
       │  diagnósticos, visitantes, termos, PIA, documentos) — Casa Abrigo
       ├─ sempras_centro_pop_perfis / sempras_encaminhamentos_
       │  externos / sempras_centro_pop_documentos            — Centro POP (abordagem de rua)
       └─ audit_logs (encadeado por hash)                    — toda leitura de ficha e escrita
  Documentos da Casa Abrigo e do Centro POP: o arquivo nunca atravessa a
  API — o navegador envia direto ao object storage (MinIO) por URL
  pré-assinada de curta duração (mesmo padrão dos módulos Arquivos/
  Trâmite); a API só grava o metadado e confere tamanho/tipo reais depois
  do envio.
  Tempo real: Hub WebSocket (sempras:fila:<unidade>) — nunca dado pessoal
  no frame (ADR 027); o painel de chamada (TV) lê esse mesmo canal.
```

O titular nunca acessa o sistema diretamente — todo tratamento é mediado por quem o atende, presencialmente, na unidade.

## 4. Necessidade e proporcionalidade

| Medida de minimização | Situação |
|---|---|
| Busca só por CPF/NIS exatos (11 dígitos) ou 3+ letras do nome | ✅ |
| CPF e NIS mascarados na busca e no resumo (`***.456.789-**`) | ✅ |
| Histórico mostrado é só o das unidades de quem consulta (`AreaDe`/`CoverageOf`, ADR 013) | ✅ |
| Sigilo psicológico: texto do registro visível só com `sempras:psicologia` **na unidade** — nem o administrador da plataforma tem por padrão | ✅ |
| Leitura do prontuário auditada (toda abertura gera `audit_logs`) | ✅ |
| Identificação mínima aceita (sem CPF/NIS): nascimento + nome da mãe, para população em situação de rua ou sem documentos | ✅ |
| Exportação (direito de acesso) exige justificativa registrada e é auditada por quem exerce, não autosserviço do titular | ✅ |
| Documento da Casa Abrigo: conteúdo nunca passa pela API (só o navegador do servidor e o object storage); URL de download pré-assinada expira em minutos | ✅ |
| Listas da Casa Abrigo com nome de criança (painel, evoluções, visitas, termos, diário, relatórios, acolhimento completo) gravam a leitura na auditoria, como a ficha | ✅ |
| Busca de endereço pelo CEP (ViaCEP, com o BrasilAPI de reserva): só o CEP sai do navegador, sem nome nem outro dado do responsável | ✅ |
| Histórico de termos e avisos do monitor mostram só as iniciais da criança (ECA art. 143) | ✅ |
| Avisos de prazo da Casa Abrigo no sino: só para quem está lotado na unidade com perfil que atende ou coordena (a recepção não recebe), e o texto leva só as iniciais da criança | ✅ |
| Linha do tempo do acolhido: leitura auditada; o texto psicológico some para quem não tem o sigilo | ✅ |
| Importação do Casa Abrigo 4.0 (ADR 033): só lê o banco antigo; o relatório aponta registros pelo id do 4.0, nunca pelo nome; a chave de cifra do 4.0 só existe em memória durante a execução | ✅ |
| Indicadores do RMA/MDS são só contagens agregadas por unidade — nenhum dado pessoal sai do módulo nesse relatório | ✅ |
| Diagnósticos de saúde e PIA da Casa Abrigo **não têm sigilo adicional** ao de quem atende a unidade (mesmo nível de `sempras:atendimento`/`psicologia` do restante do prontuário) | ⚠️ decisão consciente — ver §5 e §7 |
| Cadastro anônimo (Centro POP): quando o munícipe não tem nenhuma identificação confiável, o sistema **não inventa** dado (nome placeholder fixo, demais campos vazios) em vez de forçar preenchimento de CPF/NIS/nascimento/mãe fictícios | ✅ |
| Perfil de rua (saúde/drogas) com o mesmo gate de permissão do sigilo psicológico (`sempras:atendimento` na unidade) — recepção não vê nem edita | ✅ |

## 5. Riscos e salvaguardas

| Risco | Probabilidade | Impacto | Salvaguarda |
|---|---|---|---|
| Acesso a dado de outra unidade | baixa | alto | RBAC por permissão + unidade (ADR 013), testado ("unidade A não vê B") |
| Vazamento de registro psicológico sigiloso | baixa | alto | sigilo é verificado por registro/unidade, não herdado de papel administrativo |
| Re-identificação após anonimização | baixa | médio | CPF/NIS/nome/mãe/telefone/endereço zerados; `data_nascimento` preservado isoladamente (exigência da integridade do cadastro, ver ADR 030 §8) — **pendente avaliação do DPO** se isso é aceitável para o perfil de risco do órgão |
| Anonimização indevida de cadastro com obrigação legal de guarda (RMA/MDS, processo judicial aberto) | média | alto | aprovação humana obrigatória (Superintendente, cobertura total) — nunca automática, diferente da conta de usuário da plataforma |
| Pedido de anonimização registrado por quem não atendeu de fato o titular | baixa | médio | exige permissão de atendimento (`sempras:atendimento`/`psicologia`) + justificativa auditada; decisão final é de um segundo ator (gestão) |
| **Anonimização incompleta**: zerar o cadastro em `sempras_cidadaos` não apaga nem anonimiza os registros ligados nas tabelas novas (PAIF, Conselho Tutelar, PAEFI, MSE, notificações, Casa Abrigo) — campos de texto livre (`membros_vulneraveis`, `observacoes_gerais`, `ato_infracional`, descrição de diagnóstico) e, sobretudo, os **documentos PDF da Casa Abrigo** podem conter a identidade do titular mesmo depois do cadastro principal anonimizado | média | alto | **não mitigado ainda — pendência para o DPO, ver §7** |
| Dado de saúde do acolhido (Casa Abrigo — diagnósticos, PIA) acessível a qualquer perfil com `sempras:atendimento` na unidade, sem sigilo equivalente ao da psicologia | média | médio | decisão de produto deliberada (mesmo nível de acesso das demais informações do acolhido, para não fragmentar a equipe que cuida da criança) — **registrada aqui para avaliação do DPO**, não é uma lacuna não percebida |
| Vazamento de documento da Casa Abrigo via URL pré-assinada compartilhada indevidamente | baixa | alto | URL expira em minutos (mesmo TTL curto dos módulos Arquivos/Trâmite); toda emissão de URL e todo upload/exclusão é auditado |
| Dado de ato infracional (MSE) ou de medida protetiva (Conselho Tutelar) de adolescente exposto fora do sigilo do ECA art. 143 | baixa | alto | mesmo RBAC por unidade/permissão do restante do módulo; nenhum relatório ou exportação solta esses dados sem o gate de `sempras:atendimento` |
| Dado de saúde/drogas do perfil de rua (Centro POP) acessível a qualquer perfil com `sempras:atendimento` na unidade, sem sigilo equivalente ao da psicologia | média | médio | mesma decisão de produto já registrada para os diagnósticos da Casa Abrigo (§5 acima) — **registrada aqui para avaliação do DPO**, não é lacuna não percebida |
| Encaminhamento externo (Centro POP) sem fluxo de confirmação da outra ponta — não há como verificar se o destino (CAPS AD, albergue, polícia) de fato recebeu ou agiu sobre o encaminhamento | baixa | baixo | decisão consciente — a "outra ponta" não é um sistema nosso; o registro é só o log de que o encaminhamento foi feito por quem atende, auditado como qualquer escrita do módulo |
| Vazamento de documento do Centro POP via URL pré-assinada compartilhada indevidamente | baixa | alto | mesma salvaguarda dos documentos da Casa Abrigo — URL expira em minutos, toda emissão/upload/exclusão é auditada |
| Sistema Casa Abrigo 4.0 mantido no ar depois da importação, com a chave de cifra, a senha do banco e a do AD em claro no `.env` do servidor dele | média | alto | roteiro do GUIA_SEMPRAS: desligar o 4.0 e trocar todas as credenciais logo depois da importação — **pendência operacional da TI** |
| Munícipe do 4.0 ligado ao cadastro errado daqui (mesmo nome e nascimento, sem CPF/NIS) | baixa | médio | a ligação só acontece com nome e nascimento iguais e vai para o relatório como "a revisar"; uma criança já acolhida em outro lugar é recusada (um acolhimento ativo por criança) |

## 6. Direitos do titular — operacionalização (ADR 030 §8)

O mecanismo genérico da plataforma (`/api/v1/lgpd/*`) **não se aplica**: é self-service de conta de usuário, e o munícipe não tem conta. O SEMPRAS implementa um fluxo próprio, mediado por quem atende:

| Direito (art. 18) | Canal | Decisão |
|---|---|---|
| Acesso / portabilidade (II, V) | `GET /api/v1/sempras/cidadaos/{id}/pedidos/acesso` | resolvido na hora por quem atende, com o titular presente |
| Correção (III) | `PUT /api/v1/sempras/cidadaos/{id}` + `POST .../pedidos` (tipo `correcao`) | idem |
| Eliminação → **anonimização** (VI) | `POST /api/v1/sempras/cidadaos/{id}/pedidos` (tipo `anonimizacao`) → `POST /api/v1/sempras/pedidos/{id}/decidir` | **aprovação do Superintendente** (`sempras:gestao`, cobertura total) — nunca automática |
| Acompanhamento | `GET /api/v1/sempras/pedidos` (gestão) | — |

Toda a tela correspondente: `/sempras/cidadaos/{id}` (ficha — editar, exportar, solicitar anonimização) e `/sempras/pedidos` (fila de decisão da gestão).

**Importante:** o direito de acesso (exportação) e a anonimização, hoje, cobrem o cadastro (`sempras_cidadaos`), os atendimentos e as evoluções do prontuário — **não** os registros das tabelas adicionadas na Fase 8 (PAIF, benefícios, encaminhamentos, Conselho Tutelar, CREAS, Casa Abrigo) nem as da Fase 9 (perfil de rua, encaminhamento externo, documentos do Centro POP). Um pedido de acesso não entrega esses dados, e uma anonimização aprovada não os apaga nem anonimiza. Ver §7.

## 7. Pendências para o RIPD oficial

- [ ] Avaliação do DPO sobre a preservação de `data_nascimento` após anonimização (exigência técnica da integridade do cadastro — ver ADR 030 §8).
- [ ] Definição formal do prazo de retenção do cadastro não anonimizado (hoje: indefinido, até pedido do titular ou obrigação legal cessar).
- [x] ~~Avaliação de risco específica para as unidades de proteção especial (Conselho Tutelar, Casa Abrigo, Casa da Mulher) — hoje tratadas com o mesmo modelo genérico de prontuário~~ — **parcialmente endereçado na Fase 8**: Conselho Tutelar, CREAS (PAEFI/MSE/notificações) e Casa Abrigo agora têm campos estruturados e enums reais (não mais texto livre genérico). Casa da Mulher e Centro POP continuam no modelo genérico.
- [ ] **Decisão do DPO**: os dados de saúde do acolhido (diagnósticos, PIA da Casa Abrigo) devem ter sigilo equivalente ao da psicologia (visível só a um subconjunto de perfis), ou o nível atual (`sempras:atendimento` na unidade) é aceitável dado que a equipe da Casa Abrigo cuida integralmente da criança? Decisão de produto já tomada no sentido de **não** criar sigilo extra — registrada para revisão.
- [ ] **Estender o direito de acesso e a anonimização** às tabelas da Fase 8 (PAIF, benefícios, encaminhamentos, Conselho Tutelar, PAEFI, MSE, notificações do CREAS, Casa Abrigo — incluindo diagnósticos, PIA e **documentos PDF no MinIO**) e da Fase 9 (perfil de rua, encaminhamento externo, documentos do Centro POP). Hoje a exportação não inclui esses dados e a anonimização não os toca; o conteúdo de um documento pode, por si só, identificar o titular mesmo após o cadastro principal ser anonimizado.
- [ ] Política de retenção/guarda legal específica para RMA/MDS, processos do Conselho Tutelar, casos do PAEFI e medidas socioeducativas, que hoje não têm prazo definido no código (apenas a decisão caso a caso da gestão ao negar uma anonimização).
- [ ] Avaliar se o sigilo do ato infracional (ECA art. 143) exige uma camada de acesso própria para a MSE, separada do `sempras:atendimento` genérico do CREAS.
- [ ] Rotina de descarte/retenção dos documentos da Casa Abrigo e do Centro POP no MinIO quando o caso é encerrado há muito tempo (hoje: permanece até exclusão manual pela equipe técnica, sem expurgo automático).
- [ ] **Decisão do DPO (Centro POP)**: o perfil de rua (saúde, medicação, uso de drogas) deve ter sigilo equivalente ao da psicologia, ou o nível atual (`sempras:atendimento` na unidade) é aceitável — mesma lógica já registrada para os diagnósticos da Casa Abrigo? Decisão de produto já tomada no sentido de manter o mesmo nível de acesso do restante do prontuário.
- [ ] Avaliar se o cadastro anônimo (Centro POP) precisa de uma rotina própria de "fusão" quando a identidade da pessoa é descoberta depois (hoje: não existe — um novo cadastro identificado fica desvinculado do anônimo anterior, duplicando o histórico).
