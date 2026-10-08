"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FilePlus2, PenTool } from "lucide-react";
import { useState, type FormEvent } from "react";

import { UserPicker, type PickedUser } from "@/components/iam/UserPicker";
import { SectionTabsInline } from "@/components/nexus/SectionTabsInline";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { FileHash } from "@/components/signum/FileHash";
import { ENVELOPE_STATUS } from "@/components/signum/envelopeStatus";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { Envelope } from "@/lib/nexus/types";


function OpenEnvelope({ onDone }: { onDone: (e: Envelope) => void }) {
  const { run, pending } = useAction();
  const [hash, setHash] = useState("");
  const [fileName, setFileName] = useState("");
  const [signers, setSigners] = useState<PickedUser[]>([]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const res = await run(
      () =>
        apiClient.post<Envelope>("v1/signum/envelopes", {
          title: String(fd.get("title") ?? "").trim() || fileName,
          description: String(fd.get("description") ?? "").trim(),
          document_sha256: hash,
          signer_ids: signers.map((s) => s.id),
          sequential: fd.get("sequential") === "on",
        }),
      "Envelope aberto",
    );
    if (res) onDone(res.data);
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <FileHash
        id="env-file"
        label="Documento *"
        onHash={(h, name) => {
          setHash(h);
          setFileName(name);
        }}
      />
      <Input id="env-title" name="title" label="Título" maxLength={200} placeholder={fileName || "nome do documento"} />
      <Textarea id="env-desc" name="description" label="Descrição" rows={2} maxLength={2000} />
      <UserPicker idPrefix="env-signers" label="Signatários *" value={signers} onChange={setSigners} />
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" name="sequential" className="h-4 w-4 accent-primary" /> Assinatura sequencial (na ordem da lista)
      </label>
      <div className="flex justify-end">
        <Button type="submit" loading={pending} disabled={hash.length !== 64 || signers.length === 0}>
          Abrir envelope
        </Button>
      </div>
    </form>
  );
}

export default function SignumPage() {
  const router = useRouter();
  const { can } = useNexus();
  const [role, setRole] = useState<"to_sign" | "created" | "all" | "any">("to_sign");
  const [opening, setOpening] = useState(false);
  const list = useApiQuery<Envelope[]>(withQuery("v1/signum/envelopes", { role }));

  const tabs: { value: typeof role; label: string }[] = [
    { value: "to_sign", label: "Para eu assinar" },
    { value: "created", label: "Criados por mim" },
    { value: "all", label: "Todos os meus" },
  ];
  if (can("signum:manage")) tabs.push({ value: "any", label: "Todos (gestão)" });

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Signum"
        title="Assinaturas eletrônicas"
        description="Envelopes com conferência de integridade SHA-256 e cerimônia de reautenticação a cada assinatura."
        actions={
          <Button onClick={() => setOpening(true)}>
            <FilePlus2 size={16} aria-hidden="true" className="mr-1" /> Novo envelope
          </Button>
        }
      />
      <SectionTabsInline label="Filtro de envelopes" tabs={tabs} value={role} onChange={setRole} />
      <DataState loading={list.isLoading} error={list.error} onRetry={() => void list.mutate()} empty={(list.data ?? []).length === 0} emptyTitle="Nenhum envelope">
        <ul className="flex flex-col gap-2">
          {(list.data ?? []).map((e) => {
            const signed = e.signers.filter((s) => s.status === "signed").length;
            return (
              <li key={e.id}>
                <Link href={`/signum/${e.id}`} className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-surface-border bg-surface p-4 hover:border-primary/50">
                  <div className="flex items-start gap-3">
                    <PenTool size={18} aria-hidden="true" className="mt-0.5 text-primary" />
                    <div>
                      <p className="font-medium">{e.title}</p>
                      <p className="text-xs text-muted">
                        {e.created_by_name} · {fmtDateTime(e.created_at)}
                        {e.source_module && ` · origem: ${e.source_module}`}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 text-xs">
                    <span className="text-muted">
                      {signed}/{e.signers.length} assinaturas
                    </span>
                    <Badge tone={ENVELOPE_STATUS[e.status].tone}>{ENVELOPE_STATUS[e.status].label}</Badge>
                  </div>
                </Link>
              </li>
            );
          })}
        </ul>
      </DataState>
      <Dialog open={opening} onClose={() => setOpening(false)} title="Novo envelope de assinatura" size="lg">
        {opening && <OpenEnvelope onDone={(e) => router.push(`/signum/${e.id}`)} />}
      </Dialog>
    </div>
  );
}
