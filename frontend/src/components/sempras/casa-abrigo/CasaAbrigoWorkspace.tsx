"use client";

import { useSearchParams } from "next/navigation";
import { useState, type ReactNode } from "react";

import { DataState } from "@/components/nexus/DataState";
import { useApiQuery } from "@/lib/api/swr";
import type { AcolhimentoDetalhe, Alerta, Cidadao, PainelCasaAbrigo as Painel } from "@/lib/sempras/types";

import { AcolhimentoWizard } from "./AcolhimentoWizard";
import { Abas } from "./campos";
import { DiarioAba } from "./DiarioAba";
import { EvolucaoAba } from "./EvolucaoAba";
import { Institucionalizados } from "./Institucionalizados";
import { ABA_DO_ALERTA, PainelCasaAbrigo } from "./PainelCasaAbrigo";
import { PiaAba } from "./PiaAba";
import { RelatoriosAba } from "./RelatoriosAba";
import { TarefasAba } from "./TarefasAba";
import { TermosAba } from "./TermosAba";
import { VisitantesAba } from "./VisitantesAba";

export type AbaCasaAbrigo = "painel" | "acolhidos" | "pia" | "evolucao" | "termos" | "visitantes" | "diario" | "relatorios" | "tarefas";

const ABAS: { key: AbaCasaAbrigo; label: string }[] = [
  { key: "painel", label: "Painel" },
  { key: "acolhidos", label: "Acolhidos" },
  { key: "pia", label: "P.I.A." },
  { key: "evolucao", label: "Evolução técnica" },
  { key: "termos", label: "Advertência / passeio" },
  { key: "visitantes", label: "Visitantes" },
  { key: "diario", label: "Diário de bordo" },
  { key: "relatorios", label: "Relatórios" },
  { key: "tarefas", label: "Tarefas da equipe" },
];

/** O assistente em modo de edição (carrega o acolhimento completo). */
function EdicaoAcolhimento({ unidadeId, id, onFim }: { unidadeId: string; id: string; onFim: () => void }) {
  const detalhe = useApiQuery<AcolhimentoDetalhe>(`v1/sempras/casa-abrigo/${id}`);
  return (
    <DataState loading={detalhe.isLoading} error={detalhe.error} onRetry={() => void detalhe.mutate()} empty={!detalhe.data}>
      {detalhe.data && <AcolhimentoWizard unidadeId={unidadeId} detalhe={detalhe.data} onConcluido={onFim} onCancelar={onFim} />}
    </DataState>
  );
}

/** O assistente em modo de cadastro a partir da ficha (a criança já escolhida). */
function NovoParaCidadao({ unidadeId, cidadaoId, onFim, onCancelar }: {
  unidadeId: string;
  cidadaoId: string;
  onFim: (acolhimentoId: string) => void;
  onCancelar: () => void;
}) {
  const cidadao = useApiQuery<Cidadao>(`v1/sempras/cidadaos/${cidadaoId}`);
  return (
    <DataState loading={cidadao.isLoading} error={cidadao.error} onRetry={() => void cidadao.mutate()} empty={!cidadao.data}>
      {cidadao.data && <AcolhimentoWizard unidadeId={unidadeId} cidadaoInicial={cidadao.data} onConcluido={onFim} onCancelar={onCancelar} />}
    </DataState>
  );
}

type Modo = { tipo: "lista" } | { tipo: "novo"; cidadaoId?: string } | { tipo: "editar"; id: string };

/**
 * Espaço da Casa Abrigo (ADR 032 §3): as telas do sistema da unidade em
 * abas — painel e monitor, acolhidos (institucionalizados e assistente),
 * P.I.A., evolução, termos, visitantes, diário, relatórios e tarefas.
 * Aceita ?aba=, ?acolhido= (P.I.A.) e ?novo=<munícipe> (cadastro vindo da ficha).
 */
export function CasaAbrigoWorkspace({ unidadeId, extras }: { unidadeId: string; extras?: ReactNode }) {
  const params = useSearchParams();
  const novoDe = params.get("novo");
  const [aba, setAba] = useState<AbaCasaAbrigo>(() => {
    const pedida = params.get("aba") as AbaCasaAbrigo | null;
    if (novoDe) return "acolhidos";
    return pedida && ABAS.some((a) => a.key === pedida) ? pedida : "painel";
  });
  const [modo, setModo] = useState<Modo>(novoDe ? { tipo: "novo", cidadaoId: novoDe } : { tipo: "lista" });
  const [acolhidoPia, setAcolhidoPia] = useState(params.get("acolhido") ?? "");
  const painel = useApiQuery<Painel>(`v1/sempras/unidades/${unidadeId}/casa-abrigo`);
  const acolhidos = painel.data?.acolhidos ?? [];
  const recarregar = () => void painel.mutate();

  function abrirAlerta(a: Alerta) {
    setAba(ABA_DO_ALERTA[a.tipo]);
    if (a.acolhimento_id) setAcolhidoPia(a.acolhimento_id);
  }
  function trocarAba(k: AbaCasaAbrigo) {
    setAba(k);
    if (k !== "acolhidos") setModo({ tipo: "lista" });
    // os prazos mudam nas outras abas (relatórios, tarefas, P.I.A.): o monitor volta atualizado
    if (k === "painel") recarregar();
  }
  function fimDoAssistente() {
    setModo({ tipo: "lista" });
    recarregar();
  }

  return (
    <div className="flex flex-col gap-5">
      <Abas rotulo="Telas da Casa Abrigo" abas={ABAS} atual={aba} onChange={trocarAba} />
      <DataState loading={painel.isLoading} error={painel.error} onRetry={recarregar} empty={!painel.data}>
        {painel.data && (
          <>
            {aba === "painel" && (
              <>
                <PainelCasaAbrigo painel={painel.data} onAbrir={abrirAlerta} />
                {extras}
              </>
            )}
            {aba === "acolhidos" &&
              (modo.tipo === "editar" ? (
                <EdicaoAcolhimento unidadeId={unidadeId} id={modo.id} onFim={fimDoAssistente} />
              ) : modo.tipo === "novo" && modo.cidadaoId ? (
                <NovoParaCidadao unidadeId={unidadeId} cidadaoId={modo.cidadaoId} onFim={fimDoAssistente} onCancelar={() => setModo({ tipo: "lista" })} />
              ) : modo.tipo === "novo" ? (
                <AcolhimentoWizard unidadeId={unidadeId} onConcluido={fimDoAssistente} onCancelar={() => setModo({ tipo: "lista" })} />
              ) : (
                <Institucionalizados unidadeId={unidadeId} painel={painel.data} onNovo={() => setModo({ tipo: "novo" })}
                  onEditar={(id) => setModo({ tipo: "editar", id })} onChanged={recarregar} />
              ))}
            {aba === "pia" && <PiaAba acolhidos={acolhidos} acolhimentoId={acolhidoPia} onEscolher={setAcolhidoPia} />}
            {aba === "evolucao" && <EvolucaoAba unidadeId={unidadeId} acolhidos={acolhidos} />}
            {aba === "termos" && <TermosAba unidadeId={unidadeId} acolhidos={acolhidos} />}
            {aba === "visitantes" && <VisitantesAba unidadeId={unidadeId} acolhidos={acolhidos} />}
            {aba === "diario" && <DiarioAba unidadeId={unidadeId} acolhidos={acolhidos} />}
            {aba === "relatorios" && <RelatoriosAba unidadeId={unidadeId} acolhidos={acolhidos} />}
            {aba === "tarefas" && <TarefasAba unidadeId={unidadeId} />}
          </>
        )}
      </DataState>
    </div>
  );
}
