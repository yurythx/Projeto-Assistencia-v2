"use client";

import { useCallback } from "react";

import {
  useNotificationHistory,
  type NotificacaoServidor,
} from "@/components/notifications/NotificationHistoryProvider";
import { useToast } from "@/components/notifications/ToastProvider";
import { useFrames } from "@/components/realtime/RealtimeProvider";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { eventEnvelopeSchema } from "@/lib/validation/schemas";
import type { Frame } from "@/lib/websocket/client";

type Tone = "success" | "danger" | "info";

/** Texto do toast para cada evento de difusão geral (a fila
 * nexus.notification.websocket só recebe eventos seguros para todos). */
export function describeEvent(
  type: string,
  payload: unknown,
): { title: string; description?: string; tone: Tone } | null {
  const p = (payload ?? {}) as Record<string, unknown>;
  const s = (k: string) => (typeof p[k] === "string" ? (p[k] as string) : undefined);
  switch (type) {
    case "blog.post.published":
      return {
        title: s("kind") === "comunicado" ? "Novo comunicado" : "Nova publicação",
        description: s("title"),
        tone: "info",
      };
    case "catalog.service.published":
      return { title: "Serviço publicado no catálogo", description: s("title"), tone: "success" };
    case "calendar.event.created":
      return { title: "Novo evento na agenda", description: s("title"), tone: "info" };
    case "notification.created":
      return { title: s("title") ?? "Nova notificação", description: s("message"), tone: "info" };
    default:
      return null;
  }
}

/** Montado uma vez na área autenticada: traduz frames do WebSocket em
 * toasts e histórico; reage à desativação de módulos em tempo real. */
export function NotificationCenter() {
  const { showToast } = useToast();
  const { push, pushServer } = useNotificationHistory();
  const { refreshModules } = useNexus();

  const handle = useCallback(
    (frame: Frame) => {
      if (frame.type === "module.disabled") {
        refreshModules();
        const n = { title: "Um módulo foi desativado pelo administrador", tone: "danger" as const };
        showToast(n);
        push(n);
        return;
      }
      if (frame.type === "mercurio.unread") {
        const n = { title: "Nova mensagem direta no Mercúrio", tone: "info" as const };
        showToast(n);
        push(n);
        return;
      }
      // Aviso persistido (ADR 027): vai para o sino sem duplicar e vira toast.
      if (frame.type === "notificacao.nova") {
        const n = frame.data as NotificacaoServidor | undefined;
        if (!n?.id || !n.titulo) return;
        pushServer(n);
        showToast({ title: n.titulo, description: n.mensagem || undefined, tone: "info" });
        return;
      }
      if (frame.type !== "event") return;
      const parsed = eventEnvelopeSchema.safeParse(frame.data);
      if (!parsed.success) return;
      const n = describeEvent(parsed.data.type, parsed.data.payload);
      if (!n) return;
      showToast(n);
      push(n);
    },
    [showToast, push, pushServer, refreshModules],
  );

  useFrames(handle);
  return null;
}
