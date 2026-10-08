"use client";

import { useState, type ReactNode } from "react";

import { Button, type ButtonProps } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";

/** Botão que pede confirmação num diálogo modal acessível antes de
 * executar uma ação destrutiva (nunca window.confirm). */
export function ConfirmButton({
  title,
  description,
  confirmLabel = "Confirmar",
  onConfirm,
  children,
  variant = "ghost",
  size = "sm",
  ...rest
}: Omit<ButtonProps, "onClick"> & {
  title: string;
  description?: string;
  confirmLabel?: string;
  onConfirm: () => Promise<unknown> | void;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  async function confirm() {
    setBusy(true);
    try {
      await onConfirm();
      setOpen(false);
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Button type="button" variant={variant} size={size} onClick={() => setOpen(true)} {...rest}>
        {children}
      </Button>
      <Dialog
        open={open}
        onClose={() => setOpen(false)}
        title={title}
        description={description}
        footer={
          <>
            <Button variant="secondary" onClick={() => setOpen(false)}>
              Cancelar
            </Button>
            <Button variant="danger" className="ml-auto" loading={busy} onClick={() => void confirm()}>
              {confirmLabel}
            </Button>
          </>
        }
      />
    </>
  );
}
