// Fonte única e parametrizada das URLs de ambiente do front. Antes cada
// arquivo tinha o seu próprio `process.env.X || "http://localhost:8002"`
// hardcoded — e defasado quando a stack mudou de porta (S-04 da auditoria).
// Aqui o default vive num lugar só e acompanha a porta real da stack.
//
// Edge-safe: só lê `process.env` (sem APIs de Node), então pode ser
// importado por `proxy.ts` (middleware) e por Server/Client Components.

/** Origem pública da API Go (browser fala com ela só via WebSocket direto;
 * o REST passa pelo proxy BFF same-origin). */
export const API_PUBLIC_URL =
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "http://localhost:8003";

/** Endpoint público do WebSocket de notificações (ws(s)://host:porta/ws). */
export const WS_PUBLIC_URL =
  process.env.NEXT_PUBLIC_WS_URL || `${API_PUBLIC_URL.replace(/^http/, "ws")}/ws`;

/** Origem do MinIO/S3 — o browser faz PUT/GET direto nas URLs pré-assinadas. */
export const MINIO_PUBLIC_URL =
  process.env.NEXT_PUBLIC_MINIO_URL?.replace(/\/$/, "") || "http://localhost:9004";

/** Endereço interno do backend Go, usado só server-side (Server Components
 * e o proxy BFF). Nunca chega ao browser. */
export const BACKEND_INTERNAL_URL =
  process.env.BACKEND_INTERNAL_URL?.replace(/\/$/, "") ||
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") ||
  "http://localhost:8003";

/** Origem da própria app — usada só quando `request()` de lib/api/client
 * roda em contexto de servidor (sem `window`). */
export const APP_URL =
  process.env.NEXT_PUBLIC_APP_URL?.replace(/\/$/, "") ||
  process.env.NEXTAUTH_URL?.replace(/\/$/, "") ||
  "http://localhost:3003";

/** "esquema://host:porta" de uma URL (descarta path) — formato exigido por
 * `connect-src` da CSP. */
export function toOrigin(url: string | undefined | null): string | null {
  if (!url) return null;
  try {
    const u = new URL(url);
    return `${u.protocol}//${u.host}`;
  } catch {
    return null;
  }
}
