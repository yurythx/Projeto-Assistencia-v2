import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { Frame } from "@/lib/websocket/client";
import { identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

let captured: ((f: Frame) => void) | undefined;
vi.mock("@/components/realtime/RealtimeProvider", () => ({
  useFrames: (h: (f: Frame) => void) => {
    captured = h;
  },
}));
vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import NotificacoesPage from "@/app/(protected)/notificacoes/page";
import { SeguirProcedimento } from "@/components/atlas/SeguirProcedimento";

import { NotificationBell } from "./NotificationBell";
import { NotificationCenter } from "./NotificationCenter";
import { NotificationHistoryProvider, useNotificationHistory } from "./NotificationHistoryProvider";
import { ToastProvider } from "./ToastProvider";

const AVISO = {
  id: "n1",
  modulo: "atlas",
  titulo: "Procedimento atualizado: ADM.LIC.001",
  mensagem: "Versão 2 publicada.",
  link: "/atlas/procedimentos/wf-2",
  lida: false,
  created_at: "2026-10-04T12:00:00Z",
};

function Contador() {
  const { items, unreadCount, markAllRead } = useNotificationHistory();
  return (
    <div>
      <p data-testid="itens">{items.length}</p>
      <p data-testid="nao-lidas">{unreadCount}</p>
      <button onClick={markAllRead}>ler tudo</button>
    </div>
  );
}

const caixa = (
  <ToastProvider>
    <NotificationHistoryProvider>
      <Contador />
      <NotificationCenter />
      <NotificationBell />
    </NotificationHistoryProvider>
  </ToastProvider>
);

describe("caixa de notificações (ADR 027)", () => {
  beforeEach(() => resetNavigation({}, "/"));

  it("carrega a caixa do servidor, recebe aviso novo sem duplicar e marca como lido no servidor", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/notificacoes": { data: { itens: [AVISO], nao_lidas: 1 } },
      "POST v1/notificacoes/lidas": { status: 204 },
    });
    renderApp(caixa);
    await waitFor(() => expect(screen.getByTestId("itens")).toHaveTextContent("1"));
    expect(api.to("GET v1/notificacoes")[0]!.query.get("limit")).toBe("20");

    // O mesmo aviso em tempo real não duplica; um novo entra e vira toast.
    act(() => captured!({ type: "notificacao.nova", topic: "user:u1", data: AVISO } as Frame));
    act(() =>
      captured!({
        type: "notificacao.nova",
        topic: "user:u1",
        data: { ...AVISO, id: "n2", titulo: "Modelo atualizado: Ofício" },
      } as Frame),
    );
    act(() => captured!({ type: "notificacao.nova", topic: "user:u1", data: {} } as Frame));
    expect(screen.getByTestId("itens")).toHaveTextContent("2");
    expect(
      await screen.findByText("Modelo atualizado: Ofício", {
        selector: "[role=status] *, [role=alert] *, p, span, div",
      }),
    ).toBeInTheDocument();

    // Sino: o aviso leva ao link; "Ver todas" abre a caixa.
    await userEvent.click(screen.getByRole("button", { name: /Notificações, 2 não lidas/ }));
    const menu = screen.getByRole("menu");
    expect(
      within(menu).getByRole("link", { name: "Procedimento atualizado: ADM.LIC.001" }),
    ).toHaveAttribute("href", "/atlas/procedimentos/wf-2");
    expect(within(menu).getByRole("link", { name: "Ver todas e preferências" })).toHaveAttribute(
      "href",
      "/notificacoes",
    );
    await waitFor(() => expect(api.to("POST v1/notificacoes/lidas")).toHaveLength(1));
    expect(screen.getByTestId("nao-lidas")).toHaveTextContent("0");
    // Já lidas: não chama o servidor de novo.
    await userEvent.click(screen.getByText("ler tudo"));
    expect(api.to("POST v1/notificacoes/lidas")).toHaveLength(1);
  });

  it("sem caixa no servidor, o sino segue só com a sessão", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/notificacoes": { status: 403, error: { code: "FORBIDDEN", message: "x" } },
    });
    renderApp(caixa);
    await userEvent.click(screen.getByText("ler tudo"));
    expect(screen.getByTestId("itens")).toHaveTextContent("0");
  });
});

describe("seguir procedimento", () => {
  beforeEach(() => resetNavigation({}, "/atlas/procedimentos/wf-1"));

  it("segue e deixa de seguir", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/atlas/workflows/wf-1/seguir": {
        data: { codigo_processual: "ADM.LIC.001", seguindo: false },
      },
      "PUT v1/atlas/workflows/wf-1/seguir": {
        data: { codigo_processual: "ADM.LIC.001", seguindo: true },
      },
      "DELETE v1/atlas/workflows/wf-1/seguir": {
        data: { codigo_processual: "ADM.LIC.001", seguindo: false },
      },
    });
    renderApp(<SeguirProcedimento id="wf-1" />);
    await userEvent.click(await screen.findByRole("button", { name: /Seguir este fluxo/ }));
    const deixar = await screen.findByRole("button", { name: /Deixar de seguir/ });
    expect(deixar).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(deixar);
    expect(await screen.findByRole("button", { name: /Seguir este fluxo/ })).toBeInTheDocument();
    expect(api.to("PUT v1/atlas/workflows/wf-1/seguir")).toHaveLength(1);
    expect(api.to("DELETE v1/atlas/workflows/wf-1/seguir")).toHaveLength(1);
  });

  it("sem estado (ex.: sem usuário local), o botão não aparece", async () => {
    mockBackend({ ...identityRoutes() });
    const { container } = renderApp(<SeguirProcedimento id="wf-1" />);
    await new Promise((r) => setTimeout(r, 50));
    expect(container.querySelector("button")).toBeNull();
  });
});

describe("página de notificações", () => {
  beforeEach(() => resetNavigation({}, "/notificacoes"));

  it("lista, marca como lida, marca todas e muda a preferência do módulo", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/notificacoes": {
        data: {
          itens: [
            AVISO,
            { ...AVISO, id: "n0", titulo: "Antigo", link: "", lida: true, mensagem: "" },
          ],
          nao_lidas: 1,
        },
      },
      "POST v1/notificacoes/n1/lida": { status: 204 },
      "POST v1/notificacoes/lidas": { status: 204 },
      "GET v1/notificacoes/preferencias": { data: [{ modulo: "tramite", ativo: false }] },
      "PUT v1/notificacoes/preferencias": { data: { modulo: "atlas", ativo: false } },
    });
    renderApp(<NotificacoesPage />);
    const lista = await screen.findByRole("list", { name: "Notificações" });
    expect(
      within(lista).getByRole("link", { name: "Procedimento atualizado: ADM.LIC.001" }),
    ).toHaveAttribute("href", "/atlas/procedimentos/wf-2");
    expect(within(lista).getByText("Antigo")).toBeInTheDocument();
    await userEvent.click(within(lista).getByRole("button", { name: "Marcar como lida" }));
    await waitFor(() => expect(api.to("POST v1/notificacoes/n1/lida")).toHaveLength(1));
    await userEvent.click(screen.getByRole("button", { name: "Marcar todas como lidas" }));
    await waitFor(() => expect(api.to("POST v1/notificacoes/lidas")).toHaveLength(1));

    expect(await screen.findByRole("switch", { name: "Avisos do Trâmite" })).toHaveAttribute(
      "aria-checked",
      "false",
    );
    await userEvent.click(screen.getByRole("switch", { name: "Avisos do Atlas" }));
    await waitFor(() =>
      expect(api.to("PUT v1/notificacoes/preferencias")[0]!.body).toEqual({
        modulo: "atlas",
        ativo: false,
      }),
    );
    // Clicar no aviso com link também o marca como lido.
    await userEvent.click(
      within(lista).getByRole("link", { name: "Procedimento atualizado: ADM.LIC.001" }),
    );
    await waitFor(() => expect(api.to("POST v1/notificacoes/n1/lida")).toHaveLength(2));
  });

  it("caixa vazia", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/notificacoes": { data: { itens: [], nao_lidas: 0 } },
      "GET v1/notificacoes/preferencias": { data: [] },
    });
    renderApp(<NotificacoesPage />);
    expect(await screen.findByText("Nenhuma notificação")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Marcar todas como lidas" }),
    ).not.toBeInTheDocument();
  });
});
