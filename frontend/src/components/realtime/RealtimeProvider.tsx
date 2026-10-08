"use client";

import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { apiClient } from "@/lib/api/client";
import { WS_PUBLIC_URL } from "@/lib/env";
import { RealtimeClient, type ConnectionState, type Frame } from "@/lib/websocket/client";

interface RealtimeContextValue {
  client: RealtimeClient | null;
  state: ConnectionState;
}

const RealtimeContext = createContext<RealtimeContextValue>({ client: null, state: "idle" });

/** Uma única conexão WebSocket para toda a área autenticada. */
export function RealtimeProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ConnectionState>("idle");
  const [client] = useState(
    () =>
      new RealtimeClient({
        wsBaseUrl: WS_PUBLIC_URL,
        getTicket: async () => (await apiClient.post<{ ticket: string }>("v1/ws/ticket")).data.ticket,
        onStateChange: setState,
      }),
  );

  useEffect(() => {
    client.connect();
    return () => client.disconnect();
  }, [client]);

  const value = useMemo(() => ({ client, state }), [client, state]);
  return <RealtimeContext.Provider value={value}>{children}</RealtimeContext.Provider>;
}

export function useRealtimeState(): ConnectionState {
  return useContext(RealtimeContext).state;
}

/** Recebe todo frame (filtre por type/topic no handler). */
export function useFrames(handler: (frame: Frame) => void): void {
  const { client } = useContext(RealtimeContext);
  const ref = useRef(handler);
  useEffect(() => {
    ref.current = handler;
  });
  useEffect(() => client?.onFrame((f) => ref.current(f)), [client]);
}

/** Assina um tópico enquanto o componente estiver montado (null = nada). */
export function useTopic(topic: string | null, handler: (frame: Frame) => void): void {
  const { client } = useContext(RealtimeContext);
  const ref = useRef(handler);
  useEffect(() => {
    ref.current = handler;
  });
  useEffect(() => {
    if (!client || !topic) return;
    const off = client.onFrame((f) => {
      if (f.topic === topic) ref.current(f);
    });
    const unsub = client.subscribe(topic);
    return () => {
      off();
      unsub();
    };
  }, [client, topic]);
}

/** Envia um frame (ex.: indicador de digitação do Mercúrio). */
export function useRealtimeSend(): (frame: Frame) => void {
  const { client } = useContext(RealtimeContext);
  return (frame) => client?.send(frame);
}
