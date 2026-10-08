import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { BrandingProvider, useBranding } from "@/components/branding/BrandingContext";
import { DEFAULT_BRANDING } from "@/components/branding/brandingConfig";
import { fail, identityRoutes, mockBackend, renderApp } from "@/test/backend";

const refreshBranding = vi.hoisted(() => vi.fn(async () => {}));
vi.mock("./brandingActions", () => ({ refreshBranding }));

import { BrandingSettingsForm } from "./BrandingSettingsForm";

const SERVER = { ...DEFAULT_BRANDING, appName: "Prefeitura X", orgName: "Município X", tokens: { primary: "#003366" } };
const apiBranding = (extra: Record<string, unknown> = {}) => ({
  app_name: "Prefeitura X", app_description: "", org_name: "Município X", logo_url: "", favicon_url: "", support_email: "",
  support_phone: "", support_hours: "", tokens: { primary: "#003366" }, ...extra,
});

function AppName() {
  return <p data-testid="nome">{useBranding().branding.appName}</p>;
}

// O store do branding é um módulo único: cada teste entrega uma identidade
// de servidor diferente (supportHours) para ela ser aplicada de novo.
let run = 0;
function renderForm() {
  return renderApp(
    <BrandingProvider initialBranding={{ ...SERVER, supportHours: `seg-sex ${++run}` }}>
      <AppName />
      <BrandingSettingsForm />
    </BrandingProvider>,
  );
}

describe("BrandingSettingsForm", () => {
  beforeEach(() => {
    refreshBranding.mockClear();
    for (const pair of document.cookie.split(";")) {
      const name = pair.split("=")[0]?.trim();
      if (name) document.cookie = `${name}=; path=/; max-age=0`;
    }
  });

  it("salva no backend (tokens preservados) e a tela passa a usar a identidade salva", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["branding:manage"] }),
      "PUT v1/admin/branding": (req) => ({ data: { ...(req.body as object), updated_at: "2026-01-01T00:00:00Z" } }),
    });
    renderForm();
    const nome = await screen.findByLabelText("Nome da Aplicação / Sistema *");
    await waitFor(() => expect(nome).toBeEnabled());
    expect(nome).toHaveValue("Prefeitura X");
    await userEvent.clear(nome);
    await userEvent.type(nome, "Portal do Cidadão ");
    await userEvent.type(screen.getByLabelText("URL da Logomarca (PNG/SVG/WebP)"), "https://cdn.gov.br/logo.png");
    expect(screen.getByAltText("Logomarca de Portal do Cidadão")).toHaveAttribute("src", "https://cdn.gov.br/logo.png");
    await userEvent.type(screen.getByLabelText("E-mail de Suporte"), "suporte@x.gov.br");
    await userEvent.click(screen.getByRole("button", { name: /Salvar alterações/ }));

    await waitFor(() => expect(api.to("PUT v1/admin/branding")).toHaveLength(1));
    const sent: Record<string, unknown> = { ...(api.to("PUT v1/admin/branding")[0]!.body as object) };
    const expected: Record<string, unknown> = apiBranding({
      app_name: "Portal do Cidadão", app_description: DEFAULT_BRANDING.appDescription, logo_url: "https://cdn.gov.br/logo.png", support_email: "suporte@x.gov.br",
    });
    delete sent.support_hours;
    delete expected.support_hours;
    expect(sent).toEqual(expected);
    expect(await screen.findByText("Configurações salvas")).toBeInTheDocument();
    expect(refreshBranding).toHaveBeenCalled();
    // o novo nome fica (antes, o render seguinte o trocava de volta)
    expect(screen.getByTestId("nome")).toHaveTextContent("Portal do Cidadão");
  });

  it("recusa do backend (ex.: contraste) não altera a tela", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["branding:manage"] }),
      "PUT v1/admin/branding": fail(422, 'contraste entre "primary-foreground" e "primary" é 1.20:1'),
    });
    renderForm();
    const nome = await screen.findByLabelText("Nome da Aplicação / Sistema *");
    await waitFor(() => expect(nome).toBeEnabled());
    await userEvent.type(nome, " 2");
    await userEvent.click(screen.getByRole("button", { name: /Salvar alterações/ }));
    expect(await screen.findByText(/contraste entre/)).toBeInTheDocument();
    expect(refreshBranding).not.toHaveBeenCalled();
    expect(screen.getByTestId("nome")).toHaveTextContent("Prefeitura X");
  });

  it("falha ao expirar o cache não desfaz a gravação", async () => {
    refreshBranding.mockRejectedValueOnce(new Error("rede"));
    mockBackend({ ...identityRoutes({ permissions: ["branding:manage"] }), "PUT v1/admin/branding": (req) => ({ data: req.body }) });
    renderForm();
    const nome = await screen.findByLabelText("Nome da Aplicação / Sistema *");
    await waitFor(() => expect(nome).toBeEnabled());
    await userEvent.click(screen.getByRole("button", { name: /Salvar alterações/ }));
    expect(await screen.findByText("Configurações salvas")).toBeInTheDocument();
  });

  it("restaurar padrões grava a identidade padrão e limpa os tokens", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["branding:manage"] }),
      "PUT v1/admin/branding": (req) => ({ data: req.body }),
    });
    renderForm();
    await waitFor(() => expect(screen.getByRole("button", { name: /Restaurar padrões/ })).toBeEnabled());
    await userEvent.click(screen.getByRole("button", { name: /Restaurar padrões/ }));
    const dialog = await screen.findByRole("dialog", { name: "Restaurar configurações padrão?" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Restaurar padrões" }));
    await waitFor(() => expect(api.to("PUT v1/admin/branding")).toHaveLength(1));
    expect(api.to("PUT v1/admin/branding")[0]!.body).toMatchObject({ app_name: "Projeto Nexus", org_name: "Organização", tokens: {} });
    await waitFor(() => expect(screen.getByTestId("nome")).toHaveTextContent("Projeto Nexus"));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Restaurar configurações padrão?" })).not.toBeInTheDocument());

    // cancelar fecha sem gravar
    await userEvent.click(screen.getByRole("button", { name: /Restaurar padrões/ }));
    const again = await screen.findByRole("dialog", { name: "Restaurar configurações padrão?" });
    await userEvent.click(within(again).getByRole("button", { name: "Cancelar" }));
    expect(api.to("PUT v1/admin/branding")).toHaveLength(1);
  });

  it("URL insegura não vira pré-visualização; imagem quebrada cai no fallback", async () => {
    mockBackend(identityRoutes({ permissions: ["branding:manage"] }));
    renderForm();
    const logo = await screen.findByLabelText("URL da Logomarca (PNG/SVG/WebP)");
    await userEvent.type(logo, "javascript:alert(1)");
    expect(screen.queryByAltText(/Logomarca de/)).not.toBeInTheDocument();
    expect(screen.getByText("(fallback vetorial)")).toBeInTheDocument();

    const fav = screen.getByLabelText("URL do Favicon (ICO/PNG/SVG)");
    await userEvent.type(fav, "https://cdn.gov.br/f.png");
    const img = screen.getByAltText("Favicon personalizado");
    img.dispatchEvent(new Event("error"));
    await waitFor(() => expect(screen.getByText("(favicon padrão)")).toBeInTheDocument());
    await userEvent.type(fav, "x"); // editar de novo tenta a pré-visualização outra vez
    expect(screen.getByAltText("Favicon personalizado")).toBeInTheDocument();

    await userEvent.clear(logo);
    await userEvent.type(logo, "https://cdn.gov.br/l.png");
    screen.getByAltText(/Logomarca de/).dispatchEvent(new Event("error"));
    await waitFor(() => expect(screen.getByText("(fallback vetorial)")).toBeInTheDocument());

    await userEvent.type(screen.getByLabelText("Descrição Institucional"), "d");
    await userEvent.type(screen.getByLabelText("Órgão / Prefeitura Municipal *"), "o");
    await userEvent.type(screen.getByLabelText("Telefone de Atendimento"), "1");
    await userEvent.type(screen.getByLabelText("Horário de Atendimento"), "h");
  });

  it("sem branding:manage fica somente leitura", async () => {
    mockBackend(identityRoutes({ permissions: ["users:read"] }));
    renderForm();
    expect(await screen.findByRole("note")).toHaveTextContent("branding:manage");
    expect(screen.getByLabelText("Nome da Aplicação / Sistema *")).toBeDisabled();
    expect(screen.getByRole("button", { name: /Salvar alterações/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Restaurar padrões/ })).toBeDisabled();
  });
});
