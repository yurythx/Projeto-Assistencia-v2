# 006 — Respostas de erro no padrão RFC 7807 (Problem Details)

- **Status:** Aceito — Opção B (negociação de conteúdo), implementada em 2026-09-09
- **Data:** 2026-09-09
- **Autores:** Engenharia Projeto Nexus

## Contexto

Gap G-13 da auditoria de conformidade. Hoje todo erro da API sai no
envelope próprio da plataforma:

```json
{ "data": null, "error": { "code": "VALIDATION_ERROR", "message": "..." } }
```

com `Content-Type: application/json`. A interoperabilidade e-PING e o
padrão RFC 7807 pedem `application/problem+json` com os campos
`type` (URI), `title`, `status`, `detail` e `instance`.

Trocar o formato do corpo de erro é uma **quebra de contrato**: todo
cliente atual (o frontend Next.js inclusive) lê `error.code` /
`error.message`.

## Decisão (proposta)

Duas opções, a decidir com os consumidores da API:

### Opção A — Nova major `/api/v2` (preferida)

- `/api/v1` continua com o envelope atual, sem data de fim anunciada.
- `/api/v2` responde `application/problem+json`. `type` é uma URI estável
  de um catálogo de erros publicado (ex.: `https://<host>/errors/validation-error`).
- `apperrors.Error` ganha um método `ProblemDetails(instance string)` que
  a camada de transporte serializa; o `code` legível por máquina é
  mantido como membro de extensão (`"code": "VALIDATION_ERROR"`).
- `openapi.yaml` documenta os dois.

### Opção B — Content negotiation por `Accept` — **ADOTADA**

- Um cliente que manda `Accept: application/problem+json` recebe o
  formato RFC 7807; o default continua o envelope atual.
- Menos limpo (o mesmo endpoint com dois formatos de erro), mas sem nova
  árvore de rotas e **sem quebra de contrato** — nenhum consumidor atual
  envia esse `Accept`.

## Implementação (2026-09-09)

- `httputil.WriteError` verifica `wantsProblemJSON(r)` — `Accept` contém
  `application/problem+json` — e, se sim, chama `writeProblemDetails`:
  - `type`: `urn:nexus:error:<code minúsculo>` (URN estável,
    independente de domínio; ex.: `urn:nexus:error:validation_error`).
  - `title`: `http.StatusText(status)`.
  - `status`, `detail` (= `appErr.Message`), `instance` (= `r.URL.Path`).
  - Extensões RFC 7807: `code` (o identificador legível por máquina que o
    envelope já expunha) e `request_id` (de `logging.RequestID`).
  - `Content-Type: application/problem+json; charset=utf-8`.
- Respostas de sucesso **não** mudam — o RFC 7807 só trata de erro.
- Testes em `pkg/httputil/response_test.go`
  (`TestWriteError_ProblemJSON_WhenAccepted`,
  `TestWriteError_KeepsEnvelope_WhenProblemJSONNotAccepted`).

## Consequências

- A opção de uma major `/api/v2` com `problem+json` como **default**
  continua aberta para o futuro (bastaria inverter o default por prefixo
  de rota), mas não é mais necessária para conformidade e-PING.
- `openapi.yaml` deve documentar o `Accept` alternativo (pendente).
