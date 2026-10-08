import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import AuditoriaPage from "./page";

const rec = (pos: number, extra: Record<string, unknown> = {}) => ({
  id: `r${pos}`, chain_pos: pos, actor_roles: [], entity_context: {}, action: "iam.perfil.updated", metadata: {},
  timestamp_utc: "2026-01-01T10:00:00Z", prev_hash: "aa", hash: "bb", ...extra,
});

describe("Auditoria", () => {
  beforeEach(() => resetNavigation());

  it("auditoria com escopo (ADR 013) avisa que a trilha vale na área; a global não", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["audit:read"], scopes: [{ perfil: "p", origem: "manual", entidade_id: "e1", unidade_id: "u1", permissions: ["audit:read"] }] }),
      "GET v1/audit/logs": page([rec(1)]),
    });
    const { unmount } = renderApp(<AuditoriaPage />);
    expect(await screen.findByRole("note")).toHaveTextContent("vale na sua área");
    expect(screen.queryByRole("button", { name: /Verificar integridade/ })).not.toBeInTheDocument();
    unmount();
    mockBackend({ ...identityRoutes(), "GET v1/audit/logs": page([rec(1)]) });
    renderApp(<AuditoriaPage />);
    expect(await screen.findByRole("button", { name: /Verificar integridade/ })).toBeInTheDocument();
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
  });

  it("lista, filtra e monta a exportação LAI com os filtros aplicados", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/audit/logs": (req) =>
        req.query.get("action") === "falha" ? fail(500, "trilha indisponível") : page([rec(2, { actor_name: "Maria", ip_address: "10.0.0.1", resource_type: "perfil" }), rec(1)]),
    });
    renderApp(<AuditoriaPage />);
    expect(await screen.findByText("Maria")).toBeInTheDocument();
    expect(screen.getByText("sistema")).toBeInTheDocument();
    expect(screen.getByText("10.0.0.1")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Ação"), " login.failed ");
    await userEvent.type(screen.getByLabelText("Tipo de recurso"), "user");
    await userEvent.type(screen.getByLabelText("De"), "2026-01-01");
    await userEvent.type(screen.getByLabelText("Até"), "2026-01-31");
    await userEvent.click(screen.getByRole("button", { name: "Filtrar" }));
    await waitFor(() => expect(api.to("GET v1/audit/logs").at(-1)!.query.get("action")).toBe("login.failed"));
    expect(api.to("GET v1/audit/logs").at(-1)!.query.get("resource_type")).toBe("user");

    await userEvent.selectOptions(screen.getByLabelText("Formato de exportação"), "xml");
    const href = screen.getByRole("link", { name: /Exportar \(LAI\)/ }).getAttribute("href")!;
    expect(href).toBe("/api/backend/v1/audit/export?from=2026-01-01&to=2026-01-31&action=login.failed&format=xml");

    await userEvent.clear(screen.getByLabelText("Ação"));
    await userEvent.type(screen.getByLabelText("Ação"), "falha");
    await userEvent.click(screen.getByRole("button", { name: "Filtrar" }));
    expect(await screen.findByText("trilha indisponível")).toBeInTheDocument();
  });

  it("detalhe do registro com diff, contexto, metadados e hashes", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/audit/logs": page([
        rec(7, {
          actor_subject: "sub-9", actor_roles: ["nexus-admin"], user_agent: "Mozilla", correlation_id: "req-1", resource_type: "perfil", resource_id: "p1",
          diff_before: { nome: "A" }, diff_after: { nome: "B" }, entity_context: { unidade_id: "u1" }, metadata: { motivo: "ajuste" },
        }),
        rec(6),
      ]),
    });
    renderApp(<AuditoriaPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Detalhes do registro 7" }));
    const d = await screen.findByRole("dialog", { name: "Registro #7" });
    expect(within(d).getByText("nexus-admin")).toBeInTheDocument();
    expect(within(d).getByText("sub-9")).toBeInTheDocument();
    expect(within(d).getByText(/"nome": "A"/)).toBeInTheDocument();
    expect(within(d).getByText(/"nome": "B"/)).toBeInTheDocument();
    expect(within(d).getByText(/"motivo": "ajuste"/)).toBeInTheDocument();
    expect(within(d).getByText("req-1")).toBeInTheDocument();
    expect(within(d).getByText("posição #7")).toBeInTheDocument();
    expect(within(d).getByText("hash: bb")).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");

    await userEvent.click(screen.getByRole("button", { name: "Detalhes do registro 6" }));
    const d6 = await screen.findByRole("dialog", { name: "Registro #6" });
    expect(within(d6).getByText("sistema")).toBeInTheDocument();
    expect(within(d6).queryByText("Metadados")).not.toBeInTheDocument();
  });

  it("verificação da cadeia: íntegra e violada", async () => {
    let valid = true;
    mockBackend({
      ...identityRoutes(),
      "GET v1/audit/logs": page([]),
      "GET v1/audit/verify": () => ({
        data: valid ? { checked: 120, valid: true, verified_at: "2026-01-02T00:00:00Z" } : { checked: 50, valid: false, first_invalid_pos: 42, reason: "hash divergente", verified_at: "2026-01-02T00:00:00Z" },
      }),
    });
    renderApp(<AuditoriaPage />);
    expect(await screen.findByText("Nenhum registro no período")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Verificar integridade/ }));
    expect(await screen.findByText("Cadeia íntegra")).toBeInTheDocument();
    expect(screen.getByText(/120 registros verificados/)).toBeInTheDocument();
    valid = false;
    await userEvent.click(screen.getByRole("button", { name: /Verificar integridade/ }));
    expect(await screen.findByText("Cadeia violada")).toBeInTheDocument();
    expect(screen.getByText(/Primeira divergência na posição #42: hash divergente/)).toBeInTheDocument();
  });

  it("sem audit:verify nem audit:read não há verificação nem exportação; verificação com erro", async () => {
    mockBackend({ ...identityRoutes({ permissions: ["audit:list"] }), "GET v1/audit/logs": page([]) });
    const { unmount } = renderApp(<AuditoriaPage />);
    await screen.findByText("Nenhum registro no período");
    expect(screen.queryByRole("button", { name: /Verificar integridade/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Exportar/ })).not.toBeInTheDocument();
    unmount();

    mockBackend({ ...identityRoutes(), "GET v1/audit/logs": page([]), "GET v1/audit/verify": fail(503, "verificação indisponível") });
    renderApp(<AuditoriaPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Verificar integridade/ }));
    expect(await screen.findByText("verificação indisponível")).toBeInTheDocument();
    expect(screen.queryByText(/Cadeia/)).not.toBeInTheDocument();
  });
});
