import { NextRequest, NextResponse } from "next/server";
import { getToken } from "next-auth/jwt";

import { BACKEND_INTERNAL_URL as BACKEND_URL } from "@/lib/api/backendUrl";
import { forwardedFor } from "@/lib/api/clientIp";
import { accessTokenUsable } from "@/lib/auth/tokenState";

// Proxy BFF (Backend For Frontend): toda chamada de Client Component à
// API Go passa por aqui, em vez de carregar um bearer token no JavaScript
// executado no navegador. O access token real é lido no lado do servidor
// a partir do cookie de sessão criptografado e anexado aqui — ele nunca
// chega ao cliente (§30/§57). Páginas que buscam dados em Server
// Component (§ Migração pra Server Components) não passam por aqui — ver
// lib/api/server.ts, que fala com o backend direto, sem o hop de rede
// extra que uma chamada vinda do navegador precisa.

// proxy encaminha a requisição para GET /api/{path} ou POST /api/{path} na
// API Go, injetando o Authorization: Bearer e propagando/gerando o
// X-Request-ID (§50) para que a chamada seja correlacionável nos logs do
// backend mesmo tendo passado por este proxy intermediário.
//
// {path} já vem com o prefixo de versão incluído (ex.: chamadas do
// frontend usam apiClient.get("v1/integrations")) — este proxy NÃO deve
// prepender "v1/" de novo, senão o alvo vira /api/v1/v1/integrations e
// toda chamada volta 404 (bug real encontrado em produção: o dashboard
// inteiro ficava sem dados porque cada requisição batia nesse path
// duplicado).
async function proxy(req: NextRequest, path: string[]): Promise<NextResponse> {
  const token = await getToken({ req });
  // accessTokenUsable também rejeita um token VENCIDO (não só um marcado
  // com erro) — ver lib/auth/tokenState.ts: sem isso, uma sessão local
  // expirada continuava repassando o bearer morto e o backend respondia
  // 401 em looping.
  if (!accessTokenUsable(token)) {
    return NextResponse.json(
      { data: null, error: { code: "UNAUTHORIZED", message: "sessão expirada — faça login novamente" } },
      { status: 401 },
    );
  }

  // Previne duplicação caso 'api' ou 'backend' venha nos primeiros elementos do path
  const cleanPathSegments = [...path];
  while (cleanPathSegments.length > 0 && (cleanPathSegments[0] === "api" || cleanPathSegments[0] === "backend")) {
    cleanPathSegments.shift();
  }
  // ".." normalizaria para fora de /api (ex.: /api/../metrics vira /metrics)
  // levando o bearer do usuário junto — mesma guarda do proxy público.
  if (cleanPathSegments.some((seg) => seg === ".." || seg === "." || seg.includes("/") || seg.includes("\\"))) {
    return NextResponse.json({ data: null, error: { code: "NOT_FOUND", message: "rota inexistente" } }, { status: 404 });
  }
  const targetUrl = new URL(`/api/${cleanPathSegments.join("/")}`, BACKEND_URL);
  targetUrl.search = req.nextUrl.search;

  const requestId = req.headers.get("x-request-id") ?? crypto.randomUUID();

  const upstreamHeaders: Record<string, string> = {
    Authorization: `Bearer ${token.accessToken}`,
    "Content-Type": req.headers.get("content-type") ?? "application/json",
    "X-Request-ID": requestId,
    // IP real (prova de consentimento LGPD, auditoria, rate limit) — só
    // atrás de proxy de borda confiável; ver lib/api/clientIp.
    ...forwardedFor(req.headers),
  };
  // Chave de idempotência das mutações (skill §1 / OWASP A08): gerada no
  // navegador por apiClient e repassada intacta — o backend deduplica
  // por usuário + chave e devolve a resposta original num reenvio.
  const idempotencyKey = req.headers.get("x-idempotency-key");
  if (idempotencyKey && /^[A-Za-z0-9_-]{8,128}$/.test(idempotencyKey)) upstreamHeaders["X-Idempotency-Key"] = idempotencyKey;

  const init: RequestInit = {
    method: req.method,
    headers: upstreamHeaders,
    // GET/HEAD não podem carregar um corpo. arrayBuffer(), não text():
    // um corpo multipart/form-data com upload de arquivo (Fase 10 —
    // projeto criado por .zip) carrega bytes binários — .text() decodifica
    // como UTF-8 e corrompe qualquer byte que não seja uma sequência UTF-8
    // válida (troca por U+FFFD), inutilizando o .zip do outro lado.
    // arrayBuffer() encaminha os bytes exatamente como chegaram, correto
    // tanto pra esse caso quanto pro JSON de sempre (texto também
    // sobrevive ileso a um round-trip por bytes).
    body: ["GET", "HEAD"].includes(req.method) ? undefined : await req.arrayBuffer(),
  };

  let backendResponse: Response;
  try {
    backendResponse = await fetch(targetUrl, init);
  } catch {
    return NextResponse.json(
      {
        data: null,
        error: { code: "DEPENDENCY_UNAVAILABLE", message: "a API está indisponível no momento" },
      },
      { status: 503 },
    );
  }

  // Respostas binárias (GET .../package.zip e .../package.pdf — o
  // compilador de pacote de uma demanda) NÃO podem passar por .text(): o decode UTF-8
  // troca cada byte inválido por U+FFFD e corrompe o binário. Encaminha
  // os bytes crus nesse caso e em todo texto que não é JSON: o decode
  // também REMOVE o BOM do início — o CSV da TTDD (GET
  // .../atlas/ttdd/exportar) chegava sem ele e o Excel em português abria
  // os acentos errados.
  const upstreamContentType = backendResponse.headers.get("content-type") ?? "application/json";
  const body = upstreamContentType.startsWith("application/json")
    ? await backendResponse.text()
    : await backendResponse.arrayBuffer();
  const headers: Record<string, string> = {
    "Content-Type": upstreamContentType,
    "X-Request-ID": requestId,
  };
  // Content-Disposition (Fase 14 — Maturidade de AppSec: exportação CSV,
  // GET .../findings.csv e .../findings-history.csv): sem propagar isto,
  // o navegador abre o CSV cru na aba em vez de baixar com o nome de
  // arquivo que o backend já escolheu (ver transport/csv_export.go) — o
  // único header, além de Content-Type, que algum endpoint hoje depende
  // de atravessar o proxy intacto.
  const contentDisposition = backendResponse.headers.get("content-disposition");
  if (contentDisposition) {
    headers["Content-Disposition"] = contentDisposition;
  }
  // 204/205/304 não podem ter corpo — nem string vazia: o construtor de
  // Response lança TypeError e a chamada virava 500 (ex.: POST
  // .../mercurio/rooms/{id}/read, que responde 204).
  return new NextResponse(isNullBodyStatus(backendResponse.status) ? null : body, {
    status: backendResponse.status,
    headers,
  });
}

function isNullBodyStatus(status: number): boolean {
  return status === 204 || status === 205 || status === 304;
}

export async function GET(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  return proxy(req, path);
}

export async function POST(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  return proxy(req, path);
}

// PATCH: usado hoje só por Configurações > Feature flags
// (PATCH /api/v1/admin/feature-flags/{key}) — mesmo encaminhamento dos
// outros métodos, sem lógica própria.
export async function PATCH(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  return proxy(req, path);
}

// PUT/DELETE: nenhum módulo atual os usa ainda (ver apiClient.put/.delete
// em lib/api/client.ts) — encaminhados por completude do proxy, mesmo
// esquema dos outros métodos, sem lógica própria.
export async function PUT(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  return proxy(req, path);
}

export async function DELETE(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  return proxy(req, path);
}
