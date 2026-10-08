import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ModalShell } from "./ModalShell";

// O <dialog> nativo é stubado em vitest.setup.ts (showModal/close refletem
// o atributo `open`), então a lógica real da ModalShell é exercitada.

describe("ModalShell", () => {
  it("abre o <dialog> quando open=true e o fecha quando open=false", () => {
    const { rerender } = render(
      <ModalShell open onClose={() => {}}>
        <p>conteúdo</p>
      </ModalShell>,
    );
    const dialog = document.querySelector("dialog");
    expect(dialog).toHaveAttribute("open");

    rerender(
      <ModalShell open={false} onClose={() => {}}>
        <p>conteúdo</p>
      </ModalShell>,
    );
    expect(document.querySelector("dialog")).not.toHaveAttribute("open");
  });

  it("chama onClose no evento cancel (tecla Esc do <dialog>)", () => {
    const onClose = vi.fn();
    render(
      <ModalShell open onClose={onClose}>
        <p>x</p>
      </ModalShell>,
    );
    document.querySelector("dialog")!.dispatchEvent(new Event("cancel"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("fecha ao clicar no backdrop (clique no próprio elemento <dialog>)", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(
      <ModalShell open onClose={onClose}>
        <button>dentro</button>
      </ModalShell>,
    );

    // Clique dentro do conteúdo: NÃO fecha.
    await user.click(screen.getByRole("button", { name: "dentro" }));
    expect(onClose).not.toHaveBeenCalled();

    // Clique no <dialog> em si (a área do backdrop): fecha.
    await user.click(document.querySelector("dialog")!);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("trava o scroll do body enquanto aberto e restaura ao fechar", () => {
    const { rerender, unmount } = render(
      <ModalShell open onClose={() => {}}>
        <p>x</p>
      </ModalShell>,
    );
    expect(document.body.style.overflow).toBe("hidden");

    rerender(
      <ModalShell open={false} onClose={() => {}}>
        <p>x</p>
      </ModalShell>,
    );
    expect(document.body.style.overflow).not.toBe("hidden");
    unmount();
  });

  it("aplica a classe de largura conforme size", () => {
    render(
      <ModalShell open onClose={() => {}} size="xl">
        <p>x</p>
      </ModalShell>,
    );
    expect(document.querySelector("dialog")).toHaveClass("max-w-3xl");
  });
});
