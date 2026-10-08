import type { UUID } from "@/lib/nexus/types";

/** Formatos aceitos na biblioteca (o backend confere o conteúdo). */
export const FORMATOS_MODELO = ".docx,.odt,.pdf,.doc,.rtf,.xlsx,.ods";

/** Limite do arquivo de um modelo (o mesmo do backend). */
export const MAX_MODELO_BYTES = 10 * 1024 * 1024;

/** Download pela API (mesma origem): a versão atual ou uma específica.
 * `publico`: pelo proxy anônimo (site institucional, sem login). */
export function urlArquivoModelo(id: UUID, versao?: number, publico = false): string {
  const base = `/api/${publico ? "public" : "backend"}/v1/atlas/modelos/${encodeURIComponent(id)}/arquivo`;
  return versao ? `${base}?versao=${versao}` : base;
}

/** Extensão em maiúsculas para o selo do formato ("DOCX"). */
export function extensao(nome: string): string {
  const i = nome.lastIndexOf(".");
  return i < 0 ? "" : nome.slice(i + 1).toUpperCase();
}

/** Normaliza para comparar nomes (sem acento, minúsculas). */
export function chave(s: string): string {
  return s
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase()
    .trim();
}

/** Sugere o modelo de mesmo nome da peça (sem acento e caixa). */
export function sugerirModelo<T extends { id: UUID; nome: string }>(
  peca: string,
  modelos: T[],
): T | undefined {
  const k = chave(peca);
  return k ? modelos.find((m) => chave(m.nome) === k) : undefined;
}

/** Nome do modelo a partir do nome do arquivo (sem a extensão); se começar
 * por um código de série da TTDD ("2.0.02.00.07 - Edital.docx"), devolve a
 * série para ligar o modelo a ela. */
export function nomeDoArquivo(arquivo: string): { nome: string; serie?: string } {
  const base = arquivo.replace(/\.[^.]+$/, "").trim();
  const m = /^(\d{1,2}\.0\.\d{2}\.\d{2}\.\d{2})\s*[-–—_]?\s*(.+)$/.exec(base);
  return m ? { nome: m[2]!.trim(), serie: m[1] } : { nome: base };
}
