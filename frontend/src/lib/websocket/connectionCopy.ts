import type { ConnectionState } from "./client";

// Fonte única do rótulo/cor de cada estado da conexão WebSocket de
// notificações — Topbar (o indicador discreto do cabeçalho) e o painel de
// Monitoramento (o card "Conexão WebSocket") mostram o MESMO estado, mas
// tinham cada um sua própria cópia deste mapa, já divergentes entre si
// (um dizia "Ao vivo" pro estado `open`, o outro "Ativa" — achado de
// revisão). Um único lugar evita essa deriva.
export const CONNECTION_LABEL: Record<ConnectionState, string> = {
  idle: "Conectando…",
  connecting: "Conectando…",
  open: "Ao vivo",
  closed: "Reconectando…",
  unauthorized: "Sessão expirada",
};

// Cores a partir dos tokens do design system (D-04), não da paleta crua
// do Tailwind. dotClass é pro indicador discreto (Topbar); textClass é
// pra quando o próprio texto do rótulo carrega a cor (painel de
// Monitoramento) — as duas telas usam o mesmo estado, mas cada uma
// precisa da variante certa pro seu próprio layout.
export const CONNECTION_TONE: Record<ConnectionState, { dotClass: string; textClass: string }> = {
  idle: { dotClass: "bg-status-unknown", textClass: "text-muted" },
  connecting: { dotClass: "bg-status-unknown", textClass: "text-muted" },
  open: { dotClass: "bg-status-online", textClass: "text-success" },
  closed: { dotClass: "bg-status-degraded", textClass: "text-warning" },
  unauthorized: { dotClass: "bg-status-offline", textClass: "text-danger" },
};
