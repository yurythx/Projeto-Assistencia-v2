import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NextRequest } from "next/server";

const { getToken } = vi.hoisted(() => ({ getToken: vi.fn() }));
vi.mock("next-auth/jwt", () => ({ getToken }));

// APP_URL (lib/env.ts) é calculado na importação do módulo a partir de
// process.env — por isso cada teste ajusta as env vars e reimporta a
// rota com vi.resetModules(), em vez de importar uma vez só no topo do
// arquivo (que congelaria APP_URL no valor da primeira execução).
async function importRoute() {
  vi.resetModules();
  return import("./route");
}

function fakeRequest(): NextRequest {
  return {} as NextRequest;
}

describe("GET /api/auth/keycloak-logout-url", () => {
  const originalEnv = { ...process.env };

  beforeEach(() => {
    getToken.mockReset();
  });

  afterEach(() => {
    process.env = { ...originalEnv };
  });

  // Achado de auditoria: sem KEYCLOAK_ISSUER_URL configurado (login só
  // local, o caso deste ambiente de dev), a URL de retorno vinha de
  // req.nextUrl.origin — que, atrás do mapeamento de porta do Docker,
  // refletia o bind INTERNO do container (ex. "http://0.0.0.0:3000"),
  // nunca alcançável do navegador. Corrigido para usar sempre APP_URL
  // (lib/env.ts), a mesma fonte de verdade do resto do frontend.
  it("sem Keycloak configurado, devolve APP_URL com o parâmetro de logout amigável", async () => {
    process.env.NEXT_PUBLIC_APP_URL = "http://localhost:3002";
    delete process.env.KEYCLOAK_ISSUER_URL;
    const { GET } = await importRoute();

    const res = await GET(fakeRequest());
    const body = await res.json();

    expect(body.url).toBe("http://localhost:3002/?logout=success");
    expect(getToken).not.toHaveBeenCalled();
  });

  it("com Keycloak configurado mas sem id_token na sessão, também devolve só APP_URL", async () => {
    process.env.NEXT_PUBLIC_APP_URL = "http://localhost:3002";
    process.env.KEYCLOAK_ISSUER_URL = "https://sso.example.gov.br/realms/nexus";
    getToken.mockResolvedValue({ sub: "user-1" }); // sem idToken
    const { GET } = await importRoute();

    const res = await GET(fakeRequest());
    const body = await res.json();

    expect(body.url).toBe("http://localhost:3002/?logout=success");
  });

  it("com Keycloak + id_token, monta a URL de RP-Initiated Logout com post_logout_redirect_uri apontando pra APP_URL", async () => {
    process.env.NEXT_PUBLIC_APP_URL = "http://localhost:3002";
    process.env.KEYCLOAK_ISSUER_URL = "https://sso.example.gov.br/realms/nexus";
    getToken.mockResolvedValue({ sub: "user-1", idToken: "raw-id-token" });
    const { GET } = await importRoute();

    const res = await GET(fakeRequest());
    const body = await res.json();
    const url = new URL(body.url);

    expect(url.origin + url.pathname).toBe(
      "https://sso.example.gov.br/realms/nexus/protocol/openid-connect/logout",
    );
    expect(url.searchParams.get("id_token_hint")).toBe("raw-id-token");
    expect(url.searchParams.get("post_logout_redirect_uri")).toBe(
      "http://localhost:3002/?logout=success",
    );
  });
});
