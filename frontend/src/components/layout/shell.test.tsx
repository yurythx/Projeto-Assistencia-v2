import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SWRConfig } from "swr";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { BrandingProvider, useBranding } from "@/components/branding/BrandingContext";
import { DEFAULT_BRANDING } from "@/components/branding/brandingConfig";
import { identityRoutes, moduleStatus, mockBackend, mockWebSocket, wsTicketRoute } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);
const session = vi.hoisted(() => ({ status: "unauthenticated" }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({ status: session.status }),
  signOut: vi.fn(),
  SessionProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("next/script", () => ({ default: ({ onLoad }: { onLoad?: () => void }) => <button type="button" onClick={onLoad}>script vlibras</button> }));

import { Providers } from "@/app/providers";
import { AccessibilityShortcuts, focusSkipTarget } from "./AccessibilityShortcuts";
import { DashboardShell } from "./DashboardShell";
import { PublicShellClient as PublicShell } from "./PublicShellClient";
import { SectionTabs } from "./SectionTabs";
import { PwaInstallPrompt } from "@/components/pwa/PwaInstallPrompt";
import { Section } from "@/components/ui/Section";
import { VLibrasWidget } from "@/components/accessibility/VLibrasWidget";

const BRAND = { ...DEFAULT_BRANDING, appName: "Portal X", orgName: "Município X", appDescription: "Serviços ao cidadão", supportEmail: "a@x.gov.br", supportPhone: "3333", supportHours: "8h-17h" };

function withBrand(ui: React.ReactNode, b = BRAND) {
  return <BrandingProvider initialBranding={b}>{ui}</BrandingProvider>;
}

describe("DashboardShell", () => {
  beforeEach(() => resetNavigation({}, "/dashboard"));

  it("cabeçalho, busca global, menu, conexão em tempo real e rodapé", async () => {
    mockWebSocket();
    mockBackend({ ...identityRoutes({}, [moduleStatus("blog", { name: "Blog" })]), ...wsTicketRoute, "GET v1/lgpd/consent": { data: { accepted: true } } });
    render(
      withBrand(
        <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
          <DashboardShell userLabel="maria@x.gov.br" initialTheme="dark">
            <p>conteúdo</p>
          </DashboardShell>
        </SWRConfig>,
      ),
    );
    expect(await screen.findByText("conteúdo")).toBeInTheDocument();
    expect(screen.getAllByText("Portal X").length).toBeGreaterThan(0);
    expect(await screen.findByText("Ao vivo")).toBeInTheDocument();
    expect(screen.getByRole("contentinfo")).toHaveTextContent("a@x.gov.br");

    // busca global: mínimo de 2 caracteres
    const search = screen.getByLabelText("Buscar em todo o sistema");
    await userEvent.type(search, "a{Enter}");
    expect(router.push).not.toHaveBeenCalled();
    await userEvent.type(search, "lvará{Enter}");
    expect(router.push).toHaveBeenCalledWith("/busca?q=alvar%C3%A1");

    // alternar o menu: em tela larga recolhe a barra lateral
    const toggle = screen.getByRole("button", { name: "Alternar menu de navegação" });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const mm = vi.spyOn(window, "matchMedia").mockImplementation((q: string) => ({ matches: true, media: q, onchange: null, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent: () => false }));
    await userEvent.click(toggle);
    await waitFor(() => expect(toggle).toHaveAttribute("aria-expanded", "false"));
    await userEvent.click(toggle);
    mm.mockRestore();
    // em tela estreita abre o menu móvel
    await userEvent.click(toggle);
    await userEvent.click(toggle);
  });
});

describe("PublicShell", () => {
  beforeEach(() => resetNavigation({}, "/"));

  it("esconde links de plugins desativados; visitante entra, logado vai ao painel", async () => {
    session.status = "unauthenticated";
    const f = vi.fn(async () => new Response(JSON.stringify({ data: [{ key: "catalog", enabled: true }, { key: "calendar", enabled: false }], error: null })));
    vi.stubGlobal("fetch", f);
    const { unmount } = render(withBrand(<PublicShell><p>página</p></PublicShell>));
    const nav = screen.getByRole("navigation", { name: "Menu principal" });
    await waitFor(() => expect(within(nav).queryByRole("link", { name: "Agenda" })).not.toBeInTheDocument());
    expect(within(nav).getByRole("link", { name: "Serviços" })).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: "Sobre" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Entrar" }).closest("a")).toHaveAttribute("href", "/login");
    unmount();

    session.status = "authenticated";
    f.mockImplementation(async () => { throw new Error("fora"); });
    render(withBrand(<PublicShell><p>página</p></PublicShell>));
    expect(screen.getByRole("button", { name: "Acessar o painel" }).closest("a")).toHaveAttribute("href", "/dashboard");
    // sem saber o estado dos módulos, esconde os de plugin
    await waitFor(() => expect(screen.queryByRole("link", { name: "Serviços" })).not.toBeInTheDocument());
    vi.unstubAllGlobals();
  });

  it("com o estado vindo do servidor, plugin desativado nunca pisca no menu", async () => {
    session.status = "unauthenticated";
    let answer: (r: Response) => void = () => {};
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((r) => (answer = r))));
    render(withBrand(<PublicShell initialEnabled={{ catalog: false, directory: false, calendar: false, contact: true }}><p>página</p></PublicShell>));
    const nav = screen.getByRole("navigation", { name: "Menu principal" });
    // 1º render, antes de qualquer resposta do fetch do navegador
    expect(within(nav).queryByRole("link", { name: "Serviços" })).not.toBeInTheDocument();
    expect(within(nav).queryByRole("link", { name: "Agenda" })).not.toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: "Contato" })).toBeInTheDocument();
    // o fetch do navegador atualiza um estado em cache no servidor
    await act(async () => answer(new Response(JSON.stringify({ data: [{ key: "catalog", enabled: true }], error: null }))));
    expect(await within(nav).findByRole("link", { name: "Serviços" })).toBeInTheDocument();
    vi.unstubAllGlobals();
  });

  it("sem estado inicial, links de plugin só aparecem depois do fetch", () => {
    session.status = "unauthenticated";
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));
    render(withBrand(<PublicShell><p>página</p></PublicShell>));
    const nav = screen.getByRole("navigation", { name: "Menu principal" });
    expect(within(nav).queryByRole("link", { name: "Serviços" })).not.toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: "Sobre" })).toBeInTheDocument();
    vi.unstubAllGlobals();
  });
});

describe("barra de acessibilidade e atalhos e-MAG", () => {
  function Prefs() {
    const { branding } = useBranding();
    return <p data-testid="prefs">{`${branding.highContrast}:${branding.fontSizeScale}`}</p>;
  }

  it("alto contraste, fonte e atalhos Alt+1..4", async () => {
    session.status = "unauthenticated";
    vi.stubGlobal("fetch", vi.fn(async () => new Response('{"data":[],"error":null}')));
    render(
      <Providers initialBranding={BRAND}>
        <PublicShell>
          <Prefs />
        </PublicShell>
      </Providers>,
    );
    await userEvent.click(screen.getByRole("button", { name: /Alto contraste/ }));
    await userEvent.click(screen.getByRole("button", { name: "Aumentar fonte" }));
    expect(screen.getByTestId("prefs")).toHaveTextContent("true:110");
    await userEvent.click(screen.getByRole("button", { name: "Diminuir fonte" }));
    await userEvent.click(screen.getByRole("button", { name: "Diminuir fonte" }));
    expect(screen.getByTestId("prefs")).toHaveTextContent("true:90");
    await userEvent.click(screen.getByRole("button", { name: "Fonte padrão" }));
    expect(screen.getByTestId("prefs")).toHaveTextContent("true:100");
    await userEvent.click(screen.getByRole("button", { name: /Alto contraste/ }));
    // sem campo de busca no site público, o atalho da busca some
    expect(screen.queryByRole("link", { name: /Ir para a busca/ })).not.toBeInTheDocument();

    fireEvent.keyDown(window, { key: "1", altKey: true });
    expect(document.activeElement?.id).toBe("main-content");
    fireEvent.keyDown(window, { key: "4", code: "Digit4", altKey: true });
    expect(document.activeElement?.id).toBe("gov-footer");
    fireEvent.keyDown(window, { key: "2", altKey: true, ctrlKey: true }); // com Ctrl: ignorado
    expect(document.activeElement?.id).toBe("gov-footer");
    fireEvent.keyDown(window, { key: "9", altKey: true }); // tecla sem atalho
    fireEvent.keyDown(window, { key: "3", altKey: true }); // alvo inexistente aqui
    fireEvent.keyDown(window, { key: "1" }); // sem Alt
    vi.unstubAllGlobals();
  });

  it("focusSkipTarget torna regiões focáveis e foca campos direto", () => {
    render(
      <>
        <AccessibilityShortcuts />
        <section id="regiao">x</section>
        <input id="campo" aria-label="campo" />
      </>,
    );
    expect(focusSkipTarget("nada")).toBe(false);
    expect(focusSkipTarget("regiao")).toBe(true);
    expect(document.getElementById("regiao")).toHaveAttribute("tabindex", "-1");
    expect(focusSkipTarget("campo")).toBe(true);
    expect(document.getElementById("campo")).not.toHaveAttribute("tabindex");
  });
});

describe("componentes menores", () => {
  it("SectionTabs: índice só em match exato, demais por prefixo", () => {
    const tabs = [{ href: "/configuracao", label: "Geral" }, { href: "/configuracao/perfis", label: "Perfis" }];
    resetNavigation({}, "/configuracao/perfis/1");
    const { unmount } = render(<SectionTabs tabs={tabs} ariaLabel="Seções" />);
    expect(screen.getByRole("link", { name: "Perfis" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Geral" })).not.toHaveAttribute("aria-current");
    unmount();
    render(<SectionTabs tabs={tabs} ariaLabel="Seções" exactFirst={false} />);
    expect(screen.getByRole("link", { name: "Geral" })).toHaveAttribute("aria-current", "page");
  });

  it("Section recolhível", async () => {
    render(<Section title="Dados" description="desc" collapsible defaultExpanded={false} action={<span>ação</span>}><p>corpo</p></Section>);
    expect(screen.queryByText("corpo")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: 'Expandir seção "Dados"' }));
    expect(screen.getByText("corpo")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: 'Minimizar seção "Dados"' })).toHaveAttribute("aria-expanded", "true");
    render(<Section title="Fixa"><p>sempre</p></Section>);
    expect(screen.getByText("sempre")).toBeInTheDocument();
  });

  it("VLibras inicializa quando o script carrega; falha do widget não quebra", async () => {
    const Widget = vi.fn();
    window.VLibras = { Widget: Widget as unknown as new (url: string) => void };
    render(<VLibrasWidget />);
    expect(Widget).toHaveBeenCalledWith("https://vlibras.gov.br/app");
    window.VLibras = { Widget: vi.fn(() => { throw new Error("x"); }) as unknown as new (url: string) => void };
    await userEvent.click(screen.getByRole("button", { name: "script vlibras" }));
    delete window.VLibras;
  });

  it("PWA: oferece instalar, instala e pode ser dispensado (lembrado)", async () => {
    localStorage.clear();
    const { unmount } = render(withBrand(<PwaInstallPrompt />));
    expect(screen.queryByText(/Instalar o/)).not.toBeInTheDocument();
    const prompt = vi.fn(async () => {});
    const ev = Object.assign(new Event("beforeinstallprompt", { cancelable: true }), { prompt, userChoice: Promise.resolve({ outcome: "accepted" }) });
    act(() => void window.dispatchEvent(ev));
    expect(ev.defaultPrevented).toBe(true);
    expect(await screen.findByText("Instalar o Portal X")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Instalar" }));
    expect(prompt).toHaveBeenCalled();
    await waitFor(() => expect(screen.queryByText(/Instalar o/)).not.toBeInTheDocument());

    act(() => void window.dispatchEvent(Object.assign(new Event("beforeinstallprompt"), { prompt, userChoice: Promise.resolve({ outcome: "dismissed" }) })));
    act(() => void window.dispatchEvent(new Event("appinstalled")));
    expect(screen.queryByText(/Instalar o/)).not.toBeInTheDocument();

    act(() => void window.dispatchEvent(Object.assign(new Event("beforeinstallprompt"), { prompt, userChoice: Promise.resolve({ outcome: "dismissed" }) })));
    await userEvent.click(await screen.findByRole("button", { name: "Dispensar" }));
    expect(localStorage.getItem("nexus-pwa-install-dismissed")).toBe("1");
    unmount();
    // já dispensado: nunca mais aparece
    render(withBrand(<PwaInstallPrompt />));
    act(() => void window.dispatchEvent(Object.assign(new Event("beforeinstallprompt"), { prompt, userChoice: Promise.resolve({ outcome: "accepted" }) })));
    expect(screen.queryByText(/Instalar o/)).not.toBeInTheDocument();
  });

  it("PWA com storage bloqueado não quebra", async () => {
    const get = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("bloqueado"); });
    const set = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("bloqueado"); });
    render(withBrand(<PwaInstallPrompt />));
    act(() => void window.dispatchEvent(Object.assign(new Event("beforeinstallprompt"), { prompt: vi.fn(), userChoice: Promise.resolve({ outcome: "accepted" }) })));
    await userEvent.click(await screen.findByRole("button", { name: "Dispensar" }));
    expect(screen.queryByText(/Instalar o/)).not.toBeInTheDocument();
    get.mockRestore();
    set.mockRestore();
  });
});
