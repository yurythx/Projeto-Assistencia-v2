import type { HTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from "react";

// Família de componentes de tabela do kit de UI — encapsula o `<table>`
// num container com `overflow-x-auto` (para tabelas largas não quebrarem
// o layout da página) e aplica a tipografia/espaçamento padrão; nenhuma
// peça aqui sabe o que está sendo listado.
// `caption`: legenda da tabela de dados (eMAG 3.6). Renderizada como
// <caption> só para leitor de tela por padrão (sr-only); passe
// `captionVisible` para exibi-la.
export function Table({
  className = "",
  caption,
  captionVisible = false,
  children,
  ...rest
}: HTMLAttributes<HTMLTableElement> & { caption?: string; captionVisible?: boolean }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-surface-border">
      <table className={`w-full text-left text-sm ${className}`} {...rest}>
        {caption && (
          <caption className={captionVisible ? "px-4 py-2 text-left text-xs text-muted" : "sr-only"}>
            {caption}
          </caption>
        )}
        {children}
      </table>
    </div>
  );
}

export function TableHead({ className = "", ...rest }: HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className={`bg-black/5 dark:bg-white/5 ${className}`} {...rest} />;
}

export function TableBody({ className = "", ...rest }: HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody className={`divide-y divide-surface-border ${className}`} {...rest} />;
}

export function TableRow({ className = "", ...rest }: HTMLAttributes<HTMLTableRowElement>) {
  return <tr className={className} {...rest} />;
}

export function TableHeaderCell({
  className = "",
  ...rest
}: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      scope="col"
      className={`px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-muted ${className}`}
      {...rest}
    />
  );
}

export function TableCell({ className = "", ...rest }: TdHTMLAttributes<HTMLTableCellElement>) {
  return <td className={`px-4 py-3 ${className}`} {...rest} />;
}
