import { afterEach, describe, expect, it, vi } from "vitest";

import { randomUUID } from "./randomUUID";

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("randomUUID", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("usa crypto.randomUUID quando existe", () => {
    expect(randomUUID()).toMatch(V4);
  });

  it("funciona sem crypto.randomUUID (HTTP fora de contexto seguro)", () => {
    const real = globalThis.crypto;
    vi.stubGlobal("crypto", { getRandomValues: real.getRandomValues.bind(real) });
    const ids = new Set(Array.from({ length: 100 }, () => randomUUID()));
    expect(ids.size).toBe(100);
    for (const id of ids) expect(id).toMatch(V4);
  });
});
