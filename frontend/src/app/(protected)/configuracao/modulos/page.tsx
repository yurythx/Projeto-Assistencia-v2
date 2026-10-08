"use client";

import { ArrowDownToLine, ArrowUpFromLine, LayoutGrid, Lock, Network, TriangleAlert } from "lucide-react";
import { useState } from "react";

import { ModuleIcon } from "@/components/layout/ModuleIcon";
import { ModuleGraph } from "@/components/modules/ModuleGraph";
import { DataState } from "@/components/nexus/DataState";
import { useToast } from "@/components/notifications/ToastProvider";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Toggle } from "@/components/ui/Toggle";
import { ApiError, apiClient } from "@/lib/api/client";
import { moduleNames, planToggle, type ToggleStep } from "@/lib/modules/plan";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ModuleStatus } from "@/lib/nexus/types";

interface Pending {
  target: ModuleStatus;
  enabled: boolean;
  steps: ToggleStep[];
}

/**
 * Ciclo de vida dos plug-ins (Kernel): ativar/desativar em runtime, sem
 * recompilar nem reiniciar. Um módulo inativo desliga rotas HTTP, workers,
 * consumidores de fila e tópicos WebSocket; o núcleo (IAM, Auditoria) nunca
 * é desativável. O grafo de dependências é exibido nos dois sentidos e as
 * operações em cascata são confirmadas antes de executar (o backend recusa
 * com 409 qualquer ordem inválida). Duas visões: cartões e grafo.
 */
export default function ModulosPage() {
  const { modules, loading, can, refreshModules } = useNexus();
  const { showToast } = useToast();
  const [pending, setPending] = useState<Pending | null>(null);
  const [busy, setBusy] = useState(false);
  const [view, setView] = useState<"cards" | "graph">("cards");
  const canManage = can("modules:manage");
  const byKey = new Map(modules.map((m) => [m.key, m]));

  async function execute(steps: ToggleStep[]) {
    setBusy(true);
    try {
      for (const step of steps) {
        await apiClient.patch<ModuleStatus>(`v1/admin/modules/${step.key}`, { enabled: step.enabled });
      }
      const last = steps[steps.length - 1];
      showToast({
        title: last ? `${last.name} ${last.enabled ? "ativado" : "desativado"}` : "Nada a alterar",
        description: steps.length > 1 ? `Também ${last?.enabled ? "ativados" : "desativados"}: ${steps.slice(0, -1).map((s) => s.name).join(", ")}` : undefined,
        tone: "success",
      });
    } catch (err) {
      showToast({ title: "Operação não concluída", description: err instanceof ApiError ? err.message : "Falha ao alterar o módulo.", tone: "danger" });
    } finally {
      setBusy(false);
      setPending(null);
      refreshModules();
    }
  }

  function request(m: ModuleStatus, enabled: boolean) {
    const { steps, error } = planToggle(modules, m.key, enabled);
    if (error) {
      showToast({ title: "Operação não permitida", description: error, tone: "danger" });
      return;
    }
    if (steps.length === 0) return;
    // Só o próprio módulo: executa direto. Cascata: pede confirmação.
    if (steps.length === 1) void execute(steps);
    else setPending({ target: m, enabled, steps });
  }

  const cascade = pending?.steps.filter((s) => s.key !== pending.target.key) ?? [];

  return (
    <DataState loading={loading && modules.length === 0} error={null} empty={modules.length === 0} emptyTitle="Nenhum módulo registrado">
      <div role="group" aria-label="Visualização" className="mb-4 inline-flex w-fit self-start rounded-lg border border-surface-border p-0.5">
        {([
          ["cards", "Cartões", LayoutGrid],
          ["graph", "Grafo de dependências", Network],
        ] as const).map(([v, label, Icon]) => (
          <button
            key={v}
            type="button"
            aria-pressed={view === v}
            onClick={() => setView(v)}
            className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-primary ${
              view === v ? "bg-primary text-primary-foreground" : "text-muted hover:text-foreground"
            }`}
          >
            <Icon size={14} aria-hidden="true" /> {label}
          </button>
        ))}
      </div>

      {view === "graph" ? (
        <ModuleGraph modules={modules} canManage={canManage} busy={busy} onToggle={request} />
      ) : (
        <ul className="grid gap-4 lg:grid-cols-2">
          {modules.map((m) => {
            const waiting = m.configured && !m.enabled && m.blocked_by.length > 0;
            return (
              <li key={m.key}>
                <Card className="flex h-full flex-col gap-3 p-5" aria-labelledby={`mod-${m.key}`}>
                  <div className="flex items-start gap-3">
                    <span className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      <ModuleIcon name={m.icon} size={20} aria-hidden="true" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h2 id={`mod-${m.key}`} className="text-base font-semibold">
                          {m.name}
                        </h2>
                        <code className="text-xs text-muted">{m.key}</code>
                        {m.core && <Badge tone="info">Núcleo</Badge>}
                        {m.public && <Badge>Superfície pública</Badge>}
                        {m.enabled && !m.core && <Badge tone="success">Ativo</Badge>}
                        {!m.enabled && !waiting && <Badge>Inativo</Badge>}
                      </div>
                      <p className="mt-1 text-sm text-muted">{m.description}</p>
                    </div>
                    {m.core ? (
                      <span className="inline-flex items-center gap-1 text-xs text-muted" title="Módulo do núcleo — nunca desativável">
                        <Lock size={14} aria-hidden="true" /> Sempre ativo
                      </span>
                    ) : (
                      <Toggle
                        label={`${m.configured ? "Desativar" : "Ativar"} ${m.name}`}
                        checked={m.configured}
                        disabled={!canManage || busy}
                        onChange={(v) => request(m, v)}
                      />
                    )}
                  </div>

                  {waiting && (
                    <p role="status" className="flex items-start gap-2 rounded-md bg-warning/10 p-2 text-xs text-foreground">
                      <TriangleAlert size={14} aria-hidden="true" className="mt-0.5 shrink-0 text-warning" />
                      Configurado como ativo, mas indisponível: depende de {moduleNames(modules, m.blocked_by)}, que está inativo.
                      {canManage && (
                        <Button variant="ghost" size="sm" className="ml-auto -my-1" onClick={() => request(m, true)}>
                          Ativar dependências
                        </Button>
                      )}
                    </p>
                  )}

                  {(m.depends_on.length > 0 || m.dependents.length > 0) && (
                    <dl className="flex flex-col gap-1.5 text-xs">
                      {m.depends_on.length > 0 && (
                        <div className="flex flex-wrap items-center gap-1">
                          <dt className="inline-flex items-center gap-1 font-medium text-muted">
                            <ArrowDownToLine size={12} aria-hidden="true" /> Depende de:
                          </dt>
                          {m.depends_on.map((d) => (
                            <dd key={d}>
                              <Badge tone={byKey.get(d)?.enabled ? "success" : "warning"}>
                                {byKey.get(d)?.name ?? d}
                                <span className="sr-only">{byKey.get(d)?.enabled ? " (ativo)" : " (inativo)"}</span>
                              </Badge>
                            </dd>
                          ))}
                        </div>
                      )}
                      {m.dependents.length > 0 && (
                        <div className="flex flex-wrap items-center gap-1">
                          <dt className="inline-flex items-center gap-1 font-medium text-muted">
                            <ArrowUpFromLine size={12} aria-hidden="true" /> Necessário para:
                          </dt>
                          {m.dependents.map((d) => (
                            <dd key={d}>
                              <Badge tone={byKey.get(d)?.enabled ? "info" : "neutral"}>
                                {byKey.get(d)?.name ?? d}
                                <span className="sr-only">{byKey.get(d)?.enabled ? " (ativo)" : " (inativo)"}</span>
                              </Badge>
                            </dd>
                          ))}
                        </div>
                      )}
                    </dl>
                  )}

                  {m.permissions.length > 0 && (
                    <details className="text-xs">
                      <summary className="cursor-pointer text-muted hover:text-foreground">{m.permissions.length} permissões declaradas</summary>
                      <ul className="mt-2 flex flex-col gap-1">
                        {m.permissions.map((p) => (
                          <li key={p.key}>
                            <code className="font-mono">{p.key}</code> — <span className="text-muted">{p.description}</span>
                          </li>
                        ))}
                      </ul>
                    </details>
                  )}
                </Card>
              </li>
            );
          })}
        </ul>
      )}

      <Dialog
        open={pending !== null}
        onClose={() => setPending(null)}
        title={pending ? `${pending.enabled ? "Ativar" : "Desativar"} ${pending.target.name}?` : ""}
        description={
          pending
            ? pending.enabled
              ? `${pending.target.name} depende de módulos inativos, que serão ativados antes:`
              : `Estes módulos dependem de ${pending.target.name} e serão desativados antes:`
            : undefined
        }
        footer={
          <>
            <Button variant="secondary" onClick={() => setPending(null)}>
              Cancelar
            </Button>
            <Button
              variant={pending?.enabled ? "primary" : "danger"}
              className="ml-auto"
              loading={busy}
              onClick={() => pending && void execute(pending.steps)}
            >
              {pending?.enabled ? "Ativar todos" : "Desativar todos"}
            </Button>
          </>
        }
      >
        <ol className="ml-5 list-decimal text-sm">
          {cascade.map((s) => (
            <li key={s.key}>{s.name}</li>
          ))}
          {pending && <li className="font-semibold">{pending.target.name}</li>}
        </ol>
        {pending && !pending.enabled && (
          <p className="mt-3 text-xs text-muted">
            Módulos desativados respondem 404 na API, param workers e filas (eventos ficam retidos até a reativação) e somem do menu e do site.
          </p>
        )}
      </Dialog>
    </DataState>
  );
}
