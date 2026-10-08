import type {
  ContagemOrganograma,
  FuncaoOrganograma,
  OrgaoOrganograma,
  SubfuncaoOrganograma,
} from "@/lib/nexus/types";

/** As quatro visualizações da aba Organograma. */
export const VISOES = [
  { id: "caixas", label: "Organograma em caixas" },
  { id: "topicos", label: "Árvore em tópicos" },
  { id: "blocos", label: "Mapa de blocos" },
  { id: "fichas", label: "Fichas por secretaria" },
] as const;

export type Visao = (typeof VISOES)[number]["id"];

export function visaoValida(v: string | null): Visao {
  return VISOES.find((x) => x.id === v)?.id ?? "caixas";
}

/** Quantos procedimentos o nó tem: os publicados e, para a gestão, também
 * os rascunhos e os em validação. */
export function procedimentos(c: ContagemOrganograma, gestao: boolean): number {
  return c.publicados + (gestao ? c.em_validacao + c.rascunhos : 0);
}

const plural = (n: number, um: string, varios: string) => `${n} ${n === 1 ? um : varios}`;

/** Rótulo curto das contagens: "58 séries · 6 procedimentos". */
export function rotulo(c: ContagemOrganograma, gestao: boolean): string {
  return `${plural(c.series, "série", "séries")} · ${plural(
    procedimentos(c, gestao),
    "procedimento",
    "procedimentos",
  )}`;
}

/** Detalhe dos procedimentos para a gestão: "1 publicado · 2 rascunhos". */
export function detalheProcedimentos(c: ContagemOrganograma, gestao: boolean): string {
  const partes = [plural(c.publicados, "publicado", "publicados")];
  if (gestao && c.em_validacao) partes.push(`${c.em_validacao} em validação`);
  if (gestao && c.rascunhos) partes.push(plural(c.rascunhos, "rascunho", "rascunhos"));
  return partes.join(" · ");
}

/** Cobertura de uma função ou órgão: a fração das subfunções que têm
 * procedimento. */
export function cobertura(subfuncoes: SubfuncaoOrganograma[], gestao: boolean): number {
  if (subfuncoes.length === 0) return 0;
  return subfuncoes.filter((s) => procedimentos(s, gestao) > 0).length / subfuncoes.length;
}

export const subfuncoesDe = (o: OrgaoOrganograma): SubfuncaoOrganograma[] =>
  o.funcoes.flatMap((f: FuncaoOrganograma) => f.subfuncoes);

/** Faixa de cor da cobertura (0 = sem nenhum fluxo). */
export function faixaCobertura(fracao: number): 0 | 1 | 2 | 3 {
  if (fracao <= 0) return 0;
  if (fracao < 0.34) return 1;
  if (fracao < 0.67) return 2;
  return 3;
}

export const FAIXAS = [
  { faixa: 0, label: "Sem procedimento", classe: "bg-surface-hover" },
  { faixa: 1, label: "Até 1/3 das subfunções", classe: "bg-success/20" },
  { faixa: 2, label: "Até 2/3", classe: "bg-success/40" },
  { faixa: 3, label: "Mais de 2/3", classe: "bg-success/65" },
] as const;

export interface Retangulo {
  x: number;
  y: number;
  w: number;
  h: number;
}

/** Treemap "squarified" (Bruls, Huizing e van Wijk): divide o retângulo em
 * áreas proporcionais aos valores, com blocos o mais quadrados possível.
 * Valores devem vir em ordem decrescente; zeros são ignorados. */
export function squarify<T>(
  itens: { valor: number; item: T }[],
  r: Retangulo,
): (Retangulo & { item: T })[] {
  const validos = itens.filter((i) => i.valor > 0);
  const total = validos.reduce((s, i) => s + i.valor, 0);
  if (total === 0) return [];
  const escala = (r.w * r.h) / total;
  const areas = validos.map((i) => ({ area: i.valor * escala, item: i.item }));
  const out: (Retangulo & { item: T })[] = [];
  let resto = { ...r };

  const pior = (linha: number[], lado: number) => {
    const s = linha.reduce((a, b) => a + b, 0);
    const max = Math.max(...linha);
    const min = Math.min(...linha);
    return Math.max((lado * lado * max) / (s * s), (s * s) / (lado * lado * min));
  };
  const assentar = (linha: typeof areas) => {
    const s = linha.reduce((a, b) => a + b.area, 0);
    if (resto.w >= resto.h) {
      // Coluna à esquerda.
      const largura = s / resto.h;
      let y = resto.y;
      for (const l of linha) {
        const altura = l.area / largura;
        out.push({ x: resto.x, y, w: largura, h: altura, item: l.item });
        y += altura;
      }
      resto = { x: resto.x + largura, y: resto.y, w: resto.w - largura, h: resto.h };
    } else {
      // Faixa no topo.
      const altura = s / resto.w;
      let x = resto.x;
      for (const l of linha) {
        const largura = l.area / altura;
        out.push({ x, y: resto.y, w: largura, h: altura, item: l.item });
        x += largura;
      }
      resto = { x: resto.x, y: resto.y + altura, w: resto.w, h: resto.h - altura };
    }
  };

  let linha: typeof areas = [];
  for (const a of areas) {
    const lado = Math.min(resto.w, resto.h);
    const atual = linha.map((l) => l.area);
    if (linha.length === 0 || pior([...atual, a.area], lado) <= pior(atual, lado)) {
      linha.push(a);
    } else {
      assentar(linha);
      linha = [a];
    }
  }
  assentar(linha);
  return out;
}

/** "Secretaria Municipal de Saúde" → "Saúde", para os espaços pequenos. */
export function nomeCurto(nome: string): string {
  return nome.replace(/^Secretaria Municipal d[aeo]s? /i, "");
}

/** Links do organograma: a função abre os procedimentos do departamento; a
 * subfunção abre as séries na TTDD. */
export const linkFuncao = (prefixo: string, funcao: string) =>
  `/atlas/secretarias/${encodeURIComponent(prefixo)}?funcao=${encodeURIComponent(funcao)}`;
export const linkSecretaria = (prefixo: string) =>
  `/atlas/secretarias/${encodeURIComponent(prefixo)}`;
export const linkSubfuncao = (codigo: string) => `/atlas/ttdd?codigo=${encodeURIComponent(codigo)}`;
