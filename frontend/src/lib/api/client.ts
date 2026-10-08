// Cliente tipado para Client Components — sempre chama o proxy BFF na
// mesma origem (/api/backend/*), nunca a API Go diretamente, para que o
// bearer token nunca precise chegar ao JavaScript executado no navegador
// (§30).

import type { ZodType } from "zod";

import { APP_URL } from "@/lib/env";

interface ErrorBody {
  code: string;
  message: string;
}

interface Envelope<T> {
  data: T | null;
  error: ErrorBody | null;
  meta?: unknown;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

async function request<T>(
  path: string,
  init?: RequestInit,
  schema?: ZodType<T>,
): Promise<{ data: T; meta?: unknown }> {
  let cleanPath = path.startsWith("/") ? path.slice(1) : path;
  if (cleanPath.startsWith("api/")) {
    cleanPath = cleanPath.slice(4);
  }
  if (cleanPath.startsWith("backend/")) {
    cleanPath = cleanPath.slice(8);
  }
  const isFormData = typeof FormData !== "undefined" && init?.body instanceof FormData;
  // No browser é same-origin (""); em SSR precisa da origem absoluta da app
  // (parametrizada em lib/env.ts, default acompanhando a porta atual).
  const baseUrl = typeof window !== "undefined" ? "" : APP_URL;

  // Toda mutação leva uma X-Idempotency-Key própria (skill §1 / A08): um
  // reenvio da MESMA requisição (retry de rede, proxy, duplo clique que
  // escapou do estado de loading) é deduplicado no backend.
  const method = (init?.method ?? "GET").toUpperCase();
  const idempotency: Record<string, string> =
    method !== "GET" && method !== "HEAD" && typeof crypto !== "undefined" && "randomUUID" in crypto
      ? { "X-Idempotency-Key": crypto.randomUUID() }
      : {};

  const res = await fetch(`${baseUrl}/api/backend/${cleanPath}`, {
    ...init,
    headers: {
      ...(isFormData ? {} : { "Content-Type": "application/json" }),
      ...idempotency,
      ...(init?.headers ?? {}),
    },
  });

  let json: Envelope<T> | null = null;
  try {
    json = await res.json();
  } catch {
    json = null;
  }

  // 401 de SESSÃO (proxy BFF ou middleware do backend, código
  // UNAUTHORIZED) = sessão morta/expirada (ver lib/auth/tokenState.ts).
  // Sem isto, uma tela com polling SWR só acumulava erros silenciosos e o
  // usuário nunca era levado de volta ao login. Um 401 com código PRÓPRIO
  // é recusa de negócio com a sessão viva — ex.: senha errada na
  // cerimônia de assinatura do Signum (SIGNUM_REAUTH_FAILED): redirecionar
  // derrubava a sessão por um erro de digitação. RBAC negado usa 403.
  // Navegação DURA de propósito (location.replace, não router.push):
  // descarta todo o estado de cliente da tela expirada e reexecuta o
  // middleware; replace() não deixa a tela morta no histórico do back.
  const sessionExpired = res.status === 401 && (!json?.error || json.error.code === "UNAUTHORIZED");
  if (sessionExpired && typeof window !== "undefined" && window.location.pathname !== "/login") {
    const target = new URL("/login", window.location.origin);
    target.searchParams.set("callbackUrl", window.location.pathname + window.location.search);
    // Este 401 só acontece pra quem já tinha uma sessão (o proxy BFF exige
    // token pra sequer tentar a chamada) — sempre "sessão expirou", nunca
    // "nunca logou" (ver o mesmo aviso em LoginCard.tsx).
    target.searchParams.set("reason", "session_expired");
    window.location.replace(target.toString());
  }

  if (json === null) {
    throw new ApiError(
      res.status,
      "INVALID_RESPONSE",
      "O servidor retornou uma resposta ilegível.",
    );
  }

  if (!res.ok || json.error) {
    throw new ApiError(
      res.status,
      json.error?.code ?? "UNKNOWN_ERROR",
      json.error?.message ?? "Algo deu errado. Tente novamente.",
    );
  }

  // Validação de contrato (S-07): idêntica à de lib/api/server.ts.
  if (schema) {
    const parsed = schema.safeParse(json.data);
    if (!parsed.success) {
      throw new ApiError(
        502,
        "INVALID_RESPONSE",
        "A resposta da API não corresponde ao formato esperado.",
      );
    }
    return { data: parsed.data, meta: json.meta };
  }

  return { data: json.data as T, meta: json.meta };
}

export const apiClient = {
  get: <T>(path: string, schema?: ZodType<T>) => request<T>(path, { method: "GET" }, schema),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "POST",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
  // postForm: caminho separado de post() pra um corpo multipart/form-data
  // (ex.: upload de arquivo) — nunca passa por JSON.stringify, e nunca
  // define Content-Type manualmente: o browser precisa gerar esse header
  // sozinho a partir do FormData, incluindo o boundary multipart, que
  // request() (Content-Type: application/json fixo) sobrescreveria e
  // quebraria a requisição inteira.
  postForm: <T>(path: string, form: FormData) =>
    request<T>(path, { method: "POST", body: form, headers: {} }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "PATCH",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "PUT",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
