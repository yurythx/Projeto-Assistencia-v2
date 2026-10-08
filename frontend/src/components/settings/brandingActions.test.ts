// @vitest-environment node
import { describe, expect, it, vi } from "vitest";

const getServerSession = vi.fn();
vi.mock("next-auth/next", () => ({ getServerSession: (...a: unknown[]) => getServerSession(...a) }));
vi.mock("@/lib/auth/options", () => ({ authOptions: {} }));
const updateTag = vi.fn();
vi.mock("next/cache", () => ({ updateTag: (t: string) => updateTag(t) }));

import { refreshBranding } from "./brandingActions";

describe("refreshBranding (server action)", () => {
  it("só expira o cache com sessão válida", async () => {
    getServerSession.mockResolvedValueOnce(null);
    await refreshBranding();
    getServerSession.mockResolvedValueOnce({ error: "RefreshAccessTokenError" });
    await refreshBranding();
    expect(updateTag).not.toHaveBeenCalled();
    getServerSession.mockResolvedValueOnce({ user: { name: "a" } });
    await refreshBranding();
    expect(updateTag).toHaveBeenCalledWith("branding");
  });
});
