export type ToastTone = "info" | "success" | "danger";

export interface ToastData {
  id: string;
  title: string;
  description?: string;
  tone: ToastTone;
}

// Borda-acento a partir dos tokens de status do design system (D-04),
// não cores cruas do Tailwind.
const toneBorder: Record<ToastTone, string> = {
  info: "border-l-accent",
  success: "border-l-status-online",
  danger: "border-l-status-offline",
};

export function Toast({
  toast,
  onDismiss,
  onPause,
  onResume,
}: {
  toast: ToastData;
  onDismiss: (id: string) => void;
  /** A-16: pausa o auto-dismiss enquanto o mouse ou o foco de teclado
   * estiverem sobre o toast (WCAG 2.2.1 Timing Adjustable) — sem isso, um
   * usuário que precise de mais tempo pra ler ou alcançar o botão "✕"
   * pode perder a notificação antes de conseguir agir. */
  onPause: (id: string) => void;
  onResume: (id: string) => void;
}) {
  return (
    // w-[calc(100vw-2rem)] max-w-80 — § revisão de mobile 2026-08: um
    // toast de 320px fixos (w-80) ancorado a 1rem da borda direita
    // (ToastProvider) passa da borda ESQUERDA de qualquer tela com menos
    // de 336px de largura — comum em celulares mais antigos/estreitos.
    // Limitado à viewport menos a margem em telas pequenas, sem perder o
    // tamanho de 320px de antes em telas maiores.
    <div
      onMouseEnter={() => onPause(toast.id)}
      onMouseLeave={() => onResume(toast.id)}
      onFocus={() => onPause(toast.id)}
      onBlur={() => onResume(toast.id)}
      className={`w-[calc(100vw-2rem)] max-w-80 rounded-lg border border-surface-border border-l-4 bg-surface p-3 shadow-md ${toneBorder[toast.tone]}`}
    >
      <div className="flex items-start justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-foreground">{toast.title}</p>
          {toast.description && <p className="mt-0.5 text-xs text-muted">{toast.description}</p>}
        </div>
        <button
          type="button"
          onClick={() => onDismiss(toast.id)}
          aria-label="Dispensar notificação"
          className="shrink-0 rounded p-1 text-muted hover:bg-black/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:hover:bg-white/10"
        >
          <span aria-hidden="true">✕</span>
        </button>
      </div>
    </div>
  );
}
