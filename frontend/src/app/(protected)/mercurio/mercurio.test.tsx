import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, ME, mockBackend, mockWebSocket, renderRealtime, wsTicketRoute } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import MercurioPage from "./page";

const room = (id: string, kind: string, extra: Record<string, unknown> = {}) => ({
  id, kind, name: `Sala ${id}`, description: "", archived: false, unread: 0, ...extra,
});
const msg = (id: string, extra: Record<string, unknown> = {}) => ({
  id, room_id: "r1", author_id: "u2", author_name: "João", body: `mensagem ${id}`,
  created_at: "2026-01-01T10:00:00Z", deleted: false, ...extra,
});

const ROOMS = [
  room("r1", "global", { description: "Avisos gerais" }),
  room("r2", "department", { unread: 3, ad_group: "CN=TI", departamento_id: "d1" }),
  room("r3", "direct"),
];

function setup(routes: Parameters<typeof mockBackend>[0] = {}, permissions = ["*"]) {
  const ws = mockWebSocket();
  const api = mockBackend({
    ...identityRoutes({ permissions }),
    ...wsTicketRoute,
    "GET v1/mercurio/rooms": { data: ROOMS },
    "GET v1/mercurio/rooms/r1/messages": { data: [msg("m2"), msg("m1", { author_id: ME.id })] },
    "POST v1/mercurio/rooms/r1/read": { data: null },
    ...routes,
  });
  renderRealtime(<MercurioPage />);
  return { api, ws };
}

async function topicFrame(ws: ReturnType<typeof mockWebSocket>, frame: { type: string; data?: unknown; topic?: string }) {
  await waitFor(() => expect(ws.socket().sent).toContainEqual({ type: "subscribe", topic: "mercurio:room:r1" }));
  act(() => ws.socket().receive({ topic: "mercurio:room:r1", ...frame }));
}

describe("Mercúrio", () => {
  beforeEach(() => resetNavigation({}, "/mercurio"));

  it("agrupa as salas, abre a primeira e marca como lida", async () => {
    const { api } = setup();
    const nav = screen.getByRole("navigation", { name: "Salas do Mercúrio" });
    expect(await within(nav).findByText("Canais")).toBeInTheDocument();
    expect(within(nav).getByText("Departamentos")).toBeInTheDocument();
    expect(within(nav).getByText("Conversas diretas")).toBeInTheDocument();
    expect(within(nav).getByText("não lidas", { exact: false })).toBeInTheDocument();
    expect(within(nav).getByRole("button", { name: /Sala r1/ })).toHaveAttribute("aria-current", "page");

    const sala = await screen.findByRole("region", { name: "Sala Sala r1" });
    // a API devolve as mais recentes primeiro; a tela mostra em ordem
    const items = await within(sala).findAllByRole("listitem");
    expect(items[0]).toHaveTextContent("Você");
    expect(items[1]).toHaveTextContent("mensagem m2");
    await waitFor(() => expect(api.to("POST v1/mercurio/rooms/r1/read")).toHaveLength(1));

    await userEvent.click(within(nav).getByRole("button", { name: /Sala r2/ }));
    expect(router.replace).toHaveBeenCalledWith("/mercurio?sala=r2");
    await userEvent.selectOptions(screen.getByLabelText("Sala"), "r3");
    expect(router.replace).toHaveBeenCalledWith("/mercurio?sala=r3");
  });

  it("sem salas e sala escolhida pela URL", async () => {
    mockWebSocket();
    mockBackend({ ...identityRoutes(), ...wsTicketRoute, "GET v1/mercurio/rooms": { data: [] } });
    renderRealtime(<MercurioPage />);
    expect(await screen.findByText("Nenhuma sala")).toBeInTheDocument();
    expect(screen.getByText("Selecione uma sala.")).toBeInTheDocument();
  });

  it("envia com Enter, edita a própria mensagem e remove", async () => {
    const { api } = setup({
      "POST v1/mercurio/rooms/r1/messages": (req) => ({ data: msg("m3", { author_id: ME.id, body: (req.body as { body: string }).body }) }),
      "PATCH v1/mercurio/messages/m1": (req) => ({ data: msg("m1", { author_id: ME.id, body: (req.body as { body: string }).body, edited_at: "2026-01-01T11:00:00Z" }) }),
      "DELETE v1/mercurio/messages/m2": { data: null },
    });
    const box = await screen.findByLabelText("Mensagem");
    await screen.findByText("mensagem m2");

    // só espaços não envia
    await userEvent.type(box, "   {Enter}");
    expect(api.to("POST v1/mercurio/rooms/r1/messages")).toHaveLength(0);

    await userEvent.type(box, "olá{Shift>}{Enter}{/Shift}mundo{Enter}");
    expect(await screen.findByText(/olá\s+mundo/)).toBeInTheDocument();
    expect(api.to("POST v1/mercurio/rooms/r1/messages")[0]!.body).toEqual({ body: "olá\nmundo" });
    expect(box).toHaveValue("");

    // editar: Esc cancela, depois salva
    await userEvent.click(screen.getAllByRole("button", { name: "Editar mensagem" })[0]!);
    const editBox = screen.getByLabelText("Editar mensagem", { selector: "textarea" });
    expect(editBox).toHaveValue("mensagem m1");
    await userEvent.type(editBox, "{Escape}");
    expect(screen.getByLabelText("Mensagem")).toHaveValue("");

    await userEvent.click(screen.getAllByRole("button", { name: "Editar mensagem" })[0]!);
    await userEvent.clear(screen.getByLabelText("Editar mensagem", { selector: "textarea" }));
    await userEvent.type(screen.getByLabelText("Editar mensagem", { selector: "textarea" }), "corrigida");
    await userEvent.click(screen.getByRole("button", { name: "Salvar edição" }));
    expect(await screen.findByText("corrigida")).toBeInTheDocument();
    expect(screen.getByText(/· editada/)).toBeInTheDocument();

    // mensagem de outra pessoa: gestor remove, mas não edita
    const other = screen.getByText("mensagem m2").closest("li")!;
    expect(within(other).queryByRole("button", { name: "Editar mensagem" })).not.toBeInTheDocument();
    await userEvent.click(within(other).getByRole("button", { name: "Remover mensagem" }));
    await waitFor(() => expect(api.to("DELETE v1/mercurio/messages/m2")).toHaveLength(1));
  });

  it("falha ao enviar mantém o rascunho e avisa", async () => {
    setup({ "POST v1/mercurio/rooms/r1/messages": fail(403, "sala arquivada") }, ["mercurio:read"]);
    const box = await screen.findByLabelText("Mensagem");
    await screen.findByText("mensagem m2");
    // sem mercurio:manage: não remove mensagem alheia
    expect(within(screen.getByText("mensagem m2").closest("li")!).queryByRole("button", { name: "Remover mensagem" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Gerenciar salas" })).not.toBeInTheDocument();
    await userEvent.type(box, "oi");
    await userEvent.click(screen.getByRole("button", { name: "Enviar" }));
    expect(await screen.findByText("sala arquivada")).toBeInTheDocument();
    expect(box).toHaveValue("oi");
  });

  it("frames em tempo real: nova, editada, removida, digitando e sala derrubada", async () => {
    const { ws, api } = setup();
    await screen.findByText("mensagem m2");

    await topicFrame(ws, { type: "mercurio.typing", data: { user_id: "u2", username: "joao" } });
    expect(screen.getByText("joao está digitando…")).toBeInTheDocument();
    act(() => ws.socket().receive({ topic: "mercurio:room:r1", type: "mercurio.typing", data: { user_id: "u9", username: "ana" } }));
    expect(screen.getByText("joao, ana estão digitando…")).toBeInTheDocument();
    // o próprio usuário digitando não aparece
    act(() => ws.socket().receive({ topic: "mercurio:room:r1", type: "mercurio.typing", data: { user_id: ME.id, username: "maria" } }));
    expect(screen.queryByText(/maria/)).not.toBeInTheDocument();

    // mensagem nova limpa o "digitando" do autor e marca lida; duplicada é ignorada
    act(() => ws.socket().receive({ topic: "mercurio:room:r1", type: "mercurio.message", data: msg("m9", { body: "chegou" }) }));
    act(() => ws.socket().receive({ topic: "mercurio:room:r1", type: "mercurio.message", data: msg("m9", { body: "chegou" }) }));
    expect(screen.getAllByText("chegou")).toHaveLength(1);
    expect(screen.getByText("ana está digitando…")).toBeInTheDocument();
    await waitFor(() => expect(api.to("POST v1/mercurio/rooms/r1/read").length).toBeGreaterThanOrEqual(2));

    act(() => ws.socket().receive({ topic: "mercurio:room:r1", type: "mercurio.message.edited", data: msg("m9", { body: "chegou (editada)" }) }));
    expect(screen.getByText("chegou (editada)")).toBeInTheDocument();
    act(() => ws.socket().receive({ topic: "mercurio:room:r1", type: "mercurio.message.deleted", data: { id: "m9" } }));
    expect(screen.getByText("mensagem removida")).toBeInTheDocument();

    // frame de outro tópico é ignorado; o de não lida recarrega as salas
    act(() => ws.socket().receive({ topic: "mercurio:room:r2", type: "mercurio.message", data: msg("x", { body: "outra sala" }) }));
    expect(screen.queryByText("outra sala")).not.toBeInTheDocument();
    const before = api.to("GET v1/mercurio/rooms").length;
    act(() => ws.socket().receive({ topic: "user:1", type: "mercurio.unread" }));
    await waitFor(() => expect(api.to("GET v1/mercurio/rooms").length).toBeGreaterThan(before));

    act(() => ws.socket().receive({ topic: "mercurio:room:r1", type: "topic.dropped" }));
    expect(screen.getByText(/foi arquivada ou seu acesso foi removido/)).toHaveAttribute("role", "status");
  });

  it("indicador de digitação expira e o envio de 'digitando' é espaçado", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { ws } = setup();
      await screen.findByText("mensagem m2");
      await topicFrame(ws, { type: "mercurio.typing", data: { user_id: "u2", username: "joao" } });
      expect(screen.getByText("joao está digitando…")).toBeInTheDocument();
      act(() => void vi.advanceTimersByTime(5000));
      expect(screen.queryByText(/digitando/)).not.toBeInTheDocument();

      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      await user.type(screen.getByLabelText("Mensagem"), "abc");
      expect(ws.socket().sent.filter((f) => f.type === "mercurio.typing")).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("carrega mensagens anteriores quando há mais de uma página", async () => {
    const full = Array.from({ length: 50 }, (_, i) => msg(`n${i}`, { body: `recente ${i}` }));
    const { api } = setup({
      "GET v1/mercurio/rooms/r1/messages": (req) => ({ data: req.query.get("before") ? [msg("old", { body: "bem antiga" })] : full }),
    });
    await userEvent.click(await screen.findByRole("button", { name: "Carregar mensagens anteriores" }));
    expect(await screen.findByText("bem antiga")).toBeInTheDocument();
    expect(api.to("GET v1/mercurio/rooms/r1/messages").at(-1)!.query.get("before")).toBe("2026-01-01T10:00:00Z");
    expect(screen.queryByRole("button", { name: "Carregar mensagens anteriores" })).not.toBeInTheDocument();
  });

  it("sala sem mensagens e falha ao carregar", async () => {
    setup({ "GET v1/mercurio/rooms/r1/messages": fail(500) });
    expect(await screen.findByText("Nenhuma mensagem ainda")).toBeInTheDocument();
  });

  it("gestor cria e edita salas", async () => {
    const { api } = setup({
      "GET v1/iam/org-tree": { data: [{ id: "e1", nome: "Órgão", unidades: [{ id: "u1", nome: "Unidade", sigla: "UN", departamentos: [{ id: "d1", nome: "TI" }] }] }] },
      "POST v1/mercurio/rooms": { data: room("r9", "department") },
      "PUT v1/mercurio/rooms/r2": { data: room("r2", "department") },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Gerenciar salas" }));
    const dialog = await screen.findByRole("dialog", { name: "Salas do Mercúrio" });
    expect(within(dialog).getByText("CN=TI")).toBeInTheDocument();
    expect(within(dialog).queryByText("Sala r3")).not.toBeInTheDocument(); // conversa direta não é administrada

    await userEvent.selectOptions(within(dialog).getByLabelText("Tipo"), "department");
    await userEvent.type(within(dialog).getByLabelText("Nome *"), "Sala da TI");
    await userEvent.selectOptions(await within(dialog).findByLabelText("Departamento"), "d1");
    await userEvent.type(within(dialog).getByLabelText("Grupo do AD"), "CN=Infra");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar sala" }));
    await waitFor(() => expect(api.to("POST v1/mercurio/rooms")).toHaveLength(1));
    expect(api.to("POST v1/mercurio/rooms")[0]!.body).toEqual({
      kind: "department", name: "Sala da TI", description: "", ad_group: "CN=Infra", departamento_id: "d1", archived: false,
    });

    await userEvent.click(within(dialog).getByRole("button", { name: "Editar Sala r1" }));
    expect(within(dialog).getByText("Editar Sala r1")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancelar" }));
    expect(within(dialog).getByText("Nova sala")).toBeInTheDocument();

    await userEvent.click(within(dialog).getByRole("button", { name: "Editar Sala r2" }));
    await userEvent.click(within(dialog).getByLabelText(/Arquivada/));
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar sala" }));
    await waitFor(() => expect(api.to("PUT v1/mercurio/rooms/r2")).toHaveLength(1));
    expect(api.to("PUT v1/mercurio/rooms/r2")[0]!.body).toMatchObject({ kind: "department", archived: true, departamento_id: "d1" });
  });
});
