import { type TextareaHTMLAttributes, forwardRef } from "react";

// Área de texto do kit de UI — espelha o <Input>: label vinculada por
// htmlFor/id, erro acessível (aria-invalid/aria-describedby) e o MESMO
// indicador de foco (focus-visible:outline-2). Existe para os formulários
// pararem de usar <textarea> cru com `focus:outline-none` sem substituto
// (A-03 da auditoria).
export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { label, error, id, className = "", ...rest },
  ref,
) {
  const fieldId = id ?? rest.name;
  return (
    <div className="flex flex-col gap-1">
      {label && (
        <label htmlFor={fieldId} className="text-sm font-medium text-foreground">
          {label}
        </label>
      )}
      <textarea
        ref={ref}
        id={fieldId}
        className={`rounded-md border border-surface-border bg-surface px-3 py-2 text-sm text-foreground
          placeholder:text-muted focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary
          disabled:opacity-50 ${error ? "border-danger" : ""} ${className}`}
        aria-invalid={error ? true : undefined}
        aria-describedby={error && fieldId ? `${fieldId}-error` : undefined}
        {...rest}
      />
      {error && (
        <p
          id={fieldId ? `${fieldId}-error` : undefined}
          role="alert"
          className="text-xs text-danger"
        >
          {error}
        </p>
      )}
    </div>
  );
});
