"use client";

import { createContext, useContext, type ReactNode } from "react";

import type { ConnectionState } from "@/lib/websocket/client";

// O estado de conexão do WebSocket de notificações vive em DashboardShell
// (useState atualizado por NotificationCenter.onConnectionStateChange) —
// esta é a única fonte de verdade dele. Antes deste contexto, só a Topbar
// enxergava esse valor (recebido via prop direta); qualquer página
// renderizada como {children} (ex.: /monitoramento) não tinha como saber
// o estado real da conexão, então o card "Conexão WebSocket" do painel de
// Monitoramento mostrava "Ativa" fixo no código-fonte, sempre — mesmo
// desconectado (achado de auditoria). DashboardShell provê o valor atual
// em volta de {children}; useConnectionState() é como qualquer página lê.
const ConnectionStateContext = createContext<ConnectionState>("idle");

export function ConnectionStateProvider({
  value,
  children,
}: {
  value: ConnectionState;
  children: ReactNode;
}) {
  return (
    <ConnectionStateContext.Provider value={value}>{children}</ConnectionStateContext.Provider>
  );
}

export function useConnectionState(): ConnectionState {
  return useContext(ConnectionStateContext);
}
