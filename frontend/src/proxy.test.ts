import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { getToken } = vi.hoisted(() => ({ getToken: vi.fn() }));
vi.mock("next-auth/jwt", () => ({ getToken }));

const { accessTokenUsable } = vi.hoisted(() => ({ accessTokenUsable: vi.fn() }));
vi.mock("@/lib/auth/tokenState", () => ({ accessTokenUsable }));

import { proxy } from "./proxy";

function requestFor(path: string): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3002"));
}

describe("proxy (middleware) — proteção de rota", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Achado de auditoria: um token vencido e um visitante que NUNCA logou
  // caíam na mesma mensagem genérica no /login — "sua sessão expirou" é
  // enganoso pra quem nunca teve sessão nenhuma pra expirar.
  it("token existente mas não utilizável (expirado) — redireciona com reason=session_expired", async () => {
    getToken.mockResolvedValue({ accessToken: "expired-token" });
    accessTokenUsable.mockReturnValue(false);

    const res = await proxy(requestFor("/dashboard"));

    expect(res.status).toBe(307);
    const location = new URL(res.headers.get("location")!);
    expect(location.pathname).toBe("/login");
    expect(location.searchParams.get("reason")).toBe("session_expired");
    expect(location.searchParams.get("callbackUrl")).toBe("/dashboard");
  });

  it("nenhum token (nunca logou) — redireciona SEM reason=session_expired", async () => {
    getToken.mockResolvedValue(null);
    accessTokenUsable.mockReturnValue(false);

    const res = await proxy(requestFor("/dashboard"));

    const location = new URL(res.headers.get("location")!);
    expect(location.pathname).toBe("/login");
    expect(location.searchParams.get("reason")).toBeNull();
  });

  it("token utilizável — deixa passar, sem redirecionar", async () => {
    getToken.mockResolvedValue({ accessToken: "valid-token" });
    accessTokenUsable.mockReturnValue(true);

    const res = await proxy(requestFor("/dashboard"));

    expect(res.status).not.toBe(307);
    expect(res.headers.get("location")).toBeNull();
  });

  it("rota pública — nunca chama getToken", async () => {
    const res = await proxy(requestFor("/sobre"));

    expect(getToken).not.toHaveBeenCalled();
    expect(res.headers.get("location")).toBeNull();
  });
});
