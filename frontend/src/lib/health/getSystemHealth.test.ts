import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

import { getSystemHealth } from "./getSystemHealth";

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

describe("getSystemHealth", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("todas as dependências ok — status ok", async () => {
    mockBackendReady(200, { postgres: "ok", rabbitmq: "ok", minio: "ok" });
    const health = await getSystemHealth();
    expect(health.status).toBe("ok");
    expect(health.httpStatus).toBe(200);
    expect(health.services).toEqual({
      postgres: { status: "ok" },
      rabbitmq: { status: "ok" },
      minio: { status: "ok" },
    });
  });

  it("uma dependência indisponível — status degraded, propaga o status HTTP do backend", async () => {
    mockBackendReady(503, { postgres: "ok", rabbitmq: "unavailable", minio: "ok" });
    const health = await getSystemHealth();
    expect(health.status).toBe("degraded");
    expect(health.httpStatus).toBe(503);
  });

  it("backend inalcançável — status unhealthy, 503, nunca lança", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("connection refused")));
    const health = await getSystemHealth();
    expect(health.status).toBe("unhealthy");
    expect(health.httpStatus).toBe(503);
    expect(health.services).toEqual({});
  });
});
