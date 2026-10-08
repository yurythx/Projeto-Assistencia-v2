import { render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("@/lib/env", async (orig) => ({
  ...(await orig<typeof import("@/lib/env")>()),
  APP_URL: "https://portal.gov.br",
}));
vi.mock("@/components/layout/PublicShell", () => ({
  PublicShell: ({ children }: { children: ReactNode }) => <main>{children}</main>,
}));

class NotFound extends Error {}
vi.mock("next/navigation", () => ({
  notFound: () => {
    throw new NotFound("notFound");
  },
}));

const api = vi.hoisted(() => ({
  routes: {} as Record<string, unknown>,
  metas: {} as Record<string, unknown>,
}));
vi.mock("@/lib/api/publicServer", async () => {
  class PublicApiError extends Error {
    constructor(
      readonly status: number,
      readonly code: string,
      message: string,
    ) {
      super(message);
    }
  }
  // A rota mais longa que é prefixo do caminho vence.
  const key = (path: string) =>
    Object.keys(api.routes)
      .filter((k) => path === k || path.startsWith(k + "?") || path.startsWith(k))
      .sort((a, b) => b.length - a.length)[0];
  const lookup = (path: string) => {
    const k = key(path);
    const v = k ? api.routes[k] : new PublicApiError(404, "NOT_FOUND", "x");
    if (v instanceof Error) throw v;
    return { data: v, meta: k ? api.metas[k] : undefined };
  };
  return {
    PublicApiError,
    publicGet: async (path: string) => lookup(path),
    publicGetOr: async (path: string, fallback: unknown) => {
      try {
        return lookup(path).data;
      } catch {
        return fallback;
      }
    },
  };
});

import { PublicApiError } from "@/lib/api/publicServer";

import ProcedimentosPage from "../procedimentos/page";
import ProcedimentoPage, { generateMetadata as metaProcedimento } from "../procedimentos/[id]/page";
import TemporalidadePage from "../temporalidade/page";
import SeriePage, { generateMetadata as metaSerie } from "../temporalidade/[codigo]/page";

const ORGAO = {
  prefixo: "2.0",
  nome: "Administração",
  edicao_diario: "6.017",
  data_publicacao: "2025-08-25T00:00:00Z",
  versao: "II",
};
const SERIE = {
  codigo: "2.0.02.00.07",
  descritor: "Processos de pregão",
  fase_corrente_anos: 1,
  fase_corrente_condicao: "",
  fase_interm_anos: 4,
  fase_interm_condicao: "",
  destinacao_final: "GUARDA_PERMANENTE",
  observacoes: "Processo integral",
  revogada_em: null,
  revogada_edicao: "",
  subfuncao: {
    codigo: "2.0.02.00",
    nome: "Compras",
    recomendacao: "Transferir após o TCE.",
    funcao: { codigo: "2.0.02", nome: "Gestão de compras", orgao: ORGAO },
  },
  created_at: "",
};
const WF = {
  id: "wf-1",
  codigo_processual: "ADM.LIC.001",
  titulo: "Pregão Eletrônico",
  objetivo: "Comprar bens comuns",
  publico_alvo: "Secretarias",
  codigo_ttdd: "2.0.02.00.07",
  classificacao: SERIE,
  etapas: [
    {
      id: "e1",
      ordem: 1,
      unidade_administrativa: "SEC",
      nome_setor: "Demandante",
      atribuicoes_setor: "Elaborar o DFD",
      prazo_sla_em_dias: 1,
      documentos: [
        {
          id: "d1",
          nome_documento: "DFD",
          obrigatorio: true,
          formato: "NATO_DIGITAL",
          tipo_assinatura: "INDIVIDUAL",
          modelo_id: "m1",
          modelo: { id: "m1", nome: "DFD padrão", versao: 1, arquivo_nome: "dfd.docx" },
        },
        {
          id: "d2",
          nome_documento: "Edital",
          obrigatorio: false,
          formato: "NATO_DIGITAL",
          tipo_assinatura: "EM_BLOCO",
          modelo_id: null,
        },
      ],
      transicoes: [],
    },
  ],
};
const MODELO = {
  id: "m9",
  nome: "Edital padrão",
  ativo: true,
  atual: { versao: 2, arquivo_nome: "edital.odt" },
};

describe("Atlas público (sem login)", () => {
  beforeEach(() => {
    api.routes = {};
    api.metas = {};
  });

  it("procedimentos: lista e vazio", async () => {
    api.routes["atlas/workflows"] = [WF];
    render(await ProcedimentosPage());
    expect(screen.getByRole("link", { name: /Pregão Eletrônico/ })).toHaveAttribute(
      "href",
      "/procedimentos/wf-1",
    );
    api.routes = {};
    render(await ProcedimentosPage());
    expect(screen.getByText("Nenhum procedimento publicado no momento.")).toBeInTheDocument();
  });

  it("procedimento: etapas, modelo da peça e o da série, guarda", async () => {
    api.routes["atlas/workflows/wf-1"] = WF;
    api.routes["atlas/ttdd/2.0.02.00.07/modelos"] = [MODELO];
    const props = { params: Promise.resolve({ id: "wf-1" }) };
    expect((await metaProcedimento(props)).title).toBe("Pregão Eletrônico");
    render(await ProcedimentoPage(props));
    const docs = screen.getByRole("list", { name: "Documentos da etapa 1" });
    const [dfd, edital] = within(docs).getAllByRole("link");
    expect(dfd).toHaveAttribute("href", "/api/public/v1/atlas/modelos/m1/arquivo");
    expect(edital).toHaveAttribute("href", "/api/public/v1/atlas/modelos/m9/arquivo");
    expect(screen.getByText(/prazo previsto de 1 dia/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /2\.0\.02\.00\.07 — Processos de pregão/ }),
    ).toHaveAttribute("href", "/temporalidade/2.0.02.00.07");
  });

  it("procedimento inexistente é 404; outros erros sobem", async () => {
    const props = { params: Promise.resolve({ id: "x" }) };
    await expect(ProcedimentoPage(props)).rejects.toBeInstanceOf(NotFound);
    expect((await metaProcedimento(props)).title).toBe("Procedimento não encontrado");
    api.routes["atlas/workflows/x"] = new PublicApiError(503, "DEPENDENCY_UNAVAILABLE", "fora");
    await expect(ProcedimentoPage(props)).rejects.toThrow("fora");
  });

  it("temporalidade: busca, secretarias, paginação e planilha", async () => {
    api.routes["atlas/ttdd/estrutura"] = [{ ...ORGAO, total: 120, funcoes: [] }];
    api.routes["atlas/ttdd"] = [SERIE];
    api.metas["atlas/ttdd"] = { total_items: 120 };
    render(
      await TemporalidadePage({
        searchParams: Promise.resolve({ q: "pregão", codigo: "2.0", page: "2" }),
      }),
    );
    expect(screen.getByRole("link", { name: /Processos de pregão/ })).toHaveAttribute(
      "href",
      "/temporalidade/2.0.02.00.07",
    );
    expect(screen.getByText("120 séries")).toBeInTheDocument();
    expect(screen.getByText("Página 2 de 3")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "← Anterior" })).toHaveAttribute(
      "href",
      "/temporalidade?q=preg%C3%A3o&codigo=2.0&page=1",
    );
    expect(screen.getByRole("link", { name: "Próxima →" })).toHaveAttribute(
      "href",
      "/temporalidade?q=preg%C3%A3o&codigo=2.0&page=3",
    );
    expect(screen.getByRole("link", { name: /Baixar planilha/ })).toHaveAttribute(
      "href",
      "/api/public/v1/atlas/ttdd/exportar?q=preg%C3%A3o&codigo=2.0",
    );
    expect(screen.getByRole("link", { name: "Administração" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("temporalidade: API fora mostra a lista vazia", async () => {
    render(await TemporalidadePage({ searchParams: Promise.resolve({}) }));
    expect(screen.getByText("Nenhuma série encontrada.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Baixar planilha/ })).toHaveAttribute(
      "href",
      "/api/public/v1/atlas/ttdd/exportar",
    );
  });

  it("série: prazos, fonte, modelos e procedimentos; revogada avisa", async () => {
    api.routes["atlas/ttdd/2.0.02.00.07"] = {
      ...SERIE,
      revogada_em: "2027-05-10T00:00:00Z",
      revogada_edicao: "6.500",
    };
    api.routes["atlas/ttdd/2.0.02.00.07/modelos"] = [
      MODELO,
      { ...MODELO, id: "m0", nome: "Antigo", ativo: false },
    ];
    api.routes["atlas/workflows?codigo_ttdd"] = [WF];
    const props = { params: Promise.resolve({ codigo: "2.0.02.00.07" }) };
    expect((await metaSerie(props)).title).toBe("2.0.02.00.07 — Processos de pregão");
    render(await SeriePage(props));
    expect(
      screen.getByText("Série revogada em 10/05/2027 (Diário Oficial nº 6.500)."),
    ).toBeInTheDocument();
    expect(screen.getByText("Guarda permanente")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Baixar: Edital padrão/ })).toHaveAttribute(
      "href",
      "/api/public/v1/atlas/modelos/m9/arquivo",
    );
    expect(screen.queryByText("Antigo")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "ADM.LIC.001 — Pregão Eletrônico" })).toHaveAttribute(
      "href",
      "/procedimentos/wf-1",
    );
  });

  it("série inexistente é 404", async () => {
    const props = { params: Promise.resolve({ codigo: "9.0.01.00.01" }) };
    await expect(SeriePage(props)).rejects.toBeInstanceOf(NotFound);
    expect((await metaSerie(props)).title).toBe("Série não encontrada");
    api.routes["atlas/ttdd/9.0.01.00.01"] = new PublicApiError(503, "X", "fora");
    await expect(SeriePage(props)).rejects.toThrow("fora");
  });
});
