"use client";

import { useId, type ReactNode } from "react";

import { Select } from "@/components/ui/Select";
import type { AcolhidoResumo } from "@/lib/sempras/types";

/** Grupo de rádio de uma pergunta fechada (sim/não ou sim/não/não sei). */
export function Escolha<T extends string>({
  legenda,
  valor,
  opcoes,
  onChange,
}: {
  legenda: string;
  valor: T;
  opcoes: Record<string, string>;
  onChange: (v: T) => void;
}) {
  const nome = useId();
  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="text-sm font-medium text-foreground">{legenda}</legend>
      <div className="flex flex-wrap gap-4 text-sm">
        {Object.entries(opcoes).map(([v, label]) => (
          <label key={v} className="flex items-center gap-1.5">
            <input type="radio" name={nome} value={v} checked={valor === v} onChange={() => onChange(v as T)} /> {label}
          </label>
        ))}
        {valor !== "" && (
          <button type="button" className="text-xs text-muted underline" onClick={() => onChange("" as T)}>
            limpar
          </button>
        )}
      </div>
    </fieldset>
  );
}

/** Caixa de seleção com rótulo. */
export function Caixa({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center gap-2 text-sm">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} /> {label}
    </label>
  );
}

/** Caixa de grupo do assistente (o "group-box" do 4.0). */
export function Grupo({ titulo, children }: { titulo: string; children: ReactNode }) {
  return (
    <fieldset className="flex flex-col gap-3 rounded-xl border border-surface-border p-4">
      <legend className="px-1 text-xs font-semibold uppercase tracking-wider text-primary">{titulo}</legend>
      {children}
    </fieldset>
  );
}

/** Opções de uma lista de rótulos, com "Selecione…" ("" = não informado). */
export function opcoes(rotulos: Record<string, string>, vazio = "Selecione…"): { value: string; label: string }[] {
  return [{ value: "", label: vazio }, ...Object.entries(rotulos).map(([value, label]) => ({ value, label }))];
}

/** Seletor dos acolhidos ativos da unidade (usado em várias abas). */
export function SeletorAcolhido({
  id,
  acolhidos,
  valor,
  onChange,
  label = "Acolhido",
  vazio = "Selecione o acolhido…",
}: {
  id: string;
  acolhidos: AcolhidoResumo[];
  valor: string;
  onChange: (v: string) => void;
  label?: string;
  vazio?: string;
}) {
  return (
    <Select
      id={id}
      label={label}
      value={valor}
      onChange={(e) => onChange(e.target.value)}
      options={[{ value: "", label: vazio }, ...acolhidos.map((a) => ({ value: a.acolhimento_id, label: a.nome_social || a.nome }))]}
    />
  );
}

/** Abas internas (mesmo visual das abas da ficha). */
export function Abas<T extends string>({
  rotulo,
  abas,
  atual,
  onChange,
}: {
  rotulo: string;
  abas: { key: T; label: string }[];
  atual: T;
  onChange: (k: T) => void;
}) {
  return (
    <div role="tablist" aria-label={rotulo} className="flex flex-wrap gap-1.5 border-b border-surface-border pb-2">
      {abas.map(({ key, label }) => (
        <button
          key={key}
          type="button"
          role="tab"
          aria-selected={atual === key}
          onClick={() => onChange(key)}
          className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
            atual === key ? "bg-primary font-semibold text-primary-foreground shadow-xs" : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  );
}
