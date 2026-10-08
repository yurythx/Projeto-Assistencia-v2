# Testes E2E (Playwright)

A suíte de `frontend/e2e/` roda num navegador de verdade contra a stack
completa: frontend, API Go, worker, Postgres, RabbitMQ, Redis e MinIO. Não
há mocks. O que a tela faz é o que o backend grava, audita e publica.

## O que é coberto

| Spec | Fluxo |
|---|---|
| `public-pages.spec.ts` | Site público, redirecionamento da área logada, `robots.txt` e `sitemap.xml` |
| `accessibility.spec.ts` | e-MAG: links de salto, Alt+1/2/4, alto contraste, escala de fonte e modal LGPD |
| `auth.spec.ts` | Login local (com credencial certa e errada) e logout |
| `hydration-no-flash.spec.ts` | Branding salvo e cookies de preferência já refletidos no HTML do servidor |
| `flows.spec.ts` | Fluxos críticos, ver abaixo |

Os fluxos críticos de `flows.spec.ts`:

- **Kernel:** desativar um plug-in tira o link do menu e bloqueia a tela; reativar devolve os dois.
- **Blog:** rascunho, publicação e listagem.
- **Contato:** o visitante envia a mensagem e recebe um protocolo; a gestão faz a triagem.
- **Signum:** envelope, cerimônia de assinatura (inclusive com senha errada, sem perder a sessão) e verificação pública do arquivo.
- **Arquivos:** criação de pasta, envio direto ao MinIO por URL pré-assinada e download.
- **Auditoria:** a cadeia de hash da trilha segue íntegra.

## No CI

O job **Docker** do `ci.yml` executa a suíte em cinco passos:

1. sobe a stack com `docker compose`;
2. espera todos os serviços ficarem saudáveis;
3. cria o administrador com `cmd/seedadmin`, que gera uma senha aleatória, mascarada no log;
4. roda `npx playwright test`;
5. em caso de falha, publica o relatório como artefato.

## Localmente

```bash
cp .env.example .env
mkdir -p secrets && openssl genrsa -out secrets/local_auth_private_key.pem 2048 && chmod 644 secrets/local_auth_private_key.pem
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
make seed-admin            # imprime a senha do usuário "admin"

cd frontend
export PLAYWRIGHT_BASE_URL=http://localhost:3002 E2E_ADMIN_USERNAME=admin E2E_ADMIN_PASSWORD='<senha impressa>'
npx playwright test
```

Sem `E2E_ADMIN_PASSWORD`, os testes que exigem login são pulados.

Se o Chromium já estiver instalado em outra versão (sandbox, runner sem
download), aponte o executável com
`PLAYWRIGHT_CHROMIUM_EXECUTABLE=/caminho/do/chrome`.

A suíte faz login uma única vez (projeto `setup`, que grava a sessão em
`e2e/.auth/`) e os testes reaproveitam essa sessão. O backend limita as
tentativas de login por IP, e logar a cada teste esbarraria nesse limite.
