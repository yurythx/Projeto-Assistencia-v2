"use client";

import { useId, type ReactNode } from "react";

import { ModalShell, type ModalSize } from "@/components/ui/ModalShell";

export interface DialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  /** Largura máxima do modal. `md` (padrão) serve pra confirmações; `lg`/`xl`
   *  pra modais com formulário, tabela ou lista. */
  size?: ModalSize;
  /** Rodapé fixo (não rola com o corpo) — ex.: botões de ação. O modal já
   *  põe o fio de separação e o espaçamento; o alinhamento fica com quem
   *  chama (ex.: `ml-auto` num botão pra jogá-lo à direita). */
  footer?: ReactNode;
  children?: ReactNode;
}

/**
 * Modal estruturado (título + descrição + corpo rolável + rodapé fixo)
 * sobre a ModalShell — que dá Esc, clique-no-backdrop, captura de foco,
 * fundo `inert` e trava de scroll de graça, via <dialog> nativo.
 */
export function Dialog({
  open,
  onClose,
  title,
  description,
  size = "md",
  footer,
  children,
}: DialogProps) {
  // ids únicos por instância (A-09) — permite dois <Dialog> montados sem
  // colisão de aria-labelledby/aria-describedby.
  const titleId = useId();
  const descId = useId();
  return (
    <ModalShell
      open={open}
      onClose={onClose}
      size={size}
      labelledBy={titleId}
      describedBy={description ? descId : undefined}
    >
      <div className="flex max-h-[calc(100dvh-2rem)] flex-col p-5">
        <h2 id={titleId} className="shrink-0 text-base font-semibold">
          {title}
        </h2>
        {description && (
          <p id={descId} className="mt-1 shrink-0 text-sm text-muted">
            {description}
          </p>
        )}
        <div className="mt-4 min-h-0 flex-1 overflow-y-auto">{children}</div>
        {footer && (
          <div className="mt-4 flex shrink-0 flex-wrap items-center gap-2 border-t border-surface-border pt-4">
            {footer}
          </div>
        )}
      </div>
    </ModalShell>
  );
}
