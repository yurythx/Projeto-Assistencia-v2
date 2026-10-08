import type { HTMLAttributes } from "react";

// Selo (badge) genérico do kit de UI — sem regra de negócio própria, só
// aplica a cor de acordo com o "tone" recebido. Usado, por exemplo, para
// destacar status curtos em tabelas/cards.
type Tone = "neutral" | "success" | "danger" | "warning" | "info";

// Cores a partir dos tokens do design system (D-04), não da paleta crua
// do Tailwind — assim seguem o tema e o modo e-MAG de alto contraste.
const toneClasses: Record<Tone, string> = {
  neutral: "bg-black/5 text-foreground dark:bg-white/10",
  success: "bg-success/10 text-success",
  danger: "bg-danger/10 text-danger",
  warning: "bg-warning/10 text-warning",
  info: "bg-accent/10 text-accent",
};

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: Tone;
}

export function Badge({ tone = "neutral", className = "", ...rest }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${toneClasses[tone]} ${className}`}
      {...rest}
    />
  );
}
