"use server";

import { getServerSession } from "next-auth/next";
import { updateTag } from "next/cache";

import { authOptions } from "@/lib/auth/options";
import { BRANDING_TAG } from "@/lib/branding/tag";

/**
 * Chamada depois que o PUT /admin/branding deu certo: expira o cache de
 * GET /branding (60s) para o layout e as páginas públicas já renderizarem
 * a identidade nova — sem isto, o nome antigo voltava a cada navegação até
 * o cache vencer. Server action é um endpoint público: exige sessão (o
 * efeito é inofensivo, mas não é para anônimos forçarem a rebusca).
 */
export async function refreshBranding(): Promise<void> {
  const session = await getServerSession(authOptions);
  if (!session || session.error) return;
  updateTag(BRANDING_TAG);
}
