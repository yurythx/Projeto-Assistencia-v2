# Servidor de teste (192.168.1.42) — operação, limites e incidentes

Registro do que se sabe sobre o servidor de teste: como está montado, como
implantar e verificar, e o que já deu errado (com causa e correção). Senhas
**não** ficam aqui: estão no `.env` do servidor e com o responsável.

## Máquina

| Item | Valor |
|---|---|
| Endereço | `192.168.1.42` (acesso `root` por SSH) |
| Tipo | contêiner LXC no Proxmox (`pve-vm-103`), Ubuntu 24.04 |
| CPU | AMD Ryzen 5 PRO 4650G, **4 vCPU**, AVX2, **sem GPU** |
| Memória | **4 GB** de RAM + 4 GB de swap |
| Disco | 49 GB (`/`), compartilhado por todos os projetos do host |

Outros projetos no mesmo host: **Aurora** e **protocolo** — **nunca** apague
volumes ou dados deles (`docker volume ls` mostra todos juntos).

## Implantação

- Checkout: `/root/Projetos/Projeto-Nexus-v2` (origin `Projeto-Nexus-v2`).
- `.env`: `COMPOSE_PROJECT_NAME=projeto-nexus-v2`,
  `COMPOSE_FILE=docker-compose.yml:docker-compose.keycloak.yml:docker-compose.https.yml`,
  `HOST_BIND=127.0.0.1` (só o Caddy fica exposto na rede).
- Implantar: `git pull && make deploy`.
- **Peculiaridade:** o `make deploy` às vezes termina com "backend-api is
  unhealthy" — na primeira partida a API estoura o tempo da descoberta OIDC
  no Keycloak (via Caddy), reinicia sozinha e fica saudável. Espere ~1 min e
  rode `docker compose up -d` para subir o frontend.

| Endereço | O que é |
|---|---|
| `https://192.168.1.42` | Nexus (frontend) e **WebSocket** de notificações (`wss://…/ws`) |
| `https://192.168.1.42:8443` | API direta (health, integrações) |
| `https://192.168.1.42:9443` | MinIO (URLs de download/envio de arquivos) |
| `https://192.168.1.42:8543` | Keycloak de teste |
| `http://192.168.1.42/nexus-ca.crt` | Raiz da CA interna, para instalar nas máquinas |
| `http://127.0.0.1:8010` (no servidor) | API sem proxy — para testes com `curl` |

## Verificação depois de cada deploy

```bash
cd /root/Projetos/Projeto-Nexus-v2
docker compose ps                         # 9 serviços "healthy"
curl -s http://127.0.0.1:8010/health      # postgres, redis, rabbitmq, minio: ok
df -h /                                   # disco abaixo de ~80%
docker compose logs --since 1h backend-api backend-worker | grep -E '"level":"(ERROR|WARN)"'
docker compose exec -T postgres psql -U nexus -d nexus -Atc \
  "SELECT status, count(*) FROM outbox_events GROUP BY status"   # nada 'failed'
make ttdd-impacto                         # TTDD do banco = TTDD oficial (tudo INALTERADA)
```

Páginas protegidas respondem **307** sem sessão (redirecionam para o login) —
é o esperado. O `/api/v1` no domínio público cai no frontend; a API pública
do navegador passa por `/api/backend/…` (exige sessão).

## Testes de integração do backend (CI no servidor)

A máquina de desenvolvimento (Windows) não roda Go nem Docker: os testes com
Postgres, RabbitMQ e MinIO reais rodam no servidor, em contêineres
descartáveis (rede `nexusci-net`), sem tocar na stack de produção.

```bash
/root/ci-backend.sh up       # sobe Postgres, RabbitMQ e MinIO de teste
/root/ci-backend.sh test     # migra e roda a suíte com cobertura
/root/ci-backend.sh gate     # exige 100% de cobertura por módulo
/root/ci-backend.sh openapi  # regera docs/openapi.yaml e backend/docs/openapi.json (banco migrado)
/root/ci-backend.sh run '<comando>'
/root/ci-backend.sh down
```

O código vai para `/root/nexus-ci` (cópia de trabalho). **Sempre `down` e
`up` antes da suíte**: rodá-la duas vezes no mesmo banco quebra os testes de
IAM.

## Incidentes e lições

### 2026-10-03 — Tempo real (WebSocket) "fora"

- **Sintoma:** o indicador de conexão ficava desligado; o navegador pedia o
  ticket (`POST /ws/ticket`) mas nenhum `GET /ws` chegava à API.
- **Causa:** o WebSocket usava outra porta (`:8443`) e a máquina do usuário
  não tinha a CA interna instalada — ele aceitou o aviso de certificado só
  para o site. Para WebSocket o navegador **não mostra aviso**: recusa em
  silêncio.
- **Correção:** WebSocket na mesma origem do site (`wss://192.168.1.42/ws`;
  o Caddy encaminha `/ws` para a API). `WEBSOCKET_PUBLIC_URL` no `.env`.
- **Lição:** instale a CA (`http://192.168.1.42/nexus-ca.crt`, em
  "Autoridades de Certificação Raiz Confiáveis") em toda máquina de teste —
  envio e download de arquivos (MinIO, `:9443`) falham do mesmo jeito sem ela.

### 2026-10-03 — Disco a 98%

- **Causa:** cache de build do Docker acumulado a cada deploy (29 GB, no
  `containerd`). Com o disco cheio, o Postgres para de gravar.
- **Correção:** `make prod-limpeza` (cache sem uso há 3 dias e imagens
  órfãs; **nunca** volumes) — de 98% para 57%.
- **Lição:** confira `df -h /` antes de cada deploy; rode `make prod-limpeza`
  de tempos em tempos.

### 2026-10-03 — IA local (Ollama) inviável neste servidor

Medição com o assistente do Atlas (serviço `ia-local`, Ollama 0.35.1,
modelo `qwen2.5:1.5b`, limite de 2,5 GB e 3 CPUs):

| Medida | Resultado |
|---|---|
| Qualidade | correta ("guardar por 100 anos, depois eliminar") |
| Leitura do contexto | ~115 tokens/s (427 tokens em 3,7 s) |
| **Geração** | **0,3 token/s** (43 tokens em ~150 s); a 1ª resposta estourou 300 s |
| Memória do modelo | ~1,2 GB |
| Efeito na máquina | RAM esgotada (49 MB livres, 1,3 GB de swap), releitura de disco a ~400 MB/s, tudo lento |

- **Conclusão:** com 4 GB para a stack inteira (~2 GB) não sobra memória
  para um modelo — **não ligue o `docker-compose.ia.yml` aqui**.
- **Para ter IA:** aumentar a RAM para ≥ 8 GB (modelo de 1,5B) ou ≥ 12 GB
  (3B); ou rodar o Ollama em outra máquina e cadastrar a conexão "IA local"
  apontando para ela (`http://<máquina>:11434`); ou usar um fornecedor
  externo pela tela (Configurações → Inteligência artificial — ADR 020).
- **Sobras:** a imagem do Ollama (9,4 GB, traz bibliotecas de GPU) foi
  removida; ficou o volume `projeto-nexus-v2_ia_local_modelos` (2,3 GB, os
  modelos baixados). Se a IA local não for usada aqui:
  `docker volume rm projeto-nexus-v2_ia_local_modelos`.

### Carga do host Proxmox

O `uptime` dentro do contêiner mostra a carga **do host** (load average
~115–130 em 2026-10-03), não a do Nexus: só 2 processos do contêiner
estavam bloqueados enquanto o `vmstat` mostrava ~100. O host já estava
carregado antes dos testes de IA — vale verificar o que mais roda nele.

## TTDD

- Conferir: `make ttdd-impacto` (não grava). Nova publicação: pela tela
  (Atlas → Tabela de Temporalidade → Atualizar TTDD — ADR 022) ou pelo
  servidor ("Atlas — TTDD oficial" em [DEPLOY.md](DEPLOY.md), ADR 019).

## Segurança pendente

- Trocar a senha de `root` do servidor e a do `admin` local do Nexus
  (`make prod-seed-admin` redefine a do admin).
