"use client";

import { useState, type FormEvent } from "react";
import {
  AlertTriangle,
  Building2,
  Calendar,
  CheckCircle2,
  HeartHandshake,
  LogOut,
  Plus,
  Save,
  ShieldCheck,
  Trash2,
  UserCheck,
  Users,
  X,
} from "lucide-react";

import { fmtDate, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import {
  STATUS_PAIF,
  TIPOS_ATENDIMENTO_PAIF,
  type Cidadao,
  type PaifVinculo,
  type TipoAtendimentoPaif,
} from "@/lib/sempras/types";

export interface CrasPaifCardProps {
  cidadao: Cidadao;
  unidadeId: string;
  unidadeNome?: string;
  paifVinculos: PaifVinculo[];
  onUpdated: () => void;
}

export const MOTIVOS_DESLIGAMENTO_PAIF = [
  "Superação da situação de vulnerabilidade social e autonomia alcançada",
  "Mudança de território (residência fora da área de abrangência do CRAS)",
  "Transferência para acompanhamento em outro CRAS ou CREAS",
  "Perda de contato após busca ativa / Mudança sem endereço informado",
  "Descumprimento reiterado do Plano de Acompanhamento Familiar",
  "Óbito do responsável familiar / reorganização familiar",
  "Outro motivo fundamentado pela equipe técnica",
];

/**
 * Gestão e Formulário Completo do PAIF (Proteção e Atendimento Integral à Família):
 * Trazido diretamente do protocolo de atendimento socioassistencial e da planilha oficial do MDS.
 * Contempla:
 * - Acompanhamento dos vínculos ativos e histórico de desligamentos da família;
 * - Formulário detalhado de inclusão com tipo de atendimento (particularizado/coletivo/ambos),
 *   público prioritário (extrema pobreza, Bolsa Família, BPC), SCFV e novas inserções;
 * - Processo formal de desligamento com data e motivo justificado.
 */
export function CrasPaifCard({
  cidadao,
  unidadeId,
  unidadeNome = "CRAS",
  paifVinculos,
  onUpdated,
}: CrasPaifCardProps) {
  const [modalNovoAberto, setModalNovoAberto] = useState(false);
  const [desligandoVinculoId, setDesligandoVinculoId] = useState<string | null>(null);
  const [motivoDesligamento, setMotivoDesligamento] = useState(MOTIVOS_DESLIGAMENTO_PAIF[0]);
  const [dataDesligamento, setDataDesligamento] = useState(() => new Date().toISOString().split("T")[0]);

  // Campos do formulário de inclusão
  const [tipoAtendimento, setTipoAtendimento] = useState<TipoAtendimentoPaif>("particularizado");
  const [extremaPobreza, setExtremaPobreza] = useState(false);
  const [bolsaFamilia, setBolsaFamilia] = useState(true);
  const [bpc, setBpc] = useState(false);
  const [scfvParticipantes, setScfvParticipantes] = useState(0);
  const [novosScfvDetalhe, setNovosScfvDetalhe] = useState("");
  const [dataInclusao, setDataInclusao] = useState(() => new Date().toISOString().split("T")[0]);

  const { run: runIncluir, pending: pendingIncluir } = useAction();
  const { run: runDesligar, pending: pendingDesligar } = useAction();

  const vinculoAtivo = paifVinculos.find((v) => v.status === "ativo");

  async function handleIncluir(e: FormEvent) {
    e.preventDefault();
    if (!unidadeId) return;

    const ok = await runIncluir(
      async () => {
        await apiClient.post(`v1/sempras/cidadaos/${cidadao.id}/paif`, {
          unidade_id: unidadeId,
          tipo_atendimento: tipoAtendimento,
          extrema_pobreza: extremaPobreza,
          bolsa_familia: bolsaFamilia,
          bpc: bpc,
          scfv_participantes: Number(scfvParticipantes) || 0,
        });

        // Se houver anotação de novos integrantes SCFV, registra uma evolução
        if (novosScfvDetalhe.trim()) {
          await apiClient.post(`v1/sempras/cidadaos/${cidadao.id}/registros`, {
            unidade_id: unidadeId,
            tipo: "social",
            texto: `[PAIF — INCLUSÃO NO SCFV] Data: ${dataInclusao} | Novos integrantes inseridos no SCFV: ${novosScfvDetalhe.trim()}`,
          });
        }
        // run() devolve o retorno da função: sem ele, o sucesso viraria
        // "undefined" e o modal nunca fecharia.
        return true;
      },
      "Família inserida no acompanhamento do PAIF com sucesso!"
    );

    if (ok) {
      setModalNovoAberto(false);
      onUpdated();
    }
  }

  async function handleDesligar(e: FormEvent) {
    e.preventDefault();
    if (!desligandoVinculoId) return;

    const motivoFinal = `Desligamento em ${dataDesligamento}: ${motivoDesligamento}`;

    const ok = await runDesligar(
      () => apiClient.post(`v1/sempras/paif/${desligandoVinculoId}/excluir`, { motivo: motivoFinal }),
      "Vínculo da família no PAIF desligado com sucesso!"
    );

    if (ok) {
      setDesligandoVinculoId(null);
      onUpdated();
    }
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Cabeçalho do Cartão PAIF */}
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-3.5">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <Users size={22} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-bold text-foreground">Acompanhamento PAIF — {unidadeNome}</h3>
              {vinculoAtivo ? (
                <Badge tone="success">🟢 Família Ativa no PAIF</Badge>
              ) : (
                <Badge tone="neutral">Sem Vínculo Ativo</Badge>
              )}
            </div>
            <p className="text-xs text-muted">
              Serviço de Proteção e Atendimento Integral à Família (Tipificação Nacional / MDS).
            </p>
          </div>
        </div>

        {!vinculoAtivo && (
          <Button
            size="sm"
            variant="primary"
            onClick={() => setModalNovoAberto(true)}
          >
            <Plus size={14} className="mr-1" />
            Nova Família no PAIF
          </Button>
        )}
      </div>

      {/* Detalhe do Vínculo Ativo */}
      {vinculoAtivo ? (
        <div className="flex flex-col gap-3 rounded-xl border border-success/30 bg-success/5 p-4">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-success/20 pb-2">
            <div className="flex items-center gap-2">
              <ShieldCheck size={18} className="text-success" />
              <span className="text-sm font-bold text-foreground">
                Acompanhamento Familiar em Andamento
              </span>
            </div>
            <Button
              size="sm"
              variant="danger"
              onClick={() => setDesligandoVinculoId(vinculoAtivo.id)}
            >
              <LogOut size={13} className="mr-1" />
              Desligar do PAIF
            </Button>
          </div>

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 text-xs">
            <div>
              <span className="text-muted block">Tipo de Atendimento:</span>
              <span className="font-semibold text-foreground">
                {TIPOS_ATENDIMENTO_PAIF[vinculoAtivo.tipo_atendimento] || vinculoAtivo.tipo_atendimento}
              </span>
            </div>
            <div>
              <span className="text-muted block">Data de Inclusão:</span>
              <span className="font-semibold text-foreground">
                {fmtDate(vinculoAtivo.data_inclusao)}
              </span>
            </div>
            <div>
              <span className="text-muted block">Participantes no SCFV:</span>
              <span className="font-semibold text-foreground">
                {vinculoAtivo.scfv_participantes} membro(s)
              </span>
            </div>
            <div>
              <span className="text-muted block">Público Prioritário:</span>
              <div className="flex flex-wrap gap-1 mt-0.5">
                {vinculoAtivo.extrema_pobreza && <Badge tone="danger">Extrema Pobreza</Badge>}
                {vinculoAtivo.bolsa_familia && <Badge tone="info">Bolsa Família</Badge>}
                {vinculoAtivo.bpc && <Badge tone="warning">BPC</Badge>}
                {!vinculoAtivo.extrema_pobreza && !vinculoAtivo.bolsa_familia && !vinculoAtivo.bpc && (
                  <Badge tone="neutral">Demanda Espontânea</Badge>
                )}
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="rounded-xl border border-dashed border-surface-border p-6 text-center">
          <Users size={32} className="mx-auto text-muted mb-2" />
          <p className="text-sm font-medium text-foreground">Nenhum acompanhamento ativo do PAIF nesta unidade</p>
          <p className="text-xs text-muted mt-1 max-w-md mx-auto">
            Famílias em vulnerabilidade social referenciadas pelo CRAS podem ser inseridas no PAIF para acompanhamento
            continuado, fortalecimento de vínculos e acesso a serviços socioassistenciais.
          </p>
          <Button
            size="sm"
            variant="secondary"
            className="mt-3"
            onClick={() => setModalNovoAberto(true)}
          >
            <Plus size={14} className="mr-1" />
            Cadastrar Família no PAIF
          </Button>
        </div>
      )}

      {/* Histórico de Vínculos Desligados / Anteriores */}
      {paifVinculos.some((v) => v.status === "excluido") && (
        <div className="flex flex-col gap-2 pt-2">
          <h4 className="text-xs font-bold uppercase tracking-wider text-muted">Histórico de Acompanhamentos Anteriores</h4>
          <div className="divide-y divide-surface-border rounded-xl border border-surface-border bg-surface-muted/20">
            {paifVinculos
              .filter((v) => v.status === "excluido")
              .map((v) => (
                <div key={v.id} className="p-3 text-xs flex flex-col gap-1.5">
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-semibold text-foreground">
                      Acompanhamento de {fmtDate(v.data_inclusao)} até {v.data_exclusao ? fmtDate(v.data_exclusao) : "Desligado"}
                    </span>
                    <Badge tone="neutral">Desligado</Badge>
                  </div>
                  <p className="text-muted text-[11px]">
                    <strong>Motivo do desligamento:</strong> {v.motivo_exclusao || "Não especificado"}
                  </p>
                </div>
              ))}
          </div>
        </div>
      )}

      {/* MODAL: NOVA FAMÍLIA NO PAIF (Formulário Completo do Protocolo) */}
      <Dialog
        open={modalNovoAberto}
        onClose={() => setModalNovoAberto(false)}
        title="Inserir Família no Acompanhamento do PAIF"
        size="lg"
        description="Formulário oficial de acompanhamento territorial da Proteção e Atendimento Integral à Família (PAIF/CRAS)."
      >
        <form onSubmit={handleIncluir} className="flex flex-col gap-4">
          <div className="rounded-xl border border-primary/20 bg-primary/5 p-3 text-xs text-foreground flex items-center gap-2">
            <Building2 size={16} className="text-primary shrink-0" />
            <span>
              Vinculando ao <strong>{unidadeNome}</strong> — Munícipe: <strong>{cidadao.nome_social || cidadao.nome}</strong> (CPF: {cidadao.cpf || "Sem CPF"})
            </span>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Input
              id="paif-data-inclusao"
              label="Data de Inclusão no PAIF"
              type="date"
              value={dataInclusao}
              onChange={(e) => setDataInclusao(e.target.value)}
              required
            />

            <Select
              id="paif-tipo-atendimento"
              label="Modalidade de Atendimento"
              value={tipoAtendimento}
              onChange={(e) => setTipoAtendimento(e.target.value as TipoAtendimentoPaif)}
              options={Object.entries(TIPOS_ATENDIMENTO_PAIF).map(([value, label]) => ({ value, label }))}
            />
          </div>

          <div className="flex flex-col gap-2 pt-2 border-t border-surface-border">
            <span className="text-xs font-bold text-foreground">Público Prioritário (Indicadores RMA/MDS)</span>
            <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-3">
              <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                <input
                  type="checkbox"
                  checked={extremaPobreza}
                  onChange={(e) => setExtremaPobreza(e.target.checked)}
                  className="rounded border-surface-border text-primary"
                />
                <span className="font-semibold text-foreground">Extrema Pobreza</span>
              </label>

              <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                <input
                  type="checkbox"
                  checked={bolsaFamilia}
                  onChange={(e) => setBolsaFamilia(e.target.checked)}
                  className="rounded border-surface-border text-primary"
                />
                <span className="font-semibold text-foreground">Bolsa Família</span>
              </label>

              <label className="flex items-center gap-2.5 rounded-lg border border-surface-border p-2.5 bg-surface-muted/30 cursor-pointer text-xs">
                <input
                  type="checkbox"
                  checked={bpc}
                  onChange={(e) => setBpc(e.target.checked)}
                  className="rounded border-surface-border text-primary"
                />
                <span className="font-semibold text-foreground">Membro com BPC</span>
              </label>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 pt-2 border-t border-surface-border">
            <Input
              id="paif-scfv"
              label="Membros Participantes no SCFV"
              type="number"
              min={0}
              max={50}
              value={scfvParticipantes}
              onChange={(e) => setScfvParticipantes(Number(e.target.value))}
            />

            <Input
              id="paif-novos-membros-scfv"
              label="Novos Inseridos no SCFV (por faixa/nome)"
              placeholder="Ex: 2 crianças 0-6 anos, 1 idoso..."
              value={novosScfvDetalhe}
              onChange={(e) => setNovosScfvDetalhe(e.target.value)}
            />
          </div>

          <div className="flex justify-end gap-2 pt-3 border-t border-surface-border">
            <Button type="button" variant="secondary" onClick={() => setModalNovoAberto(false)}>
              Cancelar
            </Button>
            <Button type="submit" variant="primary" loading={pendingIncluir}>
              <Save size={15} className="mr-1.5" />
              Salvar Vínculo no PAIF
            </Button>
          </div>
        </form>
      </Dialog>

      {/* DIÁLOGO: DESLIGAMENTO DO PAIF */}
      <Dialog
        open={Boolean(desligandoVinculoId)}
        onClose={() => setDesligandoVinculoId(null)}
        title="Desligamento de Família do PAIF"
        size="md"
        description="O desligamento encerra o acompanhamento ativo, congelando o histórico para fins de estatística e RMA."
      >
        <form onSubmit={handleDesligar} className="flex flex-col gap-4">
          <Input
            id="paif-data-desligamento"
            label="Data do Desligamento"
            type="date"
            value={dataDesligamento}
            onChange={(e) => setDataDesligamento(e.target.value)}
            required
          />

          <Select
            id="paif-motivo-desligamento-padrao"
            label="Motivo do Desligamento"
            value={motivoDesligamento}
            onChange={(e) => setMotivoDesligamento(e.target.value)}
            options={MOTIVOS_DESLIGAMENTO_PAIF.map((m) => ({ value: m, label: m }))}
          />

          <div className="flex justify-end gap-2 pt-3 border-t border-surface-border">
            <Button type="button" variant="secondary" onClick={() => setDesligandoVinculoId(null)}>
              Cancelar
            </Button>
            <Button type="submit" variant="danger" loading={pendingDesligar}>
              <LogOut size={15} className="mr-1.5" />
              Confirmar Desligamento
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
