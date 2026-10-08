import { NextRequest, NextResponse } from "next/server";
import { getToken } from "next-auth/jwt";

import { APP_URL } from "@/lib/env";

// Logout completo (RP-Initiated Logout, OIDC): sem isto, o botão "Sair"
// só limpa o cookie de sessão local do NextAuth — a sessão continua viva
// no próprio Keycloak, então qualquer aba/dispositivo que ainda tenha um
// refresh token válido, ou um "silent login" (prompt=none), reautentica
// o usuário sem pedir senha de novo. É preciso mandar o NAVEGADOR visitar
// o endpoint de logout do Keycloak (não dá pra fazer isso só no servidor
// — é o Keycloak quem precisa limpar os próprios cookies de sessão dele
// no domínio dele).
//
// Esta rota só monta e devolve a URL; quem efetivamente desloga do
// Keycloak é o navegador ao navegar até ela. É chamada ANTES de
// signOut() no client, porque depois que a sessão local for limpa não
// há mais como ler o id_token daqui.
//
// appUrl vem de lib/env.ts (APP_URL), NUNCA de req.nextUrl.origin —
// achado de auditoria: atrás do mapeamento de porta do Docker
// (${FRONTEND_PORT}:3000), req.nextUrl.origin refletia o bind interno do
// container (ex.: "http://0.0.0.0:3000"), não a URL pública — o botão
// "Sair" mandava o navegador para um endereço inalcançável de fora do
// container, resultando na "página não encontrada" que deveria ser a
// home. APP_URL é a mesma fonte única de verdade que o resto do frontend
// já usa para isto.
//
// ?logout=success: o AuthFlashToast (components/layout/AuthFlashToast.tsx),
// montado na home pública, lê este parâmetro pra mostrar a mensagem
// amigável de "você saiu com segurança" — sem isto o logout terminava
// numa tela muda, sem nenhuma confirmação visível de que funcionou.
function withLogoutFlag(url: string): string {
  const u = new URL(url);
  u.searchParams.set("logout", "success");
  return u.toString();
}

export async function GET(req: NextRequest) {
  const issuer = process.env.KEYCLOAK_ISSUER_URL;
  const appUrl = withLogoutFlag(APP_URL);

  if (!issuer) {
    return NextResponse.json({ url: appUrl });
  }

  const token = await getToken({ req });
  if (!token?.idToken) {
    // Sem sessão (ou sem id_token guardado) — não há o que encerrar no
    // Keycloak; volta direto para a aplicação.
    return NextResponse.json({ url: appUrl });
  }

  const logoutUrl = new URL(`${issuer}/protocol/openid-connect/logout`);
  logoutUrl.searchParams.set("id_token_hint", token.idToken as string);
  logoutUrl.searchParams.set("post_logout_redirect_uri", appUrl);

  return NextResponse.json({ url: logoutUrl.toString() });
}
