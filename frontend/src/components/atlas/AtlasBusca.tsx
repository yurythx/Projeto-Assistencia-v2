"use client";

import { Bot, FileClock, Search, Workflow as WorkflowIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { useId, useState, type ReactNode } from "react";

import { useApiPage, withQuery } from "@/lib/api/swr";
import { useAtrasado } from "@/lib/atlas/useAtrasado";
import type { ClassificacaoTTDD, Workflow } from "@/lib/nexus/types";

import { useAssistente } from "./AssistenteGaveta";

interface Opcao {
  id: string;
  grupo: string;
  icone: ReactNode;
  label: string;
  detalhe?: string;
  acao: () => void;
}

/** Busca unificada do Atlas (combobox WAI-ARIA): procedimentos e séries da
 * TTDD em paralelo, mais "ver todas as séries" e "perguntar ao assistente".
 * Enter sem opção destacada faz a pergunta ao assistente (quando houver) ou
 * abre a consulta da TTDD. */
export function AtlasBusca() {
  const id = useId();
  const router = useRouter();
  const { disponivel, abrir } = useAssistente();
  const [texto, setTexto] = useState("");
  const [aberto, setAberto] = useState(false);
  const [ativo, setAtivo] = useState(-1);
  const termo = useAtrasado(texto.trim());
  const busca = termo.length >= 2 ? termo : null;

  const wfs = useApiPage<Workflow>(
    busca && withQuery("v1/atlas/workflows", { q: busca, page_size: 5 }),
  );
  const series = useApiPage<ClassificacaoTTDD>(
    busca && withQuery("v1/atlas/ttdd", { q: busca, page_size: 5 }),
  );
  const carregando = wfs.isLoading || series.isLoading;

  const ir = (href: string) => () => router.push(href);
  const verSeries = (q: string) => ir(withQuery("/atlas/ttdd", { q }));
  const opcoes: Opcao[] = busca
    ? [
        ...(wfs.data?.items ?? []).map((w) => ({
          id: `wf-${w.id}`,
          grupo: "Procedimentos",
          icone: <WorkflowIcon size={14} aria-hidden="true" />,
          label: w.titulo,
          detalhe: w.codigo_processual,
          acao: ir(`/atlas/procedimentos/${w.id}`),
        })),
        ...(series.data?.items ?? []).map((c) => ({
          id: `ttdd-${c.codigo}`,
          grupo: "Tabela de Temporalidade",
          icone: <FileClock size={14} aria-hidden="true" />,
          label: c.descritor,
          detalhe: c.codigo,
          acao: ir(`/atlas/ttdd/${encodeURIComponent(c.codigo)}`),
        })),
        {
          id: "todas",
          grupo: "Mais opções",
          icone: <Search size={14} aria-hidden="true" />,
          label: `Ver todas as séries com “${busca}”`,
          acao: verSeries(busca),
        },
        ...(disponivel
          ? [
              {
                id: "assistente",
                grupo: "Mais opções",
                icone: <Bot size={14} aria-hidden="true" />,
                label: `Perguntar ao assistente: “${busca}”`,
                acao: () => abrir(busca),
              },
            ]
          : []),
      ]
    : [];
  const mostrar = aberto && opcoes.length > 0;
  const optId = (i: number) => `${id}-op-${i}`;

  function escolher(op: Opcao) {
    setAberto(false);
    setAtivo(-1);
    op.acao();
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      if (opcoes.length === 0) return;
      e.preventDefault();
      setAberto(true);
      const passo = e.key === "ArrowDown" ? 1 : -1;
      setAtivo((a) => (a + passo + opcoes.length + (a < 0 && passo < 0 ? 1 : 0)) % opcoes.length);
    } else if (e.key === "Escape") {
      setAberto(false);
      setAtivo(-1);
    } else if (e.key === "Enter") {
      e.preventDefault();
      const op = mostrar ? opcoes[ativo] : undefined;
      if (op) return escolher(op);
      const q = texto.trim();
      if (!q) return;
      setAberto(false);
      if (disponivel) abrir(q);
      else verSeries(q)();
    }
  }

  let grupoAnterior = "";
  return (
    <div className="relative">
      <label htmlFor={`${id}-in`} className="sr-only">
        Buscar no Atlas
      </label>
      <div className="flex items-center gap-2 rounded-xl border border-surface-border bg-surface px-4 shadow-sm focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/20">
        <Search size={18} aria-hidden="true" className="shrink-0 text-muted" />
        <input
          id={`${id}-in`}
          role="combobox"
          aria-expanded={mostrar}
          aria-controls={`${id}-lista`}
          aria-autocomplete="list"
          aria-activedescendant={mostrar && ativo >= 0 ? optId(ativo) : undefined}
          autoComplete="off"
          placeholder={
            disponivel
              ? "Busque um procedimento, uma série da TTDD ou faça uma pergunta"
              : "Busque um procedimento ou uma série da TTDD"
          }
          value={texto}
          onChange={(e) => {
            setTexto(e.target.value);
            setAberto(true);
            setAtivo(-1);
          }}
          onFocus={() => setAberto(true)}
          onClick={() => setAberto(true)}
          onBlur={() => setAberto(false)}
          onKeyDown={onKeyDown}
          className="h-12 w-full bg-transparent text-base text-foreground outline-none placeholder:text-muted"
        />
      </div>
      <p aria-live="polite" className="sr-only">
        {busca && !carregando ? `${opcoes.length} opções` : ""}
      </p>
      <ul
        id={`${id}-lista`}
        role="listbox"
        aria-label="Resultados da busca"
        hidden={!mostrar}
        className="absolute inset-x-0 top-full z-20 mt-1 max-h-96 overflow-y-auto rounded-xl border border-surface-border bg-surface p-1 shadow-lg"
      >
        {opcoes.map((op, i) => {
          const cabecalho = op.grupo !== grupoAnterior;
          grupoAnterior = op.grupo;
          return (
            <li
              key={op.id}
              id={optId(i)}
              role="option"
              aria-selected={i === ativo}
              // mousedown antes do blur do campo: mantém a lista para o clique.
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => escolher(op)}
              onMouseEnter={() => setAtivo(i)}
              className="cursor-pointer"
            >
              {cabecalho && (
                <span
                  aria-hidden="true"
                  className="block px-3 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-wide text-muted"
                >
                  {op.grupo}
                </span>
              )}
              <span
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm ${
                  i === ativo ? "bg-primary/10 text-foreground" : "text-foreground"
                }`}
              >
                <span className="shrink-0 text-primary">{op.icone}</span>
                <span className="min-w-0 flex-1 truncate">{op.label}</span>
                {op.detalhe && (
                  <span className="shrink-0 font-mono text-xs text-muted">{op.detalhe}</span>
                )}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
