import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, ME, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import AgendaPage from "./page";

const now = new Date();
const at = (day: number, hour: number) => new Date(now.getFullYear(), now.getMonth(), day, hour).toISOString();

const room = (id: string, extra: Record<string, unknown> = {}) => ({ id, name: `Sala ${id}`, location: "Bloco A", capacity: 10, resources: [], active: true, ...extra });
const event = (id: string, extra: Record<string, unknown> = {}) => ({
  id, title: `Evento ${id}`, description: "", location: "", starts_at: at(10, 9), ends_at: at(10, 10), all_day: false,
  visibility: "internal", status: "confirmed", organizer_id: ME.id, organizer_name: "Maria", ...extra,
});

describe("Agenda", () => {
  beforeEach(() => resetNavigation());

  it("agrupa por dia, navega entre meses e filtra por sala", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["calendar:read"] }),
      "GET v1/calendar/events": {
        data: [
          event("e1", { room_name: "Sala r1", description: "Pauta" }),
          event("e2", { all_day: true, location: "https://meet", visibility: "public", organizer_id: "outro" }),
          event("e3", { status: "cancelled", starts_at: at(12, 14), ends_at: at(12, 15), visibility: "private" }),
        ],
      },
      "GET v1/calendar/rooms": { data: [room("r1")] },
    });
    renderApp(<AgendaPage />);
    expect(await screen.findByText("Evento e1")).toBeInTheDocument();
    expect(screen.getByText("Pauta")).toBeInTheDocument();
    expect(screen.getByText("Dia inteiro")).toBeInTheDocument();
    expect(screen.getByText("https://meet")).toBeInTheDocument();
    expect(screen.getByText("Cancelado")).toBeInTheDocument();
    expect(screen.getByText("Pública")).toBeInTheDocument();
    // dono edita os próprios; evento de outro, e cancelado, não
    expect(screen.getByRole("button", { name: "Editar Evento e1" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Editar Evento e2" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Editar Evento e3" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Salas/ })).not.toBeInTheDocument();

    const first = api.to("GET v1/calendar/events")[0]!.query;
    await userEvent.click(screen.getByRole("button", { name: "Próximo mês" }));
    await waitFor(() => expect(api.to("GET v1/calendar/events").at(-1)!.query.get("from")).toBe(first.get("to")));
    await userEvent.click(screen.getByRole("button", { name: "Mês anterior" }));
    await userEvent.click(screen.getByRole("button", { name: "Mês anterior" }));
    await waitFor(() => expect(api.to("GET v1/calendar/events").at(-1)!.query.get("to")).toBe(first.get("from")));

    await userEvent.selectOptions(screen.getByLabelText("Sala"), "r1");
    await waitFor(() => expect(api.to("GET v1/calendar/events").at(-1)!.query.get("room_id")).toBe("r1"));
  });

  it("mês vazio e erro", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/calendar/events": (req) => (req.query.get("room_id") ? fail(500, "agenda fora") : { data: [] }),
      "GET v1/calendar/rooms": { data: [room("r1")] },
    });
    renderApp(<AgendaPage />);
    expect(await screen.findByText("Nenhum evento neste mês")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Sala"), "r1");
    expect(await screen.findByText("agenda fora")).toBeInTheDocument();
  });

  it("cria evento com reserva (só salas ativas)", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/calendar/events": { data: [] },
      "GET v1/calendar/rooms": { data: [room("r1", { capacity: 0 }), room("r2", { active: false })] },
      "POST v1/calendar/events": { data: event("novo") },
    });
    renderApp(<AgendaPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Novo evento/ }));
    const dialog = await screen.findByRole("dialog", { name: "Novo evento" });
    expect(within(dialog).queryByRole("option", { name: /Sala r2/ })).not.toBeInTheDocument();
    await userEvent.type(within(dialog).getByLabelText("Título *"), " Reunião ");
    await userEvent.clear(within(dialog).getByLabelText("Início *"));
    await userEvent.type(within(dialog).getByLabelText("Início *"), "2026-03-10T09:00");
    await userEvent.clear(within(dialog).getByLabelText("Término *"));
    await userEvent.type(within(dialog).getByLabelText("Término *"), "2026-03-10T10:30");
    await userEvent.selectOptions(within(dialog).getByLabelText("Sala (reserva)"), "r1");
    await userEvent.selectOptions(within(dialog).getByLabelText("Visibilidade"), "public");
    await userEvent.click(within(dialog).getByLabelText("Dia inteiro"));
    await userEvent.type(within(dialog).getByLabelText("Local / link"), "Auditório");
    await userEvent.type(within(dialog).getByLabelText("Descrição"), "Pauta");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar evento" }));
    await waitFor(() => expect(api.to("POST v1/calendar/events")).toHaveLength(1));
    expect(api.to("POST v1/calendar/events")[0]!.body).toEqual({
      title: "Reunião", description: "Pauta", location: "Auditório", room_id: "r1",
      starts_at: new Date("2026-03-10T09:00").toISOString(), ends_at: new Date("2026-03-10T10:30").toISOString(),
      all_day: true, visibility: "public", publico: { entidades: [], unidades: [] },
    });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Novo evento" })).not.toBeInTheDocument());
  });

  it("público-alvo (ADR 014): restrito tira a opção Pública e vai no corpo; a lista mostra o selo", async () => {
    const ORG = [{ id: "e1", nome: "Saúde", sigla: "SEMSA", ativo: true, unidades: [{ id: "u1", entidade_id: "e1", nome: "UPA", sigla: "", departamentos: [] }] }];
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/iam/org-tree": { data: ORG },
      "GET v1/calendar/events": { data: [event("e7", { publico: { entidades: ["e1"], unidades: [] } })] },
      "GET v1/calendar/rooms": { data: [] },
      "POST v1/calendar/events": { data: event("novo") },
    });
    renderApp(<AgendaPage />);
    expect(await screen.findByText("Para: SEMSA")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Novo evento/ }));
    const dialog = await screen.findByRole("dialog", { name: "Novo evento" });
    await userEvent.type(within(dialog).getByLabelText("Título *"), "Reunião da UPA");
    expect(within(dialog).getByRole("option", { name: "Pública (site institucional)" })).toBeInTheDocument();
    await userEvent.click(within(dialog).getByText("SEMSA — Saúde"));
    await userEvent.click(within(dialog).getByLabelText("UPA"));
    expect(within(dialog).queryByRole("option", { name: "Pública (site institucional)" })).not.toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar evento" }));
    await waitFor(() => expect(api.to("POST v1/calendar/events")[0]?.body).toMatchObject({ visibility: "internal", publico: { entidades: [], unidades: ["u1"] } }));
  });

  it("conflito de sala volta como erro e o formulário fica aberto", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/calendar/events": { data: [] },
      "GET v1/calendar/rooms": { data: [] },
      "POST v1/calendar/events": fail(409, "sala já reservada neste horário"),
    });
    renderApp(<AgendaPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Novo evento/ }));
    const dialog = await screen.findByRole("dialog", { name: "Novo evento" });
    await userEvent.type(within(dialog).getByLabelText("Título *"), "X");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar evento" }));
    expect(await screen.findByText("sala já reservada neste horário")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Novo evento" })).toBeInTheDocument();
  });

  it("edita evento numa sala desativada sem perder a reserva; cancela e exclui", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/calendar/events": { data: [event("e1", { room_id: "r2", room_name: "Sala r2", location: "x" })] },
      "GET v1/calendar/rooms": { data: [room("r1"), room("r2", { active: false })] },
      "PUT v1/calendar/events/e1": { data: event("e1") },
      "POST v1/calendar/events/e1/cancel": { data: null },
      "DELETE v1/calendar/events/e1": { data: null },
    });
    renderApp(<AgendaPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Editar Evento e1" }));
    const dialog = await screen.findByRole("dialog", { name: "Editar evento" });
    expect(within(dialog).getByLabelText("Sala (reserva)")).toHaveValue("r2");
    expect(within(dialog).getByRole("option", { name: "Sala r2 · 10 lugares (inativa)" })).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar evento" }));
    await waitFor(() => expect(api.to("PUT v1/calendar/events/e1")).toHaveLength(1));
    expect(api.to("PUT v1/calendar/events/e1")[0]!.body).toMatchObject({ room_id: "r2", title: "Evento e1" });

    await userEvent.click(screen.getByRole("button", { name: "Cancelar Evento e1" }));
    let confirm = await screen.findByRole("dialog", { name: "Cancelar evento?" });
    await userEvent.click(within(confirm).getByRole("button", { name: "Cancelar evento" }));
    await waitFor(() => expect(api.to("POST v1/calendar/events/e1/cancel")).toHaveLength(1));

    await userEvent.click(screen.getByRole("button", { name: "Excluir Evento e1" }));
    confirm = await screen.findByRole("dialog", { name: "Excluir evento?" });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    await waitFor(() => expect(api.to("DELETE v1/calendar/events/e1")).toHaveLength(1));
  });

  it("gestor administra as salas", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/calendar/events": { data: [event("e9", { organizer_id: "outro" })] },
      "GET v1/calendar/rooms": { data: [room("r1", { resources: ["projetor"], location: "" }), room("r2", { active: false })] },
      "POST v1/calendar/rooms": { data: room("r3") },
      "PUT v1/calendar/rooms/r1": { data: room("r1") },
      "DELETE v1/calendar/rooms/r2": { data: null },
    });
    renderApp(<AgendaPage />);
    // calendar:manage edita evento de outra pessoa
    expect(await screen.findByRole("button", { name: "Editar Evento e9" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Salas/ }));
    const dialog = await screen.findByRole("dialog", { name: "Salas de reunião" });
    expect(within(dialog).getByText("projetor")).toBeInTheDocument();
    expect(within(dialog).getByText("Inativa")).toBeInTheDocument();

    await userEvent.type(within(dialog).getByLabelText("Nome *"), "Sala Azul");
    await userEvent.type(within(dialog).getByLabelText("Localização"), "2º andar");
    await userEvent.clear(within(dialog).getByLabelText("Capacidade"));
    await userEvent.type(within(dialog).getByLabelText("Capacidade"), "12");
    await userEvent.type(within(dialog).getByLabelText("Recursos (separados por vírgula)"), "tv, , videoconferência ");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar sala" }));
    await waitFor(() => expect(api.to("POST v1/calendar/rooms")).toHaveLength(1));
    expect(api.to("POST v1/calendar/rooms")[0]!.body).toEqual({ name: "Sala Azul", location: "2º andar", capacity: 12, resources: ["tv", "videoconferência"], active: true, unidade_id: null });

    await userEvent.click(within(dialog).getByRole("button", { name: "Editar Sala r1" }));
    expect(within(dialog).getByLabelText("Recursos (separados por vírgula)")).toHaveValue("projetor");
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancelar" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Editar Sala r1" }));
    await userEvent.click(within(dialog).getByLabelText("Ativa"));
    await userEvent.clear(within(dialog).getByLabelText("Capacidade"));
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar sala" }));
    await waitFor(() => expect(api.to("PUT v1/calendar/rooms/r1")).toHaveLength(1));
    expect(api.to("PUT v1/calendar/rooms/r1")[0]!.body).toMatchObject({ active: false, capacity: 0 });

    await userEvent.click(within(dialog).getByRole("button", { name: "Excluir Sala r2" }));
    const confirm = await screen.findByRole("dialog", { name: 'Excluir a sala "Sala r2"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    await waitFor(() => expect(api.to("DELETE v1/calendar/rooms/r2")).toHaveLength(1));
  });

  it("gestor com escopo (ADR 013): só as salas da unidade dele e os eventos reservados nelas", async () => {
    const ORG = [{ id: "e1", nome: "Órgão", unidades: [{ id: "u1", entidade_id: "e1", nome: "Protocolo", departamentos: [] }] }];
    mockBackend({
      ...identityRoutes({ permissions: ["calendar:manage"], scopes: [{ perfil: "p", origem: "manual", entidade_id: "e1", unidade_id: "u1", permissions: ["calendar:manage"] }] }),
      "GET v1/calendar/events": { data: [event("e1", { organizer_id: "outro", room_id: "r1" }), event("e2", { organizer_id: "outro", room_id: "r2" }), event("e3", { organizer_id: "outro" })] },
      "GET v1/calendar/rooms": { data: [room("r1", { unidade_id: "u1" }), room("r2")] },
      "GET v1/iam/org-tree": { data: ORG },
    });
    renderApp(<AgendaPage />);
    expect(await screen.findByRole("button", { name: "Editar Evento e1" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Editar Evento e2" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Editar Evento e3" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Salas/ }));
    const dialog = await screen.findByRole("dialog", { name: "Salas de reunião" });
    expect(within(dialog).getByRole("button", { name: "Editar Sala r1" })).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Editar Sala r2" })).not.toBeInTheDocument();
    const dona = within(dialog).getByLabelText("Unidade dona *");
    expect(dona).toBeRequired();
    expect(within(dona).getByRole("option", { name: "Protocolo" })).toBeInTheDocument();
  });

  it("sem salas cadastradas", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/calendar/events": { data: [] }, "GET v1/calendar/rooms": { data: [] } });
    renderApp(<AgendaPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Salas/ }));
    expect(await screen.findByText("Nenhuma sala cadastrada.")).toBeInTheDocument();
  });
});
