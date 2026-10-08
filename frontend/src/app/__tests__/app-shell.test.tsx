import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("next/font/local", () => ({ default: () => ({ variable: "--font" }) }));
const cookieJar = vi.hoisted(() => ({ values: {} as Record<string, string> }));
vi.mock("next/headers", () => ({ cookies: async () => ({ get: (n: string) => (n in cookieJar.values ? { value: cookieJar.values[n] } : undefined) }) }));
class Redirect extends Error {}
vi.mock("next/navigation", async () => ({
  ...(await import("@/test/navigation")).navigationModule,
  redirect: (to: string) => {
    throw new Redirect(to);
  },
}));
const getServerSession = vi.fn();
vi.mock("next-auth/next", () => ({ getServerSession: (...a: unknown[]) => getServerSession(...a) }));
vi.mock("next-auth", () => ({ default: () => "handler" }));
vi.mock("@/lib/auth/options", () => ({ authOptions: {} }));
const publicGetOr = vi.fn();
vi.mock("@/lib/api/publicServer", () => ({ publicGetOr: (...a: unknown[]) => publicGetOr(...a) }));
vi.mock("@/app/providers", () => ({ Providers: ({ children }: { children: ReactNode }) => <>{children}</> }));
vi.mock("@/components/layout/DashboardShell", () => ({
  DashboardShell: (p: { userLabel: string; initialTheme?: string; initialCollapsed: boolean; children: ReactNode }) => (
    <div data-testid="shell" data-user={p.userLabel} data-theme={p.initialTheme ?? ""} data-collapsed={String(p.initialCollapsed)}>
      {p.children}
    </div>
  ),
}));
vi.mock("@/components/monitoring/PlatformMonitoringDashboard", () => ({ PlatformMonitoringDashboard: () => <p>painel</p> }));
vi.mock("@/components/settings/BrandingSettingsForm", () => ({ BrandingSettingsForm: () => <p>branding</p> }));

import { BrandingProvider } from "@/components/branding/BrandingContext";
import { SectionTabsInline } from "@/components/nexus/SectionTabsInline";
import { Pagination } from "@/components/nexus/Pagination";
import { Spinner } from "@/components/ui/Spinner";
import { getServerBranding } from "@/lib/branding/server";
import { identityRoutes, mockBackend, moduleStatus, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

import * as authRoute from "../api/auth/[...nextauth]/route";
import GlobalError from "../error";
import RootLayout, { generateMetadata } from "../layout";
import NotFound from "../not-found";
import ProtectedError from "../(protected)/error";
import ProtectedLayout from "../(protected)/layout";
import ConfiguracaoLayout from "../(protected)/configuracao/layout";
import ConfiguracaoLoading from "../(protected)/configuracao/loading";
import ConfiguracaoPage from "../(protected)/configuracao/page";
import DashboardLoading from "../(protected)/dashboard/loading";
import MonitoramentoLoading from "../(protected)/monitoramento/loading";
import MonitoramentoPage from "../(protected)/monitoramento/page";
import WikiHome from "../(protected)/wiki/page";

beforeEach(() => {
  cookieJar.values = {};
  publicGetOr.mockReset();
  getServerSession.mockReset();
  resetNavigation({}, "/configuracao");
});

describe("layouts de servidor", () => {
  it("raiz: metadados e tokens white-label a partir do branding da API", async () => {
    publicGetOr.mockResolvedValue({ app_name: "Portal X", app_description: "Serviços", org_name: "M", logo_url: "", favicon_url: "", support_email: "", support_phone: "", support_hours: "", tokens: { primary: "#003366" } });
    expect(await generateMetadata()).toMatchObject({ title: { default: "Portal X", template: "%s — Portal X" }, description: "Serviços" });
    cookieJar.values = { "nexus-theme": "dark", "nexus-branding": JSON.stringify({ highContrast: true, fontSizeScale: 120 }) };
    const el = await RootLayout({ children: <p>filho</p> });
    const html = el as unknown as { props: { "data-theme"?: string; "data-high-contrast"?: string; "data-font-scale": string; children: unknown[] } };
    expect(html.props["data-theme"]).toBe("dark");
    expect(html.props["data-high-contrast"]).toBe("true");
    expect(html.props["data-font-scale"]).toBe("120");
    expect(JSON.stringify(html.props.children)).toContain("--primary:#003366");

    publicGetOr.mockResolvedValue(null);
    expect(await generateMetadata()).toMatchObject({ title: { default: "Projeto Nexus" } });
    cookieJar.values = { "nexus-theme": "roxo" };
    const plain = (await RootLayout({ children: null })) as unknown as { props: Record<string, unknown> };
    expect(plain.props["data-theme"]).toBeUndefined();
    expect(plain.props["data-high-contrast"]).toBeUndefined();
    // sem tokens, <head> não recebe nó de texto vazio (quebrava a hidratação
    // e a navegação do login para o dashboard)
    const head = (plain.props.children as { type: string; props: { children: unknown } }[]).find((c) => c?.type === "head")!;
    expect(head.props.children).toBeNull();
  });

  it("área autenticada: sem sessão (ou com erro) vai ao login; senão monta o shell", async () => {
    getServerSession.mockResolvedValueOnce(null);
    await expect(ProtectedLayout({ children: null })).rejects.toThrow("/login");
    getServerSession.mockResolvedValueOnce({ error: "RefreshAccessTokenError" });
    await expect(ProtectedLayout({ children: null })).rejects.toThrow("/login");

    getServerSession.mockResolvedValueOnce({ user: { email: "maria@x.gov.br" } });
    cookieJar.values = { "nexus-theme": "light", "nexus-sidebar-collapsed": "true" };
    render(await ProtectedLayout({ children: <p>tela</p> }));
    const shell = screen.getByTestId("shell");
    expect(shell).toHaveAttribute("data-user", "maria@x.gov.br");
    expect(shell).toHaveAttribute("data-theme", "light");
    expect(shell).toHaveAttribute("data-collapsed", "true");

    getServerSession.mockResolvedValueOnce({ user: { name: "Maria" } });
    cookieJar.values = {};
    const { unmount } = render(await ProtectedLayout({ children: null }));
    expect(screen.getAllByTestId("shell").at(-1)).toHaveAttribute("data-user", "Maria");
    unmount();
    getServerSession.mockResolvedValueOnce({});
    render(await ProtectedLayout({ children: null }));
    expect(screen.getAllByTestId("shell").at(-1)).toHaveAttribute("data-user", "Autenticado");
  });

  it("getServerBranding com e sem API", async () => {
    publicGetOr.mockResolvedValueOnce({ app_name: "", org_name: "Órgão", app_description: "d", support_email: "e", support_phone: "p" });
    expect(await getServerBranding()).toEqual({ appName: "Projeto Nexus", appDescription: "d", orgName: "Órgão", supportEmail: "e", supportPhone: "p" });
    publicGetOr.mockResolvedValueOnce(null);
    expect(await getServerBranding()).toMatchObject({ orgName: "Organização", supportEmail: "" });
  });

  it("rota do NextAuth exporta o mesmo handler para GET e POST", () => {
    expect(authRoute.GET).toBe("handler");
    expect(authRoute.POST).toBe(authRoute.GET);
  });
});

describe("fronteiras de erro, 404 e carregamento", () => {
  it("erro global mostra o código de referência e tenta de novo", async () => {
    const retry = vi.fn();
    const { unmount } = render(<BrandingProvider><GlobalError error={Object.assign(new Error("x"), { digest: "abc123" })} retry={retry} /></BrandingProvider>);
    expect(screen.getByText("abc123")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(retry).toHaveBeenCalled();
    unmount();
    render(<BrandingProvider><GlobalError error={new Error("x")} retry={retry} /></BrandingProvider>);
    expect(screen.queryByText(/Código de referência/)).not.toBeInTheDocument();
  });

  it("erro na área autenticada registra no console e refaz a busca", async () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const retry = vi.fn();
    const { unmount } = render(<ProtectedError error={new Error("falhou o backend")} retry={retry} />);
    expect(screen.getByText("falhou o backend")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Tentar Novamente/ }));
    expect(retry).toHaveBeenCalled();
    expect(spy).toHaveBeenCalled();
    unmount();
    render(<ProtectedError error={new Error("")} retry={retry} />);
    expect(screen.getByText(/Não foi possível carregar/)).toBeInTheDocument();
    spy.mockRestore();
  });

  it("404, carregamentos e páginas simples", () => {
    render(<BrandingProvider><NotFound /></BrandingProvider>);
    expect(screen.getByRole("heading", { name: "Página não encontrada" })).toBeInTheDocument();
    for (const L of [ConfiguracaoLoading, DashboardLoading, MonitoramentoLoading]) {
      const { unmount, container } = render(<L />);
      expect(container.firstChild).not.toBeNull();
      unmount();
    }
    render(<MonitoramentoPage />);
    expect(screen.getByText("painel")).toBeInTheDocument();
    render(<ConfiguracaoPage />);
    expect(screen.getByText("branding")).toBeInTheDocument();
    render(<WikiHome />);
    expect(screen.getByText(/Escolha uma página/)).toBeInTheDocument();
    render(<Spinner label="Salvando" />);
    expect(screen.getByRole("status", { name: "" })).toHaveTextContent("Salvando");
  });
});

describe("navegação em abas e paginação", () => {
  it("configurações: abas conforme permissão", async () => {
    mockBackend(identityRoutes({}, [moduleStatus("egress")]));
    renderApp(<ConfiguracaoLayout><p>seção</p></ConfiguracaoLayout>);
    expect(await screen.findByRole("navigation", { name: "Seções de configuração" })).toBeInTheDocument();
    expect(screen.getByText("seção")).toBeInTheDocument();
  });

  it("configurações sem nenhuma aba visível", async () => {
    mockBackend(identityRoutes({ permissions: [] }, []));
    renderApp(<ConfiguracaoLayout><p>seção</p></ConfiguracaoLayout>);
    await screen.findByText("seção");
    expect(screen.queryByRole("navigation", { name: "Seções de configuração" })).not.toBeInTheDocument();
  });

  it("abas locais navegam com as setas (WAI-ARIA)", async () => {
    const onChange = vi.fn();
    const tabs = [{ value: "a", label: "A" }, { value: "b", label: "B" }, { value: "c", label: "C" }];
    render(<SectionTabsInline label="abas" tabs={tabs} value="a" onChange={onChange} />);
    const a = screen.getByRole("tab", { name: "A" });
    expect(a).toHaveAttribute("aria-selected", "true");
    fireEvent.keyDown(a, { key: "ArrowRight" });
    expect(onChange).toHaveBeenLastCalledWith("b");
    fireEvent.keyDown(a, { key: "ArrowLeft" });
    expect(onChange).toHaveBeenLastCalledWith("c");
    fireEvent.keyDown(a, { key: "Enter" });
    expect(onChange).toHaveBeenCalledTimes(2);
    await userEvent.click(screen.getByRole("tab", { name: "B" }));
    expect(onChange).toHaveBeenLastCalledWith("b");
  });

  it("paginação: some com uma página; limites desabilitam os botões", async () => {
    const onPage = vi.fn();
    const { rerender, container } = render(<Pagination meta={{ page: 1, page_size: 10, total_items: 5, total_pages: 1 }} onPage={onPage} />);
    expect(container).toBeEmptyDOMElement();
    rerender(<Pagination meta={undefined} onPage={onPage} />);
    expect(container).toBeEmptyDOMElement();
    rerender(<Pagination meta={{ page: 1, page_size: 10, total_items: 25, total_pages: 3 }} onPage={onPage} />);
    expect(screen.getByText("Página 1 de 3 · 25 registros")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Anterior" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Próxima" }));
    expect(onPage).toHaveBeenLastCalledWith(2);
    rerender(<Pagination meta={{ page: 3, page_size: 10, total_items: 25, total_pages: 3 }} onPage={onPage} />);
    expect(screen.getByRole("button", { name: "Próxima" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Anterior" }));
    expect(onPage).toHaveBeenLastCalledWith(2);
  });
});
