import type { Metadata } from "next";
import { Database, FileJson } from "lucide-react";

import { PublicShell } from "@/components/layout/PublicShell";
import { publicGetOr } from "@/lib/api/publicServer";
import { APP_URL } from "@/lib/env";

const description = "Transparência ativa (LAI — Lei 12.527/2011): conjuntos de dados abertos da plataforma, sem dados pessoais.";

export const metadata: Metadata = { title: "Transparência", description, alternates: { canonical: `${APP_URL}/transparencia` } };

interface Dataset {
  id: string;
  titulo: string;
  endpoint: string;
  campos: Record<string, string>;
  periodicidade: string;
  formatos: string[];
}

/** Links de download passam pelo proxy público (/api/public, allowlist). */
function downloadHref(endpoint: string, format: string): string {
  const path = endpoint.replace(/^\/api\//, "");
  return `/api/public/${path}${path.includes("?") ? "&" : "?"}format=${format}`;
}

export default async function TransparenciaPage() {
  const datasets = await publicGetOr<Dataset[]>("transparencia/datasets", [], 300);

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-8 px-6 py-12">
        <div>
          <p className="dateline">Lei de Acesso à Informação</p>
          <h1 className="mt-2 text-3xl font-bold">Transparência ativa</h1>
          <p className="mt-1 text-muted">{description}</p>
        </div>
        {datasets.length === 0 ? (
          <p className="rounded-xl border border-dashed border-surface-border p-10 text-center text-muted">Catálogo de dados abertos indisponível no momento.</p>
        ) : (
          <ul className="flex flex-col gap-4">
            {datasets.map((d) => (
              <li key={d.id} className="rounded-xl border border-surface-border bg-surface p-5">
                <div className="flex items-start gap-3">
                  <Database size={20} aria-hidden="true" className="mt-1 text-primary" />
                  <div className="min-w-0 flex-1">
                    <h2 className="text-lg font-semibold">{d.titulo}</h2>
                    <p className="text-xs text-muted">Atualização: {d.periodicidade}</p>
                    <details className="mt-2 text-sm">
                      <summary className="cursor-pointer text-muted hover:text-foreground">Dicionário de dados</summary>
                      <dl className="mt-2 grid gap-1">
                        {Object.entries(d.campos).map(([k, v]) => (
                          <div key={k}>
                            <dt className="inline font-mono text-xs">{k}</dt> <dd className="inline text-muted">— {v}</dd>
                          </div>
                        ))}
                      </dl>
                    </details>
                    <div className="mt-3 flex flex-wrap gap-2" aria-label={`Downloads de ${d.titulo}`}>
                      {d.formatos.map((f) => (
                        <a
                          key={f}
                          href={downloadHref(d.endpoint, f)}
                          className="inline-flex items-center gap-1 rounded-md border border-surface-border px-3 py-1 text-xs font-medium uppercase hover:bg-surface-hover"
                        >
                          <FileJson size={12} aria-hidden="true" /> {f}
                        </a>
                      ))}
                    </div>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </PublicShell>
  );
}
