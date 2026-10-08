import { NextRequest, NextResponse } from "next/server";

import { BACKEND_INTERNAL_URL } from "@/lib/api/backendUrl";
import { forwardedFor } from "@/lib/api/clientIp";

// Proxy BFF ANÔNIMO — só para as rotas públicas do backend, numa
// allowlist explícita (método + prefixo). Qualquer outra rota responde
// 404 aqui: este proxy nunca vira um atalho para a API autenticada.
// O IP real do visitante segue no X-Forwarded-For para o rate limit por
// IP do backend — só atrás de proxy de borda confiável (lib/api/clientIp).
const ALLOW: { method: string; prefix: string }[] = [
  { method: "GET", prefix: "v1/branding" },
  { method: "GET", prefix: "v1/system/public-modules" },
  { method: "GET", prefix: "v1/catalog/" },
  { method: "GET", prefix: "v1/directory/public/" },
  { method: "GET", prefix: "v1/calendar/public/" },
  { method: "GET", prefix: "v1/signum/verify/" },
  { method: "GET", prefix: "v1/transparencia/" },
  { method: "POST", prefix: "v1/contact/messages" },
  { method: "POST", prefix: "v1/lgpd/accept-anon" },
  // Atlas (ADR 026): TTDD, procedimentos e modelos — só leitura; a gestão
  // (v1/atlas/admin) e o assistente ficam de fora.
  { method: "GET", prefix: "v1/atlas/ttdd" },
  { method: "GET", prefix: "v1/atlas/workflows" },
  { method: "GET", prefix: "v1/atlas/modelos" },
];

async function forward(req: NextRequest, path: string[]): Promise<NextResponse> {
  const joined = path.join("/");
  if (
    joined.includes("..") ||
    !ALLOW.some((a) => a.method === req.method && joined.startsWith(a.prefix))
  ) {
    return NextResponse.json(
      { data: null, error: { code: "NOT_FOUND", message: "rota pública inexistente" } },
      { status: 404 },
    );
  }
  const target = new URL(`/api/${joined}`, BACKEND_INTERNAL_URL);
  target.search = req.nextUrl.search;
  const headers: Record<string, string> = {
    "Content-Type": req.headers.get("content-type") ?? "application/json",
    "X-Request-ID": req.headers.get("x-request-id") ?? crypto.randomUUID(),
    ...forwardedFor(req.headers),
  };
  try {
    const res = await fetch(target, {
      method: req.method,
      headers,
      body: req.method === "GET" ? undefined : await req.arrayBuffer(),
      signal: AbortSignal.timeout(10000),
    });
    // 204/205/304 não podem ter corpo (o construtor de Response lança).
    const noBody = res.status === 204 || res.status === 205 || res.status === 304;
    const contentType = res.headers.get("content-type") ?? "application/json";
    // Arquivos (modelo do Atlas, CSV da TTDD) passam em bytes, com o nome:
    // .text() corromperia o binário e tiraria o BOM do CSV.
    const respHeaders: Record<string, string> = { "Content-Type": contentType };
    const disposition = res.headers.get("content-disposition");
    if (disposition) respHeaders["Content-Disposition"] = disposition;
    const body = contentType.startsWith("application/json")
      ? await res.text()
      : await res.arrayBuffer();
    return new NextResponse(noBody ? null : body, { status: res.status, headers: respHeaders });
  } catch {
    return NextResponse.json(
      {
        data: null,
        error: { code: "DEPENDENCY_UNAVAILABLE", message: "serviço indisponível no momento" },
      },
      { status: 503 },
    );
  }
}

export async function GET(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return forward(req, (await params).path);
}

export async function POST(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return forward(req, (await params).path);
}
