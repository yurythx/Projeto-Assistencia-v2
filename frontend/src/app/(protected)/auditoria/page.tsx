"use client";

import { Download, Eye, Link2, ShieldAlert, ShieldCheck } from "lucide-react";
import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Pagination } from "@/components/nexus/Pagination";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/Table";
import { apiClient } from "@/lib/api/client";
import { useApiPage, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { AuditRecord, VerifyResult } from "@/lib/nexus/types";

function hasKeys(v: unknown): boolean {
  return typeof v === "object" && v !== null && Object.keys(v).length > 0;
}

function Json({ value }: { value: unknown }) {
  if (value === undefined || value === null) return <p className="text-xs text-muted">—</p>;
  return <pre className="max-h-64 overflow-auto rounded-md bg-surface-hover p-3 font-mono text-xs">{JSON.stringify(value, null, 2)}</pre>;
}

function RecordDetail({ r }: { r: AuditRecord }) {
  return (
    <div className="flex flex-col gap-4 text-sm">
      <dl className="grid gap-3 sm:grid-cols-2">
        <div><dt className="text-xs text-muted">Ação</dt><dd className="font-mono">{r.action}</dd></div>
        <div><dt className="text-xs text-muted">Recurso</dt><dd className="font-mono text-xs">{r.resource_type ?? "—"} {r.resource_id ?? ""}</dd></div>
        <div><dt className="text-xs text-muted">Ator</dt><dd>{r.actor_name ?? r.actor_subject ?? "sistema"}</dd></div>
        <div><dt className="text-xs text-muted">Perfis do ator</dt><dd>{r.actor_roles.join(", ") || "—"}</dd></div>
        <div><dt className="text-xs text-muted">IP</dt><dd className="font-mono text-xs">{r.ip_address ?? "—"}</dd></div>
        <div><dt className="text-xs text-muted">Data (UTC)</dt><dd>{new Date(r.timestamp_utc).toISOString()}</dd></div>
        <div className="sm:col-span-2"><dt className="text-xs text-muted">User-Agent</dt><dd className="break-all text-xs">{r.user_agent ?? "—"}</dd></div>
        <div className="sm:col-span-2"><dt className="text-xs text-muted">Correlação (X-Request-ID)</dt><dd className="font-mono text-xs">{r.correlation_id ?? "—"}</dd></div>
      </dl>
      <div className="grid gap-3 md:grid-cols-2">
        <div><h3 className="mb-1 text-xs font-semibold text-muted">Antes</h3><Json value={r.diff_before} /></div>
        <div><h3 className="mb-1 text-xs font-semibold text-muted">Depois</h3><Json value={r.diff_after} /></div>
      </div>
      <div><h3 className="mb-1 text-xs font-semibold text-muted">Contexto organizacional</h3><Json value={r.entity_context} /></div>
      {hasKeys(r.metadata) && <div><h3 className="mb-1 text-xs font-semibold text-muted">Metadados</h3><Json value={r.metadata} /></div>}
      <div className="rounded-md border border-surface-border p-3 font-mono text-[11px] text-muted">
        <p>posição #{r.chain_pos}</p>
        <p className="break-all">prev: {r.prev_hash}</p>
        <p className="break-all">hash: {r.hash}</p>
      </div>
    </div>
  );
}

export default function AuditoriaPage() {
  const { can, canGlobal } = useNexus();
  const { run, pending } = useAction();
  const [filters, setFilters] = useState({ action: "", resource_type: "", from: "", to: "" });
  const [applied, setApplied] = useState(filters);
  const [page, setPage] = useState(1);
  const [detail, setDetail] = useState<AuditRecord | null>(null);
  const [verify, setVerify] = useState<VerifyResult | null>(null);
  const [format, setFormat] = useState("csv");
  const list = useApiPage<AuditRecord>(withQuery("v1/audit/logs", { ...applied, page, page_size: 50 }));

  const exportHref = `/api/backend/${withQuery("v1/audit/export", { from: applied.from, to: applied.to, action: applied.action, format })}`;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Núcleo · Auditoria"
        title="Trilha de auditoria"
        description="Registro append-only de toda operação sensível, com proveniência completa e encadeamento de hash SHA-256 verificável."
        actions={
          <>
            {can("audit:verify") && (
              <Button
                variant="secondary"
                loading={pending}
                onClick={() => void run(() => apiClient.get<VerifyResult>("v1/audit/verify")).then((res) => res && setVerify(res.data))}
              >
                <Link2 size={16} aria-hidden="true" className="mr-1" /> Verificar integridade
              </Button>
            )}
            {can("audit:read") && (
              <span className="flex items-center gap-1">
                <label htmlFor="audit-format" className="sr-only">Formato de exportação</label>
                <select
                  id="audit-format"
                  value={format}
                  onChange={(e) => setFormat(e.target.value)}
                  className="h-10 rounded-lg border border-surface-border bg-surface px-2 text-sm"
                >
                  <option value="csv">CSV</option>
                  <option value="json">JSON</option>
                  <option value="xml">XML</option>
                </select>
                <a href={exportHref} className="inline-flex h-10 items-center gap-1 rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground hover:opacity-90">
                  <Download size={16} aria-hidden="true" /> Exportar (LAI)
                </a>
              </span>
            )}
          </>
        }
      />
      {can("audit:read") && !canGlobal("audit:read") && (
        <p role="note" className="rounded-lg border border-surface-border bg-surface-hover/60 px-4 py-3 text-sm text-muted">
          Sua consulta à auditoria vale na sua área: aparecem as ações de quem estava lotado nela quando agiu — inclusive na
          exportação. Ações do sistema e de contas globais ficam com a auditoria global.
        </p>
      )}

      {verify && (
        <div role="status" className={`flex items-start gap-3 rounded-lg border p-4 text-sm ${verify.valid ? "border-success/40 bg-success/5" : "border-danger/40 bg-danger/5"}`}>
          {verify.valid ? <ShieldCheck className="text-success" aria-hidden="true" /> : <ShieldAlert className="text-danger" aria-hidden="true" />}
          <div>
            <p className="font-semibold">{verify.valid ? "Cadeia íntegra" : "Cadeia violada"}</p>
            <p className="text-muted">
              {verify.checked} registros verificados em {fmtDateTime(verify.verified_at)}.
              {!verify.valid && ` Primeira divergência na posição #${verify.first_invalid_pos}: ${verify.reason}.`}
            </p>
          </div>
        </div>
      )}

      <form
        className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5 lg:items-end"
        onSubmit={(e) => {
          e.preventDefault();
          setPage(1);
          setApplied(filters);
        }}
      >
        <Input id="au-action" label="Ação" placeholder="ex.: iam.perfil.updated" value={filters.action} onChange={(e) => setFilters({ ...filters, action: e.target.value.trim() })} />
        <Input id="au-resource" label="Tipo de recurso" value={filters.resource_type} onChange={(e) => setFilters({ ...filters, resource_type: e.target.value.trim() })} />
        <Input id="au-from" type="date" label="De" value={filters.from} onChange={(e) => setFilters({ ...filters, from: e.target.value })} />
        <Input id="au-to" type="date" label="Até" value={filters.to} onChange={(e) => setFilters({ ...filters, to: e.target.value })} />
        <Button type="submit" variant="secondary">Filtrar</Button>
      </form>

      <DataState loading={list.isLoading} error={list.error} onRetry={() => void list.mutate()} empty={(list.data?.items ?? []).length === 0} emptyTitle="Nenhum registro no período" rows={6}>
        <Table caption="Registros de auditoria">
          <TableHead>
            <TableRow>
              <TableHeaderCell>#</TableHeaderCell>
              <TableHeaderCell>Quando</TableHeaderCell>
              <TableHeaderCell>Ator</TableHeaderCell>
              <TableHeaderCell>Ação</TableHeaderCell>
              <TableHeaderCell>Recurso</TableHeaderCell>
              <TableHeaderCell>
                <span className="sr-only">Detalhes</span>
              </TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(list.data?.items ?? []).map((r) => (
              <TableRow key={r.id}>
                <TableCell className="font-mono text-xs text-muted">{r.chain_pos}</TableCell>
                <TableCell className="whitespace-nowrap text-xs">{fmtDateTime(r.timestamp_utc)}</TableCell>
                <TableCell className="text-sm">
                  {r.actor_name ?? r.actor_subject ?? <Badge>sistema</Badge>}
                  {r.ip_address && <span className="block font-mono text-[11px] text-muted">{r.ip_address}</span>}
                </TableCell>
                <TableCell>
                  <code className="font-mono text-xs">{r.action}</code>
                </TableCell>
                <TableCell className="text-xs text-muted">{r.resource_type ?? "—"}</TableCell>
                <TableCell className="text-right">
                  <Button variant="ghost" size="sm" aria-label={`Detalhes do registro ${r.chain_pos}`} onClick={() => setDetail(r)}>
                    <Eye size={16} aria-hidden="true" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <Pagination meta={list.data?.meta} onPage={setPage} />
      </DataState>

      <Dialog open={detail !== null} onClose={() => setDetail(null)} size="xl" title={detail ? `Registro #${detail.chain_pos}` : ""}>
        {detail && <RecordDetail r={detail} />}
      </Dialog>
    </div>
  );
}
