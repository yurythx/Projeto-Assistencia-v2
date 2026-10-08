import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

let sessionStatus: "authenticated" | "unauthenticated" | "loading" = "unauthenticated";
vi.mock("next-auth/react", () => ({ useSession: () => ({ status: sessionStatus }) }));
vi.mock("@/components/branding/BrandingContext", () => ({ useBranding: () => ({ branding: { appName: "Nexus", orgName: "Órgão" } }) }));

const get = vi.fn();
const post = vi.fn();
vi.mock("@/lib/api/client", async (orig) => ({
  ...(await orig<typeof import("@/lib/api/client")>()),
  apiClient: { get: (...a: unknown[]) => get(...a), post: (...a: unknown[]) => post(...a) },
}));
const publicPost = vi.fn();
vi.mock("@/lib/api/publicClient", () => ({ publicPost: (...a: unknown[]) => publicPost(...a) }));

import { LGPDConsentModal } from "./LGPDConsentModal";

async function flush() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1500);
  });
}

describe("LGPDConsentModal", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    localStorage.clear();
    get.mockReset();
    post.mockReset().mockResolvedValue({ data: {} });
    publicPost.mockReset().mockResolvedValue({});
  });
  afterEach(() => vi.useRealTimers());

  it("visitante: aceite vai pelo proxy PÚBLICO e fica lembrado", async () => {
    sessionStatus = "unauthenticated";
    render(<LGPDConsentModal />);
    await flush();
    await userEvent.click(screen.getByRole("button", { name: /Concordar/ }));
    expect(publicPost).toHaveBeenCalledWith("v1/lgpd/accept-anon", expect.objectContaining({ term_version: "v1.0.0-2026" }));
    expect(post).not.toHaveBeenCalled();
    expect(localStorage.getItem("nexus_lgpd_consent")).toBe("v1.0.0-2026");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("logado: o servidor decide — aceite local não dispensa o registro na conta", async () => {
    sessionStatus = "authenticated";
    localStorage.setItem("nexus_lgpd_consent", "v1.0.0-2026");
    get.mockResolvedValue({ data: { accepted: false, term_version: "v1.0.0-2026" } });
    render(<LGPDConsentModal />);
    await flush();
    expect(get).toHaveBeenCalledWith("v1/lgpd/status");
    await userEvent.click(await screen.findByRole("button", { name: /Concordar/ }));
    expect(post).toHaveBeenCalledWith("v1/lgpd/accept", { term_version: "v1.0.0-2026" });
  });

  it("logado com aceite no servidor: não incomoda", async () => {
    sessionStatus = "authenticated";
    get.mockResolvedValue({ data: { accepted: true, term_version: "v1.0.0-2026" } });
    render(<LGPDConsentModal />);
    await flush();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("falha ao registrar: o modal continua e explica", async () => {
    sessionStatus = "unauthenticated";
    const { ApiError } = await import("@/lib/api/client");
    publicPost.mockRejectedValue(new ApiError(503, "DEPENDENCY_UNAVAILABLE", "serviço indisponível no momento"));
    render(<LGPDConsentModal />);
    await flush();
    await userEvent.click(screen.getByRole("button", { name: /Concordar/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent("serviço indisponível");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(localStorage.getItem("nexus_lgpd_consent")).toBeNull();
  });
});
