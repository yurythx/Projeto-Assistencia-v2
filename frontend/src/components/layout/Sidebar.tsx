"use client";

import { Activity, LayoutDashboard, Settings, ShieldCheck, type LucideProps } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, type ComponentType } from "react";

import { useBranding } from "@/components/branding/BrandingContext";
import { ModuleIcon } from "@/components/layout/ModuleIcon";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ModuleStatus } from "@/lib/nexus/types";

interface NavItem {
  href: string;
  label: string;
  icon: ComponentType<LucideProps> | string;
}

// Módulos que não entram no menu de "Módulos": o núcleo tem entradas
// próprias em Administração; Egress é configuração; a Busca vive no
// cabeçalho (#global-search).
const HIDDEN_FROM_MENU = new Set(["iam", "audit", "egress", "search"]);

// Módulos cuja tela principal é de gestão (a face pública fica no site
// institucional): só aparecem no menu para quem tem a permissão.
const MENU_PERMISSION: Record<string, string> = {
  catalog: "catalog:manage",
  contact: "contact:read",
};

const ADMIN_PERMISSIONS = [
  "modules:manage", "iam:manage", "users:read", "branding:manage", "keycloak:manage", "ia:manage",
  "egress:manage", "catalog:manage", "contact:read", "mercurio:manage", "calendar:manage",
];

/** Monta o menu a partir do estado dos módulos no Kernel e das permissões
 * efetivas — um plugin desativado some do menu sem recarregar a página. */
export function buildNav(modules: ModuleStatus[], can: (p: string) => boolean): { modules: NavItem[]; admin: NavItem[] } {
  const mods = modules
    .filter((m) => m.enabled && !m.core && !HIDDEN_FROM_MENU.has(m.key) && m.route)
    .filter((m) => !MENU_PERMISSION[m.key] || can(MENU_PERMISSION[m.key]!))
    .map((m) => ({ href: m.route, label: m.name, icon: m.icon }));
  const admin: NavItem[] = [];
  if (can("audit:read")) admin.push({ href: "/auditoria", label: "Auditoria", icon: ShieldCheck });
  if (can("monitoring:read")) admin.push({ href: "/monitoramento", label: "Monitoramento", icon: Activity });
  if (ADMIN_PERMISSIONS.some(can)) admin.push({ href: "/configuracao", label: "Configurações", icon: Settings });
  return { modules: mods, admin };
}

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(href + "/");
}

export function Sidebar({
  collapsed,
  mobileOpen,
  onCloseMobile,
}: {
  collapsed: boolean;
  mobileOpen: boolean;
  onCloseMobile: () => void;
}) {
  const pathname = usePathname();
  const { branding } = useBranding();
  const { modules, can } = useNexus();
  const nav = buildNav(modules, can);

  useEffect(() => {
    if (!mobileOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onCloseMobile();
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [mobileOpen, onCloseMobile]);

  const renderItem = (item: NavItem) => {
    const active = isActive(pathname, item.href);
    return (
      <li key={item.href}>
        <Link
          href={item.href}
          onClick={onCloseMobile}
          aria-current={active ? "page" : undefined}
          title={collapsed ? item.label : undefined}
          className={`flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
            active ? "bg-primary/10 text-primary" : "text-foreground hover:bg-surface-hover"
          } ${collapsed ? "md:justify-center" : ""}`}
        >
          {typeof item.icon === "string" ? (
            <ModuleIcon name={item.icon} size={18} aria-hidden="true" className="shrink-0" />
          ) : (
            <item.icon size={18} aria-hidden="true" className="shrink-0" />
          )}
          <span className={collapsed ? "md:sr-only" : ""}>{item.label}</span>
        </Link>
      </li>
    );
  };

  return (
    <>
      {mobileOpen && (
        <div className="fixed inset-x-0 bottom-0 top-[var(--topbar-h)] z-40 bg-black/40 md:hidden" onClick={onCloseMobile} aria-hidden="true" />
      )}
      <nav
        id="main-menu"
        tabIndex={-1}
        aria-label="Menu principal"
        className={`fixed bottom-0 left-0 top-[var(--topbar-h)] z-40 flex w-72 flex-col overflow-y-auto border-r border-surface-border bg-surface outline-none transition-[transform,width] duration-[var(--shell-motion)] md:translate-x-0 ${
          collapsed ? "md:w-[var(--sidebar-w-collapsed)]" : "md:w-[var(--sidebar-w)]"
        } ${mobileOpen ? "translate-x-0" : "-translate-x-full"}`}
      >
        <ul className="flex flex-col gap-0.5 px-2 pt-3">{renderItem({ href: "/dashboard", label: "Visão geral", icon: LayoutDashboard })}</ul>

        {nav.modules.length > 0 && (
          <div className="mt-3">
            <p className={`px-4 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted ${collapsed ? "md:sr-only" : ""}`}>Módulos</p>
            <ul className="flex flex-col gap-0.5 px-2">{nav.modules.map(renderItem)}</ul>
          </div>
        )}

        {nav.admin.length > 0 && (
          <div className="mt-3">
            <p className={`px-4 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted ${collapsed ? "md:sr-only" : ""}`}>Administração</p>
            <ul className="flex flex-col gap-0.5 px-2 pb-3">{nav.admin.map(renderItem)}</ul>
          </div>
        )}

        <p className={`mt-auto border-t border-surface-border px-4 py-3 text-[11px] text-muted ${collapsed ? "md:sr-only" : ""}`}>
          {branding.appName}
        </p>
      </nav>
    </>
  );
}
