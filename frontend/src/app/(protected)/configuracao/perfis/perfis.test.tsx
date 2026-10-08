import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import PerfisPage from "./page";

const GROUPS = [
  { module: "blog", name: "Blog", permissions: [{ key: "blog:read", description: "Ler" }, { key: "blog:manage", description: "Gerir" }] },
  { module: "files", name: "Arquivos", permissions: [{ key: "files:read", description: "Ler arquivos" }] },
];
const perfil = (id: string, extra: Record<string, unknown> = {}) => ({ id, slug: id, nome: `Perfil ${id}`, descricao: "", permissoes: [], sistema: false, ativo: true, ...extra });

describe("Configurações > Perfis", () => {
  beforeEach(() => resetNavigation());

  it("lista perfis com contagem, sistema e acesso total", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/iam/perfis": { data: [perfil("admin", { sistema: true, permissoes: ["*"], descricao: "Tudo" }), perfil("leitor", { permissoes: ["blog:read"], ativo: false })] },
      "GET v1/iam/permissions": { data: GROUPS },
    });
    renderApp(<PerfisPage />);
    expect(await screen.findByText("Acesso total (*)")).toBeInTheDocument();
    expect(screen.getByText("1 permissões")).toBeInTheDocument();
    expect(screen.getByText("Sistema")).toBeInTheDocument();
    expect(screen.getByText("Inativo")).toBeInTheDocument();
    // perfil de sistema não é excluível
    expect(screen.queryByRole("button", { name: "Excluir Perfil admin" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Excluir Perfil leitor" })).toBeInTheDocument();
  });

  it("cria perfil marcando permissões do catálogo (uma a uma e por módulo)", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/iam/perfis": { data: [] },
      "GET v1/iam/permissions": { data: GROUPS },
      "POST v1/iam/perfis": { data: perfil("novo") },
    });
    renderApp(<PerfisPage />);
    expect(await screen.findByText("Nenhum perfil")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Novo perfil/ }));
    const dialog = await screen.findByRole("dialog", { name: "Novo perfil" });
    await userEvent.type(within(dialog).getByLabelText("Nome *"), " Editor ");
    await userEvent.type(within(dialog).getByLabelText("Descrição"), "Edita o blog");

    const blog = within(dialog).getByText("Blog").closest("div.rounded-lg") as HTMLElement;
    await userEvent.click(within(blog).getByLabelText("Todas"));
    expect(within(dialog).getByText("Permissões (2)")).toBeInTheDocument();
    await userEvent.click(within(blog).getByLabelText("Todas"));
    expect(within(dialog).getByText("Permissões (0)")).toBeInTheDocument();
    await userEvent.click(within(blog).getByRole("checkbox", { name: /blog:manage/ }));
    await userEvent.click(within(dialog).getByRole("checkbox", { name: /files:read/ }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar perfil" }));
    await waitFor(() => expect(api.to("POST v1/iam/perfis")).toHaveLength(1));
    expect(api.to("POST v1/iam/perfis")[0]!.body).toEqual({ nome: "Editor", descricao: "Edita o blog", permissoes: ["blog:manage", "files:read"], ativo: true });
  });

  it("edita perfil de sistema preservando curingas; remove um curinga", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/iam/perfis": { data: [perfil("admin", { sistema: true, permissoes: ["*", "blog:read", "legado:x"] })] },
      "GET v1/iam/permissions": { data: GROUPS },
      "PUT v1/iam/perfis/admin": { data: perfil("admin") },
    });
    renderApp(<PerfisPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Editar Perfil admin" }));
    const dialog = await screen.findByRole("dialog", { name: "Editar perfil — Perfil admin" });
    expect(within(dialog).getByText(/Perfil de sistema/)).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Slug")).toBeDisabled();
    expect(within(dialog).getByRole("checkbox", { name: /blog:read/ })).toBeChecked();
    await userEvent.click(within(dialog).getByRole("button", { name: "Remover legado:x" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar perfil" }));
    await waitFor(() => expect(api.to("PUT v1/iam/perfis/admin")).toHaveLength(1));
    // slug desabilitado não vai no formulário
    expect(api.to("PUT v1/iam/perfis/admin")[0]!.body).toEqual({ nome: "Perfil admin", descricao: "", permissoes: ["*", "blog:read"], ativo: true });
  });

  it("exclusão recusada (perfil em uso) e erro ao listar", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/iam/perfis": { data: [perfil("x")] },
      "GET v1/iam/permissions": { data: [] },
      "DELETE v1/iam/perfis/x": fail(409, "perfil em uso por lotações"),
    });
    renderApp(<PerfisPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Excluir Perfil x" }));
    const confirm = await screen.findByRole("dialog", { name: 'Excluir o perfil "Perfil x"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    expect(await screen.findByText("perfil em uso por lotações")).toBeInTheDocument();
    expect(api.to("DELETE v1/iam/perfis/x")).toHaveLength(1);
  });

  it("erro ao carregar perfis", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/iam/perfis": fail(500, "sem banco"), "GET v1/iam/permissions": { data: [] } });
    renderApp(<PerfisPage />);
    expect(await screen.findByText("sem banco")).toBeInTheDocument();
  });
});
