import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, renderApp, type Reply } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import WikiLayout from "./layout";
import WikiHome from "./page";
import WikiPageView from "./[ref]/page";

const wp = (id: string, extra: Record<string, unknown> = {}) => ({
  id, slug: id, title: `Página ${id}`, position: 0, version: 1, updated_by_name: "Maria", updated_at: "2026-01-01T00:00:00Z", ...extra,
});
const TREE = [wp("raiz"), wp("filha", { parent_id: "raiz" }), wp("neta", { parent_id: "filha" }), wp("outra")];

describe("Wiki — layout e árvore", () => {
  beforeEach(() => resetNavigation({ ref: "filha" }));

  it("árvore aninhada, página atual e nova página já sob a atual", async () => {
    const api = mockBackend({ ...identityRoutes(), "GET v1/wiki/tree": { data: TREE }, "POST v1/wiki/pages": { data: wp("nova") } });
    renderApp(
      <WikiLayout>
        <WikiHome />
      </WikiLayout>,
    );
    const nav = await screen.findByRole("navigation", { name: "Páginas da wiki" });
    expect(within(nav).getByRole("link", { name: "Página filha" })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: "Página neta" })).toHaveAttribute("href", "/wiki/neta");
    expect(screen.getByText(/Escolha uma página na árvore/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Nova página" }));
    const d = await screen.findByRole("dialog", { name: "Nova página" });
    expect(within(d).getByLabelText("Página-mãe")).toHaveValue("filha");
    await userEvent.type(within(d).getByLabelText("Título *"), " Procedimentos ");
    await userEvent.type(within(d).getByLabelText("Conteúdo em Markdown"), "# Passo 1");
    await userEvent.click(within(d).getByRole("button", { name: "Pré-visualizar" }));
    expect(within(d).getByRole("heading", { name: "Passo 1" })).toBeInTheDocument();
    await userEvent.click(within(d).getByRole("button", { name: "Editar" }));
    await userEvent.type(within(d).getByLabelText("Resumo da alteração"), "primeira versão");
    await userEvent.click(within(d).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/wiki/nova"));
    expect(api.to("POST v1/wiki/pages")[0]!.body).toEqual({
      title: "Procedimentos", slug: "", parent_id: "filha", unidade_id: null, publico: { entidades: [], unidades: [] }, position: 0, summary: "primeira versão", body: "# Passo 1",
    });
  });

  it("wiki vazia", async () => {
    resetNavigation({});
    mockBackend({ ...identityRoutes(), "GET v1/wiki/tree": { data: [] } });
    renderApp(<WikiLayout>{null}</WikiLayout>);
    expect(await screen.findByText("Nenhuma página ainda.")).toBeInTheDocument();
  });
});

describe("Wiki — página", () => {
  beforeEach(() => resetNavigation({ ref: "filha" }));

  const detail = (extra: Record<string, unknown> = {}) => ({
    data: wp("filha", { parent_id: "raiz", body: "Conteúdo **forte**", version: 3, breadcrumbs: [wp("raiz")], ...extra }),
  });

  it("mostra trilha e conteúdo; edita (a mãe não pode ser a própria página nem descendente)", async () => {
    let slug = "filha";
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/wiki/pages/filha": detail(),
      "GET v1/wiki/tree": { data: TREE },
      "PUT v1/wiki/pages/filha": () => ({ data: wp("filha", { slug }) }),
    });
    renderApp(<WikiPageView />);
    expect(await screen.findByRole("heading", { name: "Página filha", level: 1 })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Trilha" })).toHaveTextContent("Página raiz");
    expect(screen.getByText("forte").tagName).toBe("STRONG");
    expect(screen.getByText(/Versão 3/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Editar/ }));
    let d = await screen.findByRole("dialog", { name: "Editar — Página filha" });
    const parent = within(d).getByLabelText("Página-mãe");
    await waitFor(() => expect(parent).toHaveValue("raiz"));
    expect(within(parent).queryByRole("option", { name: "Página filha" })).not.toBeInTheDocument();
    expect(within(parent).queryByRole("option", { name: "Página neta" })).not.toBeInTheDocument();
    expect(within(parent).getByRole("option", { name: "Página outra" })).toBeInTheDocument();
    await userEvent.click(within(d).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/wiki/pages/filha")).toHaveLength(1));
    // envia a versão aberta (conflito de edição é detectado pelo backend)
    expect(api.to("PUT v1/wiki/pages/filha")[0]!.body).toMatchObject({ version: 3, parent_id: "raiz", body: "Conteúdo **forte**" });
    expect(router.replace).not.toHaveBeenCalled();

    slug = "renomeada";
    await userEvent.click(screen.getByRole("button", { name: /Editar/ }));
    d = await screen.findByRole("dialog", { name: "Editar — Página filha" });
    await userEvent.click(within(d).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/wiki/renomeada"));
  });

  it("árvore que chega depois de abrir o editor não move a página para a raiz", async () => {
    let releaseTree: (r: Reply) => void = () => {};
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/wiki/pages/filha": detail(),
      "GET v1/wiki/tree": () => new Promise<Reply>((res) => (releaseTree = res)),
      "PUT v1/wiki/pages/filha": { data: wp("filha") },
    });
    renderApp(<WikiPageView />);
    await userEvent.click(await screen.findByRole("button", { name: /Editar/ }));
    const d = await screen.findByRole("dialog", { name: "Editar — Página filha" });
    releaseTree({ data: TREE });
    await waitFor(() => expect(within(d).getByLabelText("Página-mãe")).toHaveValue("raiz"));
    await userEvent.click(within(d).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/wiki/pages/filha")[0]?.body).toMatchObject({ parent_id: "raiz" }));
  });

  it("unidade dona (ADR 013): oferece as lotadas e a atual; sem wiki:manage não é obrigatória", async () => {
    const ORG = [
      { id: "e1", nome: "Órgão", unidades: ["u1", "u2", "u3"].map((id) => ({ id, entidade_id: "e1", nome: `Unidade ${id}`, departamentos: [] })) },
    ];
    const api = mockBackend({
      ...identityRoutes({ permissions: ["contact:read"], scopes: [{ perfil: "p", origem: "manual", entidade_id: "e1", unidade_id: "u1", permissions: ["contact:read"] }] }),
      "GET v1/wiki/pages/filha": detail({ unidade_id: "u2" }),
      "GET v1/wiki/tree": { data: TREE },
      "GET v1/iam/org-tree": { data: ORG },
      "PUT v1/wiki/pages/filha": { data: wp("filha") },
    });
    renderApp(<WikiPageView />);
    await userEvent.click(await screen.findByRole("button", { name: /Editar/ }));
    const d = await screen.findByRole("dialog", { name: "Editar — Página filha" });
    const dona = within(d).getByLabelText("Unidade dona");
    await waitFor(() => expect(dona).toHaveValue("u2"));
    expect(dona).not.toBeRequired();
    expect(within(dona).getByRole("option", { name: "Unidade u1" })).toBeInTheDocument();
    expect(within(dona).queryByRole("option", { name: "Unidade u3" })).not.toBeInTheDocument();
    await userEvent.click(within(d).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/wiki/pages/filha")[0]?.body).toMatchObject({ unidade_id: "u2" }));
  });

  it("conflito de versão aparece e o editor continua aberto", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/wiki/pages/filha": detail(),
      "GET v1/wiki/tree": { data: TREE },
      "PUT v1/wiki/pages/filha": fail(409, "outra pessoa salvou esta página antes", "WIKI_STALE_VERSION"),
    });
    renderApp(<WikiPageView />);
    await userEvent.click(await screen.findByRole("button", { name: /Editar/ }));
    const d = await screen.findByRole("dialog", { name: "Editar — Página filha" });
    await userEvent.click(within(d).getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("outra pessoa salvou esta página antes")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Editar — Página filha" })).toBeInTheDocument();
  });

  it("histórico: vê uma versão antiga e restaura (a atual não se restaura)", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/wiki/pages/filha": detail(),
      "GET v1/wiki/tree": { data: TREE },
      "GET v1/wiki/pages/filha/revisions": {
        data: [
          { id: "r3", version: 3, title: "Página filha", summary: "", edited_by_name: "Maria", edited_at: "2026-01-03T00:00:00Z" },
          { id: "r1", version: 1, title: "Título antigo", summary: "criação", edited_by_name: "João", edited_at: "2026-01-01T00:00:00Z" },
        ],
      },
      "GET v1/wiki/pages/filha/revisions/1": { data: { id: "r1", version: 1, title: "Título antigo", body: "texto antigo", summary: "", edited_by_name: "João", edited_at: "" } },
      "GET v1/wiki/pages/filha/revisions/3": { data: { id: "r3", version: 3, title: "Página filha", summary: "", edited_by_name: "Maria", edited_at: "" } },
      "POST v1/wiki/pages/filha/revisions/1/restore": { data: null },
    });
    renderApp(<WikiPageView />);
    await userEvent.click(await screen.findByRole("button", { name: /Histórico/ }));
    const d = await screen.findByRole("dialog", { name: "Histórico de versões" });
    expect(within(d).getByText("Selecione uma versão para visualizar.")).toBeInTheDocument();
    await userEvent.click(await within(d).findByRole("button", { name: /v3/ }));
    expect(await within(d).findByRole("heading", { name: "Página filha" })).toBeInTheDocument();
    expect(within(d).queryByRole("button", { name: /Restaurar/ })).not.toBeInTheDocument();

    await userEvent.click(within(d).getByRole("button", { name: /v1/ }));
    expect(await within(d).findByText("texto antigo")).toBeInTheDocument();
    expect(within(d).getByText(/João — criação/)).toBeInTheDocument();
    await userEvent.click(within(d).getByRole("button", { name: /Restaurar esta versão/ }));
    await waitFor(() => expect(api.to("POST v1/wiki/pages/filha/revisions/1/restore")).toHaveLength(1));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Histórico de versões" })).not.toBeInTheDocument());
  });

  it("exclusão pelo gestor volta à wiki; sem wiki:manage não exclui", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/wiki/pages/filha": detail({ breadcrumbs: [] }),
      "GET v1/wiki/tree": { data: TREE },
      "DELETE v1/wiki/pages/filha": { data: null },
    });
    const { unmount } = renderApp(<WikiPageView />);
    await userEvent.click(await screen.findByRole("button", { name: "Excluir página" }));
    const confirm = await screen.findByRole("dialog", { name: 'Excluir "Página filha"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/wiki"));
    expect(api.to("DELETE v1/wiki/pages/filha")).toHaveLength(1);
    expect(screen.queryByRole("navigation", { name: "Trilha" })).not.toBeInTheDocument();
    unmount();

    mockBackend({ ...identityRoutes({ permissions: ["wiki:read"] }), "GET v1/wiki/pages/filha": detail(), "GET v1/wiki/tree": { data: TREE } });
    renderApp(<WikiPageView />);
    await screen.findByRole("heading", { name: "Página filha", level: 1 });
    expect(screen.queryByRole("button", { name: "Excluir página" })).not.toBeInTheDocument();
  });

  it("exclusão recusada (tem subpáginas) fica na página", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/wiki/pages/filha": detail(),
      "GET v1/wiki/tree": { data: TREE },
      "DELETE v1/wiki/pages/filha": fail(409, "a página tem subpáginas"),
    });
    renderApp(<WikiPageView />);
    await userEvent.click(await screen.findByRole("button", { name: "Excluir página" }));
    const confirm = await screen.findByRole("dialog", { name: 'Excluir "Página filha"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    expect(await screen.findByText("a página tem subpáginas")).toBeInTheDocument();
    expect(router.push).not.toHaveBeenCalled();
  });

  it("página inexistente", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/wiki/pages/filha": fail(404, "página não encontrada"), "GET v1/wiki/tree": { data: [] } });
    renderApp(<WikiPageView />);
    expect(await screen.findByText("página não encontrada")).toBeInTheDocument();
  });
});
