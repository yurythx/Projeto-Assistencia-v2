# 011 — Borda única, IP do visitante confiável, bloqueio por IP próprio e busca por prefixo

- **Status:** Aceito
- **Data:** 2026-09-27
- **Escopo:** `internal/platform/ratelimit`, `internal/app` (dependências),
  `internal/modules/signum/infrastructure`, `internal/platform/kernel`,
  migration 000125 e os repositórios com full-text, frontend (BFF, proxy
  público, login local), `docker-compose*.yml`, `deploy/caddy`

## Contexto

A plataforma foi implantada num servidor de teste com HTTPS (Caddy e CA
interna), um Keycloak de teste e uma estrutura fictícia de prefeitura: 4
entidades, 15 unidades, 76 departamentos e 754 usuários. Cenários ponta a
ponta (`scripts/demo-data/cenarios/`) fizeram login real pelo Keycloak e
exercitaram todos os módulos, com usuários de lotações diferentes. Os testes
existentes cobriam cada peça isolada. Juntas, com dados e rede reais, elas
mostraram estes problemas:

1. **IP forjável.** O proxy público do frontend repassava o
   `X-Forwarded-For` do cliente. A API confia nesse cabeçalho vindo da rede
   interna. Pela porta direta do frontend, bastava trocar o cabeçalho para
   escapar do limite do formulário de contato e do login (reproduzido).
2. **IP perdido.** O login local (NextAuth Credentials) e o BFF autenticado
   não repassavam o IP. A API via o container do frontend como origem de
   todos os logins. O limite de login e o bloqueio por IP viravam um balde
   só para a organização inteira, e a prova de consentimento LGPD gravava o
   IP do container.
3. **Bloqueio por IP igual ao da conta.** Conta e IP bloqueavam com 5
   falhas, até 24 h, e o contador do IP nunca zera com login certo. Atrás de
   NAT, um IP é um prédio: 5 senhas erradas de pessoas diferentes ao longo
   do dia bloqueavam o login local de todos.
4. **Busca só por palavra inteira.** `websearch_to_tsquery` não acha
   prefixos: "vacina" não encontrava "vacinação".
5. **Assinatura impossível com conta do Keycloak.** A reautenticação do
   Signum usa o grant `password` no client `nexus-backend`. Quando o client
   não permitia esse fluxo (`unauthorized_client`), a mensagem era "senha
   incorreta" para a senha certa.
6. **Supervisor em laço.** No processo da API nenhum plug-in tem worker, e o
   `Kernel.Supervise` voltava na hora. Era reiniciado com WARN a cada 30 s,
   para sempre.

## Decisões

**Borda única.** Em produção, o proxy HTTPS (Caddy) é a única entrada. As
portas diretas passam a escutar em `HOST_BIND=127.0.0.1`, definido por
`scripts/enable-https.sh`. O Caddy sobrescreve o `X-Forwarded-For` com o IP
real. A API confia no cabeçalho vindo da rede docker (`TRUSTED_PROXIES`).

**IP do visitante confiável ponta a ponta.** `lib/api/clientIp.forwardedFor`
repassa o `X-Forwarded-For` só com `TRUST_PROXY_HEADERS=true`, ligado pelo
`docker-compose.https.yml`. Vale para o proxy público, o BFF autenticado e o
login local. Sem proxy de borda o cabeçalho é forjável e não é repassado. O
custo aceito é que os limites por IP viram um balde único, e por isso a
implantação sem proxy não é suportada (`docs/DEPLOY.md`).

**Bloqueio por tipo de sujeito.** `ratelimit.Lockout.WithPolicy(prefixo,
política)` fixa limites por tipo de sujeito:

| Sujeito | Limite | Bloqueio | Janela |
|---|---|---|---|
| Conta | 5 falhas | 1 min, dobrando até 24 h | 24 h |
| IP | `LOGIN_LOCKOUT_IP_THRESHOLD` (padrão 30) | 1 min, dobrando até 1 h | 1 h |

O contador do IP continua sem zerar no login certo. Zerar deixaria um
atacante com uma conta válida limpar o IP durante password spraying.

**Busca por prefixo.** A migration 000125 cria
`nexus_search_tsquery(cfg, q)`, que é o websearch **OU** o prefixo de cada
termo (`'vacina':* & 'campan':*`, sem acento). Consultas com operador
(aspas, `-termo`, `or`) usam só o websearch, para não mudar o sentido.
Todo filtro full-text passa a usá-la, e ela é a regra para plug-ins novos.

**Erro de configuração não é senha errada.** Na reautenticação, só
`invalid_grant` é credencial recusada. Qualquer outro erro de client vira
503 e diz o que ajustar.

**Supervisor.** `Supervise` bloqueia até o contexto acabar, mesmo sem
unidades.

## Consequências

- **Implantação:** fica documentada em `docs/DEPLOY.md`. Sem o proxy
  HTTPS, a stack não deve ser exposta.
- **Testes:**
  - backend: `TestLockoutPolicyPerSubjectKind`, `TestSearchPrefixHTTP`
    (Postgres real), `TestSuperviseWithoutUnitsBlocksUntilShutdown` e os
    casos novos de `TestFederatedReauthentication`;
  - frontend: repasse ou não do IP no proxy público, no BFF e no login
    local;
  - ponta a ponta: `make demo-test`, 7 suítes com 267 checagens.
- **Keycloak oficial:** precisa dos clients descritos em `docs/DEPLOY.md`,
  inclusive o `nexus-backend` confidencial com Direct Access Grants.
- **ADR 003:** descreve o bloqueio de conta da época (no banco). O bloqueio
  vigente é o progressivo no Redis, com as políticas acima.
