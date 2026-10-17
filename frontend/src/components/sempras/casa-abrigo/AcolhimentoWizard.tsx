"use client";

import { useState, type FormEvent } from "react";
import { Search, Trash2 } from "lucide-react";

import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import {
  CHECKLIST_SAUDE,
  IDENTIDADES_GENERO,
  MOTIVOS_ACOLHIMENTO,
  ORIENTACOES_SEXUAIS,
  RACAS_CORES,
  RELACOES_TRABALHO,
  SIM_NAO,
  SIM_NAO_NAO_SEI,
  SITUACOES_JURIDICAS,
  VINCULOS_RESPONSAVEL,
  dadosVazios,
  dataISO,
  diaDe,
  hojeISO,
  idadeEm,
  mascaraCEP,
  mascaraCPF,
  mascaraTelefone,
  type Acolhimento,
  type AcolhimentoDados,
  type AcolhimentoDetalhe,
  type CategoriaDiagnostico,
  type Cidadao,
  type Irmao,
  type MotivoAcolhimento,
  type Resumo,
} from "@/lib/sempras/types";

import { Caixa, Escolha, Grupo, opcoes } from "./campos";
import { enviarDocumento } from "./documentos";

const PASSOS = ["Criança", "Responsável", "Acolhimento", "Jurídico", "Saúde", "Social/Fim"] as const;

interface NovoCidadao {
  nome: string;
  nome_social: string;
  cpf: string;
  sem_cpf: boolean;
  nis: string;
  data_nascimento: string;
  nome_mae: string;
}

/** Chave de um item do checklist de saúde. */
const chave = (categoria: string, nome: string) => `${categoria}|${nome}`;

/** O que do acolhimento salvo vai para o formulário (o resto é do servidor). */
function dadosDe(a: Acolhimento): AcolhimentoDados {
  const base = dadosVazios();
  const out = { ...base } as Record<string, unknown>;
  for (const k of Object.keys(base)) out[k] = (a as unknown as Record<string, unknown>)[k] ?? (base as unknown as Record<string, unknown>)[k];
  const d = out as unknown as AcolhimentoDados;
  return { ...d, responsavel_nascimento: diaDe(d.responsavel_nascimento) || null, data_guia: diaDe(d.data_guia) || null,
    data_situacao_juridica: diaDe(d.data_situacao_juridica) || null };
}

/** Datas do formulário (AAAA-MM-DD) no formato da API. */
function paraApi(d: AcolhimentoDados) {
  const data = (v: string | null) => (v ? dataISO(v) : null);
  return { ...d, responsavel_nascimento: data(d.responsavel_nascimento), data_guia: data(d.data_guia),
    data_situacao_juridica: data(d.data_situacao_juridica) };
}

/**
 * Assistente de 6 etapas do cadastro de acolhimento (ADR 032 §1), com o
 * mesmo roteiro do sistema da unidade: criança, responsável, acolhimento,
 * jurídico, saúde e contexto social. Na edição, a criança é a do cadastro
 * (editado na ficha) e irmãos e diagnósticos mudam na hora.
 */
export function AcolhimentoWizard({
  unidadeId,
  detalhe,
  cidadaoInicial,
  onConcluido,
  onCancelar,
}: {
  unidadeId: string;
  /** Edição: o acolhimento completo; cadastro: ausente. */
  detalhe?: AcolhimentoDetalhe;
  /** Cadastro a partir da ficha: a criança já escolhida. */
  cidadaoInicial?: Cidadao | Resumo;
  onConcluido: (acolhimentoId: string) => void;
  onCancelar: () => void;
}) {
  const editando = Boolean(detalhe);
  const [passo, setPasso] = useState(0);
  const [dados, setDados] = useState<AcolhimentoDados>(() => (detalhe ? dadosDe(detalhe) : dadosVazios()));
  const [dataEntrada, setDataEntrada] = useState(detalhe ? diaDe(detalhe.data_entrada) : hojeISO());
  const [erro, setErro] = useState("");
  const set = <K extends keyof AcolhimentoDados>(k: K, v: AcolhimentoDados[K]) => setDados((d) => ({ ...d, [k]: v }));

  // 1. criança: busca no cadastro único ou cadastro novo
  const [crianca, setCrianca] = useState<Cidadao | Resumo | null>(detalhe ? (detalhe.cidadao as Cidadao) : cidadaoInicial ?? null);
  const [busca, setBusca] = useState("");
  const [achados, setAchados] = useState<Resumo[] | null>(null);
  const { run: runBusca, pending: buscando } = useAction();
  const [novo, setNovo] = useState<NovoCidadao>({ nome: "", nome_social: "", cpf: "", sem_cpf: false, nis: "", data_nascimento: "", nome_mae: "" });
  const nascimento = crianca ? crianca.data_nascimento : novo.data_nascimento;
  const idade = nascimento ? idadeEm(nascimento) : null;

  async function buscar(e: FormEvent) {
    e.preventDefault();
    const res = await runBusca(() => apiClient.get<Resumo[]>(`v1/sempras/cidadaos?q=${encodeURIComponent(busca.trim())}`));
    if (res) setAchados(res.data);
  }

  // 2. responsável: endereço pelo CEP (ViaCEP, com o BrasilAPI de reserva)
  const [cepStatus, setCepStatus] = useState("");
  async function buscarCEP() {
    const cep = dados.responsavel_cep.replace(/\D/g, "");
    if (cep.length !== 8) {
      setCepStatus("Digite um CEP com 8 números.");
      return;
    }
    setCepStatus("Buscando…");
    try {
      let end: { logradouro?: string; bairro?: string; cidade?: string; uf?: string };
      try {
        const r = await fetch(`https://viacep.com.br/ws/${cep}/json/`);
        const j = await r.json();
        if (!r.ok || j.erro) throw new Error("ViaCEP");
        end = { logradouro: j.logradouro, bairro: j.bairro, cidade: j.localidade, uf: j.uf };
      } catch {
        const r = await fetch(`https://brasilapi.com.br/api/cep/v1/${cep}`);
        if (!r.ok) throw new Error("BrasilAPI");
        const j = await r.json();
        end = { logradouro: j.street, bairro: j.neighborhood, cidade: j.city, uf: j.state };
      }
      setDados((d) => ({ ...d, responsavel_logradouro: end.logradouro ?? "", responsavel_bairro: end.bairro ?? "",
        responsavel_cidade: end.cidade ?? "", responsavel_uf: end.uf ?? "" }));
      setCepStatus("");
    } catch {
      setCepStatus("CEP não encontrado.");
    }
  }

  // 3. irmãos: no cadastro vão juntos; na edição, na hora
  const [irmaos, setIrmaos] = useState<{ nome: string; esta_acolhido: boolean }[]>([]);
  const [salvos, setSalvos] = useState<Irmao[]>(detalhe?.irmaos ?? []);
  const [nomeIrmao, setNomeIrmao] = useState("");
  const [irmaoAcolhido, setIrmaoAcolhido] = useState(false);
  const { run: runIrmao } = useAction();
  async function adicionarIrmao() {
    const nome = nomeIrmao.trim();
    if (nome.length < 2) return;
    if (detalhe) {
      const res = await runIrmao(() => apiClient.post<Irmao>(`v1/sempras/casa-abrigo/${detalhe.id}/irmaos`, { nome, esta_acolhido: irmaoAcolhido }), "Irmão adicionado");
      if (res) setSalvos((l) => [...l, res.data]);
    } else {
      setIrmaos((l) => [...l, { nome, esta_acolhido: irmaoAcolhido }]);
    }
    setNomeIrmao("");
    setIrmaoAcolhido(false);
  }
  async function removerIrmao(id: string) {
    const ok = await runIrmao(() => apiClient.delete(`v1/sempras/casa-abrigo-irmaos/${id}`), "Irmão removido");
    if (ok) setSalvos((l) => l.filter((i) => i.id !== id));
  }

  // 5. checklist de saúde: cada item marcado é um diagnóstico (R6)
  const iniciais = new Map((detalhe?.diagnosticos ?? []).map((d) => [chave(d.categoria, d.diagnostico), d.id]));
  const [marcados, setMarcados] = useState<Set<string>>(() => new Set(iniciais.keys()));
  const alternar = (k: string) =>
    setMarcados((m) => {
      const n = new Set(m);
      if (n.has(k)) n.delete(k);
      else n.add(k);
      return n;
    });

  // 6. documentos (PDF), enviados depois de salvar
  const [pdfAcolhido, setPdfAcolhido] = useState<File | null>(null);
  const [pdfResponsavel, setPdfResponsavel] = useState<File | null>(null);

  const { run, pending } = useAction();

  function validar(): string {
    if (!editando && !crianca && (novo.nome.trim().length < 2 || !novo.data_nascimento)) return "Preencha o nome e a data de nascimento da criança.";
    if (!dataEntrada) return "Preencha a data na unidade.";
    if (dados.motivos.length === 0) return "Marque ao menos um motivo do acolhimento.";
    if (dados.motivos.includes("outro") && !dados.outro_motivo.trim()) return "Especifique o outro motivo do acolhimento.";
    return "";
  }

  async function finalizar() {
    const problema = validar();
    if (problema) {
      setErro(problema);
      return;
    }
    setErro("");
    const corpo = { ...paraApi(dados), data_entrada: dataISO(dataEntrada) };
    const id = await run(async () => {
      let acolhimentoId: string;
      if (detalhe) {
        await apiClient.put(`v1/sempras/casa-abrigo/${detalhe.id}`, corpo);
        acolhimentoId = detalhe.id;
        for (const [k, idDiag] of iniciais) {
          if (!marcados.has(k)) await apiClient.delete(`v1/sempras/casa-abrigo-diagnosticos/${idDiag}`);
        }
        for (const k of marcados) {
          if (iniciais.has(k)) continue;
          const [categoria, diagnostico] = k.split("|");
          await apiClient.post(`v1/sempras/casa-abrigo/${detalhe.id}/diagnosticos`, { categoria, diagnostico });
        }
      } else {
        let cidadaoId = crianca?.id;
        if (!cidadaoId) {
          const c = await apiClient.post<Cidadao>("v1/sempras/cidadaos", {
            nome: novo.nome.trim(), nome_social: novo.nome_social.trim(), cpf: novo.sem_cpf ? "" : novo.cpf, nis: novo.nis,
            data_nascimento: novo.data_nascimento, sexo: dados.sexo, nome_mae: novo.nome_mae.trim(),
          });
          cidadaoId = c.data.id;
        }
        const diagnosticos = [...marcados].map((k) => {
          const [categoria, diagnostico] = k.split("|");
          return { categoria, diagnostico };
        });
        const a = await apiClient.post<Acolhimento>(`v1/sempras/cidadaos/${cidadaoId}/casa-abrigo`, {
          ...corpo, unidade_id: unidadeId, irmaos, diagnosticos,
        });
        acolhimentoId = a.data.id;
      }
      if (pdfAcolhido) await enviarDocumento(acolhimentoId, "acolhido", pdfAcolhido);
      if (pdfResponsavel) await enviarDocumento(acolhimentoId, "responsavel", pdfResponsavel);
      return acolhimentoId;
    }, editando ? "Cadastro atualizado" : "Acolhimento cadastrado");
    if (id) onConcluido(id);
  }

  const respDesligado = dados.responsavel_nao_possui;
  const toggleMotivo = (m: MotivoAcolhimento) =>
    set("motivos", dados.motivos.includes(m) ? dados.motivos.filter((x) => x !== m) : [...dados.motivos, m]);

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-lg font-semibold">{editando ? "Editar acolhimento" : "Cadastro de acolhimento"}</h2>
        <Badge tone={editando ? "info" : "success"}>{editando ? "Editando" : "Novo registro"}</Badge>
      </div>

      <nav aria-label="Etapas do cadastro" className="flex flex-col gap-2">
        <div className="h-1.5 w-full overflow-hidden rounded-full bg-surface-border" aria-hidden="true">
          <div className="h-full bg-primary transition-all" style={{ width: `${((passo + 1) / PASSOS.length) * 100}%` }} />
        </div>
        <ol className="flex flex-wrap gap-1.5">
          {PASSOS.map((nome, i) => (
            <li key={nome}>
              <button
                type="button"
                aria-current={passo === i ? "step" : undefined}
                onClick={() => setPasso(i)}
                className={`rounded-lg px-3 py-1.5 text-xs font-medium ${passo === i ? "bg-primary text-primary-foreground" : "text-muted hover:text-foreground"}`}
              >
                {i + 1}. {nome}
              </button>
            </li>
          ))}
        </ol>
      </nav>

      {passo === 0 && (
        <section aria-label="Identificação da criança/adolescente" className="flex flex-col gap-4">
          <div className="flex flex-wrap gap-6">
            <Caixa label="Situação especial" checked={dados.situacao_especial} onChange={(v) => set("situacao_especial", v)} />
            <Caixa label="PCD" checked={dados.pcd} onChange={(v) => set("pcd", v)} />
          </div>
          {crianca ? (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-surface-border p-3 text-sm">
              <div>
                <p className="font-semibold">{crianca.nome_social || crianca.nome}</p>
                <p className="text-xs text-muted">
                  Nascimento: {crianca.data_nascimento || "não informado"}
                  {idade !== null && ` · ${idade} anos`}
                </p>
              </div>
              {!editando && (
                <Button type="button" size="sm" variant="ghost" onClick={() => setCrianca(null)}>
                  Trocar
                </Button>
              )}
            </div>
          ) : (
            <>
              <form onSubmit={buscar} className="flex flex-wrap items-end gap-2">
                <div className="min-w-60 flex-1">
                  <Input id="wizard-busca" label="Buscar no cadastro (CPF, NIS ou nome)" value={busca} onChange={(e) => setBusca(e.target.value)} autoComplete="off" />
                </div>
                <Button type="submit" variant="secondary" loading={buscando} disabled={busca.trim().length < 3}>
                  <Search size={14} className="mr-1" aria-hidden="true" /> Buscar
                </Button>
              </form>
              {achados && (
                <ul aria-label="Munícipes encontrados" className="flex flex-col gap-1 text-sm">
                  {achados.length === 0 && <li className="text-muted">Ninguém encontrado — cadastre abaixo.</li>}
                  {achados.map((r) => (
                    <li key={r.id}>
                      <button type="button" className="text-primary underline" onClick={() => setCrianca(r)}>
                        {r.nome_social || r.nome} · {r.data_nascimento || "sem nascimento"}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="sm:col-span-2">
                  <Input id="wizard-nome" label="Nome completo *" value={novo.nome} onChange={(e) => setNovo({ ...novo, nome: e.target.value })} autoComplete="off" />
                </div>
                <Input id="wizard-nome-social" label="Nome social" value={novo.nome_social} onChange={(e) => setNovo({ ...novo, nome_social: e.target.value })} />
                <div className="flex flex-col gap-1">
                  <Input id="wizard-cpf" label="CPF" value={novo.cpf} disabled={novo.sem_cpf} onChange={(e) => setNovo({ ...novo, cpf: mascaraCPF(e.target.value) })} />
                  <Caixa label="Não possui CPF" checked={novo.sem_cpf} onChange={(v) => setNovo({ ...novo, sem_cpf: v, cpf: v ? "" : novo.cpf })} />
                </div>
                <Input id="wizard-nis" label="NIS" value={novo.nis} onChange={(e) => setNovo({ ...novo, nis: e.target.value.replace(/\D/g, "").slice(0, 11) })} />
                <div className="flex flex-col gap-1">
                  <Input id="wizard-nascimento" label="Data de nascimento *" type="date" value={novo.data_nascimento}
                    onChange={(e) => setNovo({ ...novo, data_nascimento: e.target.value })} />
                  {idade !== null && <span className="text-xs text-muted">{idade} anos</span>}
                </div>
                <Input id="wizard-mae" label="Nome da mãe (cadastro)" value={novo.nome_mae} onChange={(e) => setNovo({ ...novo, nome_mae: e.target.value })} />
              </div>
            </>
          )}
          <div className="grid gap-3 sm:grid-cols-2">
            <Input id="wizard-apelido" label="Apelido" value={dados.apelido} onChange={(e) => set("apelido", e.target.value)} />
            <Select id="wizard-sexo" label="Sexo atribuído *" value={dados.sexo} onChange={(e) => set("sexo", e.target.value as AcolhimentoDados["sexo"])}
              options={[{ value: "feminino", label: "Feminino" }, { value: "masculino", label: "Masculino" }]} />
            <Select id="wizard-genero" label="Identidade de gênero" value={dados.identidade_genero} onChange={(e) => set("identidade_genero", e.target.value)}
              options={opcoes(IDENTIDADES_GENERO)} />
            <Select id="wizard-orientacao" label="Orientação sexual" value={dados.orientacao_sexual} onChange={(e) => set("orientacao_sexual", e.target.value)}
              options={opcoes(ORIENTACOES_SEXUAIS)} />
            <Select id="wizard-raca" label="Raça/cor" value={dados.raca_cor} onChange={(e) => set("raca_cor", e.target.value)} options={opcoes(RACAS_CORES)} />
          </div>
          <Grupo titulo="Filiação">
            <div className="grid gap-3 sm:grid-cols-2">
              <Input id="wizard-filiacao-1" label="Filiação 1 (mãe/pai)" value={dados.filiacao_1} onChange={(e) => set("filiacao_1", e.target.value)} />
              <Input id="wizard-filiacao-2" label="Filiação 2 (mãe/pai)" value={dados.filiacao_2} onChange={(e) => set("filiacao_2", e.target.value)} />
            </div>
          </Grupo>
        </section>
      )}

      {passo === 1 && (
        <section aria-label="Responsável antes da medida" className="flex flex-col gap-4">
          <Caixa label="Não possui responsável" checked={respDesligado} onChange={(v) => set("responsavel_nao_possui", v)} />
          <fieldset disabled={respDesligado} className={`flex flex-col gap-4 ${respDesligado ? "opacity-40" : ""}`}>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="sm:col-span-2">
                <Input id="wizard-resp-nome" label="Nome do(a) responsável" value={dados.responsavel_nome} onChange={(e) => set("responsavel_nome", e.target.value)} />
              </div>
              <Input id="wizard-resp-nome-social" label="Nome social" value={dados.responsavel_nome_social} onChange={(e) => set("responsavel_nome_social", e.target.value)} />
              <Input id="wizard-resp-apelido" label="Apelido" value={dados.responsavel_apelido} onChange={(e) => set("responsavel_apelido", e.target.value)} />
              <Input id="wizard-resp-cpf" label="CPF" value={mascaraCPF(dados.responsavel_cpf)} onChange={(e) => set("responsavel_cpf", e.target.value.replace(/\D/g, ""))} />
              <Input id="wizard-resp-nis" label="NIS" value={dados.responsavel_nis} onChange={(e) => set("responsavel_nis", e.target.value.replace(/\D/g, "").slice(0, 11))} />
              <Input id="wizard-resp-nascimento" label="Data de nascimento" type="date" value={dados.responsavel_nascimento ?? ""}
                onChange={(e) => set("responsavel_nascimento", e.target.value || null)} />
              <Select id="wizard-resp-sexo" label="Sexo" value={dados.responsavel_sexo} onChange={(e) => set("responsavel_sexo", e.target.value)}
                options={opcoes({ feminino: "Feminino", masculino: "Masculino" })} />
              <Select id="wizard-resp-vinculo" label="Grau de parentesco / vínculo" value={dados.responsavel_vinculo}
                onChange={(e) => set("responsavel_vinculo", e.target.value)} options={opcoes(VINCULOS_RESPONSAVEL)} />
            </div>
            <Grupo titulo="Endereço">
              <div className="flex flex-wrap items-end gap-2">
                <Input id="wizard-cep" label="CEP" value={mascaraCEP(dados.responsavel_cep)} onChange={(e) => set("responsavel_cep", e.target.value.replace(/\D/g, ""))} />
                <Button type="button" variant="secondary" onClick={() => void buscarCEP()}>
                  Buscar CEP
                </Button>
                {cepStatus && <span role="status" className="text-xs text-muted">{cepStatus}</span>}
              </div>
              <div className="grid gap-3 sm:grid-cols-3">
                <div className="sm:col-span-2">
                  <Input id="wizard-logradouro" label="Logradouro" value={dados.responsavel_logradouro} onChange={(e) => set("responsavel_logradouro", e.target.value)} />
                </div>
                <Input id="wizard-numero" label="Número" value={dados.responsavel_numero} onChange={(e) => set("responsavel_numero", e.target.value)} />
                <Input id="wizard-complemento" label="Complemento" value={dados.responsavel_complemento} onChange={(e) => set("responsavel_complemento", e.target.value)} />
                <Input id="wizard-bairro" label="Bairro" value={dados.responsavel_bairro} onChange={(e) => set("responsavel_bairro", e.target.value)} />
                <Input id="wizard-cidade" label="Cidade" value={dados.responsavel_cidade} onChange={(e) => set("responsavel_cidade", e.target.value)} />
                <Input id="wizard-uf" label="UF" maxLength={2} value={dados.responsavel_uf} onChange={(e) => set("responsavel_uf", e.target.value.toUpperCase())} />
              </div>
            </Grupo>
            <Grupo titulo="Contato">
              <div className="grid gap-3 sm:grid-cols-2">
                <Input id="wizard-telefone" label="Telefone fixo" value={dados.responsavel_telefone} onChange={(e) => set("responsavel_telefone", mascaraTelefone(e.target.value))} />
                <Input id="wizard-celular" label="Celular / WhatsApp" value={dados.responsavel_celular} onChange={(e) => set("responsavel_celular", mascaraTelefone(e.target.value))} />
                <div className="sm:col-span-2">
                  <Input id="wizard-email" label="E-mail" type="email" value={dados.responsavel_email} onChange={(e) => set("responsavel_email", e.target.value)} />
                </div>
              </div>
            </Grupo>
          </fieldset>
        </section>
      )}

      {passo === 2 && (
        <section aria-label="Acolhimento institucional" className="flex flex-col gap-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <Input id="wizard-entrada" label="Data na unidade *" type="date" value={dataEntrada} max={hojeISO()} onChange={(e) => setDataEntrada(e.target.value)} />
            <Input id="wizard-processo" label="Nº do 1º processo" value={dados.processo_judicial} onChange={(e) => set("processo_judicial", e.target.value)} />
            <Input id="wizard-processo-2" label="Nº do 2º processo" value={dados.processo_judicial_2} onChange={(e) => set("processo_judicial_2", e.target.value)} />
            <Input id="wizard-guia" label="Nº da guia de acolhimento" value={dados.guia_acolhimento} onChange={(e) => set("guia_acolhimento", e.target.value)} />
            <Input id="wizard-data-guia" label="Data da guia" type="date" value={dados.data_guia ?? ""} onChange={(e) => set("data_guia", e.target.value || null)} />
          </div>
          <Grupo titulo="Histórico familiar na unidade">
            <div className="grid gap-3 sm:grid-cols-3">
              <Escolha legenda="Acolhido(a) anteriormente?" valor={dados.historico_acolhido_anteriormente} opcoes={SIM_NAO_NAO_SEI}
                onChange={(v) => set("historico_acolhido_anteriormente", v)} />
              <Escolha legenda="Irmão(s) nesta unidade?" valor={dados.historico_irmaos_nesta_unidade} opcoes={SIM_NAO_NAO_SEI}
                onChange={(v) => set("historico_irmaos_nesta_unidade", v)} />
              <Escolha legenda="Irmão(s) em outra unidade?" valor={dados.historico_irmaos_outra_unidade} opcoes={SIM_NAO_NAO_SEI}
                onChange={(v) => set("historico_irmaos_outra_unidade", v)} />
            </div>
            <div className="flex flex-wrap items-end gap-2">
              <div className="min-w-60 flex-1">
                <Input id="wizard-irmao" label="Nome do irmão" value={nomeIrmao} onChange={(e) => setNomeIrmao(e.target.value)} autoComplete="off" />
              </div>
              <Caixa label="Está acolhido?" checked={irmaoAcolhido} onChange={setIrmaoAcolhido} />
              <Button type="button" variant="secondary" onClick={() => void adicionarIrmao()} disabled={nomeIrmao.trim().length < 2}>
                Adicionar irmão
              </Button>
            </div>
            <ul aria-label="Irmãos" className="flex flex-col gap-1 text-sm">
              {salvos.length + irmaos.length === 0 && <li className="text-muted">Nenhum irmão cadastrado.</li>}
              {salvos.map((i) => (
                <li key={i.id} className="flex items-center justify-between">
                  <span>{i.nome} {i.esta_acolhido ? <Badge tone="info">Acolhido</Badge> : <span className="text-muted">(não acolhido)</span>}</span>
                  <Button type="button" size="sm" variant="ghost" aria-label={`Remover ${i.nome}`} onClick={() => void removerIrmao(i.id)}>
                    <Trash2 size={14} aria-hidden="true" />
                  </Button>
                </li>
              ))}
              {irmaos.map((i, n) => (
                <li key={`${i.nome}-${n}`} className="flex items-center justify-between">
                  <span>{i.nome} {i.esta_acolhido ? <Badge tone="info">Acolhido</Badge> : <span className="text-muted">(não acolhido)</span>}</span>
                  <Button type="button" size="sm" variant="ghost" aria-label={`Remover ${i.nome}`} onClick={() => setIrmaos((l) => l.filter((_, k) => k !== n))}>
                    <Trash2 size={14} aria-hidden="true" />
                  </Button>
                </li>
              ))}
            </ul>
          </Grupo>
          <Grupo titulo="Motivo do acolhimento (marque as opções aplicáveis) *">
            <div className="grid gap-2 sm:grid-cols-2">
              {(Object.entries(MOTIVOS_ACOLHIMENTO) as [MotivoAcolhimento, string][]).map(([m, label]) => (
                <Caixa key={m} label={label} checked={dados.motivos.includes(m)} onChange={() => toggleMotivo(m)} />
              ))}
            </div>
            {dados.motivos.includes("outro") && (
              <Input id="wizard-outro-motivo" label="Especifique o outro motivo *" value={dados.outro_motivo} onChange={(e) => set("outro_motivo", e.target.value)} />
            )}
          </Grupo>
        </section>
      )}

      {passo === 3 && (
        <section aria-label="Situação jurídica" className="flex flex-col gap-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <div className="sm:col-span-2">
              <Select id="wizard-situacao" label="Situação jurídica atual" value={dados.situacao_juridica}
                onChange={(e) => set("situacao_juridica", e.target.value as AcolhimentoDados["situacao_juridica"])}
                options={opcoes(SITUACOES_JURIDICAS, "Selecione a situação legal atual…")} />
            </div>
            <Input id="wizard-data-situacao" label="Data da situação jurídica" type="date" value={dados.data_situacao_juridica ?? ""}
              onChange={(e) => set("data_situacao_juridica", e.target.value || null)} />
          </div>
          <Grupo titulo="Rede de proteção / Conselho Tutelar">
            <div className="grid gap-3 sm:grid-cols-2">
              <Input id="wizard-conselheiro" label="Conselheiro(a) tutelar responsável" value={dados.conselheiro_nome} onChange={(e) => set("conselheiro_nome", e.target.value)} />
              <Input id="wizard-tel-conselheiro" label="Telefone do conselheiro" value={dados.conselheiro_telefone}
                onChange={(e) => set("conselheiro_telefone", mascaraTelefone(e.target.value))} />
            </div>
          </Grupo>
          <Grupo titulo="Cadastros nacionais">
            <div className="grid gap-3 sm:grid-cols-2">
              <Escolha legenda="Está no Cadastro Nacional de Adoção (CNA)?" valor={dados.cna} opcoes={SIM_NAO_NAO_SEI} onChange={(v) => set("cna", v)} />
              <Escolha legenda="Está no Cadastro Nacional de Desaparecidos (CNCAD)?" valor={dados.cncad} opcoes={SIM_NAO_NAO_SEI} onChange={(v) => set("cncad", v)} />
            </div>
          </Grupo>
        </section>
      )}

      {passo === 4 && (
        <section aria-label="Saúde e diagnósticos" className="flex flex-col gap-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <Escolha legenda="Possui laudo/diagnóstico médico?" valor={dados.laudo_medico} opcoes={SIM_NAO} onChange={(v) => set("laudo_medico", v)} />
            <Escolha legenda="Uso de medicação contínua?" valor={dados.medicacao_continua} opcoes={SIM_NAO} onChange={(v) => set("medicacao_continua", v)} />
            <Escolha legenda="Possui dependência química?" valor={dados.dependencia_quimica} opcoes={SIM_NAO} onChange={(v) => set("dependencia_quimica", v)} />
          </div>
          {dados.laudo_medico === "sim" && (
            <Input id="wizard-laudo" label="Descrição do laudo/diagnóstico" value={dados.laudo_medico_descricao} onChange={(e) => set("laudo_medico_descricao", e.target.value)} />
          )}
          {CHECKLIST_SAUDE.filter((g) => g.categoria !== "transtorno").map((g) => (
            <Grupo key={g.categoria} titulo={g.titulo}>
              <div className="grid gap-2 sm:grid-cols-3">
                {g.itens.map((item) => (
                  <Caixa key={item} label={item} checked={marcados.has(chave(g.categoria, item))} onChange={() => alternar(chave(g.categoria, item))} />
                ))}
              </div>
            </Grupo>
          ))}
          <Escolha legenda="Possui transtornos, síndromes ou deficiência intelectual?" valor={dados.possui_transtorno} opcoes={SIM_NAO}
            onChange={(v) => set("possui_transtorno", v)} />
          {dados.possui_transtorno === "sim" &&
            CHECKLIST_SAUDE.filter((g) => g.categoria === "transtorno").map((g) => (
              <Grupo key={g.categoria} titulo={g.titulo}>
                <div className="grid gap-2 sm:grid-cols-3">
                  {g.itens.map((item) => (
                    <Caixa key={item} label={item} checked={marcados.has(chave(g.categoria, item))} onChange={() => alternar(chave(g.categoria as CategoriaDiagnostico, item))} />
                  ))}
                </div>
                <Input id="wizard-transtorno" label="Descrição do transtorno / síndrome / deficiência" value={dados.transtorno_descricao}
                  onChange={(e) => set("transtorno_descricao", e.target.value)} />
              </Grupo>
            ))}
        </section>
      )}

      {passo === 5 && (
        <section aria-label="Contexto social" className="flex flex-col gap-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <Escolha legenda="Está estudando?" valor={dados.estuda} opcoes={SIM_NAO} onChange={(v) => set("estuda", v)} />
            <Escolha legenda="Ameaça de morte?" valor={dados.ameaca_morte} opcoes={SIM_NAO} onChange={(v) => set("ameaca_morte", v)} />
            <Escolha legenda="Recebe visita institucional?" valor={dados.recebe_visita} opcoes={SIM_NAO} onChange={(v) => set("recebe_visita", v)} />
            <Escolha legenda="Possui benefício do governo?" valor={dados.beneficio_governo} opcoes={SIM_NAO} onChange={(v) => set("beneficio_governo", v)} />
            <Escolha legenda="Participa de programa social ou governamental?" valor={dados.programa_social} opcoes={SIM_NAO}
              onChange={(v) => set("programa_social", v)} />
            <Select id="wizard-trabalho" label="Está em uma relação de trabalho?" value={dados.relacao_trabalho}
              onChange={(e) => set("relacao_trabalho", e.target.value)} options={opcoes(RELACOES_TRABALHO)} />
          </div>
          {dados.beneficio_governo === "sim" && (
            <Input id="wizard-beneficio" label="Qual benefício?" placeholder="Ex.: Bolsa Família, BPC…" value={dados.beneficio_qual}
              onChange={(e) => set("beneficio_qual", e.target.value)} />
          )}
          <Textarea id="wizard-observacoes" label="Observações gerais" rows={3} value={dados.observacoes_gerais} onChange={(e) => set("observacoes_gerais", e.target.value)} />
          <Grupo titulo="Documentos anexos (PDF, opcional)">
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="flex flex-col gap-1 text-sm">
                Documentos do acolhido
                <input type="file" accept="application/pdf" onChange={(e) => setPdfAcolhido(e.target.files?.[0] ?? null)} />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                Documentos do responsável
                <input type="file" accept="application/pdf" onChange={(e) => setPdfResponsavel(e.target.files?.[0] ?? null)} />
              </label>
            </div>
          </Grupo>
          <p className="text-xs text-muted">O desligamento é feito pela ação própria, na lista de acolhidos ou na ficha.</p>
        </section>
      )}

      {erro && (
        <p role="alert" className="text-sm text-danger">
          {erro}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2 border-t border-surface-border pt-4">
        <Button type="button" variant="ghost" onClick={onCancelar}>
          Cancelar
        </Button>
        <div className="flex-1" />
        {passo > 0 && (
          <Button type="button" variant="secondary" onClick={() => setPasso(passo - 1)}>
            ← Voltar
          </Button>
        )}
        {passo < PASSOS.length - 1 ? (
          <Button type="button" onClick={() => setPasso(passo + 1)}>
            Próxima etapa →
          </Button>
        ) : (
          <Button type="button" loading={pending} onClick={() => void finalizar()}>
            {editando ? "Salvar alterações" : "Finalizar cadastro"}
          </Button>
        )}
      </div>
    </div>
  );
}
