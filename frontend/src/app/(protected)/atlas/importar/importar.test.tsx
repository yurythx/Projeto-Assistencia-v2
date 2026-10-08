import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { EnvioEmLote } from "@/components/atlas/EnvioEmLote";
import { nomeDoArquivo } from "@/lib/atlas/modelos";
import { identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import CoberturaPage from "../cobertura/page";
import ImportarPage from "./page";

const perms = (p: string[]) =>
  identityRoutes({ permissions: p, scopes: [{ perfil: "p", origem: "manual", permissions: p }] });

const RESULTADO = {
  hash: "b".repeat(64),
  aplicada: false,
  totais: { NOVO: 1, NOVA_VERSAO: 1, INALTERADO: 1 },
  itens: [
    { linha: 1, codigo_processual: "ADM.NOV.001", titulo: "Novo", situacao: "NOVO", versao: 1 },
    {
      linha: 2,
      codigo_processual: "ADM.LIC.001",
      titulo: "Pregão",
      situacao: "NOVA_VERSAO",
      versao: 3,
    },
    {
      linha: 3,
      codigo_processual: "ADM.DIR.002",
      titulo: "Dispensa",
      situacao: "INALTERADO",
      versao: 1,
    },
  ],
};

const json = () =>
  new File(['{"procedimentos":[]}'], "procedimentos.json", { type: "application/json" });

describe("Atlas — importar procedimentos", () => {
  beforeEach(() => resetNavigation({}, "/atlas/importar"));

  it("sem atlas:manage não mostra a importação", async () => {
    mockBackend(perms(["atlas:read"]));
    renderApp(<ImportarPage />);
    expect(await screen.findByText(/restrito à gestão do Atlas/)).toBeInTheDocument();
  });

  it("simula, mostra cada item e aplica o arquivo conferido (hash)", async () => {
    const backend = mockBackend({
      ...perms(["atlas:manage"]),
      "POST v1/atlas/admin/workflows/importacao/simular": { data: RESULTADO },
      "POST v1/atlas/admin/workflows/importacao/aplicar": {
        data: { ...RESULTADO, aplicada: true },
      },
    });
    renderApp(<ImportarPage />);
    expect(
      await screen.findByRole("link", { name: /Baixar os procedimentos em vigor/ }),
    ).toHaveAttribute("href", "/api/backend/v1/atlas/admin/workflows/exportar");
    const simular = screen.getByRole("button", { name: /Simular/ });
    expect(simular).toBeDisabled();
    await userEvent.upload(screen.getByLabelText(/Arquivo de procedimentos/), json());
    await userEvent.click(simular);
    expect(await screen.findByText(/simulação — nada foi gravado/)).toBeInTheDocument();
    // Por padrão, como rascunho (para validação, sem publicar).
    expect(backend.to("POST v1/atlas/admin/workflows/importacao/simular")[0]!.body).toEqual({
      conteudo: '{"procedimentos":[]}',
      rascunho: true,
    });
    const itens = screen.getByRole("list", { name: "Procedimentos do arquivo" });
    expect(within(itens).getByText("ADM.LIC.001")).toBeInTheDocument();
    expect(within(itens).getByText("versão 3")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Aplicar a importação" }));
    expect(await screen.findByText(/Importação aplicada/)).toBeInTheDocument();
    expect(backend.to("POST v1/atlas/admin/workflows/importacao/aplicar")[0]!.body).toEqual({
      conteudo: '{"procedimentos":[]}',
      hash: "b".repeat(64),
      rascunho: true,
    });
  });

  it("desmarcar o rascunho publica direto e refaz a simulação", async () => {
    const backend = mockBackend({
      ...perms(["atlas:manage"]),
      "POST v1/atlas/admin/workflows/importacao/simular": { data: RESULTADO },
    });
    renderApp(<ImportarPage />);
    await userEvent.upload(await screen.findByLabelText(/Arquivo de procedimentos/), json());
    await userEvent.click(screen.getByRole("button", { name: /Simular/ }));
    expect(await screen.findByText(/simulação — nada foi gravado/)).toBeInTheDocument();
    await userEvent.click(screen.getByLabelText(/Importar como rascunho/));
    expect(screen.queryByText(/simulação — nada foi gravado/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Simular/ }));
    await screen.findByText(/simulação — nada foi gravado/);
    expect(backend.to("POST v1/atlas/admin/workflows/importacao/simular")[1]!.body).toMatchObject({
      rascunho: false,
    });
  });

  it("com erro não aplica; sem mudança avisa", async () => {
    let chamada = 0;
    mockBackend({
      ...perms(["atlas:manage"]),
      "POST v1/atlas/admin/workflows/importacao/simular": () =>
        ++chamada === 1
          ? {
              data: {
                ...RESULTADO,
                totais: { ERRO: 1 },
                itens: [
                  {
                    linha: 1,
                    codigo_processual: "",
                    titulo: "",
                    situacao: "ERRO",
                    versao: 0,
                    erro: "título obrigatório",
                  },
                ],
              },
            }
          : { data: { ...RESULTADO, totais: { INALTERADO: 1 }, itens: [RESULTADO.itens[2]] } },
    });
    renderApp(<ImportarPage />);
    await userEvent.upload(await screen.findByLabelText(/Arquivo de procedimentos/), json());
    await userEvent.click(screen.getByRole("button", { name: /Simular/ }));
    expect(await screen.findByText("Item 1: título obrigatório")).toBeInTheDocument();
    expect(screen.getByText("item 1")).toBeInTheDocument();
    expect(screen.getByText(/Corrija os itens com erro/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Aplicar a importação" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Simular/ }));
    expect(await screen.findByText(/Nada muda/)).toBeInTheDocument();
  });
});

describe("Atlas — cobertura", () => {
  beforeEach(() => resetNavigation({}, "/atlas/cobertura"));

  it("sem atlas:manage não mostra o painel", async () => {
    mockBackend(perms(["atlas:read"]));
    renderApp(<CoberturaPage />);
    expect(await screen.findByText(/painel de cobertura é da gestão/)).toBeInTheDocument();
  });

  it("totais, secretarias e as listas de trabalho", async () => {
    mockBackend({
      ...perms(["atlas:manage"]),
      "GET v1/atlas/admin/cobertura": {
        data: {
          totais: {
            series: 200,
            series_com_procedimento: 3,
            series_com_modelo: 1,
            procedimentos: 3,
            pecas: 17,
            pecas_com_modelo: 2,
            modelos: 4,
            modelos_sem_uso: 1,
            rascunhos: 2,
            em_validacao: 1,
          },
          orgaos: [
            {
              prefixo: "2.0",
              nome: "Administração",
              series: 200,
              series_com_procedimento: 3,
              series_com_modelo: 1,
            },
          ],
          pecas_sem_modelo: [
            {
              workflow_id: "wf-1",
              codigo_processual: "ADM.LIC.001",
              titulo: "Pregão",
              etapa: 1,
              peca: "DFD",
            },
          ],
          modelos_sem_uso: [{ id: "mod-1", nome: "Ofício padrão" }],
          lacunas: [
            {
              codigo: "2.0.06.01",
              nome: "Descontos na Folha",
              orgao: "Administração",
              series: 9,
              exemplos: ["Vale transporte", "Pensão alimentícia", "Consignado"],
            },
          ],
        },
      },
    });
    renderApp(<CoberturaPage />);
    const totais = await screen.findByRole("list", { name: "Totais" });
    expect(within(totais).getByText("2%")).toBeInTheDocument();
    expect(within(totais).getByText("3 de 200 séries vigentes")).toBeInTheDocument();
    expect(within(totais).getByText("12%")).toBeInTheDocument();
    expect(within(totais).getByText("2 rascunhos · 1 em entrevistas")).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "Administração: séries com fluxo" }),
    ).toHaveAttribute("aria-valuenow", "2");
    expect(screen.getByRole("link", { name: "Administração" })).toHaveAttribute(
      "href",
      "/atlas/ttdd?codigo=2.0",
    );
    expect(screen.getByRole("link", { name: "DFD" })).toHaveAttribute(
      "href",
      "/atlas/procedimentos/wf-1",
    );
    expect(screen.getByRole("link", { name: "Ofício padrão" })).toHaveAttribute(
      "href",
      "/atlas/modelos?q=Of%C3%ADcio%20padr%C3%A3o",
    );
    const lacunas = screen.getByRole("list", { name: "Subfunções sem procedimento" });
    expect(within(lacunas).getByRole("link", { name: /Descontos na Folha/ })).toHaveAttribute(
      "href",
      "/atlas/ttdd?codigo=2.0.06.01",
    );
    expect(
      within(lacunas).getByText(/9 séries: Vale transporte; Pensão alimentícia; Consignado…/),
    ).toBeInTheDocument();
  });

  it("tudo coberto: listas vazias avisam", async () => {
    mockBackend({
      ...perms(["atlas:manage"]),
      "GET v1/atlas/admin/cobertura": {
        data: {
          totais: {
            series: 0,
            series_com_procedimento: 0,
            series_com_modelo: 0,
            procedimentos: 0,
            pecas: 0,
            pecas_com_modelo: 0,
            modelos: 0,
            modelos_sem_uso: 0,
          },
          orgaos: [],
          pecas_sem_modelo: [],
          modelos_sem_uso: [],
          lacunas: [],
        },
      },
    });
    renderApp(<CoberturaPage />);
    expect(await screen.findByText("Todas as peças têm modelo para baixar.")).toBeInTheDocument();
    expect(screen.getByText(/Todos os modelos ativos estão ligados/)).toBeInTheDocument();
    expect(
      screen.getByText("Toda subfunção com processo já tem procedimento."),
    ).toBeInTheDocument();
  });
});

describe("Atlas — modelos em lote", () => {
  it("nome do arquivo vira o nome do modelo; código da série liga", () => {
    expect(nomeDoArquivo("2.0.02.00.07 - Edital de pregão.docx")).toEqual({
      nome: "Edital de pregão",
      serie: "2.0.02.00.07",
    });
    expect(nomeDoArquivo("Ofício padrão.odt")).toEqual({ nome: "Ofício padrão" });
  });

  it("envia cada arquivo, liga à série e mostra o resultado de cada um", async () => {
    const onDone = vi.fn();
    const backend = mockBackend({
      ...perms(["atlas:manage"]),
      "POST v1/atlas/admin/modelos": (req) =>
        (req.body as FormData).get("nome") === "Repetido"
          ? { status: 409, error: { code: "CONFLICT", message: "já existe modelo com este nome" } }
          : { status: 201, data: { id: "mod-9" } },
      "POST v1/atlas/admin/ttdd/2.0.02.00.07/modelos": { data: [] },
      "POST v1/atlas/admin/ttdd/9.0.01.00.01/modelos": {
        status: 404,
        error: { code: "NOT_FOUND", message: "classificação TTDD não encontrada" },
      },
    });
    renderApp(<EnvioEmLote onDone={onDone} />);
    const arquivos = [
      new File(["PK"], "2.0.02.00.07 - Edital.docx"),
      new File(["PK"], "Repetido.docx"),
      new File(["PK"], "9.0.01.00.01 Termo.pdf"),
      new File(["x".repeat(10 * 1024 * 1024 + 1)], "Grande.pdf"),
    ];
    await userEvent.upload(screen.getByLabelText("Arquivos dos modelos"), arquivos);
    expect(screen.getByRole("list", { name: "Arquivos escolhidos" })).toHaveTextContent(
      "Edital → série 2.0.02.00.07",
    );
    await userEvent.click(screen.getByRole("button", { name: /Enviar 4 arquivos/ }));
    const res = await screen.findByRole("list", { name: "Resultado do envio" });
    await vi.waitFor(() => expect(within(res).getAllByRole("listitem")).toHaveLength(4));
    expect(within(res).getByText(/ligado à série 2\.0\.02\.00\.07/)).toBeInTheDocument();
    expect(within(res).getByText("já existe modelo com este nome")).toBeInTheDocument();
    expect(within(res).getByText(/não ligado à série 9\.0\.01\.00\.01/)).toBeInTheDocument();
    expect(within(res).getByText("o arquivo passa de 10 MB")).toBeInTheDocument();
    expect(backend.to("POST v1/atlas/admin/ttdd/2.0.02.00.07/modelos")[0]!.body).toEqual({
      modelo_id: "mod-9",
    });
    expect(onDone).toHaveBeenCalled();
  });

  it("falha de rede vira erro do arquivo", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Promise.reject(new Error("offline"))),
    );
    render(<EnvioEmLote onDone={() => {}} />);
    await userEvent.upload(
      screen.getByLabelText("Arquivos dos modelos"),
      new File(["PK"], "Um.docx"),
    );
    await userEvent.click(screen.getByRole("button", { name: /Enviar 1 arquivo/ }));
    expect(await screen.findByText(/falha ao enviar|offline|indisponível/)).toBeInTheDocument();
  });
});
