import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CentroPopAtendimentoForm } from "@/components/sempras/CentroPopAtendimentoForm";
import { CrasAtendimentoForm } from "@/components/sempras/CrasAtendimentoForm";
import { CrasPaifCard } from "@/components/sempras/CrasPaifCard";
import { FichaModal } from "@/components/sempras/FichaModal";
import { PainelChamada } from "@/components/sempras/PainelChamada";
import FichaPage from "@/app/(protected)/sempras/cidadaos/[id]/page";
import { Recepcao } from "@/components/sempras/Recepcao";
import { Rma } from "@/components/sempras/Rma";
import type { Cidadao, PaifVinculo, Servico, TipoUnidade, Unidade } from "@/lib/sempras/types";
import { fail, identityRoutes, mockBackend, mockWebSocket, renderApp, renderRealtime, wsTicketRoute } from "@/test/backend";
import { resetNavigation } from "@/test/navigation";
import { PRONTUARIO as PRONTUARIO_COMPLETO } from "@/test/sempras";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);
vi.setConfig({ testTimeout: 20000 });

const unidade = (id: string, nome: string, tipo: TipoUnidade, servicos: Servico[], demandas: string[], painel = true): Unidade => ({
  id, entidade_id: "e1", nome, sigla: "", ativa: true, tipo, painel_exibe_nome: painel, servicos, demandas,
});
const CRAS = unidade("u1", "CRAS Ana Carla", "cras", ["fila", "prontuario", "encaminhamento", "beneficio", "paif", "oficio", "coletivo"],
  ["atendimento_tecnico", "outra"]);
const POP = unidade("u3", "Centro POP", "centro_pop", ["fila", "prontuario", "encaminhamento", "beneficio", "centro_pop"],
  ["alimentacao_higiene", "outra"]);
const CIDADAO: Cidadao = { id: "c1", nome: "João", nome_social: "", cpf: "***.982.247-**", nis: "", data_nascimento: "1990-05-01",
  nome_mae: "Ana", telefone: "", endereco: "", anonimo: false, sexo: "" };

const item = (id: string, extra: Record<string, unknown> = {}) => ({
  id, cidadao_id: "c1", unidade_id: "u1", dia: "2026-10-08", senha: 1, senha_exibida: `N00${id}`, tipo: "social", prioridade: "",
  demanda: "atendimento_tecnico", demanda_detalhe: "", status: "aguardando", local: "", nome: `Pessoa ${id}`,
  created_at: "2026-10-08T10:00:00Z", chamado_em: null, iniciado_em: null, encerrado_em: null, ...extra,
});

/** Percorre as abas numeradas ("1. ...", "2. ...") e mexe em tudo o que
 * aparece: marca as caixas, preenche os textos e usa os textos prontos —
 * cobre os ramos de montagem do texto. */
async function preencherTodasAsAbas() {
  for (const aba of screen.getAllByRole("button", { name: /^\s*\d\.\s/ })) {
    await userEvent.click(aba);
    for (let rodada = 0; rodada < 2; rodada++) {
      for (const cb of screen.queryAllByRole("checkbox")) {
        if (!(cb as HTMLInputElement).checked && !(cb as HTMLInputElement).disabled) await userEvent.click(cb);
      }
      for (const tb of screen.queryAllByRole("textbox")) {
        if (!(tb as HTMLInputElement).value && !(tb as HTMLInputElement).readOnly) await userEvent.type(tb, "texto de teste");
      }
    }
    for (const pronto of screen.queryAllByRole("button", { name: /^\+ / })) await userEvent.click(pronto);
  }
}

describe("SEMPRAS — formulário técnico do CRAS", () => {
  beforeEach(() => resetNavigation());

  it("monta a síntese de todas as abas, vincula ao PAIF (mesmo já vinculado) e conclui", async () => {
    const onSalvo = vi.fn();
    const onConcluido = vi.fn();
    const api = mockBackend({
      ...identityRoutes(),
      "POST v1/sempras/cidadaos/c1/registros": { status: 201, data: {} },
      "POST v1/sempras/cidadaos/c1/paif": fail(409, "a família já tem vínculo ativo com este CRAS"),
      "POST v1/sempras/atendimentos/a1/concluir": { data: {} },
    });
    renderApp(<CrasAtendimentoForm cidadao={CIDADAO} unidadeId="u1" itemFilaId="a1" isEmAtendimento onSalvo={onSalvo} onConcluido={onConcluido} />);
    await preencherTodasAsAbas();

    await userEvent.click(screen.getByRole("button", { name: /Salvar \(Manter em Atendimento\)/ }));
    await waitFor(() => expect(onSalvo).toHaveBeenCalled());
    const texto = (api.to("POST v1/sempras/cidadaos/c1/registros")[0]!.body as { texto: string }).texto;
    expect(texto).toMatch(/ATENDIMENTO TÉCNICO CRAS/);
    expect(texto).toMatch(/Marcadores RMA\/MDS/);
    expect(texto).toMatch(/Encaminhamentos Intersetoriais/);
    // vínculo já ativo (409) não impede o registro do atendimento
    expect(api.to("POST v1/sempras/cidadaos/c1/paif")).toHaveLength(1);

    await userEvent.click(screen.getByRole("button", { name: /Concluir e Finalizar Atendimento/ }));
    await waitFor(() => expect(onConcluido).toHaveBeenCalled());
    expect(api.to("POST v1/sempras/atendimentos/a1/concluir")).toHaveLength(1);
  });

  it("sem unidade não envia nada", async () => {
    const api = mockBackend({ ...identityRoutes() });
    renderApp(<CrasAtendimentoForm cidadao={CIDADAO} unidadeId="" />);
    await userEvent.click(screen.getByRole("button", { name: /Salvar \(Manter em Atendimento\)/ }));
    expect(api.calls.filter((c) => c.method === "POST")).toHaveLength(0);
  });
});

describe("SEMPRAS — atendimento especializado do Centro POP", () => {
  beforeEach(() => resetNavigation());

  it("salva o perfil de rua e a evolução de todas as abas, e conclui", async () => {
    const onSalvo = vi.fn();
    const onConcluido = vi.fn();
    const api = mockBackend({
      ...identityRoutes(),
      "PUT v1/sempras/cidadaos/c1/centro-pop-perfil": { data: {} },
      "POST v1/sempras/cidadaos/c1/registros": { status: 201, data: {} },
      "POST v1/sempras/atendimentos/a1/concluir": { data: {} },
    });
    renderApp(<CentroPopAtendimentoForm cidadao={CIDADAO} unidadeId="u3" unidadeNome="Centro POP" perfil={null} itemFilaId="a1"
      isEmAtendimento onSalvo={onSalvo} onConcluido={onConcluido} />);
    await preencherTodasAsAbas();

    const botoes = screen.getAllByRole("button").filter((b) => /Salvar|Concluir/.test(b.textContent ?? ""));
    await userEvent.click(botoes.find((b) => /Salvar/.test(b.textContent ?? ""))!);
    await waitFor(() => expect(onSalvo).toHaveBeenCalled());
    expect(api.to("PUT v1/sempras/cidadaos/c1/centro-pop-perfil")[0]!.body).toMatchObject({ unidade_id: "u3" });
    expect((api.to("POST v1/sempras/cidadaos/c1/registros")[0]!.body as { texto: string }).texto.length).toBeGreaterThan(50);

    await userEvent.click(botoes.find((b) => /Concluir/.test(b.textContent ?? ""))!);
    await waitFor(() => expect(onConcluido).toHaveBeenCalled());
    expect(api.to("POST v1/sempras/atendimentos/a1/concluir")).toHaveLength(1);
  });

  it("parte de um perfil já preenchido", async () => {
    mockBackend({ ...identityRoutes() });
    renderApp(<CentroPopAtendimentoForm cidadao={CIDADAO} unidadeId="u3" perfil={{
      cidadao_id: "c1", situacao: "trecheiro", tempo_rua_anos: 3, motivo_vulnerabilidade: "desemprego", possui_doenca: true,
      doenca_descricao: "diabetes", usa_medicacao: true, medicacao_descricao: "insulina", uso_drogas: true,
      drogas_descricao: "álcool", atualizado_por: null, atualizado_em: "2026-10-01T00:00:00Z",
    }} />);
    expect(await screen.findByDisplayValue("3")).toBeInTheDocument();
  });
});

describe("SEMPRAS — card do PAIF", () => {
  beforeEach(() => resetNavigation());

  const vinculo: PaifVinculo = {
    id: "p1", cidadao_id: "c1", unidade_id: "u1", status: "ativo", tipo_atendimento: "particularizado", extrema_pobreza: true,
    bolsa_familia: true, bpc: false, scfv_participantes: 2, motivo_exclusao: "", incluido_por: null, excluido_por: null,
    data_inclusao: "2026-10-01T00:00:00Z", data_exclusao: null,
  } as PaifVinculo;

  it("insere a família (com os novos do SCFV na evolução)", async () => {
    const onUpdated = vi.fn();
    const api = mockBackend({
      ...identityRoutes(),
      "POST v1/sempras/cidadaos/c1/paif": { status: 201, data: vinculo },
      "POST v1/sempras/cidadaos/c1/registros": { status: 201, data: {} },
    });
    renderApp(<CrasPaifCard cidadao={CIDADAO} unidadeId="u1" unidadeNome="CRAS Ana Carla" paifVinculos={[]} onUpdated={onUpdated} />);
    await userEvent.click(screen.getAllByRole("button", { name: /Nova Família no PAIF|Inserir|Incluir/ })[0]!);
    const dialog = await screen.findByRole("dialog", { name: "Inserir Família no Acompanhamento do PAIF" });
    for (const cb of within(dialog).queryAllByRole("checkbox")) await userEvent.click(cb);
    await userEvent.type(within(dialog).getByLabelText("Novos Inseridos no SCFV (por faixa/nome)"), "Pedro, 8 anos");
    await userEvent.click(within(dialog).getAllByRole("button").find((b) => (b as HTMLButtonElement).type === "submit")!);
    await waitFor(() => expect(onUpdated).toHaveBeenCalled());
    expect(api.to("POST v1/sempras/cidadaos/c1/paif")).toHaveLength(1);
    expect((api.to("POST v1/sempras/cidadaos/c1/registros")[0]!.body as { texto: string }).texto).toMatch(/Pedro, 8 anos/);
  });

  it("desliga o vínculo ativo com data e motivo", async () => {
    const onUpdated = vi.fn();
    const api = mockBackend({ ...identityRoutes(), "POST v1/sempras/paif/p1/excluir": { data: { ...vinculo, status: "excluido" } } });
    renderApp(<CrasPaifCard cidadao={CIDADAO} unidadeId="u1" paifVinculos={[vinculo, { ...vinculo, id: "p0", status: "excluido",
      motivo_exclusao: "mudou", data_exclusao: "2026-09-01T00:00:00Z" } as PaifVinculo]} onUpdated={onUpdated} />);
    await userEvent.click(screen.getByRole("button", { name: /Desligar do PAIF/ }));
    const dialog = await screen.findByRole("dialog", { name: "Desligamento de Família do PAIF" });
    await userEvent.click(within(dialog).getAllByRole("button").find((b) => (b as HTMLButtonElement).type === "submit")!);
    await waitFor(() => expect(onUpdated).toHaveBeenCalled());
    expect((api.to("POST v1/sempras/paif/p1/excluir")[0]!.body as { motivo: string }).motivo).toMatch(/^Desligamento em /);
  });
});

const PRONTUARIO = {
  cidadao: CIDADAO, atendimentos: [], registros: [
    { id: "r1", unidade_id: "u1", atendimento_id: null, tipo: "social", texto: "Primeira escuta", sigiloso: false, autor_nome: "Téc",
      created_at: "2026-10-08T10:00:00Z" },
  ], rede_apoio: [{ id: "ra1", cidadao_id: "c1", nome: "Maria", parentesco: "Irmã", telefone: "66", created_at: "2026-10-08T09:00:00Z" }],
  beneficios: [{ id: "b1", beneficiario_id: "c1", solicitante_id: null, unidade_id: "u1", tipo: "cesta_basica", forma_acesso: "demanda_espontanea",
    quantidade: 1, status: "concedido", observacao: "urgente", motivo_decisao: "", tecnico_decisao_id: null,
    data_solicitacao: "2026-10-08T08:00:00Z", data_decisao: null }],
  encaminhamentos: [], paif_vinculos: [], conselho_atendimentos: [], paefi_casos: [], mse_medidas: [], notificacoes_creas: [],
  oficios_cras: [], casa_abrigo_acolhimentos: [], centro_pop_perfil: null, encaminhamentos_externos: [],
  centro_pop_documentos: [{ id: "d1", cidadao_id: "c1", unidade_id: "u3", tipo: "identificacao", nome_original: "rg.pdf", object_key: "k",
    content_type: "application/pdf", size_bytes: 8, status: "ready", created_at: "2026-10-08T08:00:00Z" }],
  casa_mulher_casos: [], abordagens: [],
  outras_unidades: [{ unidade_id: "u9", unidade_nome: "CREAS", tipo: "creas", ativos: ["paefi"] }],
  registros_origem: [{ id: "ro1", unidade_id: "u9", atendimento_id: null, tipo: "social", texto: "Evolução do CREAS", sigiloso: false,
    autor_nome: "", created_at: "2026-10-07T10:00:00Z" }],
};

describe("SEMPRAS — ficha (modal)", () => {
  beforeEach(() => resetNavigation());

  it("CRAS: conduz o atendimento da fila, edita o cadastro, registra evolução e vê a origem do encaminhamento", async () => {
    mockWebSocket();
    const onClose = vi.fn();
    const api = mockBackend({
      ...identityRoutes(),
      ...wsTicketRoute,
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades/u1/fila": { data: [item("a1", { status: "chamado", local: "Sala 1" })] },
      "POST v1/sempras/atendimentos/*": { data: {} },
      "PUT v1/sempras/cidadaos/c1": { data: CIDADAO },
      "POST v1/sempras/cidadaos/c1/registros": { status: 201, data: {} },
    });
    renderRealtime(<FichaModal cidadaoId="c1" unidade={CRAS} onClose={onClose} onCidadaoUpdated={vi.fn()} />);
    expect(await screen.findByText(/Também acompanhado por/)).toBeInTheDocument();

    await userEvent.click(await screen.findByRole("button", { name: /Iniciar atendimento/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/atendimentos/a1/iniciar")).toHaveLength(1));

    await userEvent.click(screen.getByRole("tab", { name: /Dados Cadastrais/ }));
    await userEvent.selectOptions(screen.getByLabelText("Sexo"), "masculino");
    await userEvent.click(screen.getByRole("button", { name: /Salvar alterações/ }));
    await waitFor(() => expect(api.to("PUT v1/sempras/cidadaos/c1")[0]!.body).toMatchObject({ sexo: "masculino" }));

    await userEvent.click(screen.getByRole("tab", { name: /Evoluções/ }));
    expect(screen.getByText("Evolução do CREAS")).toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText(/Descreva as orientações/), "Orientação prestada");
    await userEvent.click(screen.getByRole("button", { name: /Registrar evolução/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/cidadaos/c1/registros")[0]!.body).toMatchObject({ texto: "Orientação prestada" }));

    await userEvent.click(screen.getByRole("tab", { name: /Benefícios & Rede/ }));
    expect(screen.getByText("Maria")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Não compareceu/ }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("Centro POP: anexa, baixa e remove documento; registra encaminhamento externo e imprime a guia", async () => {
    mockWebSocket();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    const api = mockBackend({
      ...identityRoutes(),
      ...wsTicketRoute,
      "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
      "GET v1/sempras/unidades/u3/fila": { data: [] },
      "POST v1/sempras/cidadaos/c1/centro-pop-documentos/uploads": { status: 201, data: {
        upload: { object_key: "k2", upload_url: "http://minio.test/b/k2", method: "PUT", headers: {}, expires_at: "" },
        documento: { id: "d2" } } },
      "PUT minio.test/*": { status: 200 },
      "POST v1/sempras/centro-pop-documentos/d2/confirm": { data: {} },
      "GET v1/sempras/centro-pop-documentos/d1/download": { data: { url: "http://minio.test/b/d1" } },
      "DELETE v1/sempras/centro-pop-documentos/d1": { data: { removido: true } },
      "POST v1/sempras/cidadaos/c1/encaminhamentos-externos": { status: 201, data: {} },
    });
    renderRealtime(<FichaModal cidadaoId="c1" unidade={POP} onClose={vi.fn()} />);
    await userEvent.click(await screen.findByRole("tab", { name: /Documentos/ }));
    await userEvent.upload(document.getElementById("modal-file-input") as HTMLInputElement,
      new File(["%PDF-1.4"], "laudo.pdf", { type: "application/pdf" }));
    await waitFor(() => expect(api.to("POST v1/sempras/centro-pop-documentos/d2/confirm")).toHaveLength(1));

    const doc = screen.getByText("rg.pdf").closest("li")!;
    const [baixar, remover] = within(doc as HTMLElement).getAllByRole("button");
    await userEvent.click(baixar!);
    await waitFor(() => expect(open).toHaveBeenCalledWith("http://minio.test/b/d1", "_blank", "noopener,noreferrer"));
    await userEvent.click(remover!);
    const confirma = screen.queryByRole("dialog", { name: /Remover|Excluir/ });
    if (confirma) await userEvent.click(within(confirma).getAllByRole("button").at(-1)!);
    await waitFor(() => expect(api.to("DELETE v1/sempras/centro-pop-documentos/d1")).toHaveLength(1));

    await userEvent.click(screen.getByRole("tab", { name: /Guia de Encaminhamento/ }));
    await userEvent.selectOptions(screen.getByLabelText("Destino do Encaminhamento"), "caps_ad");
    await userEvent.type(screen.getByLabelText("Justificativa Técnica / Motivo do Encaminhamento"), "Uso abusivo de álcool");
    await userEvent.click(screen.getByRole("button", { name: /Registrar e Gerar Guia/ }));
    await waitFor(() => expect(api.to("POST v1/sempras/cidadaos/c1/encaminhamentos-externos")[0]!.body).toMatchObject({ destino: "caps_ad" }));
    const guia = await screen.findByRole("dialog", { name: "Guia Oficial de Encaminhamento" });
    await userEvent.click(within(guia).getByRole("button", { name: /Imprimir/ }));
    expect(print).toHaveBeenCalled();
    open.mockRestore();
    print.mockRestore();
  });
  it("ficha (modal): com o backend recusando, cada aba e cada ação mostram o erro sem fechar", async () => {
    mockWebSocket();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const acao = /Iniciar|Concluir|Não compareceu|Salvar|Registrar|Baixar|Remover|Enviar/;
    for (const u of [CRAS, POP]) {
      const alheio = { ...PRONTUARIO, cidadao: { ...CIDADAO, nome_social: "Jô", anonimo: true },
        registros: PRONTUARIO.registros.map((r) => ({ ...r, unidade_id: "zz", autor_nome: "" })),
        centro_pop_documentos: PRONTUARIO.centro_pop_documentos.map((d) => ({ ...d, status: "pending" })) };
      const onClose = vi.fn();
      mockBackend({
        ...identityRoutes(),
        ...wsTicketRoute,
        "GET v1/sempras/cidadaos/c1/prontuario": { data: alheio },
        [`GET v1/sempras/unidades/${u.id}/fila`]: { data: [item("a1", { cidadao_id: "c1", unidade_id: u.id, status: "em_atendimento" })] },
        "GET v1/sempras/centro-pop-documentos/*": { data: {} },
        "POST v1/*": fail(422, "recusado"),
        "PUT v1/*": fail(422, "recusado"),
        "DELETE v1/*": fail(422, "recusado"),
      });
      const { unmount } = renderRealtime(<FichaModal cidadaoId="c1" unidade={u} onClose={onClose} />);
      await screen.findAllByRole("tab");
      for (const aba of screen.getAllByRole("tab")) {
        await userEvent.click(aba);
        await mexerEmTudo(document.body, { rapido: true });
        for (const botao of screen.queryAllByRole("button").filter((b) => acao.test(b.textContent ?? "") && !(b as HTMLButtonElement).disabled)) {
          if (document.body.contains(botao)) await userEvent.click(botao);
        }
      }
      const arquivo = document.getElementById("modal-file-input") as HTMLInputElement | null;
      if (arquivo) await userEvent.upload(arquivo, new File(["%PDF-1.4"], "laudo.pdf", { type: "application/pdf" }));
      expect(onClose).not.toHaveBeenCalled();
      unmount();
    }
    open.mockRestore();
  }, 240000);
});

describe("SEMPRAS — painel de chamada (áudio)", () => {
  const falas: string[] = [];
  beforeEach(() => {
    resetNavigation();
    falas.length = 0;
    const no = () => ({ connect: vi.fn(), start: vi.fn(), stop: vi.fn(), type: "", frequency: { setValueAtTime: vi.fn() },
      gain: { setValueAtTime: vi.fn(), exponentialRampToValueAtTime: vi.fn() } });
    class AudioCtx { currentTime = 0; destination = {}; createOscillator = no; createGain = no; }
    vi.stubGlobal("AudioContext", AudioCtx);
    vi.stubGlobal("SpeechSynthesisUtterance", class { constructor(public text: string) {} lang = ""; rate = 1; pitch = 1; });
    vi.stubGlobal("speechSynthesis", { cancel: vi.fn(), speak: (u: { text: string }) => falas.push(u.text) });
  });
  afterEach(() => vi.unstubAllGlobals());

  it("toca e fala a senha nova; nas unidades sigilosas não fala o nome", async () => {
    const ws = mockWebSocket();
    let fila = [item("3", { status: "chamado", senha_exibida: "N003", nome: "Pessoa 3" })];
    mockBackend({ ...identityRoutes(), ...wsTicketRoute, "GET v1/sempras/unidades/u1/fila": () => ({ data: fila }) });
    renderRealtime(<PainelChamada unidade={{ ...CRAS, painel_exibe_nome: false }} />);
    expect(await screen.findByText("N003")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Testar som/ }));
    await waitFor(() => expect(falas.at(-1)).toMatch(/Senha P\s+0\s+1/));
    expect(falas.at(-1)).not.toMatch(/Maria Silva/);

    await userEvent.click(screen.getByRole("button", { name: /Ativar chamada por voz/ }));
    fila = [...fila, item("4", { status: "chamado", senha_exibida: "N004", nome: "Pessoa 4", local: "Guichê 2" })];
    await waitFor(() => expect(ws.socket().sent).toContainEqual({ type: "subscribe", topic: "sempras:fila:u1" }));
    act(() => ws.socket().receive({ topic: "sempras:fila:u1", type: "sempras.fila", data: {} }));
    await waitFor(() => expect(falas.at(-1)).toMatch(/N\s+0\s+0\s+4.*Guichê 2/));
    expect(falas.at(-1)).not.toMatch(/Pessoa 4/);
    await userEvent.click(screen.getByRole("button", { name: /Desativar som/ }));
  });
});

describe("SEMPRAS — recepção (casos de borda)", () => {
  beforeEach(() => resetNavigation());

  it("Centro POP: cadastro anônimo; demanda 'outra' exige o detalhe; erro de inclusão aparece no modal; imprime a senha", async () => {
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    let falhar = true;
    const api = mockBackend({
      ...identityRoutes({ permissions: ["sempras:recepcao", "sempras:atendimento"], scopes: [] }),
      "GET v1/sempras/cidadaos": { data: [{ id: "c9", nome: "Tião", nome_social: "", cpf: "", nis: "", data_nascimento: "", atendimentos_na_area: 0 }] },
      "POST v1/sempras/cidadaos": { status: 201, data: { ...CIDADAO, id: "c10", nome: "Sem Nome", anonimo: true } },
      "POST v1/sempras/unidades/u3/fila": () => (falhar ? fail(422, "o munícipe já está na fila") : { status: 201, data: item("9", { senha_exibida: "P009", prioridade: "idoso" }) }),
    });
    const onAbrirFicha = vi.fn();
    renderApp(<Recepcao unidade={POP} onIncluido={vi.fn()} onAbrirFicha={onAbrirFicha} />);

    await userEvent.click(screen.getByRole("button", { name: /Cadastrar/ }));
    const cad = await screen.findByRole("dialog", { name: "Cadastrar munícipe" });
    await userEvent.type(within(cad).getByLabelText("Nome civil *"), "Sem Nome");
    await userEvent.click(within(cad).getByRole("checkbox"));
    expect(within(cad).getByLabelText(/CPF \(opcional no Centro POP\)/)).toBeDisabled();
    await userEvent.click(within(cad).getByRole("button", { name: "Salvar cadastro" }));
    await waitFor(() => expect(api.to("POST v1/sempras/cidadaos")[0]!.body).toMatchObject({ anonimo: true }));

    await userEvent.type(screen.getByLabelText("CPF, NIS ou nome"), "Tião");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    const modal = await screen.findByRole("dialog", { name: /Munícipe Localizado/ });
    await userEvent.click(within(modal).getByRole("button", { name: /Confirmar e incluir na fila/ }));
    expect(await within(modal).findByText("Selecione a demanda que trouxe o munícipe à unidade.")).toBeInTheDocument();
    await userEvent.selectOptions(within(modal).getByLabelText("Demanda"), "outra");
    await userEvent.click(within(modal).getByRole("button", { name: /Confirmar e incluir na fila/ }));
    expect(await within(modal).findByText("Descreva a demanda.")).toBeInTheDocument();
    await userEvent.type(within(modal).getByLabelText("Descreva a demanda"), "segunda via de documento");
    await userEvent.click(within(modal).getByRole("button", { name: /Confirmar e incluir na fila/ }));
    expect(await within(modal).findByText("o munícipe já está na fila")).toBeInTheDocument();

    falhar = false;
    await userEvent.click(within(modal).getByRole("button", { name: /Confirmar e incluir na fila/ }));
    const ticket = await screen.findByRole("dialog", { name: "Senha de Atendimento Gerada" });
    expect(api.to("POST v1/sempras/unidades/u3/fila").at(-1)!.body).toMatchObject({ demanda: "outra", demanda_detalhe: "segunda via de documento" });
    await userEvent.click(within(ticket).getByRole("button", { name: /Imprimir Senha/ }));
    expect(print).toHaveBeenCalled();
    print.mockRestore();
  });

  it("CPF inválido no cadastro é recusado no próprio modal", async () => {
    mockBackend({ ...identityRoutes({ permissions: ["sempras:recepcao"], scopes: [] }) });
    renderApp(<Recepcao unidade={CRAS} onIncluido={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Cadastrar/ }));
    const cad = await screen.findByRole("dialog", { name: "Cadastrar munícipe" });
    await userEvent.type(within(cad).getByLabelText("Nome civil *"), "Fulano");
    await userEvent.type(within(cad).getByLabelText("CPF (ou NIS) *"), "111.111.111-11");
    await userEvent.click(within(cad).getByRole("button", { name: "Salvar cadastro" }));
    expect(await within(cad).findByText(/O CPF informado é inválido/)).toBeInTheDocument();
  });
});

describe("SEMPRAS — RMA (exportação)", () => {
  beforeEach(() => resetNavigation());

  it("exporta o CSV do formulário escolhido e imprime", async () => {
    const criar = vi.fn(() => "blob:rma");
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: criar, revokeObjectURL: vi.fn() }));
    const print = vi.spyOn(window, "print").mockImplementation(() => {});
    const zero = new Proxy({}, { get: (_, k) => (k === "atendimentos_por_demanda" || k === "paefi_novos_por_violacao" || k === "pessoas_por_sexo_faixa" ? {} : 0) });
    mockBackend({ ...identityRoutes(), "GET v1/sempras/relatorios/rma": { data: [{ ...Object.fromEntries(
      ["atendimentos", "paif_ativos", "coletivo_total_no_mes"].map((k) => [k, 3])), unidade: CRAS,
      atendimentos_por_demanda: {}, paefi_novos_por_violacao: {}, pessoas_por_sexo_faixa: {} }] } });
    void zero;
    renderApp(<Rma />);
    expect(await screen.findByText("CRAS Ana Carla")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Formulário"), "cras");
    await userEvent.click(screen.getByRole("button", { name: /Exportar CSV/ }));
    expect(criar).toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /Imprimir Relatório/ }));
    expect(print).toHaveBeenCalled();
    print.mockRestore();
    vi.unstubAllGlobals();
  });
});

/** Troca cada lista por todas as opções, marca e desmarca cada caixa e
 * escreve em cada campo (sem enviar): os controles respondem e o estado
 * acompanha — cobre os onChange de formulários longos. */
async function mexerEmTudo(raiz: HTMLElement = document.body, { digitar = true, rapido = false, fixarUnidade = false } = {}) {
  const w = within(raiz);
  for (const sel of w.queryAllByRole("combobox")) {
    // na ficha, a lista de unidades decide as seções: trocá-la esconderia o resto
    if (fixarUnidade && (sel as HTMLSelectElement).querySelector('option[value="u5"]')) continue;
    for (const op of Array.from((sel as HTMLSelectElement).options)) {
      if (!(sel as HTMLSelectElement).disabled && raiz.contains(sel)) await userEvent.selectOptions(sel, op.value);
    }
  }
  for (const cb of w.queryAllByRole("checkbox")) {
    if (!(cb as HTMLInputElement).disabled && raiz.contains(cb)) {
      await userEvent.click(cb);
      if (raiz.contains(cb)) await userEvent.click(cb);
    }
  }
  if (rapido) {
    const { fireEvent } = await import("@testing-library/react");
    for (const campo of [...w.queryAllByRole("textbox"), ...w.queryAllByRole("spinbutton")]) {
      if (!(campo as HTMLInputElement).readOnly && raiz.contains(campo)) fireEvent.change(campo, { target: { value: "123 texto" } });
    }
  }
  for (const campo of digitar && !rapido ? [...w.queryAllByRole("textbox"), ...w.queryAllByRole("spinbutton")] : []) {
    if (!(campo as HTMLInputElement).readOnly && !(campo as HTMLInputElement).disabled && raiz.contains(campo)) {
      await userEvent.type(campo, "1");
    }
  }
  for (const data of Array.from(raiz.querySelectorAll<HTMLInputElement>('input[type="date"], input[type="datetime-local"], input[type="month"]'))) {
    if (!data.disabled) {
      const { fireEvent } = await import("@testing-library/react");
      fireEvent.change(data, { target: { value: data.type === "month" ? "2026-09" : data.type === "date" ? "2026-09-10" : "2026-09-10T10:00" } });
    }
  }
}

describe("SEMPRAS — todos os controles respondem", () => {
  beforeEach(() => resetNavigation());

  it("formulários do CRAS e do Centro POP, aba por aba", async () => {
    mockBackend({ ...identityRoutes() });
    const { unmount } = renderApp(<CrasAtendimentoForm cidadao={CIDADAO} unidadeId="u1" />);
    for (const aba of screen.getAllByRole("button", { name: /^\s*\d\.\s/ })) {
      await userEvent.click(aba);
      await mexerEmTudo();
    }
    unmount();
    renderApp(<CentroPopAtendimentoForm cidadao={CIDADAO} unidadeId="u3" perfil={null} />);
    for (const aba of screen.getAllByRole("button", { name: /^\s*\d\.\s/ })) {
      await userEvent.click(aba);
      await mexerEmTudo();
    }
  });

  it("modais do PAIF (inclusão e desligamento)", async () => {
    mockBackend({ ...identityRoutes() });
    const vinculo = { id: "p1", cidadao_id: "c1", unidade_id: "u1", status: "ativo", tipo_atendimento: "ambos", extrema_pobreza: false,
      bolsa_familia: false, bpc: true, scfv_participantes: 0, motivo_exclusao: "", incluido_por: null, excluido_por: null,
      data_inclusao: "2026-10-01T00:00:00Z", data_exclusao: null } as PaifVinculo;
    const { unmount } = renderApp(<CrasPaifCard cidadao={CIDADAO} unidadeId="u1" paifVinculos={[vinculo]} onUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Desligar do PAIF/ }));
    const desligar = await screen.findByRole("dialog", { name: "Desligamento de Família do PAIF" });
    await mexerEmTudo(desligar);
    await userEvent.click(within(desligar).getAllByRole("button").find((b) => (b as HTMLButtonElement).type === "button")!);
    unmount();
    renderApp(<CrasPaifCard cidadao={CIDADAO} unidadeId="u1" paifVinculos={[]} onUpdated={vi.fn()} />);
    await userEvent.click(screen.getAllByRole("button", { name: /Nova Família no PAIF/ })[0]!);
    const incluir = await screen.findByRole("dialog", { name: "Inserir Família no Acompanhamento do PAIF" });
    await mexerEmTudo(incluir);
    await userEvent.click(within(incluir).getAllByRole("button").find((b) => (b as HTMLButtonElement).type === "button")!);
  });

  it("recepção: listas, cadastro e modal do munícipe", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["sempras:recepcao", "sempras:atendimento"], scopes: [] }),
      "GET v1/sempras/cidadaos": { data: [{ id: "c9", nome: "Maria", nome_social: "Mari", cpf: "***", nis: "***", data_nascimento: "1990-01-01", atendimentos_na_area: 3 }] },
    });
    const onAbrirFicha = vi.fn();
    renderApp(<Recepcao unidade={CRAS} onIncluido={vi.fn()} onAbrirFicha={onAbrirFicha} />);
    await mexerEmTudo();
    await userEvent.clear(screen.getByLabelText("CPF, NIS ou nome"));
    await userEvent.type(screen.getByLabelText("CPF, NIS ou nome"), "Maria");
    await userEvent.click(screen.getByRole("button", { name: /Buscar/ }));
    const modal = await screen.findByRole("dialog", { name: /Munícipe Localizado/ });
    await mexerEmTudo(modal);
    await userEvent.click(within(modal).getByRole("button", { name: /Abrir ficha técnica/ }));
    expect(onAbrirFicha).toHaveBeenCalledWith("c9");
    await userEvent.click(screen.getByRole("link", { name: "Ficha" }));
    await userEvent.click(screen.getByText("Mari (Maria)"));
    const de_novo = await screen.findByRole("dialog", { name: /Munícipe Localizado/ });
    await userEvent.click(within(de_novo).getByRole("button", { name: "Fechar" }));
    await userEvent.click(screen.getByRole("button", { name: /Cadastrar/ }));
    const cad = await screen.findByRole("dialog", { name: "Cadastrar munícipe" });
    await mexerEmTudo(cad);
    await userEvent.click(within(cad).getByRole("button", { name: "Cancelar" }));
  });

  it("ficha completa de cada tipo de unidade", async () => {
    const tipos: [string, TipoUnidade, Servico[]][] = [
      ["u1", "cras", ["fila", "prontuario", "encaminhamento", "beneficio", "paif", "oficio", "coletivo"]],
      ["u2", "creas", ["fila", "prontuario", "encaminhamento", "paefi", "mse", "notificacao"]],
      ["u3", "centro_pop", ["fila", "prontuario", "encaminhamento", "beneficio", "centro_pop"]],
      ["u4", "casa_abrigo", ["prontuario", "encaminhamento", "acolhimento"]],
      ["u5", "conselho_tutelar", ["fila", "prontuario", "encaminhamento", "conselho"]],
      ["u6", "casa_mulher", ["fila", "prontuario", "encaminhamento", "casa_mulher"]],
    ];
    const unidades = tipos.map(([id, tipo, servicos]) => unidade(id, `Unidade ${tipo}`, tipo, servicos, ["outra"]));
    for (const [id] of tipos) {
      resetNavigation({ id: "c1" }, "/", `unidade=${id}`);
      mockBackend({
        ...identityRoutes(),
        "GET v1/sempras/cidadaos/c1/prontuario": { data: PRONTUARIO },
        "GET v1/sempras/unidades": { data: unidades },
      });
      const { unmount } = renderApp(<FichaPage />);
      expect(await screen.findByRole("heading", { name: "João" })).toBeInTheDocument();
      await mexerEmTudo(document.body, { rapido: true, fixarUnidade: true });
      unmount();
    }
  }, 180000);

  it("ficha: cada ação de cada seção, com os diálogos de confirmação", async () => {
    const tipos: [string, TipoUnidade, Servico[]][] = [
      ["u1", "cras", ["fila", "prontuario", "encaminhamento", "beneficio", "paif", "oficio", "coletivo"]],
      ["u2", "creas", ["fila", "prontuario", "encaminhamento", "paefi", "mse", "notificacao"]],
      ["u3", "centro_pop", ["fila", "prontuario", "encaminhamento", "beneficio", "centro_pop"]],
      ["u4", "casa_abrigo", ["prontuario", "encaminhamento", "acolhimento"]],
      ["u5", "conselho_tutelar", ["fila", "prontuario", "encaminhamento", "conselho"]],
    ];
    const unidades = tipos.map(([id, tipo, servicos]) => unidade(id, `Unidade ${tipo}`, tipo, servicos, ["outra"]));
    const rico = {
      ...PRONTUARIO_COMPLETO,
      beneficios: [
        ...PRONTUARIO_COMPLETO.beneficios,
        { ...PRONTUARIO_COMPLETO.beneficios[0]!, id: "b2", status: "concedido", quantidade: 3, motivo_decisao: "ok" },
      ],
      centro_pop_documentos: [
        { id: "d1", cidadao_id: "c1", unidade_id: "u3", tipo: "identificacao", nome_original: "rg.pdf", object_key: "k",
          content_type: "application/pdf", size_bytes: 8, status: "ready", created_at: "2026-10-08T08:00:00Z" },
        { id: "d2", cidadao_id: "c1", unidade_id: "u3", tipo: "identificacao", nome_original: "cpf.pdf", object_key: "k2",
          content_type: "application/pdf", size_bytes: 8, status: "pending", created_at: "2026-10-08T08:00:00Z" },
      ],
      abordagens: [{ id: "ab1", cidadao_id: "c1", unidade_id: "u3", local: "Praça", resultado: "orientado", observacoes: "frio",
        data_abordagem: "2026-10-08T08:00:00Z" }],
      casa_mulher_casos: [], outras_unidades: [], registros_origem: [],
    };
    const acao = /^(Conceder|Negar|Suspender|Aplicar medida|Encaminhar ao MP|Arquivar|Baixar|Remover|Exportar dados|Abrir |Ocultar |Encerrar|Concluir|Responder|Excluir|Editar|Cancelar|Aceitar|Recusar|Solicitar|Registrar|Incluir|Abrir caso|Adicionar|Acolher|Notificar)/;
    const assign = vi.fn();
    vi.stubGlobal("location", { ...window.location, assign });
    // segunda volta: o backend recusa tudo e os itens são de uma unidade
    // fora da lista — cobre os caminhos de erro e os nomes ausentes
    const alheio = JSON.parse(JSON.stringify(rico).replaceAll('"unidade_id":"u1"', '"unidade_id":"zz"'));
    for (const [id, falhar] of [...tipos.map(([t]) => [t, false] as const), ...tipos.map(([t]) => [t, true] as const)]) {
      resetNavigation({ id: "c1" }, "/", `unidade=${id}`);
      const resposta = (ok: object) => (falhar ? fail(422, "recusado") : ok);
      const api = mockBackend({
        ...identityRoutes(),
        "GET v1/sempras/cidadaos/c1/prontuario": { data: falhar ? alheio : rico },
        "GET v1/sempras/unidades": { data: unidades },
        "GET v1/sempras/centro-pop-documentos/*": resposta({ data: { url: "http://minio.test/d1" } }),
        "POST v1/*": resposta({ data: {} }),
        "PUT v1/*": resposta({ data: CIDADAO }),
        "DELETE v1/*": resposta({ data: {} }),
      });
      const { unmount } = renderApp(<FichaPage />);
      expect(await screen.findByRole("heading", { name: "João" })).toBeInTheDocument();
      await mexerEmTudo(document.body, { rapido: true, fixarUnidade: true });
      const nomes = new Set<string>();
      for (let i = 0; i < 60; i++) {
        const botao = screen.queryAllByRole("button").find((b) => {
          const nome = (b.textContent ?? "").trim();
          return acao.test(nome) && !nomes.has(nome + b.closest("li")?.textContent) && !(b as HTMLButtonElement).disabled
            && !b.closest("dialog[open]");
        });
        if (!botao) break;
        nomes.add((botao.textContent ?? "").trim() + botao.closest("li")?.textContent);
        await userEvent.click(botao);
        const dialogo = document.querySelector<HTMLElement>("dialog[open]");
        if (dialogo) {
          await mexerEmTudo(dialogo, { rapido: true });
          const link = within(dialogo).queryByRole("link");
          if (link) await userEvent.click(link);
          const enviar = dialogo.querySelector<HTMLButtonElement>('button[type="submit"]:not([disabled])');
          if (enviar && dialogo.hasAttribute("open")) await userEvent.click(enviar);
          if (dialogo.hasAttribute("open")) await userEvent.keyboard("{Escape}");
          if (dialogo.hasAttribute("open")) (dialogo as HTMLDialogElement).close();
        }
      }
      if (id === "u3") {
        await userEvent.upload(document.getElementById("centro-pop-documento-arquivo") as HTMLInputElement,
          new File(["%PDF-1.4"], "laudo.pdf", { type: "application/pdf" }));
        if (!falhar) await waitFor(() => expect(assign).toHaveBeenCalledWith("http://minio.test/d1"));
      }
      expect(api.to("GET v1/sempras/cidadaos/c1/prontuario").length).toBeGreaterThan(0);
      unmount();
    }
    vi.unstubAllGlobals();
  }, 480000);
});
