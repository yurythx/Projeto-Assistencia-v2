"use client";

import type { ReactNode } from "react";

import { SectionTabs } from "@/components/layout/SectionTabs";
import { CONFIG_TABS, visibleTabs } from "@/lib/nav/sectionTabs";
import { useNexus } from "@/lib/nexus/NexusProvider";

// Sub-navegação de /configuracao: cada aba só aparece para quem tem a
// permissão correspondente (e, no caso do Egress, com o plugin ativo).
// Esconder a aba é só UX — cada rota da API revalida a permissão (A01).
export default function ConfiguracaoLayout({ children }: { children: ReactNode }) {
  const { can, canGlobal, enabled } = useNexus();
  const tabs = visibleTabs(CONFIG_TABS, can, enabled, canGlobal);

  return (
    <div className="flex flex-col gap-6">
      <div>
        <p className="dateline">Administração</p>
        <h1 className="mt-2 text-2xl font-semibold">Configurações</h1>
        <p className="mt-1 text-sm text-muted">
          Identidade visual, módulos do Kernel, estrutura organizacional, controle de acesso e integrações.
        </p>
      </div>
      {tabs.length > 0 && <SectionTabs tabs={tabs} ariaLabel="Seções de configuração" />}
      {children}
    </div>
  );
}
