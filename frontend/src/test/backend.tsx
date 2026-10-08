// Harness dos testes de tela: um backend falso no nível do fetch (o
// apiClient, o SWR, o NexusProvider e o ToastProvider reais rodam por
// cima) — o teste enxerga as mesmas requisições que o proxy BFF receberia.
import { render } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { SWRConfig } from "swr";
import { vi } from "vitest";

import { ToastProvider } from "@/components/notifications/ToastProvider";
import { RealtimeProvider } from "@/components/realtime/RealtimeProvider";
import { NexusProvider } from "@/lib/nexus/NexusProvider";
import type { Me, ModuleStatus } from "@/lib/nexus/types";

export interface BackendRequest {
  method: string;
  /** caminho sem o prefixo /api/backend/ e sem query (ex.: "v1/blog/posts") */
  path: string;
  query: URLSearchParams;
  body: unknown;
}

export interface Reply {
  status?: number;
  data?: unknown;
  meta?: unknown;
  error?: { code: string; message: string };
}

type Route = Reply | ((req: BackendRequest) => Reply | Promise<Reply>);

/** Resposta de erro no envelope padrão. */
export function fail(status: number, message = "falhou", code = "ERR"): Reply {
  return { status, error: { code, message } };
}

/** Página paginada no envelope padrão. */
export function page(items: unknown[], extra: Partial<{ page: number; total_pages: number }> = {}): Reply {
  return {
    data: items,
    meta: { page: extra.page ?? 1, page_size: 20, total_items: items.length, total_pages: extra.total_pages ?? 1 },
  };
}

/** Instala o backend falso. As chaves são "MÉTODO caminho" (o caminho pode
 * terminar em "*" para casar um prefixo). Rota ausente responde 404. */
export function mockBackend(routes: Record<string, Route>) {
  const calls: BackendRequest[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), "http://localhost");
    // chamadas ao próprio BFF: "v1/..."; a outros hosts (ex.: PUT na URL
    // pré-assinada do MinIO): "host/caminho".
    const path = url.host === "localhost" ? url.pathname.replace(/^\/api\/backend\//, "") : url.host + url.pathname;
    const method = (init?.method ?? "GET").toUpperCase();
    let body: unknown = undefined;
    if (typeof init?.body === "string") {
      try {
        body = JSON.parse(init.body);
      } catch {
        body = init.body;
      }
    } else if (init?.body) {
      body = init.body;
    }
    const req: BackendRequest = { method, path, query: url.searchParams, body };
    calls.push(req);

    const key = `${method} ${path}`;
    const prefix = Object.keys(routes)
      .filter((k) => k.endsWith("*") && key.startsWith(k.slice(0, -1)))
      .sort((a, b) => b.length - a.length)[0];
    const route: Route = routes[key] ?? (prefix ? routes[prefix] : undefined) ?? { status: 404, error: { code: "NOT_FOUND", message: `sem rota: ${key}` } };
    const reply = typeof route === "function" ? await route(req) : route;
    const status = reply.status ?? 200;
    return new Response(
      JSON.stringify({ data: reply.data ?? null, meta: reply.meta, error: reply.error ?? null }),
      { status, headers: { "Content-Type": "application/json" } },
    );
  });
  vi.stubGlobal("fetch", fetchMock);

  return {
    calls,
    /** Requisições feitas para "MÉTODO caminho". */
    to: (key: string) => calls.filter((c) => `${c.method} ${c.path}` === key),
    fetch: fetchMock,
  };
}

export const ME: Me = {
  id: "00000000-0000-0000-0000-000000000001",
  subject: "sub-1",
  username: "maria",
  email: "maria@orgao.gov.br",
  name: "Maria Servidora",
  source: "local",
  roles: [],
  groups: [],
  permissions: ["*"],
  // Como a API entrega um administrador: concessão global com "*" (ADR 013).
  scopes: [{ perfil: "administrador", origem: "manual", permissions: ["*"] }],
};

export function moduleStatus(key: string, extra: Partial<ModuleStatus> = {}): ModuleStatus {
  return {
    key, name: key, description: "", core: false, default_enabled: true, depends_on: [], permissions: [],
    public: false, icon: "box", route: `/${key}`, enabled: true, configured: true, dependents: [], blocked_by: [], ...extra,
  };
}

/** Rotas do NexusProvider (identidade e módulos). */
export function identityRoutes(me: Partial<Me> = {}, modules: ModuleStatus[] = []): Record<string, Route> {
  return {
    "GET v1/me": { data: { ...ME, ...me } },
    "GET v1/system/modules": { data: modules },
  };
}

function Providers({ children }: { children: ReactNode }) {
  return (
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0, shouldRetryOnError: false }}>
      <ToastProvider>
        <NexusProvider>{children}</NexusProvider>
      </ToastProvider>
    </SWRConfig>
  );
}

/** Renderiza com os providers reais da área autenticada. */
export function renderApp(ui: ReactElement) {
  return render(ui, { wrapper: Providers });
}

// ------------------------------------------------------------ tempo real

/** WebSocket falso: o RealtimeProvider real conecta nele. */
export class FakeSocket {
  static instances: FakeSocket[] = [];
  readyState = 0;
  sent: Array<Record<string, unknown>> = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {
    FakeSocket.instances.push(this);
    // abre no próximo tick, como um servidor de verdade
    setTimeout(() => {
      if (this.readyState === 0) {
        this.readyState = 1;
        this.onopen?.();
      }
    }, 0);
  }
  send(data: string) {
    this.sent.push(JSON.parse(data));
  }
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
  /** Entrega um frame do servidor. */
  receive(frame: { type: string; topic?: string; data?: unknown }) {
    this.onmessage?.({ data: JSON.stringify(frame) });
  }
}

/** Instala o WebSocket falso e devolve o último socket aberto. */
export function mockWebSocket() {
  FakeSocket.instances = [];
  vi.stubGlobal("WebSocket", FakeSocket);
  return { socket: () => FakeSocket.instances.at(-1)! };
}

/** Rota do ticket de uso único do WebSocket. */
export const wsTicketRoute: Record<string, Route> = { "POST v1/ws/ticket": { data: { ticket: "t1" } } };

function RealtimeProviders({ children }: { children: ReactNode }) {
  return (
    <Providers>
      <RealtimeProvider>{children}</RealtimeProvider>
    </Providers>
  );
}

/** renderApp + RealtimeProvider real (use com mockWebSocket). */
export function renderRealtime(ui: ReactElement) {
  return render(ui, { wrapper: RealtimeProviders });
}
