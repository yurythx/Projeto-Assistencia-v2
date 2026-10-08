import type { JWT } from "next-auth/jwt";

// Fonte única de verdade para "esta sessão ainda pode chamar o backend?".
// Usada pelo middleware (proxy.ts), pelo proxy BFF
// (app/api/backend/[...path]/route.ts) e pela busca em Server Component
// (lib/api/server.ts) — os três liam o token com getToken(), que apenas
// DESCRIPTOGRAFA o cookie de sessão e NÃO roda o callback `jwt` de
// options.ts. Consequência: para o login LOCAL (sem refresh token), o
// callback `jwt` só carimba `token.error` na próxima vez que
// /api/auth/session é chamado — até lá o cookie guarda um accessToken
// vencido, sem marca de erro, e todos os três caminhos repassavam esse
// bearer expirado ao backend. O resultado era uma enxurrada de 401
// ("invalid or expired access token") sem nunca redirecionar o usuário
// para /login. Checar o vencimento aqui fecha essa janela.

// Margem de segurança: trata como expirado alguns segundos antes do
// vencimento real, para não repassar um token que morre no meio do voo
// da requisição ao backend.
const EXPIRY_SKEW_MS = 5_000;

export function accessTokenUsable(token: JWT | null | undefined): token is JWT {
  if (!token || !token.accessToken || token.error) {
    return false;
  }
  const expiresAt = token.accessTokenExpires;
  if (typeof expiresAt === "number" && Number.isFinite(expiresAt)) {
    return Date.now() < expiresAt - EXPIRY_SKEW_MS;
  }
  // Sem vencimento conhecido no token (ex.: sessão Keycloak sem
  // expires_at) — deixa o backend ser o juiz e não bloqueia aqui.
  return true;
}
