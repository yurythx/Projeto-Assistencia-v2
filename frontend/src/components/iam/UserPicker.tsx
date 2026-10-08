"use client";

import { X } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useApiPage, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { Person } from "@/lib/nexus/types";

export interface PickedUser {
  id: string;
  name: string;
}

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Seleção de pessoas pelo Diretório (qualquer autenticado). Com o
 * plug-in Diretório desativado, degrada para informar o id do usuário. */
export function UserPicker({ value, onChange, idPrefix, label = "Pessoas" }: { value: PickedUser[]; onChange: (v: PickedUser[]) => void; idPrefix: string; label?: string }) {
  const { enabled } = useNexus();
  const [q, setQ] = useState("");
  const directory = enabled("directory");
  const results = useApiPage<Person>(directory && q.trim().length >= 2 ? withQuery("v1/directory/people", { q: q.trim(), page_size: 8 }) : null);
  const add = (u: PickedUser) => {
    if (!value.some((v) => v.id === u.id)) onChange([...value, u]);
    setQ("");
  };

  return (
    <div className="flex flex-col gap-2">
      <Input
        id={`${idPrefix}-q`}
        label={label}
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder={directory ? "Digite para buscar no Diretório" : "Id (UUID) do usuário"}
        autoComplete="off"
        onKeyDown={(e) => {
          if (!directory && e.key === "Enter" && UUID_RE.test(q.trim())) {
            e.preventDefault();
            add({ id: q.trim(), name: q.trim() });
          }
        }}
      />
      {directory && (results.data?.items ?? []).length > 0 && (
        <ul role="listbox" aria-label="Resultados" className="flex flex-col rounded-lg border border-surface-border">
          {(results.data?.items ?? []).map((p) => (
            <li key={p.user_id} role="option" aria-selected={value.some((v) => v.id === p.user_id)}>
              <button type="button" onClick={() => add({ id: p.user_id, name: p.name || p.username })} className="w-full px-3 py-2 text-left text-sm hover:bg-surface-hover">
                {p.name || p.username} <span className="text-xs text-muted">{[p.job_title, p.unidade].filter(Boolean).join(" · ")}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
      {!directory && (
        <Button type="button" variant="secondary" size="sm" className="self-start" disabled={!UUID_RE.test(q.trim())} onClick={() => add({ id: q.trim(), name: q.trim() })}>
          Adicionar
        </Button>
      )}
      {value.length > 0 && (
        <ol className="flex flex-col gap-1">
          {value.map((u, i) => (
            <li key={u.id} className="flex items-center justify-between rounded-md bg-surface-hover px-3 py-1.5 text-sm">
              <span>
                <span className="mr-2 font-mono text-xs text-muted">{i + 1}.</span>
                {u.name}
              </span>
              <button type="button" aria-label={`Remover ${u.name}`} onClick={() => onChange(value.filter((v) => v.id !== u.id))} className="rounded p-1 text-muted hover:text-danger">
                <X size={14} aria-hidden="true" />
              </button>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
