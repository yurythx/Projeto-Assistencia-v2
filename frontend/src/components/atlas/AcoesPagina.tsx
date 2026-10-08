"use client";

import { Bot, Printer } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/Button";

import { useAssistente } from "./AssistenteGaveta";

/** Ações comuns das páginas de detalhe: imprimir e perguntar ao assistente
 * com o contexto da página já preenchido. */
export function AcoesPagina({
  pergunta,
  rotulo,
  children,
}: {
  pergunta: string;
  rotulo: string;
  children?: ReactNode;
}) {
  const { disponivel, abrir } = useAssistente();
  return (
    <div className="flex flex-wrap gap-2 print:hidden">
      <Button variant="secondary" size="sm" onClick={() => window.print()}>
        <Printer size={14} aria-hidden="true" /> Imprimir
      </Button>
      {disponivel && (
        <Button variant="secondary" size="sm" onClick={() => abrir(pergunta)}>
          <Bot size={14} aria-hidden="true" /> {rotulo}
        </Button>
      )}
      {children}
    </div>
  );
}
