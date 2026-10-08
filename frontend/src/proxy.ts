import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getToken } from "next-auth/jwt";

import { accessTokenUsable } from "@/lib/auth/tokenState";
import { API_PUBLIC_URL, MINIO_PUBLIC_URL, WS_PUBLIC_URL, toOrigin } from "@/lib/env";

// O Next.js 16 renomeou a convenção middleware.ts para proxy.ts (mesmo
// mecanismo, só o nome do arquivo/export mudou). Esta função tem duas
// responsabilidades:
//
// 1. Content-Security-Policy com nonce (item de segurança prioritário):
//    gera um nonce novo a cada requisição e o injeta tanto no cabeçalho
//    CSP quanto num cabeçalho interno x-nonce que o Next.js lê
//    automaticamente para carimbar os próprios scripts/estilos do
//    framework — nada no código da aplicação precisa passar o nonce
//    manualmente. Isso exige renderização dinâmica (não hidrata bem com
//    páginas estáticas em cache), aceitável aqui pois é um dashboard
//    interno, não um site de alto tráfego dependente de cache em CDN.
//
// 2. Proteção de rota: redireciona visitas não autenticadas a qualquer
//    seção protegida (PROTECTED_PREFIXES) para /login, sem deixar a
//    página nem começar a renderizar.
// Seções autenticadas — tudo o que vive em app/(protected)/. O grupo de
// rotas não aparece na URL, então cada prefixo é listado aqui. Módulos
// plugáveis entram mesmo desativados: o 404 do módulo inativo é decidido
// pela API (Guard do Kernel), nunca pelo proxy.
const PROTECTED_PREFIXES = [
  "/dashboard",
  "/configuracao",
  "/monitoramento",
  "/auditoria",
  "/perfil",
  "/busca",
  "/mercurio",
  "/blog",
  "/wiki",
  "/agenda",
  "/diretorio",
  "/arquivos",
  "/signum",
  "/tramite",
  "/atlas",
  "/gestao",
  "/exemplos",
];

export async function proxy(request: NextRequest) {
  const nonce = Buffer.from(crypto.randomUUID()).toString("base64");
  const isDev = process.env.NODE_ENV === "development";

  // Origens que o navegador acessa diretamente (WebSocket de notificações,
  // MinIO via URL pré-assinada, e a própria API). Deduplicadas — API e WS
  // costumam ser o mesmo host:porta em esquemas diferentes.
  const apiOrigin = toOrigin(API_PUBLIC_URL);
  // Capas do Blog e miniaturas vêm do MinIO por URL pré-assinada.
  const minioOrigin = MINIO_PUBLIC_URL ? toOrigin(MINIO_PUBLIC_URL) : undefined;
  const connectOrigins = [
    ...new Set(
      [
        WS_PUBLIC_URL && toOrigin(WS_PUBLIC_URL),
        MINIO_PUBLIC_URL && toOrigin(MINIO_PUBLIC_URL),
        apiOrigin,
        apiOrigin?.replace(/^http/, "ws"),
      ].filter(Boolean) as string[],
    ),
  ].join(" ");

  // VLibras (widget oficial de tradução para Libras — Governo Federal, ver
  // components/accessibility/VLibrasWidget.tsx) roda um player em WebAssembly.
  // O loader vlibras-plugin.js faz 302 para
  // https://cdn.jsdelivr.net/gh/spbgovbr-vlibras/... — mas com 'strict-dynamic'
  // no script-src isso NÃO precisa de host allowlist: o <script> nonce-ado
  // que o next/script injeta é confiável e propaga a confiança ao script que
  // ele carrega (mesmo após o redirect) e ao que ESSE injeta (o player
  // Unity). Só o WASM exige 'wasm-unsafe-eval'.
  //
  // S-01/S-02 da auditoria: o script-src volta a ser nonce + strict-dynamic
  // (sem 'unsafe-inline'/'unsafe-eval' em produção). style-src continua com
  // 'unsafe-inline' — o VLibras injeta <style> sem nonce e um XSS de estilo
  // é muito menos grave que um de script.
  const vlibrasHosts = "https://vlibras.gov.br https://*.vlibras.gov.br https://cdn.jsdelivr.net";
  const connectSrcDev = isDev ? " http: https: ws: wss:" : "";
  const cspHeader = `
    default-src 'self';
    script-src 'self' 'nonce-${nonce}' 'strict-dynamic' 'wasm-unsafe-eval'${isDev ? " 'unsafe-eval'" : ""};
    style-src 'self' 'unsafe-inline' ${vlibrasHosts};
    img-src 'self' blob: data: ${vlibrasHosts}${minioOrigin ? ` ${minioOrigin}` : ""};
    font-src 'self' data: ${vlibrasHosts};
    connect-src 'self' blob: data: ${vlibrasHosts}${connectOrigins ? ` ${connectOrigins}` : ""}${connectSrcDev};
    worker-src 'self' blob: data: https://vlibras.gov.br https://*.vlibras.gov.br;
    child-src 'self' blob: data: https://vlibras.gov.br https://*.vlibras.gov.br;
    frame-src 'self' blob: data: https://vlibras.gov.br https://*.vlibras.gov.br;
    object-src 'none';
    base-uri 'self';
    form-action 'self';
    frame-ancestors 'none';
  `
    .replace(/\s{2,}/g, " ")
    .trim();

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", cspHeader);

  const isProtectedRoute = PROTECTED_PREFIXES.some(
    (prefix) => request.nextUrl.pathname === prefix || request.nextUrl.pathname.startsWith(prefix + "/"),
  );
  if (isProtectedRoute) {
    const token = await getToken({ req: request });
    // accessTokenUsable cobre também o token VENCIDO (o login local não
    // renova e o callback jwt não roda em toda navegação) — sem isso o
    // usuário ficava "logado" numa tela que só recebe 401 do backend.
    if (!accessTokenUsable(token)) {
      const loginUrl = new URL("/login", request.url);
      loginUrl.searchParams.set("callbackUrl", request.nextUrl.pathname);
      // token não-nulo aqui significa que EXISTIA uma sessão (o cookie
      // decriptografou), só não é mais utilizável — diferente de nunca
      // ter feito login. Só neste caso faz sentido dizer "sua sessão
      // expirou": um visitante que nunca logou não teve sessão nenhuma
      // pra expirar (achado de auditoria: LoginCard mostra esse aviso a
      // qualquer um que caísse aqui, incluindo quem só digitou a URL
      // direto sem nunca ter entrado).
      if (token) {
        loginUrl.searchParams.set("reason", "session_expired");
      }
      return NextResponse.redirect(loginUrl);
    }
  }

  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set("Content-Security-Policy", cspHeader);
  return response;
}

export const config = {
  matcher: [
    // Roda em toda página HTML, exceto rotas de API, assets estáticos
    // do Next e favicon — essas nunca precisam de CSP de página nem de
    // verificação de sessão.
    {
      source: "/((?!api|_next/static|_next/image|favicon.ico).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};

export default proxy;
