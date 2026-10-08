import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import AtualizarTTDDPage from "./page";

const prazos = (extra: Record<string, unknown> = {}) => ({
  descritor: "Processos de Dispensa de licitação",
  fase_corrente_anos: 1,
  fase_corrente_condicao: "",
  fase_interm_anos: 1,
  fase_interm_condicao: "",
  destinacao_final: "GUARDA_PERMANENTE",
  observacoes: "",
  ...extra,
});

const IMPACTO = {
  hash: "a".repeat(64),
  aplicada: false,
  orgaos: [
    {
      prefixo: "2.0",
      nome: "Secretaria de Administração",
      edicao_diario: "6.500",
      data_publicacao: "2027-05-10T00:00:00Z",
      versao: "III",
    },
  ],
  totais: { ALTERADA: 1, REVOGADA: 1, NOVA: 1, INALTERADA: 20 },
  series: [
    {
      codigo: "2.0.01.01.02",
      descritor: "Solicitação de Material",
      situacao: "REVOGADA",
      antes: prazos({ descritor: "Solicitação de Material" }),
      depois: null,
    },
    {
      codigo: "2.0.02.01.02",
      descritor: "Processos de Dispensa de licitação",
      situacao: "ALTERADA",
      antes: prazos(),
      depois: prazos({ fase_interm_anos: 4, observacoes: "Revisada" }),
    },
    {
      codigo: "2.0.09.00.01",
      descritor: "Série nova",
      situacao: "NOVA",
      antes: null,
      depois: prazos({ descritor: "Série nova", destinacao_final: null }),
    },
  ],
  procedimentos: [
    {
      id: "wf-2",
      codigo_processual: "ADM.DIR.002",
      versao: 1,
      ativo: true,
      codigo_ttdd: "2.0.02.01.02",
      situacao: "ALTERADA",
    },
  ],
};

const csv = () =>
  new File(["Código;Série documental\n"], "ttdd-revisada.csv", { type: "text/csv" });

describe("Atlas — atualizar a TTDD", () => {
  beforeEach(() => resetNavigation({}, "/atlas/ttdd/atualizar"));

  it("sem atlas:manage não mostra o envio", async () => {
    mockBackend(identityRoutes({ permissions: ["atlas:read"] }));
    renderApp(<AtualizarTTDDPage />);
    expect(await screen.findByText(/restrito à gestão do Atlas/)).toBeInTheDocument();
  });

  it("simula, mostra o impacto e só aplica depois de conferir (com o hash da simulação)", async () => {
    const backend = mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "POST v1/atlas/admin/ttdd/carga/simular": { data: IMPACTO },
      "POST v1/atlas/admin/ttdd/carga/aplicar": { data: { ...IMPACTO, aplicada: true } },
    });
    renderApp(<AtualizarTTDDPage />);

    const simular = await screen.findByRole("button", { name: /Simular/ });
    expect(simular).toBeDisabled();
    await userEvent.upload(screen.getByLabelText(/Arquivo da nova TTDD/), csv());
    await userEvent.click(simular);

    expect(await screen.findByText(/simulação — nada foi gravado/)).toBeInTheDocument();
    expect(backend.to("POST v1/atlas/admin/ttdd/carga/simular")[0]!.body).toEqual({
      formato: "csv",
      conteudo: "Código;Série documental\n",
    });
    const totais = screen.getByRole("list", { name: "Totais por situação" });
    expect(within(totais).getByText("20")).toBeInTheDocument();
    expect(
      screen.getByText(/2\.0 — TTDD da Secretaria de Administração, versão III/),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "ADM.DIR.002 v1" })).toHaveAttribute(
      "href",
      "/atlas/procedimentos/wf-2",
    );
    expect(
      screen.getByText(
        /Antes: 1 ano \+ 1 ano → guarda permanente · Depois: 1 ano \+ 4 anos → guarda permanente/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/Observações: “—” → “Revisada”/)).toBeInTheDocument();
    expect(screen.getByText(/Prazos atuais: 1 ano/)).toBeInTheDocument();
    expect(screen.getByText(/não definida na ttdd/)).toBeInTheDocument();

    const aplicar = screen.getByRole("button", { name: "Aplicar a nova TTDD" });
    expect(aplicar).toBeDisabled();
    await userEvent.click(screen.getByLabelText(/Conferi o impacto acima/));
    await userEvent.click(aplicar);
    expect(await screen.findByText(/TTDD atualizada/)).toBeInTheDocument();
    expect(backend.to("POST v1/atlas/admin/ttdd/carga/aplicar")[0]!.body).toEqual({
      formato: "csv",
      conteudo: "Código;Série documental\n",
      hash: "a".repeat(64),
    });
  });

  it("JSON do PDF; nada muda; erro de validação vem da API; trocar de arquivo limpa o resultado", async () => {
    const backend = mockBackend({
      ...identityRoutes({ permissions: ["atlas:manage"] }),
      "POST v1/atlas/admin/ttdd/carga/simular": (req) =>
        (req.body as { formato: string }).formato === "json"
          ? { data: { ...IMPACTO, totais: { INALTERADA: 21 }, series: [], procedimentos: [] } }
          : {
              status: 422,
              error: { code: "VALIDATION", message: "cabeçalho diferente do da exportação" },
            },
    });
    renderApp(<AtualizarTTDDPage />);
    const entrada = await screen.findByLabelText(/Arquivo da nova TTDD/);

    await userEvent.upload(entrada, new File(["{}"], "ttdd.json", { type: "application/json" }));
    await userEvent.click(screen.getByRole("button", { name: /Simular/ }));
    expect(await screen.findByText(/Nada muda/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Aplicar a nova TTDD" })).not.toBeInTheDocument();
    expect(backend.to("POST v1/atlas/admin/ttdd/carga/simular")[0]!.body).toMatchObject({
      formato: "json",
    });

    await userEvent.upload(entrada, csv());
    expect(screen.queryByText(/Nada muda/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Simular/ }));
    expect(await screen.findByText("cabeçalho diferente do da exportação")).toBeInTheDocument();
  });
});
