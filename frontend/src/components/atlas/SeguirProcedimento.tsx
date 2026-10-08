"use client";

import { BellOff, BellRing } from "lucide-react";

import { useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";

interface Seguimento {
  codigo_processual: string;
  seguindo: boolean;
}

/** "Seguir este fluxo" (ADR 027): avisa no sino quando sai uma nova versão
 * do procedimento ou de um modelo dele. Some se não der para saber o estado
 * (ex.: sessão sem usuário local). */
export function SeguirProcedimento({ id }: { id: string }) {
  const rota = `v1/atlas/workflows/${encodeURIComponent(id)}/seguir`;
  const estado = useApiQuery<Seguimento>(rota);
  const { run, pending } = useAction();
  if (!estado.data) return null;
  const seguindo = estado.data.seguindo;

  async function alternar() {
    const res = await run(
      () => (seguindo ? apiClient.delete<Seguimento>(rota) : apiClient.put<Seguimento>(rota)),
      seguindo
        ? "Você não será mais avisado deste fluxo"
        : "Você será avisado das novas versões deste fluxo",
    );
    if (res) await estado.mutate(res.data, { revalidate: false });
  }

  return (
    <Button
      variant={seguindo ? "secondary" : "ghost"}
      size="sm"
      loading={pending}
      aria-pressed={seguindo}
      onClick={() => void alternar()}
    >
      {seguindo ? (
        <>
          <BellOff size={14} aria-hidden="true" className="mr-1" /> Deixar de seguir
        </>
      ) : (
        <>
          <BellRing size={14} aria-hidden="true" className="mr-1" /> Seguir este fluxo
        </>
      )}
    </Button>
  );
}
