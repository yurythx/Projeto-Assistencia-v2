import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, page, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import EgressPage from "./page";

const target = (id: string, extra: Record<string, unknown> = {}) => ({
  id, name: `Destino ${id}`, kind: "n8n", url: `https://n8n.gov.br/${id}`, has_secret: false, event_patterns: [], active: true, created_at: "", ...extra,
});
const delivery = (id: string, extra: Record<string, unknown> = {}) => ({
  id, target_id: "t1", target_name: "Destino t1", event_id: "e", event_type: "blog.post.published", status: "delivered", attempts: 1,
  next_attempt_at: "", created_at: "2026-01-01T00:00:00Z", ...extra,
});

describe("Configurações > Egress", () => {
  beforeEach(() => resetNavigation());

  it("destinos e entregas; testar conexão (ok e falha); reenviar entrega esgotada", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/egress/targets": { data: [target("t1", { event_patterns: ["blog.*", "tramite.*"] }), target("t2", { active: false })] },
      "GET v1/egress/deliveries": (req) =>
        req.query.get("status") === "dead"
          ? page([delivery("d2", { status: "dead", attempts: 8, last_status_code: 500, last_error: "HTTP 500", target_name: undefined })])
          : page([delivery("d1")]),
      "POST v1/egress/targets/t1/test": { data: { ok: true, status_code: 200, took_ms: 42 } },
      "POST v1/egress/targets/t2/test": { data: { ok: false, status_code: 0, error: "endereço bloqueado (SSRF)", took_ms: 1 } },
      "POST v1/egress/deliveries/d2/redeliver": { data: null },
    });
    renderApp(<EgressPage />);
    expect(await screen.findByText("blog.*, tramite.*")).toBeInTheDocument();
    expect(screen.getByText("todos os eventos")).toBeInTheDocument();
    expect(screen.getByText("Inativo")).toBeInTheDocument();
    expect(await screen.findByText("delivered")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Reenviar" })).not.toBeInTheDocument();

    const t1 = screen.getByText("https://n8n.gov.br/t1").closest("li")!;
    await userEvent.click(within(t1).getByRole("button", { name: /Testar/ }));
    expect(await screen.findByText("HTTP 200 em 42 ms")).toBeInTheDocument();
    const t2 = screen.getByText("https://n8n.gov.br/t2").closest("li")!;
    await userEvent.click(within(t2).getByRole("button", { name: /Testar/ }));
    expect(await screen.findByText("endereço bloqueado (SSRF)")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Situação"), "dead");
    expect(await screen.findByText("HTTP 500", { selector: "span.text-danger" })).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Reenviar" }));
    await waitFor(() => expect(api.to("POST v1/egress/deliveries/d2/redeliver")).toHaveLength(1));
  });

  it("falha no teste sem mensagem mostra o código HTTP", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/egress/targets": { data: [target("t1")] },
      "GET v1/egress/deliveries": page([]),
      "POST v1/egress/targets/t1/test": { data: { ok: false, status_code: 404, took_ms: 3 } },
    });
    renderApp(<EgressPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Testar/ }));
    expect(await screen.findByText("Falha no teste")).toBeInTheDocument();
    expect(screen.getByText("HTTP 404")).toBeInTheDocument();
    expect(screen.getByText("Nenhuma entrega")).toBeInTheDocument();
  });

  it("cria destino com segredo e padrões; edita mantendo o segredo; exclui", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/egress/targets": { data: [target("t1", { has_secret: true, event_patterns: ["a.b"] })] },
      "GET v1/egress/deliveries": page([]),
      "POST v1/egress/targets": { data: target("novo") },
      "PUT v1/egress/targets/t1": { data: target("t1") },
      "DELETE v1/egress/targets/t1": { data: null },
      "POST v1/egress/targets/t1/test": fail(503, "egress desativado"),
    });
    renderApp(<EgressPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Novo destino/ }));
    let dialog = await screen.findByRole("dialog", { name: "Novo destino" });
    await userEvent.type(within(dialog).getByLabelText("Nome *"), " Zabbix ");
    await userEvent.selectOptions(within(dialog).getByLabelText("Tipo"), "zabbix");
    await userEvent.type(within(dialog).getByLabelText("URL *"), "https://zabbix.gov.br/hook");
    await userEvent.type(within(dialog).getByLabelText("Segredo HMAC"), "s3gr3d0");
    await userEvent.type(within(dialog).getByLabelText("Eventos (um por linha; curinga *)"), "blog.*{Enter}tramite.processo.aberto, signum.*");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar destino" }));
    await waitFor(() => expect(api.to("POST v1/egress/targets")).toHaveLength(1));
    expect(api.to("POST v1/egress/targets")[0]!.body).toEqual({
      name: "Zabbix", kind: "zabbix", url: "https://zabbix.gov.br/hook", secret: "s3gr3d0",
      event_patterns: ["blog.*", "tramite.processo.aberto", "signum.*"], active: true,
    });

    await userEvent.click(screen.getByRole("button", { name: "Editar Destino t1" }));
    dialog = await screen.findByRole("dialog", { name: "Editar destino" });
    expect(within(dialog).getByLabelText("Segredo HMAC (deixe em branco para manter)")).toHaveValue("");
    await userEvent.click(within(dialog).getByLabelText("Ativo"));
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar destino" }));
    await waitFor(() => expect(api.to("PUT v1/egress/targets/t1")).toHaveLength(1));
    // em branco: o segredo atual é mantido (campo ausente no corpo)
    expect(api.to("PUT v1/egress/targets/t1")[0]!.body).toEqual({ name: "Destino t1", kind: "n8n", url: "https://n8n.gov.br/t1", event_patterns: ["a.b"], active: false });

    await userEvent.click(screen.getByRole("button", { name: /Testar/ }));
    expect(await screen.findByText("egress desativado")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Excluir Destino t1" }));
    const confirm = await screen.findByRole("dialog", { name: 'Excluir o destino "Destino t1"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    await waitFor(() => expect(api.to("DELETE v1/egress/targets/t1")).toHaveLength(1));
  });

  it("sem destinos e erro nas entregas", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/egress/targets": { data: [] }, "GET v1/egress/deliveries": fail(500, "sem banco") });
    renderApp(<EgressPage />);
    expect(await screen.findByText("Nenhum destino configurado")).toBeInTheDocument();
    expect(await screen.findByText("sem banco")).toBeInTheDocument();
  });
});
