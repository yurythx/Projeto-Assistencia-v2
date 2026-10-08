import "server-only";

import { BACKEND_INTERNAL_URL } from "@/lib/env";

// Busca de dados PÚBLICOS em Server Components (site institucional:
// catálogo, setores, eventos públicos, branding, verificação de
// assinatura). Sem token — são as rotas anônimas do backend.

export class PublicApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "PublicApiError";
  }
}

interface Envelope<T> {
  data: T | null;
  error: { code: string; message: string } | null;
  meta?: unknown;
}

export async function publicGet<T>(path: string, revalidateSeconds = 60, tags?: string[]): Promise<{ data: T; meta?: unknown }> {
  const url = new URL(`/api/v1/${path.replace(/^\/+/, "")}`, BACKEND_INTERNAL_URL);
  let res: Response;
  try {
    res = await fetch(url, { next: { revalidate: revalidateSeconds, ...(tags ? { tags } : {}) }, signal: AbortSignal.timeout(8000) });
  } catch {
    throw new PublicApiError(503, "DEPENDENCY_UNAVAILABLE", "serviço indisponível no momento");
  }
  let json: Envelope<T>;
  try {
    json = await res.json();
  } catch {
    throw new PublicApiError(res.status, "INVALID_RESPONSE", "resposta ilegível do servidor");
  }
  if (!res.ok || json.error || json.data === null) {
    throw new PublicApiError(res.status, json.error?.code ?? "UNKNOWN_ERROR", json.error?.message ?? "erro inesperado");
  }
  return { data: json.data, meta: json.meta };
}

/** Variante tolerante: devolve fallback quando o módulo está desativado
 * (404 MODULE_DISABLED) ou a API está fora — o site público degrada em
 * vez de quebrar. */
export async function publicGetOr<T>(path: string, fallback: T, revalidateSeconds = 60, tags?: string[]): Promise<T> {
  try {
    return (await publicGet<T>(path, revalidateSeconds, tags)).data;
  } catch {
    return fallback;
  }
}
