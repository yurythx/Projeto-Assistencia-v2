import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { Workflow } from "@/lib/nexus/types";
import { identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import { ValidacaoProcedimento } from "./ValidacaoProcedimento";
import { WorkflowList } from "./WorkflowList";

const wf = (extra: Partial<Workflow> = {}) =>
  ({
    id: "wf-1",
    codigo_processual: "ADM.FER.001",
    titulo: "Férias",
    objetivo: "Conceder férias",
    publico_alvo: "Servidores",
    versao: 1,
    ativo: false,
    situacao: "RASCUNHO",
    nivel_acesso: "PUBLICO",
    hipotese_legal_restricao: "",
    codigo_ttdd: "2.0.06.03.01",
    total_etapas: 2,
    etapas: [],
    created_at: "",
    updated_at: "",
    ...extra,
  }) as Workflow;

const ENTREVISTA = {
  id: "v1",
  codigo_processual: "ADM.FER.001",
  realizada_em: "2026-10-01T00:00:00Z",
  unidade: "Gestão de Pessoas",
  participantes: "Chefe do RH",
  registro: "Etapas conferidas.",
  pendencias: "Falta o modelo do requerimento",
  created_at: "",
  created_by: "admin",
};

describe("validação do procedimento (ADR 028)", () => {
  beforeEach(() => resetNavigation({ id: "wf-1" }, "/atlas/procedimentos/wf-1"));

  it("rascunho: inicia a validação, registra entrevista e homologa com confirmação", async () => {
    const onAtualizado = vi.fn();
    const api = mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows/wf-1/validacoes": { data: [ENTREVISTA] },
      "POST v1/atlas/admin/workflows/wf-1/situacao": { data: wf({ situacao: "EM_VALIDACAO" }) },
      "POST v1/atlas/admin/workflows/wf-1/validacoes": { status: 201, data: ENTREVISTA },
      "POST v1/atlas/admin/workflows/wf-1/homologar": {
        data: wf({ situacao: "HOMOLOGADO", ativo: true }),
      },
    });
    renderApp(<ValidacaoProcedimento wf={wf()} onAtualizado={onAtualizado} />);
    expect(screen.getByText("Rascunho")).toBeInTheDocument();
    expect(screen.getByText(/Não publicado/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Iniciar validação/ }));
    await waitFor(() =>
      expect(api.to("POST v1/atlas/admin/workflows/wf-1/situacao")[0]!.body).toEqual({
        situacao: "EM_VALIDACAO",
      }),
    );

    const lista = await screen.findByRole("list", { name: "Entrevistas de validação" });
    expect(within(lista).getByText(/Gestão de Pessoas · Chefe do RH/)).toBeInTheDocument();
    expect(
      within(lista).getByText("Pendências: Falta o modelo do requerimento"),
    ).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Departamento *"), "Gestão de Pessoas");
    await userEvent.type(
      screen.getByLabelText("O que foi validado ou corrigido *"),
      "Prazos ajustados.",
    );
    await userEvent.click(screen.getByRole("button", { name: "Registrar" }));
    await waitFor(() =>
      expect(api.to("POST v1/atlas/admin/workflows/wf-1/validacoes")).toHaveLength(1),
    );
    expect(api.to("POST v1/atlas/admin/workflows/wf-1/validacoes")[0]!.body).toMatchObject({
      unidade: "Gestão de Pessoas",
      registro: "Prazos ajustados.",
    });

    await userEvent.click(screen.getByRole("button", { name: /^Homologar$/ }));
    expect(screen.getByText(/substitui a anterior/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Cancelar" }));
    expect(api.to("POST v1/atlas/admin/workflows/wf-1/homologar")).toHaveLength(0);
    await userEvent.click(screen.getByRole("button", { name: /^Homologar$/ }));
    await userEvent.click(screen.getByRole("button", { name: "Homologar e publicar" }));
    await waitFor(() =>
      expect(api.to("POST v1/atlas/admin/workflows/wf-1/homologar")).toHaveLength(1),
    );
    expect(onAtualizado).toHaveBeenCalled();
  });

  it("em validação volta a rascunho; homologado (ou sem situação) não tem botões", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "GET v1/atlas/admin/workflows/wf-1/validacoes": { data: [] },
      "POST v1/atlas/admin/workflows/wf-1/situacao": { data: wf() },
    });
    const { unmount } = renderApp(
      <ValidacaoProcedimento wf={wf({ situacao: "EM_VALIDACAO" })} onAtualizado={() => {}} />,
    );
    expect(
      await screen.findByText("Nenhuma entrevista registrada para este procedimento."),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Voltar a rascunho" }));
    await waitFor(() =>
      expect(api.to("POST v1/atlas/admin/workflows/wf-1/situacao")[0]!.body).toEqual({
        situacao: "RASCUNHO",
      }),
    );
    unmount();
    renderApp(
      <ValidacaoProcedimento
        wf={wf({ situacao: undefined as never, ativo: true })}
        onAtualizado={() => {}}
      />,
    );
    expect(screen.getByText("Homologado")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Homologar/ })).not.toBeInTheDocument();
  });

  it("cartões mostram a situação de quem não está publicado", () => {
    renderApp(
      <WorkflowList
        items={[
          wf(),
          wf({ id: "wf-2", situacao: "EM_VALIDACAO" }),
          wf({ id: "wf-3", situacao: "HOMOLOGADO", ativo: false }),
        ]}
      />,
    );
    expect(screen.getByText("Rascunho")).toBeInTheDocument();
    expect(screen.getByText("Em validação")).toBeInTheDocument();
    expect(screen.getByText("Inativo")).toBeInTheDocument();
  });
});
