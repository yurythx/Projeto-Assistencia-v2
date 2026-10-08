"use client";

import { useCallback, useState } from "react";

import { useToast } from "@/components/notifications/ToastProvider";
import { ApiError } from "@/lib/api/client";

/** Executa uma mutação com estado de carregamento e toast de sucesso/erro
 * (a mensagem de erro vem do backend — já segura para o usuário). */
export function useAction() {
  const { showToast } = useToast();
  const [pending, setPending] = useState(false);

  const run = useCallback(
    async <T,>(fn: () => Promise<T>, success?: string): Promise<T | undefined> => {
      setPending(true);
      try {
        const out = await fn();
        if (success) showToast({ title: success, tone: "success" });
        return out;
      } catch (err) {
        const message = err instanceof ApiError ? err.message : "Não foi possível concluir a operação.";
        showToast({ title: "Operação não concluída", description: message, tone: "danger" });
        return undefined;
      } finally {
        setPending(false);
      }
    },
    [showToast],
  );

  return { run, pending };
}

/** Formata data/hora no padrão brasileiro. */
export function fmtDateTime(iso?: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}

export function fmtDate(iso?: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("pt-BR");
}

export function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
