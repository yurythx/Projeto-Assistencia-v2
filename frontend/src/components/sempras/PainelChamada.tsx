"use client";

import { useEffect, useRef, useState } from "react";
import { Bell, Volume2, VolumeX } from "lucide-react";

import { useTopic } from "@/components/realtime/RealtimeProvider";
import { Button } from "@/components/ui/Button";
import { useApiQuery } from "@/lib/api/swr";
import { rotuloPrioridade, type FilaItem, type Unidade } from "@/lib/sempras/types";

function tocarChime() {
  try {
    const AudioCtx =
      window.AudioContext ||
      (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    if (!AudioCtx) return;
    const ctx = new AudioCtx();
    const now = ctx.currentTime;

    // Tom 1: Re 5 (587.33 Hz)
    const osc1 = ctx.createOscillator();
    const gain1 = ctx.createGain();
    osc1.type = "sine";
    osc1.frequency.setValueAtTime(587.33, now);
    gain1.gain.setValueAtTime(0.25, now);
    gain1.gain.exponentialRampToValueAtTime(0.001, now + 0.6);
    osc1.connect(gain1);
    gain1.connect(ctx.destination);
    osc1.start(now);
    osc1.stop(now + 0.6);

    // Tom 2: La 5 (880 Hz)
    const osc2 = ctx.createOscillator();
    const gain2 = ctx.createGain();
    osc2.type = "sine";
    osc2.frequency.setValueAtTime(880, now + 0.22);
    gain2.gain.setValueAtTime(0.3, now + 0.22);
    gain2.gain.exponentialRampToValueAtTime(0.001, now + 0.85);
    osc2.connect(gain2);
    gain2.connect(ctx.destination);
    osc2.start(now + 0.22);
    osc2.stop(now + 0.85);
  } catch {
    // Falha silenciosa caso áudio seja bloqueado pelo navegador
  }
}

function sintetizarVoz(senha: string, nome: string, local?: string) {
  if (typeof window === "undefined" || !("speechSynthesis" in window)) return;
  try {
    window.speechSynthesis.cancel();
    const texto = `Atenção. Senha ${senha.split("").join(" ")}.${nome ? ` ${nome}.` : ""}${
      local ? ` Comparecer ao ${local}.` : " Comparecer ao atendimento."
    }`;
    const utterance = new SpeechSynthesisUtterance(texto);
    utterance.lang = "pt-BR";
    utterance.rate = 0.92;
    utterance.pitch = 1.05;
    window.speechSynthesis.speak(utterance);
  } catch {
    // Falha silenciosa
  }
}

/**
 * Painel de chamada para TV da sala de espera:
 * - Últimos chamados em destaque;
 * - Aviso sonoro (chime) e sintetizador de voz (pt-BR);
 * - Atualização em tempo real via WebSockets (sempras:fila:<unidade>);
 * - Nas unidades sigilosas (Casa da Mulher, CREAS, Conselho Tutelar —
 *   `painel_exibe_nome` falso, ADR 031 §3) a TV mostra e fala só a senha.
 */
export function PainelChamada({ unidade }: { unidade: Unidade }) {
  const unidadeId = unidade.id;
  const exibeNome = unidade.painel_exibe_nome;
  const [audioAtivo, setAudioAtivo] = useState(false);
  const ultimoChamadoIdRef = useRef<string | null>(null);

  const fila = useApiQuery<FilaItem[]>(`v1/sempras/unidades/${unidadeId}/fila`, {
    refreshInterval: 15_000,
  });
  useTopic(`sempras:fila:${unidadeId}`, () => void fila.mutate());

  const itens = fila.data ?? [];
  const chamados = itens.filter((i) => i.status === "chamado").slice(-5).reverse();
  const aguardando = itens.filter((i) => i.status === "aguardando").length;

  const maisRecente = chamados[0];

  function dispararChamada(item: FilaItem) {
    tocarChime();
    setTimeout(() => {
      sintetizarVoz(item.senha_exibida, exibeNome ? item.nome : "", item.local);
    }, 600);
  }

  // Detecta quando uma nova senha é chamada
  useEffect(() => {
    if (!maisRecente) return;
    if (ultimoChamadoIdRef.current && ultimoChamadoIdRef.current !== maisRecente.id) {
      if (audioAtivo) {
        dispararChamada(maisRecente);
      }
    }
    ultimoChamadoIdRef.current = maisRecente.id;
  }, [maisRecente?.id, audioAtivo]);

  function handleTestarAudio() {
    tocarChime();
    setTimeout(() => {
      sintetizarVoz("P 0 1", exibeNome ? "Maria Silva" : "", "Guichê 2");
    }, 600);
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Controles de Áudio para o operador da TV */}
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-3">
        <div className="flex items-center gap-2">
          {audioAtivo ? (
            <Volume2 size={20} className="text-primary" />
          ) : (
            <VolumeX size={20} className="text-muted" />
          )}
          <span className="text-xs text-foreground font-medium">
            {audioAtivo
              ? "Áudio e voz ativados: as chamadas soarão na TV"
              : "Áudio desativado (clique em ativar para habilitar o som e a voz)"}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant={audioAtivo ? "secondary" : "primary"}
            onClick={() => setAudioAtivo((v) => !v)}
          >
            {audioAtivo ? (
              <>
                <VolumeX size={14} className="mr-1" /> Desativar som
              </>
            ) : (
              <>
                <Volume2 size={14} className="mr-1" /> Ativar chamada por voz
              </>
            )}
          </Button>
          <Button size="sm" variant="secondary" onClick={handleTestarAudio}>
            <Bell size={14} className="mr-1" /> Testar som
          </Button>
        </div>
      </div>

      <p className="text-center text-2xl text-muted" aria-live="polite">
        {aguardando} {aguardando === 1 ? "pessoa aguardando" : "pessoas aguardando"}
      </p>

      {chamados.length === 0 ? (
        <p className="py-20 text-center text-3xl text-muted">Nenhuma senha chamada</p>
      ) : (
        <ul aria-label="Últimas senhas chamadas" aria-live="polite" className="flex flex-col gap-4">
          {chamados.map((item, i) => (
            <li
              key={item.id}
              className={`flex items-center justify-between gap-6 rounded-2xl border p-6 transition-all ${
                i === 0
                  ? "border-primary/50 bg-primary/10 shadow-lg ring-2 ring-primary/20 animate-pulse"
                  : "border-surface-border bg-surface"
              }`}
            >
              <div className="flex items-center gap-6">
                <span
                  className={`font-mono font-black tabular-nums ${
                    i === 0 ? "text-8xl text-primary" : "text-4xl text-foreground"
                  }`}
                >
                  {item.senha_exibida}
                </span>
                {i === 0 && (
                  <span className="rounded-lg bg-primary px-3 py-1 text-xs font-bold uppercase tracking-wider text-primary-foreground">
                    Chamando agora
                  </span>
                )}
              </div>
              <span className="flex-1 text-right">
                {exibeNome && (
                  <span className={i === 0 ? "text-5xl font-bold block" : "text-2xl font-medium block"}>
                    {item.nome}
                  </span>
                )}
                <span className="block text-xl text-muted mt-1">
                  {[item.local, item.prioridade && rotuloPrioridade(item.prioridade)]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
