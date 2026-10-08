"use client";

import { Eye } from "lucide-react";
import { useState, type FormEvent } from "react";

import { UnidadeDonaSelect } from "@/components/iam/UnidadeDonaSelect";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Pagination } from "@/components/nexus/Pagination";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Select } from "@/components/ui/Select";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/Table";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ContactMessage, ContactSummary, OrgTree } from "@/lib/nexus/types";

const STATUS: Record<ContactSummary["status"], { label: string; tone: "info" | "warning" | "success" | "neutral" }> = {
  new: { label: "Nova", tone: "info" },
  in_progress: { label: "Em atendimento", tone: "warning" },
  answered: { label: "Respondida", tone: "success" },
  archived: { label: "Arquivada", tone: "neutral" },
};

function MessageDetail({ id, onChanged }: { id: string; onChanged: () => void }) {
  const { can } = useNexus();
  const msg = useApiQuery<ContactMessage>(`v1/contact/messages/${id}`);
  const { run, pending } = useAction();
  const m = msg.data;
  if (!m) return <DataState loading={msg.isLoading} error={msg.error} empty={false}>{null}</DataState>;

  async function triage(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const ok = await run(
      () => apiClient.patch(`v1/contact/messages/${id}`, {
        status: String(fd.get("status")),
        notes: String(fd.get("notes") ?? "").trim(),
        assigned_to: m!.assigned_to ?? null,
        unidade_id: String(fd.get("unidade_id") ?? "") || null,
      }),
      "Mensagem atualizada",
    );
    if (ok) {
      void msg.mutate();
      onChanged();
    }
  }

  return (
    <div className="flex flex-col gap-4 text-sm">
      <dl className="grid gap-2 sm:grid-cols-2">
        <div><dt className="text-xs text-muted">Protocolo</dt><dd className="font-mono">{m.protocol}</dd></div>
        <div><dt className="text-xs text-muted">Recebida</dt><dd>{fmtDateTime(m.created_at)}</dd></div>
        <div><dt className="text-xs text-muted">Nome</dt><dd>{m.name}</dd></div>
        <div><dt className="text-xs text-muted">E-mail</dt><dd><a href={`mailto:${m.email}?subject=${encodeURIComponent(`Re: ${m.subject} [${m.protocol}]`)}`} className="text-primary hover:underline">{m.email}</a></dd></div>
        {m.phone && <div><dt className="text-xs text-muted">Telefone</dt><dd>{m.phone}</dd></div>}
        <div><dt className="text-xs text-muted">Consentimento LGPD</dt><dd>{fmtDateTime(m.consent_at)}</dd></div>
      </dl>
      <div>
        <p className="text-xs text-muted">Assunto</p>
        <p className="font-medium">{m.subject}</p>
      </div>
      <p className="whitespace-pre-line rounded-lg bg-surface-hover p-3">{m.message}</p>
      {can("contact:manage") ? (
        <form onSubmit={triage} className="flex flex-col gap-3 border-t border-surface-border pt-4">
          <Select id="ct-status" name="status" label="Situação" defaultValue={m.status} options={Object.entries(STATUS).map(([value, s]) => ({ value, label: s.label }))} />
          {/* Encaminhar (ADR 013): a gestão com escopo só encaminha dentro da
              área dela e sempre pode devolver à caixa geral. */}
          <UnidadeDonaSelect id="ct-unidade" label="Setor (encaminhar para)" permission="contact:manage" optional defaultValue={m.unidade_id} placeholder="Caixa geral (triagem)" />
          <Textarea id="ct-notes" name="notes" label="Anotações internas" rows={3} maxLength={5000} defaultValue={m.notes} />
          <Button type="submit" className="self-end" loading={pending}>
            Salvar triagem
          </Button>
        </form>
      ) : (
        m.notes && <p className="text-xs text-muted">Anotações: {m.notes}</p>
      )}
    </div>
  );
}

export default function GestaoContatoPage() {
  const [status, setStatus] = useState("new");
  const [page, setPage] = useState(1);
  const [open, setOpen] = useState<ContactSummary | null>(null);
  const list = useApiPage<ContactSummary>(withQuery("v1/contact/messages", { status, page, page_size: 25 }));
  const tree = useApiQuery<OrgTree[]>("v1/iam/org-tree");
  const setor = (id?: string) => (id && (tree.data ?? []).flatMap((e) => e.unidades).find((u) => u.id === id)?.nome) || (id ? "—" : "Caixa geral");

  return (
    <div className="flex flex-col gap-6">
      <PageHeader eyebrow="Contato" title="Mensagens recebidas" description="Mensagens do formulário público, com protocolo e consentimento LGPD registrados." />
      <Select
        id="ct-filter"
        label="Situação"
        value={status}
        onChange={(e) => {
          setPage(1);
          setStatus(e.target.value);
        }}
        options={[{ value: "", label: "Todas" }, ...Object.entries(STATUS).map(([value, s]) => ({ value, label: s.label }))]}
        className="max-w-xs"
      />
      <DataState loading={list.isLoading} error={list.error} onRetry={() => void list.mutate()} empty={(list.data?.items ?? []).length === 0} emptyTitle="Nenhuma mensagem">
        <Table caption="Mensagens de contato">
          <TableHead>
            <TableRow>
              <TableHeaderCell>Protocolo</TableHeaderCell>
              <TableHeaderCell>Assunto</TableHeaderCell>
              <TableHeaderCell>Remetente</TableHeaderCell>
              <TableHeaderCell>Setor</TableHeaderCell>
              <TableHeaderCell>Situação</TableHeaderCell>
              <TableHeaderCell>Recebida</TableHeaderCell>
              <TableHeaderCell>
                <span className="sr-only">Abrir</span>
              </TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(list.data?.items ?? []).map((m) => (
              <TableRow key={m.id}>
                <TableCell className="font-mono text-xs">{m.protocol}</TableCell>
                <TableCell>
                  {m.subject}
                  <span className="block text-xs text-muted">{m.category}</span>
                </TableCell>
                <TableCell className="text-sm">{m.name}</TableCell>
                <TableCell className="text-sm">{setor(m.unidade_id)}</TableCell>
                <TableCell>
                  <Badge tone={STATUS[m.status].tone}>{STATUS[m.status].label}</Badge>
                </TableCell>
                <TableCell className="text-xs text-muted">{fmtDateTime(m.created_at)}</TableCell>
                <TableCell className="text-right">
                  <Button variant="ghost" size="sm" aria-label={`Abrir ${m.protocol}`} onClick={() => setOpen(m)}>
                    <Eye size={16} aria-hidden="true" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <Pagination meta={list.data?.meta} onPage={setPage} />
      </DataState>
      <Dialog open={open !== null} onClose={() => setOpen(null)} title={open ? `Mensagem ${open.protocol}` : ""} size="lg">
        {open && <MessageDetail key={open.id} id={open.id} onChanged={() => void list.mutate()} />}
      </Dialog>
    </div>
  );
}
