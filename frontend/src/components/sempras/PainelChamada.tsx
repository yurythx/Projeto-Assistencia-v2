"use client";

import { useTopic } from "@/components/realtime/RealtimeProvider";
import { useApiQuery } from "@/lib/api/swr";
import { rotuloPrioridade, type FilaItem } from "@/lib/sempras/types";

/** Painel de chamada para TV da sala de espera: últimos chamados em
 * destaque e o tamanho da fila — mesmo tópico em tempo real da Fila
 * (sempras:fila:<unidade>), sem nenhuma ação (só exibição). */
export function PainelChamada({ unidadeId }: { unidadeId: string }) {
  const fila = useApiQuery<FilaItem[]>(`v1/sempras/unidades/${unidadeId}/fila`, { refreshInterval: 15_000 });
  useTopic(`sempras:fila:${unidadeId}`, () => void fila.mutate());

  const itens = fila.data ?? [];
  const chamados = itens.filter((i) => i.status === "chamado").slice(-5).reverse();
  const aguardando = itens.filter((i) => i.status === "aguardando").length;

  return (
    <div className="flex flex-col gap-8">
      <p className="text-center text-2xl text-muted" aria-live="polite">
        {aguardando} {aguardando === 1 ? "pessoa aguardando" : "pessoas aguardando"}
      </p>

      {chamados.length === 0 ? (
        <p className="py-20 text-center text-3xl text-muted">Nenhuma senha chamada</p>
      ) : (
        <ul aria-label="Últimas senhas chamadas" aria-live="polite" className="flex flex-col gap-4">
          {chamados.map((item, i) => (
            <li key={item.id}
              className={`flex items-center justify-between gap-6 rounded-2xl border border-surface-border p-6 ${i === 0 ? "bg-primary/10" : "bg-surface"}`}>
              <span className={`font-mono font-bold tabular-nums ${i === 0 ? "text-7xl text-primary" : "text-4xl text-foreground"}`}>
                {item.senha_exibida}
              </span>
              <span className="flex-1 text-right">
                <span className={i === 0 ? "text-4xl font-semibold" : "text-2xl"}>{item.nome}</span>
                <span className="block text-lg text-muted">
                  {[item.local, item.prioridade && rotuloPrioridade(item.prioridade)].filter(Boolean).join(" · ")}
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
