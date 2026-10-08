"use client";

import { LogOut, UserRound } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { fullSignOut } from "@/lib/auth/logout";

function initialsFrom(label: string): string {
  const parts = label.replace(/@.*/, "").split(/[.\s_-]+/).filter(Boolean);
  const chars = parts.length >= 2 ? [parts[0]?.[0] ?? "?", parts[1]?.[0] ?? "?"] : [label[0] ?? "?"];
  return chars.join("").toUpperCase();
}

// Menu do usuário no canto superior direito (§ Redesenho de layout):
// avatar com iniciais + dropdown com o rótulo do usuário e "Sair". O
// mesmo popover hand-rolled (useState + useRef + click-fora/Escape) já
// usado em NotificationBell — nenhuma biblioteca de UI-kit nova para
// isto, consistente com o resto deste kit de componentes.
export function UserMenu({ userLabel }: { userLabel: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: PointerEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Menu do usuário ${userLabel}`}
        className="flex h-10 w-10 items-center justify-center rounded-full bg-primary text-xs font-semibold text-primary-foreground"
      >
        {initialsFrom(userLabel)}
      </button>

      {open && (
        // max-w-[calc(100vw-2rem)] — mesma folga de segurança de
        // NotificationBell, pro caso raro de userLabel ser um e-mail
        // longo demais numa tela muito estreita.
        <div
          role="menu"
          className="absolute right-0 top-11 z-50 w-56 max-w-[calc(100vw-2rem)] rounded-md border border-surface-border bg-surface py-1 shadow-lg"
        >
          <div className="truncate border-b border-surface-border px-3 py-2 text-sm text-muted">
            {userLabel}
          </div>
          <Link
            href="/perfil"
            role="menuitem"
            onClick={() => setOpen(false)}
            className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-foreground hover:bg-black/5 dark:hover:bg-white/5"
          >
            <UserRound size={15} aria-hidden="true" />
            Meu perfil e privacidade
          </Link>
          <button
            type="button"
            role="menuitem"
            onClick={() => void fullSignOut()}
            className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-foreground hover:bg-black/5 dark:hover:bg-white/5"
          >
            <LogOut size={15} aria-hidden="true" />
            Sair
          </button>
        </div>
      )}
    </div>
  );
}
