"use client";

import { useEffect } from "react";
import { AlertOctagon, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/Button";

export default function ProtectedError({
  error,
  retry,
}: {
  error: Error & { digest?: string };
  /** Refaz a busca dos dados do servidor e re-renderiza (Next 16.3). */
  retry: () => void;
}) {
  useEffect(() => {
    // Log do erro no cliente/telemetria
    console.error("Erro capturado na rota protegida:", error);
  }, [error]);

  return (
    <div className="flex min-h-[400px] flex-col items-center justify-center gap-4 rounded-xl border border-surface-border bg-surface p-8 text-center shadow-sm">
      <div className="rounded-full bg-danger/10 p-4 text-danger">
        <AlertOctagon size={32} />
      </div>
      <div>
        <h2 className="text-lg font-semibold text-foreground">Ocorreu um erro inesperado</h2>
        <p className="text-sm text-muted mt-1 max-w-md">
          {error.message || "Não foi possível carregar as informações desta seção. Tente recarregar."}
        </p>
      </div>
      <Button onClick={() => retry()} variant="primary" size="sm">
        <RotateCcw className="mr-2 h-4 w-4" />
        Tentar Novamente
      </Button>
    </div>
  );
}
