import type { Metadata } from "next";
import Link from "next/link";
import { ClipboardList } from "lucide-react";

import { PublicShell } from "@/components/layout/PublicShell";
import { publicGetOr } from "@/lib/api/publicServer";
import { APP_URL } from "@/lib/env";
import type { Workflow } from "@/lib/nexus/types";

const description =
  "Como tramita cada processo administrativo: etapas, setores, prazos, documentos exigidos e modelos para baixar (transparência ativa — LAI).";

export const metadata: Metadata = {
  title: "Procedimentos",
  description,
  alternates: { canonical: `${APP_URL}/procedimentos` },
};

/** Procedimentos homologados do Atlas, abertos ao público (ADR 026). */
export default async function ProcedimentosPublicosPage() {
  const lista = await publicGetOr<Workflow[]>("atlas/workflows?page_size=100", [], 300);
  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-8 px-6 py-12">
        <div>
          <p className="dateline">Atlas</p>
          <h1 className="mt-2 text-3xl font-bold">Procedimentos administrativos</h1>
          <p className="mt-1 text-muted">{description}</p>
          <p className="mt-2 text-sm">
            <Link href="/temporalidade" className="text-primary hover:underline">
              Tabela de Temporalidade (por quanto tempo cada documento é guardado)
            </Link>
          </p>
        </div>
        {lista.length === 0 ? (
          <p className="rounded-xl border border-dashed border-surface-border p-10 text-center text-muted">
            Nenhum procedimento publicado no momento.
          </p>
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2" aria-label="Procedimentos">
            {lista.map((w) => (
              <li key={w.id}>
                <Link
                  href={`/procedimentos/${w.id}`}
                  className="flex h-full items-start gap-3 rounded-xl border border-surface-border bg-surface p-4 hover:border-primary/50"
                >
                  <ClipboardList
                    size={20}
                    aria-hidden="true"
                    className="mt-0.5 shrink-0 text-primary"
                  />
                  <span>
                    <span className="block font-mono text-xs text-muted">
                      {w.codigo_processual}
                    </span>
                    <span className="block font-semibold">{w.titulo}</span>
                    <span className="mt-1 block text-sm text-muted">{w.objetivo}</span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </PublicShell>
  );
}
