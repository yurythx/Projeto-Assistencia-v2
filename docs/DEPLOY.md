# Deploy em servidor (Docker Compose)

Sobe a stack completa em modo produção (`APP_ENV=production`): PostgreSQL,
RabbitMQ, Redis, MinIO, migrations, API, worker e frontend. Camadas
opcionais, ligadas pelo `COMPOSE_FILE` do `.env`:

| Arquivo | O que acrescenta | Ligado por |
|---|---|---|
| `docker-compose.yml` | a stack | sempre |
| `docker-compose.keycloak.yml` | Keycloak **de teste** com usuários fictícios | `make demo-keycloak` |
| `docker-compose.https.yml` | Caddy com HTTPS (CA interna) como única entrada | `scripts/enable-https.sh` |
| `docker-compose.ia.yml` | (opcional) IA local (Ollama, serviço `ia-local`) | acrescentar ao `COMPOSE_FILE` (ver "Inteligência artificial" abaixo) |

O servidor de teste (192.168.1.42) — máquina, endereços, verificação pós-deploy, CI e incidentes já resolvidos — está em [SERVIDOR_TESTE.md](SERVIDOR_TESTE.md).

## Pré-requisitos

- Docker Engine + Docker Compose v2, `git`, `openssl`, `make`, `python3`
  (cenários de teste) e `jq`.
- ~4 GB de RAM (o build do frontend é a etapa mais pesada; o Keycloak de
  teste usa até 768 MB).
- Portas livres no host:
  - sem HTTPS: **3010** frontend, **8010** API/WebSocket, **9010** MinIO,
    **9011** console MinIO (e **8180** com o Keycloak de teste) — diferentes
    das de dev (3002/8002/9002/9003) para não colidir com outras stacks;
  - com HTTPS: **80**, **443**, **8443**, **9443** e **8543** (Caddy); as
    portas acima passam a escutar só no próprio servidor.

## Ordem recomendada (servidor novo)

```bash
git clone https://github.com/yurythx/Projeto-Nexus.git && cd Projeto-Nexus
./scripts/deploy.sh 192.168.1.42        # 1. stack em produção (gera .env e chave)
make prod-seed-admin                     # 2. admin local (senha impressa UMA vez)
make demo-keycloak                       # 3. (teste) Keycloak + estrutura e usuários fictícios
./scripts/enable-https.sh 192.168.1.42   # 4. HTTPS com o Caddy como única entrada
make demo-popular                        # 5. (teste) dados realistas em todos os apps
make demo-test                           # 6. (teste) cenários ponta a ponta
./scripts/backup.sh --install-cron       # 7. backup diário
```

`deploy.sh` gera o `.env` a partir do `.env.example` com segredos aleatórios
(`openssl rand`), `APP_ENV=production` e as URLs públicas do host; gera
`secrets/local_auth_private_key.pem` (login local RS256); faz
`docker compose up -d --build` e espera todos os serviços ficarem `healthy`.

## Atualizar

Envie as mudanças por git e, no servidor:

```bash
make deploy        # git pull --ff-only + rebuild + restart
```

`.env`, chaves e CA são reaproveitados; migrations novas rodam sozinhas
(serviço `migrate`) antes da API e do worker.

**Antes de atualizar uma implantação que já tem lotações com escopo**,
rode `make iam-scope-report` e confira o resultado: ele lista cada lotação
e mapeamento de grupo com escopo e, para cada permissão do perfil, se ela
passa a **valer só no escopo** ou **deixa de valer** (permissões de
plataforma só valem com concessão global — ADR 013). Ajuste as lotações de
quem precisa continuar com acesso amplo (concessão global) antes do deploy.

## Cuidados

- **Não apague nem regenere o `.env`** depois que houver dados: a senha do
  banco fica gravada no volume e `CONFIG_ENCRYPTION_KEY` cifra segredos
  salvos pela tela de Configurações.
- Mudou host ou porta pública? Ajuste `FRONTEND_URL`, `API_PUBLIC_URL`,
  `WEBSOCKET_PUBLIC_URL` e `MINIO_PUBLIC_URL` no `.env` e rode `make deploy`
  — as `NEXT_PUBLIC_*` são embutidas no build do frontend.
- Sem Keycloak (`KEYCLOAK_ISSUER_URL` vazio) só o login local funciona.
- **Disco:** cada `make deploy` deixa cache de build (chegou a 29 GB e
  98% do disco no servidor de teste). Rode `make prod-limpeza` de tempos em
  tempos — remove só cache sem uso há 3 dias e imagens órfãs, nunca
  volumes. Com o disco cheio, o Postgres para de gravar.
- **Instale a CA nas máquinas de teste** (`http://<host>/nexus-ca.crt`):
  aceitar o aviso do navegador vale só para a página. Downloads e envios de
  arquivo (MinIO, `:9443`) são feitos por `fetch` e falham em silêncio sem
  a CA; o WebSocket já usa a mesma origem do site (`/ws`).
- **CA recriada:** se o volume `caddy_data` se perder (ex.: `down -v`), o
  Caddy gera uma CA nova. O `deploy.sh` percebe, atualiza `secrets/ca` e
  reinicia API, worker e frontend — mas cada máquina de teste precisa
  instalar a nova raiz (`http://<host>/nexus-ca.crt`).
- **Não exponha a stack sem o proxy HTTPS.** Sem um proxy de borda que
  sobrescreva o `X-Forwarded-For`, o frontend não repassa o IP do visitante
  (seria forjável) e a API enxerga todo mundo com o IP do frontend: os
  limites por IP (login, contato) e a prova de consentimento LGPD perdem o
  sentido. Servido por HTTP num IP da rede, o navegador também desliga
  recursos de "contexto seguro".

## HTTPS (Caddy com CA interna)

```bash
scripts/enable-https.sh 192.168.1.42
```

Idempotente. Coloca o Caddy (`docker-compose.https.yml`,
`deploy/caddy/Caddyfile`) na frente e troca as URLs do `.env` (cópia em
`.env.bak-*`):

| Serviço | Endereço |
|---|---|
| Nexus e WebSocket de notificações | `https://<host>` (`wss://<host>/ws`) |
| API direta (health, integrações) | `https://<host>:8443` |
| MinIO (URLs pré-assinadas) | `https://<host>:9443` |
| Keycloak de teste | `https://<host>:8543` |
| Raiz da CA (para instalar) | `http://<host>/nexus-ca.crt` |

A porta 80 só serve a CA e redireciona o resto para https.

**TLS 1.2 e 1.3.** O Caddy recusa TLS 1.1 ou anterior e, no 1.2, só oferece cifras modernas. O 1.2 continua aceito porque o Windows 10 (TLS do sistema) não tem 1.3; para exigir só 1.3, troque `protocols tls1.2 tls1.3` por `protocols tls1.3` em `deploy/caddy/Caddyfile` e rode `make deploy`.

**O Caddy é a única entrada.** O script define `HOST_BIND=127.0.0.1`: as
portas diretas (3010/8010/9010/9011/8180) só escutam no próprio servidor —
abertas na rede, deixavam o cliente forjar o `X-Forwarded-For` e escapar dos
limites por IP. O Caddy sobrescreve o cabeçalho com o IP real, o frontend o
repassa à API (`TRUST_PROXY_HEADERS=true`, ligado pelo overlay) e a API
confia nele vindo da rede docker (`TRUSTED_PROXIES`, definido pelo script).

**CA interna.** Sem domínio público, os certificados vêm de uma CA do Caddy
(volume `caddy_data`, incluído no backup). A API e o frontend confiam nela
sozinhos (`secrets/ca/`). **Cada máquina de teste instala a raiz uma vez**:

- Windows: baixe `http://<host>/nexus-ca.crt` → duplo clique → Instalar
  certificado → Máquina local → "Autoridades de Certificação Raiz
  Confiáveis" (ou `certutil -addstore -f Root nexus-ca.crt` como
  administrador); reabra o navegador.
- Firefox usa repositório próprio: Configurações → Certificados → Importar.
- Linux: copie para `/usr/local/share/ca-certificates/` e rode
  `update-ca-certificates`.

Sem a CA o navegador bloqueia (e "continuar" numa porta não libera as
outras). **Confira o relógio da máquina**: os certificados valem a partir da
emissão e um relógio atrasado os vê como "ainda não válidos".

Com domínio e certificado oficiais, troque `tls internal` no Caddyfile.

## Backup e restauração

```bash
scripts/backup.sh                    # agora, em /root/backups/nexus/<data-hora>
scripts/backup.sh --install-cron     # todo dia às 02:30 (log: /var/log/nexus-backup.log)
scripts/restore.sh --verify <dir>    # restaura num banco temporário e compara — não altera nada
scripts/restore.sh --yes <dir> --minio   # SUBSTITUI banco (e objetos) pelo backup
```

Cada backup tem o dump do Postgres, o espelho dos buckets do MinIO, os
volumes do Keycloak de teste e da CA do Caddy (se existirem), `.env` +
`secrets/` e `SHA256SUMS`. Retenção de `BACKUP_KEEP_DAYS` (7) dias.
**O `.env` do backup é indispensável** (senha do banco e
`CONFIG_ENCRYPTION_KEY`): numa máquina nova, copie `<dir>/config/.env` e
`<dir>/config/secrets` antes de restaurar. Os backups ficam no mesmo disco —
copie-os para fora do servidor.

## Atlas — TTDD oficial

As migrations criam o modelo e corrigem as séries usadas pelos
procedimentos de exemplo. A **TTDD completa** (1.686 séries de 9 órgãos,
extraídas de `docs/ttdd.pdf`) é carregada à parte, e pode rodar de novo a
qualquer momento:

```bash
make ttdd-impacto   # mostra o que mudaria, sem gravar
make ttdd-aplicar   # mostra o mesmo relatório e grava
```

Nova versão publicada no Diário Oficial: a gestão do Atlas faz pela tela
(**Atlas → Tabela de Temporalidade → Atualizar TTDD** — envia o CSV revisado
ou o `ttdd.json`, confere o impacto e aplica; ADR 022). Pelo servidor (ADR 019):

1. troque `docs/ttdd.pdf` e rode `cd scripts/ttdd && npm ci && npm run tudo`;
2. revise `divergencias` em `deploy/ttdd/ttdd.json`;
3. `make ttdd-impacto` — séries **novas**, **alteradas** (prazos antes →
   depois), **revogadas** (saíram da tabela) e **restabelecidas**, e os
   **procedimentos afetados** (que apontam para série alterada ou
   revogada);
4. `make ttdd-aplicar`; depois, na página de cada procedimento afetado, a
   gestão revisa e publica uma **nova versão** quando preciso.

Série que sai da tabela não é apagada: fica **revogada** (data e edição do
Diário Oficial), some da consulta e do assistente, mas continua acessível
pelo código, com o histórico dos prazos anteriores.

## Inteligência artificial (assistente do Atlas)

Sem configuração nenhuma, o assistente do Atlas já funciona: responde com
a **síntese canônica** dos procedimentos e da TTDD (sem modelo de
linguagem). A IA é configurada **pela tela**, em **Configurações →
Inteligência artificial** (permissão `ia:manage`, ADR 020):

- **Conexões:** IA local (Ollama), OpenAI, Azure OpenAI, Google Gemini,
  Anthropic (Claude), Groq, OpenRouter, Mistral ou qualquer serviço
  compatível com a API de chat da OpenAI. Endereço, modelo, tempo limite e
  **chave de API** — cifrada no banco (`CONFIG_ENCRYPTION_KEY`), nunca
  devolvida pela API nem gravada na auditoria. Ao salvar, a conexão é
  **testada** com uma conversa real; se falhar, nada é gravado. Trocar o
  endereço exige informar a chave de novo.
- **Assistente do Atlas:** conexão **principal** e **reserva**; se as duas
  falharem, a síntese. Vale na próxima pergunta, sem reiniciar.
- **Fornecedor externo:** exige autorização explícita (registrada com quem
  e quando — LGPD art. 33) e, por padrão, CPF, CNPJ, e-mail e telefone são
  mascarados na pergunta antes de sair da rede.

Enquanto nada for salvo na tela, valem `ATLAS_AI_ENDPOINT`,
`ATLAS_AI_API_KEY`, `ATLAS_AI_MODEL` e `ATLAS_AI_TIMEOUT` do `.env`.

**IA local (serviço `ia-local`, Ollama).** Overlay opcional:

```bash
# .env
COMPOSE_FILE=docker-compose.yml:...:docker-compose.ia.yml   # (+ os overlays que já usa)
IA_LOCAL_MODELO=qwen2.5:1.5b    # opcional; baixado na 1ª subida (serviço ia-local-modelo)
```
```bash
make deploy          # sobe ia-local e baixa o modelo; make ia-modelo baixa de novo
```

Sem porta publicada (só a API o alcança), com limite de memória e CPU
(`IA_LOCAL_MEMORIA`, padrão 2560m; `IA_LOCAL_CPUS`, padrão 3). **Precisa de
RAM sobrando:** sem GPU, um modelo de 1,5B usa ~1,2 GB e um de 3B ~2,5 GB,
além da stack (~2 GB). No servidor de teste (4 GB) o modelo esgotou a
memória e a geração caiu a 0,3 palavra/s — use ≥ 8 GB, uma máquina
dedicada (a conexão "IA local" aceita qualquer endereço da rede, ex.
`http://<servidor-ia>:11434`) ou um fornecedor externo.

Quem usa o assistente precisa da permissão `atlas:read` (limite de
`ATLAS_CHAT_RATE_LIMIT_MAX` consultas por minuto por pessoa, padrão 10).

## Modelar a organização

**Estrutura real a partir do AD.** `deploy/estrutura/` traz a estrutura da
prefeitura extraída do export do Active Directory (25 entidades, 255
unidades, 55 departamentos — sem dados pessoais):

```bash
make estrutura-aplicar                 # idempotente: pode rodar de novo
make estrutura-gerar AD=<pasta-export> # regera a partir de um export novo
```

O export bruto (`docs/AD/`, ignorado pelo git) tem nomes de servidores e não
deve ser versionado. O gerador (`scripts/estrutura/ad-para-estrutura.mjs`)
ignora contas de usuário, só lista os grupos de segurança (viram
mapeamentos na tela de IAM) e documenta no cabeçalho as decisões de
modelagem. **Não misture com os dados fictícios** (`make demo-seed`,
`demo-popular`, `demo-test`): os cenários de teste dependem da prefeitura
fictícia.

Como desenhar entidades, unidades, departamentos, grupos e mapeamentos (e
o que o escopo restringe ou não): [REGRAS_DE_NEGOCIO.md](REGRAS_DE_NEGOCIO.md#6-como-modelar-uma-implantação).

## Ambiente de teste: Keycloak + dados fictícios

Enquanto não há um Keycloak oficial, `make demo-keycloak` sobe um
**Keycloak de teste** (`docker-compose.keycloak.yml`, Keycloak 26 em
`start-dev`) com o realm `nexus` e carrega no Postgres uma estrutura
organizacional fictícia:

| Entidade | Unidades | Departamentos |
|---|---|---|
| Prefeitura Municipal (PREF) | Sede Administrativa | RH, TI, Contabilidade, Compras, Administração, Governo |
| Secretaria de Saúde (SEMSA) | PSF Sagrada Família, PSF Conjunto, PSF Marechal Rondon, UPA 24h | padrão* |
| Secretaria de Educação (SEMED) | Escolas Maria Elza, Marechal Dutra, Silvestre, Elizabete | padrão* |
| SEMPRAS | Sede, CRAS Conjunto, CRAS Ana Carla, CRAS Alfredo, CREAS, Centro POP | padrão* |

\* Direção, Administração, Atendimento, Recursos Humanos, Almoxarifado.

- **50 usuários por unidade**, divididos igualmente entre os departamentos
  (754 no total, com as contas `teste.admin`, `teste.auditor`, `teste.iam`
  e `teste.conteudo`). Lista em `deploy/demo/usuarios.csv`; senha comum em
  `DEMO_USER_PASSWORD` no `.env` do servidor.
- Cada departamento é um grupo no Keycloak (ex.: `SEMSA-UPA-ADM`) mapeado
  para o perfil **Servidor** com lotação no departamento; a Administração
  de cada unidade também recebe **Protocolo** na unidade.
- Os usuários já existem no Nexus antes do 1º login (Diretório com cargo e
  ramal); o login pelo Keycloak só os atualiza.
- Console: `https://<host>:8543/admin` (sem HTTPS: `http://<host>:8180/admin`),
  usuário `admin` e `KEYCLOAK_ADMIN_PASSWORD` do `.env`. Os usuários do
  Nexus ficam no realm **nexus**.
- `make demo-popular` (uma vez; marca em `.demo-popular`) povoa todos os
  apps com dados criados pelos próprios usuários fictícios: serviços do
  catálogo, notícias, manuais na Wiki, salas e eventos, salas de equipe e
  conversas, processos entre unidades em vários estados, documentos
  assinados, pasta por unidade e mensagens de cidadãos.

Mudar a estrutura: edite `scripts/demo-data/generate.mjs`, rode
`make demo-generate` e faça commit dos arquivos gerados. O SQL é reaplicável
(`make demo-seed`), mas o realm só é importado quando ainda não existe —
para reimportar, `docker compose rm -sf keycloak && docker volume rm
projeto-nexus_keycloak_data` e `make deploy` (os usuários voltam com os
mesmos ids; quem estava logado entra de novo).

### Cenários ponta a ponta (`make demo-test`)

Com o Keycloak de teste no ar, as suítes de `scripts/demo-data/cenarios/`
fazem login **de verdade** pelo Keycloak (mesmo fluxo do navegador) com
usuários de lotações diferentes e exercitam casos permitidos e negados:

| Suíte | Cobre |
|---|---|
| `tramite.py` | abertura por perfil/unidade, sigilo, tramitação de ida e volta, conclusão |
| `tramite_documentos.py` | anexo no MinIO, edição, assinatura via Signum, acesso a restrito, caixa da unidade, concluir → arquivar → reabrir |
| `modulos.py` | Signum, Mercúrio, Agenda, Arquivos, Diretório, IAM, Busca, Auditoria, LGPD |
| `conteudo.py` | Blog, Wiki (revisões e conflito de edição), Catálogo (Lei 13.460), Contato anônimo, Egress (anti-SSRF) |
| `colaboracao.py` | tempo real por WebSocket (mensagem, edição, exclusão, difusão via outbox → RabbitMQ), salas de departamento, reserva de sala com conflito, ACL de pastas, Signum sequencial/recusa/cancelamento |
| `integracoes.py` | webhooks entregues e com retentativa, busca global (inclusive por prefixo), auditoria por tipo de ação |
| `plataforma.py` | CRUD do IAM, anti-escalada de privilégio, lotação com efeito imediato, usuários locais e bloqueio, módulos e dependências, branding, transparência, monitoramento, LGPD |

Chamadas avulsas como qualquer usuário:
`scripts/demo-data/cenarios/nx <usuario> GET me`.

### Trocar pelo Keycloak oficial

No realm oficial, dois clients (o realm de teste em
`deploy/keycloak/realm-nexus.json` serve de modelo):

- **`nexus-frontend`** — confidencial, Standard Flow com PKCE S256,
  redirect `https://<host>/*`, *audience mapper* incluindo `nexus-backend`
  no access token e *group membership mapper* `groups` com
  `full.path=false` (é o nome do grupo que o IAM cruza com os mapeamentos).
- **`nexus-backend`** — confidencial, **só Direct Access Grants**: é a
  audience dos tokens e o client da reautenticação do Signum (senha na hora
  de assinar). Sem ele, ninguém assina com conta do Keycloak.

Depois, no `.env`: `KEYCLOAK_ISSUER_URL`, `KEYCLOAK_CLIENT_SECRET`,
`KEYCLOAK_FRONTEND_CLIENT_SECRET`; tire `docker-compose.keycloak.yml` do
`COMPOSE_FILE` e rode `make deploy`. Se o Keycloak usar certificado da CA
interna, ele já é confiável; se usar outra CA privada, coloque a raiz em
`secrets/ca/`.

## Segurança do login local

O login local (contingência) tem dois bloqueios progressivos (Redis):

| Sujeito | Libera | Depois | Contador |
|---|---|---|---|
| Conta | 5 falhas | 1 min, dobrando até 24 h | 24 h (zera no login certo ou no desbloqueio pelo gestor) |
| IP | `LOGIN_LOCKOUT_IP_THRESHOLD` (30) falhas | 1 min, dobrando até 1 h | 1 h |

O limite do IP é alto de propósito: atrás de NAT, um IP é um prédio
inteiro. O contador do IP não zera com login bem-sucedido (senão uma conta
válida "limparia" o IP durante password spraying). Há ainda o limitador de
login por IP (10/min), que cai à metade a cada 3 falhas e volta ao normal no
primeiro login certo.

## Comandos úteis

| Comando | O que faz |
|---|---|
| `make deploy` | git pull + rebuild + restart, esperando `healthy` |
| `make prod-ps` / `make prod-logs` | estado e logs dos serviços |
| `make prod-seed-admin` | cria/reseta o admin local |
| `make prod-down` | para a stack (mantém os volumes) |
| `make demo-keycloak` | liga o Keycloak de teste e carrega a estrutura fictícia |
| `make demo-generate` / `make demo-seed` | regera / reaplica os dados fictícios |
| `make demo-popular` | povoa todos os apps (uma vez) |
| `make demo-test` | roda os cenários ponta a ponta |
| `make iam-scope-report` | impacto da permissão com escopo nas lotações atuais (ADR 013) |
| `scripts/backup.sh` / `scripts/restore.sh` | backup e restauração |
