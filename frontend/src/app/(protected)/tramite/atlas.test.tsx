import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { pecaJuntada, situacaoGuarda } from "@/components/tramite/AtlasDoProcesso";
import type { ClassificacaoTTDD } from "@/lib/nexus/types";
import { identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import GuardaPage from "./guarda/page";
import TramitePage from "./page";
import ProcessoPage from "./[id]/page";

const SERIE = {
  codigo: "2.0.02.00.07",
  descritor: "Processos de pregão",
  fase_corrente_anos: 1,
  fase_corrente_condicao: "",
  fase_interm_anos: 4,
  fase_interm_condicao: "",
  destinacao_final: "ELIMINACAO",
  observacoes: "",
  revogada_em: null,
  revogada_edicao: "",
  created_at: "",
} as unknown as ClassificacaoTTDD;

const PROCEDIMENTO = {
  id: "wf-1",
  codigo_processual: "ADM.LIC.001",
  titulo: "Pregão Eletrônico",
  codigo_ttdd: "2.0.02.00.07",
  ativo: true,
  versao: 1,
  etapas: [
    {
      id: "e1",
      ordem: 1,
      documentos: [
        {
          id: "d1",
          nome_documento: "Termo de Referência",
          obrigatorio: true,
          modelo_id: "mod-1",
          modelo: { id: "mod-1", nome: "TR padrão", versao: 2, arquivo_nome: "tr.docx" },
        },
        { id: "d2", nome_documento: "Edital", obrigatorio: false, modelo_id: null },
        { id: "d3", nome_documento: "Parecer jurídico", obrigatorio: true, modelo_id: null },
      ],
      transicoes: [],
    },
  ],
};

const processo = (extra: Record<string, unknown> = {}) => ({
  id: "p1",
  numero: "000001/2026",
  tipo_id: "t1",
  tipo: "Contratação",
  assunto: "Compra de notebooks",
  interessado: "",
  descricao: "",
  sigilo: "publico",
  status: "aberto",
  unidade_origem_id: "u1",
  unidade_origem: "PROT",
  unidade_atual_id: "u1",
  unidade_atual: "PROT",
  created_by_name: "Maria",
  created_at: "2026-01-01T10:00:00Z",
  updated_at: "2026-01-02T10:00:00Z",
  documentos: [
    {
      id: "x1",
      processo_id: "p1",
      tipo: "termo de referencia",
      titulo: "TR",
      origem: "redigido",
      size_bytes: 0,
      status: "rascunho",
      created_at: "",
    },
  ],
  movimentos: [],
  acessos: [],
  can_act: true,
  can_route: true,
  atlas_procedimento_id: "wf-1",
  codigo_ttdd: "2.0.02.00.07",
  ...extra,
});

const atlasRoutes = {
  "GET v1/atlas/workflows/wf-1": { data: PROCEDIMENTO },
  "GET v1/atlas/workflows": page([PROCEDIMENTO]),
  "GET v1/atlas/ttdd/2.0.02.00.07": { data: SERIE },
  "GET v1/atlas/ttdd/2.0.02.00.07/modelos": {
    data: [
      { id: "mod-9", nome: "Edital padrão", ativo: true, atual: { arquivo_nome: "edital.docx" } },
    ],
  },
};

describe("Trâmite — situação da guarda", () => {
  const hoje = new Date("2030-06-01T00:00:00Z");
  it("conta do encerramento e diz o que fazer", () => {
    expect(situacaoGuarda(SERIE, undefined, hoje)).toMatch(/começa a contar no encerramento/);
    expect(situacaoGuarda(SERIE, "2030-01-01T00:00:00Z", hoje)).toBe(
      "Fase corrente até 01/01/2031.",
    );
    expect(situacaoGuarda(SERIE, "2028-01-01T00:00:00Z", hoje)).toBe(
      "No arquivo intermediário até 01/01/2033.",
    );
    expect(situacaoGuarda(SERIE, "2020-01-01T00:00:00Z", hoje)).toMatch(
      /pode ser eliminado, com a aprovação da CCPAD/,
    );
    expect(
      situacaoGuarda(
        { ...SERIE, destinacao_final: "GUARDA_PERMANENTE" },
        "2020-01-01T00:00:00Z",
        hoje,
      ),
    ).toMatch(/recolher ao arquivo permanente/);
    expect(
      situacaoGuarda({ ...SERIE, destinacao_final: null }, "2020-01-01T00:00:00Z", hoje),
    ).toMatch(/não define a destinação/);
    expect(
      situacaoGuarda(
        { ...SERIE, fase_interm_anos: null, fase_interm_condicao: "" },
        "2028-01-01T00:00:00Z",
        hoje,
      ),
    ).toMatch(/pode ser eliminado/);
    expect(
      situacaoGuarda(
        { ...SERIE, fase_interm_anos: null, fase_interm_condicao: "Até a prescrição" },
        "2028-01-01T00:00:00Z",
        hoje,
      ),
    ).toMatch(/depende de condição/);
  });

  it("peça juntada pelo tipo ou título, sem acento e caixa; cancelado não conta", () => {
    const d = (tipo: string, titulo: string, status = "rascunho") =>
      ({ tipo, titulo, status }) as Parameters<typeof pecaJuntada>[1][number];
    expect(pecaJuntada("Termo de Referência", [d("TERMO DE REFERENCIA", "x")])).toBe(true);
    expect(pecaJuntada("Edital", [d("", "edital")])).toBe(true);
    expect(pecaJuntada("Edital", [d("Edital", "x", "cancelado")])).toBe(false);
  });
});

describe("Trâmite — Atlas no processo", () => {
  beforeEach(() => resetNavigation({ id: "p1" }));

  it("abrir com procedimento do Atlas leva a série da TTDD", async () => {
    const api = mockBackend({
      ...identityRoutes({
        permissions: ["tramite:create"],
        scopes: [
          { perfil: "p", unidade_id: "u1", origem: "manual", permissions: ["tramite:create"] },
        ],
      }),
      ...atlasRoutes,
      "GET v1/tramite/processos": page([]),
      "GET v1/tramite/tipos": {
        data: [{ id: "t1", slug: "c", nome: "Contratação", descricao: "" }],
      },
      "GET v1/iam/org-tree": {
        data: [
          {
            id: "e1",
            nome: "Órgão",
            ativo: true,
            unidades: [
              { id: "u1", nome: "Protocolo", sigla: "PROT", ativo: true, departamentos: [] },
            ],
          },
        ],
      },
      "POST v1/tramite/processos": { data: processo({ id: "novo" }) },
    });
    renderApp(<TramitePage />);
    await userEvent.click(await screen.findByRole("button", { name: /Abrir processo/ }));
    const dialog = await screen.findByRole("dialog", { name: "Abrir processo" });
    const proc = await within(dialog).findByLabelText("Procedimento do Atlas");
    await within(dialog).findByRole("option", { name: "ADM.LIC.001 — Pregão Eletrônico" });
    await userEvent.selectOptions(proc, "wf-1");
    expect(within(dialog).getByText(/guardado pela série 2\.0\.02\.00\.07/)).toBeInTheDocument();
    await within(dialog).findByRole("option", { name: "Contratação" });
    await userEvent.selectOptions(within(dialog).getByLabelText("Tipo *"), "t1");
    await waitFor(() =>
      expect(within(dialog).getByLabelText("Unidade de origem *")).toHaveValue("u1"),
    );
    await userEvent.type(within(dialog).getByLabelText("Assunto *"), "Notebooks");
    await userEvent.click(within(dialog).getByRole("button", { name: "Abrir processo" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/tramite/novo"));
    expect(api.to("POST v1/tramite/processos")[0]!.body).toMatchObject({
      atlas_procedimento_id: "wf-1",
      codigo_ttdd: "2.0.02.00.07",
    });
  });

  it("cartão Atlas: checklist, modelos (próprio e da série), guarda e classificar", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["tramite:read"] }),
      ...atlasRoutes,
      "GET v1/tramite/processos/p1": {
        data: processo({ status: "concluido", concluido_at: "2020-01-01T00:00:00Z" }),
      },
      "PUT v1/tramite/processos/p1/classificacao": { data: processo() },
    });
    renderApp(<ProcessoPage />);
    expect(
      await screen.findByRole("link", { name: /ADM\.LIC\.001 — Pregão Eletrônico/ }),
    ).toHaveAttribute("href", "/atlas/procedimentos/wf-1");
    expect(screen.getByText("Peças exigidas (1/3 no processo)")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Baixar o modelo TR padrão" })).toHaveAttribute(
      "href",
      "/api/backend/v1/atlas/modelos/mod-1/arquivo",
    );
    // Sem modelo próprio, vale o da série.
    expect(
      await screen.findAllByRole("link", { name: "Baixar o modelo Edital padrão" }),
    ).toHaveLength(2);
    expect(
      await screen.findByText(/pode ser eliminado, com a aprovação da CCPAD/),
    ).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Classificar/ }));
    const dialog = await screen.findByRole("dialog", { name: "Classificar o processo" });
    await within(dialog).findByRole("option", { name: "ADM.LIC.001 — Pregão Eletrônico" });
    await userEvent.selectOptions(within(dialog).getByLabelText("Procedimento do Atlas"), "");
    const serie = within(dialog).getByLabelText("Série da TTDD (guarda do processo)");
    await userEvent.clear(serie);
    await userEvent.type(serie, "2.0.02.00.08");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar classificação" }));
    await waitFor(() =>
      expect(api.to("PUT v1/tramite/processos/p1/classificacao")[0]!.body).toEqual({
        atlas_procedimento_id: null,
        codigo_ttdd: "2.0.02.00.08",
      }),
    );
    // Escolher o procedimento preenche a série dele.
    await userEvent.click(screen.getByRole("button", { name: /Classificar/ }));
    const d2 = await screen.findByRole("dialog", { name: "Classificar o processo" });
    await within(d2).findByRole("option", { name: "ADM.LIC.001 — Pregão Eletrônico" });
    await userEvent.selectOptions(within(d2).getByLabelText("Procedimento do Atlas"), "");
    await userEvent.selectOptions(within(d2).getByLabelText("Procedimento do Atlas"), "wf-1");
    expect(within(d2).getByLabelText("Série da TTDD (guarda do processo)")).toHaveValue(
      "2.0.02.00.07",
    );
  });

  it("processo sem classificação e procedimento fora de vigor", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["tramite:read"] }),
      "GET v1/tramite/processos/p1": {
        data: processo({ atlas_procedimento_id: null, codigo_ttdd: "" }),
      },
    });
    renderApp(<ProcessoPage />);
    expect(await screen.findByText(/Processo sem classificação/)).toBeInTheDocument();
  });

  it("procedimento que saiu de vigor avisa", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["tramite:read"] }),
      "GET v1/tramite/processos/p1": {
        data: processo({ atlas_procedimento_id: "wf-x", codigo_ttdd: "" }),
      },
    });
    renderApp(<ProcessoPage />);
    expect(await screen.findByText(/não está mais em vigor no Atlas/)).toBeInTheDocument();
  });

  it("novo documento: o tipo sugere as peças e oferece o modelo", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["tramite:read"] }),
      ...atlasRoutes,
      "GET v1/tramite/processos/p1": { data: processo() },
    });
    renderApp(<ProcessoPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Novo documento/ }));
    const dialog = await screen.findByRole("dialog");
    const tipo = within(dialog).getByLabelText("Tipo");
    await waitFor(() => expect(tipo).toHaveAttribute("list", "doc-tipo-pecas"));
    await userEvent.type(tipo, "termo de referência");
    expect(within(dialog).getByRole("link", { name: /Baixar TR padrão/ })).toHaveAttribute(
      "href",
      "/api/backend/v1/atlas/modelos/mod-1/arquivo",
    );
  });
});

describe("Trâmite — guarda documental", () => {
  beforeEach(() => resetNavigation());

  it("agrupa os encerrados por série e lista os sem série", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["tramite:read"] }),
      ...atlasRoutes,
      "GET v1/atlas/ttdd/9.0.01.00.01": { status: 404, error: { code: "NOT_FOUND", message: "x" } },
      "GET v1/tramite/processos": (req) =>
        req.query.get("status") === "concluido"
          ? page([
              processo({
                id: "c1",
                numero: "000010/2020",
                status: "concluido",
                concluido_at: "2020-01-01T00:00:00Z",
              }),
              processo({
                id: "c2",
                numero: "000011/2020",
                codigo_ttdd: "9.0.01.00.01",
                concluido_at: "2020-01-01T00:00:00Z",
              }),
            ])
          : page([
              processo({ id: "a1", numero: "000012/2020", codigo_ttdd: "", status: "arquivado" }),
            ]),
    });
    renderApp(<GuardaPage />);
    const grupo = await screen.findByRole("list", { name: "Processos da série 2.0.02.00.07" });
    expect(await within(grupo).findByText(/pode ser eliminado/)).toBeInTheDocument();
    expect(await screen.findByText("Série não encontrada no Atlas.")).toBeInTheDocument();
    const sem = screen.getByRole("list", { name: "Processos sem série" });
    expect(within(sem).getByRole("link", { name: /000012\/2020/ })).toHaveAttribute(
      "href",
      "/tramite/a1",
    );
  });

  it("sem processos encerrados", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["tramite:read"] }),
      "GET v1/tramite/processos": page([]),
    });
    renderApp(<GuardaPage />);
    expect(await screen.findByText("Nenhum processo encerrado")).toBeInTheDocument();
  });
});
