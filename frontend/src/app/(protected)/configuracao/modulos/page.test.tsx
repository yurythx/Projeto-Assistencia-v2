import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "@/components/notifications/ToastProvider";
import type { ModuleStatus } from "@/lib/nexus/types";

const patch = vi.fn();
vi.mock("@/lib/api/client", async (orig) => ({
  ...(await orig<typeof import("@/lib/api/client")>()),
  apiClient: { patch: (...a: unknown[]) => patch(...a) },
}));

let modules: ModuleStatus[] = [];
const refreshModules = vi.fn();
vi.mock("@/lib/nexus/NexusProvider", () => ({
  useNexus: () => ({ modules, loading: false, can: () => true, refreshModules }),
}));

import ModulosPage from "./page";

function mod(key: string, name: string, extra: Partial<ModuleStatus> = {}): ModuleStatus {
  return {
    key, name, description: `Descrição ${name}`, core: false, default_enabled: true, depends_on: [], permissions: [],
    public: false, icon: "box", route: `/${key}`, enabled: true, configured: true, dependents: [], blocked_by: [], ...extra,
  };
}

function renderPage() {
  return render(
    <ToastProvider>
      <ModulosPage />
    </ToastProvider>,
  );
}

function card(name: string) {
  return screen.getByRole("heading", { name }).closest("div.rounded-xl") as HTMLElement;
}

describe("Configurações > Módulos", () => {
  beforeEach(() => {
    patch.mockReset().mockResolvedValue({ data: {} });
    refreshModules.mockReset();
    modules = [
      mod("iam", "IAM", { core: true }),
      mod("signum", "Signum", { dependents: ["tramite"] }),
      mod("tramite", "Trâmite", { depends_on: ["signum"] }),
      mod("blog", "Blog"),
    ];
  });

  it("mostra o grafo de dependências nos dois sentidos", () => {
    renderPage();
    expect(within(card("Trâmite")).getByText("Depende de:")).toBeInTheDocument();
    expect(within(card("Trâmite")).getByText("Signum")).toBeInTheDocument();
    expect(within(card("Signum")).getByText("Necessário para:")).toBeInTheDocument();
    expect(within(card("Signum")).getByText("Trâmite")).toBeInTheDocument();
    expect(within(card("IAM")).getByText("Sempre ativo")).toBeInTheDocument();
    expect(within(card("IAM")).queryByRole("switch")).not.toBeInTheDocument();
  });

  it("módulo sem dependentes desativa direto, sem diálogo", async () => {
    renderPage();
    await userEvent.click(within(card("Blog")).getByRole("switch"));
    await waitFor(() => expect(patch).toHaveBeenCalledWith("v1/admin/modules/blog", { enabled: false }));
    expect(patch).toHaveBeenCalledTimes(1);
    expect(refreshModules).toHaveBeenCalled();
  });

  it("desativar uma dependência pede confirmação e desliga os dependentes antes", async () => {
    renderPage();
    await userEvent.click(within(card("Signum")).getByRole("switch"));
    const dialog = await screen.findByRole("dialog", { name: "Desativar Signum?" });
    expect(within(dialog).getByText("Trâmite")).toBeInTheDocument();
    expect(patch).not.toHaveBeenCalled();

    await userEvent.click(within(dialog).getByRole("button", { name: "Desativar todos" }));
    await waitFor(() => expect(patch).toHaveBeenCalledTimes(2));
    expect(patch.mock.calls.map((c) => c[0])).toEqual(["v1/admin/modules/tramite", "v1/admin/modules/signum"]);
  });

  it("cancelar a cascata não altera nada", async () => {
    renderPage();
    await userEvent.click(within(card("Signum")).getByRole("switch"));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancelar" }));
    expect(patch).not.toHaveBeenCalled();
  });

  it("módulo configurado mas bloqueado explica o motivo e oferece ativar a dependência", async () => {
    modules = [
      mod("signum", "Signum", { configured: false, enabled: false, dependents: ["tramite"] }),
      mod("tramite", "Trâmite", { depends_on: ["signum"], enabled: false, blocked_by: ["signum"] }),
    ];
    renderPage();
    const status = within(card("Trâmite")).getByRole("status");
    expect(status).toHaveTextContent("depende de Signum, que está inativo");
    await userEvent.click(within(status).getByRole("button", { name: "Ativar dependências" }));
    const dialog = await screen.findByRole("dialog", { name: "Ativar Trâmite?" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Ativar todos" }));
    await waitFor(() => expect(patch).toHaveBeenCalledTimes(2));
    expect(patch.mock.calls).toEqual([
      ["v1/admin/modules/signum", { enabled: true }],
      ["v1/admin/modules/tramite", { enabled: true }],
    ]);
  });

  it("erro 409 do backend vira aviso e a lista é recarregada", async () => {
    const { ApiError } = await import("@/lib/api/client");
    patch.mockRejectedValue(new ApiError(409, "CONFLICT", "kernel: há módulos ativos que dependem deste"));
    renderPage();
    await userEvent.click(within(card("Blog")).getByRole("switch"));
    expect(await screen.findByText("kernel: há módulos ativos que dependem deste")).toBeInTheDocument();
    expect(refreshModules).toHaveBeenCalled();
  });

  it("a visão em grafo opera o mesmo fluxo, com a mesma confirmação de cascata", async () => {
    renderPage();
    const cards = screen.getByRole("button", { name: "Cartões" });
    expect(cards).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: "Grafo de dependências" }));
    expect(screen.queryByRole("heading", { name: "Trâmite" })).not.toBeInTheDocument();
    expect(screen.getByRole("group", { name: /4 módulos, 1 dependência\./ })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /^Signum; Ativo/ }));
    await userEvent.click(screen.getByRole("switch", { name: "Desativar Signum" }));
    const dialog = await screen.findByRole("dialog", { name: "Desativar Signum?" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Desativar todos" }));
    await waitFor(() => expect(patch).toHaveBeenCalledTimes(2));
    expect(patch.mock.calls.map((c) => c[0])).toEqual(["v1/admin/modules/tramite", "v1/admin/modules/signum"]);

    await userEvent.click(cards);
    expect(screen.getByRole("heading", { name: "Trâmite" })).toBeInTheDocument();
  });
});
