import type { NextAuthOptions, Session } from "next-auth";
import type { JWT } from "next-auth/jwt";
import CredentialsProvider from "next-auth/providers/credentials";
import KeycloakProvider from "next-auth/providers/keycloak";

import { forwardedFor } from "@/lib/api/clientIp";
import { BACKEND_INTERNAL_URL } from "@/lib/env";

// Autenticação do Projeto Nexus (skill §2):
//  1. Padrão ouro — Keycloak DEDICADO (OIDC Authorization Code + PKCE),
//     com usuários federados do Active Directory via LDAP/LDAPS;
//  2. Fallback local — POST /api/v1/auth/login no backend Go, que emite
//     um JWT RS256 próprio (contingência / ambiente isolado).
//
// O frontend NÃO decide autorização: roles e grupos do token servem só
// para exibição; permissões efetivas ("recurso:ação") e lotações vêm de
// GET /api/v1/me, resolvidas pelo IAM do backend a cada requisição.

// Somente servidor — nunca NEXT_PUBLIC_* (segredos não chegam ao navegador).
const issuer = process.env.KEYCLOAK_ISSUER_URL;
const clientId = process.env.KEYCLOAK_FRONTEND_CLIENT_ID;
const clientSecret = process.env.KEYCLOAK_FRONTEND_CLIENT_SECRET;

export const keycloakEnabled = Boolean(issuer && clientId && clientSecret);

if (!keycloakEnabled) {
  console.warn(
    "Keycloak não configurado (KEYCLOAK_ISSUER_URL / KEYCLOAK_FRONTEND_CLIENT_ID / KEYCLOAK_FRONTEND_CLIENT_SECRET) — apenas o login local estará disponível.",
  );
}

interface KeycloakTokenResponse {
  access_token: string;
  refresh_token?: string;
  id_token?: string;
  expires_in: number;
  error?: string;
}

/** Renova o access token do Keycloak com o refresh_token da sessão. */
async function refreshAccessToken(token: JWT): Promise<JWT> {
  try {
    const response = await fetch(`${issuer}/protocol/openid-connect/token`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({
        client_id: clientId!,
        client_secret: clientSecret!,
        grant_type: "refresh_token",
        refresh_token: token.refreshToken as string,
      }),
      signal: AbortSignal.timeout(5000),
    });
    const refreshed: KeycloakTokenResponse = await response.json();
    if (!response.ok || refreshed.error || !refreshed.access_token) {
      throw new Error("refresh recusado");
    }
    const ttlSeconds = Number(refreshed.expires_in);
    if (!Number.isFinite(ttlSeconds) || ttlSeconds <= 0) {
      throw new Error("expires_in inválido");
    }
    return {
      ...token,
      accessToken: refreshed.access_token,
      accessTokenExpires: Date.now() + ttlSeconds * 1000,
      refreshToken: refreshed.refresh_token ?? token.refreshToken,
      idToken: refreshed.id_token ?? token.idToken,
      error: undefined,
    };
  } catch {
    // Nunca logar o erro bruto (pode ecoar o refresh_token).
    console.error("Falha ao renovar o access token do Keycloak");
    return { ...token, error: "RefreshAccessTokenError" };
  }
}

interface LocalLoginResponse {
  data: {
    access_token: string;
    expires_at: string;
    user: { id: string; username: string; email: string; display_name?: string };
  } | null;
  error: { code: string; message: string } | null;
}

/** Decodifica (sem validar — a validação é do backend) as claims usadas
 * só para exibição: roles e grupos do AD. */
function decodeDisplayClaims(accessToken: string): { roles: string[]; groups: string[] } {
  try {
    const part = accessToken.split(".")[1];
    if (!part) return { roles: [], groups: [] };
    const payload = JSON.parse(Buffer.from(part, "base64url").toString("utf-8"));
    const roles = new Set<string>();
    for (const r of payload.roles ?? []) roles.add(String(r));
    for (const r of payload.realm_access?.roles ?? []) roles.add(String(r));
    if (clientId) for (const r of payload.resource_access?.[clientId]?.roles ?? []) roles.add(String(r));
    const groups = Array.isArray(payload.groups) ? payload.groups.map(String) : [];
    return { roles: [...roles], groups };
  } catch {
    return { roles: [], groups: [] };
  }
}

export const authOptions: NextAuthOptions = {
  providers: [
    ...(keycloakEnabled
      ? [
          KeycloakProvider({
            issuer: issuer!,
            clientId: clientId!,
            clientSecret: clientSecret!,
          }),
        ]
      : []),
    CredentialsProvider({
      id: "local",
      name: "Conta local",
      credentials: {
        username: { label: "Usuário", type: "text" },
        password: { label: "Senha", type: "password" },
      },
      async authorize(credentials, req) {
        if (!credentials?.username || !credentials?.password) return null;
        let res: Response;
        try {
          res = await fetch(`${BACKEND_INTERNAL_URL}/api/v1/auth/login`, {
            method: "POST",
            // Sem o IP do visitante, a API via este servidor como a origem de
            // TODOS os logins: o limite (10/min) e o bloqueio por IP viravam
            // um balde só para a organização inteira.
            headers: { "Content-Type": "application/json", ...forwardedFor(req?.headers) },
            body: JSON.stringify({ username: credentials.username, password: credentials.password }),
            signal: AbortSignal.timeout(8000),
          });
        } catch {
          console.error("Login local: backend inacessível");
          return null;
        }
        // Antes do corpo: o 429 do rate limiter nem sempre vem em JSON.
        if (res.status === 429) {
          throw new Error("TooManyAttempts");
        }
        let body: LocalLoginResponse;
        try {
          body = await res.json();
        } catch {
          return null;
        }
        if (!res.ok || !body.data) return null;
        return {
          id: body.data.user.id,
          name: body.data.user.display_name || body.data.user.username,
          email: body.data.user.email,
          accessToken: body.data.access_token,
          accessTokenExpires: new Date(body.data.expires_at).getTime(),
        };
      },
    }),
  ],

  session: { strategy: "jwt" },

  callbacks: {
    async jwt({ token, account, user }) {
      if (account?.provider === "keycloak") {
        token.accessToken = account.access_token;
        token.accessTokenExpires = account.expires_at ? account.expires_at * 1000 : undefined;
        token.refreshToken = account.refresh_token;
        token.idToken = account.id_token;
        token.provider = "keycloak";
      } else if (account?.provider === "local" && user) {
        token.accessToken = user.accessToken;
        token.accessTokenExpires = user.accessTokenExpires;
        token.refreshToken = undefined;
        token.idToken = undefined;
        token.provider = "local";
      }
      if (typeof token.accessToken === "string") {
        const claims = decodeDisplayClaims(token.accessToken);
        token.roles = claims.roles;
        token.groups = claims.groups;
      }
      if (token.accessTokenExpires && Date.now() < token.accessTokenExpires) {
        return token;
      }
      if (token.refreshToken) {
        return refreshAccessToken(token);
      }
      return { ...token, error: "RefreshAccessTokenError" };
    },

    async session({ session, token }): Promise<Session> {
      session.user = session.user ?? {};
      session.user.roles = token.roles ?? [];
      session.user.groups = token.groups ?? [];
      session.provider = token.provider;
      session.error = token.error;
      return session;
    },
  },

  pages: { signIn: "/login" },
};
