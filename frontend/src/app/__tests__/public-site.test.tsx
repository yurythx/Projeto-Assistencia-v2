import { createHash } from "node:crypto";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("@/lib/env", async (orig) => ({ ...(await orig<typeof import("@/lib/env")>()), APP_URL: "https://portal.gov.br" }));
vi.mock("@/components/layout/PublicShell", () => ({ PublicShell: ({ children }: { children: ReactNode }) => <main>{children}</main> }));
vi.mock("@/components/layout/AccessibilityBar", () => ({ AccessibilityBar: () => <nav aria-label="barra de acessibilidade" /> }));
vi.mock("@/components/auth/LoginCard", () => ({ LoginCard: () => <p>cartão de login</p> }));
vi.mock("@/components/contact/ContactForm", () => ({ ContactForm: ({ defaultServiceSlug }: { defaultServiceSlug?: string }) => <p>formulário ({defaultServiceSlug ?? "—"})</p> }));
vi.mock("next/server", () => ({ connection: async () => {} }));
vi.mock("next/headers", () => ({ cookies: async () => ({ get: (n: string) => (n === "nexus-theme" ? { value: "dark" } : undefined) }) }));

class NotFound extends Error {}
class Redirect extends Error {}
vi.mock("next/navigation", () => ({
  notFound: () => {
    throw new NotFound("notFound");
  },
  redirect: (to: string) => {
    throw new Redirect(to);
  },
  useRouter: () => ({ push: vi.fn() }),
}));

const api = vi.hoisted(() => ({ routes: {} as Record<string, unknown> }));
vi.mock("@/lib/api/publicServer", async () => {
  class PublicApiError extends Error {
    constructor(
      readonly status: number,
      readonly code: string,
      message: string,
    ) {
      super(message);
    }
  }
  const lookup = (path: string) => {
    const key = Object.keys(api.routes).find((k) => path.startsWith(k));
    const v = key ? api.routes[key] : new PublicApiError(404, "NOT_FOUND", "x");
    if (v instanceof Error) throw v;
    return v;
  };
  return {
    PublicApiError,
    publicGet: async (path: string) => ({ data: lookup(path) }),
    publicGetOr: async (path: string, fallback: unknown) => {
      try {
        return lookup(path);
      } catch {
        return fallback;
      }
    },
  };
});

import { PublicApiError } from "@/lib/api/publicServer";

import LandingPage from "../page";
import AccessibilityPage from "../acessibilidade/page";
import ContatoPage from "../contato/page";
import EventosPage from "../eventos/page";
import LoginPage from "../login/page";
import PrivacidadePage from "../privacidade/page";
import robots from "../robots";
import ServicosPage from "../servicos/page";
import ServicoPage, { generateMetadata } from "../servicos/[slug]/page";
import SetoresPage from "../setores/page";
import sitemap from "../sitemap";
import AboutPage from "../sobre/page";
import TransparenciaPage from "../transparencia/page";
import VerificarIndex from "../verificar/page";
import VerificarPage from "../verificar/[id]/page";
import { verificar } from "../verificar/actions";

const BRANDING = { app_name: "Portal X", org_name: "Município X", app_description: "", support_email: "a@x.gov.br", support_phone: "" };
const svc = (extra: Record<string, unknown> = {}) => ({
  id: "s1", slug: "alvara", title: "Alvará", summary: "Licença", description: "## Detalhes", category: "Empresas", audience: "Empresas",
  requirements: ["CNPJ"], steps: ["Solicitar"], sla: "15 dias", cost: "", icon: "briefcase", responsible_unidade: "Protocolo",
  channels: [
    { type: "online", label: "Portal", value: "https://portal.gov.br/alvara" },
    { type: "online", label: "Link inseguro", value: "javascript:alert(1)" },
    { type: "email", label: "E-mail", value: "alvara@x.gov.br" },
    { type: "telefone", label: "Telefone", value: "(66) 3411-5000" },
    { type: "presencial", label: "Sede", value: "Rua A" },
  ],
  ...extra,
});

beforeEach(() => {
  api.routes = { branding: BRANDING };
});

describe("páginas institucionais estáticas", () => {
  it("início, sobre, privacidade e acessibilidade usam a identidade da organização", async () => {
    for (const Page of [LandingPage, AboutPage, PrivacidadePage, AccessibilityPage]) {
      const { unmount, container } = render(await Page());
      expect(container.textContent).toMatch(/Portal X|Município X/);
      unmount();
    }
    api.routes = {}; // branding fora: cai no padrão
    const { container } = render(await AboutPage());
    expect(container.textContent).toContain("Projeto Nexus");
  });

  it("login mostra a marca e o cartão", async () => {
    render(await LoginPage());
    expect(screen.getAllByText("Portal X").length).toBeGreaterThan(0);
    expect(screen.getByText("cartão de login")).toBeInTheDocument();
    expect(screen.getByText("Entre com sua conta corporativa (SSO) ou credencial local.")).toBeInTheDocument();
  });
});

describe("carta de serviços", () => {
  it("lista publicada, vazia e módulo desativado (404)", async () => {
    api.routes["catalog/services?"] = [svc(), svc({ id: "s2", slug: "iptu", title: "IPTU", category: "" })];
    render(await ServicosPage());
    expect(screen.getByRole("link", { name: /Alvará/ })).toHaveAttribute("href", "/servicos/alvara");
    expect(screen.getByRole("link", { name: /IPTU/ })).toBeInTheDocument();

    api.routes["catalog/services?"] = [];
    const { container } = render(await ServicosPage());
    expect(container.textContent).toContain("Nenhum serviço publicado ainda");

    delete api.routes["catalog/services?"];
    await expect(ServicosPage()).rejects.toBeInstanceOf(NotFound);
    api.routes["catalog/services?"] = new PublicApiError(503, "DOWN", "fora");
    await expect(ServicosPage()).rejects.toMatchObject({ status: 503 });
  });

  it("detalhe: canais seguros, requisitos, etapas e metadados", async () => {
    api.routes["catalog/services/alvara"] = svc();
    render(await ServicoPage({ params: Promise.resolve({ slug: "alvara" }) }));
    expect(screen.getByRole("heading", { name: "Alvará", level: 1 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Detalhes" })).toBeInTheDocument();
    expect(screen.getByText("Gratuito")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "https://portal.gov.br/alvara" })).toHaveAttribute("rel", "noopener noreferrer");
    // javascript: não vira link
    expect(screen.getByText("javascript:alert(1)").tagName).toBe("SPAN");
    expect(screen.getByRole("link", { name: "alvara@x.gov.br" })).toHaveAttribute("href", "mailto:alvara@x.gov.br");
    expect(screen.getByRole("link", { name: "(66) 3411-5000" })).toHaveAttribute("href", "tel:6634115000");
    expect(screen.getByText("Unidade responsável: Protocolo")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Fale conosco/ })).toHaveAttribute("href", "/contato?servico=alvara");

    expect(await generateMetadata({ params: Promise.resolve({ slug: "alvara" }) })).toMatchObject({
      title: "Alvará", alternates: { canonical: "https://portal.gov.br/servicos/alvara" },
    });
    expect(await generateMetadata({ params: Promise.resolve({ slug: "nada" }) })).toEqual({ title: "Serviço não encontrado" });
    await expect(ServicoPage({ params: Promise.resolve({ slug: "nada" }) })).rejects.toBeInstanceOf(NotFound);

    api.routes["catalog/services/min"] = svc({ audience: "", sla: "", cost: "R$ 10", description: "", requirements: [], steps: [], channels: [], responsible_unidade: "", category: "" });
    const { container } = render(await ServicoPage({ params: Promise.resolve({ slug: "min" }) }));
    expect(container.textContent).toContain("R$ 10");
    expect(container.textContent).not.toContain("Canais de atendimento");

    api.routes["catalog/services/erro"] = new PublicApiError(500, "E", "x");
    await expect(ServicoPage({ params: Promise.resolve({ slug: "erro" }) })).rejects.toMatchObject({ status: 500 });
  });
});

describe("setores e agenda pública", () => {
  it("setores com departamentos agrupados", async () => {
    api.routes["directory/public/sectors"] = [
      { id: "u1", kind: "unidade", nome: "Saúde", sigla: "SMS", email: "s@x.gov.br", telefone: "3333", endereco: "Rua B", entidade: "Município" },
      { id: "u2", kind: "unidade", nome: "Obras", sigla: "", email: "", telefone: "", entidade: "Município" },
      { id: "d1", kind: "departamento", nome: "Vigilância", sigla: "", email: "", telefone: "4444", unidade_id: "u1", entidade: "Município" },
    ];
    render(await SetoresPage());
    expect(screen.getByRole("heading", { name: /Saúde/ })).toBeInTheDocument();
    expect(screen.getByText("Departamentos")).toBeInTheDocument();
    expect(screen.getByText(/Vigilância/)).toBeInTheDocument();
    delete api.routes["directory/public/sectors"];
    await expect(SetoresPage()).rejects.toBeInstanceOf(NotFound);
    api.routes["directory/public/sectors"] = new PublicApiError(500, "E", "x");
    await expect(SetoresPage()).rejects.toMatchObject({ status: 500 });
  });

  it("próximos eventos públicos", async () => {
    api.routes["calendar/public/events"] = [
      { id: "e1", title: "Audiência pública", starts_at: "2026-03-10T13:00:00Z", ends_at: "2026-03-10T15:00:00Z", all_day: false, location: "Câmara", description: "Orçamento" },
      { id: "e2", title: "Feriado", starts_at: "2026-03-11T00:00:00Z", ends_at: "2026-03-11T23:59:00Z", all_day: true, room_name: "", location: "" },
    ];
    render(await EventosPage());
    expect(screen.getByRole("heading", { name: "Audiência pública" })).toBeInTheDocument();
    expect(screen.getByText("Dia inteiro")).toBeInTheDocument();
    expect(screen.getByText("Orçamento")).toBeInTheDocument();
    api.routes["calendar/public/events"] = [];
    const { container } = render(await EventosPage());
    expect(container.textContent).toContain("Nenhum evento público programado.");
    delete api.routes["calendar/public/events"];
    await expect(EventosPage()).rejects.toBeInstanceOf(NotFound);
    api.routes["calendar/public/events"] = new PublicApiError(502, "E", "x");
    await expect(EventosPage()).rejects.toMatchObject({ status: 502 });
  });
});

describe("transparência, contato, sitemap e robots", () => {
  it("dados abertos com links pelo proxy público", async () => {
    api.routes["transparencia/datasets"] = [
      { id: "d1", titulo: "Serviços", endpoint: "/api/v1/transparencia/servicos", campos: { titulo: "nome do serviço" }, periodicidade: "diária", formatos: ["json", "csv"] },
      { id: "d2", titulo: "Estatísticas", endpoint: "/api/v1/transparencia/stats?dias=30", campos: {}, periodicidade: "diária", formatos: ["xml"] },
    ];
    render(await TransparenciaPage());
    expect(screen.getByRole("link", { name: /csv/ })).toHaveAttribute("href", "/api/public/v1/transparencia/servicos?format=csv");
    expect(screen.getByRole("link", { name: /xml/ })).toHaveAttribute("href", "/api/public/v1/transparencia/stats?dias=30&format=xml");
    expect(screen.getByText("— nome do serviço")).toBeInTheDocument();
    api.routes = {};
    const { container } = render(await TransparenciaPage());
    expect(container.textContent).toContain("Catálogo de dados abertos indisponível no momento.");
  });

  it("contato só com o módulo ativo; serviço pré-selecionado", async () => {
    api.routes["system/public-modules"] = [{ key: "contact", enabled: true }];
    render(await ContatoPage({ searchParams: Promise.resolve({ servico: "alvara" }) }));
    expect(screen.getByText("formulário (alvara)")).toBeInTheDocument();
    api.routes["system/public-modules"] = [{ key: "contact", enabled: false }];
    await expect(ContatoPage({ searchParams: Promise.resolve({}) })).rejects.toBeInstanceOf(NotFound);
  });

  it("sitemap lista só as seções de módulos ativos; robots bloqueia a área autenticada", async () => {
    api.routes["system/public-modules"] = [{ key: "catalog", enabled: true }, { key: "calendar", enabled: false }];
    const urls = (await sitemap()).map((e) => e.url);
    expect(urls).toContain("https://portal.gov.br/servicos");
    expect(urls).not.toContain("https://portal.gov.br/eventos");
    expect(urls).toContain("https://portal.gov.br/privacidade");
    const r = robots();
    expect(r.sitemap).toBe("https://portal.gov.br/sitemap.xml");
    expect((r.rules as { disallow: string[] }).disallow).toContain("/configuracao");
  });
});

describe("verificação pública de assinatura", () => {
  const ID = "11111111-2222-3333-4444-555555555555";
  const DOC = "contrato";
  const SHA = createHash("sha256").update(DOC).digest("hex");

  it("formulário: aviso de erro; a action redireciona conforme o código", async () => {
    const { unmount } = render(await VerificarIndex({ searchParams: Promise.resolve({ erro: "1" }) }));
    expect(screen.getByText("Código inválido — confira os 36 caracteres.")).toBeInTheDocument();
    unmount();
    render(await VerificarIndex({ searchParams: Promise.resolve({}) }));
    expect(screen.queryByText(/Código inválido/)).not.toBeInTheDocument();

    const fd = (codigo: string) => {
      const f = new FormData();
      f.set("codigo", codigo);
      return f;
    };
    await expect(verificar(fd(` ${ID.toUpperCase()} `))).rejects.toThrow(`/verificar/${ID}`);
    await expect(verificar(fd("não-é-código"))).rejects.toThrow("/verificar?erro=1");
    await expect(verificar(new FormData())).rejects.toThrow("/verificar?erro=1");
  });

  it("mostra assinaturas válidas, inválidas, pendentes e recusadas; confere um arquivo", async () => {
    api.routes[`signum/verify/${ID}`] = {
      envelope_id: ID, title: "Contrato 12/2026", document_sha256: SHA, status: "completed", checked_at: "2026-01-02T00:00:00Z",
      signatures: [
        { name: "Ana", status: "signed", signed_at: "2026-01-01T10:00:00Z", method: "local-argon2id", valid: true },
        { name: "Beto", status: "signed", signed_at: "2026-01-01T11:00:00Z", method: "keycloak", valid: false },
        { name: "Caio", status: "pending", valid: false },
        { name: "Dora", status: "refused", valid: false },
      ],
    };
    const f = vi.fn(async (url: string) =>
      new Response(JSON.stringify({ data: { document_match: url.includes(SHA) }, error: null }), { status: 200 }),
    );
    vi.stubGlobal("fetch", f);
    render(await VerificarPage({ params: Promise.resolve({ id: ID }) }));
    expect(screen.getByText("Todas as assinaturas concluídas")).toBeInTheDocument();
    expect(screen.getByText("Válida")).toBeInTheDocument();
    expect(screen.getByText("Inválida")).toBeInTheDocument();
    expect(screen.getByText("Pendente")).toBeInTheDocument();
    expect(screen.getByText("Recusou")).toBeInTheDocument();

    await userEvent.upload(screen.getByLabelText("Conferir um arquivo"), new File([DOC], "c.pdf"));
    expect(await screen.findByText("O arquivo é idêntico ao documento assinado.")).toBeInTheDocument();
    expect(f.mock.calls[0]![0]).toBe(`/api/public/v1/signum/verify/${ID}?document_sha256=${SHA}`);
    await userEvent.upload(screen.getByLabelText("Conferir um arquivo"), new File(["outro"], "o.pdf"));
    expect(await screen.findByText("O arquivo é diferente do documento assinado.")).toBeInTheDocument();

    f.mockImplementationOnce(async () => { throw new Error("rede"); });
    await userEvent.upload(screen.getByLabelText("Conferir um arquivo"), new File(["x"], "x.pdf"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Não foi possível verificar agora");
    vi.unstubAllGlobals();
  });

  it("código malformado ou inexistente é 404; outros erros sobem", async () => {
    await expect(VerificarPage({ params: Promise.resolve({ id: "../x" }) })).rejects.toBeInstanceOf(NotFound);
    await expect(VerificarPage({ params: Promise.resolve({ id: ID }) })).rejects.toBeInstanceOf(NotFound);
    api.routes[`signum/verify/${ID}`] = new PublicApiError(500, "E", "x");
    await expect(VerificarPage({ params: Promise.resolve({ id: ID }) })).rejects.toMatchObject({ status: 500 });
  });
});
