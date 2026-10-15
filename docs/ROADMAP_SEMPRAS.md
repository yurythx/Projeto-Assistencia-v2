# Roadmap — SEMPRAS (Projeto-Assistencia-v2)

As decisões estão no [ADR 030](adr/030-sempras-atendimento-socioassistencial.md).
Cada fase entrega algo utilizável, com testes. Nenhuma fase cria utilitário
de plataforma que já exista (DRY):

- Kernel;
- `auth.Can` e `CoverageOf`;
- auditoria e outbox;
- Hub WebSocket;
- idempotência;
- `ScopePicker` e `ModuleGate`.

## Decisões confirmadas (2026-10-08)

| Tema | Decisão |
|---|---|
| Renomeação | Marca e repositório. Mudam o nome exibido, a documentação e o módulo Go `github.com/yurythx/projeto-assistencia-v2`. Os identificadores técnicos (`nexus-admin`, `nexus_*`, filas `nexus.*`) ficam como nome da plataforma base |
| Estrutura no Kernel | Um plug-in `sempras`; as 16 unidades são dados (Unidades do IAM) |
| Vínculo de acesso | Um grupo do AD por unidade × perfil, transformado em Mapeamento AD (perfil + unidade) |
| Escopo | Todas as 16 unidades veem só a própria; o Superintendente (na entidade SEMPRAS) e o Administrador veem todas |

## Fases

| Fase | Entrega | Situação |
|---|---|---|
| **0. Repositório** | Commit inicial na `main`, renomeação da marca e do módulo Go, migração 000142 (nome padrão do branding), remote `projeto-assistencia-v2` | Feito (o push fica com quem tem a credencial do GitHub) |
| **1. Decisões** | ADR 030, matriz de perfis × permissões, convenção e lista dos grupos do AD | Feito |
| **2. Estrutura e acesso** | Entidade e 16 unidades (já em `deploy/estrutura`), os 4 perfis semeados na migração do módulo, SQL dos 49 mapeamentos AD em `deploy/sempras/` | Feito; pedir os grupos à TI |
| **3. Esqueleto** | Plug-in `sempras` (Manifest, permissões com escopo, rotas, migração), registro no Kernel, contrato OpenAPI | Feito |
| **4. Munícipe** | Busca por CPF/NIS/nome com histórico só da área, cadastro simples com CPF validado, edição auditada | Feito |
| **5. Fila e tempo real** | Inclusão (recepção/técnico), senha do dia, prioridade legal, chamar o próximo sem corrida, estados auditados, tópico `sempras:fila:<unidade>` | Feito |
| **6. Atendimento e prontuário** | Ficha com leitura auditada, evolução só de inclusão, sigilo psicológico | Feito |
| **7. Titular e qualidade** | Pedido do munícipe (acesso/correção/anonimização, ADR 030 §8), e2e Playwright da jornada completa, revisão e-MAG (reaproveita componentes já auditados) e [RIPD do módulo](RIPD_SEMPRAS.md) | Feito |
| **8. Evolução** | Rede de apoio, benefícios eventuais (LOAS art. 22), encaminhamento entre unidades (CRAS → CREAS), vínculo PAIF (CRAS), atendimento do Conselho Tutelar com medida protetiva (Art. 101 do ECA), CREAS (PAEFI, MSE em meio aberto e notificações de entes externos), Casa Abrigo completa (acolhimento institucional, irmãos, diagnósticos, desligamento restrito à equipe técnica, visitantes, termos de advertência/passeio/saída, PIA e documentos PDF do acolhido/responsável sobre o MinIO existente — RN-15), o relatório RMA/MDS (indicadores reais por unidade e competência, em `/sempras/rma`) e o painel de chamada para TV da sala de espera (`/sempras/painel-chamada`, reaproveita a fila em tempo real) — feitos. Integração com o CadÚnico (exige credenciais/especificação oficial da API federal — não é possível implementar sem isso) e agendamento (fora de escopo: o legado tem um sistema genérico por departamento/serviço, não específico de assistência social — decisão sobre um módulo de agendamento da plataforma fica para outra conversa) seguem fora do backlog deste módulo | Feito (ressalvas documentadas) |
| **8.1 Paridade com o `protocolo_digital`** | Comparação file-a-file do legado de referência achou campos sem equivalente (notificação do CREAS sem prazo visível no formulário, benefícios sem forma de acesso/aluguel social, PAIF sem particularizado/coletivo — corrigidos) e dois módulos sem nenhuma tela (Ofícios do CRAS — nova tabela `sempras_cras_oficios`, espelha a notificação do CREAS; Atendimentos Coletivos/SCFV — nova tabela `sempras_atendimentos_coletivos`, categoria como enum real em vez do texto livre com `ILIKE` que o legado usa no RMA, alimenta 8 novos campos do RMA + nova página `/sempras/coletivos`) | Feito |
| **9. Centro POP** | Atendimento à população em situação de rua. Revendo a especificação do legado (`SocialAssistance.js`, 8 "entidades") contra o módulo que já existe aqui: Centro POP é só mais uma `unidade` do SEMPRAS (fila, prontuário, benefícios, rede de apoio e evolução já funcionam sem nenhuma mudança) — o trabalho novo de verdade foi: cadastro anônimo (`sempras_cidadaos.anonimo`, pra quem não tem nenhuma identificação confiável), perfil de rua (`sempras_centro_pop_perfis`, extensão 1:1, situação/tempo de rua/motivo/saúde/drogas — sigiloso como o registro psicológico), encaminhamento pra fora do sistema (`sempras_encaminhamentos_externos` — CAPS AD, albergue, saúde, polícia; log gerenciado por quem atende, sem fluxo de aceite de 2 pontas, diferente do encaminhamento entre unidades), documentos (`sempras_centro_pop_documentos`, mesmo padrão da Casa Abrigo) e 2 tipos de benefício (auxílio-gás, cobertor/agasalho). Abordagem de rua (`SocialAbordagem`) e um painel de indicadores dedicado ficam fora — o próprio legado não tem tela de criar/editar abordagem, e o `/sempras/painel` + RMA já cobrem o essencial | Feito |

## Matriz de acesso

| Ação | Recepcionista | Técnico | Psicólogo | Superintendente |
|---|:-:|:-:|:-:|:-:|
| Buscar munícipe (CPF/NIS/nome) | ✓ | ✓ | ✓ | ✓ |
| Cadastrar e editar munícipe | ✓ | ✓ | ✓ | ✓ |
| Incluir na fila | ✓ | ✓ | ✓ | ✓ |
| Ver a fila da unidade (tempo real) | ✓ | ✓ | ✓ | ✓ (todas) |
| Cancelar quem aguarda | ✓ | ✓ | ✓ | ✓ |
| Chamar, iniciar, concluir, marcar ausência (social) | | ✓ | ✓ | ✓ |
| Chamar e atender o psicológico | | | ✓ | |
| Ver a ficha (leitura auditada) | | ✓ | ✓ | ✓ (todas) |
| Registrar evolução social | | ✓ | ✓ | ✓ |
| Ver e registrar evolução psicológica | | | ✓ | |

Recepcionista, técnico e psicólogo atuam **na própria unidade**. O
Superintendente atua em **todas as unidades da SEMPRAS**.

## Grupos do AD a pedir à TI

Convenção: `SEMPRAS_<UNIDADE>_<PERFIL>`, com o perfil em `RECEPCIONISTA`,
`TECNICO` ou `PSICOLOGO`. Mais um grupo, `SEMPRAS_SUPERINTENDENCIA`.

| Unidade | Sufixo do grupo |
|---|---|
| Casa Abrigo | `CASA_ABRIGO` |
| Casa da Mulher | `CASA_DA_MULHER` |
| Centro POP | `CENTRO_POP` |
| CEU | `CEU` |
| Conselho Tutelar Central | `CONSELHO_TUTELAR_CENTRAL` |
| Conselho Tutelar Vila Operária | `CONSELHO_TUTELAR_VILA_OPERARIA` |
| CRAS Alfredo de Castro | `CRAS_ALFREDO_DE_CASTRO` |
| CRAS Ana Carla | `CRAS_ANA_CARLA` |
| CRAS Central | `CRAS_CENTRAL` |
| CRAS Conjunto São José | `CRAS_CONJUNTO_SAO_JOSE` |
| CRAS Iguaçu | `CRAS_IGUACU` |
| CRAS Luz Dyara | `CRAS_LUZ_DYARA` |
| CRAS Padre Lothar | `CRAS_PADRE_LOTHAR` |
| CRAS Rio Vermelho | `CRAS_RIO_VERMELHO` |
| CRAS Sagrada Família | `CRAS_SAGRADA_FAMILIA` |
| CREAS | `CREAS` |

Exemplo: `SEMPRAS_CRAS_CENTRAL_TECNICO`. Depois que os grupos existirem,
`deploy/sempras/mapeamentos-ad.sql` cria os mapeamentos. O SQL é
idempotente e localiza as unidades pelo slug da estrutura real.

## Como implantar

1. `make estrutura-aplicar`: entidade SEMPRAS e as unidades.
2. Migrations: `make migrate-up` cria as tabelas e os 4 perfis.
3. `make sempras-mapeamentos`: os 49 mapeamentos dos grupos do AD
   (`deploy/sempras/mapeamentos-ad.sql`).
   Antes de os grupos existirem, cadastre as lotações em Configurações →
   Usuários.
4. Ligue o módulo em **Configurações → Módulos → SEMPRAS**.

## Boas práticas seguidas em todas as fases

- **ADR antes do código** para qualquer mudança de RBAC, auditoria ou
  contrato.
- **Clean Architecture:**
  - o domínio não conhece o banco;
  - a aplicação delimita a transação (escrita, outbox e auditoria juntos).
- **Autorização em duas camadas:**
  - `RequirePermission` na rota;
  - `auth.Can` no recurso;
  - `CoverageOf` no SQL das listagens.
- **Dados sensíveis:**
  - minimização na busca;
  - mascaramento de CPF e NIS;
  - nada pessoal em eventos, frames ou logs;
  - leitura do prontuário auditada.
- **Testes:**
  - matriz de falhas do repositório;
  - HTTP de ponta a ponta com Postgres real;
  - "unidade A não vê B";
  - módulo desligado responde 404;
  - contrato OpenAPI;
  - gate de 100% de cobertura por pacote.
