import { render, screen } from "@testing-library/react";
import { SWRConfig } from "swr";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PlatformMonitoringDashboard } from "./PlatformMonitoringDashboard";
import { ConnectionStateProvider } from "@/components/layout/ConnectionStateContext";
import type { ConnectionState } from "@/lib/websocket/client";

// fetch() é compartilhado por duas fontes bem diferentes neste
// componente: /api/health (fetch cru, rota própria do frontend) e
// v1/monitoring/outbox-stats (apiClient, que por baixo também chama
// fetch — em /api/backend/v1/monitoring/outbox-stats). Um mock só
// precisa responder de acordo com a URL pedida.
function mockBackend({
  health,
  healthStatus = 200,
  outbox,
}: {
  health?: { postgres?: string; rabbitmq?: string; minio?: string };
  healthStatus?: number;
  outbox?: { pending: number; published: number; failed: number };
}) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      if (url.includes("/api/health")) {
        const services = Object.fromEntries(
          Object.entries(health ?? {}).map(([name, s]) => [name, { status: s }]),
        );
        return {
          ok: healthStatus >= 200 && healthStatus < 300,
          status: healthStatus,
          json: async () => ({
            data: {
              status: healthStatus === 200 ? "ok" : "degraded",
              timestamp: new Date().toISOString(),
              services,
            },
            error: null,
          }),
        };
      }
      if (url.includes("outbox-stats")) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ data: outbox ?? { pending: 0, published: 0, failed: 0 }, error: null }),
        };
      }
      throw new Error(`URL inesperada no mock de fetch: ${url}`);
    }),
  );
}

// SWR mantém um cache GLOBAL por chave entre renders — sem isto, o
// resultado de um teste anterior continuava visível como dado "stale" no
// início do próximo teste, antes do novo mock de fetch sequer resolver,
// produzindo falsos positivos. `provider: () => new Map()` dá um cache
// novo e isolado a cada render.
function renderDashboard(connectionState: ConnectionState = "idle") {
  return render(
    <SWRConfig value={{ provider: () => new Map() }}>
      <ConnectionStateProvider value={connectionState}>
        <PlatformMonitoringDashboard />
      </ConnectionStateProvider>
    </SWRConfig>,
  );
}

describe("PlatformMonitoringDashboard", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // Achado de auditoria corrigido: os quatro cartões de infraestrutura
  // mostravam "online" fixo no código-fonte, sempre — nunca refletiam o
  // /ready de verdade do backend. Este teste cobre a derivação honesta:
  // um serviço que o /ready reporta "unavailable" precisa aparecer como
  // tal na tela, não como "Online".
  it("reflete o status real do backend — rabbitmq indisponível aparece como Offline", async () => {
    mockBackend({ health: { postgres: "ok", rabbitmq: "unavailable" }, healthStatus: 503 });
    renderDashboard();

    expect(await screen.findByText("PostgreSQL 16 Engine")).toBeInTheDocument();

    const rabbitCard = await screen.findByText("RabbitMQ AMQP Broker");
    expect(rabbitCard.closest(".relative")!.textContent).toContain("Offline");

    const postgresCard = screen.getByText("PostgreSQL 16 Engine");
    expect(postgresCard.closest(".relative")!.textContent).toContain("Online");
  });

  // Achado de auditoria (2ª rodada): o MinIO nunca tinha uma checagem real
  // — /ready só verificava postgres/rabbitmq — então o card ficava preso
  // em "Desconhecido" pra sempre. Corrigido com storage.Provider.Ping
  // (backend) — agora reflete o /ready de verdade, igual aos outros três.
  it("reflete o status real do MinIO quando o backend passa a checá-lo", async () => {
    mockBackend({ health: { postgres: "ok", rabbitmq: "ok", minio: "ok" } });
    renderDashboard();

    await screen.findByText("PostgreSQL 16 Engine");
    const minioCard = await screen.findByText("MinIO Object Storage (S3)");
    expect(minioCard.closest(".relative")!.textContent).toContain("Online");
  });

  it("MinIO indisponível aparece como Offline, não mais preso em 'Desconhecido'", async () => {
    mockBackend({ health: { postgres: "ok", rabbitmq: "ok", minio: "unavailable" }, healthStatus: 503 });
    renderDashboard();

    await screen.findByText("PostgreSQL 16 Engine");
    const minioCard = await screen.findByText("MinIO Object Storage (S3)");
    expect(minioCard.closest(".relative")!.textContent).toContain("Offline");
  });

  it("não existe mais um botão \"Testar\" chamando um endpoint de integração inexistente", async () => {
    mockBackend({ health: { postgres: "ok", rabbitmq: "ok" } });
    renderDashboard();

    await screen.findByText("PostgreSQL 16 Engine");
    expect(screen.queryByRole("button", { name: /testar/i })).not.toBeInTheDocument();
  });

  it("backend inalcançável — nenhum serviço aparece como Online", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network error")));
    renderDashboard();

    await screen.findByText("PostgreSQL 16 Engine");
    await screen.findAllByText("Offline");
    expect(screen.queryAllByText("Online")).toHaveLength(0);
  });

  // Achado de auditoria corrigido: "Outbox Event Queue: 0 Pendentes" era
  // fixo no código-fonte, não importa o que houvesse na fila de verdade.
  it("mostra a contagem real de outbox_events pendentes (GET /v1/monitoring/outbox-stats)", async () => {
    mockBackend({
      health: { postgres: "ok", rabbitmq: "ok" },
      outbox: { pending: 7, published: 120, failed: 2 },
    });
    renderDashboard();

    expect(await screen.findByText("7 Pendentes")).toBeInTheDocument();
    expect(screen.getByText("2 com falha (Dead Letter)")).toBeInTheDocument();
  });

  // Achado de auditoria corrigido: "Conexão WebSocket: Ativa" era fixo no
  // código-fonte, mesmo com o WebSocket de fato desconectado.
  it("reflete o estado real da conexão WebSocket (ConnectionStateContext)", async () => {
    mockBackend({ health: { postgres: "ok", rabbitmq: "ok" } });
    renderDashboard("closed");

    await screen.findByText("PostgreSQL 16 Engine");
    expect(screen.getByText("Reconectando…")).toBeInTheDocument();
    expect(screen.queryByText("Ao vivo")).not.toBeInTheDocument();
  });
});
