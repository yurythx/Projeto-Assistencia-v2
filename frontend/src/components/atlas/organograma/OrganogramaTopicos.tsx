"use client";

import { ChevronDown, ChevronRight, TriangleAlert } from "lucide-react";
import Link from "next/link";
import { useState, type ReactNode } from "react";

import { Button } from "@/components/ui/Button";
import { useApiPage, withQuery } from "@/lib/api/swr";
import type { ClassificacaoTTDD, ContagemOrganograma, OrgaoOrganograma } from "@/lib/nexus/types";

import { linkFuncao, linkSecretaria, rotulo } from "./organograma";

/** Uma linha da árvore: botão de abrir/fechar, código, nome, pontilhado e as
 * contagens à direita (como um sumário). */
function Linha({
  nivel,
  codigo,
  nome,
  href,
  contagem,
  gestao,
  aberto,
  onToggle,
  children,
}: {
  nivel: number;
  codigo: string;
  nome: string;
  href: string;
  contagem: ContagemOrganograma;
  gestao: boolean;
  aberto: boolean;
  onToggle: () => void;
  children?: ReactNode;
}) {
  const lacuna = nivel === 2 && contagem.lacunas > 0;
  return (
    <li className="break-inside-avoid-page">
      <div
        className={`flex items-baseline gap-2 py-0.5 ${nivel === 0 ? "text-base font-bold" : nivel === 1 ? "font-semibold" : "text-sm"}`}
      >
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={aberto}
          aria-label={`${aberto ? "Recolher" : "Abrir"} ${codigo} ${nome}`}
          className="shrink-0 self-center rounded text-muted hover:text-foreground print:hidden"
        >
          {aberto ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </button>
        <Link href={href} className="min-w-0 hover:underline">
          <span className="mr-1 font-mono text-xs text-muted">{codigo}</span>
          {nome}
        </Link>
        {lacuna && (
          <TriangleAlert
            size={13}
            className="shrink-0 self-center text-warning"
            aria-label="processo sem procedimento"
          />
        )}
        <span
          aria-hidden="true"
          className="mb-1 min-w-4 flex-1 border-b border-dotted border-foreground/40"
        />
        <span className="shrink-0 whitespace-nowrap text-xs font-normal text-muted">
          {rotulo(contagem, gestao)}
        </span>
      </div>
      {aberto && children}
    </li>
  );
}

/** As séries de uma subfunção, carregadas ao abrir. */
function Series({ subfuncao }: { subfuncao: string }) {
  const lista = useApiPage<ClassificacaoTTDD>(
    withQuery("v1/atlas/ttdd", { codigo: subfuncao, page_size: 100 }),
  );
  const series = lista.data?.items ?? [];
  if (lista.isLoading) return <p className="ml-12 text-xs text-muted">Carregando séries…</p>;
  return (
    <ul
      className="ml-12 border-l border-foreground/20 pl-3 text-xs"
      aria-label={`Séries de ${subfuncao}`}
    >
      {series.map((s) => (
        <li key={s.codigo} className="py-0.5">
          <Link href={`/atlas/ttdd/${encodeURIComponent(s.codigo)}`} className="hover:underline">
            <span className="mr-1 font-mono text-muted">{s.codigo}</span>
            {s.descritor}
          </Link>
        </li>
      ))}
    </ul>
  );
}

/** Árvore em tópicos: secretaria > função > subfunção > séries, com as
 * contagens na mesma linha. É a visão mais fácil de ler e de imprimir
 * (retrato); imprime o que estiver aberto. */
export function OrganogramaTopicos({
  orgaos,
  gestao,
}: {
  orgaos: OrgaoOrganograma[];
  gestao: boolean;
}) {
  // Começa com as secretarias abertas até as funções.
  const [abertos, setAbertos] = useState<Set<string>>(() => new Set(orgaos.map((o) => o.prefixo)));
  const alternar = (codigo: string) =>
    setAbertos((a) => {
      const n = new Set(a);
      if (n.has(codigo)) n.delete(codigo);
      else n.add(codigo);
      return n;
    });
  const tudo = () =>
    setAbertos(new Set(orgaos.flatMap((o) => [o.prefixo, ...o.funcoes.map((f) => f.codigo)])));

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2 print:hidden">
        <Button size="sm" variant="secondary" onClick={tudo}>
          Abrir até as subfunções
        </Button>
        <Button size="sm" variant="secondary" onClick={() => setAbertos(new Set())}>
          Recolher tudo
        </Button>
        <p className="self-center text-xs text-muted">
          Abra uma subfunção para ver as séries. A impressão sai como está na tela.
        </p>
      </div>
      <ul className="flex flex-col gap-1" aria-label="Organograma em tópicos">
        {orgaos.map((o) => (
          <Linha
            key={o.prefixo}
            nivel={0}
            codigo={o.prefixo}
            nome={o.nome}
            href={linkSecretaria(o.prefixo)}
            contagem={o}
            gestao={gestao}
            aberto={abertos.has(o.prefixo)}
            onToggle={() => alternar(o.prefixo)}
          >
            <ul className="ml-5 border-l border-foreground/20 pl-2">
              {o.funcoes.map((f) => (
                <Linha
                  key={f.codigo}
                  nivel={1}
                  codigo={f.codigo}
                  nome={f.nome}
                  href={linkFuncao(o.prefixo, f.codigo)}
                  contagem={f}
                  gestao={gestao}
                  aberto={abertos.has(f.codigo)}
                  onToggle={() => alternar(f.codigo)}
                >
                  <ul className="ml-5 border-l border-foreground/20 pl-2">
                    {f.subfuncoes.map((s) => (
                      <Linha
                        key={s.codigo}
                        nivel={2}
                        codigo={s.codigo}
                        nome={s.nome}
                        href={`/atlas/ttdd?codigo=${encodeURIComponent(s.codigo)}`}
                        contagem={s}
                        gestao={gestao}
                        aberto={abertos.has(s.codigo)}
                        onToggle={() => alternar(s.codigo)}
                      >
                        <Series subfuncao={s.codigo} />
                      </Linha>
                    ))}
                  </ul>
                </Linha>
              ))}
            </ul>
          </Linha>
        ))}
      </ul>
    </div>
  );
}
