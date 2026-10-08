import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/client";
import { defaultBackoff, parseFrame, RealtimeClient, type ConnectionState } from "@/lib/websocket/client";

class FakeSocket {
  readyState = 0;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {}
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  receive(frame: unknown) {
    this.onmessage?.({ data: JSON.stringify(frame) });
  }
}

function setup(getTicket: () => Promise<string> = () => Promise.resolve("t1")) {
  const sockets: FakeSocket[] = [];
  const states: ConnectionState[] = [];
  const client = new RealtimeClient({
    getTicket,
    wsBaseUrl: "ws://api/ws",
    onStateChange: (s) => states.push(s),
    backoffMs: () => 10,
    socketFactory: (url) => {
      const s = new FakeSocket(url);
      sockets.push(s);
      return s as unknown as WebSocket;
    },
  });
  return { client, sockets, states };
}

describe("RealtimeClient", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("conecta com ticket de uso único e distribui frames", async () => {
    const { client, sockets } = setup();
    const got: unknown[] = [];
    client.onFrame((f) => got.push(f));
    client.connect();
    await vi.runAllTicks();
    await Promise.resolve();
    expect(sockets[0]?.url).toBe("ws://api/ws?ticket=t1");
    sockets[0]!.open();
    sockets[0]!.receive({ type: "event", topic: "broadcast", data: { a: 1 } });
    expect(got).toEqual([{ type: "event", topic: "broadcast", data: { a: 1 } }]);
  });

  it("reassina os tópicos ao reconectar e conta referências", async () => {
    const { client, sockets } = setup();
    const off1 = client.subscribe("mercurio:room:1");
    const off2 = client.subscribe("mercurio:room:1");
    client.connect();
    await Promise.resolve();
    await Promise.resolve();
    sockets[0]!.open();
    expect(sockets[0]!.sent).toContain(JSON.stringify({ type: "subscribe", topic: "mercurio:room:1" }));
    off1();
    expect(sockets[0]!.sent.filter((s) => s.includes("unsubscribe"))).toHaveLength(0);
    off2();
    expect(sockets[0]!.sent).toContain(JSON.stringify({ type: "unsubscribe", topic: "mercurio:room:1" }));
  });

  it("para de tentar quando o ticket é negado com 401", async () => {
    const { client, states } = setup(() => Promise.reject(new ApiError(401, "UNAUTHORIZED", "x")));
    client.connect();
    await Promise.resolve();
    await Promise.resolve();
    expect(states.at(-1)).toBe("unauthorized");
  });
});

describe("helpers", () => {
  it("parseFrame ignora lixo", () => {
    expect(parseFrame("nao-json")).toBeNull();
    expect(parseFrame(JSON.stringify({ sem: "type" }))).toBeNull();
    expect(parseFrame(JSON.stringify({ type: "pong" }))).toEqual({ type: "pong" });
  });
  it("backoff exponencial com teto", () => {
    expect(defaultBackoff(0)).toBe(1000);
    expect(defaultBackoff(3)).toBe(8000);
    expect(defaultBackoff(20)).toBe(30000);
  });
});
