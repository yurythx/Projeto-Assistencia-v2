import "server-only";

import { publicGetOr } from "@/lib/api/publicServer";
import type { PublicModule } from "@/lib/nexus/types";

/** Estado dos módulos com superfície pública (Kernel), para o site
 * institucional esconder seções de plugins desativados. Revalida a cada
 * 15s: desativar um módulo reflete no site público quase na hora. */
export async function getFeatures(): Promise<{ features: Record<string, boolean> }> {
  const modules = await publicGetOr<PublicModule[]>("system/public-modules", [], 15);
  const features: Record<string, boolean> = {};
  for (const m of modules) features[m.key] = m.enabled;
  return { features };
}
