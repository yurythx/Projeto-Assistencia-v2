"use client";

import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/Table";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, STATUS_PEDIDO, TIPOS_PEDIDO, type PedidoTitular } from "@/lib/sempras/types";

/** Decidir um pedido de anonimização pendente: aprovar zera os campos
 * identificadores do cadastro; negar exige justificativa. */
function Decisao({ pedido, onDecidido }: { pedido: PedidoTitular; onDecidido: () => void }) {
  const { run, pending } = useAction();
  const [justificativa, setJustificativa] = useState("");

  async function decidir(aprovar: boolean) {
    const ok = await run(
      () => apiClient.post(`v1/sempras/pedidos/${pedido.id}/decidir`, { aprovar, justificativa: justificativa.trim() }),
      aprovar ? "Anonimização aprovada" : "Pedido negado",
    );
    if (ok) onDecidido();
  }

  return (
    <div className="flex flex-col gap-4 text-sm">
      <dl className="grid gap-2 sm:grid-cols-2">
        <div><dt className="text-xs text-muted">Munícipe</dt><dd className="font-medium">{pedido.cidadao_nome || "—"}</dd></div>
        <div><dt className="text-xs text-muted">Solicitado por</dt><dd>{pedido.solicitado_por_nome || "—"}</dd></div>
        <div><dt className="text-xs text-muted">Em</dt><dd>{fmtDateTime(pedido.created_at)}</dd></div>
      </dl>
      <div>
        <p className="text-xs text-muted">Justificativa do pedido</p>
        <p className="whitespace-pre-wrap rounded-lg bg-surface-hover p-3">{pedido.justificativa_solicitacao}</p>
      </div>
      <Textarea id="decisao-justificativa" label="Justificativa da decisão (obrigatória para negar)" rows={3} maxLength={1000}
        value={justificativa} onChange={(e) => setJustificativa(e.target.value)} />
      <div className="flex flex-wrap gap-2">
        <Button loading={pending} onClick={() => void decidir(true)}>
          Aprovar anonimização
        </Button>
        <Button variant="danger" loading={pending} disabled={justificativa.trim().length < 3} onClick={() => void decidir(false)}>
          Negar
        </Button>
      </div>
    </div>
  );
}

/** Fila de decisão da gestão (sempras:gestao, cobertura total — ADR 030
 * §8): hoje só a anonimização chega pendente, acesso e correção se
 * resolvem na hora, com o titular presente. */
export default function PedidosPage() {
  const { can } = useNexus();
  const pedidos = useApiQuery<PedidoTitular[]>("v1/sempras/pedidos");
  const [aberto, setAberto] = useState<PedidoTitular | null>(null);

  if (!can(P.gestao)) {
    return (
      <p className="text-sm text-muted">
        A decisão dos pedidos do titular é da gestão do SEMPRAS (permissão {P.gestao}, cobertura total).
      </p>
    );
  }

  const lista = pedidos.data ?? [];
  return (
    <div className="flex flex-col gap-6">
      <PageHeader eyebrow="Assistência Social" title="SEMPRAS — Pedidos do titular"
        description="Pedidos de anonimização pendentes, registrados por quem atendeu o munícipe presencialmente (LGPD art. 18)." />
      <DataState loading={pedidos.isLoading} error={pedidos.error} onRetry={() => void pedidos.mutate()}
        empty={lista.length === 0} emptyTitle="Nenhum pedido pendente">
        <Table caption="Pedidos de anonimização pendentes">
          <TableHead>
            <TableRow>
              <TableHeaderCell>Munícipe</TableHeaderCell>
              <TableHeaderCell>Tipo</TableHeaderCell>
              <TableHeaderCell>Status</TableHeaderCell>
              <TableHeaderCell>Solicitado por</TableHeaderCell>
              <TableHeaderCell>Em</TableHeaderCell>
              <TableHeaderCell>
                <span className="sr-only">Decidir</span>
              </TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {lista.map((p) => (
              <TableRow key={p.id}>
                <TableCell className="font-medium">{p.cidadao_nome || "—"}</TableCell>
                <TableCell>{TIPOS_PEDIDO[p.tipo]}</TableCell>
                <TableCell>
                  <Badge tone={STATUS_PEDIDO[p.status].tone}>{STATUS_PEDIDO[p.status].label}</Badge>
                </TableCell>
                <TableCell className="text-sm">{p.solicitado_por_nome || "—"}</TableCell>
                <TableCell className="text-xs text-muted">{fmtDateTime(p.created_at)}</TableCell>
                <TableCell className="text-right">
                  <Button variant="secondary" size="sm" onClick={() => setAberto(p)}>
                    Decidir
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </DataState>
      <Dialog open={aberto !== null} onClose={() => setAberto(null)}
        title={aberto ? `Decidir anonimização — ${aberto.cidadao_nome}` : "Decidir pedido"} size="lg">
        {aberto && (
          <Decisao
            key={aberto.id}
            pedido={aberto}
            onDecidido={() => {
              setAberto(null);
              void pedidos.mutate();
            }}
          />
        )}
      </Dialog>
    </div>
  );
}
