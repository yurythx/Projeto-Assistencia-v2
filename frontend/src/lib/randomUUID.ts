/** UUID v4 que funciona também fora de contexto seguro.
 *
 * crypto.randomUUID só existe em HTTPS ou localhost — servido por HTTP num
 * IP da rede (ex.: servidor de teste), quem o chamasse no navegador
 * lançava TypeError (o aceite da LGPD nunca chegava ao servidor e o modal
 * não fechava). getRandomValues existe em qualquer contexto: UUID v4
 * montado à mão (RFC 9562 §5.4). */
export function randomUUID(): string {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6]! & 0x0f) | 0x40; // versão 4
  b[8] = (b[8]! & 0x3f) | 0x80; // variante RFC
  const h = [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}
