import type { MetadataRoute } from "next";

import { APP_URL } from "@/lib/env";
import { getFeatures } from "@/lib/features/getFeatures";

// Só páginas públicas. As seções de plug-ins entram apenas quando o
// módulo está ativo no Kernel (GET /system/public-modules) — um módulo
// desativado responde 404 e não deve ser anunciado a crawlers.
const MODULE_PAGES: { path: string; module: string }[] = [
  { path: "/servicos", module: "catalog" },
  { path: "/procedimentos", module: "atlas" },
  { path: "/temporalidade", module: "atlas" },
  { path: "/setores", module: "directory" },
  { path: "/eventos", module: "calendar" },
  { path: "/contato", module: "contact" },
];

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const { features } = await getFeatures();
  const now = new Date();
  return [
    { url: APP_URL, lastModified: now, changeFrequency: "monthly", priority: 1 },
    { url: `${APP_URL}/sobre`, lastModified: now, changeFrequency: "monthly", priority: 0.8 },
    ...MODULE_PAGES.filter((p) => features[p.module]).map((p) => ({
      url: `${APP_URL}${p.path}`,
      lastModified: now,
      changeFrequency: "weekly" as const,
      priority: 0.7,
    })),
    { url: `${APP_URL}/transparencia`, lastModified: now, changeFrequency: "daily", priority: 0.6 },
    {
      url: `${APP_URL}/acessibilidade`,
      lastModified: now,
      changeFrequency: "yearly",
      priority: 0.5,
    },
    { url: `${APP_URL}/privacidade`, lastModified: now, changeFrequency: "yearly", priority: 0.5 },
  ];
}
