"use client";

import { useState } from "react";

import { sha256Hex } from "@/lib/nexus/upload";

/** Calcula no navegador o SHA-256 do documento escolhido — o arquivo
 * nunca sai da máquina do usuário, só o hash. */
export function FileHash({ id, label, onHash }: { id: string; label: string; onHash: (hash: string, name: string) => void }) {
  const [state, setState] = useState<{ name: string; hash: string } | null>(null);
  const [busy, setBusy] = useState(false);
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <input
        id={id}
        type="file"
        className="text-sm"
        onChange={async (e) => {
          const f = e.target.files?.[0];
          if (!f) return;
          setBusy(true);
          const hash = await sha256Hex(f);
          setBusy(false);
          setState({ name: f.name, hash });
          onHash(hash, f.name);
        }}
      />
      <p className="break-all font-mono text-[11px] text-muted" aria-live="polite">
        {busy ? "Calculando SHA-256…" : state ? `SHA-256: ${state.hash}` : "O arquivo é lido apenas localmente para calcular o hash."}
      </p>
    </div>
  );
}
