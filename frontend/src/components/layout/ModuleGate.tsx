"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { PowerOff } from "lucide-react";
import type { ReactNode } from "react";

import { moduleNames } from "@/lib/modules/plan";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ModuleStatus } from "@/lib/nexus/types";

/** Módulo dono da rota atual: o de rota (Manifest.Route) mais específica
 * que prefixa o caminho. Núcleo e rotas de configuração não são guardadas
 * aqui (Configurações tem abas próprias por permissão). */
export function moduleForPath(modules: ModuleStatus[], pathname: string): ModuleStatus | undefined {
  return modules
    .filter((m) => !m.core && m.route && !m.route.startsWith("/configuracao") && (pathname === m.route || pathname.startsWith(m.route + "/")))
    .sort((a, b) => b.route.length - a.route.length)[0];
}

/**
 * Guarda de página: com o plug-in desativado no Kernel, a tela explica o
 * motivo em vez de exibir erros de API (que respondem 404 MODULE_DISABLED).
 * Reage em runtime: o estado dos módulos é revalidado pelo NexusProvider.
 */
export function ModuleGate({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const { modules, can } = useNexus();
  const mod = moduleForPath(modules, pathname);
  if (!mod || mod.enabled) return <>{children}</>;

  const blocked = mod.configured && mod.blocked_by.length > 0;
  return (
    <div role="status" className="mx-auto mt-10 flex max-w-lg flex-col items-center gap-3 rounded-xl border border-dashed border-surface-border p-10 text-center">
      <PowerOff size={32} aria-hidden="true" className="text-muted" />
      <h1 className="text-xl font-semibold">{mod.name} está desativado</h1>
      <p className="text-sm text-muted">
        {blocked
          ? `O módulo está configurado como ativo, mas depende de ${moduleNames(modules, mod.blocked_by)}, que está desativado.`
          : "Este módulo foi desativado pela administração da plataforma e está temporariamente indisponível."}
      </p>
      {can("modules:manage") && (
        <Link href="/configuracao/modulos" className="text-sm font-semibold text-primary hover:underline">
          Gerenciar módulos →
        </Link>
      )}
    </div>
  );
}
