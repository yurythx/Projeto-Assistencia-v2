"use client";

import type { ReactNode } from "react";

import { EmptyState } from "@/components/ui/EmptyState";
import { ErrorState } from "@/components/ui/ErrorState";
import { Skeleton } from "@/components/ui/Skeleton";
import { ApiError } from "@/lib/api/client";

/** Estados padrão de uma consulta SWR: carregando / erro / vazio / dados. */
export function DataState({
  loading,
  error,
  empty,
  emptyTitle = "Nada por aqui ainda",
  emptyDescription,
  onRetry,
  rows = 3,
  children,
}: {
  loading: boolean;
  error: unknown;
  empty: boolean;
  emptyTitle?: string;
  emptyDescription?: string;
  onRetry?: () => void;
  rows?: number;
  children: ReactNode;
}) {
  if (error) {
    const message = error instanceof ApiError ? error.message : "Não foi possível carregar os dados.";
    return <ErrorState message={message} onRetry={onRetry} />;
  }
  if (loading) {
    return (
      <div role="status" aria-live="polite" className="flex flex-col gap-3">
        <span className="sr-only">Carregando…</span>
        {Array.from({ length: rows }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    );
  }
  if (empty) return <EmptyState title={emptyTitle} description={emptyDescription} />;
  return <>{children}</>;
}
