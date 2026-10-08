import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import BlogPage from "./page";
import PostPage from "./[ref]/page";

const post = (extra: Record<string, unknown> = {}) => ({
  id: "p1", slug: "boas-vindas", title: "Boas-vindas", summary: "Resumo", body: "# Olá", kind: "noticia",
  status: "published", pinned: false, author_name: "Maria", published_at: "2026-01-02T10:00:00Z",
  created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-02T00:00:00Z", ...extra,
});

describe("Blog — listagem", () => {
  beforeEach(() => resetNavigation());

  it("leitor vê as publicações, sem criar nem filtrar por situação", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["blog:read"] }),
      "GET v1/blog/posts": page([post({ pinned: true, cover_url: "https://cdn/c.png" }), post({ id: "p2", slug: "c", title: "Aviso", kind: "comunicado", summary: "" })]),
    });
    renderApp(<BlogPage />);
    expect(await screen.findByText("Boas-vindas")).toBeInTheDocument();
    expect(screen.getByText("Aviso")).toBeInTheDocument();
    expect(screen.getByLabelText("Fixado")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Boas-vindas/ })).toHaveAttribute("href", "/blog/boas-vindas");
    expect(screen.queryByRole("button", { name: /Nova publicação/ })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Situação")).not.toBeInTheDocument();
  });

  it("gestor filtra por busca, tipo e situação; lista vazia e erro", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/blog/posts": (req) =>
        req.query.get("status") === "archived" ? fail(500, "banco fora") : req.query.get("q") ? page([]) : page([post({ status: "draft" })]),
    });
    renderApp(<BlogPage />);
    expect(await screen.findByText("Rascunho")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Buscar"), "  orçamento ");
    await userEvent.click(screen.getByRole("button", { name: "Filtrar" }));
    expect(await screen.findByText("Nenhuma publicação")).toBeInTheDocument();
    expect(api.to("GET v1/blog/posts").at(-1)!.query.get("q")).toBe("orçamento");

    await userEvent.selectOptions(screen.getByLabelText("Tipo"), "comunicado");
    await waitFor(() => expect(api.to("GET v1/blog/posts").at(-1)!.query.get("kind")).toBe("comunicado"));

    await userEvent.selectOptions(screen.getByLabelText("Situação"), "archived");
    expect(await screen.findByText("banco fora")).toBeInTheDocument();
  });

  it("nova publicação: envia a capa ao MinIO, salva e abre a página", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/blog/posts": page([]),
      "POST v1/blog/uploads": { data: { object_key: "blog/capa.png", upload_url: "http://minio.test/b/capa.png", method: "PUT", headers: {}, expires_at: "" } },
      "PUT minio.test/*": { status: 200 },
      "POST v1/blog/posts": (req) => ({ data: post({ ...(req.body as object), slug: "novo" }) }),
    });
    renderApp(<BlogPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Nova publicação/ }));
    const dialog = await screen.findByRole("dialog", { name: "Nova publicação" });

    await userEvent.type(within(dialog).getByLabelText("Título *"), "Novo edital");
    await userEvent.selectOptions(within(dialog).getByLabelText("Tipo"), "comunicado");
    await userEvent.click(within(dialog).getByLabelText(/Fixar no topo/));
    await userEvent.upload(within(dialog).getByLabelText("Imagem de capa"), new File(["x"], "capa.png", { type: "image/png" }));
    expect(await within(dialog).findByText("Capa: capa.png")).toBeInTheDocument();

    await userEvent.type(within(dialog).getByLabelText("Conteúdo em Markdown"), "**texto**");
    await userEvent.click(within(dialog).getByRole("button", { name: "Pré-visualizar" }));
    expect(within(dialog).getByText("texto").tagName).toBe("STRONG");
    await userEvent.click(within(dialog).getByRole("button", { name: "Editar" }));

    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("POST v1/blog/posts")).toHaveLength(1));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/blog/novo"));
    expect(api.to("POST v1/blog/posts")[0]!.body).toMatchObject({
      title: "Novo edital", kind: "comunicado", pinned: true, cover_object_key: "blog/capa.png", body: "**texto**",
    });
  });

  it("falha no upload da capa mostra o erro e não guarda a capa", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/blog/posts": page([]),
      "POST v1/blog/uploads": { data: { object_key: "blog/x.png", upload_url: "http://minio.test/b/x.png", method: "PUT", headers: {}, expires_at: "" } },
      "PUT minio.test/*": { status: 403 },
    });
    renderApp(<BlogPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Nova publicação/ }));
    const dialog = await screen.findByRole("dialog");
    // tipo vazio (alguns SOs não informam): mimeOf cai no genérico
    await userEvent.setup({ applyAccept: false }).upload(within(dialog).getByLabelText("Imagem de capa"), new File(["x"], "x.png", { type: "" }));
    expect(await screen.findByText("Operação não concluída")).toBeInTheDocument();
    expect(within(dialog).queryByText(/Capa:/)).not.toBeInTheDocument();
  });
});

describe("Blog — publicação", () => {
  beforeEach(() => resetNavigation({ ref: "boas-vindas" }));

  it("leitor só lê", async () => {
    mockBackend({ ...identityRoutes({ permissions: ["blog:read"] }), "GET v1/blog/posts/boas-vindas": { data: post({ cover_url: "https://cdn/c.png" }) } });
    renderApp(<PostPage />);
    expect(await screen.findByRole("heading", { name: "Boas-vindas", level: 1 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Olá" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Editar/ })).not.toBeInTheDocument();
  });

  it("gestor despublica, publica, arquiva e exclui", async () => {
    let current = post();
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/blog/posts/boas-vindas": () => ({ data: current }),
      "POST v1/blog/posts/p1/unpublish": () => ((current = post({ status: "draft" })), { data: current }),
      "POST v1/blog/posts/p1/publish": () => ((current = post()), { data: current }),
      "POST v1/blog/posts/p1/archive": () => ((current = post({ status: "archived", kind: "comunicado" })), { data: current }),
      "DELETE v1/blog/posts/p1": { data: null },
    });
    renderApp(<PostPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Despublicar/ }));
    expect(await screen.findByText("Rascunho")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Publicar/ }));
    await screen.findByRole("button", { name: /Despublicar/ });
    await userEvent.click(screen.getByRole("button", { name: /Arquivar/ }));
    await waitFor(() => expect(screen.queryByRole("button", { name: /Arquivar/ })).not.toBeInTheDocument());
    expect(screen.getByText("Comunicado")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Excluir/ }));
    const dialog = await screen.findByRole("dialog", { name: "Excluir publicação?" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Excluir" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/blog"));
    expect(api.to("DELETE v1/blog/posts/p1")).toHaveLength(1);
  });

  it("editar: mesmo slug recarrega; slug novo troca a URL", async () => {
    let slug = "boas-vindas";
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/blog/posts/boas-vindas": { data: post() },
      "PUT v1/blog/posts/p1": () => ({ data: post({ slug }) }),
    });
    renderApp(<PostPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Editar/ }));
    let dialog = await screen.findByRole("dialog", { name: "Editar publicação" });
    expect(within(dialog).getByLabelText("Título *")).toHaveValue("Boas-vindas");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/blog/posts/p1")).toHaveLength(1));
    expect(router.replace).not.toHaveBeenCalled();

    slug = "novo-slug";
    await userEvent.click(screen.getByRole("button", { name: /Editar/ }));
    dialog = await screen.findByRole("dialog", { name: "Editar publicação" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/blog/novo-slug"));
  });
});
