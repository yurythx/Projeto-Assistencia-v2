"use client";

import { useState, type FormEvent } from "react";
import { Plus, Save, Trash2 } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import {
  MORADIAS_ANTERIORES,
  PERIODOS_ESCOLARES,
  dataISO,
  diaDe,
  fmtDia,
  piaVazia,
  type AcolhidoResumo,
  type AcolhimentoDetalhe,
  type PiaFichaDados,
  type PiaMeta,
  type TipoPiaMeta,
} from "@/lib/sempras/types";

import { Abas, Caixa, Grupo, SeletorAcolhido, opcoes } from "./campos";

type AbaPia = "identificacao" | "saude" | "familia" | "plano" | "rotina" | "intervencao";

const ABAS: { key: AbaPia; label: string }[] = [
  { key: "identificacao", label: "Identificação e documentos" },
  { key: "saude", label: "Saúde e escola" },
  { key: "familia", label: "Situação familiar" },
  { key: "plano", label: "Plano de ação" },
  { key: "rotina", label: "Rotina e atividades" },
  { key: "intervencao", label: "Intervenção familiar" },
];

function fichaDe(d: AcolhimentoDetalhe): PiaFichaDados {
  const base = piaVazia();
  if (!d.pia) return base;
  const out = { ...base } as Record<string, unknown>;
  for (const k of Object.keys(base)) out[k] = (d.pia as unknown as Record<string, unknown>)[k];
  return out as unknown as PiaFichaDados;
}

interface LinhaMeta {
  descricao: string;
  providencia: string;
  responsavel: string;
  prazo: string;
  esportes: string;
  cultura: string;
  lazer: string;
}

const linhaVazia = (): LinhaMeta => ({ descricao: "", providencia: "", responsavel: "", prazo: "", esportes: "", cultura: "", lazer: "" });

/** Uma linha do plano de ação (editável na própria tabela, como no 4.0). */
function LinhaPlano({ meta, onChanged }: { meta: PiaMeta; onChanged: () => void }) {
  const [l, setL] = useState<LinhaMeta>({ descricao: meta.descricao, providencia: meta.providencia, responsavel: meta.responsavel,
    prazo: diaDe(meta.prazo), esportes: meta.esportes, cultura: meta.cultura, lazer: meta.lazer });
  const { run, pending } = useAction();
  const mudou = l.descricao !== meta.descricao || l.providencia !== meta.providencia || l.responsavel !== meta.responsavel ||
    l.prazo !== diaDe(meta.prazo) || l.esportes !== meta.esportes || l.cultura !== meta.cultura || l.lazer !== meta.lazer;

  async function salvar() {
    const ok = await run(() => apiClient.put(`v1/sempras/casa-abrigo-pia-metas/${meta.id}`, { ...l, tipo: meta.tipo,
      prazo: l.prazo ? dataISO(l.prazo) : null }), "Ação atualizada");
    if (ok) onChanged();
  }
  async function concluir(concluida: boolean) {
    const ok = await run(() => apiClient.post(`v1/sempras/casa-abrigo-pia-metas/${meta.id}/concluir`, { concluida }));
    if (ok) onChanged();
  }
  async function remover() {
    const ok = await run(() => apiClient.delete(`v1/sempras/casa-abrigo-pia-metas/${meta.id}`), "Ação removida");
    if (ok) onChanged();
  }
  const campo = (k: keyof LinhaMeta, rotulo: string, type = "text") => (
    <td className="px-1 py-1">
      <input aria-label={`${rotulo} — ${meta.descricao}`} type={type} value={l[k]} onChange={(e) => setL({ ...l, [k]: e.target.value })}
        className="w-full rounded border border-surface-border bg-surface px-2 py-1 text-sm" />
    </td>
  );
  return (
    <tr className={meta.concluida ? "opacity-60" : ""}>
      <td className="px-1 py-1 text-center">
        <input type="checkbox" aria-label={`Concluída — ${meta.descricao}`} checked={meta.concluida} onChange={(e) => void concluir(e.target.checked)} />
      </td>
      {campo("descricao", "Necessidade")}
      {campo("providencia", "Providências")}
      {campo("responsavel", "Responsável")}
      {campo("prazo", "Prazo", "date")}
      {campo("esportes", "Esportes")}
      {campo("cultura", "Cultura")}
      {campo("lazer", "Lazer")}
      <td className="whitespace-nowrap px-1 py-1">
        {mudou && (
          <Button type="button" size="sm" variant="ghost" aria-label={`Salvar ${meta.descricao}`} loading={pending} onClick={() => void salvar()}>
            <Save size={14} aria-hidden="true" />
          </Button>
        )}
        <Button type="button" size="sm" variant="ghost" aria-label={`Remover ${meta.descricao}`} onClick={() => void remover()}>
          <Trash2 size={14} aria-hidden="true" />
        </Button>
      </td>
    </tr>
  );
}

/** Formulário de uma meta nova (plano de ação ou intervenção). */
function NovaMeta({ acolhimentoId, tipo, onChanged }: { acolhimentoId: string; tipo: TipoPiaMeta; onChanged: () => void }) {
  const [l, setL] = useState<LinhaMeta>(linhaVazia());
  const { run, pending } = useAction();
  async function adicionar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(() => apiClient.post(`v1/sempras/casa-abrigo/${acolhimentoId}/pia-metas`, {
      ...l, tipo, prazo: l.prazo ? dataISO(l.prazo) : null }), "Ação adicionada");
    if (ok) {
      setL(linhaVazia());
      onChanged();
    }
  }
  return (
    <form onSubmit={adicionar} className="grid gap-2 sm:grid-cols-4">
      <Input id={`meta-${tipo}-descricao`} label={tipo === "plano" ? "Necessidade identificada" : "Meta da intervenção"} value={l.descricao}
        onChange={(e) => setL({ ...l, descricao: e.target.value })} />
      <Input id={`meta-${tipo}-providencia`} label="Providências" value={l.providencia} onChange={(e) => setL({ ...l, providencia: e.target.value })} />
      <Input id={`meta-${tipo}-responsavel`} label="Responsável" value={l.responsavel} onChange={(e) => setL({ ...l, responsavel: e.target.value })} />
      <Input id={`meta-${tipo}-prazo`} label="Prazo" type="date" value={l.prazo} onChange={(e) => setL({ ...l, prazo: e.target.value })} />
      {tipo === "plano" && (
        <>
          <Input id="meta-plano-esportes" label="Esportes" value={l.esportes} onChange={(e) => setL({ ...l, esportes: e.target.value })} />
          <Input id="meta-plano-cultura" label="Cultura" value={l.cultura} onChange={(e) => setL({ ...l, cultura: e.target.value })} />
          <Input id="meta-plano-lazer" label="Lazer" value={l.lazer} onChange={(e) => setL({ ...l, lazer: e.target.value })} />
        </>
      )}
      <div className="flex items-end">
        <Button type="submit" variant="secondary" loading={pending} disabled={l.descricao.trim().length < 3}>
          <Plus size={14} className="mr-1" aria-hidden="true" /> Adicionar ação
        </Button>
      </div>
    </form>
  );
}

/** A ficha do PIA de um acolhido (montada quando ele é escolhido). */
function FichaPia({ detalhe, onChanged }: { detalhe: AcolhimentoDetalhe; onChanged: () => void }) {
  const [aba, setAba] = useState<AbaPia>("identificacao");
  const [f, setF] = useState<PiaFichaDados>(() => fichaDe(detalhe));
  const set = <K extends keyof PiaFichaDados>(k: K, v: PiaFichaDados[K]) => setF((x) => ({ ...x, [k]: v }));
  const { run, pending } = useAction();
  async function salvar() {
    const ok = await run(() => apiClient.put(`v1/sempras/casa-abrigo/${detalhe.id}/pia`, f), "P.I.A. salvo");
    if (ok) onChanged();
  }
  const texto = (k: keyof PiaFichaDados, rotulo: string, linhas = 3, placeholder?: string) => (
    <Textarea id={`pia-${k}`} label={rotulo} rows={linhas} placeholder={placeholder} value={f[k] as string}
      onChange={(e) => set(k, e.target.value as never)} />
  );
  const campo = (k: keyof PiaFichaDados, rotulo: string) => (
    <Input id={`pia-${k}`} label={rotulo} value={f[k] as string} onChange={(e) => set(k, e.target.value as never)} />
  );
  const caixa = (k: keyof PiaFichaDados, rotulo: string) => (
    <Caixa label={rotulo} checked={f[k] as boolean} onChange={(v) => set(k, v as never)} />
  );
  const metas = (tipo: TipoPiaMeta) => detalhe.pia_metas.filter((m) => m.tipo === tipo);

  return (
    <div className="flex flex-col gap-4">
      <p className="text-xs text-muted">
        {detalhe.pia ? `Última reavaliação: ${fmtDia(detalhe.pia.atualizado_em)}. ` : "P.I.A. ainda não preenchido. "}
        Salvar a ficha registra a reavaliação (ECA art. 19, §1º: no máximo a cada 3 meses).
      </p>
      <Abas rotulo="Partes do P.I.A." abas={ABAS} atual={aba} onChange={setAba} />

      {aba === "identificacao" && (
        <section className="flex flex-col gap-3" aria-label="Identificação e documentação">
          <p className="text-sm">
            <span className="text-muted">Criança/adolescente: </span>
            <strong>{detalhe.cidadao.nome_social || detalhe.cidadao.nome}</strong>
          </p>
          <div className="grid gap-3 sm:grid-cols-3">
            {campo("naturalidade", "Naturalidade")}
            {campo("nacionalidade", "Nacionalidade")}
            {campo("religiao", "Religião")}
          </div>
          <Grupo titulo="Documentação existente (marque o que possui)">
            <div className="grid gap-2 sm:grid-cols-3">
              {caixa("doc_carteira_vacinacao", "Carteira de vacinação")}
              {caixa("doc_certidao_nascimento", "Certidão de nascimento")}
              {caixa("doc_rg_cpf", "RG / CPF")}
              {caixa("doc_cartao_sus", "Cartão SUS")}
              {caixa("doc_nis", "Número do NIS")}
            </div>
          </Grupo>
        </section>
      )}

      {aba === "saude" && (
        <section className="grid gap-4 sm:grid-cols-2" aria-label="Saúde e escola">
          <Grupo titulo="Informações de saúde">
            {caixa("sinais_violencia_fisica", "Sinais de violência física")}
            {caixa("vacinacao_atualizada", "Carteira de vacinação atualizada")}
            {caixa("tratamento_odontologico", "Tratamento odontológico")}
            {caixa("tratamento_medico_psicologico", "Tratamento médico/psicológico")}
            {campo("medicamentos", "Faz uso de medicamentos? Quais?")}
            {campo("alergias", "Alergias")}
            {campo("cartao_sus", "Cartão SUS")}
            {campo("deficiencias_restricoes", "Deficiências e restrições")}
          </Grupo>
          <Grupo titulo="Vínculo escolar">
            {campo("escola", "Escola atual")}
            <div className="grid gap-3 sm:grid-cols-2">
              {campo("serie_escolar", "Ano/série")}
              <Select id="pia-periodo_escolar" label="Período" value={f.periodo_escolar} onChange={(e) => set("periodo_escolar", e.target.value)}
                options={opcoes(PERIODOS_ESCOLARES)} />
              {campo("matricula_escolar", "Matrícula")}
              {campo("frequencia_escolar", "Frequência")}
            </div>
            {caixa("aula_reforco", "Aula de reforço")}
          </Grupo>
        </section>
      )}

      {aba === "familia" && (
        <section className="flex flex-col gap-3" aria-label="Situação familiar e econômica">
          {texto("historico_familiar", "Breve relato sobre o motivo do acolhimento relacionado à família", 4)}
          <div className="grid gap-3 sm:grid-cols-2">
            {caixa("possui_beneficios", "Possui benefícios (Bolsa Família, BPC)")}
            <Select id="pia-moradia_anterior" label="Condições de moradia (anterior)" value={f.moradia_anterior}
              onChange={(e) => set("moradia_anterior", e.target.value)} options={opcoes(MORADIAS_ANTERIORES)} />
          </div>
          {texto("contato_familiar", "Contato com a família")}
          {texto("visitas_autorizadas", "Visitas autorizadas")}
          {texto("observacoes_familia", "Observações sobre a família")}
        </section>
      )}

      {aba === "plano" && (
        <section className="flex flex-col gap-3" aria-label="Plano de ação">
          <div className="overflow-x-auto rounded-lg border border-surface-border">
            <table className="w-full text-left text-sm">
              <caption className="sr-only">Plano de ação</caption>
              <thead className="bg-black/5 dark:bg-white/5">
                <tr>
                  <th className="px-2 py-2">Feita</th>
                  <th className="px-2 py-2">Necessidade identificada</th>
                  <th className="px-2 py-2">Providências</th>
                  <th className="px-2 py-2">Responsável</th>
                  <th className="px-2 py-2">Prazo</th>
                  <th className="px-2 py-2">Esportes</th>
                  <th className="px-2 py-2">Cultura</th>
                  <th className="px-2 py-2">Lazer</th>
                  <th className="px-2 py-2">Ação</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-surface-border">
                {metas("plano").length === 0 && (
                  <tr>
                    <td colSpan={9} className="px-3 py-4 text-center text-muted">
                      Nenhuma ação no plano.
                    </td>
                  </tr>
                )}
                {metas("plano").map((m) => (
                  <LinhaPlano key={m.id} meta={m} onChanged={onChanged} />
                ))}
              </tbody>
            </table>
          </div>
          <NovaMeta acolhimentoId={detalhe.id} tipo="plano" onChanged={onChanged} />
        </section>
      )}

      {aba === "rotina" && (
        <section className="grid gap-4 sm:grid-cols-2" aria-label="Rotina e atividades diárias">
          <Grupo titulo="Período diurno">
            {texto("atividades_manha", "Atividades da manhã (despertar, escola…)", 3, "Descreva a rotina matutina…")}
            {texto("atividades_tarde", "Atividades da tarde (reforço, cursos…)", 3, "Descreva a rotina vespertina…")}
          </Grupo>
          <Grupo titulo="Período noturno e finais de semana">
            {texto("atividades_noite", "Atividades da noite (jantar, higiene, dormir)", 3, "Descreva a rotina noturna…")}
            {texto("atividades_fim_semana", "Rotina de finais de semana / feriados", 3, "Lazer, passeios, visitas…")}
          </Grupo>
          <div className="sm:col-span-2">
            {texto("observacoes_comportamentais", "Observações comportamentais durante a rotina", 2,
              "Ex.: tem dificuldade para acordar, interage bem no jantar…")}
          </div>
        </section>
      )}

      {aba === "intervencao" && (
        <section className="flex flex-col gap-3" aria-label="Plano de intervenção familiar">
          {texto("intervencao_objetivos", "Objetivos da intervenção", 4, "Os objetivos a alcançar com a família…")}
          {texto("intervencao_estrategias", "Estratégias e ações propostas", 4, "Ex.: encaminhamento ao CRAS, acompanhamento psicológico…")}
          {texto("intervencao_metas_prazos", "Metas e prazos", 3, "Metas de curto, médio e longo prazo…")}
          <Grupo titulo="Metas acompanhadas">
            <ul aria-label="Metas da intervenção" className="flex flex-col gap-1 text-sm">
              {metas("intervencao").length === 0 && <li className="text-muted">Nenhuma meta acompanhada.</li>}
              {metas("intervencao").map((m) => (
                <li key={m.id} className="flex items-center gap-2">
                  {m.concluida ? <Badge tone="success">Concluída</Badge> : <Badge tone="warning">Aberta</Badge>}
                  <span>{m.descricao}</span>
                  {m.prazo && <span className="text-muted">· até {fmtDia(m.prazo)}</span>}
                </li>
              ))}
            </ul>
            <NovaMeta acolhimentoId={detalhe.id} tipo="intervencao" onChanged={onChanged} />
          </Grupo>
        </section>
      )}

      {aba !== "plano" && (
        <div className="flex justify-end">
          <Button type="button" loading={pending} onClick={() => void salvar()}>
            <Save size={14} className="mr-1" aria-hidden="true" /> Salvar P.I.A.
          </Button>
        </div>
      )}
    </div>
  );
}

/**
 * Aba do P.I.A. (Plano Individual de Atendimento) com as 6 partes do
 * sistema da unidade: identificação e documentos, saúde e escola,
 * situação familiar, plano de ação (com esportes, cultura e lazer),
 * rotina e intervenção familiar.
 */
export function PiaAba({ acolhidos, acolhimentoId, onEscolher }: {
  acolhidos: AcolhidoResumo[];
  acolhimentoId: string;
  onEscolher: (id: string) => void;
}) {
  const detalhe = useApiQuery<AcolhimentoDetalhe>(acolhimentoId ? `v1/sempras/casa-abrigo/${acolhimentoId}` : null);
  return (
    <div className="flex flex-col gap-4">
      <div className="max-w-md">
        <SeletorAcolhido id="pia-acolhido" acolhidos={acolhidos} valor={acolhimentoId} onChange={onEscolher} />
      </div>
      {acolhimentoId ? (
        <DataState loading={detalhe.isLoading} error={detalhe.error} onRetry={() => void detalhe.mutate()} empty={!detalhe.data}>
          {detalhe.data && <FichaPia key={detalhe.data.id} detalhe={detalhe.data} onChanged={() => void detalhe.mutate()} />}
        </DataState>
      ) : (
        <p className="text-sm text-muted">Escolha o acolhido para abrir o P.I.A.</p>
      )}
    </div>
  );
}
