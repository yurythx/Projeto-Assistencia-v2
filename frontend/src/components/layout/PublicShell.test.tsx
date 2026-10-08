import { describe, expect, it, vi } from "vitest";

const { getFeatures } = vi.hoisted(() => ({ getFeatures: vi.fn() }));
vi.mock("@/lib/features/getFeatures", () => ({ getFeatures }));
vi.mock("@/components/layout/PublicShellClient", () => ({ PublicShellClient: () => null }));

import { PublicShell } from "./PublicShell";

describe("PublicShell (servidor)", () => {
  it("entrega ao cliente o estado dos módulos já no 1º HTML", async () => {
    getFeatures.mockResolvedValue({ features: { catalog: false, contact: true } });
    const el = await PublicShell({ children: "x" });
    expect(el.props.initialEnabled).toEqual({ catalog: false, contact: true });
  });

  it("API fora do ar (lista vazia) = estado desconhecido", async () => {
    getFeatures.mockResolvedValue({ features: {} });
    const el = await PublicShell({ children: "x" });
    expect(el.props.initialEnabled).toBeNull();
  });
});
