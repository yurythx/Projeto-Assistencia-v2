import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Mail, MapPin, Phone } from "lucide-react";

import { PublicShell } from "@/components/layout/PublicShell";
import { PublicApiError, publicGet } from "@/lib/api/publicServer";
import { APP_URL } from "@/lib/env";
import type { Sector } from "@/lib/nexus/types";

const description = "Unidades e setores de atendimento, com contatos e endereços.";

export const metadata: Metadata = { title: "Setores", description, alternates: { canonical: `${APP_URL}/setores` } };

export default async function SetoresPage() {
  let sectors: Sector[];
  try {
    sectors = (await publicGet<Sector[]>("directory/public/sectors", 300)).data;
  } catch (err) {
    // Plug-in Diretório desativado no Kernel → a página deixa de existir.
    if (err instanceof PublicApiError && err.status === 404) notFound();
    throw err;
  }
  const unidades = sectors.filter((s) => s.kind === "unidade");

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-8 px-6 py-12">
        <div>
          <p className="dateline">Onde encontrar</p>
          <h1 className="mt-2 text-3xl font-bold">Setores e unidades</h1>
          <p className="mt-1 text-muted">{description}</p>
        </div>
        <ul className="grid gap-4 md:grid-cols-2">
          {unidades.map((u) => (
            <li key={u.id} className="rounded-xl border border-surface-border bg-surface p-5">
              <h2 className="text-lg font-semibold">
                {u.nome} {u.sigla && <span className="text-muted">({u.sigla})</span>}
              </h2>
              <p className="text-xs text-muted">{u.entidade}</p>
              <ul className="mt-3 flex flex-col gap-1 text-sm">
                {u.endereco && <li className="flex items-start gap-2"><MapPin size={14} aria-hidden="true" className="mt-0.5" /> {u.endereco}</li>}
                {u.telefone && <li className="flex items-center gap-2"><Phone size={14} aria-hidden="true" /> {u.telefone}</li>}
                {u.email && (
                  <li className="flex items-center gap-2">
                    <Mail size={14} aria-hidden="true" />
                    <a href={`mailto:${u.email}`} className="text-primary hover:underline">{u.email}</a>
                  </li>
                )}
              </ul>
              {sectors.some((d) => d.kind === "departamento" && d.unidade_id === u.id) && (
                <details className="mt-3 text-sm">
                  <summary className="cursor-pointer text-muted hover:text-foreground">Departamentos</summary>
                  <ul className="mt-2 flex flex-col gap-1">
                    {sectors
                      .filter((d) => d.kind === "departamento" && d.unidade_id === u.id)
                      .map((d) => (
                        <li key={d.id}>
                          {d.nome} {d.telefone && <span className="text-muted">· {d.telefone}</span>}
                        </li>
                      ))}
                  </ul>
                </details>
              )}
            </li>
          ))}
        </ul>
      </div>
    </PublicShell>
  );
}
