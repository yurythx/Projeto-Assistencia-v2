import type { HTMLAttributes } from "react";

// Família de componentes de cartão do kit de UI — puramente estrutural
// (layout/tipografia), sem regra de negócio; cada peça (Header/Title/
// Description/Content) é opcional e composta pelo consumidor conforme a
// necessidade de cada tela.
export function Card({ className = "", ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={`rounded-xl border border-surface-border bg-surface shadow-sm ${className}`}
      {...rest}
    />
  );
}

export function CardHeader({ className = "", ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={`px-5 pt-5 ${className}`} {...rest} />;
}

// `as` permite ajustar o nível do cabeçalho para não pular níveis (A-02):
// h3 é o padrão (Card dentro de uma seção com h2), mas quando o Card é a
// primeira subdivisão sob um h1 use `as="h2"`.
export function CardTitle({
  as: Tag = "h3",
  className = "",
  ...rest
}: HTMLAttributes<HTMLHeadingElement> & { as?: "h2" | "h3" | "h4" }) {
  return <Tag className={`text-sm font-semibold text-foreground ${className}`} {...rest} />;
}

export function CardDescription({ className = "", ...rest }: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={`text-sm text-muted ${className}`} {...rest} />;
}

export function CardContent({ className = "", ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={`px-5 pb-5 pt-3 ${className}`} {...rest} />;
}
