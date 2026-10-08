"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Archive, ExternalLink, FolderKanban, Plus, Search } from "lucide-react";
import { useState, type FormEvent } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Pagination } from "@/components/nexus/Pagination";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { PROCESSO_STATUS, SIGILO } from "@/components/tramite/labels";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClass } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/ui/Table";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { OrgTree, Processo, TramiteTipo, Workflow } from "@/lib/nexus/types";

function useUnidadeOptions() {
  const tree = useApiQuery<OrgTree[]>("v1/iam/org-tree");
  return (tree.data ?? [])
    .filter((e) => e.ativo)
    .flatMap((e) =>
      e.unidades
        .filter((u) => u.ativo)
        .map((u) => ({ value: u.id, label: `${u.sigla ? `${u.sigla} — ` : ""}${u.nome}` })),
    );
}

function AbrirProcesso({ onDone }: { onDone: (p: Processo) => void }) {
  const { me } = useNexus();
  const tipos = useApiQuery<TramiteTipo[]>("v1/tramite/tipos");
  const unidades = useUnidadeOptions();
  const { run, pending } = useAction();
  const minhas = new Set((me?.scopes ?? []).map((s) => s.unidade_id).filter(Boolean));
  // Procedimento do Atlas (opcional — ADR 026): o processo herda a série da TTDD.
  const atlas = useApiPage<Workflow>(withQuery("v1/atlas/workflows", { page_size: 100 }));
  const procedimentos = atlas.data?.items ?? [];
  const [procedimento, setProcedimento] = useState("");
  const escolhido = procedimentos.find((w) => w.id === procedimento);
  const defaultUnidade = unidades.find((u) => minhas.has(u.value))?.value;

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const res = await run(
      () =>
        apiClient.post<Processo>("v1/tramite/processos", {
          tipo_id: String(fd.get("tipo_id") ?? ""),
          assunto: String(fd.get("assunto") ?? "").trim(),
          interessado: String(fd.get("interessado") ?? "").trim(),
          descricao: String(fd.get("descricao") ?? "").trim(),
          sigilo: String(fd.get("sigilo") ?? "publico"),
          unidade_origem_id: String(fd.get("unidade_origem_id") ?? ""),
          atlas_procedimento_id: escolhido?.id ?? null,
          codigo_ttdd: escolhido?.codigo_ttdd ?? "",
        }),
      "Processo aberto",
    );
    if (res) onDone(res.data);
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <div className="flex items-center justify-between rounded-lg border border-primary/20 bg-primary/5 p-3 text-xs text-muted-foreground">
        <span>Dúvidas sobre o rito processual, temporalidade ou peças obrigatórias?</span>
        <Link
          href="/atlas"
          target="_blank"
          className="ml-2 inline-flex items-center gap-1 font-medium text-primary hover:underline"
        >
          Consultar Atlas <ExternalLink size={12} aria-hidden="true" />
        </Link>
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <Select
          id="pr-tipo"
          name="tipo_id"
          label="Tipo *"
          required
          placeholder="Selecione…"
          options={(tipos.data ?? []).map((t) => ({ value: t.id, label: t.nome }))}
        />
        <Select
          id="pr-sigilo"
          name="sigilo"
          label="Nível de acesso *"
          defaultValue="publico"
          options={[
            { value: "publico", label: "Público" },
            { value: "restrito", label: "Restrito (unidades envolvidas)" },
            { value: "sigiloso", label: "Sigiloso (acesso nominal)" },
          ]}
        />
      </div>
      {procedimentos.length > 0 && (
        <div className="flex flex-col gap-1">
          <Select
            id="pr-atlas"
            label="Procedimento do Atlas"
            value={procedimento}
            onChange={(e) => setProcedimento(e.target.value)}
            options={[
              { value: "", label: "Nenhum" },
              ...procedimentos.map((w) => ({
                value: w.id,
                label: `${w.codigo_processual} — ${w.titulo}`,
              })),
            ]}
          />
          <p className="text-xs text-muted">
            {escolhido
              ? `O processo segue o fluxo do Atlas e é guardado pela série ${escolhido.codigo_ttdd} da TTDD.`
              : "Opcional: ligue o processo a um fluxo homologado para ver as peças exigidas, os modelos e a guarda."}
          </p>
        </div>
      )}
      <Input id="pr-assunto" name="assunto" label="Assunto *" required maxLength={300} />
      <Input id="pr-interessado" name="interessado" label="Interessado" maxLength={300} />
      {/* key: o select é não controlado e a árvore/identidade chegam depois do
          primeiro render — remonta para a unidade do usuário vir marcada. */}
      <Select
        key={defaultUnidade ?? "sem-unidade"}
        id="pr-origem"
        name="unidade_origem_id"
        label="Unidade de origem *"
        required
        placeholder="Selecione…"
        defaultValue={defaultUnidade}
        options={unidades}
      />
      <Textarea id="pr-desc" name="descricao" label="Descrição" rows={4} maxLength={20000} />
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Abrir processo
        </Button>
      </div>
    </form>
  );
}

export default function TramitePage() {
  const { can } = useNexus();
  const router = useRouter();
  const [q, setQ] = useState("");
  const [filters, setFilters] = useState({ q: "", status: "", minha_caixa: true });
  const [page, setPage] = useState(1);
  const [opening, setOpening] = useState(false);
  const list = useApiPage<Processo>(
    withQuery("v1/tramite/processos", {
      q: filters.q,
      status: filters.status,
      minha_caixa: filters.minha_caixa || undefined,
      page,
      page_size: 25,
    }),
  );

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Trâmite"
        title="Processos administrativos"
        description="Processos numerados com controle de sigilo, despachos entre unidades e documentos assinados pelo Signum."
        actions={
          <div className="flex flex-wrap gap-2">
            <Link href="/tramite/guarda" className={buttonClass("secondary")}>
              <Archive size={16} aria-hidden="true" /> Guarda documental
            </Link>
            {can("tramite:create") && (
              <Button onClick={() => setOpening(true)}>
                <Plus size={16} aria-hidden="true" className="mr-1" /> Abrir processo
              </Button>
            )}
          </div>
        }
      />
      <form
        role="search"
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          setPage(1);
          setFilters({ ...filters, q: q.trim() });
        }}
      >
        <Input
          id="tr-q"
          label="Número, assunto ou interessado"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <Select
          id="tr-status"
          label="Situação"
          value={filters.status}
          onChange={(e) => {
            setPage(1);
            setFilters({ ...filters, status: e.target.value });
          }}
          options={[
            { value: "", label: "Todas" },
            ...Object.entries(PROCESSO_STATUS).map(([value, s]) => ({ value, label: s.label })),
          ]}
        />
        <label className="flex items-center gap-2 pb-2 text-sm">
          <input
            type="checkbox"
            checked={filters.minha_caixa}
            onChange={(e) => {
              setPage(1);
              setFilters({ ...filters, minha_caixa: e.target.checked });
            }}
            className="h-4 w-4 accent-primary"
          />
          Só na minha unidade
        </label>
        <Button type="submit" variant="secondary">
          <Search size={14} aria-hidden="true" className="mr-1" /> Buscar
        </Button>
      </form>

      <DataState
        loading={list.isLoading}
        error={list.error}
        onRetry={() => void list.mutate()}
        empty={(list.data?.items ?? []).length === 0}
        emptyTitle="Nenhum processo"
      >
        <Table caption="Processos">
          <TableHead>
            <TableRow>
              <TableHeaderCell>Número</TableHeaderCell>
              <TableHeaderCell>Assunto</TableHeaderCell>
              <TableHeaderCell>Unidade atual</TableHeaderCell>
              <TableHeaderCell>Situação</TableHeaderCell>
              <TableHeaderCell>Atualizado</TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(list.data?.items ?? []).map((p) => (
              <TableRow key={p.id}>
                <TableCell>
                  <Link
                    href={`/tramite/${p.id}`}
                    className="inline-flex items-center gap-1 font-mono text-sm font-medium text-primary hover:underline"
                  >
                    <FolderKanban size={14} aria-hidden="true" /> {p.numero}
                  </Link>
                </TableCell>
                <TableCell>
                  <span className="font-medium">{p.assunto}</span>
                  <span className="block text-xs text-muted">
                    {p.tipo}
                    {p.interessado && ` · ${p.interessado}`}
                  </span>
                </TableCell>
                <TableCell className="text-sm">{p.unidade_atual}</TableCell>
                <TableCell>
                  <span className="flex flex-wrap gap-1">
                    <Badge tone={PROCESSO_STATUS[p.status].tone}>
                      {PROCESSO_STATUS[p.status].label}
                    </Badge>
                    {p.sigilo !== "publico" && (
                      <Badge tone={SIGILO[p.sigilo].tone}>{SIGILO[p.sigilo].label}</Badge>
                    )}
                  </span>
                </TableCell>
                <TableCell className="text-xs text-muted">{fmtDateTime(p.updated_at)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <Pagination meta={list.data?.meta} onPage={setPage} />
      </DataState>

      <Dialog open={opening} onClose={() => setOpening(false)} title="Abrir processo" size="lg">
        {opening && <AbrirProcesso onDone={(p) => router.push(`/tramite/${p.id}`)} />}
      </Dialog>
    </div>
  );
}
