import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  fail,
  identityRoutes,
  mockBackend,
  mockWebSocket,
  renderApp,
  renderRealtime,
  wsTicketRoute,
} from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import SemprasPage from "./page";
import FichaPage from "./cidadaos/[id]/page";

const UNIDADES = [
  { id: "u1", entidade_id: "e1", nome: "CRAS Central", sigla: "", ativa: true },
  { id: "u2", entidade_id: "e1", nome: "CREAS", sigla: "", ativa: true },
];

const item = (id: string, extra: Record<string, unknown> = {}) => ({
  id,
  cidadao_id: `c-${id}`,
  unidade_id: "u1",
  dia: "2026-10-08",
  senha: 1,
  senha_exibida: `N00${id}`,
  tipo: "social",
  prioridade: "",
  status: "aguardando",
  local: "",
  nome: `Pessoa ${id}`,
  created_at: "2026-10-08T10:00:00Z",
  chamado_em: null,
  iniciado_em: null,
  encerrado_em: null,
  ...extra,
});

const recepcionista = { permissions: ["sempras:recepcao"], scopes: [] };

describe("SEMPRAS — atendimento", () => {
  beforeEach(() => resetNavigation());

  it("recepção busca, cadastra e inclui na fila; a fila se atualiza pelo tempo real", async () => {
    const ws = mockWebSocket();
    let fila = [item("1")];
    const api = mockBackend({
      ...identityRoutes(recepcionista),
      ...wsTicketRoute,
      "GET v1/sempras/unidades": { data: UNIDADES },
      "GET v1/sempras/unidades/u1/fila": () => ({ data: fila }),
      "GET v1/sempras/cidadaos": (req) =>
        req.query.get("q") === "ninguem"
          ? { data: [] }
          : { data: [{ id: "c9", nome: "Maria", nome_social: "", cpf: "***.982.247-**", nis: "", data_nascimento: "", atendimentos_na_area: 2 }] },
      "POST v1/sempras/cidadaos": { status: 201, data: { id: "c10", nome: "Nova Pessoa", nome_social: "", cpf: "", nis: "", data_nascimento: "2000-01-01" } },
      "POST v1/sempras/unidades/u1/fila": { status: 201, data: item("2") },
      "POST v1/sempras/atendimentos/1/cancelar": { data: item("1", { status: "cancelado" }) },
    });
    renderRealtime(<SemprasPage />);

    expect(await screen.findByText("Pessoa 1")).toBeInTheDocument();
    expect(screen.getByLabelText("Unidade")).toHaveValue("u1");
    // recepção não chama nem vê o painel
    expect(screen.queryByRole("button", { name: /Chamar próximo/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/Painel das unidades/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Chamar N001" })).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("CPF, NIS ou nome"), "529.982.247-25");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(await screen.findByText("2 atendimento(s) na área")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Ficha" })).not.toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Prioridade legal"), "gestante");
    await userEvent.click(screen.getByRole("button", { name: "Incluir Maria na fila" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/unidades/u1/fila")[0]!.body).toEqual({ cidadao_id: "c9", tipo: "social", prioridade: "gestante" }),
    );

    // nada encontrado → cadastra
    await userEvent.clear(screen.getByLabelText("CPF, NIS ou nome"));
    await userEvent.type(screen.getByLabelText("CPF, NIS ou nome"), "ninguem");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(await screen.findByText(/Ninguém encontrado/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Cadastrar/ }));
    const dialog = await screen.findByRole("dialog", { name: "Cadastrar munícipe" });
    await userEvent.type(within(dialog).getByLabelText("Nome civil *"), "Nova Pessoa");
    await userEvent.type(within(dialog).getByLabelText("Nome da mãe"), "Ana");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar cadastro" }));
    expect(await screen.findByText("Sem histórico na área")).toBeInTheDocument();
    expect(api.to("POST v1/sempras/cidadaos")[0]!.body).toMatchObject({ nome: "Nova Pessoa", nome_mae: "Ana", cpf: "" });

    // cancelar quem aguarda
    await userEvent.click(screen.getByRole("button", { name: "Cancelar N001" }));
    await waitFor(() => expect(api.to("POST v1/sempras/atendimentos/1/cancelar")).toHaveLength(1));

    // frame do tópico da unidade → recarrega a fila
    fila = [item("1"), item("3", { nome: "Chegou Agora" })];
    await waitFor(() => expect(ws.socket().sent).toContainEqual({ type: "subscribe", topic: "sempras:fila:u1" }));
    act(() => ws.socket().receive({ topic: "sempras:fila:u1", type: "sempras.fila", data: { status: "aguardando" } }));
    expect(await screen.findByText("Chegou Agora")).toBeInTheDocument();
  });

  it("técnico chama o próximo e conduz o atendimento; a gestão vê o painel", async () => {
    mockWebSocket();
    const api = mockBackend({
      ...identityRoutes(),
      ...wsTicketRoute,
      "GET v1/sempras/unidades": { data: UNIDADES },
      "GET v1/sempras/unidades/u1/fila": {
        data: [
          item("1", { status: "em_atendimento", local: "Sala 2" }),
          item("2", { status: "chamado", tipo: "psicologico", prioridade: "idoso_80" }),
          item("3"),
          item("4", { status: "concluido" }),
        ],
      },
      "GET v1/sempras/unidades/u2/fila": { data: [] },
      "GET v1/sempras/painel": {
        data: [{ unidade: UNIDADES[0], aguardando: 1, em_atendimento: 1, concluidos: 4, nao_compareceram: 0, espera_media_min: 12.4 }],
      },
      "POST v1/sempras/unidades/u1/fila/proximo": { data: item("3", { status: "chamado" }) },
      "POST v1/sempras/atendimentos/*": (req) =>
        req.path.endsWith("/concluir") ? fail(409, "o atendimento não está no estado que a ação exige") : { data: item("x") },
    });
    renderRealtime(<SemprasPage />);

    expect(await screen.findByText("Pessoa 1")).toBeInTheDocument();
    expect(within(screen.getByRole("list", { name: "Fila" })).getByText(/Idoso 80\+/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Pessoa 1" })).toHaveAttribute("href", "/sempras/cidadaos/c-1");
    expect(await screen.findByText("12 min")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Sala ou guichê"), "Sala 3");
    await userEvent.click(screen.getByRole("button", { name: /Chamar próximo/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/unidades/u1/fila/proximo")[0]!.body).toEqual({ local: "Sala 3" }));

    await userEvent.click(screen.getByRole("button", { name: "Iniciar N002" }));
    await userEvent.click(screen.getByRole("button", { name: "Não compareceu N002" }));
    await userEvent.click(screen.getByRole("button", { name: "Chamar N003" }));
    await userEvent.click(screen.getByRole("button", { name: "Concluir N001" }));
    expect(await screen.findByText("o atendimento não está no estado que a ação exige")).toBeInTheDocument();
    const acoes = api.calls.filter((r) => r.method === "POST" && r.path.startsWith("v1/sempras/atendimentos/")).map((r) => r.path);
    expect(acoes).toEqual([
      "v1/sempras/atendimentos/2/iniciar",
      "v1/sempras/atendimentos/2/ausente",
      "v1/sempras/atendimentos/3/chamar",
      "v1/sempras/atendimentos/1/concluir",
    ]);
    // concluído não tem ação
    expect(screen.queryByRole("button", { name: /N004/ })).not.toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Unidade"), "u2");
    expect(await screen.findByText("Ninguém na fila hoje")).toBeInTheDocument();
  });

  it("sem lotação: explica como obter acesso", async () => {
    mockBackend({ ...identityRoutes({ permissions: [], scopes: [] }), "GET v1/sempras/unidades": { data: [] } });
    renderApp(<SemprasPage />);
    expect(await screen.findByText("Você não está lotado em nenhuma unidade da SEMPRAS")).toBeInTheDocument();
  });

  it("recepção sem permissão de atendimento: o psicológico chamado só pode ser cancelado", async () => {
    mockWebSocket();
    mockBackend({
      ...identityRoutes(recepcionista),
      ...wsTicketRoute,
      "GET v1/sempras/unidades": { data: UNIDADES },
      "GET v1/sempras/unidades/u1/fila": { data: [item("5", { status: "chamado", tipo: "psicologico" }), item("6", { status: "em_atendimento" })] },
    });
    renderRealtime(<SemprasPage />);
    expect(await screen.findByRole("button", { name: "Cancelar N005" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Iniciar N005" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /N006/ })).not.toBeInTheDocument();
  });
});

const PRONTUARIO = {
  cidadao: { id: "c1", nome: "João", nome_social: "", cpf: "52998224725", nis: "", data_nascimento: "1990-05-01", nome_mae: "Ana", telefone: "", endereco: "" },
  atendimentos: [{ ...item("1", { status: "concluido", prioridade: "idoso" }) }],
  registros: [
    { id: "r1", unidade_id: "u1", atendimento_id: null, tipo: "psicologico", texto: "", sigiloso: true, autor_nome: "Psi", created_at: "2026-10-08T11:00:00Z" },
    { id: "r2", unidade_id: "u1", atendimento_id: null, tipo: "social", texto: "Orientação sobre benefícios", sigiloso: false, autor_nome: "", created_at: "2026-10-08T10:00:00Z" },
  ],
};

describe("SEMPRAS — ficha do munícipe", () => {
  beforeEach(() => resetNavigation({ id: "c1" }));

  it("mostra o histórico da área, esconde o sigilo psicológico e registra evolução social", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["sempras:recepcao", "sempras:atendimento"], scopes: [] }),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: UNIDADES },
      "POST v1/sempras/cidadaos/c1/registros": { status: 201, data: {} },
    });
    renderApp(<FichaPage />);
    expect(await screen.findByRole("heading", { name: "João" })).toBeInTheDocument();
    expect(screen.getByText("Registro sigiloso da psicologia")).toBeInTheDocument();
    expect(screen.getByText("Orientação sobre benefícios")).toBeInTheDocument();
    expect(screen.getByText("Idoso 60+")).toBeInTheDocument();
    // sem o sigilo, só o tipo social
    expect(within(screen.getByLabelText("Tipo")).queryByRole("option", { name: /psicológico/ })).not.toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Unidade"), "u2");
    await userEvent.type(screen.getByLabelText("Nova evolução *"), "  Visita domiciliar agendada ");
    await userEvent.click(screen.getByRole("button", { name: /Registrar/ }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/registros")[0]!.body).toEqual({ unidade_id: "u2", tipo: "social", texto: "Visita domiciliar agendada" }),
    );
    await waitFor(() => expect(screen.getByLabelText("Nova evolução *")).toHaveValue(""));
  });

  it("psicólogo escolhe o tipo psicológico; ficha sem atendimentos na área", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: { ...PRONTUARIO, atendimentos: [], registros: [] } },
      "GET v1/sempras/unidades": { data: UNIDADES },
      "POST v1/sempras/cidadaos/c1/registros": fail(403, "sem permissão nesta unidade"),
    });
    renderApp(<FichaPage />);
    expect(await screen.findByText("Sem atendimentos nas suas unidades.")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Tipo"), "psicologico");
    await userEvent.type(screen.getByLabelText("Nova evolução *"), "Escuta");
    await userEvent.click(screen.getByRole("button", { name: /Registrar/ }));
    expect(await screen.findByText("sem permissão nesta unidade")).toBeInTheDocument();
    expect(screen.getByLabelText("Nova evolução *")).toHaveValue("Escuta");
  });
});
