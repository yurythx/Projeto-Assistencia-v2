"use client";

import { CheckCircle2, XCircle } from "lucide-react";
import { useState } from "react";

import { FileHash } from "@/components/signum/FileHash";
import { publicFetch } from "@/lib/api/publicClient";
import type { Verification } from "@/lib/nexus/types";

/** Confere, sem enviar o arquivo, se um documento é o que foi assinado. */
export function VerifyDocument({ envelopeId }: { envelopeId: string }) {
  const [result, setResult] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);

  return (
    <div className="flex flex-col gap-3">
      <FileHash
        id="verify-file"
        label="Conferir um arquivo"
        onHash={async (hash) => {
          setError(null);
          try {
            const v = await publicFetch<Verification>(`v1/signum/verify/${envelopeId}?document_sha256=${hash}`);
            setResult(Boolean(v.document_match));
          } catch {
            setError("Não foi possível verificar agora. Tente novamente.");
          }
        }}
      />
      {error && <p role="alert" className="text-sm text-danger">{error}</p>}
      {result !== null && (
        <p role="status" className={`flex items-center gap-2 text-sm font-medium ${result ? "text-success" : "text-danger"}`}>
          {result ? <CheckCircle2 size={16} aria-hidden="true" /> : <XCircle size={16} aria-hidden="true" />}
          {result ? "O arquivo é idêntico ao documento assinado." : "O arquivo é diferente do documento assinado."}
        </p>
      )}
    </div>
  );
}
