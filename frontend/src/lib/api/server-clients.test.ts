// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";

vi.mock("server-only", () => ({}));
vi.mock("@/lib/env", () => ({ BACKEND_INTERNAL_URL: "http://api:8080", APP_URL: "http://app" }));
const { getServerToken } = vi.hoisted(() => ({ getServerToken: vi.fn() }));
vi.mock("@/lib/auth/serverToken", () => ({ getServerToken }));

import { ApiError } from "@/lib/api/client";
import { publicFetch, publicPost } from "@/lib/api/publicClient";
import { PublicApiError, publicGet, publicGetOr } from "@/lib/api/publicServer";
import { serverApiGet } from "@/lib/api/server";

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

let f: ReturnType<typeof vi.fn>;
beforeEach(() => {
  f = vi.fn();
  vi.stubGlobal("fetch", f);
  getServerToken.mockReset().mockResolvedValue({ accessToken: "tok", accessTokenExpires: Date.now() + 60_000 });
});
afterEach(() => vi.unstubAllGlobals());

describe("serverApiGet (Server Components)", () => {
  it("chama a API direto com o bearer e valida o contrato", async () => {
    f.mockResolvedValueOnce(json({ data: { n: 1 }, meta: { page: 1 }, error: null }));
    const out = await serverApiGet("v1/x", z.object({ n: z.number() }));
    expect(out).toEqual({ data: { n: 1 }, meta: { page: 1 } });
    const [url, init] = f.mock.calls[0]!;
    expect(String(url)).toBe("http://api:8080/api/v1/x");
    expect(init).toMatchObject({ headers: { Authorization: "Bearer tok" }, cache: "no-store" });

    f.mockResolvedValueOnce(json({ data: { n: "um" }, error: null }));
    await expect(serverApiGet("v1/x", z.object({ n: z.number() }))).rejects.toMatchObject({ status: 502, code: "INVALID_RESPONSE" });
    f.mockResolvedValueOnce(json({ data: [1], error: null }));
    expect((await serverApiGet("v1/x")).data).toEqual([1]);
  });

  it("sessão vencida, API fora, corpo ilegível e erro do backend", async () => {
    getServerToken.mockResolvedValueOnce(null);
    await expect(serverApiGet("v1/x")).rejects.toMatchObject({ status: 401, code: "UNAUTHORIZED" });
    f.mockRejectedValueOnce(new Error("down"));
    await expect(serverApiGet("v1/x")).rejects.toMatchObject({ status: 503 });
    f.mockResolvedValueOnce(new Response("<html>", { status: 500 }));
    await expect(serverApiGet("v1/x")).rejects.toMatchObject({ code: "INVALID_RESPONSE" });
    f.mockResolvedValueOnce(json({ data: null, error: { code: "FORBIDDEN", message: "não" } }, 403));
    await expect(serverApiGet("v1/x")).rejects.toMatchObject({ status: 403, code: "FORBIDDEN", message: "não" });
    f.mockResolvedValueOnce(json({ data: null }, 500));
    const err = await serverApiGet("v1/x").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ code: "UNKNOWN_ERROR" });
  });
});

describe("publicGet / publicGetOr (site institucional)", () => {
  it("rota pública sem token, com revalidação", async () => {
    f.mockResolvedValueOnce(json({ data: { a: 1 }, error: null }));
    expect(await publicGet("/catalog/services", 30)).toEqual({ data: { a: 1 }, meta: undefined });
    const [url, init] = f.mock.calls[0]!;
    expect(String(url)).toBe("http://api:8080/api/v1/catalog/services");
    expect(init.next).toEqual({ revalidate: 30 });
    expect(init.headers).toBeUndefined();
  });

  it("erros viram PublicApiError; publicGetOr cai no fallback", async () => {
    f.mockRejectedValueOnce(new Error("down"));
    await expect(publicGet("x")).rejects.toMatchObject({ status: 503, code: "DEPENDENCY_UNAVAILABLE" });
    f.mockResolvedValueOnce(new Response("x", { status: 502 }));
    await expect(publicGet("x")).rejects.toMatchObject({ code: "INVALID_RESPONSE" });
    f.mockResolvedValueOnce(json({ data: null, error: { code: "MODULE_DISABLED", message: "desativado" } }, 404));
    const e = await publicGet("x").catch((err: unknown) => err);
    expect(e).toBeInstanceOf(PublicApiError);
    expect(e).toMatchObject({ status: 404, code: "MODULE_DISABLED", name: "PublicApiError" });
    f.mockResolvedValueOnce(json({ data: null, error: null }));
    await expect(publicGet("x")).rejects.toMatchObject({ code: "UNKNOWN_ERROR" });

    f.mockResolvedValueOnce(json({ data: "ok", error: null }));
    expect(await publicGetOr("x", "fallback")).toBe("ok");
    f.mockResolvedValueOnce(json({ data: null, error: { code: "E", message: "m" } }, 500));
    expect(await publicGetOr("x", "fallback")).toBe("fallback");
  });
});

describe("publicPost / publicFetch (navegador → /api/public)", () => {
  it("sucesso e erros", async () => {
    f.mockResolvedValueOnce(json({ data: { protocol: "P1" }, error: null }));
    expect(await publicPost("/v1/contact/messages", { a: 1 })).toEqual({ protocol: "P1" });
    expect(f.mock.calls[0]![0]).toBe("/api/public/v1/contact/messages");
    expect(f.mock.calls[0]![1]).toMatchObject({ method: "POST", body: '{"a":1}' });

    f.mockResolvedValueOnce(new Response("x", { status: 502 }));
    await expect(publicPost("v1/x", {})).rejects.toMatchObject({ code: "INVALID_RESPONSE" });
    f.mockResolvedValueOnce(json({ data: null, error: { code: "RATE_LIMITED", message: "devagar" } }, 429));
    await expect(publicPost("v1/x", {})).rejects.toMatchObject({ status: 429, code: "RATE_LIMITED" });
    f.mockResolvedValueOnce(json({ data: null }, 500));
    await expect(publicPost("v1/x", {})).rejects.toMatchObject({ code: "UNKNOWN_ERROR", message: "erro inesperado" });

    f.mockResolvedValueOnce(json({ data: [1], error: null }));
    expect(await publicFetch("/v1/catalog/services")).toEqual([1]);
    f.mockResolvedValueOnce(json({ data: null, error: { code: "NOT_FOUND", message: "x" } }, 404));
    await expect(publicFetch("v1/x")).rejects.toMatchObject({ status: 404 });
    f.mockResolvedValueOnce(json({ data: null }, 500));
    await expect(publicFetch("v1/x")).rejects.toMatchObject({ code: "UNKNOWN_ERROR" });
  });
});
