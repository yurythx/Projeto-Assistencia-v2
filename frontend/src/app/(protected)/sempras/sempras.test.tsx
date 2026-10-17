import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { EncaminhamentosRecebidos } from "@/components/sempras/EncaminhamentosRecebidos";
import { PainelChamada } from "@/components/sempras/PainelChamada";
import {
  fail,
  identityRoutes,
  mockBackend,
  mockWebSocket,
  renderApp,
  renderRealtime,
  wsTicketRoute,
} from "@/test/backend";
import type { Servico, TipoUnidade, Unidade } from "@/lib/sempras/types";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

// A ficha do munícipe já acumula muitos cartões (PAIF, Conselho, PAEFI,
// MSE, notificações, Casa Abrigo...); sob carga (suíte inteira, CI),
// sequências de interação nos testes dela passam perto do padrão de 5s.
vi.setConfig({ testTimeout: 15000 });

import SemprasPage from "./page";
import UnidadePage from "./u/[unidade]/page";
import PainelPage from "./painel/page";
import PedidosPage from "./pedidos/page";
import RmaPage from "./rma/page";
import PainelChamadaPage from "./painel-chamada/page";
import ColetivosPage from "./coletivos/page";
import UnidadesPage from "./unidades/page";
import FichaPage from "./cidadaos/[id]/page";

const unidade = (id: string, nome: string, tipo: TipoUnidade, servicos: Servico[], demandas: string[], painel_exibe_nome = true): Unidade => ({
  id, entidade_id: "e1", nome, sigla: "", ativa: true, tipo, painel_exibe_nome, servicos, demandas,
});
const CRAS = unidade("u1", "CRAS Ana Carla", "cras",
  ["fila", "prontuario", "encaminhamento", "beneficio", "paif", "oficio", "coletivo"],
  ["atendimento_tecnico", "cadunico_bolsa_familia", "beneficio_eventual", "encaminhamento_rede", "orientacao_geral", "outra"]);
const CREAS = unidade("u2", "CREAS", "creas", ["fila", "prontuario", "encaminhamento", "paefi", "mse", "notificacao"],
  ["acolhida_inicial", "atendimento_psicossocial", "outra"], false);
const POP = unidade("u3", "Centro POP", "centro_pop", ["fila", "prontuario", "encaminhamento", "beneficio", "centro_pop"],
  ["alimentacao_higiene", "banho_lavanderia", "outra"]);
const ABRIGO = unidade("u4", "Casa Abrigo", "casa_abrigo", ["prontuario", "encaminhamento", "acolhimento"], [], false);
const CONSELHO = unidade("u5", "Conselho Tutelar Central", "conselho_tutelar", ["fila", "prontuario", "encaminhamento", "conselho"],
  ["demanda_espontanea", "outra"], false);
const UNIDADES = [CRAS, CREAS];
const TODAS = [CRAS, CREAS, POP, ABRIGO, CONSELHO];

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
  demanda: "atendimento_tecnico",
  demanda_detalhe: "",
  nome: `Pessoa ${id}`,
  created_at: "2026-10-08T10:00:00Z",
  chamado_em: null,
  iniciado_em: null,
  encerrado_em: null,
  ...extra,
});

const recepcionista = { permissions: ["sempras:recepcao"], scopes: [] };

describe("SEMPRAS — atendimento", () => {
  beforeEach(() => resetNavigation({ unidade: "u1" }));

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
      "POST v1/sempras/cidadaos": { status: 201, data: { id: "c10", nome: "Nova Pessoa", nome_social: "", cpf: "529.982.247-25", nis: "", data_nascimento: "2000-01-01" } },
      "POST v1/sempras/unidades/u1/fila": { status: 201, data: item("2") },
      "POST v1/sempras/atendimentos/1/cancelar": { data: item("1", { status: "cancelado" }) },
    });
    renderRealtime(<UnidadePage />);

    expect(await screen.findByText("Pessoa 1")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "CRAS Ana Carla" })).toBeInTheDocument();
    // recepção não chama nem vê o painel
    expect(screen.queryByRole("button", { name: /Chamar próximo/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/Painel das unidades/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Chamar N001" })).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("CPF, NIS ou nome"), "529.982.247-25");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(await screen.findByText("2 atendimento(s) na área")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Ficha" })).not.toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Prioridade legal"), "gestante");
    // A demanda é obrigatória e vem da lista do tipo da unidade (ADR 031 §3).
    const demanda = screen.getAllByLabelText("Demanda")[0]!;
    expect(within(demanda).queryByRole("option", { name: /Banho/ })).not.toBeInTheDocument();
    await userEvent.selectOptions(demanda, "cadunico_bolsa_familia");
    await userEvent.click(screen.getByRole("button", { name: "Incluir Maria na fila" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/unidades/u1/fila")[0]!.body).toEqual({
        cidadao_id: "c9", tipo: "social", prioridade: "gestante", demanda: "cadunico_bolsa_familia", demanda_detalhe: "",
      }),
    );

    // nada encontrado → cadastra
    await userEvent.clear(screen.getByLabelText("CPF, NIS ou nome"));
    await userEvent.type(screen.getByLabelText("CPF, NIS ou nome"), "ninguem");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(await screen.findByText(/Ninguém encontrado/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Cadastrar/ }));
    const dialog = await screen.findByRole("dialog", { name: "Cadastrar munícipe" });
    await userEvent.type(within(dialog).getByLabelText("Nome civil *"), "Nova Pessoa");
    // no CRAS, tentar salvar sem CPF nem NIS gera erro exibido diretamente no modal
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar cadastro" }));
    expect(within(dialog).getByText(/O CRAS exige CPF ou NIS/)).toBeInTheDocument();
    // preenche o CPF obrigatório e o nome da mãe
    await userEvent.type(within(dialog).getByLabelText("CPF (ou NIS) *"), "529.982.247-25");
    await userEvent.type(within(dialog).getByLabelText("Nome da mãe"), "Ana");
    await userEvent.selectOptions(within(dialog).getByLabelText("Sexo"), "feminino");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar cadastro" }));
    expect(await screen.findByText("Sem histórico na área")).toBeInTheDocument();
    expect(api.to("POST v1/sempras/cidadaos")[0]!.body).toMatchObject({ nome: "Nova Pessoa", nome_mae: "Ana", cpf: "529.982.247-25", sexo: "feminino" });

    // cancelar quem aguarda
    await userEvent.click(screen.getByRole("button", { name: "Cancelar N001" }));
    await waitFor(() => expect(api.to("POST v1/sempras/atendimentos/1/cancelar")).toHaveLength(1));

    // frame do tópico da unidade → recarrega a fila
    fila = [item("1"), item("3", { nome: "Chegou Agora" })];
    await waitFor(() => expect(ws.socket().sent).toContainEqual({ type: "subscribe", topic: "sempras:fila:u1" }));
    act(() => ws.socket().receive({ topic: "sempras:fila:u1", type: "sempras.fila", data: { status: "aguardando" } }));
    expect(await screen.findByText("Chegou Agora")).toBeInTheDocument();
  });

  it("técnico chama o próximo e conduz o atendimento", async () => {
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
      "POST v1/sempras/unidades/u1/fila/proximo": { data: item("3", { status: "chamado" }) },
      "POST v1/sempras/atendimentos/*": (req) =>
        req.path.endsWith("/concluir") ? fail(409, "o atendimento não está no estado que a ação exige") : { data: item("x") },
    });
    renderRealtime(<UnidadePage />);

    expect(await screen.findByText("Pessoa 1")).toBeInTheDocument();
    expect(within(screen.getByRole("list", { name: "Fila" })).getByText(/Idoso 80\+/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Pessoa 1" })).toHaveAttribute("href", "/sempras/cidadaos/c-1");

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

    // Atalhos da unidade: o painel de chamada e os coletivos são DESTA unidade.
    expect(screen.getByRole("link", { name: /Painel de chamada/ })).toHaveAttribute("href", "/sempras/painel-chamada?unidade=u1");
    expect(screen.getByRole("link", { name: /Atendimentos coletivos/ })).toHaveAttribute("href", "/sempras/coletivos?unidade=u1");
    expect(screen.getByRole("link", { name: /Trocar de unidade/ })).toHaveAttribute("href", "/sempras");
  });

  it("cada tipo tem o seu atendimento: a Casa Abrigo não tem fila; o CREAS não tem coletivos", async () => {
    mockWebSocket();
    mockBackend({
      ...identityRoutes(),
      ...wsTicketRoute,
      "GET v1/sempras/unidades": { data: TODAS },
      "GET v1/sempras/unidades/u2/fila": { data: [] },
      "GET v1/sempras/unidades/u2/encaminhamentos": { data: [] },
      "GET v1/sempras/unidades/u4/encaminhamentos": { data: [] },
    });
    resetNavigation({ unidade: "u4" });
    const { unmount } = renderRealtime(<UnidadePage />);
    expect(await screen.findByRole("heading", { level: 1, name: "Casa Abrigo" })).toBeInTheDocument();
    expect(screen.getByText("Busca de munícipe")).toBeInTheDocument();
    expect(screen.queryByLabelText("Demanda")).not.toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "Fila" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Painel de chamada/ })).not.toBeInTheDocument();
    unmount();

    resetNavigation({ unidade: "u2" });
    renderRealtime(<UnidadePage />);
    expect(await screen.findByText("Ninguém na fila hoje")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Painel de chamada/ })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Atendimentos coletivos/ })).not.toBeInTheDocument();
  });

  it("Centro POP: registra uma abordagem social sem cadastro e lista as do mês", async () => {
    mockWebSocket();
    const abordagem = { id: "ab1", unidade_id: "u3", cidadao_id: null, descricao_pessoa: "homem, uns 40 anos", local: "Rodoviária",
      data_abordagem: "2026-10-10T10:00:00Z", resultado: "recusou", uso_drogas_aparente: true, demanda_saude: false,
      observacoes: "", created_at: "2026-10-10T10:00:00Z" };
    let lista: unknown[] = [];
    const api = mockBackend({
      ...identityRoutes(),
      ...wsTicketRoute,
      "GET v1/sempras/unidades": { data: TODAS },
      "GET v1/sempras/unidades/u3/fila": { data: [] },
      "GET v1/sempras/unidades/u3/encaminhamentos": { data: [] },
      "GET v1/sempras/unidades/u3/abordagens": () => ({ data: lista }),
      "POST v1/sempras/unidades/u3/abordagens": () => {
        lista = [abordagem];
        return { status: 201, data: abordagem };
      },
    });
    resetNavigation({ unidade: "u3" });
    renderRealtime(<UnidadePage />);
    const form = await screen.findByRole("form", { name: "Registrar abordagem" });
    expect(await screen.findByText("Nenhuma abordagem neste mês")).toBeInTheDocument();
    expect(within(form).getByRole("button", { name: "Registrar abordagem" })).toBeDisabled();
    await userEvent.type(within(form).getByLabelText("Pessoa abordada (descrição) *"), "homem, uns 40 anos");
    await userEvent.type(within(form).getByLabelText("Local *"), "Rodoviária");
    await userEvent.selectOptions(within(form).getByLabelText("Resultado *"), "recusou");
    await userEvent.click(within(form).getByLabelText("Uso aparente de drogas"));
    await userEvent.click(within(form).getByRole("button", { name: "Registrar abordagem" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/unidades/u3/abordagens")[0]!.body).toMatchObject({
        descricao_pessoa: "homem, uns 40 anos", local: "Rodoviária", resultado: "recusou", uso_drogas_aparente: true,
      }),
    );
    const doMes = within(await screen.findByRole("list", { name: "Abordagens do mês" }));
    expect(doMes.getByText("Recusou")).toBeInTheDocument();
    expect(doMes.getByText(/uso aparente de drogas/)).toBeInTheDocument();
  });

  it("unidade fora da lotação: não encontrada", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/sempras/unidades": { data: UNIDADES } });
    resetNavigation({ unidade: "u9" });
    renderApp(<UnidadePage />);
    expect(await screen.findByText("Unidade não encontrada")).toBeInTheDocument();
  });

  it("a escolha da unidade agrupa por tipo; com uma unidade só, entra direto nela", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/sempras/unidades": { data: TODAS } });
    const { unmount } = renderApp(<SemprasPage />);
    expect(await screen.findByRole("heading", { name: /^CRAS/ })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /^Casa Abrigo/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Centro POP/ })).toHaveAttribute("href", "/sempras/u/u3");
    expect(screen.getByRole("link", { name: /Casa Abrigo/ })).toHaveTextContent("Sem fila");
    expect(screen.getByRole("link", { name: /CREAS/ })).toHaveTextContent("Painel só com senha");
    unmount();

    mockBackend({ ...identityRoutes(), "GET v1/sempras/unidades": { data: [CRAS] } });
    renderApp(<SemprasPage />);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/sempras/u/u1"));
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
    renderRealtime(<UnidadePage />);
    expect(await screen.findByRole("button", { name: "Cancelar N005" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Iniciar N005" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /N006/ })).not.toBeInTheDocument();
  });
});

describe("SEMPRAS — painel de gestão", () => {
  beforeEach(() => resetNavigation());

  it("mostra os totais do dia e o detalhe por unidade", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/painel": {
        data: [
          { unidade: UNIDADES[0], aguardando: 1, em_atendimento: 1, concluidos: 4, nao_compareceram: 0, espera_media_min: 12.4 },
          { unidade: UNIDADES[1], aguardando: 2, em_atendimento: 0, concluidos: 1, nao_compareceram: 1, espera_media_min: 7.6 },
        ],
      },
    });
    renderApp(<PainelPage />);
    expect(await screen.findByRole("heading", { name: "SEMPRAS — Painel de gestão" })).toBeInTheDocument();
    const totais = within(screen.getByRole("list", { name: "Totais do dia" }));
    expect(totais.getByText("2")).toBeInTheDocument(); // unidades
    expect(totais.getByText("3")).toBeInTheDocument(); // na fila (1+2)
    expect(totais.getByText("5")).toBeInTheDocument(); // concluídos (4+1)
    expect(await screen.findByText("CRAS Ana Carla")).toBeInTheDocument();
    expect(screen.getByText("CREAS")).toBeInTheDocument();
  });

  it("sem a permissão de gestão: explica de quem é o painel", async () => {
    mockBackend({ ...identityRoutes(recepcionista) });
    renderApp(<PainelPage />);
    expect(await screen.findByText(/O painel de gestão é da gestão do SEMPRAS/)).toBeInTheDocument();
  });
});

describe("SEMPRAS — RMA/MDS", () => {
  beforeEach(() => resetNavigation());

  it("mostra os totais e o detalhe por unidade na competência escolhida", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/relatorios/rma": (req) => (req.query.get("competencia") === "2026-11" ? { data: [] } : {
        data: [
          { unidade: UNIDADES[0], atendimentos: 10, atendimentos_sociais: 8, atendimentos_psicologicos: 2,
            paif_ativos: 3, paif_extrema_pobreza: 1, paif_bolsa_familia: 2, paif_bpc: 0, paif_scfv_participantes: 5,
            beneficios_concedidos_no_mes: 2, paefi_casos_ativos: 1, paefi_casos_abertos_no_mes: 1, mse_ativas: 0,
            mse_concluidas_no_mes: 0, conselho_atendimentos_no_mes: 0, creas_notificacoes_recebidas_no_mes: 0,
            creas_notificacoes_respondidas_no_mes: 0, casa_abrigo_acolhidos_ativos: 0, casa_abrigo_entradas_no_mes: 0,
            casa_abrigo_desligamentos_no_mes: 0, encaminhamentos_enviados_no_mes: 1,
            oficios_cras_recebidos_no_mes: 0, oficios_cras_respondidos_no_mes: 0, coletivo_paif_familias_no_mes: 0,
            coletivo_scfv_0_6_no_mes: 0, coletivo_scfv_7_14_no_mes: 0, coletivo_scfv_15_17_no_mes: 0,
            coletivo_scfv_18_59_no_mes: 0, coletivo_scfv_idosos_no_mes: 0, coletivo_palestras_no_mes: 0,
            coletivo_pcd_no_mes: 0, coletivo_total_no_mes: 0 },
          { unidade: UNIDADES[1], atendimentos: 4, atendimentos_sociais: 4, atendimentos_psicologicos: 0,
            paif_ativos: 0, paif_extrema_pobreza: 0, paif_bolsa_familia: 0, paif_bpc: 0, paif_scfv_participantes: 0,
            beneficios_concedidos_no_mes: 0, paefi_casos_ativos: 1, paefi_casos_abertos_no_mes: 0, mse_ativas: 1,
            mse_concluidas_no_mes: 1, conselho_atendimentos_no_mes: 3, creas_notificacoes_recebidas_no_mes: 2,
            creas_notificacoes_respondidas_no_mes: 1, casa_abrigo_acolhidos_ativos: 1, casa_abrigo_entradas_no_mes: 1,
            casa_abrigo_desligamentos_no_mes: 0, encaminhamentos_enviados_no_mes: 0,
            oficios_cras_recebidos_no_mes: 0, oficios_cras_respondidos_no_mes: 0, coletivo_paif_familias_no_mes: 0,
            coletivo_scfv_0_6_no_mes: 0, coletivo_scfv_7_14_no_mes: 0, coletivo_scfv_15_17_no_mes: 0,
            coletivo_scfv_18_59_no_mes: 0, coletivo_scfv_idosos_no_mes: 0, coletivo_palestras_no_mes: 0,
            coletivo_pcd_no_mes: 0, coletivo_total_no_mes: 0 },
        ],
      }),
    });
    renderApp(<RmaPage />);
    expect(await screen.findByRole("heading", { name: "SEMPRAS — RMA/MDS" })).toBeInTheDocument();
    const totais = within(await screen.findByRole("list", { name: "Totais da competência" }));
    expect(totais.getByText("14")).toBeInTheDocument(); // atendimentos (10+4)
    expect(totais.getByText("3")).toBeInTheDocument(); // PAIF ativos
    expect(await screen.findByText("CRAS Ana Carla")).toBeInTheDocument();
    expect(screen.getByText("CREAS")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Competência"), { target: { value: "2026-11" } });
    await waitFor(() => expect(api.to("GET v1/sempras/relatorios/rma").at(-1)!.query.get("competencia")).toBe("2026-11"));
    expect(await screen.findByText("Nenhuma unidade deste tipo")).toBeInTheDocument();
  });

  it("um formulário por tipo: CREAS, Centro POP e Casa da Mulher, com as demandas da recepção", async () => {
    const zero = Object.fromEntries([
      "atendimentos", "atendimentos_sociais", "atendimentos_psicologicos", "paif_ativos", "paif_extrema_pobreza",
      "paif_bolsa_familia", "paif_bpc", "paif_scfv_participantes", "beneficios_concedidos_no_mes", "paefi_casos_ativos",
      "paefi_casos_abertos_no_mes", "mse_ativas", "mse_concluidas_no_mes", "conselho_atendimentos_no_mes",
      "creas_notificacoes_recebidas_no_mes", "creas_notificacoes_respondidas_no_mes", "casa_abrigo_acolhidos_ativos",
      "casa_abrigo_entradas_no_mes", "casa_abrigo_desligamentos_no_mes", "encaminhamentos_enviados_no_mes",
      "oficios_cras_recebidos_no_mes", "oficios_cras_respondidos_no_mes", "coletivo_paif_familias_no_mes",
      "coletivo_scfv_0_6_no_mes", "coletivo_scfv_7_14_no_mes", "coletivo_scfv_15_17_no_mes", "coletivo_scfv_18_59_no_mes",
      "coletivo_scfv_idosos_no_mes", "coletivo_palestras_no_mes", "coletivo_pcd_no_mes", "coletivo_total_no_mes",
      "paefi_casos_inicio_mes", "paefi_desligados_no_mes", "mse_la_ativas", "mse_psc_ativas", "mse_novas_la_no_mes",
      "mse_novas_psc_no_mes", "pop_pessoas_atendidas_no_mes", "pop_pessoas_novas_no_mes", "pop_pessoas_uso_drogas",
      "pop_pessoas_com_doenca", "pop_pessoas_trecheiras", "encaminhamentos_externos_no_mes", "casa_mulher_casos_ativos",
      "casa_mulher_casos_novos_no_mes", "casa_mulher_risco_alto_extremo", "casa_mulher_abrigadas", "casa_mulher_medidas_deferidas",
    ].map((k) => [k, 0]));
    const MULHER = unidade("u6", "Casa da Mulher", "casa_mulher", ["fila", "prontuario", "encaminhamento", "casa_mulher"], ["acolhida_inicial"], false);
    mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/relatorios/rma": {
        data: [
          { ...zero, unidade: CRAS, atendimentos: 9, atendimentos_por_demanda: { cadunico_bolsa_familia: 6 }, paefi_novos_por_violacao: {},
            pessoas_por_sexo_faixa: { "feminino|30-59": 4, "|": 2 } },
          { ...zero, unidade: CREAS, paefi_casos_inicio_mes: 4, paefi_casos_abertos_no_mes: 2, mse_la_ativas: 3, mse_psc_ativas: 1,
            paefi_novos_por_violacao: { violencia_sexual: 2 }, atendimentos_por_demanda: { acolhida_inicial: 5 } },
          { ...zero, unidade: POP, pop_pessoas_atendidas_no_mes: 17, pop_pessoas_novas_no_mes: 3, atendimentos_por_demanda: {},
            paefi_novos_por_violacao: {} },
          { ...zero, unidade: MULHER, casa_mulher_casos_ativos: 8, casa_mulher_abrigadas: 2, atendimentos_por_demanda: {},
            paefi_novos_por_violacao: {} },
        ],
      },
    });
    renderApp(<RmaPage />);
    // Consolidado: todas as unidades; demandas somadas.
    expect(await screen.findByText("CRAS Ana Carla")).toBeInTheDocument();
    expect(within(screen.getByRole("list", { name: "Demandas da recepção" })).getByText("Cadastro Único / Bolsa Família")).toBeInTheDocument();
    const sexoFaixa = within(screen.getByRole("table", { name: "Pessoas por sexo e faixa etária" }));
    const feminino = within(sexoFaixa.getByRole("row", { name: /^Feminino/ }));
    expect(feminino.getAllByRole("cell")[4]).toHaveTextContent("4"); // 30-59 anos
    expect(within(sexoFaixa.getByRole("row", { name: /^Não informado/ })).getAllByRole("cell")[6]).toHaveTextContent("2");

    await userEvent.selectOptions(screen.getByLabelText("Formulário"), "creas");
    const tabela = screen.getAllByRole("table")[0]!;
    expect(within(tabela).queryByText("CRAS Ana Carla")).not.toBeInTheDocument();
    expect(within(tabela).getByText("CREAS")).toBeInTheDocument();
    expect(within(tabela).getByRole("columnheader", { name: "Novos — Violência sexual" })).toBeInTheDocument();
    const totais = within(screen.getByRole("list", { name: "Totais da competência" }));
    expect(totais.getByText("Adolescentes em MSE").previousSibling).toHaveTextContent("4");

    await userEvent.selectOptions(screen.getByLabelText("Formulário"), "centro_pop");
    expect(within(screen.getByRole("list", { name: "Totais da competência" })).getByText("Pessoas atendidas").previousSibling).toHaveTextContent("17");

    await userEvent.selectOptions(screen.getByLabelText("Formulário"), "casa_mulher");
    expect(within(screen.getAllByRole("table")[0]!).getByText("Casa da Mulher")).toBeInTheDocument();
    expect(within(screen.getByRole("list", { name: "Totais da competência" })).getByText("Abrigadas").previousSibling).toHaveTextContent("2");
  });

  it("sem a permissão de gestão: explica de quem é o RMA", async () => {
    mockBackend({ ...identityRoutes(recepcionista) });
    renderApp(<RmaPage />);
    expect(await screen.findByText(/O RMA é da gestão do SEMPRAS/)).toBeInTheDocument();
  });
});

const PRONTUARIO = {
  cidadao: { id: "c1", nome: "João", nome_social: "", cpf: "52998224725", nis: "", data_nascimento: "1990-05-01", nome_mae: "Ana", telefone: "", endereco: "", anonimo: false },
  atendimentos: [{ ...item("1", { status: "concluido", prioridade: "idoso" }) }],
  registros: [
    { id: "r1", unidade_id: "u1", atendimento_id: null, tipo: "psicologico", texto: "", sigiloso: true, autor_nome: "Psi", created_at: "2026-10-08T11:00:00Z" },
    { id: "r2", unidade_id: "u1", atendimento_id: null, tipo: "social", texto: "Orientação sobre benefícios", sigiloso: false, autor_nome: "", created_at: "2026-10-08T10:00:00Z" },
  ],
  rede_apoio: [{ id: "ra1", cidadao_id: "c1", nome: "Maria", parentesco: "Irmã", telefone: "66999990000", created_at: "2026-10-08T09:00:00Z" }],
  beneficios: [
    { id: "b1", beneficiario_id: "c1", solicitante_id: null, unidade_id: "u1", tipo: "cesta_basica", forma_acesso: "demanda_espontanea",
      quantidade: 1, status: "solicitado",
      observacao: "", motivo_decisao: "", tecnico_decisao_id: null, data_solicitacao: "2026-10-08T08:00:00Z", data_decisao: null },
  ],
  encaminhamentos: [
    { id: "enc1", cidadao_id: "c1", unidade_origem_id: "u1", unidade_destino_id: "u2", motivo: "precisa de acompanhamento especializado",
      status: "pendente", observacao_decisao: "", encaminhado_por: null, decidido_por: null, created_at: "2026-10-08T07:00:00Z", decidido_em: null },
  ],
  paif_vinculos: [
    { id: "paif1", cidadao_id: "c1", unidade_id: "u1", status: "ativo", tipo_atendimento: "particularizado",
      extrema_pobreza: true, bolsa_familia: false, bpc: false,
      scfv_participantes: 2, motivo_exclusao: "", incluido_por: null, excluido_por: null, data_inclusao: "2026-10-08T06:00:00Z", data_exclusao: null },
  ],
  conselho_atendimentos: [
    { id: "ct1", protocolo: "CT-ABCD1234", crianca_id: "c1", unidade_id: "u1", responsavel_nome: "", responsavel_cpf: "",
      responsavel_telefone: "", origem_demanda: "escola", motivo_violacao: "evasao_escolar", medida_protetiva: "",
      status: "em_andamento", observacoes: "", conselheiro_id: null, created_at: "2026-10-08T05:00:00Z", atualizado_em: null },
  ],
  paefi_casos: [
    { id: "paefi1", caso_codigo: "PAEFI-ABCD1234", cidadao_id: "c1", unidade_id: "u1", violacoes: ["violencia_fisica"],
      membros_vulneraveis: "", pia_elaborado: false, status: "em_acompanhamento", motivo_encerramento: "",
      tecnico_referencia_id: null, observacoes: "", data_abertura: "2026-10-08T04:00:00Z", data_encerramento: null },
  ],
  mse_medidas: [
    { id: "mse1", adolescente_id: "c1", unidade_id: "u1", numero_processo: "123/2026", vara_origem: "Vara da Infância",
      ato_infracional: "furto (art. 155 CP)", tipo_medida: "liberdade_assistida", instituicao_parceira: "",
      status: "em_cumprimento", tecnico_referencia_id: null, observacoes: "", data_inicio: "2026-10-08T03:00:00Z",
      prazo_previsto: null, data_conclusao: null },
  ],
  notificacoes_creas: [
    { id: "notif1", protocolo: "OF-ABCD1234", vitima_id: "c1", unidade_id: "u1", origem_entidade: "ministerio_publico",
      assunto: "requisição de informações sobre acompanhamento", status: "recebida", resposta: "", tecnico_referencia_id: null,
      data_entrada: "2026-10-08T02:00:00Z", prazo_resposta: null, data_resposta: null },
  ],
  oficios_cras: [
    { id: "oficio1", cidadao_id: "c1", unidade_id: "u1", orgao_emissor: "Conselho Tutelar", numero_oficio: "123/2026",
      data_oficio: null, data_recebimento: "2026-10-08T02:30:00Z", assunto: "acompanhamento da família",
      status: "pendente", numero_resposta: "", data_resposta: null, tecnico_responsavel_id: null },
  ],
  casa_abrigo_acolhimentos: [
    { id: "acolh1", cidadao_id: "c1", unidade_id: "u1", sexo: "masculino", responsavel_nome: "", responsavel_nis: "",
      responsavel_telefone: "", responsavel_vinculo: "", responsavel_endereco: "", motivos: ["negligencia_abandono"],
      processo_judicial: "", guia_acolhimento: "", conselheiro_nome: "", conselheiro_telefone: "", situacao_juridica: "",
      historico_acolhido_anteriormente: false, historico_irmaos_outra_unidade: false, observacoes_gerais: "",
      status: "acolhido", motivo_desligamento: "", desligado_por: null, data_entrada: "2026-10-08T01:00:00Z",
      data_desligamento: null,
      irmaos: [{ id: "irmao1", acolhimento_id: "acolh1", nome: "Irmão da Criança", data_nascimento: null,
        esta_acolhido: false, unidade_descricao: "", observacao: "", created_at: "2026-10-08T01:10:00Z" }],
      diagnosticos: [{ id: "diag1", acolhimento_id: "acolh1", categoria: "doenca", diagnostico: "asma",
        descricao: "", created_at: "2026-10-08T01:20:00Z" }],
      visitantes: [{ id: "visita1", acolhimento_id: "acolh1", nome_visitante: "Tia Maria", documento_identidade: "",
        parentesco_vinculo: "Tia", data_visita: "2026-10-05T00:00:00Z", hora_entrada: "14:00", hora_saida: "16:00",
        observacoes: "", created_at: "2026-10-05T14:00:00Z" }],
      termos: [{ id: "termo1", acolhimento_id: "acolh1", tipo_termo: "passeio", responsavel_retirada: "Tia Maria",
        parentesco_responsavel: "Tia", motivo_observacao: "passeio no parque", data_termo: "2026-10-06T00:00:00Z",
        created_at: "2026-10-06T00:00:00Z" }],
      pia: { acolhimento_id: "acolh1", matricula_escolar: "", serie_escolar: "", escola: "Escola Municipal",
        medicamentos: "", alergias: "", vacinacao_atualizada: true, cartao_sus: "", contato_familiar: "",
        visitas_autorizadas: "", historico_familiar: "", atividades_manha: "", atividades_tarde: "", atividades_noite: "",
        atividades_fim_semana: "", atualizado_por: null, atualizado_em: "2026-10-07T00:00:00Z" },
      pia_metas: [{ id: "meta1", acolhimento_id: "acolh1", tipo: "intervencao", descricao: "reduzir episódios de agressividade",
        providencia: "atendimento semanal", responsavel: "Psicóloga", prazo: null, concluida: false, created_at: "2026-10-07T00:00:00Z" }],
      documentos: [{ id: "doc1", acolhimento_id: "acolh1", tipo: "acolhido", nome_original: "certidao.pdf",
        object_key: "sempras/casa-abrigo/acolh1/2026/10/x.pdf", content_type: "application/pdf", size_bytes: 1024,
        status: "ready", created_at: "2026-10-07T00:00:00Z" }] },
  ],
  centro_pop_perfil: null,
  encaminhamentos_externos: [
    { id: "encext1", cidadao_id: "c1", unidade_id: "u1", destino: "caps_ad", prioridade: "alta",
      motivo: "uso abusivo de substâncias", status: "pendente", observacoes: "", tecnico_id: null,
      created_at: "2026-10-08T00:30:00Z", atualizado_em: "2026-10-08T00:30:00Z" },
  ],
  centro_pop_documentos: [],
};

describe("SEMPRAS — ficha do munícipe", () => {
  beforeEach(() => resetNavigation({ id: "c1" }));

  it("mostra o histórico da área, esconde o sigilo psicológico e registra evolução social", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["sempras:recepcao", "sempras:atendimento"], scopes: [] }),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
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
    await userEvent.click(screen.getByRole("button", { name: "Registrar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/registros")[0]!.body).toEqual({ unidade_id: "u2", tipo: "social", texto: "Visita domiciliar agendada" }),
    );
    await waitFor(() => expect(screen.getByLabelText("Nova evolução *")).toHaveValue(""));
  });

  it("a ficha mostra só as seções do tipo da unidade e o resumo das outras unidades", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u2");
    mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: { ...PRONTUARIO,
        outras_unidades: [{ unidade_id: "u9", unidade_nome: "CRAS Iguaçu", tipo: "cras", ativos: ["paif"] }],
        registros_origem: [{ id: "ro1", unidade_id: "u9", atendimento_id: null, tipo: "social", texto: "Família em extrema pobreza",
          sigiloso: false, autor_nome: "Técnica do CRAS", created_at: "2026-10-07T10:00:00Z" }] } },
      "GET v1/sempras/unidades": { data: TODAS },
    });
    renderApp(<FichaPage />);
    expect(await screen.findByRole("heading", { name: "PAEFI (CREAS)" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "PAIF (CRAS)" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Casa Abrigo" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Benefícios eventuais" })).not.toBeInTheDocument();
    expect(screen.getByText(/CRAS Iguaçu — PAIF ativo/)).toBeInTheDocument();
    expect(screen.getByText("Família em extrema pobreza")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Voltar ao atendimento/ })).toHaveAttribute("href", "/sempras/u/u2");

    await userEvent.selectOptions(screen.getByLabelText("Atendendo pela unidade"), "u1");
    expect(await screen.findByRole("heading", { name: "PAIF (CRAS)" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "PAEFI (CREAS)" })).not.toBeInTheDocument();
  });

  it("Casa da Mulher: abre o caso, reavalia (abrigamento) e encerra", async () => {
    const MULHER = unidade("u6", "Casa da Mulher", "casa_mulher", ["fila", "prontuario", "encaminhamento", "casa_mulher"], ["acolhida_inicial"], false);
    resetNavigation({ id: "c1" }, "/", "unidade=u6");
    const caso = {
      id: "cm1", caso_codigo: "CM-ABCD1234", cidadao_id: "c1", unidade_id: "u6", violencias: ["fisica", "psicologica"],
      relacao_agressor: "ex_conjuge_companheiro", nivel_risco: "alto", medida_protetiva_status: "deferida",
      medida_protetiva_processo: "0001234-56.2026", patrulha_maria_da_penha: true, filhos: 2, abrigada: false,
      data_abrigamento: null, status: "ativo", motivo_encerramento: "", tecnico_referencia_id: null, observacoes: "",
      data_abertura: "2026-10-08T05:00:00Z", atualizado_em: null, data_encerramento: null,
    };
    let casos: unknown[] = [];
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": () => ({ data: { ...PRONTUARIO, casa_mulher_casos: casos } }),
      "GET v1/sempras/unidades": { data: [...TODAS, MULHER] },
      "POST v1/sempras/cidadaos/c1/casa-mulher": () => {
        casos = [caso];
        return { status: 201, data: caso };
      },
      "PUT v1/sempras/casa-mulher/cm1": { data: { ...caso, abrigada: true } },
      "POST v1/sempras/casa-mulher/cm1/encerrar": { data: { ...caso, status: "encerrado" } },
    });
    renderApp(<FichaPage />);
    expect(await screen.findByRole("heading", { name: "Casa da Mulher" })).toBeInTheDocument();
    // Seções de outros tipos não aparecem na Casa da Mulher.
    expect(screen.queryByRole("heading", { name: "PAIF (CRAS)" })).not.toBeInTheDocument();
    expect(screen.getByText("Nenhum caso da Casa da Mulher nas suas unidades.")).toBeInTheDocument();

    const abrir = screen.getByRole("form", { name: "Abrir caso" });
    expect(within(abrir).getByRole("button", { name: "Abrir caso" })).toBeDisabled();
    await userEvent.click(within(abrir).getByLabelText("Física"));
    await userEvent.click(within(abrir).getByLabelText("Psicológica"));
    await userEvent.selectOptions(within(abrir).getByLabelText("Relação com o agressor"), "ex_conjuge_companheiro");
    await userEvent.selectOptions(within(abrir).getByLabelText("Avaliação de risco"), "alto");
    await userEvent.click(within(abrir).getByLabelText(/Patrulha Maria da Penha/));
    await userEvent.click(within(abrir).getByRole("button", { name: "Abrir caso" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/casa-mulher")[0]!.body).toMatchObject({
        unidade_id: "u6", violencias: ["fisica", "psicologica"], relacao_agressor: "ex_conjuge_companheiro",
        nivel_risco: "alto", patrulha_maria_da_penha: true,
      }),
    );
    const lista = within(await screen.findByRole("list", { name: "Casos da Casa da Mulher" }));
    expect(lista.getByText("Risco alto")).toBeInTheDocument();
    expect(lista.getByText(/0001234-56\.2026/)).toBeInTheDocument();
    // Com um caso ativo, o formulário de abertura some (um ativo por mulher).
    expect(screen.queryByRole("form", { name: "Abrir caso" })).not.toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Reavaliar" }));
    const reavaliar = screen.getByRole("form", { name: "Reavaliar o caso" });
    await userEvent.click(within(reavaliar).getByLabelText(/Abrigada/));
    await userEvent.click(within(reavaliar).getByRole("button", { name: "Salvar reavaliação" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-mulher/cm1")[0]!.body).toMatchObject({ abrigada: true, nivel_risco: "alto" }));

    await userEvent.click(lista.getByRole("button", { name: "Encerrar" }));
    await userEvent.selectOptions(screen.getByLabelText("Motivo do encerramento", { selector: "#cm-motivo-cm1" }), "mudanca_municipio");
    await userEvent.click(screen.getByRole("button", { name: "Confirmar encerramento" }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-mulher/cm1/encerrar")[0]!.body).toEqual({ motivo: "mudanca_municipio" }));
  });

  it("psicólogo escolhe o tipo psicológico; ficha sem atendimentos na área", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: { ...PRONTUARIO, atendimentos: [], registros: [] } },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/registros": fail(403, "sem permissão nesta unidade"),
    });
    renderApp(<FichaPage />);
    expect(await screen.findByText("Sem atendimentos nas suas unidades.")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Tipo"), "psicologico");
    await userEvent.type(screen.getByLabelText("Nova evolução *"), "Escuta");
    await userEvent.click(screen.getByRole("button", { name: "Registrar" }));
    expect(await screen.findByText("sem permissão nesta unidade")).toBeInTheDocument();
    expect(screen.getByLabelText("Nova evolução *")).toHaveValue("Escuta");
  });

  it("direitos do titular: edita o cadastro a pedido, exporta os dados e solicita anonimização", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "PUT v1/sempras/cidadaos/c1": { data: { ...PRONTUARIO.cidadao, telefone: "66999990000" } },
      "POST v1/sempras/cidadaos/c1/pedidos": { status: 201, data: { id: "p1", tipo: "correcao", status: "concluido" } },
    });
    renderApp(<FichaPage />);
    expect(await screen.findByRole("heading", { name: "João" })).toBeInTheDocument();

    // editar cadastro, marcando que é a pedido do titular
    await userEvent.click(screen.getByRole("button", { name: "Editar" }));
    const editDialog = await screen.findByRole("dialog", { name: "Editar cadastro" });
    const telefone = within(editDialog).getByLabelText("Telefone");
    await userEvent.clear(telefone);
    await userEvent.type(telefone, "66999990000");
    await userEvent.click(within(editDialog).getByLabelText(/A pedido do titular/));
    await userEvent.type(within(editDialog).getByLabelText("Justificativa *"), "titular pediu para corrigir o telefone");
    await userEvent.click(within(editDialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/cidadaos/c1")[0]!.body).toMatchObject({ telefone: "66999990000" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/pedidos")[0]!.body).toEqual({
        tipo: "correcao",
        justificativa: "titular pediu para corrigir o telefone",
      }),
    );
    expect(screen.queryByRole("dialog", { name: "Editar cadastro" })).not.toBeInTheDocument();

    // exportar dados: o link de download só habilita com justificativa
    await userEvent.click(screen.getByRole("button", { name: "Exportar dados" }));
    const exportDialog = await screen.findByRole("dialog", { name: "Exportar dados do munícipe" });
    const baixar = within(exportDialog).getByRole("link", { name: /Baixar dados/ });
    expect(baixar).toHaveAttribute("aria-disabled", "true");
    await userEvent.type(within(exportDialog).getByLabelText("Justificativa *"), "titular quer uma cópia dos dados");
    expect(baixar).toHaveAttribute(
      "href",
      "/api/backend/v1/sempras/cidadaos/c1/pedidos/acesso?justificativa=titular%20quer%20uma%20c%C3%B3pia%20dos%20dados",
    );
    expect(baixar).toHaveAttribute("aria-disabled", "false");

    // solicitar anonimização
    await userEvent.click(screen.getByRole("button", { name: "Solicitar anonimização" }));
    const anonDialog = await screen.findByRole("dialog", { name: "Solicitar anonimização" });
    await userEvent.type(within(anonDialog).getByLabelText("Justificativa *"), "titular pediu para ser esquecido");
    await userEvent.click(within(anonDialog).getByRole("button", { name: "Registrar pedido" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/pedidos").at(-1)!.body).toEqual({
        tipo: "anonimizacao",
        justificativa: "titular pediu para ser esquecido",
      }),
    );
    expect(await screen.findByText("Pedido registrado — aguardando decisão da gestão")).toBeInTheDocument();
  });

  it("rede de apoio: lista, adiciona e remove um contato", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/rede-apoio": { status: 201, data: { id: "ra2", cidadao_id: "c1", nome: "Pedro", parentesco: "Vizinho", telefone: "", created_at: "2026-10-08T12:00:00Z" } },
      "DELETE v1/sempras/rede-apoio/ra1": { data: { removido: true } },
    });
    renderApp(<FichaPage />);
    expect(await screen.findByText("Maria")).toBeInTheDocument();
    expect(screen.getByText("Irmã · 66999990000")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Nome *"), "Pedro");
    await userEvent.type(screen.getByLabelText("Parentesco"), "Vizinho");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/rede-apoio")[0]!.body).toEqual({ nome: "Pedro", parentesco: "Vizinho", telefone: "" }),
    );
    expect(await screen.findByText("Contato adicionado")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Remover Maria da rede de apoio" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/rede-apoio/ra1")).toHaveLength(1));
    expect(await screen.findByText("Contato removido")).toBeInTheDocument();
  });

  it("benefícios eventuais: solicita, concede e nega", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/beneficios": { status: 201, data: { ...PRONTUARIO.beneficios[0], id: "b2", tipo: "funeral" } },
      "POST v1/sempras/beneficios/b1/decidir": (req) => ({
        data: { ...PRONTUARIO.beneficios[0], status: (req.body as { acao: string }).acao === "conceder" ? "concedido" : "negado" },
      }),
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Benefícios eventuais" }));
    expect(lista.getByText("Cesta básica")).toBeInTheDocument();
    expect(lista.getByText("Solicitado")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Tipo de benefício"), "funeral");
    await userEvent.click(screen.getByRole("button", { name: "Solicitar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/beneficios")[0]!.body).toEqual({
        unidade_id: "u1", tipo: "funeral", forma_acesso: "demanda_espontanea", quantidade: 1, observacao: "",
      }),
    );
    expect(await screen.findByText("Benefício solicitado")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Negar" }));
    const dialog = await screen.findByRole("dialog", { name: "Negar benefício" });
    expect(within(dialog).getByRole("button", { name: "Confirmar" })).toBeDisabled();
    await userEvent.type(within(dialog).getByLabelText("Motivo *"), "família já recebeu este mês");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/beneficios/b1/decidir")[0]!.body).toEqual({ acao: "negar", motivo: "família já recebeu este mês" }),
    );
    expect(await screen.findByText("Benefício negado")).toBeInTheDocument();
  });

  it("encaminhamentos: lista e encaminha para outra unidade", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/encaminhamentos": { status: 201, data: { ...PRONTUARIO.encaminhamentos[0], id: "enc2" } },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Encaminhamentos" }));
    expect(lista.getByText(/precisa de acompanhamento especializado/)).toBeInTheDocument();
    expect(lista.getByText("CRAS Ana Carla → CREAS")).toBeInTheDocument();
    expect(lista.getByText("Pendente")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Encaminhar para"), "u2");
    await userEvent.type(screen.getByLabelText("Motivo do encaminhamento *"), "nova demanda de média complexidade");
    await userEvent.click(screen.getByRole("button", { name: "Encaminhar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/encaminhamentos")[0]!.body).toEqual({
        unidade_origem_id: "u1", unidade_destino_id: "u2", motivo: "nova demanda de média complexidade",
      }),
    );
    expect(await screen.findByText("Encaminhamento registrado")).toBeInTheDocument();
  });

  it("PAIF: lista, inclui e exclui um vínculo", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/paif": { status: 201, data: { ...PRONTUARIO.paif_vinculos[0], id: "paif2" } },
      "POST v1/sempras/paif/paif1/excluir": { data: { ...PRONTUARIO.paif_vinculos[0], status: "excluido" } },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Vínculos PAIF" }));
    expect(lista.getByText("Extrema pobreza")).toBeInTheDocument();
    expect(lista.getByText("Ativo")).toBeInTheDocument();

    await userEvent.click(screen.getByLabelText("Bolsa Família"));
    await userEvent.clear(screen.getByLabelText("Participantes no SCFV"));
    await userEvent.type(screen.getByLabelText("Participantes no SCFV"), "3");
    await userEvent.click(screen.getByRole("button", { name: "Incluir no PAIF" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/paif")[0]!.body).toEqual({
        unidade_id: "u1", tipo_atendimento: "particularizado", extrema_pobreza: false, bolsa_familia: true, bpc: false, scfv_participantes: 3,
      }),
    );
    expect(await screen.findByText("Família incluída no PAIF")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Excluir" }));
    const dialog = await screen.findByRole("dialog", { name: "Excluir vínculo PAIF" });
    await userEvent.type(within(dialog).getByLabelText("Motivo da exclusão *"), "família se mudou do território");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/paif/paif1/excluir")[0]!.body).toEqual({ motivo: "família se mudou do território" }),
    );
    expect(await screen.findByText("Vínculo PAIF excluído")).toBeInTheDocument();
  });

  it("Conselho Tutelar: lista, registra um atendimento e aplica medida protetiva", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u5");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/conselho-tutelar": {
        status: 201,
        data: { ...PRONTUARIO.conselho_atendimentos[0], id: "ct2", protocolo: "CT-NOVO0001" },
      },
      "POST v1/sempras/conselho-tutelar/ct1/transitar": {
        data: { ...PRONTUARIO.conselho_atendimentos[0], status: "medida_aplicada", medida_protetiva: "matricula_escolar_obrigatoria" },
      },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Atendimentos do Conselho Tutelar" }));
    expect(lista.getByText("CT-ABCD1234 · Evasão escolar")).toBeInTheDocument();
    expect(lista.getByText("Em andamento")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Origem da demanda"), "disque_100");
    await userEvent.click(screen.getByRole("button", { name: "Abrir atendimento" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/conselho-tutelar")[0]!.body).toEqual({
        unidade_id: "u5", responsavel_nome: "", responsavel_cpf: "", responsavel_telefone: "",
        origem_demanda: "disque_100", motivo_violacao: "negligencia_abandono", observacoes: "",
      }),
    );
    expect(await screen.findByText("Atendimento do Conselho Tutelar registrado")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Aplicar medida" }));
    const dialog = await screen.findByRole("dialog", { name: "Aplicar medida protetiva" });
    await userEvent.selectOptions(within(dialog).getByLabelText("Medida protetiva *"), "matricula_escolar_obrigatoria");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/conselho-tutelar/ct1/transitar")[0]!.body).toEqual({
        acao: "aplicar_medida", medida: "matricula_escolar_obrigatoria",
      }),
    );
    expect(await screen.findByText("Medida protetiva aplicada")).toBeInTheDocument();
  });

  it("PAEFI: lista, abre um caso, marca o PIA elaborado e encerra", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u2");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/paefi": {
        status: 201,
        data: { ...PRONTUARIO.paefi_casos[0], id: "paefi2", caso_codigo: "PAEFI-NOVO0001" },
      },
      "POST v1/sempras/paefi/paefi1/pia": { data: { ...PRONTUARIO.paefi_casos[0], pia_elaborado: true } },
      "POST v1/sempras/paefi/paefi1/encerrar": {
        data: { ...PRONTUARIO.paefi_casos[0], status: "desligado", motivo_encerramento: "objetivos_alcancados" },
      },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Casos do PAEFI" }));
    expect(lista.getByText("PAEFI-ABCD1234 · Violência física")).toBeInTheDocument();
    expect(lista.getByText("Em acompanhamento")).toBeInTheDocument();

    await userEvent.click(screen.getByLabelText("Negligência"));
    await userEvent.click(screen.getByRole("button", { name: "Abrir caso" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/paefi")[0]!.body).toEqual({
        unidade_id: "u2", violacoes: ["negligencia"], membros_vulneraveis: "", observacoes: "",
      }),
    );
    expect(await screen.findByText("Caso do PAEFI aberto")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Marcar PIA elaborado" }));
    await waitFor(() => expect(api.to("POST v1/sempras/paefi/paefi1/pia")).toHaveLength(1));
    expect(await screen.findByText("PIA elaborado registrado")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Encerrar" }));
    const dialog = await screen.findByRole("dialog", { name: "Encerrar caso do PAEFI" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/paefi/paefi1/encerrar")[0]!.body).toEqual({ motivo: "objetivos_alcancados" }),
    );
    expect(await screen.findByText("Caso do PAEFI encerrado")).toBeInTheDocument();
  });

  it("MSE: lista, abre uma medida e conclui", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u2");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/mse": { status: 201, data: { ...PRONTUARIO.mse_medidas[0], id: "mse2" } },
      "POST v1/sempras/mse/mse1/concluir": { data: { ...PRONTUARIO.mse_medidas[0], status: "cumprida" } },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Medidas socioeducativas" }));
    expect(lista.getByText("Liberdade Assistida (LA)")).toBeInTheDocument();
    expect(lista.getByText("Em cumprimento")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Ato infracional *"), "ameaça (art. 147 CP)");
    await userEvent.click(screen.getByRole("button", { name: "Abrir medida" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/mse")[0]!.body).toEqual({
        unidade_id: "u2", numero_processo: "", vara_origem: "", ato_infracional: "ameaça (art. 147 CP)",
        tipo_medida: "liberdade_assistida", instituicao_parceira: "",
      }),
    );
    expect(await screen.findByText("Medida socioeducativa registrada")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Concluir" }));
    const dialog = await screen.findByRole("dialog", { name: "Concluir medida socioeducativa" });
    await userEvent.selectOptions(within(dialog).getByLabelText("Resultado"), "cumprida");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/mse/mse1/concluir")[0]!.body).toEqual({ resultado: "cumprida" }),
    );
    expect(await screen.findByText("Medida socioeducativa concluída")).toBeInTheDocument();
  });

  it("Notificações do CREAS: lista, protocola, inicia diligência, responde e arquiva", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u2");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/creas-notificacoes": { status: 201, data: { ...PRONTUARIO.notificacoes_creas[0], id: "notif2" } },
      "POST v1/sempras/creas-notificacoes/notif1/transitar": (req) => {
        const { acao, resposta } = req.body as { acao: string; resposta?: string };
        if (acao === "iniciar_diligencia") return { data: { ...PRONTUARIO.notificacoes_creas[0], status: "em_diligencia" } };
        if (acao === "responder") return { data: { ...PRONTUARIO.notificacoes_creas[0], status: "respondida", resposta } };
        return { data: { ...PRONTUARIO.notificacoes_creas[0], status: "arquivada" } };
      },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Notificações do CREAS" }));
    expect(lista.getByText("OF-ABCD1234 · Ministério Público")).toBeInTheDocument();
    expect(lista.getByText("Recebida")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Origem da notificação"), "disque_100_180");
    await userEvent.type(screen.getByLabelText("Assunto *"), "nova denúncia recebida");
    await userEvent.click(screen.getByRole("button", { name: "Protocolar notificação" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/creas-notificacoes")[0]!.body).toEqual({
        unidade_id: "u2", origem_entidade: "disque_100_180", assunto: "nova denúncia recebida", prazo_resposta: null,
      }),
    );
    expect(await screen.findByText("Notificação do CREAS registrada")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Iniciar diligência" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/creas-notificacoes/notif1/transitar")[0]!.body).toEqual({ acao: "iniciar_diligencia" }),
    );
    expect(await screen.findByText("Diligência iniciada")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Responder" }));
    const dialog = await screen.findByRole("dialog", { name: "Responder notificação" });
    await userEvent.type(within(dialog).getByLabelText("Resposta *"), "família está em acompanhamento pelo PAEFI");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/creas-notificacoes/notif1/transitar").at(-1)!.body).toEqual({
        acao: "responder", resposta: "família está em acompanhamento pelo PAEFI",
      }),
    );
    expect(await screen.findByText("Notificação respondida")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Arquivar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/creas-notificacoes/notif1/transitar").at(-1)!.body).toEqual({ acao: "arquivar" }),
    );
    expect(await screen.findByText("Notificação arquivada")).toBeInTheDocument();
  });

  it("Ofícios do CRAS: lista, registra e responde", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/oficios-cras": { status: 201, data: { ...PRONTUARIO.oficios_cras[0], id: "oficio2" } },
      "POST v1/sempras/oficios-cras/oficio1/responder": { data: { ...PRONTUARIO.oficios_cras[0], status: "respondido", numero_resposta: "456/2026" } },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Ofícios do CRAS" }));
    expect(lista.getByText("123/2026 · Conselho Tutelar")).toBeInTheDocument();
    expect(lista.getByText("Pendente")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Órgão emissor *"), "Ministério Público");
    await userEvent.type(screen.getByLabelText("Nº do ofício *"), "789/2026");
    await userEvent.type(screen.getByLabelText("Assunto"), "requisição sobre a família");
    await userEvent.click(screen.getByRole("button", { name: "Registrar ofício" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/oficios-cras")[0]!.body).toEqual({
        unidade_id: "u1", orgao_emissor: "Ministério Público", numero_oficio: "789/2026", assunto: "requisição sobre a família",
      }),
    );
    expect(await screen.findByText("Ofício registrado")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Responder" }));
    const dialog = await screen.findByRole("dialog", { name: "Responder ofício" });
    await userEvent.type(within(dialog).getByLabelText("Nº da resposta *"), "456/2026");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/oficios-cras/oficio1/responder")[0]!.body).toEqual({ numero_resposta: "456/2026" }),
    );
    expect(await screen.findByText("Ofício respondido")).toBeInTheDocument();
  });

  it("Casa Abrigo: lista, abre acolhimento, adiciona irmão/diagnóstico e desliga", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u4");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/cidadaos/c1/casa-abrigo": {
        status: 201,
        data: { ...PRONTUARIO.casa_abrigo_acolhimentos[0], id: "acolh2" },
      },
      "POST v1/sempras/casa-abrigo/acolh1/irmaos": {
        status: 201,
        data: { id: "irmao1", acolhimento_id: "acolh1", nome: "Irmão da Criança", data_nascimento: null, esta_acolhido: false, unidade_descricao: "", observacao: "", created_at: "2026-10-08T01:30:00Z" },
      },
      "DELETE v1/sempras/casa-abrigo-irmaos/irmao1": { data: { removido: true } },
      "POST v1/sempras/casa-abrigo/acolh1/diagnosticos": {
        status: 201,
        data: { id: "diag1", acolhimento_id: "acolh1", categoria: "doenca", diagnostico: "asma", descricao: "", created_at: "2026-10-08T01:40:00Z" },
      },
      "DELETE v1/sempras/casa-abrigo-diagnosticos/diag1": { data: { removido: true } },
      "POST v1/sempras/casa-abrigo/acolh1/desligar": {
        data: { ...PRONTUARIO.casa_abrigo_acolhimentos[0], status: "desligado", motivo_desligamento: "família reestruturada" },
      },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Acolhimentos" }));
    expect(lista.getByText("Negligência/abandono")).toBeInTheDocument();
    expect(lista.getByText("Acolhido")).toBeInTheDocument();

    await userEvent.click(screen.getByLabelText("Situação de rua"));
    await userEvent.click(screen.getByRole("button", { name: "Abrir acolhimento" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/casa-abrigo")[0]!.body).toEqual({
        unidade_id: "u4", sexo: "masculino", motivos: ["situacao_rua"], responsavel_nome: "",
        responsavel_vinculo: "", processo_judicial: "", conselheiro_nome: "",
      }),
    );
    expect(await screen.findByText("Acolhimento registrado")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Nome do irmão"), "Irmão da Criança");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar irmão" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/casa-abrigo/acolh1/irmaos")[0]!.body).toEqual({ nome: "Irmão da Criança", esta_acolhido: false }),
    );
    expect(await screen.findByText("Irmão adicionado")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Remover Irmão da Criança" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-irmaos/irmao1")).toHaveLength(1));
    expect(await screen.findByText("Irmão removido")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Diagnóstico"), "asma");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar diagnóstico" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/casa-abrigo/acolh1/diagnosticos")[0]!.body).toEqual({ categoria: "doenca", diagnostico: "asma" }),
    );
    expect(await screen.findByText("Diagnóstico adicionado")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Remover diagnóstico asma" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-diagnosticos/diag1")).toHaveLength(1));
    expect(await screen.findByText("Diagnóstico removido")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Desligar" }));
    const dialog = await screen.findByRole("dialog", { name: "Desligar acolhimento" });
    await userEvent.type(within(dialog).getByLabelText("Motivo do desligamento *"), "família reestruturada");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/casa-abrigo/acolh1/desligar")[0]!.body).toEqual({ motivo: "família reestruturada" }),
    );
    expect(await screen.findByText("Acolhimento desligado")).toBeInTheDocument();
  });

  it("Casa Abrigo: visitantes, termos e PIA — anota, lança, salva e conclui uma meta", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u4");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/casa-abrigo/acolh1/visitantes": {
        status: 201,
        data: { ...PRONTUARIO.casa_abrigo_acolhimentos[0]!.visitantes[0], id: "visita2" },
      },
      "DELETE v1/sempras/casa-abrigo-visitantes/visita1": { data: { removido: true } },
      "POST v1/sempras/casa-abrigo/acolh1/termos": {
        status: 201,
        data: { ...PRONTUARIO.casa_abrigo_acolhimentos[0]!.termos[0], id: "termo2" },
      },
      "DELETE v1/sempras/casa-abrigo-termos/termo1": { data: { removido: true } },
      "PUT v1/sempras/casa-abrigo/acolh1/pia": { data: { ...PRONTUARIO.casa_abrigo_acolhimentos[0]!.pia, escola: "Escola Estadual" } },
      "POST v1/sempras/casa-abrigo/acolh1/pia-metas": {
        status: 201,
        data: { ...PRONTUARIO.casa_abrigo_acolhimentos[0]!.pia_metas[0], id: "meta2" },
      },
      "POST v1/sempras/casa-abrigo-pia-metas/meta1/concluir": {
        data: { ...PRONTUARIO.casa_abrigo_acolhimentos[0]!.pia_metas[0], concluida: true },
      },
      "DELETE v1/sempras/casa-abrigo-pia-metas/meta1": { data: { removido: true } },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Acolhimentos" }));
    expect(lista.getByText(/Tia Maria/)).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Nome do visitante"), "Tia Maria");
    await userEvent.click(screen.getByRole("button", { name: "Anotar visita" }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo/acolh1/visitantes")).toHaveLength(1));
    expect(await screen.findByText("Visita registrada")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Remover visita de Tia Maria" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-visitantes/visita1")).toHaveLength(1));
    expect(await screen.findByText("Visita removida")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Lançar termo" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/casa-abrigo/acolh1/termos")[0]!.body).toMatchObject({ tipo_termo: "passeio" }),
    );
    expect(await screen.findByText("Termo registrado")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Remover termo de Passeio" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-termos/termo1")).toHaveLength(1));
    expect(await screen.findByText("Termo removido")).toBeInTheDocument();

    await userEvent.clear(screen.getByLabelText("Escola"));
    await userEvent.type(screen.getByLabelText("Escola"), "Escola Estadual");
    await userEvent.click(screen.getByRole("button", { name: "Salvar PIA" }));
    await waitFor(() =>
      expect(api.to("PUT v1/sempras/casa-abrigo/acolh1/pia")[0]!.body).toMatchObject({ escola: "Escola Estadual" }),
    );
    expect(await screen.findByText("PIA atualizado")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Descrição da meta"), "nova meta de acompanhamento");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar meta" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/casa-abrigo/acolh1/pia-metas")[0]!.body).toEqual({ tipo: "plano", descricao: "nova meta de acompanhamento" }),
    );
    expect(await screen.findByText("Meta do PIA adicionada")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Concluir meta reduzir episódios de agressividade" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/casa-abrigo-pia-metas/meta1/concluir")[0]!.body).toEqual({ concluida: true }),
    );
    expect(await screen.findByText("Meta concluída")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Remover meta reduzir episódios de agressividade" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-pia-metas/meta1")).toHaveLength(1));
    expect(await screen.findByText("Meta removida")).toBeInTheDocument();
  });

  it("Casa Abrigo: documentos — envia um PDF (ticket, PUT, confirmação), baixa e remove", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u4");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "POST v1/sempras/casa-abrigo/acolh1/documentos/uploads": {
        status: 201,
        data: {
          documento: { ...PRONTUARIO.casa_abrigo_acolhimentos[0]!.documentos[0], id: "doc2", status: "pending" },
          upload: { object_key: "k2", upload_url: "http://minio.test/b/k2", method: "PUT", headers: {}, expires_at: "" },
        },
      },
      "PUT minio.test/*": { status: 200 },
      "POST v1/sempras/casa-abrigo-documentos/doc2/confirm": { data: { ...PRONTUARIO.casa_abrigo_acolhimentos[0]!.documentos[0], id: "doc2" } },
      "GET v1/sempras/casa-abrigo-documentos/doc1/download": { data: { url: "http://minio.test/b/doc1" } },
      "DELETE v1/sempras/casa-abrigo-documentos/doc1": { data: { removido: true } },
    });
    renderApp(<FichaPage />);
    const lista = within(await screen.findByRole("list", { name: "Acolhimentos" }));
    expect(lista.getByText(/certidao\.pdf/)).toBeInTheDocument();

    await userEvent.upload(
      document.getElementById("documento-arquivo-acolh1") as HTMLInputElement,
      new File(["%PDF-1.4"], "laudo.pdf", { type: "application/pdf" }),
    );
    await waitFor(() =>
      expect(api.to("POST v1/sempras/casa-abrigo/acolh1/documentos/uploads")[0]!.body).toEqual({
        tipo: "acolhido", filename: "laudo.pdf", content_type: "application/pdf", size: 8,
      }),
    );
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo-documentos/doc2/confirm")).toHaveLength(1));
    expect(await screen.findByText("Documento enviado")).toBeInTheDocument();

    await userEvent.click(lista.getByRole("button", { name: "Baixar documento certidao.pdf" }));
    await waitFor(() => expect(api.to("GET v1/sempras/casa-abrigo-documentos/doc1/download")).toHaveLength(1));

    await userEvent.click(lista.getByRole("button", { name: "Remover documento certidao.pdf" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-documentos/doc1")).toHaveLength(1));
    expect(await screen.findByText("Documento removido")).toBeInTheDocument();
  });

  it("Centro POP: salva o perfil de rua, encaminha pra fora do sistema e conclui, envia um documento", async () => {
    resetNavigation({ id: "c1" }, "/", "unidade=u3");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades": { data: TODAS },
      "PUT v1/sempras/cidadaos/c1/centro-pop-perfil": {
        data: { cidadao_id: "c1", situacao: "situacao_rua", tempo_rua_anos: 2, motivo_vulnerabilidade: "uso_de_drogas",
          possui_doenca: false, doenca_descricao: "", usa_medicacao: false, medicacao_descricao: "",
          uso_drogas: true, drogas_descricao: "uso de crack", atualizado_por: null, atualizado_em: "2026-10-08T12:00:00Z" },
      },
      "POST v1/sempras/cidadaos/c1/encaminhamentos-externos": {
        status: 201,
        data: { ...PRONTUARIO.encaminhamentos_externos[0], id: "encext2", destino: "saude_upa_hospital" },
      },
      "POST v1/sempras/encaminhamentos-externos/encext1/transitar": { data: { ...PRONTUARIO.encaminhamentos_externos[0], status: "concluido" } },
      "POST v1/sempras/cidadaos/c1/centro-pop-documentos/uploads": {
        status: 201,
        data: {
          documento: { id: "docCp1", cidadao_id: "c1", unidade_id: "u3", tipo: "laudo_saude", nome_original: "laudo.pdf",
            object_key: "k3", content_type: "application/pdf", size_bytes: 8, status: "pending", created_at: "2026-10-08T12:30:00Z" },
          upload: { object_key: "k3", upload_url: "http://minio.test/b/k3", method: "PUT", headers: {}, expires_at: "" },
        },
      },
      "PUT minio.test/*": { status: 200 },
      "POST v1/sempras/centro-pop-documentos/docCp1/confirm": {
        data: { id: "docCp1", cidadao_id: "c1", unidade_id: "u3", tipo: "laudo_saude", nome_original: "laudo.pdf",
          object_key: "k3", content_type: "application/pdf", size_bytes: 8, status: "ready", created_at: "2026-10-08T12:30:00Z" },
      },
    });
    renderApp(<FichaPage />);
    expect(await screen.findByRole("heading", { name: "João" })).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Motivo da vulnerabilidade"), "uso_de_drogas");
    await userEvent.click(screen.getByLabelText("Uso de drogas"));
    await userEvent.type(screen.getByLabelText("Descrição do uso de drogas"), "uso de crack");
    await userEvent.click(screen.getByRole("button", { name: "Salvar perfil de rua" }));
    await waitFor(() =>
      expect(api.to("PUT v1/sempras/cidadaos/c1/centro-pop-perfil")[0]!.body).toEqual({
        unidade_id: "u3", situacao: "situacao_rua", tempo_rua_anos: null, motivo_vulnerabilidade: "uso_de_drogas",
        possui_doenca: false, doenca_descricao: "", usa_medicacao: false, medicacao_descricao: "",
        uso_drogas: true, drogas_descricao: "uso de crack",
      }),
    );
    expect(await screen.findByText("Perfil de rua atualizado")).toBeInTheDocument();

    const listaEnc = within(await screen.findByRole("list", { name: "Encaminhamentos externos" }));
    expect(listaEnc.getByText("CAPS AD")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Destino"), "saude_upa_hospital");
    await userEvent.type(screen.getByLabelText("Motivo do encaminhamento externo *"), "avaliação médica urgente");
    await userEvent.click(screen.getByRole("button", { name: "Registrar encaminhamento externo" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/encaminhamentos-externos")[0]!.body).toEqual({
        unidade_id: "u3", destino: "saude_upa_hospital", prioridade: "media", motivo: "avaliação médica urgente",
      }),
    );
    expect(await screen.findByText("Encaminhamento externo registrado")).toBeInTheDocument();

    await userEvent.click(listaEnc.getByRole("button", { name: "Concluir" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/encaminhamentos-externos/encext1/transitar")[0]!.body).toEqual({ status: "concluido", observacoes: "" }),
    );
    expect(await screen.findByText("Encaminhamento atualizado")).toBeInTheDocument();

    await userEvent.upload(
      document.getElementById("centro-pop-documento-arquivo") as HTMLInputElement,
      new File(["%PDF-1.4"], "laudo.pdf", { type: "application/pdf" }),
    );
    await waitFor(() =>
      expect(api.to("POST v1/sempras/cidadaos/c1/centro-pop-documentos/uploads")[0]!.body).toEqual({
        unidade_id: "u3", tipo: "identificacao", filename: "laudo.pdf", content_type: "application/pdf", size: 8,
      }),
    );
    await waitFor(() => expect(api.to("POST v1/sempras/centro-pop-documentos/docCp1/confirm")).toHaveLength(1));
    expect(await screen.findByText("Documento enviado")).toBeInTheDocument();
  });
});

describe("SEMPRAS — pedidos do titular", () => {
  beforeEach(() => resetNavigation());

  const PEDIDO = {
    id: "p1", cidadao_id: "c1", cidadao_nome: "João", tipo: "anonimizacao", status: "pendente",
    justificativa_solicitacao: "titular pediu para ser esquecido", solicitado_por: "u1", solicitado_por_nome: "Técnico Um",
    justificativa_decisao: "", decidido_por: null, decidido_por_nome: "", decidido_em: null, created_at: "2026-10-08T10:00:00Z",
  };

  it("gestão aprova um pedido pendente", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/pedidos": { data: [PEDIDO] },
      "POST v1/sempras/pedidos/p1/decidir": { data: { ...PEDIDO, status: "concluido" } },
    });
    renderApp(<PedidosPage />);
    expect(await screen.findByText("João")).toBeInTheDocument();
    expect(screen.getByText("Técnico Um")).toBeInTheDocument();
    expect(screen.getByText("Pendente")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Decidir" }));
    const dialog = await screen.findByRole("dialog", { name: "Decidir anonimização — João" });
    expect(within(dialog).getByText("titular pediu para ser esquecido")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Aprovar anonimização" }));
    await waitFor(() => expect(api.to("POST v1/sempras/pedidos/p1/decidir")[0]!.body).toEqual({ aprovar: true, justificativa: "" }));
    expect(await screen.findByText("Anonimização aprovada")).toBeInTheDocument();
  });

  it("negar exige justificativa", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/pedidos": { data: [PEDIDO] },
      "POST v1/sempras/pedidos/p1/decidir": { data: { ...PEDIDO, status: "negado" } },
    });
    renderApp(<PedidosPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Decidir" }));
    const dialog = await screen.findByRole("dialog", { name: "Decidir anonimização — João" });
    expect(within(dialog).getByRole("button", { name: "Negar" })).toBeDisabled();
    await userEvent.type(within(dialog).getByLabelText(/Justificativa da decisão/), "sem elementos para decidir");
    await userEvent.click(within(dialog).getByRole("button", { name: "Negar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/pedidos/p1/decidir")[0]!.body).toEqual({ aprovar: false, justificativa: "sem elementos para decidir" }),
    );
  });

  it("sem a permissão de gestão: explica de quem é a decisão", async () => {
    mockBackend({ ...identityRoutes(recepcionista) });
    renderApp(<PedidosPage />);
    expect(await screen.findByText(/A decisão dos pedidos do titular é da gestão do SEMPRAS/)).toBeInTheDocument();
  });
});

describe("SEMPRAS — encaminhamentos recebidos", () => {
  beforeEach(() => resetNavigation());

  const RECEBIDO = {
    id: "enc1", cidadao_id: "c1", unidade_origem_id: "u1", unidade_destino_id: "u2",
    motivo: "precisa de acompanhamento especializado", status: "pendente", observacao_decisao: "",
    encaminhado_por: null, decidido_por: null, created_at: "2026-10-08T07:00:00Z", decidido_em: null,
  };

  it("aceita um encaminhamento pendente", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/unidades/u2/encaminhamentos": { data: [RECEBIDO] },
      "POST v1/sempras/encaminhamentos/enc1/decidir": { data: { ...RECEBIDO, status: "aceito" } },
    });
    renderApp(<EncaminhamentosRecebidos unidadeId="u2" />);
    expect(await screen.findByText("precisa de acompanhamento especializado")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Aceitar" }));
    await waitFor(() => expect(api.to("POST v1/sempras/encaminhamentos/enc1/decidir")[0]!.body).toEqual({ aceitar: true, observacao: "" }));
    expect(await screen.findByText("Encaminhamento aceito")).toBeInTheDocument();
  });

  it("recusar exige observação", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/unidades/u2/encaminhamentos": { data: [RECEBIDO] },
      "POST v1/sempras/encaminhamentos/enc1/decidir": { data: { ...RECEBIDO, status: "recusado" } },
    });
    renderApp(<EncaminhamentosRecebidos unidadeId="u2" />);
    await userEvent.click(await screen.findByRole("button", { name: "Recusar" }));
    const dialog = await screen.findByRole("dialog", { name: "Recusar encaminhamento" });
    expect(within(dialog).getByRole("button", { name: "Confirmar" })).toBeDisabled();
    await userEvent.type(within(dialog).getByLabelText("Observação *"), "unidade sem vaga no momento");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() =>
      expect(api.to("POST v1/sempras/encaminhamentos/enc1/decidir")[0]!.body).toEqual({ aceitar: false, observacao: "unidade sem vaga no momento" }),
    );
    expect(await screen.findByText("Encaminhamento recusado")).toBeInTheDocument();
  });
});

describe("SEMPRAS — painel de chamada", () => {
  beforeEach(() => resetNavigation());

  it("mostra os últimos chamados, em destaque o mais recente, e quantos aguardam", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/unidades/u1/fila": {
        data: [
          item("1", { status: "aguardando" }),
          item("2", { status: "aguardando" }),
          item("3", { status: "chamado", senha_exibida: "N003", nome: "Pessoa 3", local: "Guichê 1" }),
          item("4", { status: "chamado", senha_exibida: "N004", nome: "Pessoa 4", local: "Guichê 2" }),
        ],
      },
    });
    const { unmount } = renderApp(<PainelChamada unidade={CRAS} />);
    expect(await screen.findByText("2 pessoas aguardando")).toBeInTheDocument();
    const lista = within(await screen.findByRole("list", { name: "Últimas senhas chamadas" }));
    const itens = lista.getAllByRole("listitem");
    expect(itens[0]).toHaveTextContent("N004");
    expect(itens[0]).toHaveTextContent("Pessoa 4");
    expect(itens[1]).toHaveTextContent("N003");
    unmount();

    // Unidade sigilosa (ADR 031 §3): a TV mostra só a senha, nunca o nome.
    renderApp(<PainelChamada unidade={{ ...CREAS, id: "u1" }} />);
    const sigiloso = within(await screen.findByRole("list", { name: "Últimas senhas chamadas" }));
    expect(sigiloso.getAllByRole("listitem")[0]).toHaveTextContent("N004");
    expect(sigiloso.queryByText("Pessoa 4")).not.toBeInTheDocument();
  });

  it("sem ninguém chamado ainda", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/unidades/u1/fila": { data: [] },
    });
    renderApp(<PainelChamada unidade={CRAS} />);
    expect(await screen.findByText("Nenhuma senha chamada")).toBeInTheDocument();
    expect(screen.getByText("0 pessoas aguardando")).toBeInTheDocument();
  });

  it("a página escolhe a unidade e tem título próprio (h1); só unidades com fila", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/unidades": { data: [ABRIGO, CRAS] },
      "GET v1/sempras/unidades/u1/fila": { data: [] },
    });
    renderApp(<PainelChamadaPage />);
    expect(await screen.findByLabelText("Unidade")).toHaveValue("u1");
    expect(within(screen.getByLabelText("Unidade")).queryByRole("option", { name: "Casa Abrigo" })).not.toBeInTheDocument();
    expect(await screen.findByRole("heading", { level: 1, name: "SEMPRAS — Painel de chamada" })).toBeInTheDocument();
    expect(await screen.findByText("Nenhuma senha chamada")).toBeInTheDocument();
  });

  it("sem a permissão de atendimento: explica de quem é o painel", async () => {
    mockBackend({ ...identityRoutes(recepcionista) });
    renderApp(<PainelChamadaPage />);
    expect(await screen.findByText(/O painel de chamada é de quem atende a fila do SEMPRAS/)).toBeInTheDocument();
  });
});

describe("SEMPRAS — atendimentos coletivos", () => {
  beforeEach(() => resetNavigation());

  it("escolhe a unidade, lista e registra um evento de grupo", async () => {
    resetNavigation({}, "/", "unidade=u1");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/unidades": { data: TODAS },
      "GET v1/sempras/unidades/u1/coletivos": { data: [
        { id: "col1", unidade_id: "u1", data_evento: "2026-10-05", categoria: "scfv_0_6", participantes: 8,
          descricao: "turma da manhã", created_at: "2026-10-05T00:00:00Z" },
      ] },
      "POST v1/sempras/unidades/u1/coletivos": { status: 201, data: { id: "col2", unidade_id: "u1", data_evento: "2026-10-10",
        categoria: "scfv_7_14", participantes: 12, descricao: "", created_at: "2026-10-10T00:00:00Z" } },
    });
    renderApp(<ColetivosPage />);
    expect(await screen.findByRole("heading", { level: 1, name: "SEMPRAS — Atendimentos coletivos" })).toBeInTheDocument();
    // O CREAS não oferta coletivos: não aparece no seletor.
    expect(within(await screen.findByLabelText("Unidade")).queryByRole("option", { name: "CREAS" })).not.toBeInTheDocument();
    const lista = within(await screen.findByRole("list", { name: "Atendimentos coletivos" }));
    expect(lista.getByText("SCFV — crianças até 6 anos")).toBeInTheDocument();
    expect(lista.getByText(/8 participantes/)).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Categoria *"), "scfv_7_14");
    await userEvent.clear(screen.getByLabelText("Participantes *"));
    await userEvent.type(screen.getByLabelText("Participantes *"), "12");
    await userEvent.click(screen.getByRole("button", { name: "Registrar" }));
    await waitFor(() => expect(api.to("POST v1/sempras/unidades/u1/coletivos")).toHaveLength(1));
    expect(await screen.findByText("Atendimento coletivo registrado")).toBeInTheDocument();
  });

  it("sem a permissão de atendimento: explica de quem são os coletivos", async () => {
    mockBackend({ ...identityRoutes(recepcionista) });
    renderApp(<ColetivosPage />);
    expect(await screen.findByText(/Os atendimentos coletivos são de quem atende a unidade do SEMPRAS/)).toBeInTheDocument();
  });
});

describe("SEMPRAS — configuração das unidades", () => {
  beforeEach(() => resetNavigation());

  it("a gestão dá o tipo a uma unidade nova; o painel segue o padrão do tipo", async () => {
    const nova = { ...CRAS, id: "u9", nome: "CRAS Novo", tipo: "", servicos: [], demandas: [], painel_exibe_nome: false };
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/sempras/unidades/configuracao": { data: [CRAS, nova] },
      "PUT v1/sempras/unidades/u9/configuracao": { data: { ...nova, tipo: "creas" } },
    });
    renderApp(<UnidadesPage />);
    expect(await screen.findByRole("heading", { level: 1, name: "SEMPRAS — Unidades" })).toBeInTheDocument();
    const linha = within(screen.getByRole("row", { name: /CRAS Novo/ }));
    expect(linha.getByText("Não atende")).toBeInTheDocument();
    expect(linha.getByRole("button", { name: "Salvar" })).toBeDisabled();
    await userEvent.selectOptions(linha.getByLabelText("Tipo de CRAS Novo"), "creas");
    // CREAS: painel só com a senha por padrão (ADR 031 §3).
    expect(linha.getByLabelText("Painel de CRAS Novo mostra o nome")).not.toBeChecked();
    await userEvent.click(linha.getByRole("button", { name: "Salvar" }));
    await waitFor(() =>
      expect(api.to("PUT v1/sempras/unidades/u9/configuracao")[0]!.body).toEqual({ tipo: "creas", painel_exibe_nome: false }),
    );
  });

  it("sem a permissão de gestão: explica de quem é a configuração", async () => {
    mockBackend({ ...identityRoutes(recepcionista) });
    renderApp(<UnidadesPage />);
    expect(await screen.findByText(/A configuração das unidades é da gestão do SEMPRAS/)).toBeInTheDocument();
  });
});
