import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const publicPost = vi.fn();
vi.mock("@/lib/api/publicClient", () => ({ publicPost: (...a: unknown[]) => publicPost(...a) }));

import { ApiError } from "@/lib/api/client";

import { ContactForm } from "./ContactForm";

async function fill() {
  await userEvent.type(screen.getByLabelText("Nome *"), "Maria Silva");
  await userEvent.type(screen.getByLabelText("E-mail *"), "maria@example.com");
  await userEvent.type(screen.getByLabelText("Assunto *"), "Prazo do serviço");
  await userEvent.type(screen.getByLabelText("Mensagem *"), "Gostaria de saber o prazo.");
}

describe("ContactForm", () => {
  beforeEach(() => {
    publicPost.mockReset();
  });

  it("envia pelo proxy público com consentimento e mostra o protocolo", async () => {
    publicPost.mockResolvedValue({ protocol: "CT-2026-000123" });
    render(<ContactForm defaultServiceSlug="certidao" />);
    expect(screen.getByLabelText("Assunto *")).toHaveValue("Serviço: certidao");
    await userEvent.clear(screen.getByLabelText("Assunto *"));
    await fill();
    await userEvent.click(screen.getByRole("checkbox", { name: /Autorizo o tratamento/ }));
    await userEvent.click(screen.getByRole("button", { name: "Enviar mensagem" }));

    expect(publicPost).toHaveBeenCalledWith("v1/contact/messages", expect.objectContaining({
      name: "Maria Silva", email: "maria@example.com", consent: true, website: "", category: "duvida",
    }));
    expect(await screen.findByText("CT-2026-000123")).toBeInTheDocument();
  });

  it("sem consentimento o navegador não envia (campo obrigatório)", async () => {
    render(<ContactForm />);
    await fill();
    await userEvent.click(screen.getByRole("button", { name: "Enviar mensagem" }));
    expect(publicPost).not.toHaveBeenCalled();
  });

  it("honeypot fica fora da ordem de tabulação e da árvore acessível", () => {
    const { container } = render(<ContactForm />);
    const hp = container.querySelector<HTMLInputElement>('input[name="website"]')!;
    expect(hp.tabIndex).toBe(-1);
    expect(hp.closest("[aria-hidden='true']")).not.toBeNull();
  });

  it("erro da API aparece em alerta acessível", async () => {
    const err = new ApiError(429, "RATE_LIMITED", "muitas mensagens; tente mais tarde");
    publicPost.mockImplementation(async () => {
      throw err;
    });
    render(<ContactForm />);
    await fill();
    await userEvent.click(screen.getByRole("checkbox", { name: /Autorizo/ }));
    await userEvent.click(screen.getByRole("button", { name: "Enviar mensagem" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("muitas mensagens");
  });
});
