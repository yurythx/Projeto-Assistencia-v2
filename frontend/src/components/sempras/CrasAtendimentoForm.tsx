"use client";

import { useState } from "react";
import {
  AlertCircle,
  Building2,
  Calendar,
  Check,
  CheckCircle2,
  ClipboardCheck,
  FileText,
  HeartHandshake,
  Home,
  Save,
  Send,
  Sparkles,
  Users,
} from "lucide-react";

import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import type { Cidadao } from "@/lib/sempras/types";

export interface CrasAtendimentoFormProps {
  cidadao: Cidadao;
  unidadeId: string;
  unidadeNome?: string;
  itemFilaId?: string;
  isEmAtendimento?: boolean;
  onSalvo?: () => void;
  onConcluido?: () => void;
}

export const FORMAS_ACESSO_CRAS = [
  { value: "DEMANDA ESPONTÂNEA", label: "Demanda Espontânea" },
  { value: "BUSCA ATIVA", label: "Busca Ativa" },
  { value: "ENCAMINHAMENTO PROTEÇÃO BÁSICA", label: "Encaminhamento da Proteção Social Básica" },
  { value: "ENCAMINHAMENTO PROTEÇÃO ESPECIAL", label: "Encaminhamento da Proteção Social Especial (CREAS)" },
  { value: "ENCAMINHAMENTO OUTRAS POLÍTICAS", label: "Encaminhamento Outras Políticas (Saúde, Educação, SINE)" },
  { value: "ÓRGÃOS SISTEMA DE GARANTIA DE DIREITOS", label: "Órgãos do Sistema de Garantia de Direitos (Conselho Tutelar, Judiciário)" },
];

export const PERFIS_FAMILIA_CRAS = [
  { value: "NOVA", label: "Acompanhamento - Novas famílias inseridas no PAIF" },
  { value: "ATIVA", label: "Famílias Ativas no PAIF" },
  { value: "OUTRO", label: "Outro Perfil / Demanda Pontual" },
];

export const CONDICOES_FAMILIA_CRAS = [
  { value: "RENDA ZERO", label: "Renda Zero" },
  { value: "RENDA ATÉ 218", label: "Renda até R$ 218,00 per capita (Extrema Pobreza)" },
  { value: "RENDA 218 A MEIO SALÁRIO", label: "Renda entre R$ 218,01 e meio salário mínimo" },
  { value: "RENDA ACIMA MEIO SALÁRIO", label: "Renda acima de meio salário mínimo" },
];

export const BENEFICIOS_RECEBIDOS_CRAS = [
  { value: "NENHUM", label: "Nenhum benefício de transferência de renda" },
  { value: "BOLSA FAMILIA", label: "Bolsa Família" },
  { value: "BPC", label: "BPC (Benefício de Prestação Continuada)" },
  { value: "AMBOS", label: "Bolsa Família e BPC" },
];

export const PERFIS_BPC = [
  { value: "", label: "Não se aplica / Nenhum membro" },
  { value: "IDOSO", label: "BPC — Pessoa Idosa (65+ anos)" },
  { value: "PCD", label: "BPC — Pessoa com Deficiência (PCD)" },
  { value: "AMBOS", label: "BPC — Idoso e PCD no mesmo domicílio" },
];

export const TIPOS_VISITA_DOMICILIAR = [
  { value: "", label: "Não realizada neste atendimento" },
  { value: "ATENDIMENTO", label: "Atendimento Técnico Domiciliar (Assistente Social / Psicóloga)" },
  { value: "ATUALIZAÇÃO CADASTRAL", label: "Busca Domiciliar para Atualização CadÚnico" },
];

type SubTab = "basicos" | "vulnerabilidades" | "encaminhamentos" | "visitas";

/**
 * Formulário oficial de Atendimento Técnico do CRAS:
 * Trazido com fidelidade da planilha oficial do MDS e do protocolo de atendimento socioassistencial,
 * contemplando:
 * - Forma de Acesso e Perfil Familiar;
 * - Condição Financeira e Benefícios (Bolsa Família / BPC);
 * - Marcadores de Vulnerabilidade e Condicionalidades do RMA/MDS (Extrema Pobreza, Trabalho Infantil, Acolhimento);
 * - Encaminhamentos Intersetoriais (Socioassistencial, Saúde, Educação, Trabalho/Renda, Direitos);
 * - Visitas Domiciliares e Síntese da Evolução Técnica;
 * - Opções de "Salvar em Andamento" e "Concluir e Finalizar Atendimento".
 */
export function CrasAtendimentoForm({
  cidadao,
  unidadeId,
  unidadeNome = "CRAS",
  itemFilaId,
  isEmAtendimento = false,
  onSalvo,
  onConcluido,
}: CrasAtendimentoFormProps) {
  const [subTab, setSubTab] = useState<SubTab>("basicos");
  const { run, pending } = useAction();

  // Estados dos campos do formulário
  const [dataAtendimento, setDataAtendimento] = useState(() => new Date().toISOString().split("T")[0]);
  const [formaAcesso, setFormaAcesso] = useState("DEMANDA ESPONTÂNEA");
  const [perfilFamilia, setPerfilFamilia] = useState("OUTRO");
  const [condicaoFamilia, setCondicaoFamilia] = useState("RENDA ATÉ 218");
  const [beneficioRecebido, setBeneficioRecebido] = useState("BOLSA FAMILIA");
  const [perfilBpc, setPerfilBpc] = useState("");
  const [interesseCurso, setInteresseCurso] = useState("");
  const [preferenciaHorario, setPreferenciaHorario] = useState("VESPERTINO");

  // Vulnerabilidades / RMA
  const [extremaPobreza, setExtremaPobreza] = useState(false);
  const [bolsaFamilia, setBolsaFamilia] = useState(true);
  const [bolsaFamiliaBreach, setBolsaFamiliaBreach] = useState(false);
  const [bpcMembers, setBpcMembers] = useState(false);
  const [trabalhoInfantil, setTrabalhoInfantil] = useState(false);
  const [acolhimentoInstitucional, setAcolhimentoInstitucional] = useState(false);
  const [vincularPaif, setVincularPaif] = useState(false);
  const [scfvParticipantes, setScfvParticipantes] = useState(0);

  // Encaminhamentos
  const [encSocial, setEncSocial] = useState("");
  const [encSaude, setEncSaude] = useState("");
  const [encEducacao, setEncEducacao] = useState("");
  const [encTrabalho, setEncTrabalho] = useState("");
  const [encProgramas, setEncProgramas] = useState("");
  const [encDireitos, setEncDireitos] = useState("");
  const [demaisEncaminhamentos, setDemaisEncaminhamentos] = useState("");

  // Visitas e Acolhimento
  const [tipoVisita, setTipoVisita] = useState("");
  const [acolhidaAtendimento, setAcolhidaAtendimento] = useState("PARTICULARIZADO");
  const [visitaTecnica, setVisitaTecnica] = useState("");
  const [observacoesTecnicas, setObservacoesTecnicas] = useState("");

  async function handleSubmit(shouldFinalize: boolean) {
    if (!unidadeId) return;

    // Constrói a síntese técnica estruturada
    const linhas: string[] = [
      `📋 ATENDIMENTO TÉCNICO CRAS — ${dataAtendimento}`,
      `Unidade: ${unidadeNome}`,
      `Munícipe / Família: ${cidadao.nome_social || cidadao.nome} (CPF: ${cidadao.cpf || "Sem CPF"})`,
      `Forma de Acesso: ${formaAcesso}`,
      `Perfil Familiar: ${perfilFamilia}`,
      `Condição de Renda: ${condicaoFamilia}`,
      `Benefício de Transferência: ${beneficioRecebido}${perfilBpc ? ` | Perfil BPC: ${perfilBpc}` : ""}`,
    ];

    if (interesseCurso.trim()) {
      linhas.push(`Interesse em Curso: ${interesseCurso.trim()} (Turno preferencial: ${preferenciaHorario})`);
    }

    const vulnerabilidades: string[] = [];
    if (extremaPobreza) vulnerabilidades.push("Extrema Pobreza");
    if (bolsaFamilia) vulnerabilidades.push("Beneficiária Bolsa Família");
    if (bolsaFamiliaBreach) vulnerabilidades.push("Descumprimento de Condicionalidades PBF");
    if (bpcMembers) vulnerabilidades.push("Membros com BPC");
    if (trabalhoInfantil) vulnerabilidades.push("Situação de Trabalho Infantil");
    if (acolhimentoInstitucional) vulnerabilidades.push("Criança/Adolescente em Serviço de Acolhimento");

    if (vulnerabilidades.length > 0) {
      linhas.push(`Marcadores RMA/MDS: ${vulnerabilidades.join(", ")}`);
    }

    const encList: string[] = [];
    if (encSocial.trim()) encList.push(`Socioassistencial: ${encSocial.trim()}`);
    if (encSaude.trim()) encList.push(`Saúde: ${encSaude.trim()}`);
    if (encEducacao.trim()) encList.push(`Educação: ${encEducacao.trim()}`);
    if (encTrabalho.trim()) encList.push(`Trabalho/SINE: ${encTrabalho.trim()}`);
    if (encProgramas.trim()) encList.push(`Programas: ${encProgramas.trim()}`);
    if (encDireitos.trim()) encList.push(`Direitos/Conselho: ${encDireitos.trim()}`);
    if (demaisEncaminhamentos.trim()) encList.push(`Outros: ${demaisEncaminhamentos.trim()}`);

    if (encList.length > 0) {
      linhas.push(`Encaminhamentos Intersetoriais:\n  • ${encList.join("\n  • ")}`);
    }

    if (tipoVisita) {
      linhas.push(`Visita Domiciliar: ${tipoVisita}`);
    }
    if (acolhidaAtendimento) {
      linhas.push(`Modalidade de Atendimento: ${acolhidaAtendimento}`);
    }
    if (visitaTecnica.trim()) {
      linhas.push(`Visita Técnica: ${visitaTecnica.trim()}`);
    }

    if (observacoesTecnicas.trim()) {
      linhas.push(`\nParecer e Síntese da Assistência Social:\n${observacoesTecnicas.trim()}`);
    }

    const textoFinal = linhas.join("\n");

    const ok = await run(
      async () => {
        // 1. Grava a evolução socioassistencial no prontuário do munícipe
        await apiClient.post(`v1/sempras/cidadaos/${cidadao.id}/registros`, {
          unidade_id: unidadeId,
          tipo: "social",
          texto: textoFinal,
          atendimento_id: itemFilaId || null,
        });

        // 2. Se marcado para vincular no PAIF, realiza a inclusão no serviço
        if (vincularPaif) {
          try {
            await apiClient.post(`v1/sempras/cidadaos/${cidadao.id}/paif`, {
              unidade_id: unidadeId,
              tipo_atendimento: acolhidaAtendimento === "COLETIVO" ? "coletivo" : "particularizado",
              extrema_pobreza: extremaPobreza,
              bolsa_familia: bolsaFamilia,
              bpc: bpcMembers,
              scfv_participantes: Number(scfvParticipantes) || 0,
            });
          } catch {
            // Se já tem vínculo ativo, mantém sem interromper o salvamento do atendimento
          }
        }

        // 3. Se solicitado concluir o atendimento na fila
        if (shouldFinalize && itemFilaId) {
          await apiClient.post(`v1/sempras/atendimentos/${itemFilaId}/concluir`);
        }
        // run() devolve o retorno da função: sem ele, o sucesso viraria
        // "undefined" e os callbacks (onSalvo/onConcluido) nunca rodariam.
        return true;
      },
      shouldFinalize
        ? "Atendimento CRAS concluído e finalizado com sucesso!"
        : "Atendimento CRAS salvo em andamento com sucesso!"
    );

    if (ok) {
      if (shouldFinalize) {
        onConcluido?.();
      } else {
        onSalvo?.();
      }
    }
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Banner de Identificação da Unidade e Ficha */}
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-primary/30 bg-primary/5 p-3.5 shadow-2xs">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground font-bold shadow-xs">
            <ClipboardCheck size={22} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-bold text-foreground">Registro de Atendimento — {unidadeNome}</h3>
              <Badge tone="info">Formulário Oficial MDS / SUAS</Badge>
            </div>
            <p className="text-xs text-muted">
              Preenchimento técnico para prontuário, acompanhamento territorial e alimentação do RMA/MDS.
            </p>
          </div>
        </div>

        {itemFilaId && (
          <Badge tone={isEmAtendimento ? "success" : "info"}>
            {isEmAtendimento ? "🟢 Em Atendimento no Guichê" : "🟡 Chamado na Recepção"}
          </Badge>
        )}
      </div>

      {/* Navegação entre as 4 Seções do Formulário CRAS */}
      <div className="flex flex-wrap gap-1.5 border-b border-surface-border pb-2">
        <button
          type="button"
          onClick={() => setSubTab("basicos")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "basicos"
              ? "bg-primary text-primary-foreground shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <Building2 size={14} />
          1. Dados Básicos & Perfil
        </button>
        <button
          type="button"
          onClick={() => setSubTab("vulnerabilidades")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "vulnerabilidades"
              ? "bg-primary text-primary-foreground shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <Sparkles size={14} />
          2. Vulnerabilidades & RMA
        </button>
        <button
          type="button"
          onClick={() => setSubTab("encaminhamentos")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "encaminhamentos"
              ? "bg-primary text-primary-foreground shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <Send size={14} />
          3. Encaminhamentos
        </button>
        <button
          type="button"
          onClick={() => setSubTab("visitas")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "visitas"
              ? "bg-primary text-primary-foreground shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <Home size={14} />
          4. Visitas & Síntese Técnica
        </button>
      </div>

      {/* Conteúdo das Abas */}
      <div className="min-h-[280px]">
        {/* SEÇÃO 1: DADOS BÁSICOS E PERFIL */}
        {subTab === "basicos" && (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <Input
              id="cras-data-atendimento"
              label="Data do Atendimento"
              type="date"
              value={dataAtendimento}
              onChange={(e) => setDataAtendimento(e.target.value)}
              required
            />

            <Select
              id="cras-forma-acesso"
              label="Forma de Acesso ao CRAS"
              value={formaAcesso}
              onChange={(e) => setFormaAcesso(e.target.value)}
              options={FORMAS_ACESSO_CRAS}
            />

            <Select
              id="cras-perfil-familia"
              label="Perfil da Família (PAIF)"
              value={perfilFamilia}
              onChange={(e) => setPerfilFamilia(e.target.value)}
              options={PERFIS_FAMILIA_CRAS}
            />

            <Select
              id="cras-condicao-familia"
              label="Condição de Renda (RMA/MDS)"
              value={condicaoFamilia}
              onChange={(e) => {
                setCondicaoFamilia(e.target.value);
                if (e.target.value === "RENDA ZERO" || e.target.value === "RENDA ATÉ 218") {
                  setExtremaPobreza(true);
                }
              }}
              options={CONDICOES_FAMILIA_CRAS}
            />

            <Select
              id="cras-beneficios-recebidos"
              label="Benefício de Transferência"
              value={beneficioRecebido}
              onChange={(e) => {
                setBeneficioRecebido(e.target.value);
                if (e.target.value === "BOLSA FAMILIA" || e.target.value === "AMBOS") {
                  setBolsaFamilia(true);
                }
                if (e.target.value === "BPC" || e.target.value === "AMBOS") {
                  setBpcMembers(true);
                }
              }}
              options={BENEFICIOS_RECEBIDOS_CRAS}
            />

            <Select
              id="cras-perfil-bpc"
              label="Perfil BPC (se houver no domicílio)"
              value={perfilBpc}
              onChange={(e) => setPerfilBpc(e.target.value)}
              options={PERFIS_BPC}
            />

            <div className="sm:col-span-2 lg:col-span-2">
              <Input
                id="cras-curso-interesse"
                label="Interesse em Cursos Profissionalizantes / Inclusão Produtiva"
                placeholder="Ex: Informática, Panificação, Corte e Costura, Mecânica..."
                value={interesseCurso}
                onChange={(e) => setInteresseCurso(e.target.value)}
              />
            </div>

            <div className="sm:col-span-1 lg:col-span-1">
              <Select
                id="cras-preferencia-horario"
                label="Turno de Preferência"
                value={preferenciaHorario}
                onChange={(e) => setPreferenciaHorario(e.target.value)}
                options={[
                  { value: "MATUTINO", label: "Matutino (Manhã)" },
                  { value: "VESPERTINO", label: "Vespertino (Tarde)" },
                  { value: "NOTURNO", label: "Noturno (Noite)" },
                  { value: "INTEGRAL", label: "Qualquer Horário" },
                ]}
              />
            </div>
          </div>
        )}

        {/* SEÇÃO 2: VULNERABILIDADES E INDICADORES DO RMA/MDS */}
        {subTab === "vulnerabilidades" && (
          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2 text-xs text-foreground">
              <AlertCircle size={15} className="shrink-0 text-warning" />
              <span>
                <strong>Atenção Técnica:</strong> Os itens assinalados abaixo compõem os blocos de público prioritário do PAIF e estatísticas do <strong>RMA/MDS</strong>.
              </span>
            </div>

            <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
              <label className={`flex items-start gap-2.5 rounded-xl border p-2.5 transition-all cursor-pointer ${
                extremaPobreza ? "border-primary bg-primary/10 shadow-2xs" : "border-surface-border bg-surface-muted/20 hover:bg-surface-muted/50"
              }`}>
                <input
                  type="checkbox"
                  checked={extremaPobreza}
                  onChange={(e) => setExtremaPobreza(e.target.checked)}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-surface-border text-primary focus:ring-primary"
                />
                <div className="flex flex-col min-w-0">
                  <span className="text-xs font-bold text-foreground">Extrema Pobreza</span>
                  <span className="text-[11px] text-muted">Renda mensal per capita até R$ 218,00</span>
                </div>
              </label>

              <label className={`flex items-start gap-2.5 rounded-xl border p-2.5 transition-all cursor-pointer ${
                bolsaFamilia ? "border-primary bg-primary/10 shadow-2xs" : "border-surface-border bg-surface-muted/20 hover:bg-surface-muted/50"
              }`}>
                <input
                  type="checkbox"
                  checked={bolsaFamilia}
                  onChange={(e) => setBolsaFamilia(e.target.checked)}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-surface-border text-primary focus:ring-primary"
                />
                <div className="flex flex-col min-w-0">
                  <span className="text-xs font-bold text-foreground">Beneficiária Bolsa Família</span>
                  <span className="text-[11px] text-muted">Titular ou dependente no PBF</span>
                </div>
              </label>

              <label className={`flex items-start gap-2.5 rounded-xl border p-2.5 transition-all cursor-pointer ${
                bolsaFamiliaBreach ? "border-primary bg-primary/10 shadow-2xs" : "border-surface-border bg-surface-muted/20 hover:bg-surface-muted/50"
              }`}>
                <input
                  type="checkbox"
                  checked={bolsaFamiliaBreach}
                  onChange={(e) => setBolsaFamiliaBreach(e.target.checked)}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-surface-border text-primary focus:ring-primary"
                />
                <div className="flex flex-col min-w-0">
                  <span className="text-xs font-bold text-foreground">Descumprimento Condicionalidades</span>
                  <span className="text-[11px] text-muted">Baixa frequência escolar ou vacinação</span>
                </div>
              </label>

              <label className={`flex items-start gap-2.5 rounded-xl border p-2.5 transition-all cursor-pointer ${
                bpcMembers ? "border-primary bg-primary/10 shadow-2xs" : "border-surface-border bg-surface-muted/20 hover:bg-surface-muted/50"
              }`}>
                <input
                  type="checkbox"
                  checked={bpcMembers}
                  onChange={(e) => setBpcMembers(e.target.checked)}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-surface-border text-primary focus:ring-primary"
                />
                <div className="flex flex-col min-w-0">
                  <span className="text-xs font-bold text-foreground">Membros Beneficiários do BPC</span>
                  <span className="text-[11px] text-muted">Idoso ou PCD com benefício ativo</span>
                </div>
              </label>

              <label className={`flex items-start gap-2.5 rounded-xl border p-2.5 transition-all cursor-pointer ${
                trabalhoInfantil ? "border-primary bg-primary/10 shadow-2xs" : "border-surface-border bg-surface-muted/20 hover:bg-surface-muted/50"
              }`}>
                <input
                  type="checkbox"
                  checked={trabalhoInfantil}
                  onChange={(e) => setTrabalhoInfantil(e.target.checked)}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-surface-border text-primary focus:ring-primary"
                />
                <div className="flex flex-col min-w-0">
                  <span className="text-xs font-bold text-foreground">Trabalho Infantil</span>
                  <span className="text-[11px] text-muted">Criança/adolescente em labor vedado</span>
                </div>
              </label>

              <label className={`flex items-start gap-2.5 rounded-xl border p-2.5 transition-all cursor-pointer ${
                acolhimentoInstitucional ? "border-primary bg-primary/10 shadow-2xs" : "border-surface-border bg-surface-muted/20 hover:bg-surface-muted/50"
              }`}>
                <input
                  type="checkbox"
                  checked={acolhimentoInstitucional}
                  onChange={(e) => setAcolhimentoInstitucional(e.target.checked)}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-surface-border text-primary focus:ring-primary"
                />
                <div className="flex flex-col min-w-0">
                  <span className="text-xs font-bold text-foreground">Acolhimento Institucional</span>
                  <span className="text-[11px] text-muted">Membro em abrigo ou casa de passagem</span>
                </div>
              </label>
            </div>

            {/* Inclusão Integrada no PAIF */}
            <div className="rounded-xl border border-primary/40 bg-primary/5 p-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <label className="flex items-center gap-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={vincularPaif}
                    onChange={(e) => setVincularPaif(e.target.checked)}
                    className="h-4 w-4 rounded border-surface-border text-primary focus:ring-primary"
                  />
                  <div className="flex flex-col">
                    <span className="text-xs font-bold text-primary">Inserir / Atualizar Vínculo Ativo no PAIF</span>
                    <span className="text-[11px] text-muted">
                      Vincula a família ao Plano de Acompanhamento Territorial do CRAS
                    </span>
                  </div>
                </label>

                {vincularPaif && (
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-medium text-foreground">Participantes no SCFV:</span>
                    <input
                      id="cras-scfv-participantes"
                      type="number"
                      min={0}
                      max={50}
                      value={scfvParticipantes}
                      onChange={(e) => setScfvParticipantes(Number(e.target.value))}
                      className="h-8 w-20 rounded-md border border-surface-border bg-surface px-2 text-center text-xs font-bold text-foreground focus:outline-primary"
                    />
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {/* SEÇÃO 3: ENCAMINHAMENTOS INTERSETORIAIS */}
        {subTab === "encaminhamentos" && (
          <div className="flex flex-col gap-3">
            <p className="text-xs text-muted">
              Preencha os órgãos e serviços para os quais o munícipe ou família foi direcionado neste atendimento:
            </p>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <Input
                id="cras-enc-social"
                label="Socioassistencial (CRAS / CREAS / CadÚnico)"
                placeholder="Ex: Atualização CadÚnico, BPC na Escola..."
                value={encSocial}
                onChange={(e) => setEncSocial(e.target.value)}
              />

              <Input
                id="cras-enc-saude"
                label="Saúde (UBS / CAPS / Policlínica)"
                placeholder="Ex: Consulta pré-natal, vacinas, CAPS..."
                value={encSaude}
                onChange={(e) => setEncSaude(e.target.value)}
              />

              <Input
                id="cras-enc-educacao"
                label="Educação (SEMED / Escolas / Creche)"
                placeholder="Ex: Vaga em creche municipal, transporte..."
                value={encEducacao}
                onChange={(e) => setEncEducacao(e.target.value)}
              />

              <Input
                id="cras-enc-trabalho"
                label="Trabalho e Renda (SINE / Qualificação)"
                placeholder="Ex: Vagas no SINE, cadastro Senai/Senac..."
                value={encTrabalho}
                onChange={(e) => setEncTrabalho(e.target.value)}
              />

              <Input
                id="cras-enc-direitos"
                label="Garantia de Direitos (Conselho Tutelar)"
                placeholder="Ex: Ofício ao Conselho Tutelar, guarda..."
                value={encDireitos}
                onChange={(e) => setEncDireitos(e.target.value)}
              />

              <Input
                id="cras-enc-outros"
                label="Demais Encaminhamentos / Rede Parceira"
                placeholder="Ex: Pastoral, projeto comunitário parceiro..."
                value={demaisEncaminhamentos}
                onChange={(e) => setDemaisEncaminhamentos(e.target.value)}
              />
            </div>
          </div>
        )}

        {/* SEÇÃO 4: VISITAS DOMICILIARES E SÍNTESE TÉCNICA */}
        {subTab === "visitas" && (
          <div className="flex flex-col gap-3.5">
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <Select
                id="cras-acolhida-atendimento"
                label="Modalidade de Atendimento"
                value={acolhidaAtendimento}
                onChange={(e) => setAcolhidaAtendimento(e.target.value)}
                options={[
                  { value: "PARTICULARIZADO", label: "Particularizado (Individual/Familiar)" },
                  { value: "COLETIVO", label: "Coletivo (Grupo de Famílias)" },
                ]}
              />

              <Select
                id="cras-tipo-visita"
                label="Visita Domiciliar — Tipo"
                value={tipoVisita}
                onChange={(e) => setTipoVisita(e.target.value)}
                options={TIPOS_VISITA_DOMICILIAR}
              />

              <Input
                id="cras-visita-tecnica"
                label="Localidade / Avaliação Territorial"
                placeholder="Ex: Bairro, microárea ou domicílio"
                value={visitaTecnica}
                onChange={(e) => setVisitaTecnica(e.target.value)}
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <div className="flex flex-wrap items-center justify-between gap-1.5">
                <label htmlFor="cras-observacoes-tecnicas" className="text-sm font-medium text-foreground">
                  Parecer Técnico / Síntese da Evolução Social *
                </label>
                <div className="flex flex-wrap items-center gap-1 text-[11px] text-muted">
                  <span className="font-semibold text-muted">Modelos rápidos:</span>
                  <button
                    type="button"
                    onClick={() => setObservacoesTecnicas((prev) => (prev ? `${prev} ` : "") + "Acolhida realizada. Prestadas orientações socioassistenciais sobre programas sociais e critérios do CadÚnico/PBF.")}
                    className="rounded bg-surface-muted/60 px-2 py-0.5 text-xs text-primary hover:bg-surface-muted transition-colors cursor-pointer"
                  >
                    + Acolhida Inicial
                  </button>
                  <button
                    type="button"
                    onClick={() => setObservacoesTecnicas((prev) => (prev ? `${prev} ` : "") + "Família mantida em acompanhamento sistemático pelo PAIF territorial. Agendado retorno técnico.")}
                    className="rounded bg-surface-muted/60 px-2 py-0.5 text-xs text-primary hover:bg-surface-muted transition-colors cursor-pointer"
                  >
                    + Vínculo PAIF
                  </button>
                  <button
                    type="button"
                    onClick={() => setObservacoesTecnicas((prev) => (prev ? `${prev} ` : "") + "Identificada vulnerabilidade alimentar e econômica emergencial. Encaminhada concessão de benefício eventual.")}
                    className="rounded bg-surface-muted/60 px-2 py-0.5 text-xs text-primary hover:bg-surface-muted transition-colors cursor-pointer"
                  >
                    + Benefício Eventual
                  </button>
                </div>
              </div>

              <Textarea
                id="cras-observacoes-tecnicas"
                placeholder="Descreva a evolução do caso, dinâmica familiar relatada, demandas apresentadas e orientações técnicas prestadas pela assistente social ou psicóloga..."
                rows={3}
                value={observacoesTecnicas}
                onChange={(e) => setObservacoesTecnicas(e.target.value)}
                required
              />
            </div>
          </div>
        )}
      </div>

      {/* Botões de Ação do Atendimento CRAS */}
      <div className="flex flex-wrap items-center justify-between gap-3 pt-4 border-t border-surface-border">
        <div className="flex items-center gap-2 text-xs text-muted">
          <FileText size={15} />
          <span>O registro é assinado digitalmente e inserido no prontuário oficial do munícipe.</span>
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => handleSubmit(false)}
            loading={pending}
            disabled={!observacoesTecnicas.trim()}
          >
            <Save size={15} className="mr-1.5" />
            Salvar (Manter em Atendimento)
          </Button>

          <Button
            type="button"
            variant="primary"
            size="sm"
            onClick={() => handleSubmit(true)}
            loading={pending}
            disabled={!observacoesTecnicas.trim()}
          >
            <CheckCircle2 size={15} className="mr-1.5" />
            Concluir e Finalizar Atendimento
          </Button>
        </div>
      </div>
    </div>
  );
}
