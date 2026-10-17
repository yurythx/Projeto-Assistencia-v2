"use client";

import { useState } from "react";
import {
  AlertTriangle,
  Bath,
  Building2,
  Bus,
  CheckCircle2,
  Clock,
  Coffee,
  FileCheck,
  FileText,
  HeartHandshake,
  MapPin,
  Pill,
  Save,
  Send,
  ShieldAlert,
  Sparkles,
  Stethoscope,
  Utensils,
} from "lucide-react";

import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import {
  MOTIVOS_VULNERABILIDADE,
  SITUACOES_RUA,
  type CentroPopPerfil,
  type Cidadao,
  type MotivoVulnerabilidade,
  type SituacaoRua,
} from "@/lib/sempras/types";

export interface CentroPopAtendimentoFormProps {
  cidadao: Cidadao;
  unidadeId: string;
  unidadeNome?: string;
  perfil?: CentroPopPerfil | null;
  itemFilaId?: string;
  isEmAtendimento?: boolean;
  onSalvo?: () => void;
  onConcluido?: () => void;
}

type SubTabPop = "perfil_rua" | "saude_drogas" | "servicos_dia" | "encaminhamentos_passagem" | "evolucao";

/**
 * Formulário Especializado de Atendimento do CENTRO POP:
 * Focado na realidade da População em Situação de Rua e Migrantes (Decreto 7.053/2009 e SUAS):
 * - Trajetória de rua e causas da vulnerabilidade;
 * - Saúde crítica, comorbidades e uso de substâncias (com sigilo técnico);
 * - Rotina de serviços imediatos do dia (Banho, Alimentação, Guarda de Volumes, Lavanderia);
 * - Encaminhamentos de rede (Consultório na Rua, CAPS AD, Albergue, Cartório para 2ª via);
 * - Benefício de Passagem / Recâmbio Rodoviário para retorno familiar;
 * - Parecer técnico da assistência social e conclusão do atendimento.
 */
export function CentroPopAtendimentoForm({
  cidadao,
  unidadeId,
  unidadeNome = "Centro POP",
  perfil,
  itemFilaId,
  isEmAtendimento = false,
  onSalvo,
  onConcluido,
}: CentroPopAtendimentoFormProps) {
  const [subTab, setSubTab] = useState<SubTabPop>("perfil_rua");
  const { run, pending } = useAction();

  // Seção 1: Perfil de Rua e Trajetória
  const [dataAtendimento, setDataAtendimento] = useState(() => new Date().toISOString().split("T")[0]);
  const [situacao, setSituacao] = useState<SituacaoRua>(perfil?.situacao ?? "situacao_rua");
  const [tempoRuaAnos, setTempoRuaAnos] = useState(perfil?.tempo_rua_anos != null ? String(perfil.tempo_rua_anos) : "1");
  const [motivo, setMotivo] = useState<MotivoVulnerabilidade>(perfil?.motivo_vulnerabilidade ?? "conflito_familiar");
  const [cidadeOrigem, setCidadeOrigem] = useState("");
  const [tempoCidade, setTempoCidade] = useState("");

  // Seção 2: Saúde e Uso de Substâncias (Sigilo Técnico)
  const [possuiDoenca, setPossuiDoenca] = useState(perfil?.possui_doenca ?? false);
  const [doencaDescricao, setDoencaDescricao] = useState(perfil?.doenca_descricao ?? "");
  const [usaMedicacao, setUsaMedicacao] = useState(perfil?.usa_medicacao ?? false);
  const [medicacaoDescricao, setMedicacaoDescricao] = useState(perfil?.medicacao_descricao ?? "");
  const [usoDrogas, setUsoDrogas] = useState(perfil?.uso_drogas ?? false);
  const [drogasDescricao, setDrogasDescricao] = useState(perfil?.drogas_descricao ?? "");
  const [tempoDependencia, setTempoDependencia] = useState("");

  // Seção 3: Rotina de Acolhimento & Serviços do Dia (Centro POP)
  const [servicoCafe, setServicoCafe] = useState(true);
  const [servicoAlmoco, setServicoAlmoco] = useState(true);
  const [servicoLanche, setServicoLanche] = useState(false);
  const [servicoBanho, setServicoBanho] = useState(true);
  const [servicoGuardaPertences, setServicoGuardaPertences] = useState(false);
  const [servicoLavanderia, setServicoLavanderia] = useState(false);
  const [servicoSegundaViaDoc, setServicoSegundaViaDoc] = useState(false);

  // Seção 4: Encaminhamentos Específicos & Passagem / Recâmbio
  const [encConsultorioRua, setEncConsultorioRua] = useState(false);
  const [encCapsAd, setEncCapsAd] = useState(false);
  const [encAlbergue, setEncAlbergue] = useState(false);
  const [encHospitalUpa, setEncHospitalUpa] = useState(false);
  const [encCartorio, setEncCartorio] = useState(false);
  const [encSineTrabalho, setEncSineTrabalho] = useState(false);

  // Recâmbio / Passagem
  const [solicitaPassagem, setSolicitaPassagem] = useState(false);
  const [destinoPassagem, setDestinoPassagem] = useState("");
  const [familiarDestino, setFamiliarDestino] = useState("");
  const [telefoneFamiliarDestino, setTelefoneFamiliarDestino] = useState("");
  const [confirmacaoFamiliar, setConfirmacaoFamiliar] = useState(false);

  // Seção 5: Parecer Técnico e Evolução
  const [observacoesTecnicas, setObservacoesTecnicas] = useState("");

  async function handleSubmit(shouldFinalize: boolean) {
    if (!unidadeId) return;

    // Constrói o texto estruturado da evolução do Centro POP
    const linhas: string[] = [
      `🏢 ATENDIMENTO TÉCNICO CENTRO POP — ${dataAtendimento}`,
      `Unidade: ${unidadeNome}`,
      `Munícipe / Acolhido: ${cidadao.nome_social || cidadao.nome} (CPF: ${cidadao.cpf || "Sem CPF - Acolhimento Humanizado"})`,
      `Situação: ${SITUACOES_RUA[situacao] || situacao}`,
      `Tempo de Vivência na Rua: ${tempoRuaAnos ? `${tempoRuaAnos} ano(s)` : "Não informado"}`,
      `Causa Principal da Vulnerabilidade: ${MOTIVOS_VULNERABILIDADE[motivo] || motivo}`,
    ];

    if (cidadeOrigem.trim()) {
      linhas.push(`Cidade/Estado de Origem: ${cidadeOrigem.trim()}${tempoCidade ? ` (Tempo em Rondonópolis: ${tempoCidade})` : ""}`);
    }

    const servicosDia: string[] = [];
    if (servicoCafe) servicosDia.push("Café da Manhã");
    if (servicoAlmoco) servicosDia.push("Almoço");
    if (servicoLanche) servicosDia.push("Lanche da Tarde");
    if (servicoBanho) servicosDia.push("Banho e Higiene Pessoal");
    if (servicoGuardaPertences) servicosDia.push("Guarda de Pertences / Bagagem");
    if (servicoLavanderia) servicosDia.push("Lavanderia / Troca de Roupas");
    if (servicoSegundaViaDoc) servicosDia.push("Busca de 2ª via de Certidão/Documentos Civis");

    if (servicosDia.length > 0) {
      linhas.push(`\n🚿 Serviços de Acolhimento Imediato Concedidos:\n  • ${servicosDia.join("\n  • ")}`);
    }

    const saudeDrogas: string[] = [];
    if (possuiDoenca) saudeDrogas.push(`Comorbidade/Doença: ${doencaDescricao.trim() || "Sim (detalhe não informado)"}`);
    if (usaMedicacao) saudeDrogas.push(`Uso de Medicação Contínua: ${medicacaoDescricao.trim() || "Sim"}`);
    if (usoDrogas) {
      saudeDrogas.push(`Uso de Substâncias Psicoativas: ${drogasDescricao.trim() || "Sim"}${tempoDependencia ? ` (Tempo: ${tempoDependencia})` : ""}`);
    }

    if (saudeDrogas.length > 0) {
      linhas.push(`\n🩺 Saúde e Dependência (Sob Sigilo Técnico):\n  • ${saudeDrogas.join("\n  • ")}`);
    }

    const encaminhamentos: string[] = [];
    if (encConsultorioRua) encaminhamentos.push("Consultório na Rua (Atenção Básica)");
    if (encCapsAd) encaminhamentos.push("CAPS AD (Atenção Psicossocial Álcool e Drogas)");
    if (encAlbergue) encaminhamentos.push("Albergue Noturno / Casa de Passagem");
    if (encHospitalUpa) encaminhamentos.push("UPA / Hospital Regional");
    if (encCartorio) encaminhamentos.push("Cartório de Registro Civil (Gratuidade de Certidão)");
    if (encSineTrabalho) encaminhamentos.push("SINE / Inclusão Produtiva");

    if (encaminhamentos.length > 0) {
      linhas.push(`\n➡️ Encaminhamentos da Rede Socioassistencial e Saúde:\n  • ${encaminhamentos.join("\n  • ")}`);
    }

    if (solicitaPassagem && destinoPassagem.trim()) {
      linhas.push(
        `\n🚌 Recâmbio / Benefício de Passagem Rodoviária:\n` +
        `  • Destino: ${destinoPassagem.trim()}\n` +
        `  • Familiar no Destino: ${familiarDestino.trim() || "Não informado"} (Tel: ${telefoneFamiliarDestino.trim() || "Não informado"})\n` +
        `  • Vínculo e Acolhida Confirmada por Telefone: ${confirmacaoFamiliar ? "SIM (Confirmado)" : "Pendente de confirmação"}`
      );
    }

    if (observacoesTecnicas.trim()) {
      linhas.push(`\n📝 Parecer Técnico e Plano Individual de Acolhimento:\n${observacoesTecnicas.trim()}`);
    }

    const textoFinal = linhas.join("\n");

    const ok = await run(
      async () => {
        // 1. Atualiza o Perfil de Rua 1:1 do Centro POP no banco
        await apiClient.put(`v1/sempras/cidadaos/${cidadao.id}/centro-pop-perfil`, {
          unidade_id: unidadeId,
          situacao,
          tempo_rua_anos: tempoRuaAnos ? Number(tempoRuaAnos) : null,
          motivo_vulnerabilidade: motivo,
          possui_doenca: possuiDoenca,
          doenca_descricao: doencaDescricao.trim(),
          usa_medicacao: usaMedicacao,
          medicacao_descricao: medicacaoDescricao.trim(),
          uso_drogas: usoDrogas,
          drogas_descricao: drogasDescricao.trim(),
        });

        // 2. Grava a evolução técnica no prontuário do munícipe
        await apiClient.post(`v1/sempras/cidadaos/${cidadao.id}/registros`, {
          unidade_id: unidadeId,
          tipo: "social",
          texto: textoFinal,
          atendimento_id: itemFilaId || null,
        });

        // 3. Se solicitado concluir o atendimento na fila
        if (shouldFinalize && itemFilaId) {
          await apiClient.post(`v1/sempras/atendimentos/${itemFilaId}/concluir`);
        }
        // run() devolve o retorno da função: sem ele, o sucesso viraria
        // "undefined" e os callbacks (onSalvo/onConcluido) nunca rodariam.
        return true;
      },
      shouldFinalize
        ? "Atendimento Centro POP concluído e finalizado com sucesso!"
        : "Atendimento Centro POP salvo em andamento com sucesso!"
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
      {/* Banner de Contexto Centro POP — Decreto 7.053/2009 */}
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3.5 shadow-2xs">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-amber-600 text-white font-bold shadow-xs">
            <Building2 size={22} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-bold text-foreground">Centro POP — Atendimento Especializado para População em Situação de Rua</h3>
              <Badge tone="warning">Proteção Especial Média</Badge>
            </div>
            <p className="text-xs text-muted">
              Atendimento humanizado e sem barreiras burocráticas (Decreto Federal nº 7.053/2009 e Tipificação Nacional).
            </p>
          </div>
        </div>

        {itemFilaId && (
          <Badge tone={isEmAtendimento ? "success" : "info"}>
            {isEmAtendimento ? "🟢 Em Atendimento no Centro POP" : "🟡 Chamado na Recepção"}
          </Badge>
        )}
      </div>

      {/* Navegação entre as Sub-Abas do Centro POP */}
      <div className="flex flex-wrap gap-1.5 border-b border-surface-border pb-2">
        <button
          type="button"
          onClick={() => setSubTab("perfil_rua")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "perfil_rua"
              ? "bg-amber-600 text-white shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <MapPin size={14} />
          1. Perfil de Rua & Trajetória
        </button>

        <button
          type="button"
          onClick={() => setSubTab("saude_drogas")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "saude_drogas"
              ? "bg-amber-600 text-white shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <Stethoscope size={14} />
          2. Saúde & Substâncias (Sigiloso)
        </button>

        <button
          type="button"
          onClick={() => setSubTab("servicos_dia")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "servicos_dia"
              ? "bg-amber-600 text-white shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <Bath size={14} />
          3. Rotina de Acolhimento do Dia
        </button>

        <button
          type="button"
          onClick={() => setSubTab("encaminhamentos_passagem")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "encaminhamentos_passagem"
              ? "bg-amber-600 text-white shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <Bus size={14} />
          4. Encaminhamentos & Recâmbio
        </button>

        <button
          type="button"
          onClick={() => setSubTab("evolucao")}
          className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all ${
            subTab === "evolucao"
              ? "bg-amber-600 text-white shadow-xs"
              : "text-muted hover:bg-surface-border/50 hover:text-foreground"
          }`}
        >
          <FileText size={14} />
          5. Parecer & Síntese Social
        </button>
      </div>

      {/* Conteúdo das Sub-Abas */}
      <div className="min-h-[320px]">
        {/* SUB-ABA 1: PERFIL DE RUA E TRAJETÓRIA */}
        {subTab === "perfil_rua" && (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <Input
              id="pop-data-atendimento"
              label="Data do Atendimento / Acolhimento"
              type="date"
              value={dataAtendimento}
              onChange={(e) => setDataAtendimento(e.target.value)}
              required
            />

            <Select
              id="pop-situacao"
              label="Situação Atual do Munícipe"
              value={situacao}
              onChange={(e) => setSituacao(e.target.value as SituacaoRua)}
              options={Object.entries(SITUACOES_RUA).map(([value, label]) => ({ value, label }))}
            />

            <Input
              id="pop-tempo-rua"
              label="Tempo de Vivência na Rua (em anos)"
              type="number"
              min={0}
              step="0.5"
              placeholder="Ex: 0.5 (6 meses), 2, 5..."
              value={tempoRuaAnos}
              onChange={(e) => setTempoRuaAnos(e.target.value)}
            />

            <Select
              id="pop-motivo-vulnerabilidade"
              label="Motivo Principal da Situação de Rua"
              value={motivo}
              onChange={(e) => setMotivo(e.target.value as MotivoVulnerabilidade)}
              options={Object.entries(MOTIVOS_VULNERABILIDADE).map(([value, label]) => ({ value, label }))}
            />

            <Input
              id="pop-cidade-origem"
              label="Cidade e Estado de Origem / Naturalidade"
              placeholder="Ex: Cuiabá - MT, Goiânia - GO..."
              value={cidadeOrigem}
              onChange={(e) => setCidadeOrigem(e.target.value)}
            />

            <Input
              id="pop-tempo-cidade"
              label="Tempo de Permanência em Rondonópolis"
              placeholder="Ex: 3 dias (em trânsito), 4 meses, 2 anos..."
              value={tempoCidade}
              onChange={(e) => setTempoCidade(e.target.value)}
            />
          </div>
        )}

        {/* SUB-ABA 2: SAÚDE, MEDICAÇÕES E DROGAS (SIGILO TÉCNICO) */}
        {subTab === "saude_drogas" && (
          <div className="flex flex-col gap-4">
            <div className="rounded-xl border border-warning/30 bg-warning/10 p-3 text-xs text-foreground">
              <strong>Sigilo Profissional:</strong> As informações de saúde e dependência química ficam sob
              reserva e proteção da equipe técnica (assistentes sociais e psicólogas), não sendo exibidas na recepção.
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {/* Doenças Crônicas */}
              <div className="rounded-xl border border-surface-border bg-surface-muted/30 p-3.5 flex flex-col gap-2.5">
                <label className="flex items-center gap-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={possuiDoenca}
                    onChange={(e) => setPossuiDoenca(e.target.checked)}
                    className="rounded border-surface-border text-primary"
                  />
                  <span className="text-xs font-bold text-foreground">Possui Doença Crônica / Comorbidade</span>
                </label>
                {possuiDoenca && (
                  <Textarea
                    id="pop-doenca-desc"
                    label="Detalhes da Doença / Diagnóstico"
                    placeholder="Ex: Hipertensão, Diabetes, Tuberculose em tratamento, IST..."
                    rows={2}
                    value={doencaDescricao}
                    onChange={(e) => setDoencaDescricao(e.target.value)}
                  />
                )}
              </div>

              {/* Medicação Contínua */}
              <div className="rounded-xl border border-surface-border bg-surface-muted/30 p-3.5 flex flex-col gap-2.5">
                <label className="flex items-center gap-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={usaMedicacao}
                    onChange={(e) => setUsaMedicacao(e.target.checked)}
                    className="rounded border-surface-border text-primary"
                  />
                  <span className="text-xs font-bold text-foreground">Faz Uso de Medicação Contínua</span>
                </label>
                {usaMedicacao && (
                  <Textarea
                    id="pop-medicacao-desc"
                    label="Remédios / Posologia Prescrita"
                    placeholder="Ex: Anti-hipertensivo diário, medicação psiquiátrica..."
                    rows={2}
                    value={medicacaoDescricao}
                    onChange={(e) => setMedicacaoDescricao(e.target.value)}
                  />
                )}
              </div>

              {/* Uso de Substâncias */}
              <div className="sm:col-span-2 rounded-xl border border-surface-border bg-surface-muted/30 p-3.5 flex flex-col gap-2.5">
                <label className="flex items-center gap-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={usoDrogas}
                    onChange={(e) => setUsoDrogas(e.target.checked)}
                    className="rounded border-surface-border text-primary"
                  />
                  <span className="text-xs font-bold text-foreground">Uso Abusivo de Substâncias Psicoativas (Álcool / Drogas)</span>
                </label>
                {usoDrogas && (
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 pt-1">
                    <Textarea
                      id="pop-drogas-desc"
                      label="Substâncias Utilizadas"
                      placeholder="Ex: Álcool frequente, Crack, Tabaco, Cocaína..."
                      rows={2}
                      value={drogasDescricao}
                      onChange={(e) => setDrogasDescricao(e.target.value)}
                    />
                    <Input
                      id="pop-tempo-dependencia"
                      label="Tempo de Uso / Tentativas de Tratamento"
                      placeholder="Ex: Uso há 4 anos, aceita acolhimento em CAPS AD..."
                      value={tempoDependencia}
                      onChange={(e) => setTempoDependencia(e.target.value)}
                    />
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {/* SUB-ABA 3: ROTINA DE ACOLHIMENTO & SERVIÇOS DO DIA */}
        {subTab === "servicos_dia" && (
          <div className="flex flex-col gap-3">
            <span className="text-xs font-bold text-foreground">
              Serviços de Acolhida Imediata Concedidos no Centro POP Hoje:
            </span>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <label className="flex items-start gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-3 cursor-pointer hover:bg-surface-muted/60 transition-colors">
                <input
                  type="checkbox"
                  checked={servicoCafe}
                  onChange={(e) => setServicoCafe(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-surface-border text-amber-600"
                />
                <div className="flex flex-col">
                  <span className="text-xs font-bold text-foreground flex items-center gap-1.5">
                    <Coffee size={14} className="text-amber-700" /> Café da Manhã
                  </span>
                  <span className="text-[11px] text-muted">Alimentação matinal servida na unidade</span>
                </div>
              </label>

              <label className="flex items-start gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-3 cursor-pointer hover:bg-surface-muted/60 transition-colors">
                <input
                  type="checkbox"
                  checked={servicoAlmoco}
                  onChange={(e) => setServicoAlmoco(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-surface-border text-amber-600"
                />
                <div className="flex flex-col">
                  <span className="text-xs font-bold text-foreground flex items-center gap-1.5">
                    <Utensils size={14} className="text-amber-700" /> Almoço
                  </span>
                  <span className="text-[11px] text-muted">Refeição balanceada do meio-dia</span>
                </div>
              </label>

              <label className="flex items-start gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-3 cursor-pointer hover:bg-surface-muted/60 transition-colors">
                <input
                  type="checkbox"
                  checked={servicoLanche}
                  onChange={(e) => setServicoLanche(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-surface-border text-amber-600"
                />
                <div className="flex flex-col">
                  <span className="text-xs font-bold text-foreground flex items-center gap-1.5">
                    <Coffee size={14} className="text-amber-700" /> Lanche da Tarde
                  </span>
                  <span className="text-[11px] text-muted">Lanche e hidratação vespertina</span>
                </div>
              </label>

              <label className="flex items-start gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-3 cursor-pointer hover:bg-surface-muted/60 transition-colors">
                <input
                  type="checkbox"
                  checked={servicoBanho}
                  onChange={(e) => setServicoBanho(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-surface-border text-amber-600"
                />
                <div className="flex flex-col">
                  <span className="text-xs font-bold text-foreground flex items-center gap-1.5">
                    <Bath size={14} className="text-blue-600" /> Banho & Higiene Pessoal
                  </span>
                  <span className="text-[11px] text-muted">Kit de higiene, toalha e sabonete</span>
                </div>
              </label>

              <label className="flex items-start gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-3 cursor-pointer hover:bg-surface-muted/60 transition-colors">
                <input
                  type="checkbox"
                  checked={servicoGuardaPertences}
                  onChange={(e) => setServicoGuardaPertences(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-surface-border text-amber-600"
                />
                <div className="flex flex-col">
                  <span className="text-xs font-bold text-foreground flex items-center gap-1.5">
                    <Building2 size={14} className="text-amber-700" /> Guarda de Pertences
                  </span>
                  <span className="text-[11px] text-muted">Armazenamento seguro de mochilas/volumes</span>
                </div>
              </label>

              <label className="flex items-start gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-3 cursor-pointer hover:bg-surface-muted/60 transition-colors">
                <input
                  type="checkbox"
                  checked={servicoLavanderia}
                  onChange={(e) => setServicoLavanderia(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-surface-border text-amber-600"
                />
                <div className="flex flex-col">
                  <span className="text-xs font-bold text-foreground flex items-center gap-1.5">
                    <Sparkles size={14} className="text-cyan-600" /> Lavanderia & Vestuário
                  </span>
                  <span className="text-[11px] text-muted">Lavagem de roupas e doação de agasalho</span>
                </div>
              </label>

              <label className="sm:col-span-3 flex items-start gap-3 rounded-xl border border-primary/30 bg-primary/5 p-3 cursor-pointer hover:bg-primary/10 transition-colors">
                <input
                  type="checkbox"
                  checked={servicoSegundaViaDoc}
                  onChange={(e) => setServicoSegundaViaDoc(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-surface-border text-primary"
                />
                <div className="flex flex-col">
                  <span className="text-xs font-bold text-primary flex items-center gap-1.5">
                    <FileCheck size={14} /> Resgate de Cidadania — 2ª Via de Documentos Civis
                  </span>
                  <span className="text-[11px] text-muted">
                    Solicitação de certidão de nascimento/casamento em cartório de origem ou emissão de RG e CPF
                  </span>
                </div>
              </label>
            </div>
          </div>
        )}

        {/* SUB-ABA 4: ENCAMINHAMENTOS & RECÂMBIO / PASSAGEM */}
        {subTab === "encaminhamentos_passagem" && (
          <div className="flex flex-col gap-4">
            {/* Encaminhamentos Específicos da Rede de Rua */}
            <div className="flex flex-col gap-2">
              <span className="text-xs font-bold text-foreground">Encaminhamentos Específicos para a População de Rua:</span>
              <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-3">
                <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                  <input
                    type="checkbox"
                    checked={encConsultorioRua}
                    onChange={(e) => setEncConsultorioRua(e.target.checked)}
                    className="rounded border-surface-border text-amber-600"
                  />
                  <span className="font-semibold text-foreground">Consultório na Rua (Saúde)</span>
                </label>

                <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                  <input
                    type="checkbox"
                    checked={encCapsAd}
                    onChange={(e) => setEncCapsAd(e.target.checked)}
                    className="rounded border-surface-border text-amber-600"
                  />
                  <span className="font-semibold text-foreground">CAPS AD (Álcool e Drogas)</span>
                </label>

                <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                  <input
                    type="checkbox"
                    checked={encAlbergue}
                    onChange={(e) => setEncAlbergue(e.target.checked)}
                    className="rounded border-surface-border text-amber-600"
                  />
                  <span className="font-semibold text-foreground">Albergue Noturno / Abrigo</span>
                </label>

                <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                  <input
                    type="checkbox"
                    checked={encHospitalUpa}
                    onChange={(e) => setEncHospitalUpa(e.target.checked)}
                    className="rounded border-surface-border text-amber-600"
                  />
                  <span className="font-semibold text-foreground">UPA / Pronto Atendimento</span>
                </label>

                <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                  <input
                    type="checkbox"
                    checked={encCartorio}
                    onChange={(e) => setEncCartorio(e.target.checked)}
                    className="rounded border-surface-border text-amber-600"
                  />
                  <span className="font-semibold text-foreground">Cartório (Certidão de Nasc.)</span>
                </label>

                <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                  <input
                    type="checkbox"
                    checked={encSineTrabalho}
                    onChange={(e) => setEncSineTrabalho(e.target.checked)}
                    className="rounded border-surface-border text-amber-600"
                  />
                  <span className="font-semibold text-foreground">SINE / Inclusão Produtiva</span>
                </label>
              </div>
            </div>

            {/* Recâmbio / Benefício de Passagem */}
            <div className="rounded-xl border border-amber-500/40 bg-amber-500/5 p-4 flex flex-col gap-3">
              <label className="flex items-center gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  checked={solicitaPassagem}
                  onChange={(e) => setSolicitaPassagem(e.target.checked)}
                  className="h-4 w-4 rounded border-surface-border text-amber-600"
                />
                <div className="flex flex-col">
                  <span className="text-xs font-bold text-amber-900 flex items-center gap-1.5">
                    <Bus size={15} /> Conceder Benefício de Passagem / Recâmbio Rodoviário
                  </span>
                  <span className="text-[11px] text-muted">
                    Transporte rodoviário para retorno à cidade natal ou reinserção familiar/trabalho no destino
                  </span>
                </div>
              </label>

              {solicitaPassagem && (
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 pt-2 border-t border-amber-500/20">
                  <Input
                    id="pop-passagem-destino"
                    label="Município e UF de Destino *"
                    placeholder="Ex: Campo Grande - MS, Goiânia - GO..."
                    value={destinoPassagem}
                    onChange={(e) => setDestinoPassagem(e.target.value)}
                    required
                  />

                  <Input
                    id="pop-familiar-destino"
                    label="Nome do Familiar / Acolhedor no Destino"
                    placeholder="Nome do parente que receberá"
                    value={familiarDestino}
                    onChange={(e) => setFamiliarDestino(e.target.value)}
                  />

                  <Input
                    id="pop-tel-familiar-destino"
                    label="Telefone com DDD do Destino"
                    placeholder="(00) 00000-0000"
                    value={telefoneFamiliarDestino}
                    onChange={(e) => setTelefoneFamiliarDestino(e.target.value)}
                  />

                  <div className="sm:col-span-2 lg:col-span-3 flex items-center gap-2 rounded-lg bg-surface p-2 border border-surface-border">
                    <label className="flex items-center gap-2.5 cursor-pointer text-xs font-semibold text-foreground">
                      <input
                        type="checkbox"
                        checked={confirmacaoFamiliar}
                        onChange={(e) => setConfirmacaoFamiliar(e.target.checked)}
                        className="rounded border-surface-border text-amber-600"
                      />
                      <span>Contato telefônico realizado previamente confirmando acolhida familiar/emprego</span>
                    </label>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* SUB-ABA 5: PARECER TÉCNICO E EVOLUÇÃO SOCIAL */}
        {subTab === "evolucao" && (
          <div className="flex flex-col gap-2.5">
            <div className="flex flex-wrap items-center justify-between gap-1.5">
              <label htmlFor="pop-observacoes-tecnicas" className="text-sm font-medium text-foreground">
                Parecer Técnico Psicossocial / Plano Individual de Acolhimento *
              </label>
              <div className="flex flex-wrap items-center gap-1 text-[11px] text-muted">
                <span className="font-semibold text-muted">Modelos rápidos:</span>
                <button
                  type="button"
                  onClick={() => setObservacoesTecnicas((prev) => (prev ? `${prev} ` : "") + "Acolhida inicial realizada no Centro POP. Munícipe inserido nos serviços imediatos (banho, alimentação e guarda de volumes).")}
                  className="rounded bg-surface-muted/60 px-2 py-0.5 text-xs text-amber-700 hover:bg-surface-muted transition-colors cursor-pointer"
                >
                  + Acolhida Imediata
                </button>
                <button
                  type="button"
                  onClick={() => setObservacoesTecnicas((prev) => (prev ? `${prev} ` : "") + "Acolhido em trânsito. Confirmado vínculo e acolhida familiar no destino por telefone para emissão de recâmbio rodoviário.")}
                  className="rounded bg-surface-muted/60 px-2 py-0.5 text-xs text-amber-700 hover:bg-surface-muted transition-colors cursor-pointer"
                >
                  + Recâmbio Confirmado
                </button>
                <button
                  type="button"
                  onClick={() => setObservacoesTecnicas((prev) => (prev ? `${prev} ` : "") + "Identificadas demandas de saúde e dependência química. Articulado encaminhamento conjunto com Consultório na Rua e CAPS AD.")}
                  className="rounded bg-surface-muted/60 px-2 py-0.5 text-xs text-amber-700 hover:bg-surface-muted transition-colors cursor-pointer"
                >
                  + Saúde & CAPS AD
                </button>
              </div>
            </div>

            <Textarea
              id="pop-observacoes-tecnicas"
              placeholder="Descreva a escuta técnica, narrativa do acolhido sobre a trajetória de rua, estado emocional e físico, metas de superação e encaminhamentos acordados com a assistente social ou psicóloga..."
              rows={3}
              value={observacoesTecnicas}
              onChange={(e) => setObservacoesTecnicas(e.target.value)}
              required
            />
          </div>
        )}
      </div>

      {/* Botões de Ação do Centro POP */}
      <div className="flex flex-wrap items-center justify-between gap-3 pt-4 border-t border-surface-border">
        <div className="flex items-center gap-2 text-xs text-muted">
          <FileText size={15} />
          <span>O registro é assinado e atualiza o Perfil de Rua e o prontuário socioassistencial.</span>
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
            Salvar (Manter em Andamento)
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
            Concluir Atendimento Centro POP
          </Button>
        </div>
      </div>
    </div>
  );
}
