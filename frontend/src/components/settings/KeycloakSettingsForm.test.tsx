import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "@/components/notifications/ToastProvider";
import type { KeycloakSettingsStatus, KeycloakTestResult } from "@/types/api";

import { KeycloakSettingsForm } from "./KeycloakSettingsForm";

function mockFetchOnce(status: number, body: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: status >= 200 && status < 300,
      status,
      json: async () => body,
    }),
  );
}

function makeStatus(overrides: Partial<KeycloakSettingsStatus> = {}): KeycloakSettingsStatus {
  return {
    source: "database",
    issuer_url: "https://sso.orgao.gov.br/realms/nexus",
    realm: "nexus",
    client_id: "nexus-backend",
    client_secret_set: true,
    audience: "nexus-backend",
    frontend_client_id: "nexus-frontend",
    frontend_client_secret_set: true,
    updated_at: "2026-01-01T00:00:00Z",
    updated_by: "admin-1",
    ...overrides,
  };
}

describe("KeycloakSettingsForm", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renderiza os valores atuais e o selo de origem (banco)", () => {
    render(
      <ToastProvider>
        <KeycloakSettingsForm initialStatus={makeStatus()} />
      </ToastProvider>,
    );

    expect(screen.getByDisplayValue("https://sso.orgao.gov.br/realms/nexus")).toBeInTheDocument();
    expect(screen.getByLabelText("Client ID do Backend *")).toHaveValue("nexus-backend");
    expect(screen.getByText("Salvo no banco — em uso agora")).toBeInTheDocument();
    // O client secret NUNCA vem preenchido — só um placeholder indicando
    // que já existe um valor salvo.
    expect(screen.getByLabelText("Client Secret do Backend")).toHaveValue("");
  });

  it("mostra o aviso de adoção quando a configuração ainda vem do ambiente", () => {
    render(
      <ToastProvider>
        <KeycloakSettingsForm initialStatus={makeStatus({ source: "environment" })} />
      </ToastProvider>,
    );
    expect(screen.getByText("Vindo de variável de ambiente")).toBeInTheDocument();
    expect(screen.getByText(/nunca foi salva por esta tela/)).toBeInTheDocument();
  });

  it("testar conexão chama POST /admin/keycloak/test e mostra o resultado", async () => {
    const testResult: KeycloakTestResult = {
      status: "ok",
      discovery_ok: true,
      discovery_message: "Discovery OIDC respondeu com sucesso.",
      credentials_checked: false,
      credentials_ok: false,
      credentials_message: "",
    };
    mockFetchOnce(200, { data: testResult, error: null });
    const user = userEvent.setup();

    render(
      <ToastProvider>
        <KeycloakSettingsForm initialStatus={makeStatus()} />
      </ToastProvider>,
    );

    await user.click(screen.getByRole("button", { name: /testar conexão/i }));

    expect(await screen.findByText(/Discovery OIDC respondeu com sucesso/)).toBeInTheDocument();
    const [path, init] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] ?? [];
    expect(path).toBe("/api/backend/v1/admin/keycloak/test");
    expect(init.method).toBe("POST");
  });

  it("salvar chama PUT /admin/keycloak, atualiza o selo e mostra um toast de sucesso", async () => {
    const saved = makeStatus({ realm: "nexus-v2" });
    mockFetchOnce(200, {
      data: {
        settings: saved,
        test: {
          status: "ok",
          discovery_ok: true,
          discovery_message: "Discovery OIDC respondeu com sucesso.",
          credentials_checked: false,
          credentials_ok: false,
          credentials_message: "",
        },
      },
      error: null,
    });
    const user = userEvent.setup();

    render(
      <ToastProvider>
        <KeycloakSettingsForm initialStatus={makeStatus()} />
      </ToastProvider>,
    );

    await user.clear(screen.getByLabelText("Realm *"));
    await user.type(screen.getByLabelText("Realm *"), "nexus-v2");
    await user.click(screen.getByRole("button", { name: /salvar alterações/i }));

    expect(await screen.findByText("Configuração do Keycloak salva")).toBeInTheDocument();
    const [path, init] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] ?? [];
    expect(path).toBe("/api/backend/v1/admin/keycloak");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toMatchObject({ realm: "nexus-v2", client_secret: "" });
  });

  it("se o salvamento falhar (ex.: discovery recusado), mostra um toast de erro e não perde os dados digitados", async () => {
    mockFetchOnce(422, {
      data: null,
      error: { code: "VALIDATION_ERROR", message: "Não foi possível salvar: issuer inalcançável" },
    });
    const user = userEvent.setup();

    render(
      <ToastProvider>
        <KeycloakSettingsForm initialStatus={makeStatus()} />
      </ToastProvider>,
    );

    await user.click(screen.getByRole("button", { name: /salvar alterações/i }));

    expect(await screen.findByText("Não foi possível salvar")).toBeInTheDocument();
    expect(screen.getByText("Não foi possível salvar: issuer inalcançável")).toBeInTheDocument();
    // Os campos continuam preenchidos com o que o admin digitou.
    expect(screen.getByDisplayValue("https://sso.orgao.gov.br/realms/nexus")).toBeInTheDocument();
  });
});
