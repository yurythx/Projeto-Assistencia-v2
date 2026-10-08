import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import OrganizacaoPage from "./page";

const TREE = [
  {
    id: "e1", nome: "Prefeitura", sigla: "PM", documento: "00.000.000/0001-00", slug: "pm", ativo: true,
    unidades: [
      {
        id: "u1", entidade_id: "e1", parent_id: "u0", nome: "Secretaria de Saúde", sigla: "SMS", slug: "sms", ativo: false, ad_group: "CN=GRP_SMS",
        email: "", telefone: "", endereco: "Rua A",
        departamentos: [{ id: "d1", unidade_id: "u1", nome: "Vigilância", sigla: "VS", slug: "vs", ativo: true, ad_group: "CN=GRP_VS" }],
      },
    ],
  },
  { id: "e2", nome: "Câmara", sigla: "", documento: "", slug: "cm", ativo: false, unidades: [] },
];

describe("Configurações > Organização", () => {
  beforeEach(() => resetNavigation());

  it("administração delegada (ADR 013): só a estrutura que cobre; entidades, só a global", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["iam:manage"], scopes: [{ perfil: "p", origem: "manual", entidade_id: "e1", unidade_id: "u0", permissions: ["iam:manage"] }] }),
      "GET v1/iam/org-tree": {
        data: [{ ...TREE[0], unidades: [{ ...TREE[0]!.unidades[0], parent_id: "u0" }, { id: "u0", entidade_id: "e1", nome: "Governo", sigla: "", slug: "gov", ativo: true, ad_group: "", email: "", telefone: "", endereco: "", departamentos: [] }] }],
      },
      "POST v1/iam/unidades": { data: {} },
    });
    renderApp(<OrganizacaoPage />);
    expect(await screen.findByRole("heading", { name: /Prefeitura/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Nova entidade/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Editar Prefeitura" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Unidade$/ })).not.toBeInTheDocument();
    // u0 (a dele): edita, mas não exclui (fica na entidade); u1 (filha): edita e exclui.
    expect(screen.getByRole("button", { name: "Editar Governo" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Excluir Governo" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Excluir Secretaria de Saúde" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Excluir Vigilância" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Nova subunidade de Governo" }));
    const d = await screen.findByRole("dialog", { name: "Nova unidade" });
    await userEvent.type(within(d).getByLabelText("Nome *"), "Ouvidoria");
    await userEvent.click(within(d).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("POST v1/iam/unidades")[0]?.body).toMatchObject({ entidade_id: "e1", parent_id: "u0", nome: "Ouvidoria" }));
  });

  it("mostra a árvore e cria entidade, unidade e departamento", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/iam/org-tree": { data: TREE },
      "POST v1/iam/entidades": { data: {} },
      "POST v1/iam/unidades": { data: {} },
      "POST v1/iam/departamentos": { data: {} },
    });
    renderApp(<OrganizacaoPage />);
    expect(await screen.findByRole("heading", { name: /Prefeitura/ })).toBeInTheDocument();
    expect(screen.getByText("00.000.000/0001-00 · 1 unidade(s)")).toBeInTheDocument();
    expect(screen.getByText("sem documento · 0 unidade(s)")).toBeInTheDocument();
    expect(screen.getByText("CN=GRP_SMS")).toBeInTheDocument();
    expect(screen.getByText("CN=GRP_VS")).toBeInTheDocument();
    expect(screen.getAllByText("Inativa")).toHaveLength(2);

    await userEvent.click(screen.getByRole("button", { name: /Nova entidade/ }));
    let dialog = await screen.findByRole("dialog", { name: "Nova entidade" });
    await userEvent.type(within(dialog).getByLabelText("Nome *"), " Autarquia ");
    await userEvent.type(within(dialog).getByLabelText("CNPJ / documento"), "11.111");
    expect(within(dialog).queryByLabelText("Grupo do Active Directory")).not.toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("POST v1/iam/entidades")).toHaveLength(1));
    expect(api.to("POST v1/iam/entidades")[0]!.body).toEqual({ nome: "Autarquia", sigla: "", ativo: true, documento: "11.111" });

    await userEvent.click(screen.getAllByRole("button", { name: /Unidade/ })[0]!);
    dialog = await screen.findByRole("dialog", { name: "Nova unidade" });
    await userEvent.type(within(dialog).getByLabelText("Nome *"), "Educação");
    await userEvent.type(within(dialog).getByLabelText("Grupo do Active Directory"), "CN=GRP_EDU");
    await userEvent.type(within(dialog).getByLabelText("E-mail"), "edu@pm.gov.br");
    await userEvent.type(within(dialog).getByLabelText("Endereço"), "Rua B");
    await userEvent.click(within(dialog).getByLabelText("Ativo"));
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("POST v1/iam/unidades")).toHaveLength(1));
    expect(api.to("POST v1/iam/unidades")[0]!.body).toEqual({
      nome: "Educação", sigla: "", ativo: false, ad_group: "CN=GRP_EDU", email: "edu@pm.gov.br", telefone: "", entidade_id: "e1", endereco: "Rua B",
    });

    await userEvent.click(screen.getByRole("button", { name: /Depto\./ }));
    dialog = await screen.findByRole("dialog", { name: "Nova departamento" });
    await userEvent.type(within(dialog).getByLabelText("Nome *"), "Compras");
    await userEvent.type(within(dialog).getByLabelText("Slug (URL)"), "compras");
    expect(within(dialog).queryByLabelText("Endereço")).not.toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("POST v1/iam/departamentos")).toHaveLength(1));
    expect(api.to("POST v1/iam/departamentos")[0]!.body).toMatchObject({ nome: "Compras", slug: "compras", unidade_id: "u1" });
  });

  it("edita a unidade sem trocar de entidade nem de mãe; exclusão recusada vira aviso", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/iam/org-tree": { data: TREE },
      "PUT v1/iam/unidades/u1": { data: {} },
      "DELETE v1/iam/departamentos/d1": fail(409, "departamento possui lotações vinculadas"),
    });
    renderApp(<OrganizacaoPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Editar Secretaria de Saúde" }));
    const dialog = await screen.findByRole("dialog", { name: "Editar unidade" });
    expect(within(dialog).getByLabelText("Nome *")).toHaveValue("Secretaria de Saúde");
    expect(within(dialog).getByLabelText("Ativo")).not.toBeChecked();
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/iam/unidades/u1")).toHaveLength(1));
    expect(api.to("PUT v1/iam/unidades/u1")[0]!.body).toMatchObject({ entidade_id: "e1", parent_id: "u0", slug: "sms", ad_group: "CN=GRP_SMS" });

    await userEvent.click(screen.getByRole("button", { name: "Excluir Vigilância" }));
    const confirm = await screen.findByRole("dialog", { name: 'Excluir departamento "Vigilância"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    expect(await screen.findByText("departamento possui lotações vinculadas")).toBeInTheDocument();
  });

  it("vazio e erro", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/iam/org-tree": { data: [] } });
    const { unmount } = renderApp(<OrganizacaoPage />);
    expect(await screen.findByText("Nenhuma entidade cadastrada")).toBeInTheDocument();
    unmount();
    mockBackend({ ...identityRoutes(), "GET v1/iam/org-tree": fail(500, "IAM fora") });
    renderApp(<OrganizacaoPage />);
    expect(await screen.findByText("IAM fora")).toBeInTheDocument();
  });
});
