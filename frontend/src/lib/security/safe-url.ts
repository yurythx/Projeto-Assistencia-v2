// Allowlist de esquema para URLs fornecidas pelo usuário que acabam num
// atributo do DOM (img[src], link[href], a[href]). Nada aqui "sanitiza"
// conteúdo — só recusa esquemas perigosos (javascript:, data:, blob:,
// vbscript:, file:) antes que a URL vire src/href.
//
// Usado pela identidade visual white-label: `branding.logoUrl` vem de um
// input de texto livre (BrandingSettingsForm), é persistido em cookie e
// reaproveitado como <img src> na Topbar/preview — precisa passar por aqui
// (S-03 da auditoria).

/**
 * Retorna a URL normalizada se usar um esquema seguro para carregar um
 * recurso (por padrão só `https:`), ou null caso contrário. Rejeita
 * entradas relativas de propósito — espera-se uma URL absoluta de asset
 * externo.
 */
export function safeResourceUrl(
  raw: string | null | undefined,
  { allowHttp = false }: { allowHttp?: boolean } = {},
): string | null {
  if (!raw) return null;
  const trimmed = raw.trim();
  if (!trimmed) return null;
  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return null;
  }
  const allowed = allowHttp ? ["https:", "http:"] : ["https:"];
  return allowed.includes(parsed.protocol) ? parsed.toString() : null;
}
