import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, moduleStatus, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

const getServerSession = vi.fn();
vi.mock("next-auth/next", () => ({ getServerSession: (...a: unknown[]) => getServerSession(...a) }));
vi.mock("@/lib/auth/options", () => ({ authOptions: {} }));
const getSystemHealth = vi.fn();
vi.mock("@/lib/health/getSystemHealth", () => ({ getSystemHealth: () => getSystemHealth() }));

import BuscaPage from "./busca/page";
import DashboardOverviewPage from "./dashboard/page";
import ExemplosPage from "./exemplos/page";
import PerfilPage from "./perfil/page";

describe("Busca global", () => {
  beforeEach(() => resetNavigation({}, "/busca", "q=alvará"));

  it("resultados com filtro por módulo e aviso de resultado parcial", async () => {
    const api = mockBackend({
      ...identityRoutes({}, [moduleStatus("catalog", { name: "Catálogo", icon: "list" })]),
      "GET v1/search": {
        data: {
          took_ms: 12, modules: ["catalog", "wiki"], degraded: ["wiki"],
          results: [
            { module: "catalog", type: "service", id: "s1", title: "Alvará", snippet: "Licença", url: "/servicos/alvara", updated_at: "2026-01-01T00:00:00Z" },
            { module: "wiki", type: "page", id: "w1", title: "Como pedir alvará", snippet: "", url: "/wiki/alvara" },
          ],
        },
      },
    });
    renderApp(<BuscaPage />);
    expect(await screen.findByRole("heading", { name: "Resultados para “alvará”" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Alvará" })).toHaveAttribute("href", "/servicos/alvara");
    expect(screen.getByText(/2 resultado\(s\) em 12 ms/)).toBeInTheDocument();
    expect(screen.getByText(/wiki não respondeu a tempo/)).toHaveAttribute("role", "status");
    expect(api.to("GET v1/search")[0]!.query.get("limit")).toBe("50");

    await userEvent.click(screen.getByRole("button", { name: "Catálogo" }));
    expect(router.replace).toHaveBeenCalledWith("/busca?q=alvar%C3%A1&modulo=catalog");
    await userEvent.click(screen.getByRole("button", { name: "Todos" }));
    expect(router.replace).toHaveBeenCalledWith("/busca?q=alvar%C3%A1");

    await userEvent.clear(screen.getByLabelText("Termo"));
    await userEvent.type(screen.getByLabelText("Termo"), "a");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(router.replace).toHaveBeenCalledTimes(2); // termo curto não busca
    await userEvent.type(screen.getByLabelText("Termo"), "gua ");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(router.replace).toHaveBeenLastCalledWith("/busca?q=agua");
  });

  it("sem termo não consulta; nada encontrado; filtro ativo mantido ao buscar", async () => {
    resetNavigation({}, "/busca", "");
    const api = mockBackend({ ...identityRoutes(), "GET v1/search": { data: { took_ms: 1, modules: [], degraded: [], results: [] } } });
    const { unmount } = renderApp(<BuscaPage />);
    expect(await screen.findByRole("heading", { name: "Buscar" })).toBeInTheDocument();
    expect(api.to("GET v1/search")).toHaveLength(0);
    unmount();

    resetNavigation({}, "/busca", "q=xyz&modulo=wiki");
    renderApp(<BuscaPage />);
    expect(await screen.findByText("Nada encontrado")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(router.replace).toHaveBeenCalledWith("/busca?q=xyz&modulo=wiki");
  });
});

describe("Perfil", () => {
  beforeEach(() => resetNavigation());

  it("acesso efetivo, perfil do Diretório e pedido de anonimização", async () => {
    let requests: unknown[] = [];
    const api = mockBackend({
      ...identityRoutes(
        { name: "", username: "maria", source: "keycloak", permissions: ["a:b", "c:d"], scopes: [{ perfil: "gestor", origem: "ad", permissions: ["a:b"] }, { perfil: "servidor", origem: "manual", permissions: [] }] },
        [moduleStatus("directory")],
      ),
      "GET v1/lgpd/minhas-solicitacoes": () => ({ data: requests }),
      "GET v1/directory/me": { data: { user_id: "1", name: "Maria", username: "maria", job_title: "Analista", phone: "", extension: "", bio: "", visible: true, unidade_id: "u1", from_ad: true } },
      "GET v1/iam/org-tree": { data: [{ id: "e1", nome: "Órgão", unidades: [{ id: "u1", nome: "TI", departamentos: [{ id: "d1", nome: "Infra" }] }] }] },
      "PUT v1/directory/me": { data: null },
      "POST v1/lgpd/solicitar-exclusao": () => ((requests = [{ id: "r1", kind: "erasure", status: "pending", detail: "", created_at: "2026-01-01T00:00:00Z" }]), { data: null }),
    });
    renderApp(<PerfilPage />);
    expect(await screen.findByRole("heading", { name: "maria" })).toBeInTheDocument();
    expect(screen.getByText("Keycloak (SSO corporativo)")).toBeInTheDocument();
    expect(screen.getByText(/gestor · AD/)).toBeInTheDocument();
    expect(screen.getByText("2 permissões efetivas")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Baixar meus dados/ })).toHaveAttribute("href", "/api/backend/v1/lgpd/meus-dados");

    expect(await screen.findByText(/Lotação sincronizada do Active Directory/)).toBeInTheDocument();
    await userEvent.clear(screen.getByLabelText("Cargo / função"));
    await userEvent.type(screen.getByLabelText("Cargo / função"), "Coordenadora");
    await userEvent.type(screen.getByLabelText("Ramal"), "210");
    await userEvent.selectOptions(await screen.findByLabelText("Entidade"), "e1");
    await userEvent.selectOptions(screen.getByLabelText("Unidade"), "u1");
    await userEvent.selectOptions(screen.getByLabelText("Departamento"), "d1");
    await userEvent.click(screen.getByLabelText("Aparecer no Diretório interno"));
    await userEvent.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PUT v1/directory/me")).toHaveLength(1));
    expect(api.to("PUT v1/directory/me")[0]!.body).toEqual({
      job_title: "Coordenadora", phone: "", extension: "210", bio: "", visible: false, unidade_id: "u1", departamento_id: "d1",
    });

    await userEvent.click(screen.getByRole("button", { name: /Solicitar exclusão/ }));
    const d = await screen.findByRole("dialog", { name: "Solicitar anonimização da conta?" });
    await userEvent.click(within(d).getByRole("button", { name: "Solicitar" }));
    expect(await screen.findByText("Exclusão / anonimização", { exact: false })).toBeInTheDocument();
    // pedido em andamento bloqueia um novo
    await waitFor(() => expect(screen.getByRole("button", { name: /Solicitar exclusão/ })).toBeDisabled());
  });

  it("login local, sem lotações, sem Diretório e solicitações concluídas/recusadas", async () => {
    mockBackend({
      ...identityRoutes({ scopes: [], permissions: [] }),
      "GET v1/lgpd/minhas-solicitacoes": {
        data: [
          { id: "a", kind: "export", status: "completed", detail: "", created_at: "2026-01-01T00:00:00Z" },
          { id: "b", kind: "erasure", status: "rejected", detail: "", created_at: "2026-01-01T00:00:00Z" },
        ],
      },
    });
    renderApp(<PerfilPage />);
    expect(await screen.findByText("credencial local")).toBeInTheDocument();
    expect(screen.getByText("Nenhuma")).toBeInTheDocument();
    expect(screen.queryByText("Perfil no Diretório")).not.toBeInTheDocument();
    expect(await screen.findByText("completed")).toBeInTheDocument();
    expect(screen.getByText("rejected")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Solicitar exclusão/ })).toBeEnabled();
  });

  it("Diretório indisponível", async () => {
    mockBackend({
      ...identityRoutes({}, [moduleStatus("directory")]),
      "GET v1/lgpd/minhas-solicitacoes": { data: [] },
      "GET v1/directory/me": fail(500, "x"),
    });
    renderApp(<PerfilPage />);
    expect(await screen.findByText("Perfil do Diretório indisponível.")).toBeInTheDocument();
  });
});

describe("Exemplo (blueprint)", () => {
  beforeEach(() => resetNavigation());

  it("cria item e limpa o formulário; lista e estados", async () => {
    let items: unknown[] = [];
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/examples": () => ({ data: items }),
      "POST v1/examples": (req) => ((items = [{ id: "1", status: "active", created_at: "2026-01-01T00:00:00Z", ...(req.body as object) }]), { data: items[0] }),
    });
    renderApp(<ExemplosPage />);
    expect(await screen.findByText("Nenhum item cadastrado")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Título *"), " Primeiro ");
    await userEvent.type(screen.getByLabelText("Descrição"), "desc");
    await userEvent.click(screen.getByRole("button", { name: /Criar/ }));
    expect(await screen.findByText("Primeiro")).toBeInTheDocument();
    expect(screen.getByText("desc")).toBeInTheDocument();
    expect(api.to("POST v1/examples")[0]!.body).toEqual({ title: "Primeiro", description: "desc" });
    expect(screen.getByLabelText("Título *")).toHaveValue("");
  });

  it("sem example:manage só lista; falha ao criar mantém o formulário", async () => {
    mockBackend({ ...identityRoutes({ permissions: ["example:read"] }), "GET v1/examples": { data: [{ id: "1", title: "T", description: "", status: "active", created_at: "" }] } });
    const { unmount } = renderApp(<ExemplosPage />);
    expect(await screen.findByText("T")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Criar/ })).not.toBeInTheDocument();
    unmount();

    mockBackend({ ...identityRoutes(), "GET v1/examples": { data: [] }, "POST v1/examples": fail(422, "título obrigatório") });
    renderApp(<ExemplosPage />);
    await userEvent.type(await screen.findByLabelText("Título *"), "X");
    await userEvent.click(screen.getByRole("button", { name: /Criar/ }));
    expect(await screen.findByText("título obrigatório")).toBeInTheDocument();
    expect(screen.getByLabelText("Título *")).toHaveValue("X");
  });
});

describe("Dashboard (Server Component)", () => {
  it("saudação, estado saudável e grade de módulos", async () => {
    getServerSession.mockResolvedValueOnce({ user: { name: "Maria Silva" } });
    getSystemHealth.mockResolvedValueOnce({ status: "ok", services: { postgres: { status: "ok" } } });
    mockBackend(identityRoutes({}, [moduleStatus("blog", { name: "Blog", description: "Notícias" }), moduleStatus("iam", { name: "IAM", core: true }), moduleStatus("wiki", { name: "Wiki", enabled: false })]));
    renderApp(await DashboardOverviewPage());
    expect(screen.getByRole("heading", { name: "Olá, Maria" })).toBeInTheDocument();
    expect(screen.getByText("Saudável")).toBeInTheDocument();
    expect(await screen.findByText("Notícias")).toBeInTheDocument();
    expect(screen.getByText("Núcleo")).toBeInTheDocument();
    expect(screen.getByText(/Desativados: Wiki/)).toBeInTheDocument();
  });

  it("degradado e indisponível; sem nome", async () => {
    getServerSession.mockResolvedValueOnce({ user: { email: "joao@orgao.gov.br" } });
    getSystemHealth.mockResolvedValueOnce({ status: "degraded", services: { postgres: { status: "ok" }, rabbitmq: { status: "error" } } });
    const { unmount } = render(await DashboardOverviewPage());
    expect(screen.getByRole("heading", { name: "Olá, joao" })).toBeInTheDocument();
    expect(screen.getByText("Degradado")).toBeInTheDocument();
    expect(screen.getByText("rabbitmq")).toBeInTheDocument();
    unmount();

    getServerSession.mockResolvedValueOnce(null);
    getSystemHealth.mockResolvedValueOnce({ status: "down", services: {} });
    render(await DashboardOverviewPage());
    expect(screen.getByRole("heading", { name: "Visão geral" })).toBeInTheDocument();
    expect(screen.getByText("Indisponível")).toBeInTheDocument();
    expect(screen.getByText("Backend não respondeu")).toBeInTheDocument();
    // sem NexusProvider por perto: sem módulos, sem esqueleto
    expect(screen.queryByText(/Desativados/)).not.toBeInTheDocument();
  });
});
