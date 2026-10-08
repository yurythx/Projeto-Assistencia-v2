"use client";

import { Pencil, Plus, RotateCcw, Send, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";

import { useToast } from "@/components/notifications/ToastProvider";
import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { Pagination } from "@/components/nexus/Pagination";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/Table";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import type { EgressDelivery, EgressTarget } from "@/lib/nexus/types";

const KINDS = [
  { value: "webhook", label: "Webhook genérico" },
  { value: "n8n", label: "n8n" },
  { value: "zabbix", label: "Zabbix" },
  { value: "grafana", label: "Grafana" },
];

const STATUS_TONE: Record<EgressDelivery["status"], "success" | "warning" | "danger" | "neutral"> = {
  delivered: "success",
  pending: "neutral",
  failed: "warning",
  dead: "danger",
};

function TargetForm({ target, onDone }: { target?: EgressTarget; onDone: () => void }) {
  const { run, pending } = useAction();
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const secret = String(fd.get("secret") ?? "");
    const body = {
      name: String(fd.get("name") ?? "").trim(),
      kind: String(fd.get("kind") ?? "webhook"),
      url: String(fd.get("url") ?? "").trim(),
      // Segredo só é enviado quando preenchido (em branco = manter o atual).
      secret: secret ? secret : undefined,
      event_patterns: String(fd.get("event_patterns") ?? "")
        .split(/[\n,]/)
        .map((s) => s.trim())
        .filter(Boolean),
      active: fd.get("active") === "on",
    };
    const ok = await run(
      () => (target ? apiClient.put(`v1/egress/targets/${target.id}`, body) : apiClient.post("v1/egress/targets", body)),
      "Destino salvo",
    );
    if (ok) onDone();
  }
  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <Input id="eg-name" name="name" label="Nome *" required maxLength={120} defaultValue={target?.name} />
        <Select id="eg-kind" name="kind" label="Tipo" options={KINDS} defaultValue={target?.kind ?? "webhook"} />
      </div>
      <div>
        <Input id="eg-url" name="url" type="url" label="URL *" required maxLength={1000} defaultValue={target?.url} placeholder="https://" />
        <p className="mt-1 text-xs text-muted">
          Validada contra SSRF no envio: IPs privados (RFC 1918), loopback, link-local/metadata (169.254.169.254) e DNS interno são bloqueados.
        </p>
      </div>
      <Input
        id="eg-secret"
        name="secret"
        type="password"
        label={target?.has_secret ? "Segredo HMAC (deixe em branco para manter)" : "Segredo HMAC"}
        maxLength={500}
        autoComplete="new-password"
      />
      <Textarea
        id="eg-patterns"
        name="event_patterns"
        label="Eventos (um por linha; curinga *)"
        rows={3}
        defaultValue={(target?.event_patterns ?? []).join("\n")}
        placeholder={"blog.post.published\ntramite.*"}
      />
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" name="active" defaultChecked={target?.active ?? true} className="h-4 w-4 accent-primary" /> Ativo
      </label>
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Salvar destino
        </Button>
      </div>
    </form>
  );
}

export default function EgressPage() {
  const targets = useApiQuery<EgressTarget[]>("v1/egress/targets");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const deliveries = useApiPage<EgressDelivery>(withQuery("v1/egress/deliveries", { status, page, page_size: 20 }), { refreshInterval: 15_000 });
  const [editing, setEditing] = useState<EgressTarget | "new" | null>(null);
  const { run } = useAction();
  const { showToast } = useToast();

  async function test(t: EgressTarget) {
    const res = await run(() => apiClient.post<{ ok: boolean; status_code: number; error?: string; took_ms: number }>(`v1/egress/targets/${t.id}/test`));
    if (!res) return;
    const r = res.data;
    showToast(
      r.ok
        ? { title: "Destino respondeu", description: `HTTP ${r.status_code} em ${r.took_ms} ms`, tone: "success" }
        : { title: "Falha no teste", description: r.error || `HTTP ${r.status_code}`, tone: "danger" },
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-3">
          <CardTitle as="h2">Destinos de saída</CardTitle>
          <Button size="sm" onClick={() => setEditing("new")}>
            <Plus size={14} aria-hidden="true" className="mr-1" /> Novo destino
          </Button>
        </CardHeader>
        <CardContent className="pb-5 pt-3">
          <DataState
            loading={targets.isLoading}
            error={targets.error}
            empty={(targets.data ?? []).length === 0}
            emptyTitle="Nenhum destino configurado"
            emptyDescription="Eventos do barramento (outbox) podem ser enviados a n8n, Zabbix, Grafana ou qualquer webhook HTTPS."
          >
            <ul className="flex flex-col divide-y divide-surface-border">
              {(targets.data ?? []).map((t) => (
                <li key={t.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                  <div className="min-w-0">
                    <p className="font-medium">
                      {t.name} <Badge className="ml-1">{t.kind}</Badge> {!t.active && <Badge tone="warning" className="ml-1">Inativo</Badge>}
                    </p>
                    <p className="truncate font-mono text-xs text-muted">{t.url}</p>
                    <p className="text-xs text-muted">{t.event_patterns.length ? t.event_patterns.join(", ") : "todos os eventos"}</p>
                  </div>
                  <span className="flex items-center gap-1">
                    <Button variant="secondary" size="sm" onClick={() => void test(t)}>
                      <Send size={14} aria-hidden="true" className="mr-1" /> Testar
                    </Button>
                    <Button variant="ghost" size="sm" aria-label={`Editar ${t.name}`} onClick={() => setEditing(t)}>
                      <Pencil size={14} aria-hidden="true" />
                    </Button>
                    <ConfirmButton
                      aria-label={`Excluir ${t.name}`}
                      title={`Excluir o destino "${t.name}"?`}
                      confirmLabel="Excluir"
                      onConfirm={() => run(() => apiClient.delete(`v1/egress/targets/${t.id}`), "Destino excluído").then(() => targets.mutate())}
                    >
                      <Trash2 size={14} aria-hidden="true" />
                    </ConfirmButton>
                  </span>
                </li>
              ))}
            </ul>
          </DataState>
        </CardContent>
      </Card>

      <section aria-labelledby="entregas" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <h2 id="entregas" className="text-lg font-semibold">
            Entregas
          </h2>
          <Select
            id="eg-status"
            label="Situação"
            value={status}
            onChange={(e) => {
              setPage(1);
              setStatus(e.target.value);
            }}
            options={[
              { value: "", label: "Todas" },
              { value: "pending", label: "Pendentes" },
              { value: "delivered", label: "Entregues" },
              { value: "failed", label: "Com falha (retentando)" },
              { value: "dead", label: "Esgotadas (DLQ)" },
            ]}
          />
        </div>
        <DataState loading={deliveries.isLoading} error={deliveries.error} empty={(deliveries.data?.items ?? []).length === 0} emptyTitle="Nenhuma entrega">
          <Table caption="Entregas de eventos para destinos externos">
            <TableHead>
              <TableRow>
                <TableHeaderCell>Evento</TableHeaderCell>
                <TableHeaderCell>Destino</TableHeaderCell>
                <TableHeaderCell>Situação</TableHeaderCell>
                <TableHeaderCell>Tentativas</TableHeaderCell>
                <TableHeaderCell>Criada</TableHeaderCell>
                <TableHeaderCell>
                  <span className="sr-only">Ações</span>
                </TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(deliveries.data?.items ?? []).map((d) => (
                <TableRow key={d.id}>
                  <TableCell>
                    <code className="font-mono text-xs">{d.event_type}</code>
                  </TableCell>
                  <TableCell className="text-sm">{d.target_name ?? "—"}</TableCell>
                  <TableCell>
                    <Badge tone={STATUS_TONE[d.status]}>{d.status}</Badge>
                    {d.last_error && <span className="block max-w-xs truncate text-xs text-danger" title={d.last_error}>{d.last_error}</span>}
                  </TableCell>
                  <TableCell className="text-sm">
                    {d.attempts}
                    {d.last_status_code ? <span className="text-xs text-muted"> · HTTP {d.last_status_code}</span> : null}
                  </TableCell>
                  <TableCell className="text-xs text-muted">{fmtDateTime(d.created_at)}</TableCell>
                  <TableCell className="text-right">
                    {(d.status === "dead" || d.status === "failed") && (
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label="Reenviar"
                        onClick={() => void run(() => apiClient.post(`v1/egress/deliveries/${d.id}/redeliver`), "Reenvio agendado").then(() => deliveries.mutate())}
                      >
                        <RotateCcw size={14} aria-hidden="true" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Pagination meta={deliveries.data?.meta} onPage={setPage} />
        </DataState>
      </section>

      <Dialog open={editing !== null} onClose={() => setEditing(null)} size="lg" title={editing === "new" ? "Novo destino" : "Editar destino"}>
        {editing && (
          <TargetForm
            key={editing === "new" ? "new" : editing.id}
            target={editing === "new" ? undefined : editing}
            onDone={() => {
              setEditing(null);
              void targets.mutate();
            }}
          />
        )}
      </Dialog>
    </div>
  );
}
