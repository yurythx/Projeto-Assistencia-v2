import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, ME, moduleStatus, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import DiretorioPage from "./page";
import PessoaPage from "./[id]/page";

const person = (id: string, extra: Record<string, unknown> = {}) => ({
  user_id: id, name: `Pessoa ${id}`, username: `p${id}`, email: `${id}@orgao.gov.br`, job_title: "Analista", phone: "3333-0000",
  extension: "201", bio: "", visible: true, unidade: "TI", departamento: "Infra", from_ad: false, ...extra,
});
const ORG = [{ id: "e1", nome: "Órgão", unidades: [{ id: "u1", nome: "TI", departamentos: [{ id: "d1", nome: "Infra" }] }] }];

describe("Diretório", () => {
  beforeEach(() => resetNavigation());

  it("busca pessoas, filtra por lotação e abre conversa no Mercúrio", async () => {
    const api = mockBackend({
      ...identityRoutes({}, [moduleStatus("mercurio")]),
      "GET v1/directory/people": (req) =>
        req.query.get("q") === "ninguém"
          ? page([])
          : page([person("1", { name: "Ana Maria Souza", from_ad: true }), person("2", { name: "", job_title: "", departamento: "", unidade: "", email: "", phone: "", extension: "" }), person(ME.id)]),
      "GET v1/iam/org-tree": { data: ORG },
      "POST v1/mercurio/direct": { data: { id: "sala-dm" } },
    });
    renderApp(<DiretorioPage />);
    expect(await screen.findByText("Ana Maria Souza")).toBeInTheDocument();
    expect(screen.getByText("AM")).toBeInTheDocument();
    expect(screen.getByText("AD")).toBeInTheDocument();
    expect(screen.getByText("p2")).toBeInTheDocument();
    expect(screen.getAllByText(/ramal 201/).length).toBeGreaterThan(0);
    // não conversa consigo mesmo
    expect(screen.queryByRole("button", { name: `Conversar com Pessoa ${ME.id}` })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Conversar com Ana Maria Souza" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/mercurio?sala=sala-dm"));
    expect(api.to("POST v1/mercurio/direct")[0]!.body).toEqual({ user_id: "1" });

    await userEvent.selectOptions(await screen.findByLabelText("Entidade"), "e1");
    await userEvent.selectOptions(screen.getByLabelText("Unidade"), "u1");
    await userEvent.selectOptions(screen.getByLabelText("Departamento"), "d1");
    await waitFor(() => expect(api.to("GET v1/directory/people").at(-1)!.query.get("departamento_id")).toBe("d1"));
    expect(api.to("GET v1/directory/people").at(-1)!.query.get("unidade_id")).toBe("u1");

    await userEvent.type(screen.getByLabelText("Nome, cargo ou e-mail"), " ninguém ");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    expect(await screen.findByText("Ninguém encontrado")).toBeInTheDocument();
  });

  it("sem Mercúrio não oferece conversa; aba de setores", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/directory/people": page([person("1")]),
      "GET v1/iam/org-tree": { data: [] },
      "GET v1/directory/sectors": {
        data: [
          { id: "u1", kind: "unidade", nome: "Secretaria", sigla: "SEC", email: "sec@orgao.gov.br", telefone: "3333", endereco: "Rua A", entidade: "Órgão" },
          { id: "d1", kind: "departamento", nome: "Protocolo", sigla: "", email: "", telefone: "", unidade: "Secretaria", entidade: "Órgão" },
        ],
      },
    });
    renderApp(<DiretorioPage />);
    await screen.findByText("Pessoa 1");
    expect(screen.queryByRole("button", { name: /Conversar/ })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "Setores" }));
    expect(await screen.findByText("Unidade · Órgão")).toBeInTheDocument();
    expect(screen.getByText("Departamento · Secretaria")).toBeInTheDocument();
    expect(screen.getByText("Rua A")).toBeInTheDocument();
  });

  it("falha ao abrir conversa e erros de carga", async () => {
    mockBackend({
      ...identityRoutes({}, [moduleStatus("mercurio")]),
      "GET v1/directory/people": page([person("1")]),
      "GET v1/iam/org-tree": { data: [] },
      "POST v1/mercurio/direct": fail(403, "conversa não permitida"),
      "GET v1/directory/sectors": fail(500, "setores fora"),
    });
    renderApp(<DiretorioPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Conversar com Pessoa 1" }));
    expect(await screen.findByText("conversa não permitida")).toBeInTheDocument();
    expect(router.push).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("tab", { name: "Setores" }));
    expect(await screen.findByText("setores fora")).toBeInTheDocument();
  });

  it("sem setores", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/directory/people": page([]), "GET v1/iam/org-tree": { data: [] }, "GET v1/directory/sectors": { data: [] } });
    renderApp(<DiretorioPage />);
    await userEvent.click(await screen.findByRole("tab", { name: "Setores" }));
    expect(await screen.findByText("Nenhum setor cadastrado")).toBeInTheDocument();
  });
});

describe("Diretório — pessoa", () => {
  it("mostra o cartão completo", async () => {
    resetNavigation({ id: "7" });
    mockBackend({ ...identityRoutes(), "GET v1/directory/people/7": { data: person("7", { from_ad: true, bio: "Atende\nde manhã" }) } });
    renderApp(<PessoaPage />);
    expect(await screen.findByRole("heading", { name: "Pessoa 7" })).toBeInTheDocument();
    expect(screen.getByText("Infra · TI")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "7@orgao.gov.br" })).toHaveAttribute("href", "mailto:7@orgao.gov.br");
    expect(screen.getByText(/ramal 201/)).toBeInTheDocument();
    expect(screen.getByText(/Atende/)).toBeInTheDocument();
  });

  it("cartão mínimo e pessoa inexistente", async () => {
    resetNavigation({ id: "8" });
    mockBackend({ ...identityRoutes(), "GET v1/directory/people/8": { data: person("8", { name: "", job_title: "", departamento: "", unidade: "", email: "", phone: "", extension: "" }) } });
    const { unmount } = renderApp(<PessoaPage />);
    expect(await screen.findByRole("heading", { name: "p8" })).toBeInTheDocument();
    unmount();
    resetNavigation({ id: "9" });
    mockBackend({ ...identityRoutes(), "GET v1/directory/people/9": fail(404, "pessoa não encontrada") });
    renderApp(<PessoaPage />);
    expect(await screen.findByText("pessoa não encontrada")).toBeInTheDocument();
  });
});
