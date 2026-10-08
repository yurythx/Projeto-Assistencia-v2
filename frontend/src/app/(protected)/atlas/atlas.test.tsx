import { fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import AtlasLayout from "./layout";
import AtlasPage from "./page";
import ProcedimentoPage from "./procedimentos/[id]/page";
import SeriePage from "./ttdd/[codigo]/page";
import TTDDPage from "./ttdd/page";

/** Preenche colando (rápido e estável sob carga) em vez de digitar tecla a tecla. */
async function fill(el: HTMLElement, text: string) {
  await userEvent.click(el);
  await userEvent.paste(text);
}

const ORGAO = {
  prefixo: "2.0",
  nome: "Secretaria Municipal de Administração",
  edicao_diario: "6.017",
  data_publicacao: "2025-08-25T00:00:00Z",
  versao: "II",
};

const CLASSIFICACAO = {
  codigo: "2.0.02.00.07",
  descritor: "Pregão Presencial / Eletrônico",
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
    nome: "Processo de Compra",
    recomendacao: "Transferir após aprovação do TCE-MT.",
    funcao: { codigo: "2.0.02", nome: "Gestão de Compras", orgao: ORGAO },
  },
  created_at: "2026-10-01T00:00:00Z",
};

const ESTRUTURA = [
  {
    ...ORGAO,
    total: 2,
    funcoes: [
      {
        codigo: "2.0.02",
        nome: "Gestão de Compras",
        total: 2,
        subfuncoes: [{ codigo: "2.0.02.00", nome: "Processo de Compra", total: 2 }],
      },
    ],
  },
];

const RESUMO = {
  id: "wf-1",
  codigo_processual: "ADM.LIC.001",
  titulo: "Pregão Eletrônico",
  objetivo: "Contratação de bens e serviços comuns",
  publico_alvo: "Secretarias",
  versao: 1,
  ativo: true,
  nivel_acesso: "PUBLICO",
  hipotese_legal_restricao: "",
  codigo_ttdd: "2.0.02.00.07",
  classificacao: CLASSIFICACAO,
  total_etapas: 1,
  etapas: [],
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
};

const DETALHE = {
  ...RESUMO,
  etapas: [
    {
      id: "et-1",
      ordem: 1,
      unidade_administrativa: "SEC/DEM",
      nome_setor: "Setor Demandante",
      atribuicoes_setor: "Elaborar o Termo de Referência",
      prazo_sla_em_dias: 5,
      manter_aberto_apos_remessa: true,
      documentos: [
        {
          id: "doc-1",
          nome_documento: "Termo de Referência",
          obrigatorio: true,
          formato: "NATO_DIGITAL",
          tipo_assinatura: "CONJUNTA_MULTINIVEL",
          exige_conferencia_copia: false,
          modelo_minuta_padrao_url: "",
        },
      ],
      transicoes: [
        {
          id: "tr-1",
          destino_ordem: 2,
          condicao_transicao: "TR aprovado",
          is_devolucao_diligencia: false,
          descricao_diligencia: "",
        },
      ],
    },
  ],
};

const base = (perms: string[]) => ({
  ...identityRoutes({
    permissions: perms,
    scopes: [{ perfil: "p", origem: "manual", permissions: perms }],
  }),
  "GET v1/atlas/workflows": page([RESUMO]),
  "GET v1/atlas/admin/workflows": page([RESUMO]),
  "GET v1/atlas/workflows/wf-1": { data: DETALHE },
  "GET v1/atlas/admin/workflows/wf-1": { data: DETALHE },
  "GET v1/atlas/ttdd": page([
    CLASSIFICACAO,
    {
      ...CLASSIFICACAO,
      codigo: "2.0.01.00.01",
      descritor: "Organogramas",
      destinacao_final: "ELIMINACAO",
    },
    {
      ...CLASSIFICACAO,
      codigo: "12.0.03.01.02",
      descritor: "Resultado de Exames Médicos",
      fase_corrente_anos: null,
      fase_corrente_condicao: "Enquanto estiver vigorando",
      fase_interm_anos: null,
      destinacao_final: null,
      observacoes: "Entregue ao paciente após transcrição no prontuário",
      subfuncao: undefined,
    },
  ]),
  "GET v1/atlas/ttdd/estrutura": { data: ESTRUTURA },
  "GET v1/atlas/ttdd/2.0.02.00.07/historico": { data: [] },
  "GET v1/atlas/ttdd/*": { data: [] },
});

/** As páginas do Atlas rodam dentro do layout (assistente em gaveta). */
const renderAtlas = (ui: React.ReactElement) => renderApp(<AtlasLayout>{ui}</AtlasLayout>);

const CHAT_OK = {
  answer: "Guarde por 1 ano na fase corrente e 4 na intermediária.",
  score: 0.9,
  refused: false,
  mode: "sintese",
  sources: [
    { tipo: "procedimento", id: "wf-1", codigo: "ADM.LIC.001", titulo: "Pregão", relevancia: 0.9 },
    { tipo: "ttdd", codigo: "2.0.02.00.07", titulo: "Pregão", relevancia: 0.85 },
  ],
  generated_at: "2026-10-01T00:00:00Z",
};

describe("Atlas — início", () => {
  beforeEach(() => resetNavigation({}, "/atlas"));

  it("consulta pública: cartões com link, atalhos por secretaria, sem gestão nem assistente", async () => {
    const backend = mockBackend(base([]));
    renderAtlas(<AtlasPage />);

    const card = await screen.findByRole("link", { name: /ADM\.LIC\.001/ });
    expect(card).toHaveAttribute("href", "/atlas/procedimentos/wf-1");
    expect(within(card).getByText("1 etapa")).toBeInTheDocument();
    expect(backend.to("GET v1/atlas/workflows")).toHaveLength(1);
    expect(backend.to("GET v1/atlas/admin/workflows")).toHaveLength(0);
    expect(
      await screen.findByRole("link", { name: /Secretaria Municipal de Administração/ }),
    ).toHaveAttribute("href", "/atlas/ttdd?codigo=2.0");
    expect(screen.queryByRole("button", { name: /Novo procedimento/ })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Perguntar ao assistente" }),
    ).not.toBeInTheDocument();
  });

  it("links antigos (?procedimento=, ?ttdd=) redirecionam para as páginas próprias", async () => {
    mockBackend(base([]));
    resetNavigation({}, "/atlas", "procedimento=wf-1");
    const { unmount } = renderAtlas(<AtlasPage />);
    expect(router.replace).toHaveBeenCalledWith("/atlas/procedimentos/wf-1");
    unmount();

    resetNavigation({}, "/atlas", "ttdd=2.0.02.00.07");
    renderAtlas(<AtlasPage />);
    expect(router.replace).toHaveBeenCalledWith("/atlas/ttdd/2.0.02.00.07");
  });

  it("busca unificada: procedimentos e séries em paralelo, teclado e pergunta ao assistente", async () => {
    const backend = mockBackend({
      ...base(["atlas:read"]),
      "POST v1/atlas/chat": { data: CHAT_OK },
    });
    renderAtlas(<AtlasPage />);

    const campo = screen.getByRole("combobox", { name: "Buscar no Atlas" });
    await fill(campo, "pregão");
    expect(await screen.findByRole("option", { name: /Pregão Eletrônico/ })).toBeInTheDocument();
    expect(
      await screen.findByRole("option", { name: /Pregão Presencial \/ Eletrônico/ }),
    ).toBeInTheDocument();
    expect(backend.to("GET v1/atlas/workflows").at(-1)!.query.get("q")).toBe("pregão");
    expect(backend.to("GET v1/atlas/ttdd").at(-1)!.query.get("q")).toBe("pregão");

    // Seta para cima a partir do campo vai à última opção; para baixo, volta à 1ª.
    await userEvent.keyboard("{ArrowUp}");
    expect(campo.getAttribute("aria-activedescendant")).toMatch(/-op-\d+$/);
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.keyboard("{Enter}");
    expect(router.push).toHaveBeenCalledWith("/atlas/procedimentos/wf-1");

    await userEvent.click(campo);
    await userEvent.click(await screen.findByRole("option", { name: /Pregão Presencial/ }));
    expect(router.push).toHaveBeenCalledWith("/atlas/ttdd/2.0.02.00.07");

    await userEvent.click(campo);
    await userEvent.click(await screen.findByRole("option", { name: /Ver todas as séries/ }));
    expect(router.push).toHaveBeenCalledWith("/atlas/ttdd?q=preg%C3%A3o");

    // Escape fecha a lista.
    await userEvent.click(campo);
    expect(campo).toHaveAttribute("aria-expanded", "true");
    await userEvent.keyboard("{Escape}");
    expect(campo).toHaveAttribute("aria-expanded", "false");

    // Pergunta ao assistente: abre a gaveta já com a pergunta, sem enviar.
    await userEvent.click(campo);
    await userEvent.click(await screen.findByRole("option", { name: /Perguntar ao assistente/ }));
    const gaveta = await screen.findByRole("dialog", { name: "Assistente do Atlas" });
    expect(within(gaveta).getByLabelText("Sua pergunta")).toHaveValue("pregão");
    expect(backend.to("POST v1/atlas/chat")).toHaveLength(0);
  });

  it("Enter sem opção destacada: pergunta ao assistente ou, sem ele, abre a TTDD", async () => {
    mockBackend(base([]));
    const { unmount } = renderAtlas(<AtlasPage />);
    const campo = () => screen.getByRole("combobox", { name: "Buscar no Atlas" });
    await userEvent.click(campo());
    await userEvent.keyboard("{Enter}"); // vazio: nada acontece
    await fill(campo(), "alvará");
    await userEvent.keyboard("{Enter}");
    expect(router.push).toHaveBeenCalledWith("/atlas/ttdd?q=alvar%C3%A1");
    unmount();

    mockBackend(base(["atlas:read"]));
    renderAtlas(<AtlasPage />);
    await fill(campo(), "alvará");
    await userEvent.keyboard("{Enter}");
    const gaveta = await screen.findByRole("dialog", { name: "Assistente do Atlas" });
    expect(within(gaveta).getByLabelText("Sua pergunta")).toHaveValue("alvará");
  });

  it("cadastro: várias etapas na ordem e peças uma por linha", async () => {
    const backend = mockBackend({
      ...base(["atlas:manage"]),
      "POST v1/atlas/admin/workflows": { status: 201, data: { ...DETALHE, id: "wf-2" } },
    });
    renderAtlas(<AtlasPage />);

    await userEvent.click(await screen.findByRole("button", { name: /Novo procedimento/ }));
    await fill(screen.getByLabelText("Código processual *"), "adm.fin.021");
    await userEvent.selectOptions(screen.getByLabelText("Nível de acesso sugerido *"), "RESTRITO");
    await fill(screen.getByLabelText("Hipótese legal da restrição *"), "LAI, art. 31");
    await fill(screen.getByLabelText("Título do procedimento *"), "Adiantamento");
    await fill(screen.getByLabelText("Objetivo *"), "Prestar contas");
    await fill(screen.getByLabelText("Público-alvo *"), "Supridos");
    // Série por busca (código ou descritor): a sugestão mostra o descritor.
    await fill(screen.getByLabelText("Classificação TTDD *"), "2.0.02.00.07");
    expect(await screen.findByText("Pregão Presencial / Eletrônico")).toBeInTheDocument();

    const setores = () => screen.getAllByLabelText("Setor *");
    await fill(setores()[0]!, "Gabinete");
    await fill(screen.getAllByLabelText("Sigla da unidade *")[0]!, "GAB");
    await fill(screen.getAllByLabelText("Atribuições *")[0]!, "Solicitar");
    await fill(
      screen.getAllByLabelText("Peças exigidas (uma por linha)")[0]!,
      "Requerimento\nComprovante",
    );
    await userEvent.click(screen.getByRole("button", { name: /Adicionar etapa/ }));
    await fill(setores()[1]!, "Contabilidade");
    await fill(screen.getAllByLabelText("Sigla da unidade *")[1]!, "SEFIN");
    await fill(screen.getAllByLabelText("Atribuições *")[1]!, "Analisar");

    // Publicar direto: desmarca o rascunho (padrão).
    await userEvent.click(screen.getByLabelText(/Salvar como rascunho/));
    await userEvent.click(screen.getByRole("button", { name: "Cadastrar procedimento" }));
    const body = backend.to("POST v1/atlas/admin/workflows")[0]!.body as {
      codigo_processual: string;
      codigo_ttdd: string;
      nivel_acesso: string;
      hipotese_legal_restricao: string;
      etapas: { ordem: number; nome_setor: string; documentos: { nome_documento: string }[] }[];
    };
    expect(body.codigo_processual).toBe("adm.fin.021");
    expect(body.codigo_ttdd).toBe("2.0.02.00.07");
    expect(body.nivel_acesso).toBe("RESTRITO");
    expect(body.hipotese_legal_restricao).toBe("LAI, art. 31");
    expect(body.etapas.map((e) => [e.ordem, e.nome_setor])).toEqual([
      [1, "Gabinete"],
      [2, "Contabilidade"],
    ]);
    expect(body.etapas[0]!.documentos.map((d) => d.nome_documento)).toEqual([
      "Requerimento",
      "Comprovante",
    ]);
    expect(router.push).toHaveBeenCalledWith("/atlas/procedimentos/wf-2");
  });
});

describe("Atlas — procedimento", () => {
  beforeEach(() => resetNavigation({ id: "wf-1" }, "/atlas/procedimentos/wf-1"));

  it("percurso em linha do tempo, peças, temporalidade e calculadora", async () => {
    const backend = mockBackend(base([]));
    renderAtlas(<ProcedimentoPage />);

    expect(
      await screen.findByRole("heading", { level: 1, name: "Pregão Eletrônico" }),
    ).toBeInTheDocument();
    expect(backend.to("GET v1/atlas/workflows/wf-1")).toHaveLength(1);
    const trilha = screen.getByRole("navigation", { name: "Trilha de navegação" });
    expect(within(trilha).getByText("ADM.LIC.001")).toHaveAttribute("aria-current", "page");
    expect(screen.getByText(/5 dias/, { selector: "dd" })).toBeInTheDocument();

    const percurso = screen.getByRole("region", { name: "Percurso" });
    expect(within(percurso).getByText("Setor Demandante")).toBeInTheDocument();
    expect(within(percurso).getByText(/mantém o processo aberto/)).toBeInTheDocument();
    expect(within(percurso).getByText(/TR aprovado → etapa 2/)).toBeInTheDocument();

    const pecas = screen.getByRole("region", { name: "Peças exigidas" });
    expect(within(pecas).getByText("Termo de Referência")).toBeInTheDocument();
    expect(within(pecas).getByText(/assinatura conjunta \(multinível\)/)).toBeInTheDocument();
    expect(within(pecas).getByText("Obrigatória")).toBeInTheDocument();

    const temp = screen.getByRole("region", { name: "Temporalidade" });
    expect(within(temp).getByRole("link", { name: /2\.0\.02\.00\.07/ })).toHaveAttribute(
      "href",
      "/atlas/ttdd/2.0.02.00.07",
    );
    expect(within(temp).getByText("1 ano")).toBeInTheDocument();
    expect(within(temp).getByText("4 anos")).toBeInTheDocument();
    expect(within(temp).getByText(/Transferir após aprovação do TCE-MT/)).toBeInTheDocument();
    expect(
      within(temp).getByText(/versão II — Diário Oficial nº 6\.017 de 25\/08\/2025/),
    ).toBeInTheDocument();

    fireEvent.change(within(temp).getByLabelText(/Data de encerramento/), {
      target: { value: "2020-01-15" },
    });
    expect(within(temp).getByText("15/01/2021")).toBeInTheDocument();
    expect(within(temp).getAllByText("15/01/2025")).toHaveLength(2);
    expect(within(temp).getByText(/: guarda permanente/)).toBeInTheDocument();

    // Sem atlas:read não há "Perguntar"; imprimir sempre.
    expect(
      screen.queryByRole("button", { name: "Perguntar sobre este fluxo" }),
    ).not.toBeInTheDocument();
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    await userEvent.click(screen.getByRole("button", { name: "Imprimir" }));
    expect(print).toHaveBeenCalled();
  });

  it("gestão: rota administrativa, desativação e reativação", async () => {
    const backend = mockBackend({
      ...base(["atlas:manage", "atlas:read"]),
      "POST v1/atlas/admin/workflows/wf-1/desativar": { data: { ...DETALHE, ativo: false } },
      "POST v1/atlas/admin/workflows/wf-1/ativar": { data: DETALHE },
    });
    renderAtlas(<ProcedimentoPage />);

    await userEvent.click(await screen.findByRole("button", { name: "Desativar procedimento" }));
    expect(backend.to("GET v1/atlas/admin/workflows/wf-1")).toHaveLength(1);
    expect(backend.to("POST v1/atlas/admin/workflows/wf-1/desativar")).toHaveLength(1);
    await userEvent.click(await screen.findByRole("button", { name: "Ativar procedimento" }));
    expect(backend.to("POST v1/atlas/admin/workflows/wf-1/ativar")).toHaveLength(1);
    expect(
      await screen.findByRole("button", { name: "Desativar procedimento" }),
    ).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Perguntar sobre este fluxo" }));
    const gaveta = await screen.findByRole("dialog", { name: "Assistente do Atlas" });
    expect(within(gaveta).getByLabelText("Sua pergunta")).toHaveValue(
      'Como funciona o fluxo de "Pregão Eletrônico" (ADM.LIC.001), do início ao fim?',
    );
  });

  it("série sem classificação, procedimento restrito e sem peças", async () => {
    mockBackend({
      ...base([]),
      "GET v1/atlas/workflows/wf-1": {
        data: {
          ...DETALHE,
          classificacao: undefined,
          nivel_acesso: "RESTRITO",
          hipotese_legal_restricao: "LAI, art. 31",
          etapas: [
            {
              ...DETALHE.etapas[0],
              manter_aberto_apos_remessa: false,
              documentos: [
                {
                  ...DETALHE.etapas[0]!.documentos[0],
                  obrigatorio: false,
                  formato: "EXTERNO_DIGITALIZADO",
                  exige_conferencia_copia: true,
                  modelo_minuta_padrao_url: "https://exemplo.gov.br/modelo.docx",
                },
              ],
              transicoes: [
                {
                  id: "tr-2",
                  destino_ordem: 1,
                  condicao_transicao: "Pendência",
                  is_devolucao_diligencia: true,
                  descricao_diligencia: "corrigir o TR",
                },
              ],
            },
          ],
        },
      },
    });
    renderAtlas(<ProcedimentoPage />);
    expect(await screen.findByText("LAI, art. 31")).toBeInTheDocument();
    expect(screen.getByText(/Série 2\.0\.02\.00\.07 não encontrada/)).toBeInTheDocument();
    expect(screen.getByText("Opcional")).toBeInTheDocument();
    expect(screen.getByText("Conferência da cópia")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Modelo/ })).toHaveAttribute(
      "href",
      "https://exemplo.gov.br/modelo.docx",
    );
    expect(screen.getByText(/diligência: corrigir o TR/)).toBeInTheDocument();
  });

  it("procedimento sem peças", async () => {
    mockBackend({
      ...base([]),
      "GET v1/atlas/workflows/wf-1": {
        data: { ...DETALHE, etapas: [{ ...DETALHE.etapas[0], documentos: [], transicoes: [] }] },
      },
    });
    renderAtlas(<ProcedimentoPage />);
    expect(await screen.findByText(/não lista peças obrigatórias/)).toBeInTheDocument();
  });
});

describe("Atlas — TTDD", () => {
  beforeEach(() => resetNavigation({}, "/atlas/ttdd"));

  it("consulta: prazos por anos ou condição, recomendação uma vez por subfunção, exportar", async () => {
    mockBackend(base([]));
    renderAtlas(<TTDDPage />);

    expect(await screen.findByText("Organogramas")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Organogramas/ })).toHaveAttribute(
      "href",
      "/atlas/ttdd/2.0.01.00.01",
    );
    expect(screen.getAllByText("Guarda permanente").length).toBeGreaterThan(0);
    expect(screen.getByText("Eliminação")).toBeInTheDocument();
    expect(screen.getByText(/Corrente: Enquanto estiver vigorando/)).toBeInTheDocument();
    expect(screen.getByText("Não definida na TTDD")).toBeInTheDocument();
    // Duas séries da mesma subfunção: a recomendação aparece uma só vez.
    expect(screen.getAllByText(/Recomendação: Transferir/)).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 2, name: /Todas as séries/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Exportar CSV/ })).toHaveAttribute(
      "href",
      "/api/backend/v1/atlas/ttdd/exportar",
    );

    await fill(screen.getByLabelText("Código ou descritor"), "pregão");
    await userEvent.click(screen.getByRole("button", { name: "Buscar" }));
    expect(router.replace).toHaveBeenCalledWith("/atlas/ttdd?q=preg%C3%A3o");
  });

  it("filtro na URL: árvore aberta no ramo, selo da fonte, paginação e exportação filtrada", async () => {
    resetNavigation({}, "/atlas/ttdd", "codigo=2.0.02.00&q=preg&page=2");
    const backend = mockBackend({
      ...base([]),
      "GET v1/atlas/ttdd": {
        data: [CLASSIFICACAO],
        meta: { page: 2, page_size: 50, total_items: 51, total_pages: 2 },
      },
    });
    renderAtlas(<TTDDPage />);

    const arvore = await screen.findByRole("navigation", {
      name: "Plano de classificação da TTDD",
    });
    expect(await within(arvore).findByRole("link", { name: /Processo de Compra/ })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(within(arvore).getByRole("link", { name: /Gestão de Compras/ })).toHaveAttribute(
      "href",
      "/atlas/ttdd?codigo=2.0.02",
    );
    expect(
      screen.getByRole("heading", { level: 2, name: /Processo de Compra/ }),
    ).toBeInTheDocument();
    expect(await screen.findByText("51 séries")).toBeInTheDocument();
    expect(
      screen.getAllByText(/TTDD da Secretaria Municipal de Administração, versão II/).length,
    ).toBeGreaterThan(0);
    const req = backend.to("GET v1/atlas/ttdd").at(-1)!.query;
    expect([req.get("codigo"), req.get("q"), req.get("page")]).toEqual(["2.0.02.00", "preg", "2"]);
    expect(screen.getByRole("link", { name: /Exportar CSV/ })).toHaveAttribute(
      "href",
      "/api/backend/v1/atlas/ttdd/exportar?codigo=2.0.02.00&q=preg",
    );

    // Voltar à página 1 tira o parâmetro da URL.
    await userEvent.click(screen.getByRole("button", { name: /anterior/i }));
    expect(router.replace).toHaveBeenCalledWith("/atlas/ttdd?codigo=2.0.02.00&q=preg");
  });

  it("série: prazos, guarda total, procedimentos que a produzem e pergunta ao assistente", async () => {
    resetNavigation({ codigo: "2.0.02.00.07" }, "/atlas/ttdd/2.0.02.00.07");
    const backend = mockBackend({
      ...base(["atlas:read"]),
      "GET v1/atlas/ttdd/2.0.02.00.07": { data: CLASSIFICACAO },
    });
    renderAtlas(<SeriePage />);

    expect(
      await screen.findByRole("heading", { level: 1, name: "Pregão Presencial / Eletrônico" }),
    ).toBeInTheDocument();
    const trilha = screen.getByRole("navigation", { name: "Trilha de navegação" });
    expect(within(trilha).getByRole("link", { name: "Processo de Compra" })).toHaveAttribute(
      "href",
      "/atlas/ttdd?codigo=2.0.02.00",
    );
    expect(screen.getByText("5 anos")).toBeInTheDocument();
    expect(screen.getByText(/Observações:/)).toBeInTheDocument();
    expect(screen.getByText(/Recomendação da subfunção:/)).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: /ADM\.LIC\.001/ })).toBeInTheDocument();
    expect(backend.to("GET v1/atlas/workflows").at(-1)!.query.get("codigo_ttdd")).toBe(
      "2.0.02.00.07",
    );
    expect(
      screen.getByRole("link", { name: /Ver as demais séries de Processo de Compra/ }),
    ).toHaveAttribute("href", "/atlas/ttdd?codigo=2.0.02.00");

    await userEvent.click(screen.getByRole("button", { name: "Perguntar sobre esta série" }));
    const gaveta = await screen.findByRole("dialog", { name: "Assistente do Atlas" });
    expect(within(gaveta).getByLabelText("Sua pergunta")).toHaveValue(
      'Por quanto tempo guardar "Pregão Presencial / Eletrônico" (2.0.02.00.07) e qual a destinação?',
    );
    await userEvent.click(within(gaveta).getByRole("button", { name: "Fechar o assistente" }));
    expect(screen.queryByRole("dialog", { name: "Assistente do Atlas" })).not.toBeInTheDocument();
  });

  it("série por condição: calculadora explica quando não há data", async () => {
    resetNavigation({ codigo: "12.0.03.01.02" }, "/atlas/ttdd/12.0.03.01.02");
    mockBackend({
      ...base([]),
      "GET v1/atlas/ttdd/12.0.03.01.02": {
        data: {
          ...CLASSIFICACAO,
          codigo: "12.0.03.01.02",
          fase_corrente_anos: null,
          fase_corrente_condicao: "Enquanto estiver vigorando",
          fase_interm_anos: null,
          fase_interm_condicao: "Até a prescrição",
          destinacao_final: null,
          observacoes: "",
          subfuncao: undefined,
        },
      },
      "GET v1/atlas/workflows": page([]),
    });
    renderAtlas(<SeriePage />);

    fireEvent.change(await screen.findByLabelText("Data em que deixou de vigorar"), {
      target: { value: "2024-03-01" },
    });
    expect(screen.getByText(/depende de condição \("Até a prescrição"\)/)).toBeInTheDocument();
    expect(screen.queryByText(/Guarda total/)).not.toBeInTheDocument();
    expect(
      await screen.findByText("Nenhum procedimento cadastrado nesta série"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Ver as demais séries da tabela/ })).toHaveAttribute(
      "href",
      "/atlas/ttdd",
    );
  });

  it("série sem fase intermediária e sem prazo corrente informado", async () => {
    resetNavigation({ codigo: "2.0.01.00.01" }, "/atlas/ttdd/2.0.01.00.01");
    mockBackend({
      ...base([]),
      "GET v1/atlas/ttdd/2.0.01.00.01": {
        data: { ...CLASSIFICACAO, fase_corrente_anos: 2, fase_interm_anos: null },
      },
    });
    const { unmount } = renderAtlas(<SeriePage />);
    fireEvent.change(await screen.findByLabelText(/Data de encerramento/), {
      target: { value: "2020-02-29" },
    });
    expect(screen.getByText("Sem fase intermediária")).toBeInTheDocument();
    expect(screen.getAllByText("28/02/2022")).toHaveLength(2);
    unmount();

    mockBackend({
      ...base([]),
      "GET v1/atlas/ttdd/2.0.01.00.01": {
        data: { ...CLASSIFICACAO, fase_corrente_anos: null, fase_corrente_condicao: "" },
      },
    });
    renderAtlas(<SeriePage />);
    fireEvent.change(await screen.findByLabelText(/Data de encerramento/), {
      target: { value: "2020-01-01" },
    });
    expect(screen.getByText(/não informa o prazo da fase corrente/)).toBeInTheDocument();
  });
});

describe("Atlas — assistente em gaveta", () => {
  beforeEach(() => resetNavigation({}, "/atlas"));

  it("envia a pergunta e mostra modo e fontes com links para as páginas", async () => {
    const backend = mockBackend({
      ...base(["atlas:read"]),
      "POST v1/atlas/chat": { data: CHAT_OK },
    });
    renderAtlas(<AtlasPage />);

    await userEvent.click(await screen.findByRole("button", { name: "Perguntar ao assistente" }));
    const gaveta = await screen.findByRole("dialog", { name: "Assistente do Atlas" });
    await userEvent.type(
      within(gaveta).getByLabelText("Sua pergunta"),
      "Por quanto tempo guardar o pregão?",
    );
    await userEvent.click(within(gaveta).getByRole("button", { name: "Enviar pergunta" }));

    expect(
      await within(gaveta).findByText("Guarde por 1 ano na fase corrente e 4 na intermediária."),
    ).toBeInTheDocument();
    expect(
      within(gaveta).getByText(/Síntese direta dos fluxos e da TTDD oficial · relevância 90%/),
    ).toBeInTheDocument();
    expect(within(gaveta).getByRole("link", { name: "ADM.LIC.001" })).toHaveAttribute(
      "href",
      "/atlas/procedimentos/wf-1",
    );
    expect(within(gaveta).getByRole("link", { name: "2.0.02.00.07" })).toHaveAttribute(
      "href",
      "/atlas/ttdd/2.0.02.00.07",
    );
    expect(backend.to("POST v1/atlas/chat")[0]!.body).toEqual({
      query: "Por quanto tempo guardar o pregão?",
    });

    // Seguir uma fonte fecha a gaveta.
    await userEvent.click(within(gaveta).getByRole("link", { name: "2.0.02.00.07" }));
    expect(screen.queryByRole("dialog", { name: "Assistente do Atlas" })).not.toBeInTheDocument();
  });

  it("mostra a mensagem do backend quando a consulta falha", async () => {
    mockBackend({
      ...base(["atlas:read"]),
      "POST v1/atlas/chat": {
        status: 429,
        error: { code: "RATE_LIMITED", message: "Muitas consultas; aguarde um minuto." },
      },
    });
    renderAtlas(<AtlasPage />);

    await userEvent.click(await screen.findByRole("button", { name: "Perguntar ao assistente" }));
    const gaveta = await screen.findByRole("dialog", { name: "Assistente do Atlas" });
    await userEvent.type(within(gaveta).getByLabelText("Sua pergunta"), "pregão eletrônico");
    await userEvent.click(within(gaveta).getByRole("button", { name: "Enviar pergunta" }));
    expect(
      await within(gaveta).findByText("Muitas consultas; aguarde um minuto."),
    ).toBeInTheDocument();
  });
});

describe("Atlas — vigência da TTDD e versões", () => {
  const REVOGADA = {
    ...CLASSIFICACAO,
    revogada_em: "2027-05-10T00:00:00Z",
    revogada_edicao: "6.500",
  };
  // Duas etapas com ida e volta (diligência) e peça com atributos próprios.
  const BASE = {
    ...DETALHE,
    classificacao: REVOGADA,
    etapas: [
      {
        ...DETALHE.etapas[0]!,
        documentos: [
          {
            ...DETALHE.etapas[0]!.documentos[0]!,
            formato: "EXTERNO_DIGITALIZADO",
            exige_conferencia_copia: true,
          },
        ],
      },
      {
        id: "et-2",
        ordem: 2,
        unidade_administrativa: "SEMAD/LIC",
        nome_setor: "Licitações",
        atribuicoes_setor: "Conduzir o certame",
        prazo_sla_em_dias: 10,
        manter_aberto_apos_remessa: false,
        documentos: [],
        transicoes: [
          {
            id: "tr-2",
            destino_ordem: 1,
            condicao_transicao: "Pendência",
            is_devolucao_diligencia: true,
            descricao_diligencia: "Completar o TR",
          },
        ],
      },
    ],
  };

  beforeEach(() => resetNavigation({ id: "wf-1" }, "/atlas/procedimentos/wf-1"));

  it("procedimento numa série revogada: aviso, versões e nova versão sem perder peças nem transições", async () => {
    const backend = mockBackend({
      ...base(["atlas:manage"]),
      "GET v1/atlas/admin/workflows/wf-1": { data: BASE },
      "GET v1/atlas/admin/workflows": page([
        { ...RESUMO, id: "wf-1", versao: 2 },
        { ...RESUMO, id: "wf-0", versao: 1, ativo: false },
      ]),
      "POST v1/atlas/admin/workflows/wf-1/versoes": {
        status: 201,
        data: { ...DETALHE, id: "wf-3", versao: 3 },
      },
    });
    renderAtlas(<ProcedimentoPage />);

    expect(await screen.findByText(/deste procedimento foi revogada em/)).toHaveTextContent(
      "A série 2.0.02.00.07 deste procedimento foi revogada em 10/05/2027 (Diário Oficial nº 6.500)",
    );
    const temp = screen.getByRole("region", { name: "Temporalidade" });
    expect(within(temp).getByText(/Série revogada em 10\/05\/2027/)).toBeInTheDocument();
    expect(within(temp).queryByText(/Vigente/)).not.toBeInTheDocument();

    const versoes = await screen.findByRole("region", { name: "Versões" });
    expect(within(versoes).getByRole("link", { name: "Versão 1" })).toHaveAttribute(
      "href",
      "/atlas/procedimentos/wf-0",
    );
    expect(backend.to("GET v1/atlas/admin/workflows").at(-1)!.query.get("codigo_processual")).toBe(
      "ADM.LIC.001",
    );

    await userEvent.click(screen.getByRole("button", { name: "Nova versão" }));
    const dialogo = await screen.findByRole("dialog", { name: /Nova versão de ADM\.LIC\.001/ });
    const d = within(dialogo);
    expect(d.getByLabelText("Código processual *")).toHaveAttribute("readonly");
    expect(d.getByLabelText("Título do procedimento *")).toHaveValue("Pregão Eletrônico");
    // A série revogada não vem preenchida: é preciso escolher uma vigente.
    expect(d.getByLabelText("Classificação TTDD *")).toHaveValue("");
    expect(d.getByText(/A série 2\.0\.02\.00\.07 foi revogada/)).toBeInTheDocument();
    expect(d.getByText("TR aprovado → etapa 2")).toBeInTheDocument();
    expect(d.getByText("Pendência → etapa 1")).toBeInTheDocument();

    await fill(d.getByLabelText("Classificação TTDD *"), "2.0.02.01.02");
    await fill(d.getAllByLabelText("Peças exigidas (uma por linha)")[0]!, "\nEstudo Técnico");
    // Padrão: a nova versão vai como rascunho para validação.
    await userEvent.click(d.getByRole("button", { name: "Salvar rascunho" }));

    const body = backend.to("POST v1/atlas/admin/workflows/wf-1/versoes")[0]!.body as {
      codigo_processual: string;
      codigo_ttdd: string;
      etapas: { documentos: unknown[]; transicoes: unknown[] }[];
      rascunho: boolean;
    };
    expect(body.rascunho).toBe(true);
    expect(body.codigo_processual).toBe("ADM.LIC.001");
    expect(body.codigo_ttdd).toBe("2.0.02.01.02");
    // Peça existente mantém formato/assinatura; a nova entra com o padrão.
    expect(body.etapas[0]!.documentos).toEqual([
      expect.objectContaining({
        nome_documento: "Termo de Referência",
        formato: "EXTERNO_DIGITALIZADO",
        tipo_assinatura: "CONJUNTA_MULTINIVEL",
        exige_conferencia_copia: true,
      }),
      expect.objectContaining({
        nome_documento: "Estudo Técnico",
        formato: "NATO_DIGITAL",
        tipo_assinatura: "INDIVIDUAL",
      }),
    ]);
    expect(body.etapas[1]!.transicoes).toEqual([
      expect.objectContaining({
        destino_ordem: 1,
        is_devolucao_diligencia: true,
        descricao_diligencia: "Completar o TR",
      }),
    ]);
    expect(router.push).toHaveBeenCalledWith("/atlas/procedimentos/wf-3");
  });

  it("remover a etapa de destino descarta a transição para ela", async () => {
    const backend = mockBackend({
      ...base(["atlas:manage"]),
      "GET v1/atlas/admin/workflows/wf-1": { data: { ...BASE, classificacao: CLASSIFICACAO } },
      "POST v1/atlas/admin/workflows/wf-1/versoes": {
        status: 201,
        data: { ...DETALHE, id: "wf-3" },
      },
    });
    renderAtlas(<ProcedimentoPage />);

    await userEvent.click(await screen.findByRole("button", { name: "Nova versão" }));
    const d = within(await screen.findByRole("dialog"));
    expect(d.getByLabelText("Classificação TTDD *")).toHaveValue("2.0.02.00.07");
    await userEvent.click(d.getByRole("button", { name: "Remover etapa 1" }));
    expect(d.getByText("Pendência — removida (a etapa de destino saiu)")).toBeInTheDocument();
    await userEvent.click(d.getByLabelText(/Salvar como rascunho/));
    await userEvent.click(d.getByRole("button", { name: "Publicar nova versão" }));
    const body = backend.to("POST v1/atlas/admin/workflows/wf-1/versoes")[0]!.body as {
      etapas: unknown[];
    };
    expect(body.etapas).toEqual([
      expect.objectContaining({ ordem: 1, nome_setor: "Licitações", transicoes: [] }),
    ]);
  });

  it("série revogada: aviso no lugar do selo e histórico com os prazos anteriores", async () => {
    resetNavigation({ codigo: "2.0.02.00.07" }, "/atlas/ttdd/2.0.02.00.07");
    mockBackend({
      ...base([]),
      "GET v1/atlas/ttdd/2.0.02.00.07": { data: REVOGADA },
      "GET v1/atlas/ttdd/2.0.02.00.07/historico": {
        data: [
          {
            evento: "REVOGADA",
            anterior: { ...CLASSIFICACAO, fase_interm_anos: 4 },
            edicao_diario: "6.500",
            registrado_em: "2027-05-10T12:00:00Z",
          },
          {
            evento: "ALTERADA",
            anterior: { ...CLASSIFICACAO, fase_interm_anos: 2, destinacao_final: "ELIMINACAO" },
            edicao_diario: "",
            registrado_em: "2026-01-10T12:00:00Z",
          },
        ],
      },
    });
    renderAtlas(<SeriePage />);

    expect(await screen.findByText(/não está na TTDD em vigor./)).toHaveTextContent(
      "Série revogada em 10/05/2027 (Diário Oficial nº 6.500): não está na TTDD em vigor.",
    );
    const hist = screen.getByRole("region", { name: "Histórico" });
    expect(await within(hist).findByText("Série revogada")).toBeInTheDocument();
    expect(within(hist).getByText(/Diário Oficial nº 6\.500/)).toBeInTheDocument();
    expect(within(hist).getByText("Prazos ou descrição alterados")).toBeInTheDocument();
    expect(within(hist).getByText(/intermediária 2 anos · eliminação/)).toBeInTheDocument();
  });

  it("série sem alterações e cartão de procedimento com série revogada", async () => {
    resetNavigation({ codigo: "2.0.02.00.07" }, "/atlas/ttdd/2.0.02.00.07");
    mockBackend({
      ...base([]),
      "GET v1/atlas/ttdd/2.0.02.00.07": { data: CLASSIFICACAO },
      "GET v1/atlas/workflows": page([{ ...RESUMO, classificacao: REVOGADA }]),
    });
    renderAtlas(<SeriePage />);
    expect(await screen.findByText("Sem alterações registradas")).toBeInTheDocument();
    expect(await screen.findByText("Série revogada")).toBeInTheDocument();
  });
});
