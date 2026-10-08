"use client";

import Link from "next/link";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import type { NotificacaoServidor } from "@/components/notifications/NotificationHistoryProvider";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Toggle } from "@/components/ui/Toggle";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";

/** Módulos que enviam avisos (as preferências valem por módulo). */
const MODULOS: { chave: string; nome: string; descricao: string }[] = [
  {
    chave: "atlas",
    nome: "Atlas",
    descricao:
      "Novas versões dos procedimentos que você segue ou dos quais sua unidade participa, e dos modelos deles.",
  },
  {
    chave: "tramite",
    nome: "Trâmite",
    descricao: "Mudança no procedimento do Atlas seguido por um processo que você abriu.",
  },
];

interface Caixa {
  itens: NotificacaoServidor[];
  nao_lidas: number;
}

/** Caixa de notificações e preferências (ADR 027). */
export default function NotificacoesPage() {
  const caixa = useApiQuery<Caixa>("v1/notificacoes?limit=50");
  const prefs = useApiQuery<{ modulo: string; ativo: boolean }[]>("v1/notificacoes/preferencias");
  const { run, pending } = useAction();
  const itens = caixa.data?.itens ?? [];
  const ativo = (m: string) => prefs.data?.find((p) => p.modulo === m)?.ativo ?? true;

  async function lida(id: string) {
    await apiClient.post(`v1/notificacoes/${id}/lida`).catch(() => {});
    void caixa.mutate();
  }

  async function todas() {
    if (await run(() => apiClient.post("v1/notificacoes/lidas"), "Todas marcadas como lidas"))
      void caixa.mutate();
  }

  async function preferir(modulo: string, valor: boolean) {
    if (
      await run(
        () => apiClient.put("v1/notificacoes/preferencias", { modulo, ativo: valor }),
        valor ? "Avisos ligados" : "Avisos desligados",
      )
    )
      void prefs.mutate();
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Notificações"
        description="Os avisos que chegaram para você e quais módulos podem avisar."
        actions={
          (caixa.data?.nao_lidas ?? 0) > 0 && (
            <Button variant="secondary" loading={pending} onClick={() => void todas()}>
              Marcar todas como lidas
            </Button>
          )
        }
      />
      <div className="grid gap-6 lg:grid-cols-[1fr_22rem]">
        <Card>
          <CardHeader>
            <CardTitle as="h2">Caixa ({caixa.data?.nao_lidas ?? 0} não lidas)</CardTitle>
          </CardHeader>
          <CardContent className="pb-4 pt-2">
            <DataState
              loading={caixa.isLoading}
              error={caixa.error}
              onRetry={() => void caixa.mutate()}
              empty={itens.length === 0}
              emptyTitle="Nenhuma notificação"
            >
              <ul
                className="flex flex-col divide-y divide-surface-border"
                aria-label="Notificações"
              >
                {itens.map((n) => (
                  <li
                    key={n.id}
                    className="flex flex-wrap items-start justify-between gap-2 py-3 text-sm"
                  >
                    <span className="min-w-0">
                      {n.link ? (
                        <Link
                          href={n.link}
                          onClick={() => void lida(n.id)}
                          className={`hover:underline ${n.lida ? "text-foreground" : "font-semibold text-foreground"}`}
                        >
                          {n.titulo}
                        </Link>
                      ) : (
                        <span className={n.lida ? "" : "font-semibold"}>{n.titulo}</span>
                      )}
                      {n.mensagem && <span className="block text-xs text-muted">{n.mensagem}</span>}
                      <span className="block text-xs text-muted">{fmtDateTime(n.created_at)}</span>
                    </span>
                    {!n.lida && (
                      <Button variant="ghost" size="sm" onClick={() => void lida(n.id)}>
                        Marcar como lida
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            </DataState>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle as="h2">Receber avisos de</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4 pb-4 pt-2 text-sm">
            {MODULOS.map((m) => (
              <div key={m.chave} className="flex items-start justify-between gap-3">
                <span>
                  <span className="block font-medium text-foreground">{m.nome}</span>
                  <span className="block text-xs text-muted">{m.descricao}</span>
                </span>
                <Toggle
                  checked={ativo(m.chave)}
                  onChange={(v) => void preferir(m.chave, v)}
                  label={`Avisos do ${m.nome}`}
                />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
