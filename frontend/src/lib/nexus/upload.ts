import type { UploadTicket } from "@/lib/nexus/types";

/** Envia o arquivo direto ao object storage (MinIO) pela URL pré-assinada
 * emitida pelo backend — o binário nunca passa pela API nem pelo BFF. */
export async function putToTicket(ticket: UploadTicket, file: Blob): Promise<void> {
  const res = await fetch(ticket.upload_url, {
    method: ticket.method || "PUT",
    headers: ticket.headers,
    body: file,
  });
  if (!res.ok) throw new Error(`upload recusado pelo armazenamento (HTTP ${res.status})`);
}

/** Tipo MIME com fallback genérico (alguns SOs não informam). */
export function mimeOf(file: File): string {
  return file.type || "application/octet-stream";
}

/** SHA-256 hexadecimal de um arquivo (Web Crypto) — usado para conferir
 * a integridade de documentos assinados pelo Signum. crypto.subtle só
 * existe em contexto seguro (HTTPS ou localhost); servido por HTTP num IP
 * da rede cai na implementação em JS puro abaixo. */
export async function sha256Hex(file: Blob): Promise<string> {
  const data = new Uint8Array(await file.arrayBuffer());
  const digest = globalThis.crypto?.subtle
    ? new Uint8Array(await crypto.subtle.digest("SHA-256", data))
    : sha256(data);
  return [...digest].map((b) => b.toString(16).padStart(2, "0")).join("");
}

const K = new Uint32Array([
  0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
  0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
  0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
  0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
  0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
  0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
  0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
  0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
]);

/** SHA-256 (FIPS 180-4) em JS puro — fallback de sha256Hex. */
export function sha256(data: Uint8Array): Uint8Array {
  const bitLen = data.length * 8;
  const padded = new Uint8Array((((data.length + 9 + 63) >> 6) << 6));
  padded.set(data);
  padded[data.length] = 0x80;
  const view = new DataView(padded.buffer);
  view.setUint32(padded.length - 8, Math.floor(bitLen / 0x100000000));
  view.setUint32(padded.length - 4, bitLen >>> 0);

  const H = new Uint32Array([
    0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
  ]);
  const W = new Uint32Array(64);
  // Índices sempre dentro de W/K/H (tamanhos fixos) — `!` só satisfaz o
  // noUncheckedIndexedAccess.
  const w = (i: number) => W[i]!;
  const rotr = (x: number, n: number) => (x >>> n) | (x << (32 - n));
  for (let off = 0; off < padded.length; off += 64) {
    for (let i = 0; i < 16; i++) W[i] = view.getUint32(off + i * 4);
    for (let i = 16; i < 64; i++) {
      const s0 = rotr(w(i - 15), 7) ^ rotr(w(i - 15), 18) ^ (w(i - 15) >>> 3);
      const s1 = rotr(w(i - 2), 17) ^ rotr(w(i - 2), 19) ^ (w(i - 2) >>> 10);
      W[i] = w(i - 16) + s0 + w(i - 7) + s1;
    }
    let a = H[0]!, b = H[1]!, c = H[2]!, d = H[3]!, e = H[4]!, f = H[5]!, g = H[6]!, h = H[7]!;
    for (let i = 0; i < 64; i++) {
      const t1 = (h + (rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)) + ((e & f) ^ (~e & g)) + K[i]! + w(i)) >>> 0;
      const t2 = ((rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)) + ((a & b) ^ (a & c) ^ (b & c))) >>> 0;
      h = g;
      g = f;
      f = e;
      e = (d + t1) >>> 0;
      d = c;
      c = b;
      b = a;
      a = (t1 + t2) >>> 0;
    }
    [a, b, c, d, e, f, g, h].forEach((v, i) => (H[i] = H[i]! + v));
  }
  const out = new Uint8Array(32);
  const outView = new DataView(out.buffer);
  H.forEach((v, i) => outView.setUint32(i * 4, v));
  return out;
}
