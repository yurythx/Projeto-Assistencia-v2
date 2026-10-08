import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import FichaPage from "./page";

const WF = {
  id: "wf-1",
  codigo_processual: "RH.FER.001",
  titulo: "Férias do servidor",
  objetivo: "Conceder férias",
  publico_alvo: "Servidores",
  versao: 1,
  ativo: false,
  situacao: "RASCUNHO",
  nivel_acesso: "RESTRITO",
  hipotese_legal_restricao: "LAI, art. 31",
  codigo_ttdd: "2.0.06.03.01",
  classificacao: { descritor: "Requerimento de gozo de férias" },
  total_etapas: 2,
  etapas: [
    {
      id: "e1",
      ordem: 1,
      unidade_administrativa: "SECRETARIA",
      nome_setor: "Chefia imediata",
      atribuicoes_setor: "Aprovar o período",
      prazo_sla_em_dias: 1,
      documentos: [
        {
          id: "d1",
          nome_documento: "Requerimento de férias",
          obrigatorio: true,
          formato: "NATO_DIGITAL",
          tipo_assinatura: "INDIVIDUAL",
          modelo_id: null,
        },
        {
          id: "d2",
          nome_documento: "Aviso de férias",
          obrigatorio: false,
          formato: "EXTERNO_DIGITALIZADO",
          tipo_assinatura: "EM_BLOCO",
          modelo_id: "m1",
          modelo: { id: "m1", nome: "Aviso padrão", versao: 1, arquivo_nome: "aviso.docx" },
        },
      ],
      transicoes: [
        {
          id: "t1",
          destino_ordem: 2,
          condicao_transicao: "Período aprovado",
          is_devolucao_diligencia: false,
          descricao_diligencia: "",
        },
      ],
    },
    {
      id: "e2",
      ordem: 2,
      unidade_administrativa: "SEGEP/RH",
      nome_setor: "Recursos Humanos",
      atribuicoes_setor: "Lançar as férias",
      prazo_sla_em_dias: 10,
      documentos: [],
      transicoes: [
        {
          id: "t2",
          destino_ordem: 1,
          condicao_transicao: "Período inválido",
          is_devolucao_diligencia: true,
          descricao_diligencia: "Ajustar o período",
        },
      ],
    },
  ],
  created_at: "",
  updated_at: "",
};

describe("ficha de validação", () => {
  beforeEach(() => resetNavigation({ id: "wf-1" }, "/atlas/procedimentos/wf-1/ficha"));

  it("mostra o fluxo proposto item a item, com espaço para corrigir e padronizar, e imprime", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows/wf-1": { data: WF },
    });
    const print = vi.fn();
    vi.stubGlobal("print", print);
    renderApp(<FichaPage />);
    expect(
      await screen.findByRole("heading", { level: 1, name: "RH.FER.001 — Férias do servidor" }),
    ).toBeInTheDocument();
    expect(api.to("GET v1/atlas/admin/workflows/wf-1")).toHaveLength(1);
    expect(
      screen.getByText(
        /situação: rascunho · série TTDD 2\.0\.06\.03\.01 \(Requerimento de gozo de férias\)/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/Restrito \(LAI, art\. 31\)/)).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "2. Etapa 1: Chefia imediata" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/1 dia/)).toBeInTheDocument();
    expect(screen.getByText(/Período aprovado → etapa 2/)).toBeInTheDocument();
    expect(
      screen.getByText(/Período inválido → etapa 1 \(Ajustar o período\)/),
    ).toBeInTheDocument();
    const docs = screen.getByRole("table", { name: "Documentos da etapa 1" });
    expect(within(docs).getByText("sim (Aviso padrão)")).toBeInTheDocument();
    expect(within(docs).getByText("☐ sim ☐ não")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "4. Fechamento" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Imprimir/ }));
    expect(print).toHaveBeenCalled();
    vi.unstubAllGlobals();
  });

  it("sem gestão usa a rota pública", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: [] }),
      "GET v1/atlas/workflows/wf-1": {
        data: {
          ...WF,
          situacao: undefined,
          ativo: true,
          hipotese_legal_restricao: "",
          classificacao: undefined,
          etapas: [],
        },
      },
    });
    renderApp(<FichaPage />);
    expect(
      await screen.findByText(/situação: homologado · série TTDD 2\.0\.06\.03\.01$/),
    ).toBeInTheDocument();
    expect(api.to("GET v1/atlas/workflows/wf-1")).toHaveLength(1);
  });
});
