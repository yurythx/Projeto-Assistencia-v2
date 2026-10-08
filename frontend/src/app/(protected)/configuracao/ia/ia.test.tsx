import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import IAPage from "./page";

const PROVEDORES = [
  {
    provedor: "ollama",
    nome: "IA local (Ollama, no servidor)",
    endpoint: "http://ia-local:11434",
    modelo_sugerido: "qwen2.5:1.5b",
    externo: false,
    exige_chave: false,
  },
  {
    provedor: "openai",
    nome: "OpenAI",
    endpoint: "https://api.openai.com/v1",
    modelo_sugerido: "gpt-4o-mini",
    externo: true,
    exige_chave: true,
  },
  {
    provedor: "compativel",
    nome: "Outra compatível com a API da OpenAI",
    endpoint: "",
    modelo_sugerido: "",
    externo: false,
    exige_chave: false,
  },
];

const conexao = (id: string, extra: Record<string, unknown> = {}) => ({
  id,
  nome: `Conexão ${id}`,
  provedor: "ollama",
  endpoint: "http://ia-local:11434",
  modelo: "qwen2.5:1.5b",
  externo: false,
  timeout_segundos: 120,
  tem_chave: false,
  chave_final: "",
  ultimo_teste: null,
  updated_at: "",
  updated_by: "admin",
  ...extra,
});

const LOCAL = conexao("c1", {
  ultimo_teste: { ok: true, em: "2026-10-03T12:00:00Z", latencia_ms: 4200, erro: "" },
});
const NUVEM = conexao("c2", {
  nome: "OpenAI",
  provedor: "openai",
  endpoint: "https://api.openai.com/v1",
  modelo: "gpt-4o-mini",
  externo: true,
  tem_chave: true,
  chave_final: "a1b2",
  ultimo_teste: {
    ok: false,
    em: "2026-10-03T12:00:00Z",
    latencia_ms: 300,
    erro: "chave de API recusada (HTTP 401)",
  },
});

const USO = {
  funcao: "atlas.assistente",
  configurado: false,
  principal_id: null,
  reserva_id: null,
  mascarar_dados_pessoais: true,
  externo_autorizado_por: "",
  externo_autorizado_em: null,
  updated_at: null,
  updated_by: "",
  ambiente: conexao("amb", { nome: "Variáveis de ambiente", endpoint: "http://ia-local:11434" }),
};

const rotas = (extra: Record<string, unknown> = {}) => ({
  ...identityRoutes({ permissions: ["ia:manage"] }),
  "GET v1/ia/provedores": { data: PROVEDORES },
  "GET v1/ia/conexoes": { data: [LOCAL, NUVEM] },
  "GET v1/ia/uso/atlas.assistente": { data: USO },
  ...extra,
});

describe("Configurações > Inteligência artificial", () => {
  beforeEach(() => resetNavigation({}, "/configuracao/ia"));

  it("lista as conexões sem a chave, com o último teste, e testa de novo", async () => {
    mockBackend(
      rotas({
        "POST v1/ia/conexoes/c1/testar": {
          data: { ok: true, em: "", latencia_ms: 1500, erro: "" },
        },
        "POST v1/ia/conexoes/c2/testar": {
          data: {
            ok: false,
            em: "",
            latencia_ms: 1,
            erro: "limite de uso do fornecedor atingido (HTTP 429)",
          },
        },
      }),
    );
    renderApp(<IAPage />);

    const local = (await screen.findByText("Conexão c1")).closest("li")!;
    expect(within(local).getByText("Na rede")).toBeInTheDocument();
    expect(within(local).getByText(/Respondeu em 4\.2 s/)).toBeInTheDocument();
    const nuvem = screen.getByText("gpt-4o-mini").closest("li")!;
    expect(within(nuvem).getByText("Externo")).toBeInTheDocument();
    expect(within(nuvem).getByText(/••••a1b2/)).toBeInTheDocument();
    expect(within(nuvem).getByText(/Falhou: chave de API recusada/)).toBeInTheDocument();
    expect(
      screen.getByText(/Hoje: definido no servidor \(qwen2\.5:1\.5b em http:\/\/ia-local:11434\)/),
    ).toBeInTheDocument();

    await userEvent.click(within(local).getByRole("button", { name: "Testar Conexão c1" }));
    expect(await screen.findByText("em 1.5 s")).toBeInTheDocument();
    await userEvent.click(within(nuvem).getByRole("button", { name: "Testar OpenAI" }));
    expect(
      await screen.findByText("limite de uso do fornecedor atingido (HTTP 429)"),
    ).toBeInTheDocument();
  });

  it("nova conexão: o fornecedor pré-preenche o endereço; a chave vai só no envio", async () => {
    const backend = mockBackend(
      rotas({ "POST v1/ia/conexoes": { status: 201, data: conexao("c3") } }),
    );
    renderApp(<IAPage />);

    await userEvent.click(await screen.findByRole("button", { name: /Nova conexão/ }));
    const d = within(await screen.findByRole("dialog"));
    expect(d.getByLabelText("Endereço da API *")).toHaveValue("http://ia-local:11434");
    await userEvent.selectOptions(d.getByLabelText("Fornecedor *"), "openai");
    expect(d.getByLabelText("Endereço da API *")).toHaveValue("https://api.openai.com/v1");
    expect(d.getByText(/Fornecedor externo: as perguntas saem da rede/)).toBeInTheDocument();
    await userEvent.type(d.getByLabelText("Modelo *"), "gpt-4o-mini");
    await userEvent.type(d.getByLabelText("Chave de API *"), "sk-segredo");
    await userEvent.click(d.getByRole("button", { name: "Testar e salvar" }));

    const body = backend.to("POST v1/ia/conexoes")[0]!.body as Record<string, unknown>;
    expect(body).toMatchObject({
      nome: "IA local (Ollama, no servidor)",
      provedor: "openai",
      endpoint: "https://api.openai.com/v1",
      modelo: "gpt-4o-mini",
      chave: "sk-segredo",
      externo: true,
      remover_chave: false,
    });
  });

  it("editar: chave em branco mantém a salva; compatível escolhe se é externa; teste que falha mostra o motivo", async () => {
    const backend = mockBackend(
      rotas({
        "PUT v1/ia/conexoes/c2": {
          status: 422,
          error: {
            code: "VALIDATION",
            message: "o teste da conexão falhou, nada foi salvo: chave de API recusada (HTTP 401)",
          },
        },
        "PUT v1/ia/conexoes/c1": { data: LOCAL },
      }),
    );
    renderApp(<IAPage />);

    await userEvent.click(await screen.findByRole("button", { name: "Editar OpenAI" }));
    let d = within(await screen.findByRole("dialog"));
    expect(
      d.getByLabelText(/Chave de API \(salva: ••••a1b2; deixe em branco para manter\)/),
    ).toHaveValue("");
    await userEvent.click(d.getByRole("button", { name: "Testar e salvar" }));
    expect(await screen.findByText(/nada foi salvo: chave de API recusada/)).toBeInTheDocument();
    expect(backend.to("PUT v1/ia/conexoes/c2")[0]!.body).toMatchObject({
      chave: "",
      remover_chave: false,
    });
    await userEvent.keyboard("{Escape}");

    await userEvent.click(screen.getByRole("button", { name: "Editar Conexão c1" }));
    d = within(await screen.findByRole("dialog"));
    await userEvent.selectOptions(d.getByLabelText("Fornecedor *"), "compativel");
    await userEvent.type(d.getByLabelText("Endereço da API *"), "http://10.0.0.5:8000/v1");
    await userEvent.click(d.getByLabelText(/Serviço fora da rede/));
    await userEvent.click(d.getByRole("button", { name: "Testar e salvar" }));
    expect(backend.to("PUT v1/ia/conexoes/c1")[0]!.body).toMatchObject({
      provedor: "compativel",
      endpoint: "http://10.0.0.5:8000/v1",
      externo: true,
    });
  });

  it("remover a chave de uma conexão que não exige chave", async () => {
    const backend = mockBackend(
      rotas({
        "GET v1/ia/conexoes": { data: [conexao("c1", { tem_chave: true, chave_final: "" })] },
        "PUT v1/ia/conexoes/c1": { data: LOCAL },
      }),
    );
    renderApp(<IAPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Editar Conexão c1" }));
    const d = within(await screen.findByRole("dialog"));
    await userEvent.click(d.getByLabelText("Remover a chave salva"));
    expect(d.getByLabelText(/Chave de API/)).toBeDisabled();
    await userEvent.click(d.getByRole("button", { name: "Testar e salvar" }));
    expect(backend.to("PUT v1/ia/conexoes/c1")[0]!.body).toMatchObject({ remover_chave: true });
  });

  it("assistente: principal e reserva; fornecedor externo pede autorização explícita", async () => {
    const backend = mockBackend(
      rotas({
        "PUT v1/ia/uso/atlas.assistente": {
          data: { ...USO, configurado: true, principal_id: "c1", reserva_id: "c2" },
        },
      }),
    );
    renderApp(<IAPage />);

    await screen.findByText("Conexão c1");
    expect(screen.getByLabelText("Reserva (se a principal falhar)")).toBeDisabled();
    await userEvent.selectOptions(screen.getByLabelText("Conexão principal"), "c1");
    await userEvent.selectOptions(screen.getByLabelText("Reserva (se a principal falhar)"), "c2");
    expect(
      screen.getByText(/é um fornecedor externo: as perguntas dos usuários saem da rede/),
    ).toBeInTheDocument();
    await userEvent.click(
      screen.getByLabelText("Autorizo o envio das perguntas a este fornecedor"),
    );
    await userEvent.click(screen.getByRole("button", { name: "Salvar" }));

    expect(backend.to("PUT v1/ia/uso/atlas.assistente")[0]!.body).toEqual({
      principal_id: "c1",
      reserva_id: "c2",
      mascarar_dados_pessoais: true,
      autorizo_envio_externo: true,
    });
  });

  it("assistente configurado (autorização registrada) ou desligado; sem ambiente; lista vazia", async () => {
    mockBackend(
      rotas({
        "GET v1/ia/conexoes": { data: [NUVEM] },
        "GET v1/ia/uso/atlas.assistente": {
          data: {
            ...USO,
            configurado: true,
            principal_id: "c2",
            externo_autorizado_por: "admin",
            externo_autorizado_em: "2026-10-03T12:00:00Z",
            updated_at: "2026-10-03T12:00:00Z",
            updated_by: "admin",
          },
        },
      }),
    );
    const { unmount } = renderApp(<IAPage />);
    expect(await screen.findByText(/Configurado por admin em/)).toBeInTheDocument();
    expect(screen.getByText(/Autorizado por admin em/)).toBeInTheDocument();
    expect(screen.getByLabelText("Autorizo o envio das perguntas a este fornecedor")).toBeChecked();
    unmount();

    mockBackend(
      rotas({
        "GET v1/ia/conexoes": { data: [] },
        "GET v1/ia/uso/atlas.assistente": {
          data: { ...USO, configurado: true, updated_by: "", ambiente: null },
        },
      }),
    );
    const segundo = renderApp(<IAPage />);
    expect(
      await screen.findByText(/IA desligada: o assistente responde com a síntese/),
    ).toBeInTheDocument();
    expect(screen.getByText("Nenhuma conexão cadastrada")).toBeInTheDocument();
    segundo.unmount();

    mockBackend(rotas({ "GET v1/ia/uso/atlas.assistente": { data: { ...USO, ambiente: null } } }));
    renderApp(<IAPage />);
    expect(await screen.findByText(/Hoje: sem IA/)).toBeInTheDocument();
  });

  it("excluir conexão", async () => {
    const backend = mockBackend(rotas({ "DELETE v1/ia/conexoes/c1": { status: 204 } }));
    renderApp(<IAPage />);
    await userEvent.click(await screen.findByRole("button", { name: "Excluir Conexão c1" }));
    await userEvent.click(await screen.findByRole("button", { name: "Excluir" }));
    expect(backend.to("DELETE v1/ia/conexoes/c1")).toHaveLength(1);
  });
});
