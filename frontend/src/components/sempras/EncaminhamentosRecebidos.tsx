"use client";

import { Check, X } from "lucide-react";
import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { type Encaminhamento } from "@/lib/sempras/types";

/** Fila de encaminhamentos recebidos de outras unidades (ex.: CRAS →
 * CREAS) — quem atende aqui decide aceitar ou recusar (ADR 030, Fase 8). */
export function EncaminhamentosRecebidos({ unidadeId }: { unidadeId: string }) {
  const lista = useApiQuery<Encaminhamento[]>(`v1/sempras/unidades/${unidadeId}/encaminhamentos`);
  const { run, pending } = useAction();
  const [recusando, setRecusando] = useState<Encaminhamento | null>(null);
  const [observacao, setObservacao] = useState("");

  async function aceitar(enc: Encaminhamento) {
    const ok = await run(() => apiClient.post(`v1/sempras/encaminhamentos/${enc.id}/decidir`, { aceitar: true, observacao: "" }), "Encaminhamento aceito");
    if (ok) void lista.mutate();
  }

  async function confirmarRecusa() {
    if (!recusando) return;
    const ok = await run(
      () => apiClient.post(`v1/sempras/encaminhamentos/${recusando.id}/decidir`, { aceitar: false, observacao: observacao.trim() }),
      "Encaminhamento recusado",
    );
    if (ok) {
      setRecusando(null);
      setObservacao("");
      void lista.mutate();
    }
  }

  const itens = lista.data ?? [];
  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Encaminhamentos recebidos</CardTitle>
      </CardHeader>
      <CardContent className="pb-5">
        <DataState loading={lista.isLoading} error={lista.error} onRetry={() => void lista.mutate()}
          empty={itens.length === 0} emptyTitle="Nenhum encaminhamento pendente">
          <ul aria-label="Encaminhamentos recebidos" className="mt-3 flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
            {itens.map((enc) => (
              <li key={enc.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                <div>
                  <p className="font-medium">{enc.motivo}</p>
                  <p className="text-xs text-muted">{fmtDateTime(enc.created_at)}</p>
                </div>
                <span className="flex items-center gap-2">
                  <Badge tone="warning">Pendente</Badge>
                  <Button size="sm" loading={pending} onClick={() => void aceitar(enc)}>
                    <Check size={14} aria-hidden="true" className="mr-1" /> Aceitar
                  </Button>
                  <Button size="sm" variant="danger" onClick={() => setRecusando(enc)}>
                    <X size={14} aria-hidden="true" className="mr-1" /> Recusar
                  </Button>
                </span>
              </li>
            ))}
          </ul>
        </DataState>
      </CardContent>

      <Dialog open={recusando !== null} onClose={() => setRecusando(null)} title="Recusar encaminhamento">
        <div className="flex flex-col gap-4">
          <Textarea id="encaminhamento-recusa" label="Observação *" rows={3} maxLength={1000}
            value={observacao} onChange={(e) => setObservacao(e.target.value)} required />
          <div>
            <Button variant="danger" loading={pending} disabled={observacao.trim().length < 3} onClick={() => void confirmarRecusa()}>
              Confirmar
            </Button>
          </div>
        </div>
      </Dialog>
    </Card>
  );
}
