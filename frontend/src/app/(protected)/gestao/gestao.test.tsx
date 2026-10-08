import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import GestaoContatoPage from "./contato/page";
import GestaoServicosPage from "./servicos/page";

const service = (id: string, extra: Record<string, unknown> = {}) => ({
  id, slug: id, title: `Serviço ${id}`, summary: "", description: "", category: "Saúde", audience: "", requirements: [], steps: [],
  channels: [], sla: "", cost: "", icon: "", status: "draft", position: 0, updated_at: "2026-01-01T00:00:00Z", ...extra,
});
const ORG = [{ id: "e1", nome: "Órgão", unidades: [{ id: "u1", nome: "Protocolo", departamentos: [] }] }];

describe("Gestão > Serviços", () => {
  beforeEach(() => resetNavigation());

  it("publica, despublica, arquiva e exclui conforme a situação", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/catalog/admin/services": (req) =>
        req.query.get("status") === "archived"
          ? fail(500, "catálogo fora")
          : page([service("s1"), service("s2", { status: "published", category: "" }), service("s3", { status: "archived" })]),
      "POST v1/catalog/admin/services/s1/publish": { data: null },
      "POST v1/catalog/admin/services/s2/unpublish": { data: null },
      "POST v1/catalog/admin/services/s1/archive": { data: null },
      "DELETE v1/catalog/admin/services/s3": { data: null },
    });
    renderApp(<GestaoServicosPage />);
    expect(await screen.findByText("/s1")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Arquivar Serviço s3" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Despublicar Serviço s1" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Publicar Serviço s1" }));
    await userEvent.click(screen.getByRole("button", { name: "Despublicar Serviço s2" }));
    await userEvent.click(screen.getByRole("button", { name: "Arquivar Serviço s1" }));
    await waitFor(() => expect(api.to("POST v1/catalog/admin/services/s1/archive")).toHaveLength(1));
    expect(api.to("POST v1/catalog/admin/services/s1/publish")).toHaveLength(1);
    expect(api.to("POST v1/catalog/admin/services/s2/unpublish")).toHaveLength(1);

    await userEvent.click(screen.getByRole("button", { name: "Excluir Serviço s3" }));
    const confirm = await screen.findByRole("dialog", { name: 'Excluir "Serviço s3"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    await waitFor(() => expect(api.to("DELETE v1/catalog/admin/services/s3")).toHaveLength(1));

    await userEvent.selectOptions(screen.getByLabelText("Situação"), "archived");
    expect(await screen.findByText("catálogo fora")).toBeInTheDocument();
  });

  it("cria serviço com requisitos, etapas e canais (canal incompleto é descartado)", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/catalog/admin/services": page([]),
      "GET v1/iam/org-tree": { data: ORG },
      "POST v1/catalog/admin/services": { data: service("novo") },
    });
    renderApp(<GestaoServicosPage />);
    expect(await screen.findByText("Nenhum serviço")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Novo serviço/ }));
    const d = await screen.findByRole("dialog", { name: "Novo serviço" });
    await userEvent.type(within(d).getByLabelText("Título *"), "Alvará de funcionamento");
    await userEvent.type(within(d).getByLabelText("Resumo"), "Licença para empresas");
    await userEvent.type(within(d).getByLabelText("Categoria"), "Empresas");
    await userEvent.type(within(d).getByLabelText("Prazo"), "até 15 dias");
    await userEvent.selectOptions(await within(d).findByLabelText("Unidade responsável"), "u1");
    await userEvent.clear(within(d).getByLabelText("Ordem"));
    await userEvent.type(within(d).getByLabelText("Ordem"), "3");
    await userEvent.type(within(d).getByLabelText("Requisitos (um por linha)"), "CNPJ{Enter}{Enter}  Contrato social ");
    await userEvent.type(within(d).getByLabelText("Etapas (uma por linha)"), "Solicitar{Enter}Vistoria");

    await userEvent.click(within(d).getByRole("button", { name: /Canal/ }));
    await userEvent.click(within(d).getByRole("button", { name: /Canal/ }));
    await userEvent.click(within(d).getByRole("button", { name: /Canal/ }));
    await userEvent.selectOptions(within(d).getByLabelText("Tipo", { selector: "#ch-type-0" }), "presencial");
    await userEvent.type(within(d).getByLabelText("Rótulo", { selector: "#ch-label-0" }), "Sede");
    await userEvent.type(within(d).getByLabelText("Valor", { selector: "#ch-value-0" }), "Rua A, 10");
    await userEvent.type(within(d).getByLabelText("Rótulo", { selector: "#ch-label-1" }), "sem valor");
    // remove o terceiro canal
    await userEvent.click(within(d).getAllByRole("button", { name: "Remover canal" })[2]!);
    expect(within(d).getAllByRole("button", { name: "Remover canal" })).toHaveLength(2);

    await userEvent.click(within(d).getByRole("button", { name: "Salvar serviço" }));
    await waitFor(() => expect(api.to("POST v1/catalog/admin/services")).toHaveLength(1));
    expect(api.to("POST v1/catalog/admin/services")[0]!.body).toEqual({
      title: "Alvará de funcionamento", slug: "", summary: "Licença para empresas", description: "", category: "Empresas", audience: "",
      requirements: ["CNPJ", "Contrato social"], steps: ["Solicitar", "Vistoria"],
      channels: [{ type: "presencial", label: "Sede", value: "Rua A, 10" }],
      sla: "até 15 dias", cost: "", icon: "", responsible_unidade_id: "u1", position: 3,
    });
  });

  it("edita serviço existente; recusa do backend mantém o formulário", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/catalog/admin/services": page([service("s1", { requirements: ["RG"], channels: [{ type: "online", label: "Portal", value: "https://x" }], responsible_unidade_id: "u1" })]),
      "GET v1/iam/org-tree": { data: ORG },
      "PUT v1/catalog/admin/services/s1": fail(422, "publicar exige resumo e canal"),
    });
    renderApp(<GestaoServicosPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Editar Serviço s1" }));
    const d = await screen.findByRole("dialog", { name: "Editar serviço" });
    expect(within(d).getByLabelText("Requisitos (um por linha)")).toHaveValue("RG");
    expect(within(d).getByLabelText("Valor", { selector: "#ch-value-0" })).toHaveValue("https://x");
    await userEvent.click(within(d).getByRole("button", { name: "Salvar serviço" }));
    expect(await screen.findByText("publicar exige resumo e canal")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Editar serviço" })).toBeInTheDocument();
    expect(api.to("PUT v1/catalog/admin/services/s1")[0]!.body).toMatchObject({ requirements: ["RG"], position: 0 });
  });
});

const msg = (id: string, extra: Record<string, unknown> = {}) => ({
  id, protocol: `2026-${id}`, name: "Fulano", email: "fulano@email.com", subject: "Buraco na rua", category: "Obras", status: "new",
  created_at: "2026-01-01T00:00:00Z", ...extra,
});

describe("Gestão > Contato", () => {
  beforeEach(() => resetNavigation());

  it("abre a mensagem e faz a triagem mantendo o responsável", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/contact/messages": (req) => (req.query.get("status") === "archived" ? page([]) : page([msg("1")])),
      "GET v1/contact/messages/1": { data: { ...msg("1"), phone: "(11) 99999-0000", message: "Texto\nlongo", consent_at: "2026-01-01T00:00:00Z", notes: "", assigned_to: "u9" } },
      "PATCH v1/contact/messages/1": { data: null },
    });
    renderApp(<GestaoContatoPage />);
    // abre filtrando as novas
    await waitFor(() => expect(api.to("GET v1/contact/messages")[0]!.query.get("status")).toBe("new"));
    await userEvent.click(await screen.findByRole("button", { name: "Abrir 2026-1" }));
    const d = await screen.findByRole("dialog", { name: "Mensagem 2026-1" });
    expect(await within(d).findByText("(11) 99999-0000")).toBeInTheDocument();
    expect(within(d).getByRole("link", { name: "fulano@email.com" }).getAttribute("href")).toContain("subject=Re%3A%20Buraco%20na%20rua%20%5B2026-1%5D");
    await userEvent.selectOptions(within(d).getByLabelText("Situação"), "answered");
    await userEvent.type(within(d).getByLabelText("Anotações internas"), " respondido por e-mail ");
    await userEvent.click(within(d).getByRole("button", { name: "Salvar triagem" }));
    await waitFor(() => expect(api.to("PATCH v1/contact/messages/1")).toHaveLength(1));
    expect(api.to("PATCH v1/contact/messages/1")[0]!.body).toEqual({ status: "answered", notes: "respondido por e-mail", assigned_to: "u9", unidade_id: null });

    await userEvent.keyboard("{Escape}");
    await userEvent.selectOptions(screen.getAllByLabelText("Situação")[0]!, "archived");
    expect(await screen.findByText("Nenhuma mensagem")).toBeInTheDocument();
  });

  it("sem contact:manage só lê (anotações visíveis); triagem recusada", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["contact:read"] }),
      "GET v1/contact/messages": page([msg("2", { status: "in_progress" })]),
      "GET v1/contact/messages/2": { data: { ...msg("2"), phone: "", message: "m", consent_at: "2026-01-01T00:00:00Z", notes: "ligar amanhã" } },
    });
    renderApp(<GestaoContatoPage />);
    expect(await screen.findByText("Em atendimento")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Abrir 2026-2" }));
    const d = await screen.findByRole("dialog", { name: "Mensagem 2026-2" });
    expect(await within(d).findByText("Anotações: ligar amanhã")).toBeInTheDocument();
    expect(within(d).queryByRole("button", { name: "Salvar triagem" })).not.toBeInTheDocument();
    expect(within(d).queryByText("Telefone")).not.toBeInTheDocument();
  });

  it("triagem com erro e mensagem inexistente", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/contact/messages": page([msg("3"), msg("4")]),
      "GET v1/contact/messages/3": { data: { ...msg("3"), phone: "", message: "m", consent_at: "2026-01-01T00:00:00Z", notes: "" } },
      "PATCH v1/contact/messages/3": fail(422, "responsável inválido"),
      "GET v1/contact/messages/4": fail(404, "mensagem não encontrada"),
    });
    renderApp(<GestaoContatoPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Abrir 2026-3" }));
    let d = await screen.findByRole("dialog", { name: "Mensagem 2026-3" });
    await userEvent.click(await within(d).findByRole("button", { name: "Salvar triagem" }));
    expect(await screen.findByText("responsável inválido")).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    await userEvent.click(screen.getByRole("button", { name: "Abrir 2026-4" }));
    d = await screen.findByRole("dialog", { name: "Mensagem 2026-4" });
    expect(await within(d).findByText("mensagem não encontrada")).toBeInTheDocument();
  });

  it("setor (ADR 013): mostra na lista e encaminha; a caixa geral é a opção vazia", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/iam/org-tree": { data: ORG },
      "GET v1/contact/messages": page([msg("5", { unidade_id: "u1" }), msg("6"), msg("7", { unidade_id: "sumiu" })]),
      "GET v1/contact/messages/6": { data: { ...msg("6"), phone: "", message: "m", consent_at: "2026-01-01T00:00:00Z", notes: "" } },
      "PATCH v1/contact/messages/6": { data: null },
    });
    renderApp(<GestaoContatoPage />);
    await waitFor(() => expect(screen.getAllByText("Protocolo")).toHaveLength(2)); // cabeçalho + setor u1
    expect(screen.getByText("Caixa geral")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Abrir 2026-6" }));
    const d = await screen.findByRole("dialog", { name: "Mensagem 2026-6" });
    const setor = await within(d).findByLabelText("Setor (encaminhar para)");
    expect(setor).toHaveValue("");
    await userEvent.selectOptions(setor, "u1");
    await userEvent.click(within(d).getByRole("button", { name: "Salvar triagem" }));
    await waitFor(() => expect(api.to("PATCH v1/contact/messages/6")[0]?.body).toMatchObject({ unidade_id: "u1" }));
  });

  it("erro ao listar", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/contact/messages": fail(500, "fora") });
    renderApp(<GestaoContatoPage />);
    expect(await screen.findByText("fora")).toBeInTheDocument();
  });
});
