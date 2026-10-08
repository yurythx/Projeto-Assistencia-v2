import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/client";

import { ConfirmButton } from "./ConfirmButton";
import { DataState } from "./DataState";

describe("ConfirmButton", () => {
  it("só executa após confirmar no diálogo; cancelar não executa", async () => {
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    render(
      <ConfirmButton title="Excluir?" confirmLabel="Excluir" onConfirm={onConfirm}>
        Remover
      </ConfirmButton>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Remover" }));
    await userEvent.click(screen.getByRole("button", { name: "Cancelar" }));
    expect(onConfirm).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "Remover" }));
    await userEvent.click(screen.getByRole("button", { name: "Excluir" }));
    await waitFor(() => expect(onConfirm).toHaveBeenCalledTimes(1));
  });
});

describe("DataState", () => {
  it("prioriza erro, depois carregamento, depois vazio", () => {
    const { rerender } = render(
      <DataState loading={false} error={new ApiError(403, "FORBIDDEN", "sem permissão")} empty={false}>
        dados
      </DataState>,
    );
    expect(screen.getByText("sem permissão")).toBeInTheDocument();
    rerender(<DataState loading error={null} empty={false}>dados</DataState>);
    expect(screen.getByRole("status")).toHaveTextContent("Carregando");
    rerender(<DataState loading={false} error={null} empty emptyTitle="Nada aqui">dados</DataState>);
    expect(screen.getByText("Nada aqui")).toBeInTheDocument();
    rerender(<DataState loading={false} error={null} empty={false}>dados</DataState>);
    expect(screen.getByText("dados")).toBeInTheDocument();
  });
});
