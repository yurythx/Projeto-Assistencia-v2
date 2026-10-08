import Link from "next/link";

import type { OrgaoOrganograma, SubfuncaoOrganograma } from "@/lib/nexus/types";

import {
  cobertura,
  FAIXAS,
  faixaCobertura,
  linkFuncao,
  linkSecretaria,
  linkSubfuncao,
  nomeCurto,
  rotulo,
  squarify,
  subfuncoesDe,
} from "./organograma";

/** Um bloco do mapa: tamanho pelas séries, cor pela cobertura. */
interface Bloco {
  chave: string;
  codigo: string;
  titulo: string;
  href: string;
  series: number;
  descricao: string;
  subfuncoes: SubfuncaoOrganograma[];
  filhos: Omit<Bloco, "filhos">[];
}

/** Coordenadas em "unidades" de um retângulo 100 × 60 (proporção 5:3). */
const LARGURA = 100;
const ALTURA = 60;
const pct = (v: number, total: number) => `${(v / total) * 100}%`;
/** Altura do título do bloco: ~2 linhas (1 unidade ≈ 11 px na tela cheia),
 * no máximo 35% de um bloco baixo. */
const TITULO = 3.4;
/** Abaixo disso o bloco interno mostra só o código (o nome fica na dica). */
const cabeNome = (r: { w: number; h: number }) => r.w >= 7 && r.h >= 3;

const classeFaixa = (subfuncoes: SubfuncaoOrganograma[], gestao: boolean) =>
  FAIXAS[faixaCobertura(cobertura(subfuncoes, gestao))].classe;

/** Com todas as secretarias: secretarias divididas em funções. Com uma só:
 * funções divididas em subfunções. */
function blocos(orgaos: OrgaoOrganograma[], gestao: boolean): Bloco[] {
  if (orgaos.length === 1) {
    const o = orgaos[0]!;
    return o.funcoes.map((f) => ({
      chave: f.codigo,
      codigo: f.codigo,
      titulo: f.nome,
      href: linkFuncao(o.prefixo, f.codigo),
      series: f.series,
      descricao: rotulo(f, gestao),
      subfuncoes: f.subfuncoes,
      filhos: f.subfuncoes.map((s) => ({
        chave: s.codigo,
        codigo: s.codigo,
        titulo: s.nome,
        href: linkSubfuncao(s.codigo),
        series: s.series,
        descricao: rotulo(s, gestao),
        subfuncoes: [s],
      })),
    }));
  }
  return orgaos.map((o) => ({
    chave: o.prefixo,
    codigo: o.prefixo,
    titulo: nomeCurto(o.nome),
    href: linkSecretaria(o.prefixo),
    series: o.series,
    descricao: rotulo(o, gestao),
    subfuncoes: subfuncoesDe(o),
    filhos: o.funcoes.map((f) => ({
      chave: f.codigo,
      codigo: f.codigo,
      titulo: f.nome,
      href: linkFuncao(o.prefixo, f.codigo),
      series: f.series,
      descricao: rotulo(f, gestao),
      subfuncoes: f.subfuncoes,
    })),
  }));
}

const ordenar = <T extends { series: number }>(itens: T[]) =>
  [...itens].sort((a, b) => b.series - a.series).map((item) => ({ valor: item.series, item }));

/** Mapa de blocos (treemap): cada bloco tem área proporcional às séries da
 * TTDD e cor pela cobertura de procedimentos. Mostra de longe onde está o
 * volume de documentos e onde faltam fluxos. */
export function OrganogramaBlocos({
  orgaos,
  gestao,
}: {
  orgaos: OrgaoOrganograma[];
  gestao: boolean;
}) {
  const externos = squarify(ordenar(blocos(orgaos, gestao)), { x: 0, y: 0, w: LARGURA, h: ALTURA });
  return (
    <figure className="flex flex-col gap-3 [print-color-adjust:exact]">
      <div className="relative aspect-[5/3] w-full overflow-hidden rounded-lg border border-surface-border">
        {externos.map(({ item: b, ...r }) => {
          // Os filhos ocupam o bloco abaixo do título.
          const titulo = Math.min(TITULO, r.h * 0.35);
          const area = { x: 0, y: 0, w: r.w, h: r.h - titulo };
          const internos = squarify(ordenar(b.filhos), area);
          return (
            <section
              key={b.chave}
              aria-label={`${b.codigo} ${b.titulo}: ${b.descricao}`}
              className={`absolute border-2 border-background ${classeFaixa(b.subfuncoes, gestao)}`}
              style={{
                left: pct(r.x, LARGURA),
                top: pct(r.y, ALTURA),
                width: pct(r.w, LARGURA),
                height: pct(r.h, ALTURA),
              }}
            >
              <Link
                href={b.href}
                title={`${b.codigo} ${b.titulo} — ${b.descricao}`}
                style={{ height: pct(titulo, r.h) }}
                className="absolute inset-x-0 top-0 block overflow-hidden px-1 text-[11px] font-bold leading-tight text-foreground hover:underline"
              >
                <span className="font-mono font-normal">{b.codigo}</span> {b.titulo}
                <span className="block truncate font-normal text-muted">{b.descricao}</span>
              </Link>
              <div className="absolute inset-x-0 bottom-0" style={{ height: pct(area.h, r.h) }}>
                {internos.map(({ item: f, ...q }) => (
                  <Link
                    key={f.chave}
                    href={f.href}
                    title={`${f.codigo} ${f.titulo} — ${f.descricao}`}
                    aria-label={`${f.codigo} ${f.titulo}: ${f.descricao}`}
                    className={`absolute overflow-hidden border border-background/80 p-0.5 text-[10px] leading-tight text-foreground hover:outline hover:outline-2 hover:outline-primary ${classeFaixa(f.subfuncoes, gestao)}`}
                    style={{
                      left: pct(q.x, area.w),
                      top: pct(q.y, area.h),
                      width: pct(q.w, area.w),
                      height: pct(q.h, area.h),
                    }}
                  >
                    <span className="font-mono">{f.codigo.split(".").slice(-1)[0]}</span>
                    {cabeNome(q) && ` ${f.titulo}`}
                  </Link>
                ))}
              </div>
            </section>
          );
        })}
      </div>
      <figcaption className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
        <span>Tamanho = número de séries da TTDD. Cor = subfunções com procedimento:</span>
        {FAIXAS.map((f) => (
          <span key={f.faixa} className="inline-flex items-center gap-1">
            <span
              aria-hidden="true"
              className={`inline-block h-3 w-3 rounded-sm border border-foreground/30 ${f.classe}`}
            />
            {f.label}
          </span>
        ))}
      </figcaption>
    </figure>
  );
}
