import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import UsuariosPage from "./page";

const future = new Date(Date.now() + 3600_000).toISOString();
const user = (id: string, extra: Record<string, unknown> = {}) => ({
  id, username: `user${id}`, email: `${id}@orgao.gov.br`, display_name: `Usuário ${id}`, active: true, federated: false, local_login: true,
  roles: [], groups: [], created_at: "2026-01-01T00:00:00Z", last_seen_at: "2026-01-02T00:00:00Z", lotacoes: [] as unknown[], ...extra,
});
const ORG = [{ id: "e1", nome: "Órgão", sigla: "ORG", unidades: [{ id: "un1", nome: "Unidade", sigla: "", departamentos: [] }] }];

describe("Configurações > Usuários", () => {
  beforeEach(() => resetNavigation());

  it("lista, busca e filtra por situação", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/users": (req) =>
        req.query.get("active") === "false"
          ? fail(500, "diretório indisponível")
          : req.query.get("q")
            ? page([])
            : page([user("1", { display_name: "" }), user("2", { federated: true, active: false, locked_until: future })]),
    });
    renderApp(<UsuariosPage />);
    expect(await screen.findByText("user1", { selector: "span.font-medium" })).toBeInTheDocument();
    expect(screen.getByText("AD / Keycloak")).toBeInTheDocument();
    expect(screen.getByText("Inativa")).toBeInTheDocument();
    expect(screen.getByText("Bloqueada")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Buscar"), " ninguém ");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(await screen.findByText("Nenhum usuário encontrado")).toBeInTheDocument();
    expect(api.to("GET v1/users").at(-1)!.query.get("q")).toBe("ninguém");
    await userEvent.selectOptions(screen.getByLabelText("Situação"), "false");
    expect(await screen.findByText("diretório indisponível")).toBeInTheDocument();
  });

  it("cria conta local", async () => {
    const api = mockBackend({ ...identityRoutes(), "GET v1/users": page([]), "POST v1/users": { data: user("9") } });
    renderApp(<UsuariosPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Conta local/ }));
    const dialog = await screen.findByRole("dialog", { name: "Nova conta local" });
    await userEvent.type(within(dialog).getByLabelText("Usuário *"), " ana ");
    await userEvent.type(within(dialog).getByLabelText("E-mail *"), "ana@orgao.gov.br");
    await userEvent.type(within(dialog).getByLabelText("Nome de exibição"), "Ana");
    await userEvent.type(within(dialog).getByLabelText("Senha inicial *"), "Senha-Forte-123!");
    await userEvent.click(within(dialog).getByRole("button", { name: "Criar conta" }));
    await waitFor(() => expect(api.to("POST v1/users")).toHaveLength(1));
    expect(api.to("POST v1/users")[0]!.body).toEqual({ username: "ana", email: "ana@orgao.gov.br", display_name: "Ana", password: "Senha-Forte-123!", roles: [] });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Nova conta local" })).not.toBeInTheDocument());
  });

  it("conta local: desativa, redefine a senha, adiciona e remove lotação", async () => {
    let current = user("1");
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/users": page([current]),
      "GET v1/users/1": () => ({ data: current }),
      "GET v1/iam/perfis": { data: [{ id: "p1", slug: "gestor", nome: "Gestor", ativo: true }, { id: "p2", slug: "velho", nome: "Antigo", ativo: false }] },
      "GET v1/iam/org-tree": { data: ORG },
      "PATCH v1/users/1": (req) => ((current = { ...current, active: (req.body as { active: boolean }).active }), { data: current }),
      "POST v1/users/1/password": { data: null },
      "POST v1/users/1/lotacoes": () => (
        (current = { ...current, lotacoes: [{ id: "l1", user_id: "1", perfil_id: "p1", perfil_nome: "Gestor", scope_label: "", principal: true, created_at: "" }] }),
        { data: null }
      ),
      "DELETE v1/users/1/lotacoes/l1": () => ((current = { ...current, lotacoes: [] }), { data: null }),
    });
    renderApp(<UsuariosPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Gerenciar user1" }));
    const dialog = await screen.findByRole("dialog", { name: "Usuário 1" });
    expect(await within(dialog).findByText("Local")).toBeInTheDocument();
    expect(within(dialog).getByText("Sem lotações manuais. Perfis vindos do AD são resolvidos no login.")).toBeInTheDocument();

    await userEvent.click(within(dialog).getByRole("button", { name: /Desativar/ }));
    await waitFor(() => expect(api.to("PATCH v1/users/1")[0]?.body).toEqual({ display_name: "Usuário 1", active: false, roles: [] }));
    await userEvent.click(await within(dialog).findByRole("button", { name: /Reativar/ }));
    await waitFor(() => expect(api.to("PATCH v1/users/1")).toHaveLength(2));

    await userEvent.type(within(dialog).getByLabelText("Nova senha local"), "Outra-Senha-123!");
    await userEvent.click(within(dialog).getByRole("button", { name: /Redefinir/ }));
    await waitFor(() => expect(api.to("POST v1/users/1/password")[0]?.body).toEqual({ password: "Outra-Senha-123!" }));
    await waitFor(() => expect(within(dialog).getByLabelText("Nova senha local")).toHaveValue(""));

    // perfil inativo não é oferecido
    expect(within(dialog).queryByRole("option", { name: "Antigo" })).not.toBeInTheDocument();
    await userEvent.selectOptions(await within(dialog).findByLabelText("Perfil *"), "p1");
    await userEvent.selectOptions(await within(dialog).findByLabelText("Entidade"), "e1");
    await userEvent.click(within(dialog).getByRole("button", { name: /Adicionar lotação/ }));
    await waitFor(() => expect(api.to("POST v1/users/1/lotacoes")[0]?.body).toEqual({ perfil_id: "p1", principal: true, entidade_id: "e1" }));
    expect(await within(dialog).findByText("Principal")).toBeInTheDocument();
    expect(within(dialog).getByText(/Global/)).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Perfil *")).toHaveValue("");

    await userEvent.click(within(dialog).getByRole("button", { name: "Remover lotação Gestor" }));
    const confirm = await screen.findByRole("dialog", { name: "Remover lotação?" });
    await userEvent.click(within(confirm).getByRole("button", { name: "Remover" }));
    await waitFor(() => expect(api.to("DELETE v1/users/1/lotacoes/l1")).toHaveLength(1));
  });

  it("conta federada bloqueada: sem senha local, remove o bloqueio e mostra os grupos do AD", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/users": page([user("2")]),
      "GET v1/users/2": {
        data: user("2", {
          federated: true, local_login: true, locked_until: future, ad_synced_at: "2026-01-03T00:00:00Z", groups: ["GRP_TI", "GRP_ALL"],
          lotacoes: [{ id: "l9", user_id: "2", perfil_id: "p1", perfil_nome: "Gestor", scope_label: "ORG / Unidade", principal: false, created_at: "" }],
        }),
      },
      "GET v1/iam/perfis": { data: [] },
      "GET v1/iam/org-tree": { data: [] },
      "POST v1/users/2/unlock": { data: null },
      "POST v1/users/2/lotacoes": fail(422, "perfil obrigatório"),
    });
    renderApp(<UsuariosPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Gerenciar user2" }));
    const dialog = await screen.findByRole("dialog", { name: "Usuário 2" });
    expect(await within(dialog).findByText("Federado (Keycloak/AD) + login local")).toBeInTheDocument();
    expect(within(dialog).getByText("GRP_TI")).toBeInTheDocument();
    expect(within(dialog).getByText("Sincronizado com o AD")).toBeInTheDocument();
    expect(within(dialog).getByText(/ORG \/ Unidade/)).toBeInTheDocument();
    expect(within(dialog).queryByLabelText("Nova senha local")).not.toBeInTheDocument();
    expect(within(dialog).getByText(/Senha gerida pelo Active Directory/)).toBeInTheDocument();

    await userEvent.click(within(dialog).getByRole("button", { name: /Remover bloqueio/ }));
    await waitFor(() => expect(api.to("POST v1/users/2/unlock")).toHaveLength(1));
  });

  it("sem users:manage nem iam:manage: só consulta", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["users:read"] }),
      "GET v1/users": page([user("3")]),
      "GET v1/users/3": { data: user("3", { lotacoes: [{ id: "l1", user_id: "3", perfil_id: "p1", perfil_nome: "Gestor", scope_label: "", principal: false, created_at: "" }] }) },
    });
    renderApp(<UsuariosPage />);
    expect(screen.queryByRole("button", { name: /Conta local/ })).not.toBeInTheDocument();
    await userEvent.click(await screen.findByRole("button", { name: "Gerenciar user3" }));
    const dialog = await screen.findByRole("dialog", { name: "Usuário 3" });
    expect(await within(dialog).findByText("Gestor")).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /Desativar/ })).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /Remover lotação/ })).not.toBeInTheDocument();
    expect(within(dialog).queryByLabelText("Perfil *")).not.toBeInTheDocument();
  });

  it("usuário inexistente no detalhe", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/users": page([user("4")]), "GET v1/users/4": fail(404, "usuário não encontrado") });
    renderApp(<UsuariosPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Gerenciar user4" }));
    expect(await screen.findByText("usuário não encontrado")).toBeInTheDocument();
  });
});
