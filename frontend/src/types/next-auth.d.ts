import type { DefaultSession } from "next-auth";
import "next-auth";
import "next-auth/jwt";

// Campos extras da sessão/JWT do Projeto Nexus (ver lib/auth/options.ts).
declare module "next-auth" {
  interface Session {
    /** "RefreshAccessTokenError" quando a renovação falha — proxy.ts
     * redireciona para /login. */
    error?: string;
    /** "keycloak" (SSO/AD) ou "local" (fallback RS256). */
    provider?: string;
    user?: {
      id?: string;
      /** Roles do token — só exibição; autorização vem de /api/v1/me. */
      roles?: string[];
      /** Grupos do Active Directory (mapper de grupo do Keycloak). */
      groups?: string[];
    } & DefaultSession["user"];
  }

  interface User {
    accessToken?: string;
    accessTokenExpires?: number;
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string;
    accessTokenExpires?: number;
    refreshToken?: string;
    /** id_token_hint do RP-Initiated Logout no Keycloak. */
    idToken?: string;
    provider?: string;
    error?: string;
    roles?: string[];
    groups?: string[];
  }
}
