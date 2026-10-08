import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChecklistDocumentos } from "@/components/atlas/LinhaDoTempo";
import { ModelosDaSerie } from "@/components/atlas/ModelosDaSerie";
import { NovoProcedimentoForm } from "@/components/atlas/NovoProcedimentoForm";
import { extensao, sugerirModelo, urlArquivoModelo } from "@/lib/atlas/modelos";
import type { Etapa, ModeloDocumento } from "@/lib/nexus/types";
import { identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import ModelosPage from "./page";

async function fill(el: HTMLElement, text: string) {
  await userEvent.click(el);
  await userEvent.paste(text);
}

const versao = (n: number, extra: Record<string, unknown> = {}) => ({
  versao: n,
  arquivo_nome: n === 1 ? "Requerimento.docx" : "Requerimento v2.docx",
  content_type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  tamanho: 2048,
  sha256: "a".repeat(64),
  nota: n === 1 ? "" : "Novo cabeçalho",
  created_at: "2026-10-01T00:00:00Z",
  created_by: "maria",
  ...extra,
});

const REQUERIMENTO: ModeloDocumento = {
  id: "mod-1",
  nome: "Requerimento",
  descricao: "Pedido inicial do interessado",
  ativo: true,
  atual: versao(2),
  updated_at: "2026-10-02T00:00:00Z",
  updated_by: "maria",
  pecas_ligadas: 3,
};

const OFICIO: ModeloDocumento = {
  ...REQUERIMENTO,
  id: "mod-2",
  nome: "Ofício padrão",
  descricao: "",
  ativo: false,
  atual: { ...versao(1), arquivo_nome: "oficio.pdf" },
  pecas_ligadas: 0,
};

const rotas = (perms: string[]) => ({
  ...identityRoutes({
    permissions: perms,
    scopes: [{ perfil: "p", origem: "manual", permissions: perms }],
  }),
  "GET v1/atlas/modelos": { data: [REQUERIMENTO] },
  "GET v1/atlas/admin/modelos": { data: [REQUERIMENTO, OFICIO] },
  "GET v1/atlas/modelos/mod-1": { data: { ...REQUERIMENTO, versoes: [versao(2), versao(1)] } },
});

const docx = (bytes = 10) =>
  new File(["x".repeat(bytes)], "Requerimento.docx", {
    type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  });

describe("Atlas — biblioteca de modelos", () => {
  beforeEach(() => resetNavigation({}, "/atlas/modelos"));

  it("consulta: só ativos, download da versão atual, histórico e busca; sem gestão", async () => {
    const backend = mockBackend(rotas(["atlas:read"]));
    renderApp(<ModelosPage />);

    const lista = await screen.findByRole("list", { name: "Modelos" });
    expect(within(lista).getByRole("heading", { name: "Requerimento" })).toBeInTheDocument();
    expect(
      within(lista).getByText(/DOCX · versão 2 · 2\.0 KB · atualizado em/),
    ).toBeInTheDocument();
    expect(within(lista).queryByText(/por maria/)).not.toBeInTheDocument();
    expect(within(lista).getByText("3 peças ligadas")).toBeInTheDocument();
    expect(within(lista).getByRole("link", { name: /Baixar Requerimento/ })).toHaveAttribute(
      "href",
      "/api/backend/v1/atlas/modelos/mod-1/arquivo",
    );
    expect(backend.to("GET v1/atlas/admin/modelos")).toHaveLength(0);
    expect(screen.queryByRole("button", { name: /Novo modelo/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Nova versão/ })).not.toBeInTheDocument();

    await userEvent.click(within(lista).getByRole("button", { name: /Versões/ }));
    expect(await screen.findByText("Novo cabeçalho")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Baixar a versão 1/ })).toHaveAttribute(
      "href",
      "/api/backend/v1/atlas/modelos/mod-1/arquivo?versao=1",
    );

    await fill(screen.getByLabelText("Procurar modelo"), "contrato");
    expect(await screen.findByText("Nenhum modelo encontrado")).toBeInTheDocument();
  });

  it("gestão: cadastrar, publicar versão e desativar", async () => {
    const backend = mockBackend({
      ...rotas(["atlas:manage"]),
      "POST v1/atlas/admin/modelos": { status: 201, data: REQUERIMENTO },
      "POST v1/atlas/admin/modelos/mod-1/versoes": { status: 201, data: REQUERIMENTO },
      "PUT v1/atlas/admin/modelos/mod-2": { data: OFICIO },
    });
    renderApp(<ModelosPage />);

    const lista = await screen.findByRole("list", { name: "Modelos" });
    expect(within(lista).getByText("Desativado")).toBeInTheDocument();
    // PDF tem "Visualizar" (abre no navegador); docx não.
    expect(within(lista).getByRole("link", { name: /Visualizar Ofício padrão/ })).toHaveAttribute(
      "href",
      "/api/backend/v1/atlas/modelos/mod-2/arquivo?inline=1",
    );
    expect(
      within(lista).queryByRole("link", { name: /Visualizar Requerimento/ }),
    ).not.toBeInTheDocument();
    expect(within(lista).getByText("Nenhuma peça ligada")).toBeInTheDocument();
    expect(within(lista).getAllByText(/por maria/)).toHaveLength(2);

    // Novo modelo: nome, arquivo e nota em multipart.
    await userEvent.click(screen.getByRole("button", { name: /Novo modelo/ }));
    let dialogo = await screen.findByRole("dialog", { name: "Novo modelo" });
    const cadastrar = within(dialogo).getByRole("button", { name: "Cadastrar modelo" });
    expect(cadastrar).toBeDisabled();
    await fill(within(dialogo).getByLabelText("Nome do modelo"), "Termo de referência");
    // Acima de 10 MB é recusado já no navegador.
    await userEvent.upload(
      within(dialogo).getByLabelText("Arquivo do modelo"),
      docx(10 * 1024 * 1024 + 1),
    );
    expect(within(dialogo).getByRole("alert")).toHaveTextContent("passa de 10 MB");
    expect(cadastrar).toBeDisabled();
    await userEvent.upload(within(dialogo).getByLabelText("Arquivo do modelo"), docx());
    await fill(within(dialogo).getByLabelText("Nota (opcional)"), "Aprovado pela PGM");
    await userEvent.click(cadastrar);
    const form = backend.to("POST v1/atlas/admin/modelos")[0]!.body as FormData;
    expect(form.get("nome")).toBe("Termo de referência");
    expect(form.get("nota")).toBe("Aprovado pela PGM");
    expect((form.get("arquivo") as File).name).toBe("Requerimento.docx");

    // Nova versão: só arquivo e o que mudou.
    const cartao = (nome: string) =>
      within(lista).getByRole("heading", { name: nome }).closest("li") as HTMLElement;
    await userEvent.click(
      within(cartao("Requerimento")).getByRole("button", { name: /Nova versão/ }),
    );
    dialogo = await screen.findByRole("dialog", { name: "Nova versão — Requerimento" });
    expect(within(dialogo).queryByLabelText("Nome do modelo")).not.toBeInTheDocument();
    await userEvent.upload(within(dialogo).getByLabelText("Arquivo do modelo"), docx());
    await fill(within(dialogo).getByLabelText("O que mudou nesta versão"), "Novo cabeçalho");
    await userEvent.click(within(dialogo).getByRole("button", { name: "Publicar versão" }));
    const v = backend.to("POST v1/atlas/admin/modelos/mod-1/versoes")[0]!.body as FormData;
    expect(v.get("nota")).toBe("Novo cabeçalho");
    expect(v.get("nome")).toBeNull();

    // Editar: reativar o modelo desativado.
    await userEvent.click(within(cartao("Ofício padrão")).getByRole("button", { name: /Editar/ }));
    dialogo = await screen.findByRole("dialog", { name: "Editar modelo" });
    expect(within(dialogo).getByText(/segue baixável onde já está ligado/)).toBeInTheDocument();
    await userEvent.click(within(dialogo).getByRole("switch", { name: "Modelo ativo" }));
    await userEvent.click(within(dialogo).getByRole("button", { name: "Salvar" }));
    expect(backend.to("PUT v1/atlas/admin/modelos/mod-2")[0]!.body).toEqual({
      nome: "Ofício padrão",
      descricao: "",
      ativo: true,
    });
  });
});

describe("Atlas — modelos nas peças", () => {
  const etapa = (documentos: Etapa["documentos"]): Etapa => ({
    id: "et-1",
    ordem: 1,
    unidade_administrativa: "GAB",
    nome_setor: "Gabinete",
    atribuicoes_setor: "Solicitar",
    prazo_sla_em_dias: 1,
    manter_aberto_apos_remessa: false,
    documentos,
    transicoes: [],
  });
  const peca = {
    id: "d-1",
    nome_documento: "Requerimento",
    obrigatorio: true,
    formato: "NATO_DIGITAL" as const,
    tipo_assinatura: "INDIVIDUAL" as const,
    exige_conferencia_copia: false,
    modelo_minuta_padrao_url: "",
    modelo_id: "mod-1",
    modelo: { id: "mod-1", nome: "Requerimento", versao: 2, arquivo_nome: "Requerimento v2.docx" },
  };

  it("checklist: baixar o modelo da biblioteca (versão atual)", () => {
    render(
      <ChecklistDocumentos
        etapas={[
          etapa([
            peca,
            { ...peca, id: "d-2", nome_documento: "Anexo", modelo_id: null, modelo: undefined },
          ]),
        ]}
      />,
    );
    const link = screen.getByRole("link", { name: /Baixar modelo Requerimento, versão 2/ });
    expect(link).toHaveAttribute("href", "/api/backend/v1/atlas/modelos/mod-1/arquivo");
    expect(link).toHaveAttribute("download", "Requerimento v2.docx");
    expect(screen.getAllByRole("link")).toHaveLength(1);
  });

  it("formulário: liga pelo nome, mantém a escolha anterior e permite tirar o modelo", async () => {
    resetNavigation({}, "/atlas");
    const backend = mockBackend({
      ...rotas(["atlas:manage"]),
      "GET v1/atlas/ttdd": page([]),
      "POST v1/atlas/admin/workflows/wf-1/versoes": { status: 201, data: { id: "wf-2" } },
    });
    const base = {
      id: "wf-1",
      codigo_processual: "ADM.X.1",
      titulo: "T",
      objetivo: "O",
      publico_alvo: "P",
      versao: 1,
      ativo: true,
      situacao: "HOMOLOGADO" as const,
      nivel_acesso: "PUBLICO" as const,
      hipotese_legal_restricao: "",
      codigo_ttdd: "2.0.02.00.07",
      total_etapas: 1,
      // A peça "Ofício padrão" já estava ligada ao modelo, hoje desativado.
      etapas: [
        etapa([{ ...peca, id: "d-3", nome_documento: "Ofício padrão", modelo_id: "mod-2" }]),
      ],
      created_at: "",
      updated_at: "",
    };
    renderApp(<NovoProcedimentoForm base={base} onDone={() => {}} />);

    await fill(
      screen.getAllByLabelText("Peças exigidas (uma por linha)")[0]!,
      "\nrequerimento\nAnexo",
    );
    const modelo = (peca: string) => screen.getByRole("combobox", { name: peca });
    // "requerimento" casa com o modelo "Requerimento" (sem caixa/acento).
    await vi.waitFor(() => expect(modelo("requerimento")).toHaveValue("mod-1"));
    expect(modelo("Anexo")).toHaveValue("");
    expect(modelo("Ofício padrão")).toHaveValue("mod-2");
    expect(
      within(modelo("Ofício padrão")).getByRole("option", { name: "Ofício padrão (desativado)" }),
    ).toBeInTheDocument();
    expect(
      within(modelo("Anexo")).queryByRole("option", { name: /desativado/ }),
    ).not.toBeInTheDocument();
    await userEvent.selectOptions(modelo("requerimento"), "");
    await userEvent.selectOptions(modelo("Anexo"), "mod-1");

    await fill(screen.getByLabelText("Classificação TTDD *"), "2.0.02.00.07");
    await userEvent.click(screen.getByRole("button", { name: "Salvar rascunho" }));
    const body = backend.to("POST v1/atlas/admin/workflows/wf-1/versoes")[0]!.body as {
      etapas: { documentos: { nome_documento: string; modelo_id: string | null }[] }[];
    };
    expect(body.etapas[0]!.documentos.map((d) => [d.nome_documento, d.modelo_id])).toEqual([
      ["Ofício padrão", "mod-2"],
      ["requerimento", null],
      ["Anexo", "mod-1"],
    ]);
  });

  it("utilitários", () => {
    expect(urlArquivoModelo("a b", 3)).toBe("/api/backend/v1/atlas/modelos/a%20b/arquivo?versao=3");
    expect(extensao("x.tar.gz")).toBe("GZ");
    expect(extensao("sem")).toBe("");
    expect(sugerirModelo("  ", [REQUERIMENTO])).toBeUndefined();
    expect(sugerirModelo("OFÍCIO PADRAO", [OFICIO])).toBe(OFICIO);
  });
});

describe("Atlas — detalhe da peça", () => {
  const pecaCom = (extra: Record<string, unknown> = {}) => ({
    id: "d-1",
    nome_documento: "Requerimento",
    obrigatorio: true,
    formato: "NATO_DIGITAL" as const,
    tipo_assinatura: "INDIVIDUAL" as const,
    exige_conferencia_copia: false,
    modelo_minuta_padrao_url: "",
    modelo_id: "mod-1" as string | null,
    modelo: {
      id: "mod-1",
      nome: "Requerimento",
      versao: 2,
      arquivo_nome: "Requerimento v2.docx",
    } as { id: string; nome: string; versao: number; arquivo_nome: string } | undefined,
    ...extra,
  });
  const etapas = (...docs: ReturnType<typeof pecaCom>[]): Etapa[] => [
    {
      id: "et-1",
      ordem: 1,
      unidade_administrativa: "GAB",
      nome_setor: "Gabinete",
      atribuicoes_setor: "Solicitar",
      prazo_sla_em_dias: 1,
      manter_aberto_apos_remessa: false,
      documentos: docs,
      transicoes: [],
    },
  ];
  const semModelo = pecaCom({
    id: "d-2",
    nome_documento: "Termo de referência",
    modelo_id: null,
    modelo: undefined,
  });
  const abrir = async (peca: string) => {
    await userEvent.click(screen.getByRole("button", { name: `Detalhes da peça ${peca}` }));
  };

  beforeEach(() => resetNavigation({ id: "wf-1" }, "/atlas/procedimentos/wf-1"));

  it("peça sem modelo próprio oferece o modelo da série do procedimento", async () => {
    const backend = mockBackend({
      ...rotas(["atlas:read"]),
      // Só os ativos valem para a peça.
      "GET v1/atlas/ttdd/2.0.02.00.07/modelos": { data: [REQUERIMENTO, OFICIO] },
    });
    renderApp(
      <ChecklistDocumentos etapas={etapas(pecaCom(), semModelo)} codigoTTDD="2.0.02.00.07" />,
    );
    const herdado = await screen.findByRole("link", {
      name: "Baixar modelo Requerimento (modelo da série 2.0.02.00.07)",
    });
    expect(herdado).toHaveAttribute("href", "/api/backend/v1/atlas/modelos/mod-1/arquivo");
    expect(backend.to("GET v1/atlas/ttdd/2.0.02.00.07/modelos")).toHaveLength(1);

    await abrir("Termo de referência");
    expect(screen.getByText(/do procedimento:/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "2.0.02.00.07" })).toHaveAttribute(
      "href",
      "/atlas/ttdd/2.0.02.00.07",
    );
    expect(screen.queryByText("Modelo: Ofício padrão")).not.toBeInTheDocument();
    // A peça com modelo próprio continua com o dela (fora da lista da série).
    await abrir("Termo de referência");
    await abrir("Requerimento");
    expect(screen.getByText("Modelo: Requerimento")).toBeInTheDocument();
    expect(screen.queryByText(/do procedimento:/)).not.toBeInTheDocument();
  });

  it("vários modelos na série: o atalho abre o detalhe com todos", async () => {
    mockBackend({
      ...rotas(["atlas:read"]),
      "GET v1/atlas/ttdd/2.0.02.00.07/modelos": {
        data: [REQUERIMENTO, { ...OFICIO, ativo: true }],
      },
    });
    renderApp(<ChecklistDocumentos etapas={etapas(semModelo)} codigoTTDD="2.0.02.00.07" />);
    await userEvent.click(await screen.findByRole("button", { name: /2 modelos/ }));
    expect(screen.getByRole("link", { name: "Baixar o modelo Requerimento" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Baixar o modelo Ofício padrão" })).toBeInTheDocument();
  });

  it("consulta: o detalhe mostra o modelo para baixar, sem gestão", async () => {
    mockBackend(rotas(["atlas:read"]));
    renderApp(<ChecklistDocumentos etapas={etapas(pecaCom(), semModelo)} />);
    const detalhes = screen.getByRole("button", { name: "Detalhes da peça Requerimento" });
    expect(detalhes).toHaveAttribute("aria-expanded", "false");
    await abrir("Requerimento");
    expect(detalhes).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Modelo: Requerimento")).toBeInTheDocument();
    expect(screen.getByText(/Versão 2 · DOCX · Requerimento v2\.docx/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Baixar o modelo Requerimento" })).toHaveAttribute(
      "href",
      "/api/backend/v1/atlas/modelos/mod-1/arquivo",
    );
    expect(screen.queryByLabelText("Modelo da biblioteca")).not.toBeInTheDocument();
    await abrir("Termo de referência");
    expect(screen.getByText("Nenhum modelo cadastrado para esta peça.")).toBeInTheDocument();
  });

  it("gestão: trocar o modelo, enviar nova versão e criar modelo pela peça", async () => {
    const onAtualizado = vi.fn();
    const backend = mockBackend({
      ...rotas(["atlas:manage"]),
      "PUT v1/atlas/admin/workflows/wf-1/pecas/*": { data: { id: "wf-1" } },
      "POST v1/atlas/admin/modelos/mod-1/versoes": { status: 201, data: REQUERIMENTO },
      "POST v1/atlas/admin/modelos": { status: 201, data: { ...REQUERIMENTO, id: "mod-9" } },
    });
    renderApp(
      <ChecklistDocumentos
        etapas={etapas(pecaCom(), semModelo)}
        gestao={{ workflowId: "wf-1", onAtualizado }}
      />,
    );

    // Peça ligada: o desativado aparece só se for o atual; trocar por nenhum.
    await abrir("Requerimento");
    const select = await screen.findByLabelText("Modelo da biblioteca");
    await vi.waitFor(() => expect(within(select).getAllByRole("option")).toHaveLength(2));
    const salvar = screen.getByRole("button", { name: "Salvar" });
    expect(salvar).toBeDisabled();
    await userEvent.selectOptions(select, "");
    await userEvent.click(salvar);
    expect(backend.to("PUT v1/atlas/admin/workflows/wf-1/pecas/d-1/modelo")[0]!.body).toEqual({
      modelo_id: null,
    });
    expect(onAtualizado).toHaveBeenCalledWith({ id: "wf-1" });

    // Enviar arquivo na peça ligada: nova versão do modelo dela.
    expect(
      screen.getByText(/vale para todos os procedimentos que usam este modelo/),
    ).toBeInTheDocument();
    await userEvent.upload(screen.getByLabelText("Enviar nova versão do modelo"), docx());
    await userEvent.click(screen.getByRole("button", { name: /Enviar/ }));
    await vi.waitFor(() =>
      expect(backend.to("POST v1/atlas/admin/modelos/mod-1/versoes")).toHaveLength(1),
    );
    expect(
      (backend.to("POST v1/atlas/admin/modelos/mod-1/versoes")[0]!.body as FormData).get("nota"),
    ).toBe('Enviado pela peça "Requerimento"');

    // Peça sem modelo: cria o modelo com o nome da peça e liga.
    await abrir("Requerimento");
    await abrir("Termo de referência");
    expect(
      screen.getByText(/Cria o modelo "Termo de referência" na biblioteca/),
    ).toBeInTheDocument();
    await userEvent.upload(
      screen.getByLabelText("Enviar arquivo de modelo para esta peça"),
      docx(),
    );
    await userEvent.click(screen.getByRole("button", { name: /Enviar/ }));
    await vi.waitFor(() =>
      expect(backend.to("PUT v1/atlas/admin/workflows/wf-1/pecas/d-2/modelo")).toHaveLength(1),
    );
    expect((backend.to("POST v1/atlas/admin/modelos")[0]!.body as FormData).get("nome")).toBe(
      "Termo de referência",
    );
    expect(backend.to("PUT v1/atlas/admin/workflows/wf-1/pecas/d-2/modelo")[0]!.body).toEqual({
      modelo_id: "mod-9",
    });
  });

  it("gestão: arquivo acima de 10 MB é recusado no navegador", async () => {
    mockBackend(rotas(["atlas:manage"]));
    renderApp(
      <ChecklistDocumentos
        etapas={etapas(semModelo)}
        gestao={{ workflowId: "wf-1", onAtualizado: vi.fn() }}
      />,
    );
    await abrir("Termo de referência");
    await userEvent.upload(
      screen.getByLabelText("Enviar arquivo de modelo para esta peça"),
      docx(10 * 1024 * 1024 + 1),
    );
    expect(screen.getByRole("alert")).toHaveTextContent("passa de 10 MB");
    expect(screen.getByRole("button", { name: /Enviar/ })).toBeDisabled();
  });
});

describe("Atlas — modelos da série da TTDD", () => {
  const serie = "v1/atlas/ttdd/6.0.01.00.03/modelos";
  const admin = "v1/atlas/admin/ttdd/6.0.01.00.03/modelos";
  beforeEach(() => resetNavigation({ codigo: "6.0.01.00.03" }, "/atlas/ttdd/6.0.01.00.03"));

  it("consulta: modelos para baixar, desativado marcado, sem gestão", async () => {
    mockBackend({ ...rotas(["atlas:read"]), [`GET ${serie}`]: { data: [REQUERIMENTO, OFICIO] } });
    renderApp(
      <ModelosDaSerie
        codigo="6.0.01.00.03"
        descritor="Relatório técnico social e parecer"
        vigente
      />,
    );
    const lista = await screen.findByRole("list", { name: "Modelos da série" });
    expect(
      within(lista).getByRole("link", { name: "Baixar o modelo Requerimento" }),
    ).toHaveAttribute("href", "/api/backend/v1/atlas/modelos/mod-1/arquivo");
    expect(within(lista).getByText("Desativado")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Retirar/ })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Ligar um modelo da biblioteca")).not.toBeInTheDocument();
  });

  it("sem modelos: aviso e link para a biblioteca; série revogada não tem gestão", async () => {
    mockBackend({ ...rotas(["atlas:manage"]), [`GET ${serie}`]: { data: [] } });
    renderApp(<ModelosDaSerie codigo="6.0.01.00.03" descritor="Relatório" vigente={false} />);
    expect(await screen.findByText(/Nenhum modelo cadastrado para esta série/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ver a biblioteca de modelos" })).toHaveAttribute(
      "href",
      "/atlas/modelos",
    );
    expect(screen.queryByLabelText("Ligar um modelo da biblioteca")).not.toBeInTheDocument();
  });

  it("gestão: ligar da biblioteca, enviar modelo novo com o nome da série e retirar", async () => {
    const backend = mockBackend({
      ...rotas(["atlas:manage"]),
      [`GET ${serie}`]: { data: [] },
      [`POST ${admin}`]: { data: [REQUERIMENTO] },
      "POST v1/atlas/admin/modelos": { status: 201, data: { ...REQUERIMENTO, id: "mod-9" } },
      [`DELETE ${admin}/mod-1`]: { data: [] },
    });
    renderApp(
      <ModelosDaSerie
        codigo="6.0.01.00.03"
        descritor="Relatório técnico social e parecer"
        vigente
      />,
    );
    const select = await screen.findByLabelText("Ligar um modelo da biblioteca");
    // Só os ativos ainda não ligados.
    await vi.waitFor(() => expect(within(select).getAllByRole("option")).toHaveLength(2));
    await userEvent.selectOptions(select, "mod-1");
    await userEvent.click(screen.getByRole("button", { name: /Ligar/ }));
    expect(backend.to(`POST ${admin}`)[0]!.body).toEqual({ modelo_id: "mod-1" });
    expect(
      await screen.findByRole("link", { name: "Baixar o modelo Requerimento" }),
    ).toBeInTheDocument();

    expect(screen.getByLabelText("Ou enviar um modelo novo — nome")).toHaveValue(
      "Relatório técnico social e parecer",
    );
    await userEvent.upload(screen.getByLabelText("Arquivo do modelo"), docx());
    await userEvent.click(screen.getByRole("button", { name: /Enviar/ }));
    await vi.waitFor(() => expect(backend.to(`POST ${admin}`)).toHaveLength(2));
    expect((backend.to("POST v1/atlas/admin/modelos")[0]!.body as FormData).get("nome")).toBe(
      "Relatório técnico social e parecer",
    );
    expect(backend.to(`POST ${admin}`)[1]!.body).toEqual({ modelo_id: "mod-9" });

    await userEvent.click(
      screen.getByRole("button", { name: "Retirar o modelo Requerimento da série" }),
    );
    expect(await screen.findByText(/Nenhum modelo cadastrado para esta série/)).toBeInTheDocument();
    expect(backend.to(`DELETE ${admin}/mod-1`)).toHaveLength(1);
  });

  it("gestão: arquivo acima de 10 MB é recusado no navegador", async () => {
    mockBackend({ ...rotas(["atlas:manage"]), [`GET ${serie}`]: { data: [] } });
    renderApp(<ModelosDaSerie codigo="6.0.01.00.03" descritor="R" vigente />);
    await userEvent.upload(
      await screen.findByLabelText("Arquivo do modelo"),
      docx(10 * 1024 * 1024 + 1),
    );
    expect(screen.getByRole("alert")).toHaveTextContent("passa de 10 MB");
    expect(screen.getByRole("button", { name: /Enviar/ })).toBeDisabled();
  });
});

describe("Atlas — biblioteca pela Busca Global", () => {
  it("?q= da Busca Global já filtra a biblioteca", async () => {
    resetNavigation({}, "/atlas/modelos", "q=Ofício");
    mockBackend(rotas(["atlas:manage"]));
    renderApp(<ModelosPage />);
    expect(await screen.findByLabelText("Procurar modelo")).toHaveValue("Ofício");
    const lista = await screen.findByRole("list", { name: "Modelos" });
    expect(within(lista).getByRole("heading", { name: "Ofício padrão" })).toBeInTheDocument();
    expect(within(lista).queryByRole("heading", { name: "Requerimento" })).not.toBeInTheDocument();
  });
});
