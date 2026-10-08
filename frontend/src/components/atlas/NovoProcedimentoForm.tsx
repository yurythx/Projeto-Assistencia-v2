"use client";

import { ArrowRight, Plus, Trash2 } from "lucide-react";
import { useId, useState, type FormEvent } from "react";

import { useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { sugerirModelo } from "@/lib/atlas/modelos";
import { useAtrasado } from "@/lib/atlas/useAtrasado";
import type {
  ClassificacaoTTDD,
  EtapaDocumento,
  EtapaTransicao,
  ModeloDocumento,
  NivelAcesso,
  Workflow,
} from "@/lib/nexus/types";

import { NIVEL_ACESSO } from "./labels";

type Transicao = Omit<EtapaTransicao, "id" | "destino_ordem"> & { destinoKey: number };

interface EtapaForm {
  key: number;
  nome_setor: string;
  unidade_administrativa: string;
  atribuicoes_setor: string;
  prazo_sla_em_dias: number;
  manter_aberto_apos_remessa: boolean;
  /** Uma peça por linha. Peça nova: nato-digital, assinatura individual,
   * obrigatória; peça que já existia mantém os atributos (`docs`). */
  pecas: string;
  docs: EtapaDocumento[];
  /** Transições da versão anterior, com o destino pela etapa (key): seguem
   * a etapa se a ordem mudar e somem se o destino for removido. */
  transicoes: Transicao[];
}

const novaEtapa = (key: number): EtapaForm => ({
  key,
  nome_setor: "",
  unidade_administrativa: "",
  atribuicoes_setor: "",
  prazo_sla_em_dias: 5,
  manter_aberto_apos_remessa: false,
  pecas: "",
  docs: [],
  transicoes: [],
});

/** Peças de uma etapa (uma por linha, sem vazias). */
const linhas = (pecas: string) =>
  pecas
    .split("\n")
    .map((p) => p.trim())
    .filter(Boolean);

/** Modelo já ligado a cada peça da versão atual ("" = sem modelo). */
function modelosDe(base?: Workflow): Record<string, string> {
  const out: Record<string, string> = {};
  for (const e of base?.etapas ?? [])
    for (const d of e.documentos) out[d.nome_documento] ??= d.modelo_id ?? "";
  return out;
}

/** Etapas da versão atual no formato do formulário (key = ordem original). */
function etapasDe(base: Workflow): EtapaForm[] {
  return base.etapas.map((e) => ({
    key: e.ordem,
    nome_setor: e.nome_setor,
    unidade_administrativa: e.unidade_administrativa,
    atribuicoes_setor: e.atribuicoes_setor,
    prazo_sla_em_dias: e.prazo_sla_em_dias,
    manter_aberto_apos_remessa: e.manter_aberto_apos_remessa,
    pecas: e.documentos.map((d) => d.nome_documento).join("\n"),
    docs: e.documentos,
    transicoes: e.transicoes.map((t) => ({
      condicao_transicao: t.condicao_transicao,
      is_devolucao_diligencia: t.is_devolucao_diligencia,
      descricao_diligencia: t.descricao_diligencia,
      destinoKey: t.destino_ordem,
    })),
  }));
}

/** Série da TTDD por busca (código ou descritor) — a tabela tem ~1.700
 * séries; só as vigentes podem ser escolhidas. */
function CampoSerie({ inicial, aviso }: { inicial: string; aviso?: string }) {
  const id = useId();
  const [texto, setTexto] = useState(inicial);
  const termo = useAtrasado(texto.trim());
  const busca = useApiPage<ClassificacaoTTDD>(
    termo.length >= 2 ? withQuery("v1/atlas/ttdd", { q: termo, page_size: 20 }) : null,
  );
  const itens = busca.data?.items ?? [];
  const escolhida = itens.find((c) => c.codigo === texto.trim());
  return (
    <div className="flex flex-col gap-1">
      <Input
        id="atlas-ttdd"
        name="codigo_ttdd"
        label="Classificação TTDD *"
        placeholder="Código ou descritor (ex.: pregão)"
        list={`${id}-opcoes`}
        autoComplete="off"
        required
        maxLength={32}
        value={texto}
        onChange={(e) => setTexto(e.target.value)}
        aria-describedby={`${id}-ajuda`}
      />
      <datalist id={`${id}-opcoes`}>
        {itens.map((c) => (
          <option key={c.codigo} value={c.codigo}>
            {c.codigo} — {c.descritor}
          </option>
        ))}
      </datalist>
      <p
        id={`${id}-ajuda`}
        className={`text-xs ${aviso && !escolhida ? "text-warning" : "text-muted"}`}
      >
        {escolhida
          ? escolhida.descritor
          : (aviso ?? "Digite o código ou parte do descritor e escolha a série na lista.")}
      </p>
    </div>
  );
}

/** Cadastro de procedimento (atlas:manage) ou, com `base`, a versão seguinte
 * dele: o formulário vem preenchido com a versão atual (peças e transições
 * preservadas) e, ao publicar, as demais versões são desativadas. */
export function NovoProcedimentoForm({
  base,
  onDone,
}: {
  base?: Workflow;
  onDone: (wf: Workflow) => void;
}) {
  const { run, pending } = useAction();
  const [nivel, setNivel] = useState<NivelAcesso>(base?.nivel_acesso ?? "PUBLICO");
  // Rascunho (padrão — ADR 028): o fluxo vai para validação nos departamentos
  // e só é publicado ao ser homologado. Desmarcado, publica direto.
  const [rascunho, setRascunho] = useState(true);
  const [etapas, setEtapas] = useState<EtapaForm[]>(base ? etapasDe(base) : [novaEtapa(1)]);
  const serieRevogada = base?.classificacao?.revogada_em ? base.codigo_ttdd : undefined;
  // Modelos da biblioteca (ADR 024): a peça sem escolha explícita recebe o
  // modelo ativo de mesmo nome, se houver.
  const biblioteca = useApiQuery<ModeloDocumento[]>("v1/atlas/admin/modelos");
  const modelos = biblioteca.data ?? [];
  const ativos = modelos.filter((m) => m.ativo);
  const [escolhas, setEscolhas] = useState<Record<string, string>>(() => modelosDe(base));
  const modeloDa = (peca: string) => escolhas[peca] ?? sugerirModelo(peca, ativos)?.id ?? "";
  const pecas = [...new Set(etapas.flatMap((e) => linhas(e.pecas)))];

  function update(key: number, patch: Partial<EtapaForm>) {
    setEtapas((list) => list.map((e) => (e.key === key ? { ...e, ...patch } : e)));
  }
  const ordemDe = (key: number) => etapas.findIndex((e) => e.key === key) + 1;

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const corpo = {
      codigo_processual:
        base?.codigo_processual ?? String(fd.get("codigo_processual") ?? "").trim(),
      titulo: String(fd.get("titulo") ?? "").trim(),
      objetivo: String(fd.get("objetivo") ?? "").trim(),
      publico_alvo: String(fd.get("publico_alvo") ?? "").trim(),
      nivel_acesso: nivel,
      hipotese_legal_restricao:
        nivel === "PUBLICO" ? "" : String(fd.get("hipotese_legal") ?? "").trim(),
      codigo_ttdd: String(fd.get("codigo_ttdd") ?? "").trim(),
      rascunho,
      etapas: etapas.map((etapa, i) => ({
        ordem: i + 1,
        nome_setor: etapa.nome_setor.trim(),
        unidade_administrativa: etapa.unidade_administrativa.trim(),
        atribuicoes_setor: etapa.atribuicoes_setor.trim(),
        prazo_sla_em_dias: etapa.prazo_sla_em_dias,
        manter_aberto_apos_remessa: etapa.manter_aberto_apos_remessa,
        documentos: linhas(etapa.pecas).map((nome) => {
          const d = etapa.docs.find((x) => x.nome_documento === nome);
          return {
            nome_documento: nome,
            obrigatorio: d?.obrigatorio ?? true,
            formato: d?.formato ?? "NATO_DIGITAL",
            tipo_assinatura: d?.tipo_assinatura ?? "INDIVIDUAL",
            exige_conferencia_copia: d?.exige_conferencia_copia ?? false,
            modelo_minuta_padrao_url: d?.modelo_minuta_padrao_url ?? "",
            modelo_id: modeloDa(nome) || null,
          };
        }),
        transicoes: etapa.transicoes
          .filter((t) => ordemDe(t.destinoKey) > 0)
          .map(({ destinoKey, ...t }) => ({ ...t, destino_ordem: ordemDe(destinoKey) })),
      })),
    };
    const res = await run(
      () =>
        base
          ? apiClient.post<Workflow>(`v1/atlas/admin/workflows/${base.id}/versoes`, corpo)
          : apiClient.post<Workflow>("v1/atlas/admin/workflows", corpo),
      rascunho
        ? "Rascunho salvo para validação"
        : base
          ? "Nova versão publicada"
          : "Procedimento cadastrado",
    );
    if (res) onDone(res.data);
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      {base && (
        <p className="rounded-md bg-primary/5 px-3 py-2 text-sm text-foreground">
          Revise o procedimento e publique: a versão atual ({base.versao}) e as anteriores ficam
          desativadas e continuam no histórico da gestão.
        </p>
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        <Input
          id="atlas-codigo"
          name="codigo_processual"
          label="Código processual *"
          placeholder="Ex.: ADM.FIN.021"
          required
          maxLength={64}
          defaultValue={base?.codigo_processual}
          readOnly={!!base}
        />
        <Select
          id="atlas-nivel"
          label="Nível de acesso sugerido *"
          value={nivel}
          onChange={(e) => setNivel(e.target.value as NivelAcesso)}
          options={Object.entries(NIVEL_ACESSO).map(([value, n]) => ({ value, label: n.label }))}
        />
      </div>
      {nivel !== "PUBLICO" && (
        <Input
          id="atlas-hipotese"
          name="hipotese_legal"
          label="Hipótese legal da restrição *"
          placeholder="Ex.: LAI, art. 31"
          required
          maxLength={255}
          defaultValue={base?.hipotese_legal_restricao}
        />
      )}
      <Input
        id="atlas-titulo"
        name="titulo"
        label="Título do procedimento *"
        required
        maxLength={255}
        defaultValue={base?.titulo}
      />
      <Textarea
        id="atlas-objetivo"
        name="objetivo"
        label="Objetivo *"
        rows={2}
        required
        maxLength={5000}
        defaultValue={base?.objetivo}
      />
      <div className="grid gap-3 sm:grid-cols-2">
        <Input
          id="atlas-publico"
          name="publico_alvo"
          label="Público-alvo *"
          placeholder="Ex.: Secretarias municipais"
          required
          maxLength={255}
          defaultValue={base?.publico_alvo}
        />
        <CampoSerie
          inicial={serieRevogada ? "" : (base?.codigo_ttdd ?? "")}
          aviso={
            serieRevogada &&
            `A série ${serieRevogada} foi revogada na TTDD em vigor: escolha uma série vigente.`
          }
        />
      </div>

      <fieldset className="flex flex-col gap-3 border-t border-surface-border pt-3">
        <legend className="text-xs font-semibold uppercase tracking-wide text-foreground">
          Etapas, na ordem de tramitação
        </legend>
        {etapas.map((etapa, i) => (
          <div key={etapa.key} className="rounded-lg border border-surface-border p-3">
            <div className="mb-2 flex items-center justify-between">
              <span className="text-sm font-semibold text-foreground">Etapa {i + 1}</span>
              {etapas.length > 1 && (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  aria-label={`Remover etapa ${i + 1}`}
                  onClick={() => setEtapas((list) => list.filter((e) => e.key !== etapa.key))}
                >
                  <Trash2 size={14} aria-hidden="true" />
                </Button>
              )}
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <Input
                id={`atlas-setor-${etapa.key}`}
                label="Setor *"
                required
                maxLength={150}
                value={etapa.nome_setor}
                onChange={(e) => update(etapa.key, { nome_setor: e.target.value })}
              />
              <Input
                id={`atlas-sigla-${etapa.key}`}
                label="Sigla da unidade *"
                placeholder="Ex.: SEMAD/LIC"
                required
                maxLength={64}
                value={etapa.unidade_administrativa}
                onChange={(e) => update(etapa.key, { unidade_administrativa: e.target.value })}
              />
            </div>
            <div className="mt-3 grid gap-3 sm:grid-cols-[1fr_8rem]">
              <Input
                id={`atlas-atrib-${etapa.key}`}
                label="Atribuições *"
                required
                maxLength={5000}
                value={etapa.atribuicoes_setor}
                onChange={(e) => update(etapa.key, { atribuicoes_setor: e.target.value })}
              />
              <Input
                id={`atlas-prazo-${etapa.key}`}
                label="Prazo (dias) *"
                type="number"
                min={0}
                max={3650}
                required
                value={etapa.prazo_sla_em_dias}
                onChange={(e) => update(etapa.key, { prazo_sla_em_dias: Number(e.target.value) })}
              />
            </div>
            <div className="mt-3">
              <Textarea
                id={`atlas-pecas-${etapa.key}`}
                label="Peças exigidas (uma por linha)"
                rows={2}
                value={etapa.pecas}
                onChange={(e) => update(etapa.key, { pecas: e.target.value })}
              />
            </div>
            <label className="mt-2 flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="h-4 w-4 accent-primary"
                checked={etapa.manter_aberto_apos_remessa}
                onChange={(e) =>
                  update(etapa.key, { manter_aberto_apos_remessa: e.target.checked })
                }
              />
              Manter o processo aberto na unidade após a remessa
            </label>
            {etapa.transicoes.length > 0 && (
              <ul
                className="mt-2 flex flex-col gap-0.5 text-xs text-muted"
                aria-label={`Transições da etapa ${i + 1}`}
              >
                {etapa.transicoes.map((t, j) => {
                  const destino = ordemDe(t.destinoKey);
                  return (
                    <li key={j} className="flex items-center gap-1">
                      <ArrowRight size={12} aria-hidden="true" />
                      {destino > 0
                        ? `${t.condicao_transicao} → etapa ${destino}`
                        : `${t.condicao_transicao} — removida (a etapa de destino saiu)`}
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        ))}
        <div>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() =>
              setEtapas((list) => [...list, novaEtapa(Math.max(0, ...list.map((e) => e.key)) + 1)])
            }
          >
            <Plus size={14} aria-hidden="true" className="mr-1" /> Adicionar etapa
          </Button>
        </div>
      </fieldset>

      {pecas.length > 0 && (
        <fieldset className="flex flex-col gap-3 border-t border-surface-border pt-3">
          <legend className="text-xs font-semibold uppercase tracking-wide text-foreground">
            Modelos das peças
          </legend>
          <p className="text-xs text-muted">
            Ligue cada peça a um modelo da biblioteca: quem consulta o procedimento baixa a versão
            atual dele. Peças com o mesmo nome de um modelo já vêm ligadas.
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            {pecas.map((peca, i) => {
              const atual = modeloDa(peca);
              const opcoes = modelos.filter((m) => m.ativo || m.id === atual);
              return (
                <Select
                  key={peca}
                  id={`atlas-modelo-${i}`}
                  label={peca}
                  value={atual}
                  onChange={(e) => setEscolhas((x) => ({ ...x, [peca]: e.target.value }))}
                  options={[
                    { value: "", label: "Sem modelo" },
                    ...opcoes.map((m) => ({
                      value: m.id,
                      label: m.ativo ? m.nome : `${m.nome} (desativado)`,
                    })),
                  ]}
                />
              );
            })}
          </div>
        </fieldset>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-surface-border pt-3">
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={rascunho}
            onChange={(e) => setRascunho(e.target.checked)}
            className="h-4 w-4 accent-primary"
          />
          Salvar como rascunho, para validação (não publica)
        </label>
        <Button type="submit" loading={pending}>
          {rascunho ? "Salvar rascunho" : base ? "Publicar nova versão" : "Cadastrar procedimento"}
        </Button>
      </div>
    </form>
  );
}
