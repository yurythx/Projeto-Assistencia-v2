import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { CheckCircle2, ShieldCheck, XCircle } from "lucide-react";

import { PublicShell } from "@/components/layout/PublicShell";
import { VerifyDocument } from "@/components/public/VerifyDocument";
import { PublicApiError, publicGet } from "@/lib/api/publicServer";
import type { Verification } from "@/lib/nexus/types";

export const metadata: Metadata = { title: "Verificação de assinatura", robots: { index: false } };

const STATUS: Record<Verification["status"], string> = {
  pending: "Aguardando assinaturas",
  completed: "Todas as assinaturas concluídas",
  refused: "Recusado por um signatário",
  cancelled: "Cancelado",
};

export default async function VerificarPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[0-9a-f-]{36}$/i.test(id)) notFound();
  let v: Verification;
  try {
    v = (await publicGet<Verification>(`signum/verify/${id}`, 0)).data;
  } catch (err) {
    if (err instanceof PublicApiError && err.status === 404) notFound();
    throw err;
  }

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-6 px-6 py-12">
        <div className="flex items-center gap-3">
          <ShieldCheck size={28} aria-hidden="true" className="text-primary" />
          <div>
            <p className="dateline">Verificação de autenticidade</p>
            <h1 className="text-2xl font-bold">{v.title}</h1>
          </div>
        </div>
        <dl className="grid gap-3 rounded-xl border border-surface-border p-5 text-sm">
          <div>
            <dt className="text-xs text-muted">Situação</dt>
            <dd className="font-medium">{STATUS[v.status]}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted">SHA-256 do documento</dt>
            <dd className="break-all font-mono text-xs">{v.document_sha256}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted">Consultado em</dt>
            <dd>{new Date(v.checked_at).toLocaleString("pt-BR")}</dd>
          </div>
        </dl>
        <section aria-labelledby="assinaturas">
          <h2 id="assinaturas" className="mb-3 text-lg font-semibold">
            Assinaturas
          </h2>
          <ul className="flex flex-col gap-2">
            {v.signatures.map((s, i) => (
              <li key={i} className="flex items-center justify-between gap-2 rounded-lg border border-surface-border p-3 text-sm">
                <span>
                  <span className="font-medium">{s.name}</span>
                  {s.signed_at && <span className="block text-xs text-muted">{new Date(s.signed_at).toLocaleString("pt-BR")} · {s.method}</span>}
                </span>
                {s.status === "signed" ? (
                  s.valid ? (
                    <span className="flex items-center gap-1 text-success"><CheckCircle2 size={16} aria-hidden="true" /> Válida</span>
                  ) : (
                    <span className="flex items-center gap-1 text-danger"><XCircle size={16} aria-hidden="true" /> Inválida</span>
                  )
                ) : (
                  <span className="text-xs text-muted">{s.status === "refused" ? "Recusou" : "Pendente"}</span>
                )}
              </li>
            ))}
          </ul>
        </section>
        <VerifyDocument envelopeId={v.envelope_id} />
      </div>
    </PublicShell>
  );
}
