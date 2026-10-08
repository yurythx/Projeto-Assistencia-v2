import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { Frame } from "@/lib/websocket/client";

// useFrames é quem fala com o WebSocket — mockado para capturar o handler.
let captured: ((f: Frame) => void) | undefined;
vi.mock("@/components/realtime/RealtimeProvider", () => ({
  useFrames: (h: (f: Frame) => void) => {
    captured = h;
  },
}));
const refreshModules = vi.fn();
vi.mock("@/lib/nexus/NexusProvider", () => ({ useNexus: () => ({ refreshModules }) }));

import { describeEvent, NotificationCenter } from "./NotificationCenter";
import { NotificationHistoryProvider, useNotificationHistory } from "./NotificationHistoryProvider";
import { ToastProvider } from "./ToastProvider";

function HistoryCount() {
  const { items } = useNotificationHistory();
  return <p data-testid="history-count">{items.length}</p>;
}

function renderCenter() {
  return render(
    <ToastProvider>
      <NotificationHistoryProvider>
        <HistoryCount />
        <NotificationCenter />
      </NotificationHistoryProvider>
    </ToastProvider>,
  );
}

const envelope = (type: string, payload: unknown) => ({
  type: "event",
  topic: "broadcast",
  data: { id: "e1", type, version: 1, source: "nexus", occurred_at: "2026-09-25T12:00:00Z", correlation_id: "c1", payload },
});

describe("NotificationCenter", () => {
  it("evento de publicação vira toast + histórico", () => {
    renderCenter();
    act(() => captured!(envelope("blog.post.published", { title: "Manutenção", kind: "comunicado" })));
    expect(screen.getByText("Novo comunicado")).toBeInTheDocument();
    expect(screen.getByText("Manutenção")).toBeInTheDocument();
    expect(screen.getByTestId("history-count")).toHaveTextContent("1");
  });

  it("evento desconhecido ou envelope inválido é ignorado", () => {
    renderCenter();
    act(() => captured!(envelope("algo.que.ninguem.emite", {})));
    act(() => captured!({ type: "event", data: { lixo: true } }));
    expect(screen.getByTestId("history-count")).toHaveTextContent("0");
  });

  it("module.disabled atualiza os módulos e avisa", () => {
    renderCenter();
    act(() => captured!({ type: "module.disabled", data: "blog" }));
    expect(refreshModules).toHaveBeenCalled();
    expect(screen.getByTestId("history-count")).toHaveTextContent("1");
  });

  it("describeEvent cobre os eventos de difusão", () => {
    expect(describeEvent("catalog.service.published", { title: "X" })?.tone).toBe("success");
    expect(describeEvent("tramite.processo.aberto", {})).toBeNull();
  });
});
