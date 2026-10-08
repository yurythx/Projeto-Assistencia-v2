// @vitest-environment node
import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
const { getToken } = vi.hoisted(() => ({ getToken: vi.fn() }));
vi.mock("next-auth/jwt", () => ({ getToken }));
vi.mock("@/lib/api/backendUrl", () => ({ BACKEND_INTERNAL_URL: "http://api:8080" }));

import { DELETE, GET, PATCH, POST, PUT } from "./route";

const params = (...path: string[]) => ({ params: Promise.resolve({ path }) });
const token = { accessToken: "tok", accessTokenExpires: Date.now() + 3600_000 };

describe("proxy BFF autenticado", () => {
  let upstream: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    getToken.mockReset().mockResolvedValue(token);
    upstream = vi.fn(async () => new Response(JSON.stringify({ data: { ok: true }, error: null }), { status: 200, headers: { "content-type": "application/json" } }));
    vi.stubGlobal("fetch", upstream);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
  });

  it("sem sessão usável responde 401 sem chamar a API", async () => {
    getToken.mockResolvedValue({ accessToken: "x", accessTokenExpires: Date.now() - 1000 });
    const res = await GET(new NextRequest("http://app/api/backend/v1/me"), params("v1", "me"));
    expect(res.status).toBe(401);
    expect((await res.json()).error.code).toBe("UNAUTHORIZED");
    expect(upstream).not.toHaveBeenCalled();
  });

  it("injeta o bearer, repassa query, request-id e chave de idempotência válida", async () => {
    const req = new NextRequest("http://app/api/backend/v1/blog/posts?page=2", {
      method: "POST",
      body: JSON.stringify({ a: 1 }),
      headers: { "content-type": "application/json", "x-request-id": "req-1", "x-idempotency-key": "chave-valida-123" },
    });
    const res = await POST(req, params("api", "backend", "v1", "blog", "posts"));
    expect(res.status).toBe(200);
    const [url, init] = upstream.mock.calls[0]!;
    expect(String(url)).toBe("http://api:8080/api/v1/blog/posts?page=2");
    expect(init.headers).toMatchObject({ Authorization: "Bearer tok", "X-Request-ID": "req-1", "X-Idempotency-Key": "chave-valida-123" });
    expect(new TextDecoder().decode(init.body)).toBe('{"a":1}');
    expect(res.headers.get("x-request-id")).toBe("req-1");
  });

  it("chave de idempotência malformada não é repassada; request-id gerado", async () => {
    await PUT(new NextRequest("http://app/x", { method: "PUT", body: "{}", headers: { "x-idempotency-key": "curta" } }), params("v1", "x"));
    const init = upstream.mock.calls[0]![1];
    expect(init.headers["X-Idempotency-Key"]).toBeUndefined();
    expect(init.headers["X-Request-ID"]).toMatch(/[0-9a-f-]{36}/);
  });

  it("IP do visitante segue para a API só atrás de proxy confiável", async () => {
    const req = () => new NextRequest("http://app/x", { headers: { "x-forwarded-for": "203.0.113.8" } });
    await GET(req(), params("v1", "me"));
    expect(upstream.mock.calls[0]![1].headers["X-Forwarded-For"]).toBeUndefined();
    vi.stubEnv("TRUST_PROXY_HEADERS", "true");
    await GET(req(), params("v1", "me"));
    expect(upstream.mock.calls[1]![1].headers["X-Forwarded-For"]).toBe("203.0.113.8");
  });

  it("recusa '..' e outros segmentos que sairiam de /api", async () => {
    for (const seg of ["..", ".", "a/b", "a\\b"]) {
      const res = await GET(new NextRequest("http://app/x"), params("v1", seg, "metrics"));
      expect(res.status).toBe(404);
    }
    expect(upstream).not.toHaveBeenCalled();
  });

  it("API fora do ar vira 503", async () => {
    upstream.mockRejectedValueOnce(new Error("ECONNREFUSED"));
    const res = await PATCH(new NextRequest("http://app/x", { method: "PATCH", body: "{}" }), params("v1", "x"));
    expect(res.status).toBe(503);
    expect((await res.json()).error.code).toBe("DEPENDENCY_UNAVAILABLE");
  });

  it("resposta binária passa intacta e o Content-Disposition é propagado", async () => {
    const bytes = new Uint8Array([0x50, 0x4b, 0xff, 0x00]);
    upstream.mockResolvedValueOnce(new Response(bytes, { status: 200, headers: { "content-type": "application/zip", "content-disposition": 'attachment; filename="p.zip"' } }));
    const res = await DELETE(new NextRequest("http://app/x", { method: "DELETE" }), params("v1", "x"));
    expect(new Uint8Array(await res.arrayBuffer())).toEqual(bytes);
    expect(res.headers.get("content-disposition")).toBe('attachment; filename="p.zip"');
    expect(upstream.mock.calls[0]![1].body).toBeInstanceOf(ArrayBuffer);
  });

  it("CSV passa com o BOM (o Excel em português depende dele)", async () => {
    const csv = new Uint8Array([0xef, 0xbb, 0xbf, ...new TextEncoder().encode("Código;Série\n")]);
    upstream.mockResolvedValueOnce(new Response(csv, { status: 200, headers: { "content-type": "text/csv; charset=utf-8" } }));
    const res = await GET(new NextRequest("http://app/x"), params("v1", "atlas", "ttdd", "exportar"));
    expect(new Uint8Array(await res.arrayBuffer())).toEqual(csv);
  });

  it("204 da API passa como 204 sem corpo (antes: TypeError → 500)", async () => {
    upstream.mockResolvedValueOnce(new Response(null, { status: 204 }));
    const res = await POST(new NextRequest("http://app/x", { method: "POST", body: "{}" }), params("v1", "mercurio", "rooms", "r1", "read"));
    expect(res.status).toBe(204);
    expect(await res.text()).toBe("");
  });

  it("resposta sem content-type é tratada como JSON; GET sem corpo", async () => {
    upstream.mockResolvedValueOnce(new Response("{}", { status: 200 }));
    const res = await GET(new NextRequest("http://app/x"), params("v1", "x"));
    expect(res.headers.get("content-type")).toContain("text/plain");
    const init = upstream.mock.calls[0]![1];
    expect(init.body).toBeUndefined();
    // sem content-type na requisição, assume JSON
    expect(init.headers["Content-Type"]).toBe("application/json");
  });
});
