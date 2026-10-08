import "server-only";

import { PublicApiError, publicGet } from "@/lib/api/publicServer";
import type { CatalogService } from "@/lib/nexus/types";

// Leitura do Catálogo de Serviços (plugin "catalog") para o site público.
// Rotas anônimas do backend; um 404 significa módulo desativado no Kernel
// (MODULE_DISABLED) ou serviço inexistente/não publicado.

export async function listPublishedServices(): Promise<CatalogService[]> {
  return (await publicGet<CatalogService[]>("catalog/services?page_size=100", 60)).data;
}

/** Serviço publicado pelo slug; null quando não existe (ou módulo inativo). */
export async function getPublishedService(slug: string): Promise<CatalogService | null> {
  try {
    return (await publicGet<CatalogService>(`catalog/services/${encodeURIComponent(slug)}`, 60)).data;
  } catch (err) {
    if (err instanceof PublicApiError && err.status === 404) return null;
    throw err;
  }
}
