import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CasaAbrigoWorkspace } from "@/components/sempras/casa-abrigo/CasaAbrigoWorkspace";
import { intervalo } from "@/components/sempras/casa-abrigo/RelatoriosAba";
import { dadosVazios, fmtDia, iniciais, idadeEm, mascaraCEP, mascaraCPF, mascaraTelefone, piaVazia } from "@/lib/sempras/casaAbrigo";
import { identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";
import { PRONTUARIO } from "@/test/sempras";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);
vi.setConfig({ testTimeout: 40000 });

const B = "v1/sempras/unidades/u4/casa-abrigo";

const resumo = (id: string, extra: Record<string, unknown> = {}) => ({
  acolhimento_id: id, cidadao_id: `c-${id}`, nome: `Criança ${id}`, nome_social: "", data_nascimento: "2015-03-01", idade: 11,
  sexo: "feminino", data_entrada: "2026-09-01T00:00:00Z", processo_judicial: "", guia_acolhimento: "", situacao_juridica: "",
  motivos: ["negligencia"], status: "acolhido", desligamento_tipo: "", data_desligamento: null, pia_atualizado_em: null,
  proxima_reavaliacao_pia: "2026-11-30T00:00:00Z", documento_acolhido: false, documento_responsavel: false, ...extra,
});

const PAINEL = {
  criancas: 1, adolescentes: 1, ativos: 2, pias_vencendo: 1, desacolhidos: 1,
  acolhidos: [
    resumo("a1", { nome: "Ana Souza", processo_judicial: "123", guia_acolhimento: "G1", documento_acolhido: true, documento_responsavel: true }),
    resumo("a2", { nome: "Bruno Lima", nome_social: "Bru", idade: 14 }),
  ],
  desligados: [resumo("a3", { nome: "Caio Reis", status: "desligado", desligamento_tipo: "adocao", data_desligamento: "2026-10-01T00:00:00Z" }),
    resumo("a4", { nome: "Dora", status: "desligado" })],
  alertas: [
    { tipo: "pia", nivel: "aviso", mensagem: "Reavaliação do PIA de A. S.", prazo: "2026-10-20T00:00:00Z", dias: 3, acolhimento_id: "a1", ref_id: "a1" },
    { tipo: "relatorio", nivel: "atraso", mensagem: "Relatório mensal atrasado", prazo: "2026-10-15T00:00:00Z", dias: -2, ref_id: "r1" },
    { tipo: "tarefa", nivel: "aviso", mensagem: "Tarefa: comprar material", prazo: "2026-10-17T00:00:00Z", dias: 0, ref_id: "t1" },
  ],
};
const VAZIO = { criancas: 0, adolescentes: 0, ativos: 0, pias_vencendo: 0, desacolhidos: 0, acolhidos: [], desligados: [], alertas: [] };

const base = PRONTUARIO.casa_abrigo_acolhimentos[0]!;
const meta = (id: string, tipo: string, extra: Record<string, unknown> = {}) => ({
  id, acolhimento_id: "a1", tipo, descricao: `meta ${id}`, providencia: "", responsavel: "", prazo: null, esportes: "", cultura: "",
  lazer: "", concluida: false, created_at: "2026-10-01T00:00:00Z", ...extra,
});
const DETALHE = {
  ...base, id: "a1", unidade_id: "u4", cidadao_id: "c-a1",
  cidadao: { id: "c-a1", nome: "Ana Souza", nome_social: "", data_nascimento: "2015-03-01", sexo: "feminino", cpf: "", nis: "", nome_mae: "M" },
  responsavel_nascimento: "1980-01-02T00:00:00Z", data_guia: null, data_situacao_juridica: "2026-09-02T00:00:00Z",
  diagnosticos: [{ id: "d1", acolhimento_id: "a1", categoria: "doenca", diagnostico: "Diabetes", descricao: "", created_at: "" }],
  irmaos: [{ id: "i1", acolhimento_id: "a1", nome: "Irmã Salva", data_nascimento: null, esta_acolhido: true, unidade_descricao: "", observacao: "", created_at: "" }],
  pia_metas: [meta("m1", "plano", { prazo: "2026-11-01T00:00:00Z", esportes: "futebol" }), meta("m2", "intervencao", { concluida: true, prazo: "2026-12-01T00:00:00Z" }),
    meta("m3", "intervencao")],
};

const backend = (extra: Record<string, unknown> = {}) =>
  mockBackend({
    ...identityRoutes(),
    [`GET ${B}`]: { data: PAINEL },
    "GET v1/sempras/casa-abrigo/a1": { data: DETALHE },
    "GET v1/sempras/unidades/u4/encaminhamentos": { data: [] },
    ...(extra as Record<string, { data?: unknown; status?: number }>),
  });

const aba = async (nome: RegExp | string) => userEvent.click(await screen.findByRole("tab", { name: nome }));

describe("Casa Abrigo — utilitários", () => {
  it("idade, iniciais, máscaras e o período dos relatórios", () => {
    expect(idadeEm("2015-03-01", new Date(2026, 1, 28))).toBe(10);
    expect(idadeEm("2015-03-01", new Date(2026, 2, 1))).toBe(11);
    expect(idadeEm("2015-03-01", new Date(2026, 2, 0))).toBe(10);
    expect(idadeEm("ruim")).toBeNull();
    expect(idadeEm("2030-01-01", new Date(2026, 0, 1))).toBe(0);
    expect(iniciais("ana maria da silva")).toBe("A. M. S.");
    expect(iniciais(" ")).toBe("N/I");
    expect(mascaraCPF("52998224725")).toBe("529.982.247-25");
    expect(mascaraCEP("78700000")).toBe("78700-000");
    expect(mascaraTelefone("66999990000")).toBe("(66) 99999-0000");
    expect(intervalo("mensal", "2026-02")).toEqual({ de: "2026-02-01", ate: "2026-02-28", rotulo: "Mês 02/2026" });
    expect(intervalo("trimestral", "2026-05")).toEqual({ de: "2026-04-01", ate: "2026-06-30", rotulo: "2º trimestre de 2026" });
    expect(intervalo("anual", "2026-05")).toEqual({ de: "2026-01-01", ate: "2026-12-31", rotulo: "Ano de 2026" });
    // a data "pura" volta no fuso da API (Cuiabá): o dia é o de UTC
    expect(fmtDia("2026-09-30T20:00:00-04:00")).toBe("01/10/2026");
    expect(fmtDia("2026-10-01")).toBe("01/10/2026");
    expect(fmtDia("data ruim")).toBe("—");
    expect(fmtDia(null)).toBe("—");
    expect(Object.keys(dadosVazios())).toContain("motivos");
    expect(piaVazia().escola).toBe("");
  });
});

describe("Casa Abrigo — painel e acolhidos", () => {
  beforeEach(() => resetNavigation({}, "/", ""));
  afterEach(() => vi.unstubAllGlobals());

  it("painel: cartões, monitor e o aviso que abre a aba certa", async () => {
    backend({ "GET v1/sempras/unidades/u4/casa-abrigo/relatorios": { data: [] }, "GET v1/sempras/unidades/u4/casa-abrigo/tarefas": { data: [] } });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" extras={<p>extra do painel</p>} />);
    const cartoes = within(await screen.findByRole("list", { name: "Indicadores da unidade" }));
    expect(cartoes.getByText("PIAs vencendo").nextSibling).toHaveTextContent("01");
    expect(screen.getByText("extra do painel")).toBeInTheDocument();
    const avisos = within(screen.getByRole("list", { name: "Avisos de prazo" }));
    expect(avisos.getByText(/vence em 3 dia/)).toBeInTheDocument();
    expect(avisos.getByText(/atrasado há 2 dia/)).toBeInTheDocument();
    expect(avisos.getByText(/vence hoje/)).toBeInTheDocument();
    await userEvent.click(avisos.getByText("Reavaliação do PIA de A. S."));
    expect(await screen.findByRole("tab", { name: "P.I.A.", selected: true })).toBeInTheDocument();
    expect(await screen.findByText(/Última reavaliação/)).toBeInTheDocument();
    await aba("Painel");
    await userEvent.click(within(screen.getByRole("list", { name: "Avisos de prazo" })).getByText("Relatório mensal atrasado"));
    expect(await screen.findByRole("tab", { name: "Relatórios", selected: true })).toBeInTheDocument();
    await aba("Painel");
    await userEvent.click(within(screen.getByRole("list", { name: "Avisos de prazo" })).getByText("Tarefa: comprar material"));
    expect(await screen.findByRole("tab", { name: "Tarefas da equipe", selected: true })).toBeInTheDocument();
  });

  it("painel sem pendências; aba pedida pela URL; aba inexistente cai no painel", async () => {
    mockBackend({ ...identityRoutes(), [`GET ${B}`]: { data: VAZIO }, [`GET ${B}/tarefas`]: { data: [] } });
    resetNavigation({}, "/", "aba=tarefas");
    const { unmount } = renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    expect(await screen.findByRole("tab", { name: "Tarefas da equipe", selected: true })).toBeInTheDocument();
    unmount();
    resetNavigation({}, "/", "aba=nada");
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    expect(await screen.findByText(/Tudo em ordem/)).toBeInTheDocument();
  });

  it("institucionalizados: busca, desligados, CSV, impressão e desligamento", async () => {
    const api = backend({ "POST v1/sempras/casa-abrigo/a2/desligar": { data: {} } });
    const criar = vi.fn(() => "blob:csv");
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: criar, revokeObjectURL: vi.fn() }));
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Acolhidos");
    const tabela = await screen.findByRole("table", { name: "Relatório de institucionalizados" });
    expect(within(tabela).getByRole("link", { name: "Ana Souza" })).toHaveAttribute("href", "/sempras/cidadaos/c-a1?unidade=u4");
    expect(within(tabela).getByText("Bru")).toBeInTheDocument();
    expect(within(tabela).getAllByText("N/I")).toHaveLength(2); // processo e guia que faltam ficam em destaque
    await userEvent.type(screen.getByLabelText("Buscar por nome"), "ana");
    expect(within(tabela).queryByText("Bru")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Exportar CSV/ }));
    expect(criar).toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /Imprimir/ }));
    expect(print).toHaveBeenCalled();
    await userEvent.clear(screen.getByLabelText("Buscar por nome"));
    await userEvent.click(screen.getByRole("button", { name: /Ver desligados/ }));
    const desligados = await screen.findByRole("table", { name: "Desligados da unidade" });
    expect(within(desligados).getByText("Adoção")).toBeInTheDocument();
    expect(within(desligados).getAllByText("—").length).toBeGreaterThan(0);
    await userEvent.click(screen.getByRole("button", { name: "Ver acolhidos" }));

    await userEvent.click(await screen.findByRole("button", { name: "Desligar Bruno Lima" }));
    const dialog = await screen.findByRole("dialog", { name: "Desligar acolhimento" });
    await userEvent.selectOptions(within(dialog).getByLabelText("Tipo de desligamento"), "transferencia");
    await userEvent.type(within(dialog).getByLabelText("Motivo *"), "transferido para outra casa");
    await userEvent.type(within(dialog).getByLabelText("Nº da guia de desligamento"), "GD-2");
    fireEvent.change(within(dialog).getByLabelText("Data da guia"), { target: { value: "2026-10-16" } });
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar desligamento" }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo/a2/desligar")[0]!.body).toMatchObject({
      tipo: "transferencia", motivo: "transferido para outra casa", guia: "GD-2", data_guia: "2026-10-16T00:00:00Z" }));
    print.mockRestore();
  });

  it("desligamento recusado mantém o diálogo; sem data de guia vai nulo", async () => {
    const api = backend({ "POST v1/sempras/casa-abrigo/a1/desligar": { status: 422, error: { code: "V", message: "data inválida" } } });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Acolhidos");
    await userEvent.click(await screen.findByRole("button", { name: "Desligar Ana Souza" }));
    const dialog = await screen.findByRole("dialog", { name: "Desligar acolhimento" });
    await userEvent.type(within(dialog).getByLabelText("Motivo *"), "evadiu");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar desligamento" }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo/a1/desligar")[0]!.body).toMatchObject({ data_guia: null }));
    expect(await screen.findByText("data inválida")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Desligar acolhimento" })).toBeInTheDocument();
  });

  it("linha do tempo do acolhido: eventos do mais recente ao mais antigo, sigilo e impressão", async () => {
    backend({
      "GET v1/sempras/casa-abrigo/a1/linha-do-tempo": { data: [
        { data: "2026-10-12T15:00:00-04:00", tipo: "evolucao", titulo: "Evolução psicológica", detalhe: "Registro sigiloso da psicologia" },
        { data: "2026-10-05T00:00:00Z", tipo: "visita", titulo: "Visita de Avó", detalhe: "" },
        { data: "invalida", tipo: "documento", titulo: "Documento anexado", detalhe: "rg.pdf" },
        { data: "2026-09-01T00:00:00Z", tipo: "entrada", titulo: "Entrada na unidade", detalhe: "Processo 123." },
      ] },
      "GET v1/sempras/casa-abrigo/a2/linha-do-tempo": { data: [] },
    });
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Acolhidos");
    await userEvent.click(await screen.findByRole("button", { name: "Linha do tempo de Ana Souza" }));
    const dialog = await screen.findByRole("dialog", { name: "Linha do tempo — Ana Souza" });
    expect(await within(dialog).findAllByText("Registro sigiloso da psicologia")).not.toHaveLength(0);
    expect(within(dialog).getAllByText("12/10/2026")).not.toHaveLength(0);
    expect(within(dialog).getAllByText("05/10/2026")).not.toHaveLength(0);
    expect(within(dialog).getAllByText("—")).not.toHaveLength(0);
    expect(within(dialog).getAllByText("Processo 123.")).not.toHaveLength(0);
    await userEvent.click(within(dialog).getByRole("button", { name: /Imprimir/ }));
    expect(print).toHaveBeenCalled();
    fireEvent(dialog, new Event("cancel"));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: /Linha do tempo/ })).not.toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "Linha do tempo de Bruno Lima" }));
    const vazio = await screen.findByRole("dialog", { name: "Linha do tempo — Bru" });
    expect(await within(vazio).findByText("Nada registrado ainda")).toBeInTheDocument();
    expect(within(vazio).getByRole("button", { name: /Imprimir/ })).toBeDisabled();
    print.mockRestore();
  });
});

describe("Casa Abrigo — assistente de 6 etapas", () => {
  beforeEach(() => resetNavigation({}, "/", ""));
  afterEach(() => vi.unstubAllGlobals());

  it("cadastro novo: criança nova, responsável com CEP, irmãos, motivos, saúde, social e PDFs", async () => {
    const api = backend({
      "GET v1/sempras/cidadaos": { data: [] },
      "POST v1/sempras/cidadaos": { status: 201, data: { id: "c-nova" } },
      "POST v1/sempras/cidadaos/c-nova/casa-abrigo": { status: 201, data: { id: "a9" } },
      "POST v1/sempras/casa-abrigo/a9/documentos/uploads": { status: 201, data: {
        upload: { object_key: "k", upload_url: "http://minio.test/b/k", method: "PUT", headers: {}, expires_at: "" }, documento: { id: "doc9" } } },
      "PUT minio.test/*": { status: 200 },
      "POST v1/sempras/casa-abrigo-documentos/doc9/confirm": { data: {} },
    });
    // o CEP vai ao ViaCEP (fora do backend): 1º falha, cai no BrasilAPI; 2º acha no ViaCEP; 3º nenhum acha
    const fetchBackend = globalThis.fetch;
    let cepTentativa = 0;
    vi.stubGlobal("fetch", async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("viacep")) {
        cepTentativa++;
        if (cepTentativa === 2) return new Response(JSON.stringify({ logradouro: "Rua ViaCEP", bairro: "Centro", localidade: "Rondonópolis", uf: "MT" }));
        return new Response(JSON.stringify({ erro: true }));
      }
      if (url.includes("brasilapi")) {
        if (cepTentativa === 1) return new Response(JSON.stringify({ street: "Rua BrasilAPI", neighborhood: "Vila", city: "Rondonópolis", state: "MT" }));
        return new Response("{}", { status: 404 });
      }
      return fetchBackend(input, init);
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Acolhidos");
    await userEvent.click(await screen.findByRole("button", { name: /Novo acolhimento/ }));

    // 1. criança: busca sem resultado, cadastra nova
    await userEvent.type(screen.getByLabelText(/Buscar no cadastro/), "Joana");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(await screen.findByText(/Ninguém encontrado/)).toBeInTheDocument();
    await userEvent.click(screen.getByLabelText("Situação especial"));
    await userEvent.click(screen.getByLabelText("PCD"));
    await userEvent.type(screen.getByLabelText("Nome completo *"), "Joana Nova");
    await userEvent.type(screen.getByLabelText("Nome social"), "Jô");
    await userEvent.type(screen.getByLabelText("CPF"), "52998224725");
    expect(screen.getByLabelText("CPF")).toHaveValue("529.982.247-25");
    await userEvent.click(screen.getByLabelText("Não possui CPF"));
    expect(screen.getByLabelText("CPF")).toBeDisabled();
    await userEvent.click(screen.getByLabelText("Não possui CPF"));
    await userEvent.type(screen.getByLabelText("NIS"), "1234567890123");
    fireEvent.change(screen.getByLabelText("Data de nascimento *"), { target: { value: "2016-04-02" } });
    expect(screen.getByText(/\d+ anos/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Nome da mãe (cadastro)"), "Mãe da Joana");
    await userEvent.type(screen.getByLabelText("Apelido"), "Jojo");
    await userEvent.selectOptions(screen.getByLabelText("Identidade de gênero"), "cisgenero");
    await userEvent.selectOptions(screen.getByLabelText("Orientação sexual"), "outro");
    await userEvent.selectOptions(screen.getByLabelText("Raça/cor"), "parda");
    await userEvent.type(screen.getByLabelText("Filiação 1 (mãe/pai)"), "Mãe da Joana");
    await userEvent.type(screen.getByLabelText("Filiação 2 (mãe/pai)"), "Pai");

    // tentar finalizar cedo: falta o motivo
    await userEvent.click(screen.getByRole("button", { name: /6\. Social/ }));
    await userEvent.click(screen.getByRole("button", { name: "Finalizar cadastro" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Marque ao menos um motivo");

    // 2. responsável
    await userEvent.click(screen.getByRole("button", { name: /2\. Responsável/ }));
    await userEvent.type(screen.getByLabelText("Nome do(a) responsável"), "Tia Rosa");
    await userEvent.type(screen.getByLabelText("CPF"), "52998224725");
    await userEvent.type(screen.getByLabelText("Nome social"), "Rosa");
    await userEvent.type(screen.getByLabelText("Apelido"), "Ro");
    await userEvent.type(screen.getByLabelText("NIS"), "12345678901");
    fireEvent.change(screen.getByLabelText("Data de nascimento"), { target: { value: "1980-01-02" } });
    await userEvent.selectOptions(screen.getByLabelText("Sexo"), "feminino");
    await userEvent.selectOptions(screen.getByLabelText("Grau de parentesco / vínculo"), "tia");
    await userEvent.type(screen.getByLabelText("CEP"), "787");
    await userEvent.click(screen.getByRole("button", { name: "Buscar CEP" }));
    expect(await screen.findByText("Digite um CEP com 8 números.")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("CEP"), "00000");
    await userEvent.click(screen.getByRole("button", { name: "Buscar CEP" }));
    await waitFor(() => expect(screen.getByLabelText("Logradouro")).toHaveValue("Rua BrasilAPI"));
    await userEvent.click(screen.getByRole("button", { name: "Buscar CEP" }));
    await waitFor(() => expect(screen.getByLabelText("Logradouro")).toHaveValue("Rua ViaCEP"));
    await userEvent.click(screen.getByRole("button", { name: "Buscar CEP" }));
    expect(await screen.findByText("CEP não encontrado.")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Número"), "10");
    await userEvent.type(screen.getByLabelText("Complemento"), "fundos");
    await userEvent.clear(screen.getByLabelText("Bairro"));
    await userEvent.type(screen.getByLabelText("Bairro"), "Vila Operária");
    await userEvent.clear(screen.getByLabelText("Cidade"));
    await userEvent.type(screen.getByLabelText("Cidade"), "Rondonópolis");
    await userEvent.clear(screen.getByLabelText("UF"));
    await userEvent.type(screen.getByLabelText("UF"), "mt");
    await userEvent.type(screen.getByLabelText("Telefone fixo"), "6634210000");
    await userEvent.type(screen.getByLabelText("Celular / WhatsApp"), "66999990000");
    await userEvent.type(screen.getByLabelText("E-mail"), "rosa@ex.com");
    await userEvent.click(screen.getByLabelText("Não possui responsável"));
    await userEvent.click(screen.getByLabelText("Não possui responsável"));

    // 3. acolhimento: data, processos, histórico, irmãos e motivos
    await userEvent.click(screen.getByRole("button", { name: /Próxima etapa/ }));
    fireEvent.change(screen.getByLabelText("Data na unidade *"), { target: { value: "2026-10-01" } });
    await userEvent.type(screen.getByLabelText("Nº do 1º processo"), "123/2026");
    await userEvent.type(screen.getByLabelText("Nº do 2º processo"), "456/2026");
    await userEvent.type(screen.getByLabelText("Nº da guia de acolhimento"), "G-1");
    fireEvent.change(screen.getByLabelText("Data da guia"), { target: { value: "2026-10-01" } });
    const historico = screen.getByRole("group", { name: "Acolhido(a) anteriormente?" });
    await userEvent.click(within(historico).getByLabelText("Não sei"));
    await userEvent.click(within(historico).getByRole("button", { name: "limpar" }));
    await userEvent.click(within(historico).getByLabelText("Sim"));
    await userEvent.click(within(screen.getByRole("group", { name: "Irmão(s) nesta unidade?" })).getByLabelText("Não"));
    await userEvent.click(within(screen.getByRole("group", { name: "Irmão(s) em outra unidade?" })).getByLabelText("Sim"));
    await userEvent.type(screen.getByLabelText("Nome do irmão"), "Irmão Um");
    await userEvent.click(screen.getByLabelText("Está acolhido?"));
    await userEvent.click(screen.getByRole("button", { name: "Adicionar irmão" }));
    await userEvent.type(screen.getByLabelText("Nome do irmão"), "Irmão Dois");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar irmão" }));
    await userEvent.click(screen.getByRole("button", { name: "Remover Irmão Dois" }));
    expect(within(screen.getByRole("list", { name: "Irmãos" })).getByText("Irmão Um")).toBeInTheDocument();
    await userEvent.click(screen.getByLabelText("Negligência"));
    await userEvent.click(screen.getByLabelText("Abandono"));
    await userEvent.click(screen.getByLabelText("Abandono"));
    await userEvent.click(screen.getByLabelText("Outro"));
    await userEvent.click(screen.getByRole("button", { name: /6\. Social/ }));
    await userEvent.click(screen.getByRole("button", { name: "Finalizar cadastro" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Especifique o outro motivo");
    await userEvent.click(screen.getByRole("button", { name: /3\. Acolhimento/ }));
    await userEvent.type(screen.getByLabelText("Especifique o outro motivo *"), "pais presos");

    // 4. jurídico
    await userEvent.click(screen.getByRole("button", { name: /Próxima etapa/ }));
    await userEvent.selectOptions(screen.getByLabelText("Situação jurídica atual"), "processo_guarda");
    fireEvent.change(screen.getByLabelText("Data da situação jurídica"), { target: { value: "2026-10-02" } });
    await userEvent.type(screen.getByLabelText("Conselheiro(a) tutelar responsável"), "Conselheira Ana");
    await userEvent.type(screen.getByLabelText("Telefone do conselheiro"), "66999991111");
    await userEvent.click(within(screen.getByRole("group", { name: /CNA/ })).getByLabelText("Não"));
    await userEvent.click(within(screen.getByRole("group", { name: /CNCAD/ })).getByLabelText("Não sei"));

    // 5. saúde: laudo, checklist e transtornos
    await userEvent.click(screen.getByRole("button", { name: /Próxima etapa/ }));
    await userEvent.click(within(screen.getByRole("group", { name: "Possui laudo/diagnóstico médico?" })).getByLabelText("Sim"));
    await userEvent.type(screen.getByLabelText("Descrição do laudo/diagnóstico"), "laudo de TEA");
    await userEvent.click(within(screen.getByRole("group", { name: "Uso de medicação contínua?" })).getByLabelText("Sim"));
    await userEvent.click(within(screen.getByRole("group", { name: "Possui dependência química?" })).getByLabelText("Não"));
    await userEvent.click(screen.getByLabelText("Diabetes"));
    await userEvent.click(screen.getByLabelText("Surdez"));
    await userEvent.click(screen.getByLabelText("Surdez"));
    await userEvent.click(within(screen.getByRole("group", { name: /Possui transtornos/ })).getByLabelText("Sim"));
    await userEvent.click(screen.getByLabelText("TEA - Autismo (Grau 1)"));
    await userEvent.type(screen.getByLabelText("Descrição do transtorno / síndrome / deficiência"), "suporte leve");

    // 6. contexto social e documentos
    await userEvent.click(screen.getByRole("button", { name: /Próxima etapa/ }));
    for (const g of ["Está estudando?", "Ameaça de morte?", "Recebe visita institucional?", "Participa de programa social ou governamental?"]) {
      await userEvent.click(within(screen.getByRole("group", { name: g })).getByLabelText("Sim"));
    }
    await userEvent.click(within(screen.getByRole("group", { name: "Possui benefício do governo?" })).getByLabelText("Sim"));
    await userEvent.type(screen.getByLabelText("Qual benefício?"), "BPC");
    await userEvent.selectOptions(screen.getByLabelText("Está em uma relação de trabalho?"), "nao");
    await userEvent.type(screen.getByLabelText("Observações gerais"), "chegou à noite");
    await userEvent.upload(screen.getByLabelText("Documentos do acolhido"), new File(["%PDF"], "rg.pdf", { type: "application/pdf" }));
    await userEvent.upload(screen.getByLabelText("Documentos do responsável"), new File(["%PDF"], "resp.pdf", { type: "application/pdf" }));
    await userEvent.click(screen.getByRole("button", { name: /← Voltar/ }));
    await userEvent.click(screen.getByRole("button", { name: /Próxima etapa/ }));
    await userEvent.click(screen.getByRole("button", { name: "Finalizar cadastro" }));

    await waitFor(() => expect(api.to("POST v1/sempras/cidadaos/c-nova/casa-abrigo")).toHaveLength(1));
    expect(api.to("POST v1/sempras/cidadaos")[0]!.body).toMatchObject({ nome: "Joana Nova", nome_social: "Jô", cpf: "",
      nis: "12345678901", data_nascimento: "2016-04-02", sexo: "feminino", nome_mae: "Mãe da Joana" });
    const corpo = api.to("POST v1/sempras/cidadaos/c-nova/casa-abrigo")[0]!.body as Record<string, unknown>;
    expect(corpo).toMatchObject({
      unidade_id: "u4", data_entrada: "2026-10-01T00:00:00Z", situacao_especial: true, pcd: true, apelido: "Jojo", raca_cor: "parda",
      responsavel_nome: "Tia Rosa", responsavel_cpf: "52998224725", responsavel_nascimento: "1980-01-02T00:00:00Z", responsavel_vinculo: "tia",
      responsavel_uf: "MT", responsavel_celular: "(66) 99999-0000", motivos: ["negligencia", "outro"], outro_motivo: "pais presos",
      historico_acolhido_anteriormente: "sim", situacao_juridica: "processo_guarda", data_situacao_juridica: "2026-10-02T00:00:00Z",
      data_guia: "2026-10-01T00:00:00Z", cna: "nao", cncad: "nao_sei", laudo_medico: "sim", possui_transtorno: "sim",
      beneficio_qual: "BPC", relacao_trabalho: "nao", irmaos: [{ nome: "Irmão Um", esta_acolhido: true }],
      diagnosticos: [{ categoria: "doenca", diagnostico: "Diabetes" }, { categoria: "transtorno", diagnostico: "TEA - Autismo (Grau 1)" }],
    });
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo-documentos/doc9/confirm")).toHaveLength(2));
    expect(await screen.findByText("Acolhimento cadastrado")).toBeInTheDocument();
    expect(await screen.findByRole("table", { name: "Relatório de institucionalizados" })).toBeInTheDocument();
  });

  it("cadastro com criança já no cadastro; validação do nome; cancelar volta à lista", async () => {
    const api = backend({
      "GET v1/sempras/cidadaos": { data: [{ id: "c7", nome: "Pedro", nome_social: "", cpf: "", nis: "", data_nascimento: "", atendimentos_na_area: 0 }] },
      "POST v1/sempras/cidadaos/c7/casa-abrigo": { status: 201, data: { id: "a7" } },
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Acolhidos");
    await userEvent.click(await screen.findByRole("button", { name: /Novo acolhimento/ }));
    await userEvent.click(screen.getByRole("button", { name: /6\. Social/ }));
    await userEvent.click(screen.getByRole("button", { name: "Finalizar cadastro" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Preencha o nome e a data de nascimento");
    await userEvent.click(screen.getByRole("button", { name: /1\. Criança/ }));
    await userEvent.type(screen.getByLabelText(/Buscar no cadastro/), "Pedro");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    await userEvent.click(await screen.findByRole("button", { name: /Pedro · sem nascimento/ }));
    expect(screen.getByText(/não informado/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Trocar" }));
    await userEvent.click(await screen.findByRole("button", { name: /Pedro · sem nascimento/ }));
    await userEvent.click(screen.getByRole("button", { name: /3\. Acolhimento/ }));
    fireEvent.change(screen.getByLabelText("Data na unidade *"), { target: { value: "" } });
    await userEvent.click(screen.getByLabelText("Orfandade"));
    await userEvent.click(screen.getByRole("button", { name: /6\. Social/ }));
    await userEvent.click(screen.getByRole("button", { name: "Finalizar cadastro" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Preencha a data na unidade");
    await userEvent.click(screen.getByRole("button", { name: /3\. Acolhimento/ }));
    fireEvent.change(screen.getByLabelText("Data na unidade *"), { target: { value: "2026-10-05" } });
    await userEvent.click(screen.getByRole("button", { name: /6\. Social/ }));
    await userEvent.click(screen.getByRole("button", { name: "Finalizar cadastro" }));
    await waitFor(() => expect(api.to("POST v1/sempras/cidadaos/c7/casa-abrigo")[0]!.body).toMatchObject({ motivos: ["orfandade"], irmaos: [], diagnosticos: [] }));
    expect(api.to("POST v1/sempras/cidadaos")).toHaveLength(0);
    await userEvent.click(await screen.findByRole("button", { name: /Novo acolhimento/ }));
    await userEvent.click(screen.getByRole("button", { name: "Cancelar" }));
    expect(await screen.findByRole("table", { name: "Relatório de institucionalizados" })).toBeInTheDocument();
  });

  it("edição: irmãos na hora, checklist por diferença e dados salvos", async () => {
    const api = backend({
      "POST v1/sempras/casa-abrigo/a1/irmaos": { status: 201, data: { id: "i2", nome: "Irmão Novo", esta_acolhido: false } },
      "DELETE v1/sempras/casa-abrigo-irmaos/i1": { data: {} },
      "PUT v1/sempras/casa-abrigo/a1": { data: {} },
      "DELETE v1/sempras/casa-abrigo-diagnosticos/d1": { data: {} },
      "POST v1/sempras/casa-abrigo/a1/diagnosticos": { status: 201, data: {} },
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Acolhidos");
    await userEvent.click(await screen.findByRole("button", { name: "Editar Ana Souza" }));
    expect(await screen.findByText("Editar acolhimento")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Trocar" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /2\. Responsável/ }));
    expect(screen.getByLabelText("Data de nascimento")).toHaveValue("1980-01-02");
    await userEvent.click(screen.getByRole("button", { name: /3\. Acolhimento/ }));
    expect(screen.getByLabelText("Data na unidade *")).toHaveValue("2026-10-08");
    await userEvent.type(screen.getByLabelText("Nome do irmão"), "Irmão Novo");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar irmão" }));
    expect(await screen.findByText("Irmão adicionado")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Remover Irmã Salva" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-irmaos/i1")).toHaveLength(1));
    await userEvent.click(screen.getByRole("button", { name: /5\. Saúde/ }));
    expect(screen.getByLabelText("Diabetes")).toBeChecked();
    await userEvent.click(screen.getByLabelText("Diabetes"));
    await userEvent.click(screen.getByLabelText("Hepatite"));
    await userEvent.click(screen.getByRole("button", { name: /6\. Social/ }));
    await userEvent.click(screen.getByRole("button", { name: "Salvar alterações" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo/a1")).toHaveLength(1));
    expect(api.to("PUT v1/sempras/casa-abrigo/a1")[0]!.body).toMatchObject({ processo_judicial: "123/2026", motivos: ["negligencia", "outro"],
      data_entrada: "2026-10-08T00:00:00Z", data_situacao_juridica: "2026-09-02T00:00:00Z" });
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo/a1/diagnosticos")[0]!.body).toEqual({ categoria: "doenca", diagnostico: "Hepatite" }));
    expect(api.to("DELETE v1/sempras/casa-abrigo-diagnosticos/d1")).toHaveLength(1);
    expect(await screen.findByText("Cadastro atualizado")).toBeInTheDocument();
  });

  it("cadastro vindo da ficha (?novo=): a criança já vem escolhida", async () => {
    resetNavigation({}, "/", "novo=c-ficha");
    const api = backend({
      "GET v1/sempras/cidadaos/c-ficha": { data: { id: "c-ficha", nome: "Luiza", nome_social: "Lu", data_nascimento: "2018-01-01" } },
      "POST v1/sempras/cidadaos/c-ficha/casa-abrigo": { status: 201, data: { id: "a8" } },
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    expect(await screen.findByText("Lu")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /3\. Acolhimento/ }));
    await userEvent.click(screen.getByLabelText("Exploração sexual"));
    await userEvent.click(screen.getByRole("button", { name: /6\. Social/ }));
    await userEvent.click(screen.getByRole("button", { name: "Finalizar cadastro" }));
    await waitFor(() => expect(api.to("POST v1/sempras/cidadaos/c-ficha/casa-abrigo")).toHaveLength(1));
    resetNavigation({}, "/", "novo=c-ficha");
  });

  it("cadastro vindo da ficha: cancelar volta à lista", async () => {
    resetNavigation({}, "/", "novo=c-ficha");
    backend({ "GET v1/sempras/cidadaos/c-ficha": { data: { id: "c-ficha", nome: "Luiza", nome_social: "", data_nascimento: "2018-01-01" } } });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    expect(await screen.findByText("Luiza")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Cancelar" }));
    expect(await screen.findByRole("table", { name: "Relatório de institucionalizados" })).toBeInTheDocument();
    await aba("Painel");
    await aba("Acolhidos");
  });
});

describe("Casa Abrigo — P.I.A.", () => {
  beforeEach(() => resetNavigation({}, "/", "aba=pia"));

  it("escolhe o acolhido, preenche as 6 partes, edita e conclui o plano de ação", async () => {
    const api = backend({
      "PUT v1/sempras/casa-abrigo/a1/pia": { data: {} },
      "PUT v1/sempras/casa-abrigo-pia-metas/m1": { data: {} },
      "POST v1/sempras/casa-abrigo-pia-metas/m1/concluir": { data: {} },
      "DELETE v1/sempras/casa-abrigo-pia-metas/m1": { data: {} },
      "POST v1/sempras/casa-abrigo/a1/pia-metas": { status: 201, data: {} },
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    expect(await screen.findByText("Escolha o acolhido para abrir o P.I.A.")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Acolhido"), "a1");
    expect(await screen.findByText(/Última reavaliação: 07\/10\/2026/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Naturalidade"), "Rondonópolis");
    await userEvent.click(screen.getByLabelText("Número do NIS"));
    await userEvent.click(screen.getByRole("tab", { name: "Saúde e escola" }));
    await userEvent.click(screen.getByLabelText("Sinais de violência física"));
    await userEvent.selectOptions(screen.getByLabelText("Período"), "vespertino");
    await userEvent.click(screen.getByRole("tab", { name: "Situação familiar" }));
    await userEvent.selectOptions(screen.getByLabelText("Condições de moradia (anterior)"), "cedida");
    await userEvent.type(screen.getByLabelText(/Breve relato/), "conflito familiar");
    await userEvent.click(screen.getByRole("tab", { name: "Rotina e atividades" }));
    await userEvent.type(screen.getByLabelText("Observações comportamentais durante a rotina"), "dorme cedo");
    await userEvent.click(screen.getByRole("tab", { name: "Intervenção familiar" }));
    await userEvent.type(screen.getByLabelText("Objetivos da intervenção"), "fortalecer vínculos");
    const metas = within(screen.getByRole("list", { name: "Metas da intervenção" }));
    expect(metas.getByText("Concluída")).toBeInTheDocument();
    expect(metas.getByText(/até 01\/12\/2026/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Meta da intervenção"), "visitas quinzenais");
    await userEvent.click(screen.getByRole("button", { name: /Adicionar ação/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo/a1/pia-metas")[0]!.body).toMatchObject({ tipo: "intervencao", descricao: "visitas quinzenais", prazo: null }));
    await userEvent.click(screen.getByRole("button", { name: /Salvar P\.I\.A\./ }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo/a1/pia")[0]!.body).toMatchObject({
      naturalidade: "Rondonópolis", doc_nis: true, sinais_violencia_fisica: true, periodo_escolar: "vespertino", moradia_anterior: "cedida",
      observacoes_comportamentais: "dorme cedo", intervencao_objetivos: "fortalecer vínculos", escola: "Escola Municipal" }));

    await userEvent.click(screen.getByRole("tab", { name: "Plano de ação" }));
    const plano = screen.getByRole("table", { name: "Plano de ação" });
    const providencia = within(plano).getByLabelText("Providências — meta m1");
    await userEvent.type(providencia, "matricular");
    fireEvent.change(within(plano).getByLabelText("Prazo — meta m1"), { target: { value: "2026-11-15" } });
    await userEvent.click(within(plano).getByRole("button", { name: "Salvar meta m1" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo-pia-metas/m1")[0]!.body).toMatchObject({ tipo: "plano", providencia: "matricular",
      prazo: "2026-11-15T00:00:00Z", esportes: "futebol" }));
    await userEvent.click(within(plano).getByLabelText("Concluída — meta m1"));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo-pia-metas/m1/concluir")[0]!.body).toEqual({ concluida: true }));
    await userEvent.click(within(plano).getByRole("button", { name: "Remover meta m1" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-pia-metas/m1")).toHaveLength(1));
    await userEvent.type(screen.getByLabelText("Necessidade identificada"), "documentação (RG)");
    fireEvent.change(screen.getByLabelText("Prazo"), { target: { value: "2026-12-01" } });
    await userEvent.type(screen.getByLabelText("Lazer"), "parque");
    await userEvent.click(screen.getByRole("button", { name: /Adicionar ação/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo/a1/pia-metas")[1]!.body).toMatchObject({ tipo: "plano",
      descricao: "documentação (RG)", prazo: "2026-12-01T00:00:00Z", lazer: "parque" }));
  });

  it("acolhido sem P.I.A. e sem ações no plano", async () => {
    backend({ "GET v1/sempras/casa-abrigo/a1": { data: { ...DETALHE, pia: null, pia_metas: [] } } });
    resetNavigation({}, "/", "aba=pia&acolhido=a1");
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    expect(await screen.findByText(/P\.I\.A\. ainda não preenchido/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "Plano de ação" }));
    expect(screen.getByText("Nenhuma ação no plano.")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "Intervenção familiar" }));
    expect(screen.getByText("Nenhuma meta acompanhada.")).toBeInTheDocument();
  });
});

describe("Casa Abrigo — evolução, termos, visitantes e diário", () => {
  beforeEach(() => resetNavigation({}, "/", ""));

  it("evolução: registra no prontuário e lista com sigilo e busca", async () => {
    const api = backend({
      [`GET ${B}/evolucoes`]: { data: [
        { id: "e1", cidadao_id: "c-a1", unidade_id: "u4", atendimento_id: null, tipo: "social", texto: "foi à escola", sigiloso: false, autor_nome: "Téc", created_at: "2026-10-10T10:00:00Z", acolhido_nome: "Ana Souza" },
        { id: "e2", cidadao_id: "c-a2", unidade_id: "u4", atendimento_id: null, tipo: "psicologico", texto: "", sigiloso: true, autor_nome: "", created_at: "2026-10-09T10:00:00Z", acolhido_nome: "Bru" },
      ] },
      "POST v1/sempras/cidadaos/c-a2/registros": { status: 201, data: {} },
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Evolução técnica");
    const lista = within(await screen.findByRole("list", { name: "Evoluções dos acolhidos" }));
    expect(lista.getByText("foi à escola")).toBeInTheDocument();
    expect(lista.getByText("Registro sigiloso da psicologia")).toBeInTheDocument();
    expect(lista.getByText("Técnico responsável")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Buscar por nome do acolhido"), "bru");
    expect(lista.queryByText("foi à escola")).not.toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Acolhido"), "a2");
    await userEvent.selectOptions(screen.getByLabelText("Tipo"), "psicologico");
    await userEvent.type(screen.getByLabelText("Relato da evolução *"), "sessão de escuta");
    await userEvent.click(screen.getByRole("button", { name: "Salvar evolução" }));
    await waitFor(() => expect(api.to("POST v1/sempras/cidadaos/c-a2/registros")[0]!.body).toEqual({ unidade_id: "u4", tipo: "psicologico", texto: "sessão de escuta" }));
  });

  it("termos: gera o documento com assinatura, imprime e mostra o histórico com iniciais", async () => {
    const api = backend({
      [`GET ${B}/termos`]: { data: [{ id: "t1", acolhimento_id: "a1", tipo_termo: "passeio", responsavel_retirada: "", parentesco_responsavel: "",
        motivo_observacao: "", data_termo: "2026-10-06T00:00:00Z", acolhido_nome: "Ana Souza", created_at: "" }] },
      "POST v1/sempras/casa-abrigo/a1/termos": { status: 201, data: {} },
    });
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    const ctx = { beginPath: vi.fn(), moveTo: vi.fn(), lineTo: vi.fn(), stroke: vi.fn(), clearRect: vi.fn(), lineWidth: 0, lineCap: "", strokeStyle: "" };
    const getContext = vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(ctx as unknown as CanvasRenderingContext2D);
    const toDataURL = vi.spyOn(HTMLCanvasElement.prototype, "toDataURL").mockReturnValue("data:image/png;base64,AAA");
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba(/Advertência/);
    const historico = within(await screen.findByRole("table", { name: "Termos gerados" }));
    expect(historico.getByText("A. S.")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Tipo de documento"), "passeio");
    await userEvent.selectOptions(screen.getByLabelText("Acolhido"), "a1");
    await userEvent.type(screen.getByLabelText("Responsável pela retirada"), "Tia Rosa");
    await userEvent.type(screen.getByLabelText("Parentesco"), "tia");
    await userEvent.type(screen.getByLabelText("Descrição / motivo / observações"), "passeio ao parque");
    const quadro = screen.getByLabelText("Quadro de assinatura");
    fireEvent.pointerMove(quadro, { clientX: 1, clientY: 1 });
    fireEvent.pointerDown(quadro, { clientX: 5, clientY: 5 });
    fireEvent.pointerMove(quadro, { clientX: 20, clientY: 15 });
    fireEvent.pointerUp(quadro);
    fireEvent.pointerLeave(quadro);
    expect(ctx.stroke).toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /Salvar e gerar documento/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo/a1/termos")[0]!.body).toMatchObject({ tipo_termo: "passeio",
      responsavel_retirada: "Tia Rosa", parentesco_responsavel: "tia", motivo_observacao: "passeio ao parque" }));
    await waitFor(() => expect(print).toHaveBeenCalled());
    expect(screen.getByRole("heading", { name: "Termo de Liberação para Passeio" })).toBeInTheDocument();
    expect(screen.getByAltText("Assinatura do responsável")).toHaveAttribute("src", "data:image/png;base64,AAA");
    await userEvent.click(screen.getByRole("button", { name: /Limpar quadro/ }));
    expect(ctx.clearRect).toHaveBeenCalled();
    // advertência: sem retirada, sem assinatura
    await userEvent.selectOptions(screen.getByLabelText("Tipo de documento"), "advertencia");
    await userEvent.click(screen.getByRole("button", { name: /Salvar e gerar documento/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo/a1/termos")[1]!.body).toMatchObject({ tipo_termo: "advertencia",
      responsavel_retirada: "", motivo_observacao: "" }));
    expect(await screen.findByRole("heading", { name: "Termo de Advertência" })).toBeInTheDocument();
    getContext.mockReturnValue(null);
    fireEvent.pointerDown(quadro, { clientX: 5, clientY: 5 });
    fireEvent.pointerMove(quadro, { clientX: 6, clientY: 6 });
    fireEvent.pointerUp(quadro);
    await userEvent.click(screen.getByRole("button", { name: /Limpar quadro/ }));
    print.mockRestore();
    getContext.mockRestore();
    toDataURL.mockRestore();
  });

  it("visitantes: entrada (acolhido exigido só na visita a acolhido), saída e remoção", async () => {
    const api = backend({
      [`GET ${B}/visitantes`]: { data: [
        { id: "v1", unidade_id: "u4", acolhimento_id: "a1", tipo_visita: "acolhido", nome_visitante: "Avó", documento_identidade: "", parentesco_vinculo: "avó",
          data_visita: "2026-10-10T00:00:00Z", hora_entrada: "14:00", hora_saida: "", observacoes: "", acolhido_nome: "Ana Souza", created_at: "" },
        { id: "v2", unidade_id: "u4", acolhimento_id: null, tipo_visita: "doacao", nome_visitante: "Igreja", documento_identidade: "", parentesco_vinculo: "",
          data_visita: "2026-10-09T00:00:00Z", hora_entrada: "09:00", hora_saida: "10:00", observacoes: "", acolhido_nome: "", created_at: "" },
      ] },
      [`POST ${B}/visitantes`]: { status: 201, data: {} },
      "POST v1/sempras/casa-abrigo-visitantes/v1/saida": { data: {} },
      "DELETE v1/sempras/casa-abrigo-visitantes/v2": { data: {} },
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Visitantes");
    const tabela = within(await screen.findByRole("table", { name: "Visitas recentes" }));
    expect(tabela.getByText("10:00")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Nome do visitante *"), "Faculdade X");
    expect(screen.getByRole("button", { name: "Registrar entrada" })).toBeDisabled();
    await userEvent.selectOptions(screen.getByLabelText("Tipo de visita"), "voluntariado");
    expect(screen.getByLabelText("Acolhido (opcional)")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("CPF / RG"), "123");
    await userEvent.type(screen.getByLabelText("Vínculo / instituição"), "faculdade");
    await userEvent.type(screen.getByLabelText("Atividade desenvolvida"), "oficina de leitura");
    await userEvent.click(screen.getByRole("button", { name: "Registrar entrada" }));
    await waitFor(() => expect(api.to(`POST ${B}/visitantes`)[0]!.body).toMatchObject({ tipo_visita: "voluntariado", acolhimento_id: null,
      nome_visitante: "Faculdade X", documento_identidade: "123", parentesco_vinculo: "faculdade", observacoes: "oficina de leitura" }));
    await userEvent.click(tabela.getByRole("button", { name: /Registrar saída/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/casa-abrigo-visitantes/v1/saida")).toHaveLength(1));
    await userEvent.click(tabela.getByRole("button", { name: "Remover visita de Igreja" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-visitantes/v2")).toHaveLength(1));
  });

  it("diário: calendário, anotação geral e de acolhido, troca de mês e busca", async () => {
    const hoje = new Date();
    const mes = `${hoje.getFullYear()}-${String(hoje.getMonth() + 1).padStart(2, "0")}`;
    const api = backend({
      [`GET ${B}/diario`]: { data: [
        { id: "n1", unidade_id: "u4", acolhimento_id: "a1", data_referencia: `${mes}-02T00:00:00Z`, texto: "consulta médica", autor_nome: "Ana",
          acolhido_nome: "Ana Souza", created_at: "2026-10-01T10:00:00Z" },
        { id: "n2", unidade_id: "u4", acolhimento_id: null, data_referencia: `${mes}-03T00:00:00Z`, texto: "rotina da casa", autor_nome: "Ana", created_at: "2026-10-01T10:00:00Z" },
      ] },
      [`POST ${B}/diario`]: { status: 201, data: {} },
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await aba("Diário de bordo");
    const lista = within(await screen.findByRole("list", { name: "Anotações do diário" }));
    expect(lista.getByText(/Geral \(rotina da casa\)/)).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /\(com anotação\)/ })).toHaveLength(2);
    await userEvent.type(screen.getByLabelText("Buscar por texto, acolhido ou data"), "consulta");
    expect(lista.queryByText("rotina da casa")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: new RegExp(`^05/`) }));
    await userEvent.selectOptions(screen.getByLabelText("Referente a qual acolhido?"), "a1");
    await userEvent.type(screen.getByLabelText("Anotação / ocorrência *"), "reunião com a família");
    await userEvent.click(screen.getByRole("button", { name: "Salvar no diário" }));
    await waitFor(() => expect(api.to(`POST ${B}/diario`)[0]!.body).toMatchObject({ acolhimento_id: "a1", data_referencia: `${mes}-05T00:00:00Z` }));
    await userEvent.click(screen.getByRole("button", { name: "Próximo mês" }));
    await waitFor(() => expect(api.to(`GET ${B}/diario`).some((c) => c.query.get("mes") !== mes)).toBe(true));
    await userEvent.click(screen.getByRole("button", { name: "Mês anterior" }));
    await userEvent.click(screen.getByRole("button", { name: "Mês anterior" }));
    // anotação para um dia de outro mês: a lista passa a esse mês
    await userEvent.click(screen.getByRole("button", { name: /^01\// }));
    await userEvent.click(screen.getByRole("button", { name: "Próximo mês" }));
    await userEvent.type(screen.getByLabelText("Anotação / ocorrência *"), "agenda");
    await userEvent.click(screen.getByRole("button", { name: "Salvar no diário" }));
    await waitFor(() => expect(api.to(`POST ${B}/diario`)).toHaveLength(2));
  });
});

describe("Casa Abrigo — relatórios e tarefas", () => {
  beforeEach(() => resetNavigation({}, "/", "aba=relatorios"));
  afterEach(() => vi.unstubAllGlobals());

  const EST = { de: "2026-10-01", ate: "2026-10-31", entradas: 2, saidas: 1, reintegracoes: 1, adocoes: 0, transferencias: 0,
    saidas_por_tipo: { reintegracao_familia_natural: 1 }, motivos: { negligencia: 2, abandono: 1 }, ativos_no_fim: 3, criancas: 2,
    adolescentes: 1, meninos: 1, meninas: 2 };

  it("estatístico: período, barras, impressão e CSV; sem movimento", async () => {
    const api = backend({ [`GET ${B}/estatisticas`]: (req: { query: URLSearchParams }) =>
      ({ data: req.query.get("de")?.endsWith("-01-01") ? { ...EST, entradas: 0, saidas: 0, motivos: {}, saidas_por_tipo: {} } : EST }) });
    const criar = vi.fn(() => "blob:x");
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: criar, revokeObjectURL: vi.fn() }));
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    const motivos = within(await screen.findByRole("region", { name: "Motivos de acolhimento" }));
    expect(motivos.getByText("2 (67%)")).toBeInTheDocument();
    expect(screen.getByText(/Reintegração à família natural/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Exportar CSV/ }));
    expect(criar).toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /Imprimir/ }));
    expect(print).toHaveBeenCalled();
    await userEvent.selectOptions(screen.getByLabelText("Período"), "trimestral");
    expect(await screen.findByText(/trimestre/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Mês de referência"), { target: { value: "" } });
    fireEvent.change(screen.getByLabelText("Mês de referência"), { target: { value: "2026-02" } });
    await userEvent.selectOptions(screen.getByLabelText("Período"), "anual");
    expect(await screen.findByText("Nenhuma entrada no período.")).toBeInTheDocument();
    expect(screen.getByText("Nenhuma saída no período.")).toBeInTheDocument();
    await waitFor(() => expect(api.to(`GET ${B}/estatisticas`).at(-1)!.query.get("ate")).toBe("2026-12-31"));
    print.mockRestore();
  });

  it("relatórios escritos: registra, escreve, exporta, muda a etapa e exclui; o entregue só se lê", async () => {
    const rel = (id: string, extra: Record<string, unknown> = {}) => ({ id, unidade_id: "u4", acolhimento_id: null, tipo: "mensal", tipo_outro: "",
      status: "em_elaboracao", data_entrega: "2020-01-01T00:00:00Z", conteudo: "", created_at: "", atualizado_em: "", ...extra });
    const api = backend({
      [`GET ${B}/relatorios`]: { data: [rel("r1"), rel("r2", { tipo: "outro", tipo_outro: "Relatório ao MP", status: "entregue", conteudo: "texto entregue",
        acolhido_nome: "Ana Souza", data_entrega: "2099-01-01T00:00:00Z" })] },
      [`POST ${B}/relatorios`]: { status: 201, data: {} },
      "PUT v1/sempras/casa-abrigo-relatorios/r1": { data: {} },
      "DELETE v1/sempras/casa-abrigo-relatorios/r1": { data: {} },
    });
    const criar = vi.fn(() => "blob:x");
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: criar, revokeObjectURL: vi.fn() }));
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    await userEvent.click(await screen.findByRole("tab", { name: "Relatórios escritos" }));
    const lista = within(await screen.findByRole("list", { name: "Relatórios escritos" }));
    expect(lista.getByText(/Atrasado/)).toBeInTheDocument();
    expect(lista.getByText("Sem conteúdo")).toBeInTheDocument();
    expect(lista.getByText(/Escrito \(14 caracteres\)/)).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Tipo do relatório"), "outro");
    await userEvent.type(screen.getByLabelText("Nome do relatório *"), "Relatório à Vara");
    await userEvent.selectOptions(screen.getByLabelText("Etapa atual"), "revisao");
    fireEvent.change(screen.getByLabelText("Data de entrega *"), { target: { value: "2026-11-10" } });
    await userEvent.selectOptions(screen.getByLabelText("Acolhido (opcional)"), "a1");
    await userEvent.click(screen.getByRole("button", { name: "Registrar" }));
    await waitFor(() => expect(api.to(`POST ${B}/relatorios`)[0]!.body).toMatchObject({ tipo: "outro", tipo_outro: "Relatório à Vara",
      status: "revisao", data_entrega: "2026-11-10T00:00:00Z", acolhimento_id: "a1" }));

    await userEvent.click(lista.getAllByRole("button", { name: /Escrever/ })[0]!);
    const editor = await screen.findByRole("dialog", { name: "Relatório mensal" });
    await userEvent.type(within(editor).getByLabelText("Conteúdo do relatório"), "A criança está bem.");
    expect(within(editor).getByText("19 caracteres")).toBeInTheDocument();
    await userEvent.click(within(editor).getByRole("button", { name: /Exportar \.TXT/ }));
    await userEvent.click(within(editor).getByRole("button", { name: /Exportar \.DOC/ }));
    await userEvent.click(within(editor).getByRole("button", { name: /Imprimir/ }));
    expect(criar).toHaveBeenCalledTimes(2);
    expect(print).toHaveBeenCalled();
    await userEvent.click(within(editor).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo-relatorios/r1")[0]!.body).toMatchObject({ conteudo: "A criança está bem." }));

    await userEvent.click(lista.getByRole("button", { name: "Editar etapa e prazo de Relatório mensal" }));
    const info = await screen.findByRole("dialog", { name: "Editar etapa e prazo" });
    await userEvent.selectOptions(within(info).getByLabelText("Etapa do relatório"), "entregue");
    expect(within(info).getByText(/não muda nem pode ser excluído/)).toBeInTheDocument();
    fireEvent.change(within(info).getByLabelText("Data de entrega"), { target: { value: "2026-10-20" } });
    await userEvent.click(within(info).getByRole("button", { name: "Salvar alterações" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo-relatorios/r1")[1]!.body).toMatchObject({ status: "entregue",
      data_entrega: "2026-10-20T00:00:00Z" }));

    await userEvent.click(lista.getByRole("button", { name: "Excluir Relatório mensal" }));
    const confirmar = await screen.findByRole("dialog", { name: "Confirmar exclusão" });
    await userEvent.click(within(confirmar).getByRole("button", { name: "Excluir relatório" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-relatorios/r1")).toHaveLength(1));

    // o entregue: só leitura, sem editar nem excluir
    expect(lista.queryByRole("button", { name: "Excluir Relatório ao MP" })).not.toBeInTheDocument();
    await userEvent.click(lista.getByRole("button", { name: /Ler/ }));
    const leitura = await screen.findByRole("dialog", { name: "Relatório ao MP" });
    expect(within(leitura).getByLabelText("Conteúdo do relatório")).toHaveAttribute("readonly");
    expect(within(leitura).queryByRole("button", { name: "Salvar" })).not.toBeInTheDocument();
    print.mockRestore();
  });

  it("Kanban: cria, edita, move por botão e por arraste, filtra e exclui", async () => {
    resetNavigation({}, "/", "aba=tarefas");
    const tarefa = (id: string, extra: Record<string, unknown> = {}) => ({ id, unidade_id: "u4", titulo: `Tarefa ${id}`, descricao: "", status: "todo",
      prioridade: "media", responsavel: "", sprint: "", prazo: null, ordem: 0, created_at: "", atualizado_em: "", ...extra });
    const api = backend({
      [`GET ${B}/tarefas`]: { data: [tarefa("t1", { descricao: "comprar lápis", responsavel: "Ana", sprint: "Sprint 1", prazo: "2020-01-01T00:00:00Z" }),
        tarefa("t2", { status: "backlog", prioridade: "critica" }), tarefa("t3", { status: "done", prazo: "2020-01-01T00:00:00Z" })] },
      [`POST ${B}/tarefas`]: { status: 201, data: {} },
      "PUT v1/sempras/casa-abrigo-tarefas/t1": { data: {} },
      "PUT v1/sempras/casa-abrigo-tarefas/t2": { data: {} },
      "DELETE v1/sempras/casa-abrigo-tarefas/t3": { data: {} },
    });
    renderApp(<CasaAbrigoWorkspace unidadeId="u4" />);
    const afazer = within(await screen.findByRole("region", { name: "A fazer" }));
    expect(afazer.getByText(/prazo 01\/01\/2020/)).toHaveClass("text-danger");

    await userEvent.click(screen.getByRole("button", { name: /Nova tarefa/ }));
    const nova = await screen.findByRole("dialog", { name: "Nova tarefa" });
    await userEvent.type(within(nova).getByLabelText("Título *"), "Organizar doações");
    await userEvent.type(within(nova).getByLabelText("Descrição"), "roupas de inverno");
    await userEvent.selectOptions(within(nova).getByLabelText("Prioridade"), "alta");
    await userEvent.selectOptions(within(nova).getByLabelText("Coluna"), "todo");
    await userEvent.type(within(nova).getByLabelText("Responsável"), "Ana");
    await userEvent.type(within(nova).getByLabelText("Sprint"), "Sprint 2");
    fireEvent.change(within(nova).getByLabelText("Prazo"), { target: { value: "2026-10-30" } });
    await userEvent.click(within(nova).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to(`POST ${B}/tarefas`)[0]!.body).toMatchObject({ titulo: "Organizar doações", prioridade: "alta",
      status: "todo", prazo: "2026-10-30T00:00:00Z", ordem: 0 }));

    await userEvent.click(afazer.getByRole("button", { name: "Editar Tarefa t1" }));
    const editar = await screen.findByRole("dialog", { name: "Editar tarefa" });
    expect(within(editar).getByLabelText("Prazo")).toHaveValue("2020-01-01");
    await userEvent.clear(within(editar).getByLabelText("Título *"));
    await userEvent.type(within(editar).getByLabelText("Título *"), "Tarefa um");
    fireEvent.change(within(editar).getByLabelText("Prazo"), { target: { value: "" } });
    await userEvent.click(within(editar).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo-tarefas/t1")[0]!.body).toMatchObject({ titulo: "Tarefa um", prazo: null }));

    await userEvent.click(afazer.getByRole("button", { name: "Mover Tarefa t1 para Em andamento" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo-tarefas/t1")[1]!.body).toMatchObject({ status: "doing" }));
    await userEvent.click(afazer.getByRole("button", { name: "Mover Tarefa t1 para Backlog" }));
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo-tarefas/t1")[2]!.body).toMatchObject({ status: "backlog" }));

    // arrastar t2 do backlog para "Em revisão"; soltar na mesma coluna não muda nada
    const dados = new Map<string, string>();
    const dataTransfer = { setData: (k: string, v: string) => dados.set(k, v), getData: (k: string) => dados.get(k) ?? "" };
    const cartao = screen.getByText("Tarefa t2").closest("article")!;
    fireEvent.dragStart(cartao, { dataTransfer });
    fireEvent.dragOver(screen.getByRole("region", { name: "Em revisão" }), { dataTransfer });
    fireEvent.drop(screen.getByRole("region", { name: "Em revisão" }), { dataTransfer });
    await waitFor(() => expect(api.to("PUT v1/sempras/casa-abrigo-tarefas/t2")[0]!.body).toMatchObject({ status: "review" }));
    fireEvent.drop(screen.getByRole("region", { name: "Backlog" }), { dataTransfer });
    fireEvent.drop(screen.getByRole("region", { name: "Backlog" }), { dataTransfer: { getData: () => "nenhuma" } });
    expect(api.to("PUT v1/sempras/casa-abrigo-tarefas/t2")).toHaveLength(1);

    await userEvent.type(screen.getByLabelText("Filtrar por sprint"), "sprint 1");
    expect(screen.queryByText("Tarefa t2")).not.toBeInTheDocument();
    await userEvent.clear(screen.getByLabelText("Filtrar por sprint"));
    await userEvent.selectOptions(screen.getByLabelText("Filtrar por prioridade"), "critica");
    expect(screen.queryByText("Tarefa t1")).not.toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Filtrar por prioridade"), "");
    await userEvent.click(screen.getByRole("button", { name: "Excluir Tarefa t3" }));
    await waitFor(() => expect(api.to("DELETE v1/sempras/casa-abrigo-tarefas/t3")).toHaveLength(1));
    await userEvent.click(screen.getByRole("button", { name: /Nova tarefa/ }));
    await userEvent.keyboard("{Escape}");
  });
});
