"use client";

import { AlertTriangle, Baby, CalendarCheck, CheckCircle2, ClipboardList, UserMinus, Users } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { fmtDia, type Alerta, type PainelCasaAbrigo as Painel, type TipoAlerta } from "@/lib/sempras/types";

/** A aba que cada aviso abre. */
export const ABA_DO_ALERTA: Record<TipoAlerta, "pia" | "relatorios" | "tarefas"> = {
  pia: "pia",
  meta: "pia",
  relatorio: "relatorios",
  tarefa: "tarefas",
};

function prazoTexto(a: Alerta): string {
  if (a.dias < 0) return `atrasado há ${-a.dias} dia(s) (${fmtDia(a.prazo)})`;
  if (a.dias === 0) return `vence hoje (${fmtDia(a.prazo)})`;
  return `vence em ${a.dias} dia(s) (${fmtDia(a.prazo)})`;
}

/**
 * Tela inicial da Casa Abrigo: os cartões do sistema da unidade (crianças,
 * adolescentes, ativos, PIAs vencendo, desacolhidos) e o monitor de prazos
 * — reavaliação do PIA (ECA art. 19, §1º), metas, relatórios e tarefas.
 */
export function PainelCasaAbrigo({ painel, onAbrir }: { painel: Painel; onAbrir: (alerta: Alerta) => void }) {
  const cartoes = [
    { titulo: "Crianças (0 a 11 anos)", valor: painel.criancas, icone: Baby },
    { titulo: "Adolescentes (12 a 17)", valor: painel.adolescentes, icone: Users },
    { titulo: "Acolhimentos ativos", valor: painel.ativos, icone: CalendarCheck },
    { titulo: "PIAs vencendo", valor: painel.pias_vencendo, icone: AlertTriangle },
    { titulo: "Desacolhidos", valor: painel.desacolhidos, icone: UserMinus },
  ];
  return (
    <div className="flex flex-col gap-6">
      <ul aria-label="Indicadores da unidade" className="grid gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {cartoes.map(({ titulo, valor, icone: Icone }) => (
          <li key={titulo} className="flex items-center gap-3 rounded-xl border border-surface-border bg-surface p-4">
            <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Icone size={20} aria-hidden="true" />
            </span>
            <span className="flex flex-col">
              <span className="text-xs text-muted">{titulo}</span>
              <strong className="text-2xl">{String(valor).padStart(2, "0")}</strong>
            </span>
          </li>
        ))}
      </ul>

      <Card>
        <CardHeader>
          <CardTitle as="h2">
            <ClipboardList size={16} className="mr-1 inline" aria-hidden="true" />
            Monitor de prazos
          </CardTitle>
        </CardHeader>
        <CardContent className="pb-5">
          {painel.alertas.length === 0 ? (
            <p className="flex items-center gap-2 text-sm text-success">
              <CheckCircle2 size={16} aria-hidden="true" /> Tudo em ordem: nenhum prazo de PIA, meta, relatório ou tarefa vencendo.
            </p>
          ) : (
            <ul aria-label="Avisos de prazo" className="flex flex-col gap-2">
              {painel.alertas.map((a) => (
                <li key={`${a.tipo}-${a.ref_id}`}>
                  <button
                    type="button"
                    onClick={() => onAbrir(a)}
                    className={`flex w-full flex-wrap items-center justify-between gap-2 rounded-lg border p-3 text-left text-sm ${
                      a.nivel === "atraso" ? "border-danger/40 bg-danger/5" : "border-warning/40 bg-warning/5"
                    }`}
                  >
                    <span className="font-medium">{a.mensagem}</span>
                    <span className={a.nivel === "atraso" ? "text-danger" : "text-warning"}>{prazoTexto(a)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
