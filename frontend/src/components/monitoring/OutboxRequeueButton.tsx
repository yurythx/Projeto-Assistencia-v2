"use client";

import { RotateCcw } from "lucide-react";

import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { useAction } from "@/components/nexus/useAction";
import { apiClient } from "@/lib/api/client";

/** Devolve à fila os eventos do outbox que esgotaram as tentativas
 * (POST /api/v1/monitoring/outbox/requeue — exige monitoring:manage, que a
 * API revalida). Pensado para depois de uma queda prolongada do RabbitMQ. */
export function OutboxRequeueButton({ failed, onDone }: { failed: number; onDone: () => void }) {
  const { run } = useAction();
  return (
    <ConfirmButton
      size="sm"
      variant="secondary"
      title="Reprocessar eventos com falha?"
      description={`${failed} evento(s) voltam à fila do outbox e serão publicados assim que o RabbitMQ aceitar. Os consumidores ignoram duplicados.`}
      confirmLabel="Reprocessar"
      onConfirm={async () => {
        const res = await run(
          () => apiClient.post<{ requeued: number }>("v1/monitoring/outbox/requeue", {}),
          "Eventos devolvidos à fila",
        );
        if (res) onDone();
      }}
    >
      <RotateCcw size={14} aria-hidden="true" className="mr-1" /> Reprocessar
    </ConfirmButton>
  );
}
