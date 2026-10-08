"use client";

import { useEffect, useRef, type ReactNode } from "react";

export type ModalSize = "sm" | "md" | "lg" | "xl";

const sizeClass: Record<ModalSize, string> = {
  sm: "max-w-sm",
  md: "max-w-md",
  lg: "max-w-2xl",
  xl: "max-w-3xl",
};

/**
 * Casca de modal sobre o <dialog> nativo. O consumidor controla todo o
 * conteúdo interno (cabeçalho / corpo / rodapé); esta casca só entrega o
 * comportamento correto de graça:
 *
 * - fechar com Esc (evento `cancel` do <dialog>);
 * - fechar clicando no backdrop (o clique no ::backdrop chega como clique
 *   no próprio <dialog>);
 * - captura de foco e fundo `inert` (padrão do <dialog>.showModal());
 * - render no top layer, acima de qualquer z-index;
 * - trava o scroll do body enquanto aberto.
 *
 * Largura = min(100vw-2rem, size) e altura máxima = 100dvh-2rem, com o
 * conteúdo esperado em coluna flex (cabeçalho/rodapé shrink-0, corpo com
 * overflow-y-auto).
 */
export function ModalShell({
  open,
  onClose,
  size = "md",
  labelledBy,
  describedBy,
  className = "",
  variant = "center",
  children,
}: {
  open: boolean;
  onClose: () => void;
  size?: ModalSize;
  labelledBy?: string;
  describedBy?: string;
  className?: string;
  /** "drawer": painel encostado à direita, na altura toda (ex.: assistente). */
  variant?: "center" | "drawer";
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previous;
    };
  }, [open]);

  return (
    <dialog
      ref={ref}
      onClose={onClose}
      onCancel={onClose}
      onClick={(e) => {
        if (e.target === ref.current) onClose();
      }}
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      className={`${
        variant === "drawer"
          ? `my-0 ml-auto mr-0 h-dvh max-h-dvh w-[min(100vw,30rem)] rounded-l-xl`
          : `m-auto w-[calc(100vw-2rem)] ${sizeClass[size]} max-h-[calc(100dvh-2rem)] rounded-xl`
      } overflow-hidden border border-surface-border bg-surface p-0 text-foreground
        shadow-2xl backdrop:bg-black/50 backdrop:backdrop-blur-sm ${className}`}
    >
      <div
        className={`flex flex-col ${variant === "drawer" ? "h-full" : "max-h-[calc(100dvh-2rem)]"}`}
      >
        {children}
      </div>
    </dialog>
  );
}
