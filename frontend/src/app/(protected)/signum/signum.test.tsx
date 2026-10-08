import { createHash } from "node:crypto";

import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, ME, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import SignumPage from "./page";
import EnvelopePage from "./[id]/page";

const DOC = "conteúdo do contrato";
const SHA = createHash("sha256").update(DOC).digest("hex");
const docFile = (content = DOC, name = "contrato.pdf") => new File([content], name, { type: "application/pdf" });

const signer = (id: string, extra: Record<string, unknown> = {}) => ({ id, user_id: `u-${id}`, name: `Pessoa ${id}`, position: 1, status: "pending", ...extra });
const envelope = (extra: Record<string, unknown> = {}) => ({
  id: "env1", title: "Contrato 12/2026", description: "Prestação de serviço", document_sha256: SHA, source_module: "", source_ref: "",
  sequential: false, status: "pending", created_by: "outro", created_by_name: "João", created_at: "2026-01-01T10:00:00Z",
  signers: [signer("s1", { user_id: ME.id })], ...extra,
});

describe("Signum — envelopes", () => {
  beforeEach(() => resetNavigation());

  it("abas de filtro, contagem de assinaturas e aba de gestão", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/signum/envelopes": (req) =>
        req.query.get("role") === "any"
          ? fail(500, "falha")
          : { data: req.query.get("role") === "created" ? [] : [envelope({ source_module: "tramite", signers: [signer("a", { status: "signed" }), signer("b")] })] },
    });
    renderApp(<SignumPage />);
    expect(await screen.findByRole("link", { name: /Contrato 12\/2026/ })).toHaveAttribute("href", "/signum/env1");
    expect(screen.getByText("1/2 assinaturas")).toBeInTheDocument();
    expect(screen.getByText(/origem: tramite/)).toBeInTheDocument();
    expect(screen.getByText("Aguardando assinaturas")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("tab", { name: "Criados por mim" }));
    expect(await screen.findByText("Nenhum envelope")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "Todos (gestão)" }));
    expect(await screen.findByText("falha")).toBeInTheDocument();
    expect(api.to("GET v1/signum/envelopes").map((c) => c.query.get("role"))).toEqual(["to_sign", "created", "any"]);
  });

  it("sem signum:manage não há aba de gestão", async () => {
    mockBackend({ ...identityRoutes({ permissions: ["signum:read"] }), "GET v1/signum/envelopes": { data: [] } });
    renderApp(<SignumPage />);
    await screen.findByText("Nenhum envelope");
    expect(screen.queryByRole("tab", { name: "Todos (gestão)" })).not.toBeInTheDocument();
  });

  it("novo envelope: hash calculado no navegador, signatários e título padrão", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/signum/envelopes": { data: [] },
      "POST v1/signum/envelopes": { data: envelope({ id: "novo" }) },
    });
    renderApp(<SignumPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Novo envelope/ }));
    const dialog = await screen.findByRole("dialog", { name: "Novo envelope de assinatura" });
    const submit = within(dialog).getByRole("button", { name: "Abrir envelope" });
    expect(submit).toBeDisabled();
    expect(within(dialog).getByText("O arquivo é lido apenas localmente para calcular o hash.")).toBeInTheDocument();

    await userEvent.upload(within(dialog).getByLabelText("Documento *"), docFile());
    expect(await within(dialog).findByText(`SHA-256: ${SHA}`)).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Título")).toHaveAttribute("placeholder", "contrato.pdf");
    expect(submit).toBeDisabled(); // falta signatário

    const uuid = "11111111-2222-3333-4444-555555555555";
    await userEvent.type(within(dialog).getByLabelText("Signatários *"), `${uuid}{Enter}`);
    await userEvent.click(within(dialog).getByLabelText(/Assinatura sequencial/));
    await userEvent.click(submit);
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/signum/novo"));
    expect(api.to("POST v1/signum/envelopes")[0]!.body).toEqual({
      title: "contrato.pdf", description: "", document_sha256: SHA, signer_ids: [uuid], sequential: true,
    });
  });
});

describe("Signum — envelope", () => {
  beforeEach(() => resetNavigation({ id: "env1" }));

  it("cerimônia completa: confere o documento, gera o desafio e reautentica", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/signum/envelopes/env1": { data: envelope() },
      "POST v1/signum/envelopes/env1/challenge": { data: { challenge_id: "c1", nonce: "n1", expires_at: "2026-01-01T10:05:00Z", document_sha256: SHA } },
      "POST v1/signum/envelopes/env1/sign": (req) =>
        (req.body as { password: string }).password === "errada"
          ? fail(401, "senha incorreta — a assinatura não foi realizada", "SIGNUM_REAUTH_FAILED")
          : { data: envelope({ status: "completed" }) },
    });
    renderApp(<EnvelopePage />);
    expect(await screen.findByRole("heading", { name: "Contrato 12/2026" })).toBeInTheDocument();
    expect(screen.getByText(`SHA-256 do documento: ${SHA}`)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Página pública de verificação/ })).toHaveAttribute("href", "/verificar/env1");
    // não é o criador nem gestor... mas tem "*": pode cancelar
    expect(screen.getByRole("button", { name: /Cancelar envelope/ })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /^Assinar$/ }));
    const dialog = await screen.findByRole("dialog", { name: "Assinar documento" });
    const gerar = within(dialog).getByRole("button", { name: "Gerar desafio de assinatura" });
    expect(gerar).toBeDisabled();

    // arquivo errado: não confere e não libera
    await userEvent.upload(within(dialog).getByLabelText("Selecione o arquivo que você vai assinar"), docFile("outro conteúdo", "outro.pdf"));
    expect(await within(dialog).findByText("Este arquivo NÃO é o documento do envelope.")).toBeInTheDocument();
    expect(gerar).toBeDisabled();

    await userEvent.upload(within(dialog).getByLabelText("Selecione o arquivo que você vai assinar"), docFile());
    expect(await within(dialog).findByText("Documento íntegro — idêntico ao do envelope.")).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Sua senha")).toBeDisabled();
    await userEvent.click(gerar);
    expect(await within(dialog).findByText(/Desafio válido até/)).toBeInTheDocument();
    expect(gerar).toBeDisabled();

    // senha errada: avisa, mantém o desafio e a sessão (sem ir ao login)
    await userEvent.type(within(dialog).getByLabelText("Sua senha"), "errada");
    await userEvent.click(within(dialog).getByRole("button", { name: /Assinar/ }));
    expect(await screen.findByText("senha incorreta — a assinatura não foi realizada")).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Sua senha")).toHaveValue("");
    expect(within(dialog).getByText(/Desafio válido até/)).toBeInTheDocument();

    await userEvent.type(within(dialog).getByLabelText("Sua senha"), "Senha-Certa-1");
    await userEvent.click(within(dialog).getByRole("button", { name: /Assinar/ }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Assinar documento" })).not.toBeInTheDocument());
    expect(api.to("POST v1/signum/envelopes/env1/sign").at(-1)!.body).toEqual({
      challenge_id: "c1", nonce: "n1", password: "Senha-Certa-1", confirm_document_sha256: SHA,
    });
  });

  it("desafio expirado libera gerar outro", async () => {
    let n = 0;
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/signum/envelopes/env1": { data: envelope() },
      "POST v1/signum/envelopes/env1/challenge": () => ({ data: { challenge_id: `c${++n}`, nonce: "n", expires_at: "2026-01-01T10:05:00Z", document_sha256: SHA } }),
      "POST v1/signum/envelopes/env1/sign": fail(422, "signum: desafio inválido, expirado ou já utilizado", "VALIDATION_ERROR"),
    });
    renderApp(<EnvelopePage />);
    await userEvent.click(await screen.findByRole("button", { name: /^Assinar$/ }));
    const dialog = await screen.findByRole("dialog", { name: "Assinar documento" });
    await userEvent.upload(within(dialog).getByLabelText("Selecione o arquivo que você vai assinar"), docFile());
    await userEvent.click(await within(dialog).findByRole("button", { name: "Gerar desafio de assinatura" }));
    await userEvent.type(await within(dialog).findByLabelText("Sua senha"), "x");
    await waitFor(() => expect(within(dialog).getByLabelText("Sua senha")).toBeEnabled());
    await userEvent.type(within(dialog).getByLabelText("Sua senha"), "y");
    await userEvent.click(within(dialog).getByRole("button", { name: /Assinar/ }));
    expect(await screen.findByText("signum: desafio inválido, expirado ou já utilizado")).toBeInTheDocument();
    const gerar = within(dialog).getByRole("button", { name: "Gerar desafio de assinatura" });
    await waitFor(() => expect(gerar).toBeEnabled());
    await userEvent.click(gerar);
    await waitFor(() => expect(api.to("POST v1/signum/envelopes/env1/challenge")).toHaveLength(2));
  });

  it("falha ao gerar o desafio não abre o passo da senha", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/signum/envelopes/env1": { data: envelope() },
      "POST v1/signum/envelopes/env1/challenge": fail(409, "não é a sua vez"),
    });
    renderApp(<EnvelopePage />);
    await userEvent.click(await screen.findByRole("button", { name: /^Assinar$/ }));
    const dialog = await screen.findByRole("dialog", { name: "Assinar documento" });
    await userEvent.upload(within(dialog).getByLabelText("Selecione o arquivo que você vai assinar"), docFile());
    await userEvent.click(await within(dialog).findByRole("button", { name: "Gerar desafio de assinatura" }));
    expect(await screen.findByText("não é a sua vez")).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Sua senha")).toBeDisabled();
  });

  it("recusa exige motivo", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/signum/envelopes/env1": { data: envelope() },
      "POST v1/signum/envelopes/env1/refuse": { data: envelope({ status: "refused" }) },
    });
    renderApp(<EnvelopePage />);
    await userEvent.click(await screen.findByRole("button", { name: /Recusar/ }));
    const dialog = await screen.findByRole("dialog", { name: "Recusar assinatura" });
    const recusar = within(dialog).getByRole("button", { name: "Recusar" });
    await userEvent.type(within(dialog).getByLabelText("Motivo *"), " a ");
    expect(recusar).toBeDisabled();
    await userEvent.type(within(dialog).getByLabelText("Motivo *"), "valor divergente");
    await userEvent.click(recusar);
    await waitFor(() => expect(api.to("POST v1/signum/envelopes/env1/refuse")[0]?.body).toEqual({ reason: "a valor divergente" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Recusar assinatura" })).not.toBeInTheDocument());
  });

  it("recusa com erro mantém o diálogo", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/signum/envelopes/env1": { data: envelope() },
      "POST v1/signum/envelopes/env1/refuse": fail(409, "envelope encerrado"),
    });
    renderApp(<EnvelopePage />);
    await userEvent.click(await screen.findByRole("button", { name: /Recusar/ }));
    const dialog = await screen.findByRole("dialog", { name: "Recusar assinatura" });
    await userEvent.type(within(dialog).getByLabelText("Motivo *"), "motivo");
    await userEvent.click(within(dialog).getByRole("button", { name: "Recusar" }));
    expect(await screen.findByText("envelope encerrado")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Recusar assinatura" })).toBeInTheDocument();
  });

  it("sequencial: só a vez do primeiro pendente; criador cancela", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: ["signum:read"] }),
      "GET v1/signum/envelopes/env1": {
        data: envelope({
          sequential: true, created_by: ME.id, description: "",
          signers: [
            signer("s2", { user_id: ME.id, position: 2 }),
            signer("s1", { position: 1 }),
          ],
        }),
      },
      "POST v1/signum/envelopes/env1/cancel": { data: null },
    });
    renderApp(<EnvelopePage />);
    expect(await screen.findByText(/assinatura sequencial/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Assinar$/ })).not.toBeInTheDocument();
    const names = screen.getAllByText(/Pessoa s/).map((el) => el.textContent);
    expect(names).toEqual(["1.Pessoa s1", "2.Pessoa s2"]);

    await userEvent.click(screen.getByRole("button", { name: /Cancelar envelope/ }));
    const dialog = await screen.findByRole("dialog", { name: "Cancelar envelope?" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancelar envelope" }));
    await waitFor(() => expect(api.to("POST v1/signum/envelopes/env1/cancel")).toHaveLength(1));
  });

  it("concluído: mostra carimbos, sem ações", async () => {
    mockBackend({
      ...identityRoutes({ permissions: ["signum:read"] }),
      "GET v1/signum/envelopes/env1": {
        data: envelope({
          status: "completed", completed_at: "2026-01-02T10:00:00Z",
          signers: [
            signer("s1", { user_id: ME.id, status: "signed", signed_at: "2026-01-02T10:00:00Z", method: "local-argon2id", signature_hash: "selo123" }),
            signer("s2", { status: "refused", reason: "fora do prazo", position: 2 }),
          ],
        }),
      },
    });
    renderApp(<EnvelopePage />);
    expect(await screen.findByText(/concluído em/)).toBeInTheDocument();
    expect(screen.getByText(/método: local-argon2id/)).toBeInTheDocument();
    expect(screen.getByText("selo123")).toBeInTheDocument();
    expect(screen.getByText("Motivo: fora do prazo")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Assinar|Recusar|Cancelar envelope/ })).not.toBeInTheDocument();
  });

  it("envelope inexistente", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/signum/envelopes/env1": fail(404, "envelope não encontrado") });
    renderApp(<EnvelopePage />);
    expect(await screen.findByText("envelope não encontrado")).toBeInTheDocument();
  });
});
