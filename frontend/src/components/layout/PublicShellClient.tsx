"use client";

import Link from "next/link";
import { useSession } from "next-auth/react";
import { Suspense, useEffect, useState, type ReactNode } from "react";

import { AuthFlashToast } from "@/components/layout/AuthFlashToast";
import { GovFooter } from "@/components/layout/GovFooter";
import { GovHeader } from "@/components/layout/GovHeader";
import { LGPDConsentModal } from "@/components/layout/LGPDConsentModal";
import { ToastProvider } from "@/components/notifications/ToastProvider";
import { Button } from "@/components/ui/Button";
import { ThemeToggle } from "@/components/ui/ThemeToggle";
import { publicFetch } from "@/lib/api/publicClient";
import type { PublicModule } from "@/lib/nexus/types";

const PUBLIC_LINKS: { href: string; label: string; module?: string }[] = [
  { href: "/servicos", label: "Serviços", module: "catalog" },
  { href: "/procedimentos", label: "Procedimentos", module: "atlas" },
  { href: "/temporalidade", label: "Temporalidade", module: "atlas" },
  { href: "/setores", label: "Setores", module: "directory" },
  { href: "/eventos", label: "Agenda", module: "calendar" },
  { href: "/contato", label: "Contato", module: "contact" },
  { href: "/sobre", label: "Sobre" },
];

/** Shell do site institucional (visitante anônimo). Os links de plugins
 * desativados no Kernel somem do menu.
 *
 * initialEnabled vem do servidor (PublicShell) já no 1º HTML. Antes o
 * estado só chegava por fetch no navegador e, enquanto isso, TODOS os
 * links eram mostrados — a cada F5 os plugins desativados piscavam no
 * menu. Sem estado conhecido, links de plugin ficam escondidos; o fetch
 * aqui só atualiza um estado do servidor em cache (até 15s). */
export function PublicShellClient({
  children,
  initialEnabled = null,
}: {
  children: ReactNode;
  initialEnabled?: Record<string, boolean> | null;
}) {
  const { status } = useSession();
  const [enabled, setEnabled] = useState<Record<string, boolean> | null>(initialEnabled);

  useEffect(() => {
    publicFetch<PublicModule[]>("v1/system/public-modules")
      .then((mods) => setEnabled(Object.fromEntries(mods.map((m) => [m.key, m.enabled]))))
      .catch(() => setEnabled((prev) => prev ?? {}));
  }, []);

  const links = PUBLIC_LINKS.filter((l) => !l.module || Boolean(enabled?.[l.module]));

  return (
    <ToastProvider>
      <div className="flex min-h-screen flex-col pt-[var(--topbar-h)]">
        <Suspense fallback={null}>
          <AuthFlashToast />
        </Suspense>
        <GovHeader
          homeHref="/"
          nav={
            <nav
              id="main-menu"
              tabIndex={-1}
              aria-label="Menu principal"
              className="ml-4 hidden outline-none lg:block"
            >
              <ul className="flex items-center gap-5 text-sm">
                {links.map((l) => (
                  <li key={l.href}>
                    <Link
                      href={l.href}
                      className="text-muted transition-colors hover:text-foreground"
                    >
                      {l.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </nav>
          }
          actions={
            <>
              <ThemeToggle />
              <Link href={status === "authenticated" ? "/dashboard" : "/login"}>
                <Button size="sm">
                  {status === "authenticated" ? "Acessar o painel" : "Entrar"}
                </Button>
              </Link>
            </>
          }
        />
        <main id="main-content" tabIndex={-1} className="flex-1 outline-none">
          {children}
        </main>
        <GovFooter />
        <LGPDConsentModal />
      </div>
    </ToastProvider>
  );
}
