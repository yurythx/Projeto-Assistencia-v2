import { ApiError } from "@/lib/api/client";

// Chamadas ANÔNIMAS feitas no navegador (formulário de contato,
// consentimento de visitante) — passam pelo proxy /api/public, que só
// encaminha as rotas públicas da allowlist.
export async function publicPost<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`/api/public/${path.replace(/^\/+/, "")}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  let json: { data: T | null; error: { code: string; message: string } | null };
  try {
    json = await res.json();
  } catch {
    throw new ApiError(res.status, "INVALID_RESPONSE", "resposta ilegível do servidor");
  }
  if (!res.ok || json.error) {
    throw new ApiError(res.status, json.error?.code ?? "UNKNOWN_ERROR", json.error?.message ?? "erro inesperado");
  }
  return json.data as T;
}

export async function publicFetch<T>(path: string): Promise<T> {
  const res = await fetch(`/api/public/${path.replace(/^\/+/, "")}`);
  const json = await res.json();
  if (!res.ok || json.error) {
    throw new ApiError(res.status, json.error?.code ?? "UNKNOWN_ERROR", json.error?.message ?? "erro inesperado");
  }
  return json.data as T;
}
