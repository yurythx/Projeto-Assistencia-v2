import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { OrgaoOrganograma } from "@/lib/nexus/types";
import { identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import OrganogramaPage from "./page";

const zero = { publicados: 0, em_validacao: 0, rascunhos: 0, lacunas: 0 };

const ORGANOGRAMA: OrgaoOrganograma[] = [
  {
    prefixo: "2.0",
    nome: "Secretaria Municipal de Administração",
    series: 30,
    ...zero,
    publicados: 1,
    rascunhos: 2,
    lacunas: 1,
    funcoes: [
      {
        codigo: "2.0.02",
        nome: "Gestão de Compras",
        series: 20,
        ...zero,
        publicados: 1,
        rascunhos: 1,
        subfuncoes: [
          {
            codigo: "2.0.02.00",
            nome: "Processo de Compra",
            series: 15,
            series_de_processo: 5,
            ...zero,
            publicados: 1,
            rascunhos: 1,
          },
          { codigo: "2.0.02.01", nome: "Atas", series: 5, series_de_processo: 0, ...zero },
        ],
      },
      {
        codigo: "2.0.06",
        nome: "Folha de Pagamento",
        series: 10,
        ...zero,
        rascunhos: 1,
        lacunas: 1,
        subfuncoes: [
          {
            codigo: "2.0.06.04",
            nome: "INSS",
            series: 6,
            series_de_processo: 2,
            ...zero,
            lacunas: 1,
          },
          {
            codigo: "2.0.06.00",
            nome: "Folha",
            series: 4,
            series_de_processo: 1,
            ...zero,
            rascunhos: 1,
          },
        ],
      },
    ],
  },
  {
    prefixo: "12.0",
    nome: "Secretaria Municipal de Saúde",
    series: 50,
    ...zero,
    funcoes: [
      {
        codigo: "12.0.09",
        nome: "SAMU",
        series: 50,
        ...zero,
        subfuncoes: [
          { codigo: "12.0.09.00", nome: "Atendimento", series: 50, series_de_processo: 0, ...zero },
        ],
      },
    ],
  },
];

const rotas = (gestao: boolean) => ({
  ...identityRoutes({ permissions: gestao ? ["atlas:manage"] : [] }),
  [`GET v1/atlas/${gestao ? "admin/" : ""}organograma`]: { data: ORGANOGRAMA },
  "GET v1/atlas/ttdd": page([
    {
      codigo: "2.0.02.00.07",
      descritor: "Pregão Eletrônico",
      fase_corrente_anos: 1,
      fase_corrente_condicao: "",
      fase_interm_anos: 4,
      fase_interm_condicao: "",
      destinacao_final: "GUARDA_PERMANENTE",
      observacoes: "",
      revogada_em: null,
      revogada_edicao: "",
      created_at: "",
    },
  ]),
});

describe("Atlas — organograma da TTDD", () => {
  beforeEach(() => resetNavigation({}, "/atlas/organograma"));

  it("caixas (padrão): secretaria no topo, funções com link para os procedimentos, lacuna", async () => {
    const api = mockBackend(rotas(true));
    renderApp(<OrganogramaPage />);
    expect(
      await screen.findByRole("heading", {
        level: 2,
        name: "Secretaria Municipal de Administração",
      }),
    ).toBeInTheDocument();
    expect(api.to("GET v1/atlas/admin/organograma")).toHaveLength(1);
    expect(screen.getByRole("button", { name: "Organograma em caixas" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("link", { name: /2\.0\.02\s*Gestão de Compras/ })).toHaveAttribute(
      "href",
      "/atlas/secretarias/2.0?funcao=2.0.02",
    );
    const compras = screen.getByRole("list", { name: "Subfunções de Gestão de Compras" });
    expect(within(compras).getByRole("link", { name: /Processo de Compra \(2\)/ })).toHaveAttribute(
      "href",
      "/atlas/ttdd?codigo=2.0.02.00",
    );
    expect(screen.getAllByLabelText("processo sem procedimento")).toHaveLength(1);
    expect(
      screen.getByText(/2 secretarias · 3 funções · 5 subfunções · 80 séries/),
    ).toHaveTextContent("1 publicado · 2 rascunhos · 1 subfunção com processo sem fluxo");
    // Papel em paisagem na impressão.
    expect(document.querySelector("style")?.textContent).toContain("A4 landscape");
  });

  it("troca de visualização e de secretaria pela URL; imprimir", async () => {
    mockBackend(rotas(false));
    const print = vi.fn();
    vi.stubGlobal("print", print);
    renderApp(<OrganogramaPage />);
    await screen.findByRole("heading", { level: 2, name: "Secretaria Municipal de Saúde" });
    await userEvent.click(screen.getByRole("button", { name: "Mapa de blocos" }));
    expect(router.replace).toHaveBeenLastCalledWith("/atlas/organograma?visao=blocos");
    await userEvent.selectOptions(screen.getByLabelText("Secretaria"), "12.0");
    expect(router.replace).toHaveBeenLastCalledWith("/atlas/organograma?visao=caixas&orgao=12.0");
    await userEvent.click(screen.getByRole("button", { name: /Imprimir/ }));
    expect(print).toHaveBeenCalled();
    // Público: sem rascunhos nas contagens.
    expect(screen.getByText(/procedimentos: 1 publicado/)).not.toHaveTextContent("rascunho");
  });

  it("tópicos: abre e recolhe, carrega as séries da subfunção, retrato", async () => {
    resetNavigation({}, "/atlas/organograma", "visao=topicos&orgao=2.0");
    const api = mockBackend(rotas(true));
    renderApp(<OrganogramaPage />);
    const arvore = await screen.findByRole("list", { name: "Organograma em tópicos" });
    expect(within(arvore).getByRole("link", { name: /Gestão de Compras/ })).toBeInTheDocument();
    expect(
      within(arvore).queryByRole("link", { name: /Processo de Compra/ }),
    ).not.toBeInTheDocument();
    expect(document.querySelector("style")?.textContent).toContain("A4 portrait");

    await userEvent.click(screen.getByRole("button", { name: "Abrir até as subfunções" }));
    await userEvent.click(
      screen.getByRole("button", { name: "Abrir 2.0.02.00 Processo de Compra" }),
    );
    expect(
      await screen.findByRole("link", { name: /2\.0\.02\.00\.07\s*Pregão Eletrônico/ }),
    ).toHaveAttribute("href", "/atlas/ttdd/2.0.02.00.07");
    expect(api.to("GET v1/atlas/ttdd").at(-1)?.query.get("codigo")).toBe("2.0.02.00");
    await userEvent.click(
      screen.getByRole("button", { name: "Recolher 2.0.02.00 Processo de Compra" }),
    );
    expect(screen.queryByText("Pregão Eletrônico")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Recolher tudo" }));
    expect(
      within(arvore).queryByRole("link", { name: /Gestão de Compras/ }),
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Abrir 2\.0 / }));
    expect(within(arvore).getByRole("link", { name: /Gestão de Compras/ })).toBeInTheDocument();
  });

  it("blocos: todas as secretarias divididas em funções, ou uma secretaria em subfunções", async () => {
    resetNavigation({}, "/atlas/organograma", "visao=blocos");
    mockBackend(rotas(true));
    const { unmount } = renderApp(<OrganogramaPage />);
    expect(
      await screen.findByRole("region", { name: /^2\.0 Administração: 30 séries/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /^2\.0\.06 Folha de Pagamento: 10 séries/ }),
    ).toHaveAttribute("href", "/atlas/secretarias/2.0?funcao=2.0.06");
    expect(screen.getByText(/Tamanho = número de séries/)).toBeInTheDocument();
    unmount();

    resetNavigation({}, "/atlas/organograma", "visao=blocos&orgao=2.0");
    renderApp(<OrganogramaPage />);
    expect(
      await screen.findByRole("region", { name: /^2\.0\.02 Gestão de Compras/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /^2\.0\.06\.04 INSS: 6 séries/ })).toHaveAttribute(
      "href",
      "/atlas/ttdd?codigo=2.0.06.04",
    );
  });

  it("fichas: quadro-resumo, funções em colunas e lacunas da secretaria", async () => {
    resetNavigation({}, "/atlas/organograma", "visao=fichas");
    mockBackend(rotas(true));
    renderApp(<OrganogramaPage />);
    const ficha = await screen.findByRole("article", {
      name: "Secretaria Municipal de Administração",
    });
    expect(within(ficha).getByText("Procedimentos: 1 publicado · 2 rascunhos")).toBeInTheDocument();
    expect(within(ficha).getByText("Processos da TTDD sem procedimento (1):")).toBeInTheDocument();
    expect(within(ficha).getByText("2.0.06.04 INSS")).toBeInTheDocument();
    expect(
      within(ficha).getByRole("link", { name: /Processo de Compra \(15 séries, 2 proc\.\)/ }),
    ).toBeInTheDocument();
    const saude = screen.getByRole("article", { name: "Secretaria Municipal de Saúde" });
    expect(
      within(saude).getByRole("link", { name: /Atendimento \(50 séries\)/ }),
    ).toBeInTheDocument();
  });

  it("vazio e falha ao carregar", async () => {
    mockBackend({
      ...identityRoutes({ permissions: [] }),
      "GET v1/atlas/organograma": { data: [] },
    });
    const { unmount } = renderApp(<OrganogramaPage />);
    expect(await screen.findByText("Nenhuma secretaria na TTDD")).toBeInTheDocument();
    unmount();
    mockBackend({
      ...identityRoutes({ permissions: [] }),
      "GET v1/atlas/organograma": { status: 500, error: { code: "INTERNAL", message: "falhou" } },
    });
    renderApp(<OrganogramaPage />);
    expect(await screen.findByRole("button", { name: /Tentar novamente/ })).toBeInTheDocument();
  });
});
