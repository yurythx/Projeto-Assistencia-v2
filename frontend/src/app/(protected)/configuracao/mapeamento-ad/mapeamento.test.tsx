import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import MapeamentoADPage from "./page";

const ORG = [{ id: "e1", nome: "Órgão", sigla: "ORG", unidades: [{ id: "u1", nome: "RH", sigla: "", departamentos: [] }] }];

describe("Configurações > Mapeamento AD", () => {
  beforeEach(() => resetNavigation());

  it("cria mapeamento com escopo e limpa o formulário; remove", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/iam/ad-mappings": {
        data: [
          { id: "m1", ad_group: "GRP_RH", perfil_id: "p1", perfil_nome: "Gestor", scope_label: "ORG / RH", descricao: "RH", created_at: "2026-01-01T00:00:00Z", created_by: "admin" },
          { id: "m2", ad_group: "GRP_ALL", perfil_id: "p1", perfil_nome: "Gestor", scope_label: "", created_at: "2026-01-01T00:00:00Z" },
        ],
      },
      "GET v1/iam/perfis": { data: [{ id: "p1", nome: "Gestor", ativo: true }, { id: "p2", nome: "Antigo", ativo: false }] },
      "GET v1/iam/org-tree": { data: ORG },
      "POST v1/iam/ad-mappings": { data: {} },
      "DELETE v1/iam/ad-mappings/m1": { data: null },
    });
    renderApp(<MapeamentoADPage />);
    expect(await screen.findByText("GRP_RH")).toBeInTheDocument();
    expect(screen.getByText("Global")).toBeInTheDocument();
    expect(screen.getByText("por admin")).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Antigo" })).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Grupo do AD *"), " GRP_TI ");
    await userEvent.selectOptions(await screen.findByLabelText("Perfil *"), "p1");
    await userEvent.selectOptions(await screen.findByLabelText("Entidade"), "e1");
    await userEvent.selectOptions(screen.getByLabelText("Unidade"), "u1");
    await userEvent.type(screen.getByLabelText("Descrição"), "TI");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar mapeamento" }));
    await waitFor(() => expect(api.to("POST v1/iam/ad-mappings")).toHaveLength(1));
    expect(api.to("POST v1/iam/ad-mappings")[0]!.body).toEqual({ ad_group: "GRP_TI", perfil_id: "p1", descricao: "TI", entidade_id: "e1", unidade_id: "u1" });
    await waitFor(() => expect(screen.getByLabelText("Grupo do AD *")).toHaveValue(""));
    expect(screen.getByLabelText("Entidade")).toHaveValue("");

    await userEvent.click(screen.getByRole("button", { name: "Remover mapeamento GRP_RH" }));
    const confirm = await screen.findByRole("dialog", { name: "Remover mapeamento?" });
    expect(within(confirm).getByText(/perdem o perfil Gestor/)).toBeInTheDocument();
    await userEvent.click(within(confirm).getByRole("button", { name: "Remover" }));
    await waitFor(() => expect(api.to("DELETE v1/iam/ad-mappings/m1")).toHaveLength(1));
  });

  it("falha mantém o que foi digitado; lista vazia", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/iam/ad-mappings": { data: [] },
      "GET v1/iam/perfis": { data: [{ id: "p1", nome: "Gestor", ativo: true }] },
      "GET v1/iam/org-tree": { data: [] },
      "POST v1/iam/ad-mappings": fail(409, "grupo já mapeado para este perfil e escopo"),
    });
    renderApp(<MapeamentoADPage />);
    expect(await screen.findByText("Nenhum grupo mapeado")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Grupo do AD *"), "GRP_X");
    await userEvent.selectOptions(await screen.findByLabelText("Perfil *"), "p1");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar mapeamento" }));
    expect(await screen.findByText("grupo já mapeado para este perfil e escopo")).toBeInTheDocument();
    expect(screen.getByLabelText("Grupo do AD *")).toHaveValue("GRP_X");
  });

  it("erro ao listar", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/iam/ad-mappings": fail(500, "fora"), "GET v1/iam/perfis": { data: [] }, "GET v1/iam/org-tree": { data: [] } });
    renderApp(<MapeamentoADPage />);
    expect(await screen.findByText("fora")).toBeInTheDocument();
  });
});
