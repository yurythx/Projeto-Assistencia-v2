import type { SectionTab } from "@/components/layout/SectionTabs";

// Sub-navegação por seção — uma fonte de verdade para módulos e sub-páginas.

export interface GuardedTab extends SectionTab {
  /** Permissão exigida para exibir a aba (a API revalida sempre — A01). */
  permission?: string;
  /** Módulo do Kernel que precisa estar ativo. */
  module?: string;
  /** Só com a permissão numa concessão global (ADR 013). */
  global?: boolean;
}

export const CONFIG_TABS: GuardedTab[] = [
  { href: "/configuracao", label: "Identidade visual", permission: "branding:manage" },
  { href: "/configuracao/modulos", label: "Módulos", permission: "modules:manage" },
  { href: "/configuracao/organizacao", label: "Estrutura organizacional", permission: "iam:manage" },
  { href: "/configuracao/perfis", label: "Perfis de acesso", permission: "iam:manage", global: true },
  { href: "/configuracao/mapeamento-ad", label: "Mapeamento AD", permission: "iam:manage", global: true },
  { href: "/configuracao/usuarios", label: "Usuários", permission: "users:read" },
  { href: "/configuracao/keycloak", label: "Keycloak (IAM)", permission: "keycloak:manage" },
  { href: "/configuracao/ia", label: "Inteligência artificial", permission: "ia:manage" },
  { href: "/configuracao/egress", label: "Egress & Webhooks", permission: "egress:manage", module: "egress" },
];

export function visibleTabs(
  tabs: GuardedTab[],
  can: (p: string) => boolean,
  enabled: (m: string) => boolean,
  canGlobal: (p: string) => boolean = can,
): SectionTab[] {
  return tabs.filter(
    (t) => (!t.permission || (can(t.permission) && (!t.global || canGlobal(t.permission)))) && (!t.module || enabled(t.module)),
  );
}
