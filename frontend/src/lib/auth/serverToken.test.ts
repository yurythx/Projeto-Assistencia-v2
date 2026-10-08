// @vitest-environment node
import { describe, expect, it, vi } from "vitest";

const { getToken } = vi.hoisted(() => ({ getToken: vi.fn() }));
vi.mock("next-auth/jwt", () => ({ getToken }));
vi.mock("next/headers", () => ({
  cookies: async () => ({ getAll: () => [{ name: "next-auth.session-token", value: "cifrado" }] }),
  headers: async () => new Headers({ host: "app" }),
}));

import { accessTokenUsable } from "./tokenState";
import { getServerToken } from "./serverToken";

describe("getServerToken", () => {
  it("monta um req mínimo (cookies e cabeçalhos) para o getToken", async () => {
    getToken.mockResolvedValue({ accessToken: "t" });
    expect(await getServerToken()).toEqual({ accessToken: "t" });
    expect(getToken.mock.calls[0]![0].req).toEqual({ cookies: { "next-auth.session-token": "cifrado" }, headers: { host: "app" } });
  });
});

describe("accessTokenUsable", () => {
  it("sem token, com erro, vencido (com folga) ou sem vencimento conhecido", () => {
    expect(accessTokenUsable(null)).toBe(false);
    expect(accessTokenUsable({ accessToken: "" })).toBe(false);
    expect(accessTokenUsable({ accessToken: "t", error: "RefreshAccessTokenError" })).toBe(false);
    expect(accessTokenUsable({ accessToken: "t", accessTokenExpires: Date.now() + 2000 })).toBe(false);
    expect(accessTokenUsable({ accessToken: "t", accessTokenExpires: Date.now() + 60_000 })).toBe(true);
    expect(accessTokenUsable({ accessToken: "t" })).toBe(true);
  });
});
