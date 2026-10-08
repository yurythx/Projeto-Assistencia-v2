import "server-only";

// IP do visitante para a API Go, que o usa no rate limit, no bloqueio de
// login por IP e na prova de consentimento LGPD (e só confia no
// X-Forwarded-For vindo de TRUSTED_PROXIES — a rede interna, onde este
// servidor Next está).
//
// Só repassamos o cabeçalho quando há um proxy de borda confiável NA
// FRENTE deste servidor, que o sobrescreve com o IP real
// (TRUST_PROXY_HEADERS=true, ligado pelo docker-compose.https.yml — o
// Caddy faz isso). Sem proxy, o X-Forwarded-For é o que o cliente quiser
// mandar: repassá-lo deixava forjar o IP e escapar dos limites por IP
// (achado testando o formulário de contato pela porta direta).
//
// Sem ele, porém, todo mundo chega à API com o IP deste servidor — os
// limites por IP viram um balde só. Por isso a implantação deve sempre ter
// o proxy de borda (docs/DEPLOY.md, HTTPS).

type HeaderSource = Headers | Record<string, string | string[] | undefined> | undefined;

function read(h: HeaderSource, name: string): string | undefined {
  if (!h) return undefined;
  if (typeof (h as Headers).get === "function") return (h as Headers).get(name) ?? undefined;
  const v = (h as Record<string, string | string[] | undefined>)[name];
  return Array.isArray(v) ? v.join(", ") : v;
}

/** Cabeçalho X-Forwarded-For para repassar à API, ou nada. */
export function forwardedFor(h: HeaderSource): Record<string, string> {
  if (process.env.TRUST_PROXY_HEADERS !== "true") return {};
  const xff = read(h, "x-forwarded-for")?.trim();
  return xff ? { "X-Forwarded-For": xff } : {};
}
