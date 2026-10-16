"use client";

import Link from "next/link";
import {
  AlertCircle,
  CheckCircle2,
  FileText,
  Info,
  ListPlus,
  Printer,
  Search,
  ShieldAlert,
  UserCheck,
  UserPlus,
  X,
} from "lucide-react";
import { useState, type FormEvent } from "react";

import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { apiClient, ApiError } from "@/lib/api/client";
import { useNexus } from "@/lib/nexus/NexusProvider";
import {
  CAMPOS_CIDADAO,
  P,
  PRIORIDADES,
  TIPOS,
  rotuloPrioridade,
  type Cidadao,
  type FilaItem,
  type Resumo,
} from "@/lib/sempras/types";

/** Valida os 11 dígitos e dígitos verificadores de um CPF brasileiro */
function validarCPF(cpf: string): boolean {
  const limpo = cpf.replace(/\D/g, "");
  if (limpo.length !== 11) return false;
  if (/^(\d)\1{10}$/.test(limpo)) return false;

  for (const n of [9, 10]) {
    let soma = 0;
    for (let i = 0; i < n; i++) {
      soma += parseInt(limpo[i]!, 10) * (n + 1 - i);
    }
    const dv = ((soma * 10) % 11) % 10;
    if (dv !== parseInt(limpo[n]!, 10)) {
      return false;
    }
  }
  return true;
}

/**
 * Recepção & Pré-cadastro (ADR 030 §4):
 * - Busca de munícipe na recepção: CPF é obrigatório para conferência nas unidades territoriais (CRAS);
 * - Centro POP: exceção legal (Decreto Federal nº 7.053/2009) onde o CPF é opcional e o acolhimento é incondicional;
 * - Ao localizar, abre modal com informações cadastrais básicas e inclusão na fila;
 * - Se não localizar, abre modal de pré-cadastro simplificado com o CPF digitado pré-preenchido;
 * - Mensagens de erro e validações aparecem com destaque diretamente dentro do modal;
 * - Emissão e impressão de senha térmica para o munícipe.
 */
export function Recepcao({
  unidadeId,
  unidadeNome,
  onIncluido,
  onAbrirFicha,
}: {
  unidadeId: string;
  unidadeNome?: string;
  onIncluido: () => void;
  onAbrirFicha?: (id: string) => void;
}) {
  const { can } = useNexus();
  const { run, pending } = useAction();
  const [q, setQ] = useState("");
  const [achados, setAchados] = useState<Resumo[] | null>(null);
  const [cadastro, setCadastro] = useState(false);
  const [municipeModal, setMunicipeModal] = useState<Resumo | null>(null);
  const [tipo, setTipo] = useState("social");
  const [prioridade, setPrioridade] = useState("");
  const [ticketGerado, setTicketGerado] = useState<FilaItem | null>(null);

  // Estados de erro destacados dentro dos modais para visibilidade do usuário
  const [erroBusca, setErroBusca] = useState<string | null>(null);
  const [erroCadastro, setErroCadastro] = useState<string | null>(null);
  const [errosCampos, setErrosCampos] = useState<Record<string, string>>({});
  const [erroInclusao, setErroInclusao] = useState<string | null>(null);
  const [anonimo, setAnonimo] = useState(false);

  // Identifica se a unidade selecionada é o Centro POP (Proteção Social Especial de Média Complexidade)
  const isCentroPop = Boolean(
    unidadeNome?.toLowerCase().includes("pop") ||
    unidadeNome?.toLowerCase().includes("centro pop")
  );

  async function buscar(e: FormEvent) {
    e.preventDefault();
    setErroBusca(null);
    const query = q.trim();
    if (!query) {
      setErroBusca("Informe o CPF para realizar a consulta.");
      return;
    }

    const out = await run(() =>
      apiClient.get<Resumo[]>(`v1/sempras/cidadaos?q=${encodeURIComponent(query)}`)
    );

    if (out) {
      setAchados(out.data);
      if (out.data.length === 1 && out.data[0]) {
        // Encontrou exatamente um munícipe -> abre modal com dados básicos e opção de fila
        setMunicipeModal(out.data[0]);
        setErroInclusao(null);
      } else if (out.data.length === 0) {
        // Se a busca tiver 11 dígitos (CPF digitado na recepção) e não localizar, abre o pré-cadastro básico
        const digitos = query.replace(/\D/g, "");
        if (digitos.length === 11) {
          setCadastro(true);
          setErroCadastro(null);
          setErrosCampos({});
        }
      }
    }
  }

  async function cadastrar(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setErroCadastro(null);
    setErrosCampos({});

    const fd = new FormData(e.currentTarget);
    const nome = String(fd.get("nome") ?? "").trim();
    const cpf = String(fd.get("cpf") ?? "").trim();
    const isAnonimoChecked = fd.get("anonimo") === "on";

    const novosErros: Record<string, string> = {};

    let bannerMsg: string | null = null;

    // 1. Validação do Nome civil
    if (!nome) {
      novosErros.nome = "Nome civil é obrigatório.";
    } else if (nome.length < 2) {
      novosErros.nome = "Nome deve conter pelo menos 2 caracteres.";
    }

    // 2. Validação do CPF:
    // O CPF é obrigatório para cadastrar em unidades convencionais (CRAS/CREAS).
    // A única exceção legal é o Centro POP (Decreto Federal nº 7.053/2009).
    const digitosCpf = cpf.replace(/\D/g, "");
    if (cpf) {
      if (digitosCpf.length !== 11 || !validarCPF(digitosCpf)) {
        novosErros.cpf = "Confira os 11 dígitos do CPF.";
        bannerMsg = "O CPF informado é inválido. Digite um CPF válido com 11 dígitos.";
      }
    } else if (!isCentroPop && !isAnonimoChecked) {
      novosErros.cpf = "Informe o CPF do munícipe.";
      bannerMsg = "O CPF é obrigatório para cadastrar nesta unidade. Apenas o Centro POP possui exceção legal para acolhimento sem documento.";
    }

    if (Object.keys(novosErros).length > 0) {
      setErrosCampos(novosErros);
      const primeiroErro = bannerMsg || Object.values(novosErros)[0];
      setErroCadastro(primeiroErro || "Corrija os campos destacados no formulário.");
      return;
    }

    const body = {
      ...Object.fromEntries(CAMPOS_CIDADAO.map((c) => [c.name, String(fd.get(c.name) ?? "").trim()])),
      anonimo: isAnonimoChecked,
    };

    try {
      const out = await run(
        () => apiClient.post<Cidadao>("v1/sempras/cidadaos", body),
        "Pré-cadastro do munícipe concluído"
      );

      if (out) {
        const c = out.data;
        const novoResumo: Resumo = { ...c, atendimentos_na_area: 0 };
        setAchados([novoResumo]);
        setCadastro(false);
        setErroCadastro(null);
        setErrosCampos({});
      } else {
        setErroCadastro("Não foi possível salvar o cadastro. Verifique os dados ou se o CPF já está cadastrado.");
      }
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : "Erro ao processar o cadastro do munícipe.";
      setErroCadastro(msg);
      if (msg.toLowerCase().includes("cpf")) {
        setErrosCampos({ cpf: msg });
      }
    }
  }

  async function incluir(cidadao: Resumo) {
    setErroInclusao(null);
    try {
      const out = await run(
        () =>
          apiClient.post<FilaItem>(`v1/sempras/unidades/${unidadeId}/fila`, {
            cidadao_id: cidadao.id,
            tipo,
            prioridade,
          }),
        "Munícipe incluído na fila de atendimento"
      );

      if (out) {
        if (out.data) {
          setTicketGerado(out.data);
        }
        setMunicipeModal(null);
        onIncluido();
      } else {
        setErroInclusao("Não foi possível incluir o munícipe na fila de atendimento.");
      }
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : "Erro ao incluir munícipe na fila.";
      setErroInclusao(msg);
    }
  }

  // Identifica se a busca é um CPF para pré-preencher no pré-cadastro básico
  const digitosBusca = q.replace(/\D/g, "");
  const cpfPrePreenchido = digitosBusca.length === 11 ? q.trim() : "";

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle as="h2">Recepção & Triagem</CardTitle>
              <p className="mt-1 text-xs text-muted">
                {isCentroPop
                  ? "Centro POP — Acolhimento Humanizado: Consulta por CPF, NIS, nome civil, nome social ou apelido."
                  : "Consulta obrigatória por CPF para conferência cadastral e encaminhamento à fila da unidade."}
              </p>
            </div>
            <Badge tone={isCentroPop ? "warning" : "neutral"}>
              {isCentroPop ? "Centro POP (Média Complexidade)" : "Entrada da Unidade (CRAS)"}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-4 pb-5">
          <form onSubmit={buscar} className="mt-3 grid gap-3 sm:grid-cols-[1fr_auto_auto] sm:items-end">
            <Input
              id="sempras-busca"
              label="CPF, NIS ou nome"
              value={q}
              onChange={(e) => {
                setQ(e.target.value);
                setErroBusca(null);
              }}
              placeholder={
                isCentroPop
                  ? "Digite o CPF, nome civil, social ou apelido"
                  : "Digite o CPF do munícipe (obrigatório para consulta)"
              }
              maxLength={200}
              autoComplete="off"
              required
              error={erroBusca || undefined}
            />
            <Button type="submit" loading={pending}>
              <Search size={16} aria-hidden="true" className="mr-1" /> Buscar
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setCadastro(true);
                setErroCadastro(null);
                setErrosCampos({});
              }}
            >
              <UserPlus size={16} aria-hidden="true" className="mr-1" /> Cadastrar
            </Button>
          </form>

          <div className="grid gap-3 sm:grid-cols-2">
            <Select
              id="sempras-tipo"
              label="Tipo de atendimento"
              options={TIPOS}
              value={tipo}
              onChange={(e) => setTipo(e.target.value)}
            />
            <Select
              id="sempras-prioridade"
              label="Prioridade legal"
              options={PRIORIDADES}
              value={prioridade}
              onChange={(e) => setPrioridade(e.target.value)}
            />
          </div>

          {achados !== null &&
            (achados.length === 0 ? (
              <div className="flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-surface-border p-6 text-center">
                <p className="text-sm text-muted">
                  Ninguém encontrado. Confira o documento informado ou realize o pré-cadastro básico.
                </p>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => {
                    setCadastro(true);
                    setErroCadastro(null);
                    setErrosCampos({});
                  }}
                >
                  <UserPlus size={14} className="mr-1.5" /> Abrir Pré-cadastro com este CPF
                </Button>
              </div>
            ) : (
              <ul
                aria-label="Munícipes encontrados"
                className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border"
              >
                {achados.map((c) => (
                  <li key={c.id} className="flex flex-wrap items-center justify-between gap-3 p-3">
                    <div
                      className="cursor-pointer"
                      onClick={() => {
                        setMunicipeModal(c);
                        setErroInclusao(null);
                      }}
                      title="Clique para ver detalhes e incluir na fila"
                    >
                      <p className="font-semibold text-foreground hover:text-primary">
                        {c.nome_social ? `${c.nome_social} (${c.nome})` : c.nome}
                      </p>
                      <p className="text-xs text-muted">
                        {[
                          c.cpf && `CPF ${c.cpf}`,
                          c.nis && `NIS ${c.nis}`,
                          c.data_nascimento && `Nasc. ${c.data_nascimento}`,
                        ]
                          .filter(Boolean)
                          .join(" · ")}
                      </p>
                    </div>
                    <span className="flex items-center gap-2">
                      <Badge tone={c.atendimentos_na_area > 0 ? "info" : "neutral"}>
                        {c.atendimentos_na_area > 0
                          ? `${c.atendimentos_na_area} atendimento(s) na área`
                          : "Sem histórico na área"}
                      </Badge>
                      {(can(P.atendimento) || can(P.psicologia)) && (
                        <Link
                          href={`/sempras/cidadaos/${c.id}`}
                          onClick={(e) => {
                            if (onAbrirFicha) {
                              e.preventDefault();
                              onAbrirFicha(c.id);
                            }
                          }}
                          className="text-sm text-primary underline"
                        >
                          Ficha
                        </Link>
                      )}
                      <Button
                        size="sm"
                        onClick={() => void incluir(c)}
                        disabled={pending}
                        aria-label={`Incluir ${c.nome_social || c.nome} na fila`}
                      >
                        <ListPlus size={14} aria-hidden="true" className="mr-1" /> Incluir na fila
                      </Button>
                    </span>
                  </li>
                ))}
              </ul>
            ))}
        </CardContent>
      </Card>

      {/* Modal 1: Munícipe Localizado (Informações Básicas de Recepção + Inclusão na Fila) */}
      <Dialog
        open={Boolean(municipeModal)}
        onClose={() => {
          setMunicipeModal(null);
          setErroInclusao(null);
        }}
        title="Munícipe Localizado — Recepção"
        size="md"
        description="Confira os dados cadastrais básicos e confirme a inclusão na fila de atendimento."
      >
        {municipeModal && (
          <div className="flex flex-col gap-4">
            {/* Mensagem de Erro de Inclusão destacada no topo do Modal */}
            {erroInclusao && (
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-xl border border-danger/40 bg-danger/10 p-3 text-xs text-danger shadow-xs"
              >
                <AlertCircle size={16} className="mt-0.5 shrink-0 text-danger" />
                <span className="flex-1 font-medium">{erroInclusao}</span>
                <button
                  type="button"
                  onClick={() => setErroInclusao(null)}
                  className="text-danger/70 hover:text-danger"
                  aria-label="Fechar erro"
                >
                  <X size={14} />
                </button>
              </div>
            )}

            <div className="rounded-xl border border-surface-border bg-surface-muted/30 p-4">
              <div className="flex items-start justify-between gap-2">
                <div>
                  <h3 className="text-base font-bold text-foreground">
                    {municipeModal.nome_social ? `${municipeModal.nome_social} (${municipeModal.nome})` : municipeModal.nome}
                  </h3>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted">
                    {municipeModal.cpf && <span>CPF: <strong className="text-foreground">{municipeModal.cpf}</strong></span>}
                    {municipeModal.nis && <span>NIS: <strong className="text-foreground">{municipeModal.nis}</strong></span>}
                    {municipeModal.data_nascimento && <span>Nascimento: <strong className="text-foreground">{municipeModal.data_nascimento}</strong></span>}
                  </div>
                </div>
                <Badge tone={municipeModal.atendimentos_na_area > 0 ? "info" : "neutral"}>
                  {municipeModal.atendimentos_na_area > 0
                    ? `${municipeModal.atendimentos_na_area} registro(s) anterior(es)`
                    : "Sem histórico anterior"}
                </Badge>
              </div>

              <div className="mt-3 flex items-center gap-1.5 rounded-lg border border-info/30 bg-info/5 p-2.5 text-[11px] text-muted">
                <Info size={14} className="shrink-0 text-info" />
                <span>
                  Dados sensíveis, prontuário social e relato sigiloso são acessados apenas pela equipe técnica durante o atendimento.
                </span>
              </div>
            </div>

            {/* Opções de Inclusão na Fila */}
            <div className="rounded-xl border border-surface-border bg-surface p-4">
              <h4 className="text-xs font-semibold uppercase tracking-wider text-muted mb-3">
                Parâmetros do Atendimento
              </h4>
              <div className="grid gap-3 sm:grid-cols-2">
                <Select
                  id="sempras-modal-tipo"
                  label="Modalidade de atendimento"
                  options={TIPOS}
                  value={tipo}
                  onChange={(e) => setTipo(e.target.value)}
                />
                <Select
                  id="sempras-modal-prioridade"
                  label="Prioridade de acolhimento"
                  options={PRIORIDADES}
                  value={prioridade}
                  onChange={(e) => setPrioridade(e.target.value)}
                />
              </div>
            </div>

            {/* Ações do Rodapé */}
            <div className="flex flex-wrap items-center justify-between gap-2 border-t border-surface-border pt-3">
              {(can(P.atendimento) || can(P.psicologia)) ? (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    if (onAbrirFicha && municipeModal) {
                      onAbrirFicha(municipeModal.id);
                      setMunicipeModal(null);
                    }
                  }}
                >
                  <FileText size={14} className="mr-1.5" /> Abrir ficha técnica
                </Button>
              ) : <div />}

              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  onClick={() => void incluir(municipeModal)}
                  loading={pending}
                >
                  <ListPlus size={15} className="mr-1.5" /> Confirmar e incluir na fila
                </Button>
                <Button variant="secondary" size="sm" onClick={() => setMunicipeModal(null)}>
                  Fechar
                </Button>
              </div>
            </div>
          </div>
        )}
      </Dialog>

      {/* Modal 2: Novo Pré-cadastro Básico (Recepção) */}
      <Dialog
        open={cadastro}
        onClose={() => {
          setCadastro(false);
          setErroCadastro(null);
          setErrosCampos({});
        }}
        title="Cadastrar munícipe"
        size="lg"
        description="Pré-cadastro simplificado de recepção. O prontuário completo será aprofundado no atendimento pelas técnicas/psicólogas."
      >
        <form onSubmit={cadastrar} noValidate className="flex flex-col gap-4">
          {/* Mensagem de Erro destacada diretamente no topo do Modal */}
          {erroCadastro && (
            <div
              role="alert"
              className="flex items-start gap-2.5 rounded-xl border border-danger/40 bg-danger/10 p-3.5 text-sm text-danger shadow-xs"
            >
              <AlertCircle size={18} className="mt-0.5 shrink-0 text-danger" />
              <div className="flex-1">
                <p className="font-semibold text-danger">Não foi possível concluir o cadastro</p>
                <p className="mt-0.5 text-xs text-danger/90">{erroCadastro}</p>
              </div>
              <button
                type="button"
                onClick={() => setErroCadastro(null)}
                className="text-danger/70 hover:text-danger"
                aria-label="Fechar mensagem de erro"
              >
                <X size={16} />
              </button>
            </div>
          )}

          {/* Banner de Contexto da Unidade e Regra de Documentação */}
          {isCentroPop ? (
            <div className="rounded-xl border border-warning/40 bg-warning/10 p-3.5 text-xs text-foreground flex items-start gap-2.5">
              <ShieldAlert size={18} className="shrink-0 text-warning mt-0.5" />
              <div>
                <strong className="block text-foreground font-semibold">
                  Centro POP — Acolhimento Humanizado (Decreto Federal nº 7.053/2009)
                </strong>
                <span className="mt-0.5 block text-muted">
                  O acolhimento socioassistencial, alimentação e higiene para a população em situação de rua <strong>não podem ser recusados por ausência de documentação civil</strong>. Nesta unidade o CPF é opcional e é permitido o cadastro por nome civil, nome social ou apelido.
                </span>
              </div>
            </div>
          ) : (
            <div className="rounded-xl border border-primary/30 bg-primary/5 p-3 text-xs text-muted flex items-start gap-2.5">
              <UserCheck size={18} className="shrink-0 text-primary mt-0.5" />
              <div>
                <strong className="block text-foreground font-semibold">
                  Recepção & Triagem da Unidade (CRAS / Proteção Básica)
                </strong>
                <span className="mt-0.5 block text-muted">
                  O <strong>CPF é obrigatório</strong> para localização cadastral, triagem e vinculação no SUAS/CadÚnico. Dados sensíveis e prontuário completo serão aprofundados pela equipe técnica na sala de atendimento.
                </span>
              </div>
            </div>
          )}

          <div className="grid gap-3 sm:grid-cols-2">
            {CAMPOS_CIDADAO.map((c) => {
              const isCpf = c.name === "cpf";
              const isNome = c.name === "nome";
              const isRequired = isNome || (!isCentroPop && isCpf && !anonimo);
              const label = isCpf
                ? isCentroPop
                  ? "CPF (opcional no Centro POP)"
                  : "CPF *"
                : c.label;

              return (
                <Input
                  key={c.name}
                  id={`cad-${c.name}`}
                  name={c.name}
                  label={label}
                  type={c.type}
                  maxLength={c.max}
                  defaultValue={isCpf ? cpfPrePreenchido : ""}
                  required={isRequired}
                  disabled={isCpf && isCentroPop && anonimo}
                  error={errosCampos[c.name]}
                  autoComplete="off"
                  onChange={() => {
                    if (errosCampos[c.name]) {
                      setErrosCampos((prev) => {
                        const copy = { ...prev };
                        delete copy[c.name];
                        return copy;
                      });
                    }
                  }}
                />
              );
            })}

            {/* Checkbox de exceção legal: Visível e ativo exclusivamente no Centro POP */}
            {isCentroPop ? (
              <label className="flex items-start gap-2 text-sm sm:col-span-2 rounded-lg border border-warning/30 bg-warning/5 p-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  name="anonimo"
                  checked={anonimo}
                  onChange={(e) => setAnonimo(e.target.checked)}
                  className="rounded border-surface-border mt-0.5"
                />
                <span className="text-xs text-foreground">
                  <strong>Não possui documento / Acolhimento sem CPF</strong> (População em situação de rua — Decreto Federal nº 7.053/2009)
                </span>
              </label>
            ) : (
              <div className="sm:col-span-2 rounded-lg border border-surface-border bg-surface-muted/30 p-2.5 text-[11px] text-muted flex items-center gap-1.5">
                <Info size={14} className="shrink-0 text-muted" />
                <span>
                  <strong>Exceção de CPF:</strong> Válida exclusivamente no <strong>Centro POP</strong> para acolhimento de pessoas em situação de rua. No CRAS, o documento é indispensável.
                </span>
              </div>
            )}
          </div>

          <div className="flex items-center justify-end gap-2 border-t border-surface-border pt-3">
            <Button type="submit" loading={pending}>
              <CheckCircle2 size={15} className="mr-1.5" />
              Salvar cadastro
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setCadastro(false);
                setErroCadastro(null);
                setErrosCampos({});
              }}
            >
              Cancelar
            </Button>
          </div>
        </form>
      </Dialog>

      {/* Diálogo de Impressão de Senha Térmica */}
      <Dialog
        open={Boolean(ticketGerado)}
        onClose={() => setTicketGerado(null)}
        title="Senha de Atendimento Gerada"
        size="sm"
      >
        {ticketGerado && (
          <div className="flex flex-col items-center gap-4 py-2">
            <div
              id="ticket-termico"
              className="flex w-full max-w-[280px] flex-col items-center rounded-xl border border-dashed border-surface-border bg-surface-muted/20 p-5 text-center font-mono shadow-xs print:m-0 print:w-full print:border-none print:p-0"
            >
              <span className="text-xs uppercase tracking-wider text-muted font-sans font-bold">
                PREFEITURA MUNICIPAL
              </span>
              <span className="text-[11px] text-muted font-sans">
                SEMPRAS — Atendimento Social
              </span>
              <div className="my-3 w-full border-y border-dashed border-surface-border py-3">
                <span className="block text-5xl font-black tracking-tight text-primary print:text-black">
                  {ticketGerado.senha_exibida}
                </span>
                <span className="mt-1 block text-xs font-semibold uppercase text-foreground">
                  {ticketGerado.tipo === "psicologico" ? "Atendimento Psicológico" : "Atendimento Social"}
                </span>
                {ticketGerado.prioridade && ticketGerado.prioridade !== "nenhuma" && (
                  <Badge tone="warning" className="mt-1">
                    {rotuloPrioridade(ticketGerado.prioridade)}
                  </Badge>
                )}
              </div>
              <p className="text-xs font-sans text-foreground">
                <strong>Munícipe:</strong> {ticketGerado.nome}
              </p>
              <p className="mt-1 text-[11px] font-sans text-muted">
                {fmtDateTime(ticketGerado.created_at)}
              </p>
              <p className="mt-3 w-full border-t border-dashed border-surface-border pt-2 text-[10px] italic font-sans text-muted">
                Aguarde ser chamado pelo painel da sala de espera.
              </p>
            </div>

            <div className="flex items-center gap-2">
              <Button size="sm" onClick={() => window.print()}>
                <Printer size={15} className="mr-1.5" /> Imprimir Senha
              </Button>
              <Button size="sm" variant="secondary" onClick={() => setTicketGerado(null)}>
                Fechar
              </Button>
            </div>
          </div>
        )}
      </Dialog>
    </>
  );
}
