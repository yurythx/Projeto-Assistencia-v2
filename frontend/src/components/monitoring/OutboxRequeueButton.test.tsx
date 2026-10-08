import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { OutboxRequeueButton } from "./OutboxRequeueButton";
import { ToastProvider } from "@/components/notifications/ToastProvider";
import { apiClient } from "@/lib/api/client";

describe("OutboxRequeueButton", () => {
  afterEach(() => vi.restoreAllMocks());

  it("pede confirmação, chama a API e atualiza o painel", async () => {
    const post = vi.spyOn(apiClient, "post").mockResolvedValue({ data: { requeued: 3 } } as never);
    const onDone = vi.fn();
    render(
      <ToastProvider>
        <OutboxRequeueButton failed={3} onDone={onDone} />
      </ToastProvider>,
    );
    await userEvent.click(screen.getByRole("button", { name: /reprocessar/i }));
    expect(screen.getByText(/3 evento\(s\) voltam à fila/)).toBeInTheDocument();
    expect(post).not.toHaveBeenCalled();
    await userEvent.click(screen.getAllByRole("button", { name: "Reprocessar" }).at(-1)!);
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(post).toHaveBeenCalledWith("v1/monitoring/outbox/requeue", {});
  });

  it("não atualiza o painel se a API recusar", async () => {
    vi.spyOn(apiClient, "post").mockRejectedValue(new Error("403"));
    const onDone = vi.fn();
    render(
      <ToastProvider>
        <OutboxRequeueButton failed={1} onDone={onDone} />
      </ToastProvider>,
    );
    await userEvent.click(screen.getByRole("button", { name: /reprocessar/i }));
    await userEvent.click(screen.getAllByRole("button", { name: "Reprocessar" }).at(-1)!);
    await waitFor(() => expect(screen.getByText("Operação não concluída")).toBeInTheDocument());
    expect(onDone).not.toHaveBeenCalled();
  });
});
