import { ApiError } from "@/lib/api/client";

// Cliente de tempo real do Nexus. O servidor trabalha com TÓPICOS
// (internal/platform/ws): "broadcast" e "user:<id>" são assinados
// automaticamente; tópicos de plugin ("mercurio:room:<id>") são assinados
// sob demanda e revalidados pelo autorizador do módulo. Frames:
//   { "type": "...", "topic"?: "...", "data"?: ... }
//
// "unauthorized" é TERMINAL: o ticket foi negado com 401 (sessão expirou)
// — reconectar não adianta até um novo login.
export type ConnectionState = "idle" | "connecting" | "open" | "closed" | "unauthorized";

export interface Frame {
  type: string;
  topic?: string;
  data?: unknown;
}

export type FrameHandler = (frame: Frame) => void;

interface RealtimeClientOptions {
  /** Ticket de uso único (POST /api/v1/ws/ticket) — um novo a cada conexão. */
  getTicket: () => Promise<string>;
  wsBaseUrl: string;
  onStateChange?: (state: ConnectionState) => void;
  /** Sobrescrevível nos testes. */
  backoffMs?: (attempt: number) => number;
  /** Sobrescrevível nos testes. */
  socketFactory?: (url: string) => WebSocket;
}

const MAX_BACKOFF_MS = 30_000;

export function defaultBackoff(attempt: number): number {
  return Math.min(1000 * 2 ** attempt, MAX_BACKOFF_MS);
}

export function parseFrame(raw: string): Frame | null {
  try {
    const v = JSON.parse(raw) as unknown;
    if (v && typeof v === "object" && typeof (v as Frame).type === "string") return v as Frame;
  } catch {
    // frame malformado
  }
  return null;
}

/**
 * Uma conexão lógica com reconexão exponencial limitada (nunca um loop
 * agressivo), reassinatura automática dos tópicos após reconectar e
 * distribuição dos frames para os assinantes locais.
 */
export class RealtimeClient {
  private socket: WebSocket | null = null;
  private attempt = 0;
  private stopped = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly topics = new Map<string, number>();
  private readonly handlers = new Set<FrameHandler>();
  state: ConnectionState = "idle";

  constructor(private readonly opts: RealtimeClientOptions) {}

  connect(): void {
    this.stopped = false;
    void this.open();
  }

  disconnect(): void {
    this.stopped = true;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    this.socket?.close(1000, "client disconnect");
    this.socket = null;
  }

  /** Registra um ouvinte de frames; devolve a função de remoção. */
  onFrame(handler: FrameHandler): () => void {
    this.handlers.add(handler);
    return () => this.handlers.delete(handler);
  }

  /** Assina um tópico (contagem de referências: vários componentes podem
   * assinar o mesmo tópico). Devolve a função de cancelamento. */
  subscribe(topic: string): () => void {
    const n = this.topics.get(topic) ?? 0;
    this.topics.set(topic, n + 1);
    if (n === 0) this.send({ type: "subscribe", topic });
    return () => {
      const cur = this.topics.get(topic) ?? 0;
      if (cur <= 1) {
        this.topics.delete(topic);
        this.send({ type: "unsubscribe", topic });
      } else {
        this.topics.set(topic, cur - 1);
      }
    };
  }

  send(frame: Frame): void {
    if (this.socket && this.socket.readyState === 1) {
      this.socket.send(JSON.stringify(frame));
    }
  }

  private async open(): Promise<void> {
    if (this.stopped) return;
    this.setState("connecting");
    let ticket: string;
    try {
      ticket = await this.opts.getTicket();
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        this.setState("unauthorized");
        return;
      }
      this.scheduleReconnect();
      return;
    }
    if (this.stopped) return;

    const url = `${this.opts.wsBaseUrl}?ticket=${encodeURIComponent(ticket)}`;
    const socket = this.opts.socketFactory ? this.opts.socketFactory(url) : new WebSocket(url);
    this.socket = socket;

    socket.onopen = () => {
      this.attempt = 0;
      this.setState("open");
      for (const topic of this.topics.keys()) socket.send(JSON.stringify({ type: "subscribe", topic }));
    };
    socket.onmessage = (ev) => {
      const frame = parseFrame(typeof ev.data === "string" ? ev.data : "");
      if (!frame) return;
      for (const h of this.handlers) h(frame);
    };
    socket.onclose = () => {
      this.setState("closed");
      if (!this.stopped) this.scheduleReconnect();
    };
    socket.onerror = () => socket.close();
  }

  private scheduleReconnect(): void {
    if (this.stopped) return;
    const delay = (this.opts.backoffMs ?? defaultBackoff)(this.attempt);
    this.attempt += 1;
    this.reconnectTimer = setTimeout(() => void this.open(), delay);
  }

  private setState(state: ConnectionState): void {
    this.state = state;
    this.opts.onStateChange?.(state);
  }
}
