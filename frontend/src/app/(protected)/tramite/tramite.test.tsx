import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, moduleStatus, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import TramitePage from "./page";
import ProcessoPage from "./[id]/page";

const ORG = [
  {
    id: "e1",
    nome: "Órgão",
    ativo: true,
    unidades: [
      { id: "u1", nome: "Protocolo", sigla: "PROT", ativo: true, departamentos: [] },
      { id: "u2", nome: "Jurídico", sigla: "", ativo: true, departamentos: [] },
      { id: "u3", nome: "Extinta", sigla: "EXT", ativo: false, departamentos: [] },
    ],
  },
  {
    // órgão desativado: as unidades dele não servem, mesmo ativas (a API recusa)
    id: "e2",
    nome: "Órgão extinto",
    ativo: false,
    unidades: [
      { id: "u4", nome: "Unidade do órgão extinto", sigla: "UOE", ativo: true, departamentos: [] },
    ],
  },
];

const processo = (extra: Record<string, unknown> = {}) => ({
  id: "p1",
  numero: "00001.000001/2026-01",
  tipo_id: "t1",
  tipo: "Requerimento",
  assunto: "Compra de material",
  interessado: "Fulano",
  descricao: "Detalhes\ndo pedido",
  sigilo: "publico",
  status: "aberto",
  unidade_origem_id: "u1",
  unidade_origem: "PROT",
  unidade_atual_id: "u1",
  unidade_atual: "PROT",
  created_by_name: "Maria",
  created_at: "2026-01-01T10:00:00Z",
  updated_at: "2026-01-02T10:00:00Z",
  documentos: [],
  movimentos: [],
  acessos: [],
  can_act: true,
  can_route: true,
  ...extra,
});
const doc = (id: string, extra: Record<string, unknown> = {}) => ({
  id,
  processo_id: "p1",
  tipo: "Ofício",
  titulo: `Documento ${id}`,
  origem: "redigido",
  conteudo: "# Texto",
  size_bytes: 0,
  status: "rascunho",
  created_at: "2026-01-01T10:00:00Z",
  ...extra,
});

describe("Trâmite — processos", () => {
  beforeEach(() => resetNavigation());

  it("lista com filtros; sem permissão de abrir", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["tramite:read"] }),
      "GET v1/tramite/processos": (req) =>
        req.query.get("status") === "arquivado"
          ? fail(500, "erro no banco")
          : page([
              processo(),
              processo({
                id: "p2",
                numero: "N2",
                sigilo: "sigiloso",
                interessado: "",
                status: "concluido",
              }),
            ]),
    });
    renderApp(<TramitePage />);
    expect(await screen.findByRole("link", { name: /00001.000001\/2026-01/ })).toHaveAttribute(
      "href",
      "/tramite/p1",
    );
    expect(screen.getByText("Requerimento · Fulano")).toBeInTheDocument();
    expect(screen.getByText("Sigiloso")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Abrir processo/ })).not.toBeInTheDocument();
    expect(api.to("GET v1/tramite/processos")[0]!.query.get("minha_caixa")).toBe("true");

    await userEvent.click(screen.getByLabelText("Só na minha unidade"));
    await waitFor(() =>
      expect(api.to("GET v1/tramite/processos").at(-1)!.query.get("minha_caixa")).toBeNull(),
    );
    await userEvent.type(screen.getByLabelText("Número, assunto ou interessado"), " compra ");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    await waitFor(() =>
      expect(api.to("GET v1/tramite/processos").at(-1)!.query.get("q")).toBe("compra"),
    );
    await userEvent.selectOptions(screen.getByLabelText("Situação"), "arquivado");
    expect(await screen.findByText("erro no banco")).toBeInTheDocument();
  });

  it("abre processo já na unidade do usuário e vai para ele", async () => {
    const api = mockBackend({
      ...identityRoutes({
        scopes: [{ perfil: "servidor", unidade_id: "u2", origem: "manual", permissions: [] }],
      }),
      "GET v1/tramite/processos": page([]),
      "GET v1/tramite/tipos": {
        data: [{ id: "t1", slug: "req", nome: "Requerimento", descricao: "" }],
      },
      "GET v1/iam/org-tree": { data: ORG },
      "POST v1/tramite/processos": { data: processo({ id: "novo" }) },
    });
    renderApp(<TramitePage />);
    expect(await screen.findByText("Nenhum processo")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Abrir processo/ }));
    const dialog = await screen.findByRole("dialog", { name: "Abrir processo" });
    await within(dialog).findByRole("option", { name: "Requerimento" });
    // unidade inativa não aparece; a do usuário vem selecionada
    await waitFor(() =>
      expect(within(dialog).getByLabelText("Unidade de origem *")).toHaveValue("u2"),
    );
    expect(within(dialog).queryByRole("option", { name: /Extinta/ })).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("option", { name: /órgão extinto/ })).not.toBeInTheDocument();
    expect(within(dialog).getByRole("option", { name: "PROT — Protocolo" })).toBeInTheDocument();

    await userEvent.selectOptions(within(dialog).getByLabelText("Tipo *"), "t1");
    await userEvent.selectOptions(within(dialog).getByLabelText("Nível de acesso *"), "restrito");
    await userEvent.type(within(dialog).getByLabelText("Assunto *"), " Diárias ");
    await userEvent.type(within(dialog).getByLabelText("Interessado"), "Setor X");
    await userEvent.type(within(dialog).getByLabelText("Descrição"), "texto");
    await userEvent.click(within(dialog).getByRole("button", { name: "Abrir processo" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/tramite/novo"));
    expect(api.to("POST v1/tramite/processos")[0]!.body).toEqual({
      tipo_id: "t1",
      assunto: "Diárias",
      interessado: "Setor X",
      descricao: "texto",
      sigilo: "restrito",
      unidade_origem_id: "u2",
      // Sem procedimento do Atlas escolhido (ADR 026).
      atlas_procedimento_id: null,
      codigo_ttdd: "",
    });
  });
});

describe("Trâmite — processo", () => {
  beforeEach(() => resetNavigation({ id: "p1" }));

  it("mostra dados, documentos e andamento; tramita para outra unidade", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/tramite/processos/p1": {
        data: processo({
          sigilo: "sigiloso",
          status: "em_tramitacao",
          concluido_at: undefined,
          documentos: [doc("d1"), doc("d2", { origem: "anexo", status: "assinado", tipo: "" })],
          movimentos: [
            {
              id: "m1",
              acao: "abertura",
              despacho: "",
              created_at: "2026-01-01T10:00:00Z",
              actor_name: "Maria",
            },
            {
              id: "m2",
              acao: "tramitacao_recebida",
              de_unidade: "PROT",
              para_unidade: "Jurídico",
              despacho: "Para análise",
              created_at: "2026-01-02T10:00:00Z",
            },
            {
              id: "m3",
              acao: "x",
              para_unidade: "PROT",
              despacho: "",
              created_at: "2026-01-03T10:00:00Z",
            },
          ],
        }),
      },
      "GET v1/iam/org-tree": { data: ORG },
      "POST v1/tramite/processos/p1/tramitar": { data: null },
    });
    renderApp(<ProcessoPage />);
    expect(await screen.findByRole("heading", { name: "Compra de material" })).toBeInTheDocument();
    expect(screen.getByText("Em tramitação")).toBeInTheDocument();
    expect(screen.getByText(/Detalhes/)).toBeInTheDocument();
    expect(screen.getByText("tramitacao recebida")).toBeInTheDocument();
    expect(screen.getByText(/PROT → Jurídico/)).toBeInTheDocument();
    expect(screen.getByText(/sistema · → PROT/)).toBeInTheDocument();
    expect(screen.getByText("Somente as unidades envolvidas.")).toBeInTheDocument();
    // anexo assinado não se edita nem se assina
    const d2 = screen.getByText("Documento d2").closest("li")!;
    expect(within(d2).queryByRole("button", { name: "Editar" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Tramitar/ }));
    const dialog = await screen.findByRole("dialog", { name: "Tramitar processo" });
    expect(within(dialog).getByRole("note")).toHaveTextContent("sigiloso");
    // a unidade atual não é destino
    await within(dialog).findByRole("option", { name: "Jurídico" });
    expect(within(dialog).queryByRole("option", { name: /Protocolo/ })).not.toBeInTheDocument();
    await userEvent.selectOptions(within(dialog).getByLabelText("Unidade de destino *"), "u2");
    await userEvent.type(within(dialog).getByLabelText("Despacho *"), "Encaminho para parecer");
    await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
    await waitFor(() => expect(api.to("POST v1/tramite/processos/p1/tramitar")).toHaveLength(1));
    expect(api.to("POST v1/tramite/processos/p1/tramitar")[0]!.body).toEqual({
      despacho: "Encaminho para parecer",
      para_unidade_id: "u2",
    });
    await waitFor(() =>
      expect(screen.queryByRole("dialog", { name: "Tramitar processo" })).not.toBeInTheDocument(),
    );
  });

  it("concluir, arquivar e reabrir conforme a situação", async () => {
    let status = "aberto";
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/tramite/processos/p1": () => ({
        data: processo({
          status,
          concluido_at: status === "aberto" ? undefined : "2026-02-01T10:00:00Z",
        }),
      }),
      "POST v1/tramite/processos/p1/concluir": () => ((status = "concluido"), { data: null }),
      "POST v1/tramite/processos/p1/arquivar": () => ((status = "arquivado"), { data: null }),
      "POST v1/tramite/processos/p1/reabrir": () => ((status = "aberto"), { data: null }),
    });
    renderApp(<ProcessoPage />);
    const act = async (label: string, title: string) => {
      await userEvent.click(await screen.findByRole("button", { name: new RegExp(label) }));
      const dialog = await screen.findByRole("dialog", { name: title });
      expect(within(dialog).queryByLabelText("Unidade de destino *")).not.toBeInTheDocument();
      await userEvent.type(within(dialog).getByLabelText("Despacho *"), "ok ok");
      await userEvent.click(within(dialog).getByRole("button", { name: "Confirmar" }));
      await waitFor(() =>
        expect(screen.queryByRole("dialog", { name: title })).not.toBeInTheDocument(),
      );
    };
    await act("Concluir", "Concluir processo");
    expect(await screen.findByText("Concluído", { selector: "dt" })).toBeInTheDocument();
    await act("Arquivar", "Arquivar processo");
    await act("Reabrir", "Reabrir processo");
    expect(api.to("POST v1/tramite/processos/p1/reabrir")[0]!.body).toEqual({ despacho: "ok ok" });
    expect(screen.queryByText(/Novo documento/)).not.toBeNull();
  });

  it("sem atuação: nada de ações nem documentos novos", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["tramite:read"] }),
      "GET v1/tramite/processos/p1": {
        data: processo({
          status: "concluido",
          can_act: false,
          can_route: false,
          sigilo: "restrito",
          acessos: [{ user_id: "u9", name: "Ana", granted_at: "2026-01-01T10:00:00Z" }],
        }),
      },
    });
    renderApp(<ProcessoPage />);
    expect(await screen.findByText("Nenhum documento.")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: /Tramitar|Concluir|Arquivar|Reabrir|Novo documento|Conceder/,
      }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Ana")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Revogar acesso de Ana" })).not.toBeInTheDocument();
  });

  it("documento redigido: cria, edita, vê e pede assinatura", async () => {
    const api = mockBackend({
      ...identityRoutes({}, [moduleStatus("directory")]),
      "GET v1/tramite/processos/p1": { data: processo({ documentos: [doc("d1")] }) },
      "POST v1/tramite/processos/p1/documentos": { data: doc("d9") },
      "PUT v1/tramite/documentos/d1": { data: doc("d1") },
      "GET v1/tramite/documentos/d1": {
        data: doc("d1", { sha256: "abc123", envelope_id: "env1" }),
      },
      "GET v1/directory/people": {
        data: [{ user_id: "s1", name: "", username: "joao", job_title: "Analista", unidade: "TI" }],
        meta: {},
      },
      "POST v1/tramite/documentos/d1/assinatura": { data: null },
    });
    renderApp(<ProcessoPage />);

    await userEvent.click(await screen.findByRole("button", { name: /Novo documento/ }));
    let dialog = await screen.findByRole("dialog", { name: "Novo documento" });
    await userEvent.type(within(dialog).getByLabelText("Título *"), "Parecer");
    await userEvent.type(within(dialog).getByLabelText("Tipo"), "Parecer");
    await userEvent.type(
      within(dialog).getByLabelText("Conteúdo (Markdown)"),
      "Opino pelo deferimento",
    );
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar documento" }));
    await waitFor(() => expect(api.to("POST v1/tramite/processos/p1/documentos")).toHaveLength(1));
    expect(api.to("POST v1/tramite/processos/p1/documentos")[0]!.body).toEqual({
      tipo: "Parecer",
      titulo: "Parecer",
      conteudo: "Opino pelo deferimento",
      object_key: "",
    });

    await userEvent.click(await screen.findByRole("button", { name: "Editar" }));
    dialog = await screen.findByRole("dialog", { name: "Editar documento" });
    expect(within(dialog).queryByRole("radiogroup")).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText("Tipo")).toBeDisabled();
    await userEvent.clear(within(dialog).getByLabelText("Conteúdo (Markdown)"));
    await userEvent.type(within(dialog).getByLabelText("Conteúdo (Markdown)"), "novo");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar documento" }));
    await waitFor(() =>
      expect(api.to("PUT v1/tramite/documentos/d1")[0]?.body).toEqual({
        titulo: "Documento d1",
        conteudo: "novo",
      }),
    );

    await userEvent.click(await screen.findByRole("button", { name: /Documento d1/ }));
    dialog = await screen.findByRole("dialog", { name: "Documento d1" });
    expect(await within(dialog).findByText("SHA-256: abc123")).toBeInTheDocument();
    expect(within(dialog).getByRole("heading", { name: "Texto" })).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: /Ver envelope/ })).toHaveAttribute(
      "href",
      "/signum/env1",
    );
    await userEvent.keyboard("{Escape}");

    await userEvent.click(screen.getByRole("button", { name: /Assinar/ }));
    dialog = await screen.findByRole("dialog", { name: "Solicitar assinatura" });
    const pedir = within(dialog).getByRole("button", { name: /Solicitar assinatura/ });
    expect(pedir).toBeDisabled();
    await userEvent.type(within(dialog).getByLabelText("Signatários *"), "jo");
    await userEvent.click(await within(dialog).findByRole("button", { name: /joao/ }));
    await userEvent.click(within(dialog).getByLabelText("Assinatura sequencial"));
    await userEvent.click(pedir);
    await waitFor(() =>
      expect(api.to("POST v1/tramite/documentos/d1/assinatura")[0]?.body).toEqual({
        signer_ids: ["s1"],
        sequential: true,
      }),
    );
  });

  it("documento anexado: envia ao armazenamento e baixa depois", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/tramite/processos/p1": {
        data: processo({ documentos: [doc("d3", { origem: "anexo", conteudo: "" })] }),
      },
      "POST v1/tramite/processos/p1/uploads": {
        data: {
          object_key: "tramite/p1/a.pdf",
          upload_url: "http://minio.test/b/a.pdf",
          method: "PUT",
          headers: {},
          expires_at: "",
        },
      },
      "PUT minio.test/*": { status: 200 },
      "POST v1/tramite/processos/p1/documentos": { data: doc("d4") },
      "GET v1/tramite/documentos/d3": {
        data: doc("d3", {
          origem: "anexo",
          download_url: "https://minio/a.pdf",
          size_bytes: 3000,
          tipo: "",
        }),
      },
    });
    renderApp(<ProcessoPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Novo documento/ }));
    const dialog = await screen.findByRole("dialog", { name: "Novo documento" });
    await userEvent.click(within(dialog).getByLabelText(/Anexar arquivo/));
    await userEvent.type(within(dialog).getByLabelText("Título *"), "Nota fiscal");
    await userEvent.upload(
      within(dialog).getByLabelText("Arquivo *"),
      new File(["%PDF"], "a.pdf", { type: "application/pdf" }),
    );
    // o jsdom não enxerga o arquivo do userEvent.upload na validação de "required"
    within(dialog).getByLabelText("Arquivo *").removeAttribute("required");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar documento" }));
    await waitFor(() => expect(api.to("POST v1/tramite/processos/p1/documentos")).toHaveLength(1));
    expect(api.to("POST v1/tramite/processos/p1/documentos")[0]!.body).toEqual({
      tipo: "",
      titulo: "Nota fiscal",
      conteudo: "",
      object_key: "tramite/p1/a.pdf",
    });

    // anexo em rascunho: assina, mas não edita
    const d3 = screen.getByText("Documento d3").closest("li")!;
    expect(within(d3).queryByRole("button", { name: "Editar" })).not.toBeInTheDocument();
    await userEvent.click(within(d3).getByRole("button", { name: /Documento d3/ }));
    expect(await screen.findByRole("link", { name: /Baixar \(2.9 KB\)/ })).toHaveAttribute(
      "href",
      "https://minio/a.pdf",
    );
  });

  it("anexo sem arquivo é recusado antes de qualquer envio", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/tramite/processos/p1": { data: processo() },
    });
    renderApp(<ProcessoPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Novo documento/ }));
    const dialog = await screen.findByRole("dialog", { name: "Novo documento" });
    await userEvent.click(within(dialog).getByLabelText(/Anexar arquivo/));
    await userEvent.type(within(dialog).getByLabelText("Título *"), "Sem arquivo");
    within(dialog).getByLabelText("Arquivo *").removeAttribute("required");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar documento" }));
    expect(await screen.findByText("Operação não concluída")).toBeInTheDocument();
    expect(api.to("POST v1/tramite/processos/p1/uploads")).toHaveLength(0);
  });

  it("acesso nominal em processo restrito: concede (sem Diretório, por id) e revoga", async () => {
    const uuid = "11111111-2222-3333-4444-555555555555";
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/tramite/processos/p1": {
        data: processo({
          sigilo: "restrito",
          acessos: [{ user_id: "u9", name: "Ana", granted_at: "2026-01-01T10:00:00Z" }],
        }),
      },
      "POST v1/tramite/processos/p1/acessos": { data: null },
      "DELETE v1/tramite/processos/p1/acessos/u9": { data: null },
    });
    renderApp(<ProcessoPage />);
    const input = await screen.findByLabelText("Conceder acesso a");
    expect(input).toHaveAttribute("placeholder", "Id (UUID) do usuário");
    await userEvent.type(input, "nao-e-uuid{Enter}");
    expect(screen.getByRole("button", { name: "Adicionar" })).toBeDisabled();
    await userEvent.clear(input);
    await userEvent.type(input, `${uuid}{Enter}`);
    await userEvent.type(input, "22222222-2222-3333-4444-555555555555");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar" }));
    // adicionar de novo o mesmo não duplica; remover tira da lista
    await userEvent.type(input, `${uuid}{Enter}`);
    expect(screen.getAllByText(uuid)).toHaveLength(1);
    await userEvent.click(
      screen.getByRole("button", { name: "Remover 22222222-2222-3333-4444-555555555555" }),
    );

    await userEvent.click(screen.getByRole("button", { name: /Conceder/ }));
    await waitFor(() => expect(api.to("POST v1/tramite/processos/p1/acessos")).toHaveLength(1));
    expect(api.to("POST v1/tramite/processos/p1/acessos")[0]!.body).toEqual({ user_id: uuid });

    await userEvent.click(screen.getByRole("button", { name: "Revogar acesso de Ana" }));
    const dialog = await screen.findByRole("dialog", { name: "Revogar o acesso de Ana?" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Revogar" }));
    await waitFor(() =>
      expect(api.to("DELETE v1/tramite/processos/p1/acessos/u9")).toHaveLength(1),
    );
  });

  it("processo inexistente ou sem acesso", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/tramite/processos/p1": fail(404, "processo não encontrado"),
    });
    renderApp(<ProcessoPage />);
    expect(await screen.findByText("processo não encontrado")).toBeInTheDocument();
  });
});
