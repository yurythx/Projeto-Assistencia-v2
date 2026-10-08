import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { porFuncao, resumoProcedimentos } from "@/components/atlas/secretaria";
import type { Workflow } from "@/lib/nexus/types";
import { identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import AtlasPage from "../../page";

import CadernoPage from "./caderno/page";
import SecretariaPage from "./page";

const ORGAO = {
  prefixo: "2.0",
  nome: "Administração",
  edicao_diario: "",
  data_publicacao: null,
  versao: "",
};
const funcao = (codigo: string, nome: string) => ({
  codigo: "",
  descritor: "",
  subfuncao: {
    codigo: `${codigo}.00`,
    nome: "Sub",
    recomendacao: "",
    funcao: { codigo, nome, orgao: ORGAO },
  },
});

const wf = (id: string, codigo: string, extra: Partial<Workflow> = {}) =>
  ({
    id,
    codigo_processual: codigo,
    titulo: `Procedimento ${codigo}`,
    objetivo: "Objetivo",
    publico_alvo: "Servidores",
    versao: 1,
    ativo: false,
    situacao: "RASCUNHO",
    nivel_acesso: "PUBLICO",
    hipotese_legal_restricao: "",
    codigo_ttdd: "2.0.06.03.01",
    total_etapas: 1,
    etapas: [],
    created_at: "",
    updated_at: "",
    ...extra,
  }) as Workflow;

const FERIAS = wf("wf-1", "RH.FER.001", {
  classificacao: funcao("2.0.06", "Gestão de Pessoas") as never,
});
const COMPRAS = wf("wf-2", "ADM.CMP.004", {
  situacao: "HOMOLOGADO",
  ativo: true,
  codigo_ttdd: "2.0.02.00.10",
  classificacao: funcao("2.0.02", "Gestão de Compras") as never,
});
const SEM_SERIE = wf("wf-3", "ADM.OUT.001");

const ADM = { prefixo: "2.0", nome: "Administração", publicados: 1, em_validacao: 1, rascunhos: 2 };
const FAZ = { prefixo: "3.0", nome: "Fazenda", publicados: 0, em_validacao: 0, rascunhos: 4 };
const CI = {
  prefixo: "4.0",
  nome: "Controle Interno",
  publicados: 0,
  em_validacao: 0,
  rascunhos: 0,
};
const RESUMO = [ADM, FAZ, CI];
const PUBLICO = RESUMO.map((o) => ({ ...o, em_validacao: 0, rascunhos: 0 }));

const detalhe = (w: Workflow) => ({
  ...w,
  etapas: [
    {
      id: `${w.id}-e1`,
      ordem: 1,
      unidade_administrativa: "SEGEP/RH",
      nome_setor: "Recursos Humanos",
      atribuicoes_setor: "Instruir",
      prazo_sla_em_dias: 5,
      documentos: [],
      transicoes: [],
    },
  ],
});

const ESTRUTURA = [
  {
    ...ORGAO,
    total: 3,
    funcoes: [
      { codigo: "2.0.02", nome: "Gestão de Compras", total: 1, subfuncoes: [] },
      { codigo: "2.0.06", nome: "Gestão de Pessoas", total: 2, subfuncoes: [] },
    ],
  },
];

describe("helpers da secretaria", () => {
  it("resumo: rascunhos e em validação só para a gestão; agrupamento pela função", () => {
    expect(resumoProcedimentos(ADM, true)).toBe("1 publicado · 1 em validação · 2 rascunhos");
    expect(resumoProcedimentos({ ...FAZ, rascunhos: 1 }, true)).toBe("0 publicados · 1 rascunho");
    expect(resumoProcedimentos(ADM, false)).toBe("1 publicado");
    expect(porFuncao([FERIAS, SEM_SERIE, COMPRAS]).map((g) => g.nome)).toEqual([
      "Gestão de Compras",
      "Gestão de Pessoas",
      "Outros",
    ]);
  });
});

describe("Atlas — procedimentos por secretaria", () => {
  beforeEach(() => resetNavigation({ prefixo: "2.0" }, "/atlas/secretarias/2.0"));

  it("início: o público vê só as secretarias com procedimento publicado", async () => {
    mockBackend({
      ...identityRoutes({ permissions: [] }),
      "GET v1/atlas/workflows": page([COMPRAS]),
      "GET v1/atlas/workflows/secretarias": { data: PUBLICO },
      "GET v1/atlas/ttdd/estrutura": { data: [] },
    });
    resetNavigation({}, "/atlas");
    renderApp(<AtlasPage />);
    const secao = await screen.findByRole("region", { name: "Procedimentos por secretaria" });
    const links = within(secao).getAllByRole("link");
    expect(links).toHaveLength(1);
    expect(links.at(0)).toHaveAttribute("href", "/atlas/secretarias/2.0");
    expect(links.at(0)).toHaveTextContent("1 publicado");
  });

  it("início: a gestão vê também as secretarias só com rascunhos", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows": page([FERIAS]),
      "GET v1/atlas/admin/workflows/secretarias": { data: RESUMO },
      "GET v1/atlas/ttdd/estrutura": { data: [] },
    });
    resetNavigation({}, "/atlas");
    renderApp(<AtlasPage />);
    const secao = await screen.findByRole("region", { name: "Procedimentos por secretaria" });
    expect(within(secao).getAllByRole("link")).toHaveLength(2);
    expect(within(secao).getByRole("link", { name: /Fazenda/ })).toHaveTextContent("4 rascunhos");
  });

  it("gestão: última versão de cada código, agrupada por função, filtro, caderno e lacunas", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows/secretarias": { data: RESUMO },
      "GET v1/atlas/admin/workflows": page([FERIAS, COMPRAS]),
      "GET v1/atlas/admin/cobertura": {
        data: {
          lacunas: [
            {
              codigo: "2.0.04.00",
              nome: "Pesquisa",
              orgao: "Administração",
              series: 1,
              exemplos: ["Projeto PAPIRO"],
            },
            {
              codigo: "12.0.08.00",
              nome: "Hospitais",
              orgao: "Saúde",
              series: 1,
              exemplos: ["Internação"],
            },
          ],
        },
      },
    });
    renderApp(<SecretariaPage />);

    expect(
      await screen.findByRole("heading", { level: 1, name: "Administração" }),
    ).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: /Gestão de Compras/ })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /Gestão de Pessoas/ })).toBeInTheDocument();
    expect(
      screen.getByText("Procedimentos: 1 publicado · 1 em validação · 2 rascunhos."),
    ).toBeInTheDocument();
    const q = api.to("GET v1/atlas/admin/workflows").at(0)?.query;
    expect([q?.get("prefixo_ttdd"), q?.get("ultima"), q?.get("situacao")]).toEqual([
      "2.0",
      "true",
      null,
    ]);

    expect(screen.getByRole("link", { name: /Caderno da entrevista/ })).toHaveAttribute(
      "href",
      "/atlas/secretarias/2.0/caderno",
    );
    expect(screen.getByRole("link", { name: /Temporalidade da secretaria/ })).toHaveAttribute(
      "href",
      "/atlas/ttdd?codigo=2.0",
    );
    const lacunas = await screen.findByRole("list", { name: "Subfunções sem procedimento" });
    expect(within(lacunas).getAllByRole("listitem")).toHaveLength(1);
    expect(lacunas).toHaveTextContent("Projeto PAPIRO");

    await userEvent.selectOptions(screen.getByLabelText("Situação"), "RASCUNHO");
    expect(router.replace).toHaveBeenCalledWith("/atlas/secretarias/2.0?situacao=RASCUNHO");
  });

  it("filtro por função: lista, lacunas e caderno só do departamento", async () => {
    resetNavigation(
      { prefixo: "2.0" },
      "/atlas/secretarias/2.0",
      "funcao=2.0.06&situacao=RASCUNHO",
    );
    const api = mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/ttdd/estrutura": { data: ESTRUTURA },
      "GET v1/atlas/admin/workflows/secretarias": { data: RESUMO },
      "GET v1/atlas/admin/workflows": page([FERIAS]),
      "GET v1/atlas/admin/cobertura": {
        data: {
          lacunas: [
            {
              codigo: "2.0.06.04",
              nome: "INSS",
              orgao: "Administração",
              series: 1,
              exemplos: ["Tempo de contribuição"],
            },
            {
              codigo: "2.0.04.00",
              nome: "Pesquisa",
              orgao: "Administração",
              series: 1,
              exemplos: ["Projeto PAPIRO"],
            },
          ],
        },
      },
    });
    renderApp(<SecretariaPage />);

    const funcao = await screen.findByLabelText("Função (departamento)");
    expect(funcao).toHaveValue("2.0.06");
    expect(screen.getByText(/Secretaria · TTDD 2.0 · Gestão de Pessoas/)).toBeInTheDocument();
    const q = api.to("GET v1/atlas/admin/workflows").at(-1)?.query;
    expect([q?.get("prefixo_ttdd"), q?.get("situacao")]).toEqual(["2.0.06", "RASCUNHO"]);
    const lacunas = await screen.findByRole("list", { name: "Subfunções sem procedimento" });
    expect(lacunas).toHaveTextContent("Tempo de contribuição");
    expect(lacunas).not.toHaveTextContent("PAPIRO");
    expect(await screen.findByRole("link", { name: /Caderno da entrevista/ })).toHaveAttribute(
      "href",
      "/atlas/secretarias/2.0/caderno?situacao=RASCUNHO&funcao=2.0.06",
    );

    // Trocar a função mantém a situação; "Todas" tira o filtro.
    await userEvent.selectOptions(funcao, "2.0.02");
    expect(router.replace).toHaveBeenLastCalledWith(
      "/atlas/secretarias/2.0?situacao=RASCUNHO&funcao=2.0.02",
    );
    await userEvent.selectOptions(funcao, "");
    expect(router.replace).toHaveBeenLastCalledWith("/atlas/secretarias/2.0?situacao=RASCUNHO");
  });

  it("função de outra secretaria na URL é ignorada; o público também filtra por função", async () => {
    resetNavigation({ prefixo: "2.0" }, "/atlas/secretarias/2.0", "funcao=3.0.01");
    const api = mockBackend({
      ...identityRoutes({ permissions: [] }),
      "GET v1/atlas/ttdd/estrutura": { data: ESTRUTURA },
      "GET v1/atlas/workflows/secretarias": { data: PUBLICO },
      "GET v1/atlas/workflows": page([COMPRAS]),
    });
    renderApp(<SecretariaPage />);
    expect(await screen.findByLabelText("Função (departamento)")).toHaveValue("");
    expect(api.to("GET v1/atlas/workflows").at(-1)?.query.get("prefixo_ttdd")).toBe("2.0");
  });

  it("gestão: filtro na URL, nada encontrado e aviso de lista parcial", async () => {
    resetNavigation({ prefixo: "9.0" }, "/atlas/secretarias/9.0", "situacao=EM_VALIDACAO");
    const api = mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows/secretarias": { data: RESUMO },
      "GET v1/atlas/admin/workflows": {
        data: [],
        meta: { page: 1, page_size: 100, total_items: 0, total_pages: 0 },
      },
      "GET v1/atlas/admin/cobertura": { data: { lacunas: [] } },
    });
    renderApp(<SecretariaPage />);
    expect(await screen.findByText("Nenhum procedimento com estes filtros")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Órgão 9.0" })).toBeInTheDocument();
    expect(api.to("GET v1/atlas/admin/workflows").at(0)?.query.get("situacao")).toBe(
      "EM_VALIDACAO",
    );
    expect(screen.queryByRole("link", { name: /Caderno/ })).not.toBeInTheDocument();
  });

  it("gestão: mais procedimentos que a página mostra", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows/secretarias": { data: RESUMO },
      "GET v1/atlas/admin/workflows": {
        data: [FERIAS],
        meta: { page: 1, page_size: 100, total_items: 150, total_pages: 2 },
      },
      "GET v1/atlas/admin/cobertura": { data: { lacunas: [] } },
    });
    renderApp(<SecretariaPage />);
    expect(await screen.findByText(/Mostrando 1 de 150/)).toBeInTheDocument();
  });

  it("público: só os publicados, sem filtro, caderno nem lacunas", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: [] }),
      "GET v1/atlas/workflows/secretarias": { data: PUBLICO },
      "GET v1/atlas/workflows": page([]),
    });
    renderApp(<SecretariaPage />);
    expect(await screen.findByText("Nenhum procedimento nesta secretaria")).toBeInTheDocument();
    expect(screen.getByText("Procedimentos: 1 publicado.")).toBeInTheDocument();
    expect(api.to("GET v1/atlas/workflows").at(0)?.query.get("prefixo_ttdd")).toBe("2.0");
    expect(screen.queryByLabelText("Situação")).not.toBeInTheDocument();
    expect(api.to("GET v1/atlas/admin/cobertura")).toHaveLength(0);
  });

  it("caderno: capa, sumário por função e uma ficha por procedimento", async () => {
    resetNavigation(
      { prefixo: "2.0" },
      "/atlas/secretarias/2.0/caderno",
      "situacao=RASCUNHO&funcao=2.0.06",
    );
    const print = vi.fn();
    vi.stubGlobal("print", print);
    mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows/secretarias": { data: RESUMO },
      "GET v1/atlas/admin/workflows": page([FERIAS, COMPRAS]),
      "GET v1/atlas/ttdd/estrutura": { data: ESTRUTURA },
      "GET v1/atlas/admin/workflows/wf-1": { data: detalhe(FERIAS) },
      "GET v1/atlas/admin/workflows/wf-2": { data: detalhe(COMPRAS) },
    });
    renderApp(<CadernoPage />);

    expect(
      await screen.findByRole("heading", { level: 1, name: "Administração" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/2 procedimentos · situação: rascunho/)).toBeInTheDocument();
    expect(await screen.findByText("Função 2.0.06 — Gestão de Pessoas")).toBeInTheDocument();
    const sumario = screen.getByRole("region", { name: "Sumário" });
    expect(sumario).toHaveTextContent("Gestão de Compras");
    expect(
      await screen.findByRole("heading", {
        level: 2,
        name: "RH.FER.001 — Procedimento RH.FER.001",
      }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", {
        level: 2,
        name: "ADM.CMP.004 — Procedimento ADM.CMP.004",
      }),
    ).toBeInTheDocument();
    // Ids únicos por ficha: cada etapa 1 tem o seu título.
    expect(screen.getAllByRole("region", { name: "2. Etapa 1: Recursos Humanos" })).toHaveLength(2);
    expect(screen.getByRole("link", { name: /Voltar à secretaria/ })).toHaveAttribute(
      "href",
      "/atlas/secretarias/2.0?situacao=RASCUNHO&funcao=2.0.06",
    );
    await userEvent.click(screen.getByRole("button", { name: /Imprimir caderno/ }));
    expect(print).toHaveBeenCalled();
  });

  it("caderno: vazio e restrito à gestão", async () => {
    resetNavigation({ prefixo: "3.0" }, "/atlas/secretarias/3.0/caderno");
    mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows/secretarias": { data: RESUMO },
      "GET v1/atlas/admin/workflows": page([]),
    });
    const { unmount } = renderApp(<CadernoPage />);
    expect(await screen.findByText("Nenhum procedimento para o caderno")).toBeInTheDocument();
    unmount();

    mockBackend({ ...identityRoutes({ permissions: [] }) });
    renderApp(<CadernoPage />);
    expect(await screen.findByText(/caderno da entrevista é da gestão/)).toBeInTheDocument();
  });
});
