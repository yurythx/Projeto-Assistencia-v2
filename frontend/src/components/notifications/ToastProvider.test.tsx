import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider, useToast } from "./ToastProvider";

// Componente mínimo só para disparar showToast() de dentro da árvore do
// Provider — useToast() exige um ToastProvider acima, então não dá pra
// chamar showToast diretamente no teste.
function Trigger() {
  const { showToast } = useToast();
  return (
    <button type="button" onClick={() => showToast({ title: "Integração online" })}>
      disparar
    </button>
  );
}

// O dismiss automático roda dentro de um setTimeout — fora de qualquer
// handler de evento do React, então o setState que ele dispara só
// re-renderiza de fato se o avanço do timer estiver dentro de act().
function advance(ms: number) {
  act(() => {
    vi.advanceTimersByTime(ms);
  });
}

describe("ToastProvider", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("dispensa o toast sozinho depois de 6s", () => {
    render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "disparar" }));
    expect(screen.getByText("Integração online")).toBeInTheDocument();

    advance(6000);
    expect(screen.queryByText("Integração online")).not.toBeInTheDocument();
  });

  // A-16 (WCAG 2.2.1 Timing Adjustable): passar o mouse por cima precisa
  // pausar a contagem — sem isso, um toast pode sumir enquanto o usuário
  // ainda está lendo ou tentando alcançar o botão de dispensar.
  it("pausa o auto-dismiss com o mouse sobre o toast e retoma ao sair", () => {
    render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "disparar" }));
    // <p>título</p> -> <div> (título+descrição) -> <div flex...> -> <div>
    // raiz do Toast (onde o onMouseEnter/onMouseLeave de fato estão).
    const toast = screen.getByText("Integração online").parentElement!.parentElement!.parentElement!;

    // Passa boa parte do tempo, depois entra com o mouse ANTES de vencer.
    advance(4000);
    fireEvent.mouseEnter(toast);

    // Mesmo esperando bem além dos 6s originais, o toast continua —
    // pausado, não corre contra o relógio.
    advance(10_000);
    expect(screen.getByText("Integração online")).toBeInTheDocument();

    // Sai do hover: retoma pelos ~2s que faltavam quando pausou.
    fireEvent.mouseLeave(toast);
    advance(1999);
    expect(screen.getByText("Integração online")).toBeInTheDocument();
    advance(1);
    expect(screen.queryByText("Integração online")).not.toBeInTheDocument();
  });

  // Mesmo comportamento por teclado (foco), não só mouse — um usuário
  // navegando por Tab até o botão "✕" do toast não pode ter o conteúdo
  // sumindo debaixo do foco.
  it("pausa o auto-dismiss com o foco de teclado sobre o toast", () => {
    render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "disparar" }));
    const dismissButton = screen.getByRole("button", { name: "Dispensar notificação" });

    advance(5000);
    fireEvent.focus(dismissButton);

    advance(10_000);
    expect(screen.getByText("Integração online")).toBeInTheDocument();

    fireEvent.blur(dismissButton);
    advance(1000);
    expect(screen.queryByText("Integração online")).not.toBeInTheDocument();
  });

  it("dispensa manualmente pelo botão, mesmo antes do timer vencer", () => {
    render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "disparar" }));
    fireEvent.click(screen.getByRole("button", { name: "Dispensar notificação" }));
    expect(screen.queryByText("Integração online")).not.toBeInTheDocument();
  });
});
