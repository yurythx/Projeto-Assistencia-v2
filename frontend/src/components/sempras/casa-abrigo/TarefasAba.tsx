"use client";

import { useState, type DragEvent, type FormEvent } from "react";
import { ChevronLeft, ChevronRight, Pencil, Plus, Trash2 } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import {
  COLUNAS_KANBAN,
  PRIORIDADES_TAREFA,
  dataISO,
  diaDe,
  fmtDia,
  hojeISO,
  type PrioridadeTarefa,
  type StatusTarefa,
  type TarefaCasaAbrigo,
} from "@/lib/sempras/types";

const COLUNAS = Object.keys(COLUNAS_KANBAN) as StatusTarefa[];

interface Form {
  titulo: string;
  descricao: string;
  status: StatusTarefa;
  prioridade: PrioridadeTarefa;
  responsavel: string;
  sprint: string;
  prazo: string;
}

const novoForm = (): Form => ({ titulo: "", descricao: "", status: "backlog", prioridade: "media", responsavel: "", sprint: "", prazo: "" });

const corpo = (f: Form, ordem: number) => ({ ...f, titulo: f.titulo.trim(), prazo: f.prazo ? dataISO(f.prazo) : null, ordem });

/**
 * Quadro de tarefas da equipe (o SCRUM/Kanban do sistema da unidade):
 * 5 colunas, prioridade, sprint e prazo, arrastar e soltar — e botões de
 * mover para quem usa o teclado. O monitor avisa 3 dias antes do prazo.
 */
export function TarefasAba({ unidadeId }: { unidadeId: string }) {
  const lista = useApiQuery<TarefaCasaAbrigo[]>(`v1/sempras/unidades/${unidadeId}/casa-abrigo/tarefas`);
  const [filtroSprint, setFiltroSprint] = useState("");
  const [filtroPrioridade, setFiltroPrioridade] = useState("");
  const [aberta, setAberta] = useState<TarefaCasaAbrigo | "nova" | null>(null);
  const [form, setForm] = useState<Form>(novoForm());
  const { run, pending } = useAction();
  const recarregar = () => void lista.mutate();
  const tarefas = (lista.data ?? []).filter(
    (t) => t.sprint.toLowerCase().includes(filtroSprint.trim().toLowerCase()) && (!filtroPrioridade || t.prioridade === filtroPrioridade),
  );

  function abrir(t: TarefaCasaAbrigo | "nova") {
    setAberta(t);
    setForm(t === "nova" ? novoForm() : { titulo: t.titulo, descricao: t.descricao, status: t.status, prioridade: t.prioridade,
      responsavel: t.responsavel, sprint: t.sprint, prazo: diaDe(t.prazo) });
  }

  async function salvar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(() =>
      aberta === "nova"
        ? apiClient.post(`v1/sempras/unidades/${unidadeId}/casa-abrigo/tarefas`, corpo(form, 0))
        : apiClient.put(`v1/sempras/casa-abrigo-tarefas/${(aberta as TarefaCasaAbrigo).id}`, corpo(form, (aberta as TarefaCasaAbrigo).ordem)),
    aberta === "nova" ? "Tarefa criada" : "Tarefa atualizada");
    if (ok) {
      setAberta(null);
      recarregar();
    }
  }

  async function mover(t: TarefaCasaAbrigo, status: StatusTarefa) {
    if (t.status === status) return;
    const ok = await run(() => apiClient.put(`v1/sempras/casa-abrigo-tarefas/${t.id}`, {
      titulo: t.titulo, descricao: t.descricao, status, prioridade: t.prioridade, responsavel: t.responsavel, sprint: t.sprint,
      prazo: t.prazo, ordem: t.ordem }));
    if (ok) recarregar();
  }

  async function excluir(t: TarefaCasaAbrigo) {
    const ok = await run(() => apiClient.delete(`v1/sempras/casa-abrigo-tarefas/${t.id}`), "Tarefa removida");
    if (ok) recarregar();
  }

  function soltar(e: DragEvent<HTMLElement>, status: StatusTarefa) {
    e.preventDefault();
    const t = (lista.data ?? []).find((x) => x.id === e.dataTransfer.getData("text/plain"));
    if (t) void mover(t, status);
  }

  const hoje = hojeISO();
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end gap-3">
        <Input id="tarefas-sprint" label="Filtrar por sprint" value={filtroSprint} onChange={(e) => setFiltroSprint(e.target.value)} />
        <Select id="tarefas-prioridade" label="Filtrar por prioridade" value={filtroPrioridade} onChange={(e) => setFiltroPrioridade(e.target.value)}
          options={[{ value: "", label: "Todas" }, ...Object.entries(PRIORIDADES_TAREFA).map(([value, p]) => ({ value, label: p.label }))]} />
        <div className="flex-1" />
        <Button type="button" onClick={() => abrir("nova")}>
          <Plus size={14} className="mr-1" aria-hidden="true" /> Nova tarefa
        </Button>
      </div>

      <DataState loading={lista.isLoading} error={lista.error} onRetry={recarregar} empty={false}>
        <div className="grid gap-3 overflow-x-auto md:grid-cols-5">
          {COLUNAS.map((col, i) => {
            const daColuna = tarefas.filter((t) => t.status === col);
            return (
              <section
                key={col}
                aria-label={COLUNAS_KANBAN[col]}
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => soltar(e, col)}
                className="flex min-h-64 flex-col gap-2 rounded-xl border border-dashed border-surface-border bg-surface/50 p-2"
              >
                <h3 className="flex items-center justify-between rounded-lg bg-primary/10 px-3 py-1.5 text-sm font-semibold">
                  {COLUNAS_KANBAN[col]} <Badge>{daColuna.length}</Badge>
                </h3>
                {daColuna.map((t) => (
                  <article
                    key={t.id}
                    draggable
                    onDragStart={(e) => e.dataTransfer.setData("text/plain", t.id)}
                    className="flex cursor-grab flex-col gap-1 rounded-lg border border-surface-border bg-surface p-2 text-sm shadow-xs"
                  >
                    <span className="flex items-start justify-between gap-1">
                      <strong>{t.titulo}</strong>
                      <Badge tone={PRIORIDADES_TAREFA[t.prioridade].tone}>{PRIORIDADES_TAREFA[t.prioridade].label}</Badge>
                    </span>
                    {t.descricao && <span className="text-xs text-muted">{t.descricao}</span>}
                    <span className="text-xs text-muted">
                      {t.responsavel && `${t.responsavel} · `}
                      {t.sprint && `${t.sprint} · `}
                      {t.prazo && (
                        <span className={t.status !== "done" && diaDe(t.prazo) < hoje ? "font-semibold text-danger" : ""}>prazo {fmtDia(t.prazo)}</span>
                      )}
                    </span>
                    <span className="flex justify-end gap-0.5">
                      {i > 0 && (
                        <Button type="button" size="sm" variant="ghost" aria-label={`Mover ${t.titulo} para ${COLUNAS_KANBAN[COLUNAS[i - 1]!]}`}
                          onClick={() => void mover(t, COLUNAS[i - 1]!)}>
                          <ChevronLeft size={14} aria-hidden="true" />
                        </Button>
                      )}
                      {i < COLUNAS.length - 1 && (
                        <Button type="button" size="sm" variant="ghost" aria-label={`Mover ${t.titulo} para ${COLUNAS_KANBAN[COLUNAS[i + 1]!]}`}
                          onClick={() => void mover(t, COLUNAS[i + 1]!)}>
                          <ChevronRight size={14} aria-hidden="true" />
                        </Button>
                      )}
                      <Button type="button" size="sm" variant="ghost" aria-label={`Editar ${t.titulo}`} onClick={() => abrir(t)}>
                        <Pencil size={14} aria-hidden="true" />
                      </Button>
                      <Button type="button" size="sm" variant="ghost" aria-label={`Excluir ${t.titulo}`} onClick={() => void excluir(t)}>
                        <Trash2 size={14} aria-hidden="true" />
                      </Button>
                    </span>
                  </article>
                ))}
              </section>
            );
          })}
        </div>
      </DataState>

      <Dialog open={aberta !== null} onClose={() => setAberta(null)} title={aberta === "nova" ? "Nova tarefa" : "Editar tarefa"} size="lg">
        <form onSubmit={salvar} className="flex flex-col gap-3">
          <Input id="tarefa-titulo" label="Título *" value={form.titulo} onChange={(e) => setForm({ ...form, titulo: e.target.value })}
            placeholder="Ex.: finalizar o relatório trimestral" />
          <Textarea id="tarefa-descricao" label="Descrição" rows={3} value={form.descricao} onChange={(e) => setForm({ ...form, descricao: e.target.value })} />
          <div className="grid gap-3 sm:grid-cols-2">
            <Select id="tarefa-prioridade" label="Prioridade" value={form.prioridade} onChange={(e) => setForm({ ...form, prioridade: e.target.value as PrioridadeTarefa })}
              options={Object.entries(PRIORIDADES_TAREFA).map(([value, p]) => ({ value, label: p.label }))} />
            <Select id="tarefa-status" label="Coluna" value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as StatusTarefa })}
              options={COLUNAS.map((c) => ({ value: c, label: COLUNAS_KANBAN[c] }))} />
            <Input id="tarefa-responsavel" label="Responsável" value={form.responsavel} onChange={(e) => setForm({ ...form, responsavel: e.target.value })} />
            <Input id="tarefa-sprint" label="Sprint" value={form.sprint} onChange={(e) => setForm({ ...form, sprint: e.target.value })} placeholder="Ex.: Sprint 3" />
            <Input id="tarefa-prazo" label="Prazo" type="date" value={form.prazo} onChange={(e) => setForm({ ...form, prazo: e.target.value })} />
          </div>
          <div>
            <Button type="submit" loading={pending} disabled={form.titulo.trim().length < 2}>
              Salvar
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
