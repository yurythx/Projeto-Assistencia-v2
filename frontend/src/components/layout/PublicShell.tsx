import type { ReactNode } from "react";

import { PublicShellClient } from "@/components/layout/PublicShellClient";
import { getFeatures } from "@/lib/features/getFeatures";

/** Shell do site institucional. Server Component: resolve o estado dos
 * módulos no servidor para o menu já sair certo no 1º HTML (sem piscar
 * links de plugins desativados). API fora do ar = estado desconhecido:
 * o cliente decide pelo próprio fetch. */
export async function PublicShell({ children }: { children: ReactNode }) {
  const { features } = await getFeatures();
  const initialEnabled = Object.keys(features).length > 0 ? features : null;
  return <PublicShellClient initialEnabled={initialEnabled}>{children}</PublicShellClient>;
}
