import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/client";

const serverApiGet = vi.fn();
vi.mock("@/lib/api/server", () => ({ serverApiGet: (...a: unknown[]) => serverApiGet(...a) }));
vi.mock("@/components/settings/KeycloakSettingsForm", () => ({
  KeycloakSettingsForm: ({ initialStatus }: { initialStatus: { issuer_url: string } }) => <p>formulário: {initialStatus.issuer_url}</p>,
}));

import KeycloakConfigPage from "./page";

describe("Configurações > Keycloak (Server Component)", () => {
  beforeEach(() => serverApiGet.mockReset());

  it("mostra o formulário com o status atual", async () => {
    serverApiGet.mockResolvedValue({ data: { issuer_url: "https://sso/realms/n", source: "database" } });
    render(await KeycloakConfigPage());
    expect(screen.getByText("formulário: https://sso/realms/n")).toBeInTheDocument();
    expect(serverApiGet.mock.calls[0]![0]).toBe("v1/admin/keycloak");
  });

  it("403 vira aviso de acesso restrito", async () => {
    serverApiGet.mockImplementationOnce(async () => {
      throw new ApiError(403, "FORBIDDEN", "proibido");
    });
    render(await KeycloakConfigPage());
    expect(screen.getByText(/Restrito a administradores/)).toBeInTheDocument();
  });

  it("outros erros mostram a mensagem", async () => {
    serverApiGet.mockImplementationOnce(async () => {
      throw new ApiError(503, "DEP", "Keycloak fora");
    });
    const { unmount } = render(await KeycloakConfigPage());
    expect(screen.getByText("Keycloak fora")).toBeInTheDocument();
    unmount();
    serverApiGet.mockImplementationOnce(async () => {
      throw new Error("rede");
    });
    render(await KeycloakConfigPage());
    expect(screen.getByText("Falha ao carregar a configuração do Keycloak")).toBeInTheDocument();
  });
});
