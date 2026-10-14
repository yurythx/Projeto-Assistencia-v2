"use client";

import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { PageHeader } from "@/components/nexus/PageHeader";
import { Painel } from "@/components/sempras/Painel";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P } from "@/lib/sempras/types";

/** Painel de gestão do SEMPRAS (sempras:gestao — ADR 030): números do dia de
 * todas as unidades, para o Superintendente e a Administração. Rota própria
 * (antes, uma seção dentro de /sempras) para caber na árvore da sidebar e
 * não misturar a tela operacional da recepção com a visão gerencial. */
export default function PainelPage() {
  const { can } = useNexus();
  if (!can(P.gestao)) {
    return (
      <p className="text-sm text-muted">
        O painel de gestão é da gestão do SEMPRAS (permissão {P.gestao}).
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-6">
      <Link href="/sempras" className="inline-flex items-center gap-1 text-sm text-primary underline">
        <ArrowLeft size={14} aria-hidden="true" /> Voltar ao atendimento
      </Link>
      <PageHeader
        eyebrow="Assistência Social"
        title="SEMPRAS — Painel de gestão"
        description="Números do dia de todas as unidades: fila, atendimentos em curso, concluídos e espera média."
      />
      <Painel />
    </div>
  );
}
