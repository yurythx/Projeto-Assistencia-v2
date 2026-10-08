import { afterEach, describe, expect, it, vi } from "vitest";

// server-only detecta ambiente client/server via um campo "browser" no
// package.json que só o bundler do Next.js respeita — no runtime puro do
// Vitest, o import cru sempre lança. lib/health/getSystemHealth.ts (que
// esta rota agora reaproveita) usa "server-only" de propósito (é chamado
// direto por um Server Component, e nunca deveria vazar pro bundle do
// cliente) — mockado aqui como um módulo vazio, o jeito padrão de manter
// a guarda real no build do Next sem quebrar o teste.
vi.mock("server-only", () => ({}));

import { GET } from "./route";

// GET /api/health é a ponte server-to-server pro /ready do backend Go —
// achado de auditoria: o painel de Monitoramento chamava
// apiClient.get("health"), que o proxy BFF genérico sempre traduz pra
// GET /api/health no backend (/health não vive sob /api/v1), resultando
// em 404 sempre. Esta rota substitui aquela chamada.
function mockBackendReady(status: number, data: Record<string, string> | null) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: status >= 200 && status < 300,
      status,
      json: async () => ({ data, error: null }),
    }),
  );
}

describe("GET /api/health", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("todas as dependências ok — status ok, 200", async () => {
    mockBackendReady(200, { postgres: "ok", rabbitmq: "ok" });

    const res = await GET();
    const body = await res.json();

    expect(res.status).toBe(200);
    expect(body.data.status).toBe("ok");
    expect(body.data.services).toEqual({
      postgres: { status: "ok" },
      rabbitmq: { status: "ok" },
    });
    expect(body.error).toBeNull();
  });

  it("uma dependência indisponível — status degraded, propaga o 503 do backend", async () => {
    mockBackendReady(503, { postgres: "ok", rabbitmq: "unavailable" });

    const res = await GET();
    const body = await res.json();

    expect(res.status).toBe(503);
    expect(body.data.status).toBe("degraded");
    expect(body.data.services.rabbitmq).toEqual({ status: "unavailable" });
  });

  it("backend inalcançável (erro de rede) — status unhealthy, 503, nunca lança", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("connection refused")));

    const res = await GET();
    const body = await res.json();

    expect(res.status).toBe(503);
    expect(body.data.status).toBe("unhealthy");
    expect(body.error).not.toBeNull();
  });

  it("resposta do backend ilegível — não lança, ainda reporta um status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => {
          throw new Error("invalid json");
        },
      }),
    );

    const res = await GET();
    const body = await res.json();

    expect(res.status).toBe(200);
    expect(body.data.services).toEqual({});
  });
});
