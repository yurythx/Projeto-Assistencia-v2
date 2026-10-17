"use client";

import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { ArrowLeft, ClipboardCheck, Download, Lock, NotebookPen, Pencil, ShieldAlert, Sparkles, Trash2, UserPlus } from "lucide-react";
import { useRef, useState, type FormEvent } from "react";

import { CentroPopAtendimentoForm } from "@/components/sempras/CentroPopAtendimentoForm";
import { CrasAtendimentoForm } from "@/components/sempras/CrasAtendimentoForm";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtDate, fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClass } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { mimeOf, putToTicket } from "@/lib/nexus/upload";
import type { UploadTicket } from "@/lib/nexus/types";
import { AcolhimentoItem } from "@/components/sempras/AcolhimentoItem";
import { AbordagemSocial } from "@/components/sempras/AbordagemSocial";
import { CasaMulherCard } from "@/components/sempras/CasaMulherCard";
import { CentroPopPerfilCard } from "@/components/sempras/CentroPopPerfilCard";
import {
  CAMPOS_CIDADAO,
  DESTINOS_EXTERNOS,
  FORMAS_ACESSO_BENEFICIO,
  MEDIDAS_PROTETIVAS,
  MOTIVOS_ENCERRAMENTO_PAEFI,
  MOTIVOS_VIOLACAO,
  ORIGENS_DEMANDA,
  ORIGENS_NOTIFICACAO_CREAS,
  P,
  PRIORIDADES_ENCAMINHAMENTO_EXTERNO,
  STATUS,
  STATUS_BENEFICIO,
  STATUS_CONSELHO,
  STATUS_ENCAMINHAMENTO,
  STATUS_ENCAMINHAMENTO_EXTERNO,
  STATUS_MSE,
  STATUS_NOTIFICACAO_CREAS,
  STATUS_OFICIO_CRAS,
  STATUS_PAEFI,
  STATUS_PAIF,
  TIPOS,
  TIPOS_ATENDIMENTO_PAIF,
  TIPOS_BENEFICIO,
  TIPOS_DOCUMENTO_CENTRO_POP,
  TIPOS_MSE,
  VIOLACOES_PAEFI,
  rotuloPrioridade,
  type Cidadao,
  type DestinoExterno,
  type DocumentoCentroPop,
  type FormaAcessoBeneficio,
  type MedidaProtetiva,
  type MotivoEncerramentoPaefi,
  type MotivoViolacaoConselho,
  type OrigemDemandaConselho,
  type OrigemNotificacaoCreas,
  type PrioridadeEncaminhamentoExterno,
  type Prontuario,
  type TipoAtendimentoPaif,
  type TipoBeneficio,
  type TipoDocumentoCentroPop,
  type TipoMse,
  type Unidade,
  type ViolacaoPaefi,
  ACOMPANHAMENTOS,
  RESULTADOS_ABORDAGEM,
  SEXOS,
  TIPOS_UNIDADE,
  oferta,
} from "@/lib/sempras/types";

/**
 * Ficha do munícipe (prontuário): cadastro, atendimentos e evoluções da
 * área de quem consulta. Toda abertura é registrada na trilha de
 * auditoria; registros psicológicos aparecem sem o texto para quem não
 * tem o sigilo da psicologia (ADR 030 §5–6).
 */
export default function FichaPage() {
  const { id } = useParams<{ id: string }>();
  const params = useSearchParams();
  const { can } = useNexus();
  const { run, pending } = useAction();
  const ficha = useApiQuery<Prontuario>(`v1/sempras/cidadaos/${id}/prontuario`, { revalidateOnFocus: false });
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [unidade, setUnidade] = useState(params.get("unidade") ?? "");
  const [tipo, setTipo] = useState("social");
  const [texto, setTexto] = useState("");
  const [mostrarFormAtendimentoCras, setMostrarFormAtendimentoCras] = useState(false);
  const [mostrarFormAtendimentoPop, setMostrarFormAtendimentoPop] = useState(false);
  const nomes = new Map((unidades.data ?? []).map((u) => [u.id, u.nome]));
  // A unidade em uso decide as seções da ficha (ADR 031 §4) — vem do
  // espaço de atendimento (?unidade=) e pode ser trocada por quem atua em mais de uma.
  const unidadeAtual = unidades.data?.find((u) => u.id === unidade) ?? unidades.data?.[0];
  const unidadeId = unidadeAtual?.id ?? "";
  const tipos = can(P.psicologia) ? TIPOS : TIPOS.filter((t) => t.value === "social");
  const isCentroPopUnidade = oferta(unidadeAtual, "centro_pop");
  const isCrasUnidade = unidadeAtual?.tipo === "cras";

  async function registrar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/registros`, { unidade_id: unidadeId, tipo, texto: texto.trim() }),
      "Evolução registrada",
    );
    if (ok) {
      setTexto("");
      void ficha.mutate();
    }
  }

  // Direitos do titular (ADR 030 §8): acesso e correção se resolvem na
  // hora, com o titular presente; anonimização fica pendente até a gestão decidir.
  const [editando, setEditando] = useState(false);
  const [pedidoCorrecao, setPedidoCorrecao] = useState(false);
  const [justificativaCorrecao, setJustificativaCorrecao] = useState("");
  const { run: runEditar, pending: pendingEditar } = useAction();

  const [exportando, setExportando] = useState(false);
  const [justificativaAcesso, setJustificativaAcesso] = useState("");

  const [anonimizando, setAnonimizando] = useState(false);
  const [justificativaAnonimizacao, setJustificativaAnonimizacao] = useState("");
  const { run: runAnonimizar, pending: pendingAnonimizar } = useAction();

  async function salvarEdicao(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const body = {
      ...Object.fromEntries(CAMPOS_CIDADAO.map((campo) => [campo.name, String(fd.get(campo.name) ?? "").trim()])),
      sexo: String(fd.get("sexo") ?? ""),
      anonimo: fd.get("anonimo") === "on",
    };
    const ok = await runEditar(() => apiClient.put<Cidadao>(`v1/sempras/cidadaos/${id}`, body), "Cadastro atualizado");
    if (!ok) return;
    if (pedidoCorrecao) {
      await runEditar(
        () => apiClient.post(`v1/sempras/cidadaos/${id}/pedidos`, { tipo: "correcao", justificativa: justificativaCorrecao.trim() }),
      );
    }
    setEditando(false);
    setPedidoCorrecao(false);
    setJustificativaCorrecao("");
    void ficha.mutate();
  }

  async function solicitarAnonimizacao(e: FormEvent) {
    e.preventDefault();
    const ok = await runAnonimizar(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/pedidos`, { tipo: "anonimizacao", justificativa: justificativaAnonimizacao.trim() }),
      "Pedido registrado — aguardando decisão da gestão",
    );
    if (ok) {
      setAnonimizando(false);
      setJustificativaAnonimizacao("");
    }
  }

  // Rede de apoio: família, vizinhos ou referências do munícipe.
  const { run: runRedeApoio, pending: pendingRedeApoio } = useAction();
  const { run: runRemoverContato } = useAction();

  async function adicionarContato(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget; // capturado antes do await — o SyntheticEvent é reciclado depois
    const fd = new FormData(form);
    const body = {
      nome: String(fd.get("nome") ?? "").trim(),
      parentesco: String(fd.get("parentesco") ?? "").trim(),
      telefone: String(fd.get("telefone") ?? "").trim(),
    };
    const ok = await runRedeApoio(() => apiClient.post(`v1/sempras/cidadaos/${id}/rede-apoio`, body), "Contato adicionado");
    if (ok) {
      form.reset();
      void ficha.mutate();
    }
  }

  async function removerContato(contatoId: string) {
    const ok = await runRemoverContato(() => apiClient.delete(`v1/sempras/rede-apoio/${contatoId}`), "Contato removido");
    if (ok) void ficha.mutate();
  }

  // Benefícios eventuais (LOAS art. 22): solicitado → concedido/negado;
  // concedido → suspenso. Negar e suspender exigem motivo; conceder não.
  const [tipoBeneficio, setTipoBeneficio] = useState<TipoBeneficio>("cesta_basica");
  const [formaAcessoBeneficio, setFormaAcessoBeneficio] = useState<FormaAcessoBeneficio>("demanda_espontanea");
  const [quantidadeBeneficio, setQuantidadeBeneficio] = useState("1");
  const [observacaoBeneficio, setObservacaoBeneficio] = useState("");
  const { run: runBeneficio, pending: pendingBeneficio } = useAction();
  const { run: runDecidirBeneficio, pending: pendingDecisaoBeneficio } = useAction();
  const [decisaoBeneficio, setDecisaoBeneficio] = useState<{ id: string; acao: "negar" | "suspender" } | null>(null);
  const [motivoBeneficio, setMotivoBeneficio] = useState("");

  async function solicitarBeneficio(e: FormEvent) {
    e.preventDefault();
    const ok = await runBeneficio(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/beneficios`, {
        unidade_id: unidadeId, tipo: tipoBeneficio, forma_acesso: formaAcessoBeneficio,
        quantidade: Number(quantidadeBeneficio) || 1, observacao: observacaoBeneficio.trim(),
      }),
      "Benefício solicitado",
    );
    if (ok) {
      setObservacaoBeneficio("");
      setQuantidadeBeneficio("1");
      void ficha.mutate();
    }
  }

  async function conceder(beneficioId: string) {
    const ok = await runDecidirBeneficio(
      () => apiClient.post(`v1/sempras/beneficios/${beneficioId}/decidir`, { acao: "conceder", motivo: "" }),
      "Benefício concedido",
    );
    if (ok) void ficha.mutate();
  }

  async function confirmarDecisaoBeneficio(e: FormEvent) {
    e.preventDefault();
    if (!decisaoBeneficio) return;
    const ok = await runDecidirBeneficio(
      () => apiClient.post(`v1/sempras/beneficios/${decisaoBeneficio.id}/decidir`, { acao: decisaoBeneficio.acao, motivo: motivoBeneficio.trim() }),
      decisaoBeneficio.acao === "negar" ? "Benefício negado" : "Benefício suspenso",
    );
    if (ok) {
      setDecisaoBeneficio(null);
      setMotivoBeneficio("");
      void ficha.mutate();
    }
  }

  // Encaminhamentos entre unidades (ex.: CRAS → CREAS): quem recebe decide.
  const [unidadeDestino, setUnidadeDestino] = useState("");
  const [motivoEncaminhamento, setMotivoEncaminhamento] = useState("");
  const { run: runEncaminhar, pending: pendingEncaminhar } = useAction();
  const outrasUnidades = (unidades.data ?? []).filter((u) => u.id !== unidadeId);

  async function encaminhar(e: FormEvent) {
    e.preventDefault();
    const ok = await runEncaminhar(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/encaminhamentos`, {
        unidade_origem_id: unidadeId, unidade_destino_id: unidadeDestino, motivo: motivoEncaminhamento.trim(),
      }),
      "Encaminhamento registrado",
    );
    if (ok) {
      setMotivoEncaminhamento("");
      void ficha.mutate();
    }
  }

  // PAIF (CRAS): vínculo da família ao serviço, com status real (não
  // inferido pela ausência de data de exclusão).
  const [tipoAtendimentoPaif, setTipoAtendimentoPaif] = useState<TipoAtendimentoPaif>("particularizado");
  const [extremaPobreza, setExtremaPobreza] = useState(false);
  const [bolsaFamilia, setBolsaFamilia] = useState(false);
  const [bpc, setBpc] = useState(false);
  const [scfv, setScfv] = useState("0");
  const { run: runPaif, pending: pendingPaif } = useAction();
  const { run: runExcluirPaif, pending: pendingExcluirPaif } = useAction();
  const [excluindoPaif, setExcluindoPaif] = useState<string | null>(null);
  const [motivoExclusaoPaif, setMotivoExclusaoPaif] = useState("");

  async function incluirPaif(e: FormEvent) {
    e.preventDefault();
    const ok = await runPaif(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/paif`, {
        unidade_id: unidadeId, tipo_atendimento: tipoAtendimentoPaif, extrema_pobreza: extremaPobreza,
        bolsa_familia: bolsaFamilia, bpc, scfv_participantes: Number(scfv) || 0,
      }),
      "Família incluída no PAIF",
    );
    if (ok) {
      setTipoAtendimentoPaif("particularizado");
      setExtremaPobreza(false);
      setBolsaFamilia(false);
      setBpc(false);
      setScfv("0");
      void ficha.mutate();
    }
  }

  async function confirmarExclusaoPaif(e: FormEvent) {
    e.preventDefault();
    if (!excluindoPaif) return;
    const ok = await runExcluirPaif(
      () => apiClient.post(`v1/sempras/paif/${excluindoPaif}/excluir`, { motivo: motivoExclusaoPaif.trim() }),
      "Vínculo PAIF excluído",
    );
    if (ok) {
      setExcluindoPaif(null);
      setMotivoExclusaoPaif("");
      void ficha.mutate();
    }
  }

  // Conselho Tutelar (Art. 101 do ECA): medida/encaminhamento ao MP/arquivamento.
  const [respNome, setRespNome] = useState("");
  const [respCPF, setRespCPF] = useState("");
  const [respTelefone, setRespTelefone] = useState("");
  const [origemDemanda, setOrigemDemanda] = useState<OrigemDemandaConselho>("escola");
  const [motivoViolacao, setMotivoViolacao] = useState<MotivoViolacaoConselho>("negligencia_abandono");
  const [observacoesConselho, setObservacoesConselho] = useState("");
  const { run: runConselho, pending: pendingConselho } = useAction();
  const { run: runTransitarConselho, pending: pendingTransitarConselho } = useAction();
  const [aplicandoMedida, setAplicandoMedida] = useState<string | null>(null);
  const [medidaEscolhida, setMedidaEscolhida] = useState<Exclude<MedidaProtetiva, "">>("termo_responsabilidade");

  async function registrarConselho(e: FormEvent) {
    e.preventDefault();
    const ok = await runConselho(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/conselho-tutelar`, {
        unidade_id: unidadeId, responsavel_nome: respNome.trim(), responsavel_cpf: respCPF.trim(),
        responsavel_telefone: respTelefone.trim(), origem_demanda: origemDemanda, motivo_violacao: motivoViolacao,
        observacoes: observacoesConselho.trim(),
      }),
      "Atendimento do Conselho Tutelar registrado",
    );
    if (ok) {
      setRespNome("");
      setRespCPF("");
      setRespTelefone("");
      setObservacoesConselho("");
      void ficha.mutate();
    }
  }

  async function transitarConselho(atendimentoId: string, acao: "encaminhar_mp" | "arquivar") {
    const ok = await runTransitarConselho(
      () => apiClient.post(`v1/sempras/conselho-tutelar/${atendimentoId}/transitar`, { acao }),
      acao === "encaminhar_mp" ? "Encaminhado ao Ministério Público" : "Atendimento arquivado",
    );
    if (ok) void ficha.mutate();
  }

  async function confirmarMedida(e: FormEvent) {
    e.preventDefault();
    if (!aplicandoMedida) return;
    const ok = await runTransitarConselho(
      () => apiClient.post(`v1/sempras/conselho-tutelar/${aplicandoMedida}/transitar`, { acao: "aplicar_medida", medida: medidaEscolhida }),
      "Medida protetiva aplicada",
    );
    if (ok) {
      setAplicandoMedida(null);
      void ficha.mutate();
    }
  }

  // PAEFI (CREAS): acompanhamento de violação de direitos (Resolução CNAS
  // nº 109/2009).
  const [violacoesPaefi, setViolacoesPaefi] = useState<ViolacaoPaefi[]>([]);
  const [membrosVulneraveis, setMembrosVulneraveis] = useState("");
  const [observacoesPaefi, setObservacoesPaefi] = useState("");
  const { run: runPaefi, pending: pendingPaefi } = useAction();
  const { run: runPiaElaborado, pending: pendingPiaElaborado } = useAction();
  const { run: runEncerrarPaefi, pending: pendingEncerrarPaefi } = useAction();
  const [encerrandoPaefi, setEncerrandoPaefi] = useState<string | null>(null);
  const [motivoEncerramentoPaefi, setMotivoEncerramentoPaefi] = useState<Exclude<MotivoEncerramentoPaefi, "">>("objetivos_alcancados");

  function toggleViolacaoPaefi(v: ViolacaoPaefi) {
    setViolacoesPaefi((atual) => (atual.includes(v) ? atual.filter((x) => x !== v) : [...atual, v]));
  }

  async function registrarPaefi(e: FormEvent) {
    e.preventDefault();
    const ok = await runPaefi(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/paefi`, {
        unidade_id: unidadeId, violacoes: violacoesPaefi, membros_vulneraveis: membrosVulneraveis.trim(), observacoes: observacoesPaefi.trim(),
      }),
      "Caso do PAEFI aberto",
    );
    if (ok) {
      setViolacoesPaefi([]);
      setMembrosVulneraveis("");
      setObservacoesPaefi("");
      void ficha.mutate();
    }
  }

  async function marcarPiaElaborado(casoId: string) {
    const ok = await runPiaElaborado(() => apiClient.post(`v1/sempras/paefi/${casoId}/pia`, {}), "PIA elaborado registrado");
    if (ok) void ficha.mutate();
  }

  async function confirmarEncerramentoPaefi(e: FormEvent) {
    e.preventDefault();
    if (!encerrandoPaefi) return;
    const ok = await runEncerrarPaefi(
      () => apiClient.post(`v1/sempras/paefi/${encerrandoPaefi}/encerrar`, { motivo: motivoEncerramentoPaefi }),
      "Caso do PAEFI encerrado",
    );
    if (ok) {
      setEncerrandoPaefi(null);
      void ficha.mutate();
    }
  }

  // MSE: medida socioeducativa em meio aberto (ECA art. 112, III e VI).
  const [numeroProcesso, setNumeroProcesso] = useState("");
  const [varaOrigem, setVaraOrigem] = useState("");
  const [atoInfracional, setAtoInfracional] = useState("");
  const [tipoMedida, setTipoMedida] = useState<TipoMse>("liberdade_assistida");
  const [instituicaoParceira, setInstituicaoParceira] = useState("");
  const { run: runMse, pending: pendingMse } = useAction();
  const { run: runConcluirMse, pending: pendingConcluirMse } = useAction();
  const [concluindoMse, setConcluindoMse] = useState<string | null>(null);
  const [resultadoMse, setResultadoMse] = useState<"cumprida" | "descumprida" | "extinta">("cumprida");

  async function registrarMse(e: FormEvent) {
    e.preventDefault();
    const ok = await runMse(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/mse`, {
        unidade_id: unidadeId, numero_processo: numeroProcesso.trim(), vara_origem: varaOrigem.trim(),
        ato_infracional: atoInfracional.trim(), tipo_medida: tipoMedida, instituicao_parceira: instituicaoParceira.trim(),
      }),
      "Medida socioeducativa registrada",
    );
    if (ok) {
      setNumeroProcesso("");
      setVaraOrigem("");
      setAtoInfracional("");
      setInstituicaoParceira("");
      void ficha.mutate();
    }
  }

  async function confirmarConclusaoMse(e: FormEvent) {
    e.preventDefault();
    if (!concluindoMse) return;
    const ok = await runConcluirMse(
      () => apiClient.post(`v1/sempras/mse/${concluindoMse}/concluir`, { resultado: resultadoMse }),
      "Medida socioeducativa concluída",
    );
    if (ok) {
      setConcluindoMse(null);
      void ficha.mutate();
    }
  }

  // Notificações do CREAS: ofícios/requisições de um ente externo (MP,
  // Judiciário, Conselho Tutelar, Disque 100/180, Delegacia).
  const [origemNotificacao, setOrigemNotificacao] = useState<OrigemNotificacaoCreas>("ministerio_publico");
  const [assuntoNotificacao, setAssuntoNotificacao] = useState("");
  const [prazoNotificacao, setPrazoNotificacao] = useState("");
  const { run: runNotificacao, pending: pendingNotificacao } = useAction();
  const { run: runTransitarNotificacao, pending: pendingTransitarNotificacao } = useAction();
  const [respondendoNotificacao, setRespondendoNotificacao] = useState<string | null>(null);
  const [respostaNotificacao, setRespostaNotificacao] = useState("");

  async function registrarNotificacao(e: FormEvent) {
    e.preventDefault();
    const ok = await runNotificacao(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/creas-notificacoes`, {
        unidade_id: unidadeId, origem_entidade: origemNotificacao, assunto: assuntoNotificacao.trim(),
        prazo_resposta: prazoNotificacao ? new Date(prazoNotificacao).toISOString() : null,
      }),
      "Notificação do CREAS registrada",
    );
    if (ok) {
      setAssuntoNotificacao("");
      setPrazoNotificacao("");
      void ficha.mutate();
    }
  }

  async function iniciarDiligencia(notifId: string) {
    const ok = await runTransitarNotificacao(
      () => apiClient.post(`v1/sempras/creas-notificacoes/${notifId}/transitar`, { acao: "iniciar_diligencia" }),
      "Diligência iniciada",
    );
    if (ok) void ficha.mutate();
  }

  async function arquivarNotificacao(notifId: string) {
    const ok = await runTransitarNotificacao(
      () => apiClient.post(`v1/sempras/creas-notificacoes/${notifId}/transitar`, { acao: "arquivar" }),
      "Notificação arquivada",
    );
    if (ok) void ficha.mutate();
  }

  async function confirmarRespostaNotificacao(e: FormEvent) {
    e.preventDefault();
    if (!respondendoNotificacao) return;
    const ok = await runTransitarNotificacao(
      () => apiClient.post(`v1/sempras/creas-notificacoes/${respondendoNotificacao}/transitar`, { acao: "responder", resposta: respostaNotificacao.trim() }),
      "Notificação respondida",
    );
    if (ok) {
      setRespondendoNotificacao(null);
      setRespostaNotificacao("");
      void ficha.mutate();
    }
  }

  // Ofícios do CRAS: ofício recebido de um ente externo (Conselho
  // Tutelar, MP etc.) sobre a família acompanhada.
  const [orgaoOficio, setOrgaoOficio] = useState("");
  const [numeroOficio, setNumeroOficio] = useState("");
  const [assuntoOficio, setAssuntoOficio] = useState("");
  const { run: runOficio, pending: pendingOficio } = useAction();
  const { run: runResponderOficio, pending: pendingResponderOficio } = useAction();
  const [respondendoOficio, setRespondendoOficio] = useState<string | null>(null);
  const [numeroRespostaOficio, setNumeroRespostaOficio] = useState("");

  async function registrarOficio(e: FormEvent) {
    e.preventDefault();
    const ok = await runOficio(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/oficios-cras`, {
        unidade_id: unidadeId, orgao_emissor: orgaoOficio.trim(), numero_oficio: numeroOficio.trim(), assunto: assuntoOficio.trim(),
      }),
      "Ofício registrado",
    );
    if (ok) {
      setOrgaoOficio("");
      setNumeroOficio("");
      setAssuntoOficio("");
      void ficha.mutate();
    }
  }

  async function confirmarRespostaOficio(e: FormEvent) {
    e.preventDefault();
    if (!respondendoOficio) return;
    const ok = await runResponderOficio(
      () => apiClient.post(`v1/sempras/oficios-cras/${respondendoOficio}/responder`, { numero_resposta: numeroRespostaOficio.trim() }),
      "Ofício respondido",
    );
    if (ok) {
      setRespondendoOficio(null);
      setNumeroRespostaOficio("");
      void ficha.mutate();
    }
  }

  // Centro POP: encaminhamento pra fora do sistema (CAPS AD, albergue,
  // saúde, polícia) — log gerenciado por quem atende, sem fluxo de
  // aceite de 2 pontas (diferente de Encaminhamento entre unidades).
  const [destinoExterno, setDestinoExterno] = useState<DestinoExterno>("caps_ad");
  const [prioridadeExterno, setPrioridadeExterno] = useState<PrioridadeEncaminhamentoExterno>("media");
  const [motivoExterno, setMotivoExterno] = useState("");
  const { run: runEncaminhamentoExterno, pending: pendingEncaminhamentoExterno } = useAction();
  const { run: runTransitarExterno, pending: pendingTransitarExterno } = useAction();

  async function registrarEncaminhamentoExterno(e: FormEvent) {
    e.preventDefault();
    const ok = await runEncaminhamentoExterno(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/encaminhamentos-externos`, {
        unidade_id: unidadeId, destino: destinoExterno, prioridade: prioridadeExterno, motivo: motivoExterno.trim(),
      }),
      "Encaminhamento externo registrado",
    );
    if (ok) {
      setMotivoExterno("");
      void ficha.mutate();
    }
  }

  async function transitarEncaminhamentoExterno(encId: string, status: string) {
    const ok = await runTransitarExterno(
      () => apiClient.post(`v1/sempras/encaminhamentos-externos/${encId}/transitar`, { status, observacoes: "" }),
      "Encaminhamento atualizado",
    );
    if (ok) void ficha.mutate();
  }

  // Centro POP: documentos do munícipe (mesmo padrão dos documentos da
  // Casa Abrigo, mas chaveados pelo cidadão, não por um acolhimento).
  const { run: runDocumentoCp } = useAction();
  const [tipoDocumentoCp, setTipoDocumentoCp] = useState<TipoDocumentoCentroPop>("identificacao");
  const [enviandoCp, setEnviandoCp] = useState(false);
  const { run: runRemoverDocumentoCp } = useAction();
  const fileInputCp = useRef<HTMLInputElement>(null);

  async function enviarDocumentoCp(file: File) {
    setEnviandoCp(true);
    await runDocumentoCp(async () => {
      const { data } = await apiClient.post<{ documento: DocumentoCentroPop; upload: UploadTicket }>(
        `v1/sempras/cidadaos/${id}/centro-pop-documentos/uploads`,
        { unidade_id: unidadeId, tipo: tipoDocumentoCp, filename: file.name, content_type: mimeOf(file), size: file.size },
      );
      await putToTicket(data.upload, file);
      await apiClient.post(`v1/sempras/centro-pop-documentos/${data.documento.id}/confirm`);
    }, "Documento enviado");
    setEnviandoCp(false);
    void ficha.mutate();
  }

  async function baixarDocumentoCp(docId: string) {
    const res = await runDocumentoCp(() => apiClient.get<{ url: string }>(`v1/sempras/centro-pop-documentos/${docId}/download`));
    if (res) window.location.assign(res.data.url);
  }

  async function removerDocumentoCp(docId: string) {
    const ok = await runRemoverDocumentoCp(() => apiClient.delete(`v1/sempras/centro-pop-documentos/${docId}`), "Documento removido");
    if (ok) void ficha.mutate();
  }

  const c = ficha.data?.cidadao;
  return (
    <div className="flex flex-col gap-6">
      <Link href={unidadeId ? `/sempras/u/${unidadeId}` : "/sempras"} className="inline-flex items-center gap-1 text-sm text-primary underline">
        <ArrowLeft size={14} aria-hidden="true" /> Voltar ao atendimento
      </Link>
      <DataState loading={ficha.isLoading} error={ficha.error} onRetry={() => void ficha.mutate()} empty={!c}>
        {c && (
          <>
            <PageHeader eyebrow="Ficha do munícipe" title={c.nome_social || c.nome}
              description="Acesso registrado na trilha de auditoria (LGPD). Mostra só o histórico das unidades onde você atua." />
            {(unidades.data?.length ?? 0) > 1 && (
              <div className="max-w-sm">
                <Select id="ficha-unidade" label="Atendendo pela unidade" value={unidadeId} onChange={(e) => setUnidade(e.target.value)}
                  options={(unidades.data ?? []).map((u) => ({ value: u.id, label: `${u.nome} (${TIPOS_UNIDADE[u.tipo]?.label ?? u.tipo})` }))} />
              </div>
            )}
            {(ficha.data?.outras_unidades?.length ?? 0) > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle as="h2">Acompanhamento em outras unidades</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-3 pb-5">
                  <p className="text-xs text-muted">
                    Só o resumo: o conteúdo é de quem atende lá. A evolução social aparece quando a unidade encaminhou o munícipe para cá.
                  </p>
                  <ul className="flex flex-wrap gap-2">
                    {ficha.data!.outras_unidades.map((o) => (
                      <li key={o.unidade_id}>
                        <Badge tone="neutral">
                          {o.unidade_nome} — {o.ativos.map((a) => ACOMPANHAMENTOS[a] ?? a).join(", ")}
                        </Badge>
                      </li>
                    ))}
                  </ul>
                  {ficha.data!.registros_origem.length > 0 && (
                    <ol className="flex flex-col divide-y divide-surface-border rounded-xl border border-info/30 bg-info/5">
                      {ficha.data!.registros_origem.map((r) => (
                        <li key={r.id} className="flex flex-col gap-1 p-3 text-sm">
                          <span className="text-xs text-muted">
                            {r.autor_nome || "Técnico"} · {ficha.data!.outras_unidades.find((o) => o.unidade_id === r.unidade_id)?.unidade_nome ?? "outra unidade"} · {fmtDateTime(r.created_at)}
                          </span>
                          <p className="whitespace-pre-wrap">{r.texto}</p>
                        </li>
                      ))}
                    </ol>
                  )}
                </CardContent>
              </Card>
            )}
            <Card>
              <CardHeader className="flex flex-wrap items-center justify-between gap-3">
                <CardTitle as="h2">Cadastro</CardTitle>
                <div className="flex flex-wrap gap-2">
                  <Button size="sm" variant="secondary" onClick={() => setEditando(true)}>
                    <Pencil size={14} aria-hidden="true" className="mr-1" /> Editar
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => setExportando(true)}>
                    <Download size={14} aria-hidden="true" className="mr-1" /> Exportar dados
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => setAnonimizando(true)}>
                    <ShieldAlert size={14} aria-hidden="true" className="mr-1" /> Solicitar anonimização
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="pb-5">
                <dl className="mt-3 grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
                  {[
                    ["Nome civil", c.nome], ["Nome social", c.nome_social], ["CPF", c.cpf], ["NIS", c.nis],
                    ["Nascimento", c.data_nascimento], ["Mãe", c.nome_mae], ["Telefone", c.telefone], ["Endereço", c.endereco],
                  ].map(([rotulo, valor]) => (
                    <div key={rotulo}>
                      <dt className="text-muted">{rotulo}</dt>
                      <dd>{valor || "—"}</dd>
                    </div>
                  ))}
                </dl>
              </CardContent>
            </Card>

            <div className="grid gap-6 xl:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle as="h2">Atendimentos</CardTitle>
                </CardHeader>
                <CardContent className="pb-5">
                  {ficha.data!.atendimentos.length === 0 ? (
                    <p className="mt-3 text-sm text-muted">Sem atendimentos nas suas unidades.</p>
                  ) : (
                    <ul aria-label="Atendimentos" className="mt-3 flex flex-col gap-2 text-sm">
                      {ficha.data!.atendimentos.map((a) => (
                        <li key={a.id} className="flex flex-wrap items-center gap-2">
                          <Badge tone={STATUS[a.status].tone}>{STATUS[a.status].label}</Badge>
                          <span>{a.tipo === "psicologico" ? "Psicológico" : "Social"}</span>
                          {a.prioridade && <span className="text-muted">{rotuloPrioridade(a.prioridade)}</span>}
                          <span className="text-muted">{nomes.get(a.unidade_id) ?? ""} · {fmtDateTime(a.created_at)}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle as="h2">Evoluções</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-4 pb-5">
                  <div className="flex flex-wrap items-center justify-between gap-2 border-b border-surface-border pb-3">
                    <span className="text-xs text-muted">
                      {isCentroPopUnidade
                        ? "Registro de evolução rápida ou atendimento especializado do Centro POP (Decreto 7.053/2009)."
                        : isCrasUnidade
                        ? "Registro de evolução rápida ou formulário técnico oficial do CRAS (MDS)."
                        : "Registro de evolução do atendimento."}
                    </span>
                    {isCentroPopUnidade ? (
                      <Button
                        type="button"
                        size="sm"
                        variant={mostrarFormAtendimentoPop ? "secondary" : "primary"}
                        onClick={() => setMostrarFormAtendimentoPop(!mostrarFormAtendimentoPop)}
                      >
                        <Sparkles size={14} className="mr-1" />
                        {mostrarFormAtendimentoPop ? "Ocultar Atendimento Centro POP" : "Abrir Atendimento Técnico Centro POP"}
                      </Button>
                    ) : isCrasUnidade && (
                      <Button
                        type="button"
                        size="sm"
                        variant={mostrarFormAtendimentoCras ? "secondary" : "primary"}
                        onClick={() => setMostrarFormAtendimentoCras(!mostrarFormAtendimentoCras)}
                      >
                        <ClipboardCheck size={14} className="mr-1" />
                        {mostrarFormAtendimentoCras ? "Ocultar Formulário CRAS" : "Abrir Formulário Técnico CRAS (MDS)"}
                      </Button>
                    )}
                  </div>

                  {mostrarFormAtendimentoPop && isCentroPopUnidade ? (
                    <div className="rounded-xl border border-primary/30 bg-surface p-4">
                      <CentroPopAtendimentoForm
                        cidadao={ficha.data!.cidadao}
                        unidadeId={unidadeId}
                        unidadeNome={nomes.get(unidadeId) ?? "Centro POP"}
                        perfil={ficha.data!.centro_pop_perfil}
                        onSalvo={() => {
                          void ficha.mutate();
                          setMostrarFormAtendimentoPop(false);
                        }}
                        onConcluido={() => {
                          void ficha.mutate();
                          setMostrarFormAtendimentoPop(false);
                        }}
                      />
                    </div>
                  ) : mostrarFormAtendimentoCras && isCrasUnidade ? (
                    <div className="rounded-xl border border-primary/30 bg-surface p-4">
                      <CrasAtendimentoForm
                        cidadao={ficha.data!.cidadao}
                        unidadeId={unidadeId}
                        unidadeNome={nomes.get(unidadeId) ?? "CRAS"}
                        onSalvo={() => {
                          void ficha.mutate();
                          setMostrarFormAtendimentoCras(false);
                        }}
                        onConcluido={() => {
                          void ficha.mutate();
                          setMostrarFormAtendimentoCras(false);
                        }}
                      />
                    </div>
                  ) : (
                    <form onSubmit={registrar} className="mt-3 flex flex-col gap-3">
                    <div className="grid gap-3 sm:grid-cols-2">
                      <Select id="evo-unidade" label="Unidade" value={unidadeId} onChange={(e) => setUnidade(e.target.value)}
                        options={(unidades.data ?? []).map((u) => ({ value: u.id, label: u.nome }))} />
                      <Select id="evo-tipo" label="Tipo" value={tipo} onChange={(e) => setTipo(e.target.value)} options={tipos} />
                    </div>
                    <Textarea id="evo-texto" label="Nova evolução *" value={texto} onChange={(e) => setTexto(e.target.value)}
                      maxLength={20000} rows={4} required />
                    <div>
                      <Button type="submit" loading={pending} disabled={!unidadeId}>
                        <NotebookPen size={16} aria-hidden="true" className="mr-1" /> Registrar
                      </Button>
                    </div>
                  </form>
                  )}
                  <ol aria-label="Evoluções" className="flex flex-col gap-3">
                    {ficha.data!.registros.map((r) => (
                      <li key={r.id} className="rounded-lg border border-surface-border p-3 text-sm">
                        <p className="text-xs text-muted">
                          {fmtDateTime(r.created_at)} · {r.autor_nome || "—"} · {nomes.get(r.unidade_id) ?? ""} ·{" "}
                          {r.tipo === "psicologico" ? "Psicológico" : "Social"}
                        </p>
                        {r.sigiloso ? (
                          <p className="mt-1 inline-flex items-center gap-1 text-muted">
                            <Lock size={14} aria-hidden="true" /> Registro sigiloso da psicologia
                          </p>
                        ) : (
                          <p className="mt-1 whitespace-pre-wrap">{r.texto}</p>
                        )}
                      </li>
                    ))}
                  </ol>
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle as="h2">Rede de apoio</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={adicionarContato} className="mt-3 grid gap-3 sm:grid-cols-[1fr_1fr_1fr_auto] sm:items-end">
                  <Input id="rede-nome" name="nome" label="Nome *" maxLength={200} required autoComplete="off" />
                  <Input id="rede-parentesco" name="parentesco" label="Parentesco" maxLength={100} autoComplete="off" />
                  <Input id="rede-telefone" name="telefone" label="Telefone" maxLength={30} autoComplete="off" />
                  <Button type="submit" loading={pendingRedeApoio}>
                    <UserPlus size={16} aria-hidden="true" className="mr-1" /> Adicionar
                  </Button>
                </form>
                {ficha.data!.rede_apoio.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum contato da rede de apoio cadastrado.</p>
                ) : (
                  <ul aria-label="Rede de apoio" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.rede_apoio.map((contato) => (
                      <li key={contato.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">{contato.nome}</p>
                          <p className="text-xs text-muted">
                            {[contato.parentesco, contato.telefone].filter(Boolean).join(" · ") || "—"}
                          </p>
                        </div>
                        <Button variant="ghost" size="sm" aria-label={`Remover ${contato.nome} da rede de apoio`} onClick={() => void removerContato(contato.id)}>
                          <Trash2 size={14} aria-hidden="true" />
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>

            {oferta(unidadeAtual, "beneficio") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Benefícios eventuais</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={solicitarBeneficio} className="mt-3 grid gap-3 sm:grid-cols-[1fr_1fr_auto_1fr_auto] sm:items-end">
                  <Select id="beneficio-tipo" label="Tipo de benefício" value={tipoBeneficio} onChange={(e) => setTipoBeneficio(e.target.value as TipoBeneficio)}
                    options={Object.entries(TIPOS_BENEFICIO).map(([value, label]) => ({ value, label }))} />
                  <Select id="beneficio-forma-acesso" label="Forma de acesso" value={formaAcessoBeneficio}
                    onChange={(e) => setFormaAcessoBeneficio(e.target.value as FormaAcessoBeneficio)}
                    options={Object.entries(FORMAS_ACESSO_BENEFICIO).map(([value, label]) => ({ value, label }))} />
                  <Input id="beneficio-quantidade" label="Qtd." type="number" min={1} max={100} value={quantidadeBeneficio}
                    onChange={(e) => setQuantidadeBeneficio(e.target.value)} className="w-20" />
                  <Input id="beneficio-observacao" label="Observação" maxLength={2000} value={observacaoBeneficio}
                    onChange={(e) => setObservacaoBeneficio(e.target.value)} autoComplete="off" />
                  <Button type="submit" loading={pendingBeneficio} disabled={!unidadeId}>
                    Solicitar
                  </Button>
                </form>
                {ficha.data!.beneficios.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum benefício eventual nas suas unidades.</p>
                ) : (
                  <ul aria-label="Benefícios eventuais" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.beneficios.map((b) => (
                      <li key={b.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">
                            {TIPOS_BENEFICIO[b.tipo]} {b.quantidade > 1 ? `(${b.quantidade}x)` : ""}
                          </p>
                          <p className="text-xs text-muted">
                            {nomes.get(b.unidade_id) ?? ""} · {FORMAS_ACESSO_BENEFICIO[b.forma_acesso]} · {fmtDateTime(b.data_solicitacao)}
                            {b.motivo_decisao && ` · ${b.motivo_decisao}`}
                          </p>
                        </div>
                        <span className="flex items-center gap-2">
                          <Badge tone={STATUS_BENEFICIO[b.status].tone}>{STATUS_BENEFICIO[b.status].label}</Badge>
                          {b.status === "solicitado" && (
                            <>
                              <Button size="sm" loading={pendingDecisaoBeneficio} onClick={() => void conceder(b.id)}>
                                Conceder
                              </Button>
                              <Button size="sm" variant="danger" onClick={() => setDecisaoBeneficio({ id: b.id, acao: "negar" })}>
                                Negar
                              </Button>
                            </>
                          )}
                          {b.status === "concedido" && (
                            <Button size="sm" variant="danger" onClick={() => setDecisaoBeneficio({ id: b.id, acao: "suspender" })}>
                              Suspender
                            </Button>
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "encaminhamento") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Encaminhamentos entre unidades</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={encaminhar} className="mt-3 grid gap-3 sm:grid-cols-[1fr_2fr_auto] sm:items-end">
                  <Select id="encaminhamento-destino" label="Encaminhar para" value={unidadeDestino} onChange={(e) => setUnidadeDestino(e.target.value)}
                    options={[{ value: "", label: "Selecione a unidade" }, ...outrasUnidades.map((u) => ({ value: u.id, label: u.nome }))]} required />
                  <Input id="encaminhamento-motivo" label="Motivo do encaminhamento *" maxLength={2000} value={motivoEncaminhamento}
                    onChange={(e) => setMotivoEncaminhamento(e.target.value)} autoComplete="off" required />
                  <Button type="submit" loading={pendingEncaminhar} disabled={!unidadeId || !unidadeDestino}>
                    Encaminhar
                  </Button>
                </form>
                {ficha.data!.encaminhamentos.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum encaminhamento nas suas unidades.</p>
                ) : (
                  <ul aria-label="Encaminhamentos" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.encaminhamentos.map((enc) => (
                      <li key={enc.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">
                            {nomes.get(enc.unidade_origem_id) ?? ""} → {nomes.get(enc.unidade_destino_id) ?? ""}
                          </p>
                          <p className="text-xs text-muted">
                            {enc.motivo} · {fmtDateTime(enc.created_at)}
                            {enc.observacao_decisao && ` · ${enc.observacao_decisao}`}
                          </p>
                        </div>
                        <Badge tone={STATUS_ENCAMINHAMENTO[enc.status].tone}>{STATUS_ENCAMINHAMENTO[enc.status].label}</Badge>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "paif") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">PAIF (CRAS)</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={incluirPaif} className="mt-3 flex flex-col gap-3">
                  <Select id="paif-tipo-atendimento" label="Tipo de atendimento" value={tipoAtendimentoPaif}
                    onChange={(e) => setTipoAtendimentoPaif(e.target.value as TipoAtendimentoPaif)}
                    options={Object.entries(TIPOS_ATENDIMENTO_PAIF).map(([value, label]) => ({ value, label }))} className="max-w-xs" />
                  <div className="flex flex-wrap gap-4 text-sm">
                    <label className="flex items-center gap-2">
                      <input type="checkbox" checked={extremaPobreza} onChange={(e) => setExtremaPobreza(e.target.checked)} /> Extrema pobreza
                    </label>
                    <label className="flex items-center gap-2">
                      <input type="checkbox" checked={bolsaFamilia} onChange={(e) => setBolsaFamilia(e.target.checked)} /> Bolsa Família
                    </label>
                    <label className="flex items-center gap-2">
                      <input type="checkbox" checked={bpc} onChange={(e) => setBpc(e.target.checked)} /> BPC
                    </label>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
                    <Input id="paif-scfv" label="Participantes no SCFV" type="number" min={0} max={50} value={scfv}
                      onChange={(e) => setScfv(e.target.value)} />
                    <Button type="submit" loading={pendingPaif} disabled={!unidadeId} className="self-end">
                      Incluir no PAIF
                    </Button>
                  </div>
                </form>
                {ficha.data!.paif_vinculos.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum vínculo PAIF nas suas unidades.</p>
                ) : (
                  <ul aria-label="Vínculos PAIF" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.paif_vinculos.map((v) => (
                      <li key={v.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">
                            {[v.extrema_pobreza && "Extrema pobreza", v.bolsa_familia && "Bolsa Família", v.bpc && "BPC"].filter(Boolean).join(" · ") || "Sem indicadores"}
                          </p>
                          <p className="text-xs text-muted">
                            {nomes.get(v.unidade_id) ?? ""} · {TIPOS_ATENDIMENTO_PAIF[v.tipo_atendimento]} · SCFV: {v.scfv_participantes} · desde {fmtDateTime(v.data_inclusao)}
                            {v.motivo_exclusao && ` · ${v.motivo_exclusao}`}
                          </p>
                        </div>
                        <span className="flex items-center gap-2">
                          <Badge tone={STATUS_PAIF[v.status].tone}>{STATUS_PAIF[v.status].label}</Badge>
                          {v.status === "ativo" && (
                            <Button size="sm" variant="danger" onClick={() => setExcluindoPaif(v.id)}>
                              Excluir
                            </Button>
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "conselho") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Conselho Tutelar</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={registrarConselho} className="mt-3 flex flex-col gap-3">
                  <div className="grid gap-3 sm:grid-cols-3">
                    <Input id="conselho-resp-nome" label="Responsável" maxLength={200} value={respNome}
                      onChange={(e) => setRespNome(e.target.value)} autoComplete="off" />
                    <Input id="conselho-resp-cpf" label="CPF do responsável" maxLength={20} value={respCPF}
                      onChange={(e) => setRespCPF(e.target.value)} autoComplete="off" />
                    <Input id="conselho-resp-telefone" label="Telefone do responsável" maxLength={30} value={respTelefone}
                      onChange={(e) => setRespTelefone(e.target.value)} autoComplete="off" />
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Select id="conselho-origem" label="Origem da demanda" value={origemDemanda}
                      onChange={(e) => setOrigemDemanda(e.target.value as OrigemDemandaConselho)}
                      options={Object.entries(ORIGENS_DEMANDA).map(([value, label]) => ({ value, label }))} />
                    <Select id="conselho-motivo" label="Motivo da violação" value={motivoViolacao}
                      onChange={(e) => setMotivoViolacao(e.target.value as MotivoViolacaoConselho)}
                      options={Object.entries(MOTIVOS_VIOLACAO).map(([value, label]) => ({ value, label }))} />
                  </div>
                  <Textarea id="conselho-observacoes" label="Observações" rows={3} maxLength={4000}
                    value={observacoesConselho} onChange={(e) => setObservacoesConselho(e.target.value)} />
                  <div>
                    <Button type="submit" loading={pendingConselho} disabled={!unidadeId}>
                      Abrir atendimento
                    </Button>
                  </div>
                </form>
                {ficha.data!.conselho_atendimentos.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum atendimento do Conselho Tutelar nas suas unidades.</p>
                ) : (
                  <ul aria-label="Atendimentos do Conselho Tutelar" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.conselho_atendimentos.map((ct) => (
                      <li key={ct.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">
                            {ct.protocolo} · {MOTIVOS_VIOLACAO[ct.motivo_violacao]}
                          </p>
                          <p className="text-xs text-muted">
                            {ORIGENS_DEMANDA[ct.origem_demanda]} · {nomes.get(ct.unidade_id) ?? ""} · {fmtDateTime(ct.created_at)}
                            {ct.medida_protetiva && ` · ${MEDIDAS_PROTETIVAS[ct.medida_protetiva]}`}
                          </p>
                        </div>
                        <span className="flex items-center gap-2">
                          <Badge tone={STATUS_CONSELHO[ct.status].tone}>{STATUS_CONSELHO[ct.status].label}</Badge>
                          {ct.status === "em_andamento" && (
                            <>
                              <Button size="sm" loading={pendingTransitarConselho} onClick={() => setAplicandoMedida(ct.id)}>
                                Aplicar medida
                              </Button>
                              <Button size="sm" variant="secondary" loading={pendingTransitarConselho} onClick={() => void transitarConselho(ct.id, "encaminhar_mp")}>
                                Encaminhar ao MP
                              </Button>
                            </>
                          )}
                          {ct.status !== "arquivado" && (
                            <Button size="sm" variant="danger" loading={pendingTransitarConselho} onClick={() => void transitarConselho(ct.id, "arquivar")}>
                              Arquivar
                            </Button>
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            <Dialog open={aplicandoMedida !== null} onClose={() => setAplicandoMedida(null)} title="Aplicar medida protetiva"
              description="Art. 101 do ECA.">
              <form onSubmit={confirmarMedida} className="flex flex-col gap-4">
                <Select id="conselho-medida" label="Medida protetiva *" value={medidaEscolhida}
                  onChange={(e) => setMedidaEscolhida(e.target.value as Exclude<MedidaProtetiva, "">)}
                  options={Object.entries(MEDIDAS_PROTETIVAS).map(([value, label]) => ({ value, label }))} />
                <div>
                  <Button type="submit" loading={pendingTransitarConselho}>
                    Confirmar
                  </Button>
                </div>
              </form>
            </Dialog>

            {oferta(unidadeAtual, "paefi") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">PAEFI (CREAS)</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={registrarPaefi} className="mt-3 flex flex-col gap-3">
                  <fieldset className="flex flex-wrap gap-3 text-sm">
                    <legend className="mb-1 text-muted">Violações (Resolução CNAS nº 109/2009)</legend>
                    {Object.entries(VIOLACOES_PAEFI).map(([value, label]) => (
                      <label key={value} className="flex items-center gap-2">
                        <input type="checkbox" checked={violacoesPaefi.includes(value as ViolacaoPaefi)}
                          onChange={() => toggleViolacaoPaefi(value as ViolacaoPaefi)} /> {label}
                      </label>
                    ))}
                  </fieldset>
                  <Input id="paefi-membros" label="Membros vulneráveis" maxLength={2000} value={membrosVulneraveis}
                    onChange={(e) => setMembrosVulneraveis(e.target.value)} autoComplete="off" />
                  <Textarea id="paefi-observacoes" label="Observações do caso" rows={3} maxLength={4000}
                    value={observacoesPaefi} onChange={(e) => setObservacoesPaefi(e.target.value)} />
                  <div>
                    <Button type="submit" loading={pendingPaefi} disabled={!unidadeId || violacoesPaefi.length === 0}>
                      Abrir caso
                    </Button>
                  </div>
                </form>
                {ficha.data!.paefi_casos.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum caso do PAEFI nas suas unidades.</p>
                ) : (
                  <ul aria-label="Casos do PAEFI" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.paefi_casos.map((p) => (
                      <li key={p.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">
                            {p.caso_codigo} · {p.violacoes.map((v) => VIOLACOES_PAEFI[v]).join(", ")}
                          </p>
                          <p className="text-xs text-muted">
                            {nomes.get(p.unidade_id) ?? ""} · {fmtDateTime(p.data_abertura)} · PIA {p.pia_elaborado ? "elaborado" : "pendente"}
                            {p.motivo_encerramento && ` · ${MOTIVOS_ENCERRAMENTO_PAEFI[p.motivo_encerramento]}`}
                          </p>
                        </div>
                        <span className="flex items-center gap-2">
                          <Badge tone={STATUS_PAEFI[p.status].tone}>{STATUS_PAEFI[p.status].label}</Badge>
                          {p.status === "em_acompanhamento" && (
                            <>
                              {!p.pia_elaborado && (
                                <Button size="sm" loading={pendingPiaElaborado} onClick={() => void marcarPiaElaborado(p.id)}>
                                  Marcar PIA elaborado
                                </Button>
                              )}
                              <Button size="sm" variant="danger" onClick={() => setEncerrandoPaefi(p.id)}>
                                Encerrar
                              </Button>
                            </>
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "centro_pop") && (
              <>
                <AbordagemSocial unidadeId={unidadeId} cidadaoId={id} />
                {(ficha.data!.abordagens ?? []).length > 0 && (
                  <ul aria-label="Abordagens do munícipe" className="-mt-4 flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border bg-surface">
                    {ficha.data!.abordagens.map((a) => (
                      <li key={a.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <span>
                          {fmtDateTime(a.data_abordagem)} · {a.local}
                          {a.observacoes && <span className="text-muted"> · {a.observacoes}</span>}
                        </span>
                        <Badge tone={RESULTADOS_ABORDAGEM[a.resultado].tone}>{RESULTADOS_ABORDAGEM[a.resultado].label}</Badge>
                      </li>
                    ))}
                  </ul>
                )}
              </>
            )}

            {oferta(unidadeAtual, "casa_mulher") && (
              <CasaMulherCard cidadaoId={id} unidadeId={unidadeId} casos={ficha.data!.casa_mulher_casos ?? []}
                nomeUnidade={(u) => nomes.get(u) ?? ""} onChanged={() => void ficha.mutate()} />
            )}

            {oferta(unidadeAtual, "mse") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Medidas socioeducativas (MSE)</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={registrarMse} className="mt-3 flex flex-col gap-3">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Input id="mse-processo" label="Número do processo" maxLength={100} value={numeroProcesso}
                      onChange={(e) => setNumeroProcesso(e.target.value)} autoComplete="off" />
                    <Input id="mse-vara" label="Vara de origem" maxLength={200} value={varaOrigem}
                      onChange={(e) => setVaraOrigem(e.target.value)} autoComplete="off" />
                  </div>
                  <Input id="mse-ato" label="Ato infracional *" maxLength={500} value={atoInfracional}
                    onChange={(e) => setAtoInfracional(e.target.value)} autoComplete="off" required />
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Select id="mse-tipo" label="Tipo de medida" value={tipoMedida} onChange={(e) => setTipoMedida(e.target.value as TipoMse)}
                      options={Object.entries(TIPOS_MSE).map(([value, label]) => ({ value, label }))} />
                    <Input id="mse-instituicao" label="Instituição parceira" maxLength={200} value={instituicaoParceira}
                      onChange={(e) => setInstituicaoParceira(e.target.value)} autoComplete="off" />
                  </div>
                  <div>
                    <Button type="submit" loading={pendingMse} disabled={!unidadeId || atoInfracional.trim().length < 3}>
                      Abrir medida
                    </Button>
                  </div>
                </form>
                {ficha.data!.mse_medidas.length === 0 ? (
                  <p className="text-sm text-muted">Nenhuma medida socioeducativa nas suas unidades.</p>
                ) : (
                  <ul aria-label="Medidas socioeducativas" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.mse_medidas.map((m) => (
                      <li key={m.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">{TIPOS_MSE[m.tipo_medida]}</p>
                          <p className="text-xs text-muted">
                            {m.ato_infracional} · {nomes.get(m.unidade_id) ?? ""} · desde {fmtDateTime(m.data_inicio)}
                          </p>
                        </div>
                        <span className="flex items-center gap-2">
                          <Badge tone={STATUS_MSE[m.status].tone}>{STATUS_MSE[m.status].label}</Badge>
                          {m.status === "em_cumprimento" && (
                            <Button size="sm" onClick={() => setConcluindoMse(m.id)}>
                              Concluir
                            </Button>
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "notificacao") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Notificações do CREAS</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={registrarNotificacao} className="mt-3 flex flex-col gap-3">
                  <div className="grid gap-3 sm:grid-cols-[1fr_2fr]">
                    <Select id="notificacao-origem" label="Origem da notificação" value={origemNotificacao}
                      onChange={(e) => setOrigemNotificacao(e.target.value as OrigemNotificacaoCreas)}
                      options={Object.entries(ORIGENS_NOTIFICACAO_CREAS).map(([value, label]) => ({ value, label }))} />
                    <Input id="notificacao-assunto" label="Assunto *" maxLength={2000} value={assuntoNotificacao}
                      onChange={(e) => setAssuntoNotificacao(e.target.value)} autoComplete="off" required />
                  </div>
                  <Input id="notificacao-prazo" label="Prazo de resposta" type="date" value={prazoNotificacao}
                    onChange={(e) => setPrazoNotificacao(e.target.value)} className="w-40" />
                  <div>
                    <Button type="submit" loading={pendingNotificacao} disabled={!unidadeId || assuntoNotificacao.trim().length < 3}>
                      Protocolar notificação
                    </Button>
                  </div>
                </form>
                {ficha.data!.notificacoes_creas.length === 0 ? (
                  <p className="text-sm text-muted">Nenhuma notificação do CREAS nas suas unidades.</p>
                ) : (
                  <ul aria-label="Notificações do CREAS" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.notificacoes_creas.map((n) => {
                      const vencida = !!n.prazo_resposta && n.status !== "respondida" && n.status !== "arquivada"
                        && new Date(n.prazo_resposta).getTime() < new Date().getTime();
                      return (
                      <li key={n.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">
                            {n.protocolo} · {ORIGENS_NOTIFICACAO_CREAS[n.origem_entidade]}
                          </p>
                          <p className="text-xs text-muted">
                            {n.assunto} · {fmtDateTime(n.data_entrada)}
                            {n.prazo_resposta && ` · Prazo: ${fmtDate(n.prazo_resposta)}`}
                            {n.resposta && ` · ${n.resposta}`}
                          </p>
                        </div>
                        <span className="flex items-center gap-2">
                          {vencida && <Badge tone="danger">Prazo vencido</Badge>}
                          <Badge tone={STATUS_NOTIFICACAO_CREAS[n.status].tone}>{STATUS_NOTIFICACAO_CREAS[n.status].label}</Badge>
                          {n.status === "recebida" && (
                            <Button size="sm" loading={pendingTransitarNotificacao} onClick={() => void iniciarDiligencia(n.id)}>
                              Iniciar diligência
                            </Button>
                          )}
                          {(n.status === "recebida" || n.status === "em_diligencia") && (
                            <Button size="sm" variant="secondary" onClick={() => setRespondendoNotificacao(n.id)}>
                              Responder
                            </Button>
                          )}
                          {n.status !== "arquivada" && (
                            <Button size="sm" variant="danger" loading={pendingTransitarNotificacao} onClick={() => void arquivarNotificacao(n.id)}>
                              Arquivar
                            </Button>
                          )}
                        </span>
                      </li>
                      );
                    })}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "oficio") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Ofícios recebidos (CRAS)</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={registrarOficio} className="mt-3 flex flex-col gap-3">
                  <div className="grid gap-3 sm:grid-cols-3">
                    <Input id="oficio-orgao" label="Órgão emissor *" maxLength={200} value={orgaoOficio}
                      onChange={(e) => setOrgaoOficio(e.target.value)} autoComplete="off" required />
                    <Input id="oficio-numero" label="Nº do ofício *" maxLength={100} value={numeroOficio}
                      onChange={(e) => setNumeroOficio(e.target.value)} autoComplete="off" required />
                    <Input id="oficio-assunto" label="Assunto" maxLength={2000} value={assuntoOficio}
                      onChange={(e) => setAssuntoOficio(e.target.value)} autoComplete="off" />
                  </div>
                  <div>
                    <Button type="submit" loading={pendingOficio}
                      disabled={!unidadeId || orgaoOficio.trim().length < 3 || numeroOficio.trim().length < 1}>
                      Registrar ofício
                    </Button>
                  </div>
                </form>
                {ficha.data!.oficios_cras.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum ofício do CRAS nas suas unidades.</p>
                ) : (
                  <ul aria-label="Ofícios do CRAS" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.oficios_cras.map((o) => (
                      <li key={o.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">
                            {o.numero_oficio} · {o.orgao_emissor}
                          </p>
                          <p className="text-xs text-muted">
                            {o.assunto} · {fmtDateTime(o.data_recebimento)}
                            {o.numero_resposta && ` · resposta nº ${o.numero_resposta}`}
                          </p>
                        </div>
                        <span className="flex items-center gap-2">
                          <Badge tone={STATUS_OFICIO_CRAS[o.status].tone}>{STATUS_OFICIO_CRAS[o.status].label}</Badge>
                          {o.status === "pendente" && (
                            <Button size="sm" variant="secondary" onClick={() => setRespondendoOficio(o.id)}>
                              Responder
                            </Button>
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "acolhimento") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Casa Abrigo</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                {!ficha.data!.casa_abrigo_acolhimentos.some((a) => a.status === "acolhido") && (
                  <Link href={`/sempras/u/${unidadeId}?novo=${id}`} className={buttonClass("primary", "md", "w-fit")}>
                    Novo acolhimento (assistente de 6 etapas)
                  </Link>
                )}
                {ficha.data!.casa_abrigo_acolhimentos.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum acolhimento nas suas unidades.</p>
                ) : (
                  <ul aria-label="Acolhimentos" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.casa_abrigo_acolhimentos.map((a) => (
                      <AcolhimentoItem key={a.id} acolhimento={a} nomeUnidade={nomes.get(a.unidade_id) ?? ""} onChanged={() => void ficha.mutate()} />
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "centro_pop") && (
              <CentroPopPerfilCard cidadaoId={id} unidadeId={unidadeId} perfil={ficha.data!.centro_pop_perfil}
                onChanged={() => void ficha.mutate()} />
            )}

            {oferta(unidadeAtual, "centro_pop") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Encaminhamentos externos</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                <form onSubmit={registrarEncaminhamentoExterno} className="mt-3 flex flex-col gap-3">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Select id="enc-externo-destino" label="Destino" value={destinoExterno}
                      onChange={(e) => setDestinoExterno(e.target.value as DestinoExterno)}
                      options={Object.entries(DESTINOS_EXTERNOS).map(([value, label]) => ({ value, label }))} />
                    <Select id="enc-externo-prioridade" label="Prioridade" value={prioridadeExterno}
                      onChange={(e) => setPrioridadeExterno(e.target.value as PrioridadeEncaminhamentoExterno)}
                      options={Object.entries(PRIORIDADES_ENCAMINHAMENTO_EXTERNO).map(([value, label]) => ({ value, label }))} />
                  </div>
                  <Textarea id="enc-externo-motivo" label="Motivo do encaminhamento externo *" rows={2} maxLength={2000}
                    value={motivoExterno} onChange={(e) => setMotivoExterno(e.target.value)} required />
                  <div>
                    <Button type="submit" loading={pendingEncaminhamentoExterno} disabled={!unidadeId || motivoExterno.trim().length < 3}>
                      Registrar encaminhamento externo
                    </Button>
                  </div>
                </form>
                {ficha.data!.encaminhamentos_externos.length === 0 ? (
                  <p className="text-sm text-muted">Nenhum encaminhamento externo nas suas unidades.</p>
                ) : (
                  <ul aria-label="Encaminhamentos externos" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                    {ficha.data!.encaminhamentos_externos.map((enc) => (
                      <li key={enc.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                        <div>
                          <p className="font-medium">{DESTINOS_EXTERNOS[enc.destino]}</p>
                          <p className="text-xs text-muted">
                            {enc.motivo} · {fmtDateTime(enc.created_at)}
                            {enc.observacoes && ` · ${enc.observacoes}`}
                          </p>
                        </div>
                        <span className="flex items-center gap-2">
                          <Badge tone={STATUS_ENCAMINHAMENTO_EXTERNO[enc.status].tone}>{STATUS_ENCAMINHAMENTO_EXTERNO[enc.status].label}</Badge>
                          {enc.status !== "concluido" && enc.status !== "cancelado" && (
                            <Button size="sm" loading={pendingTransitarExterno} onClick={() => void transitarEncaminhamentoExterno(enc.id, "concluido")}>
                              Concluir
                            </Button>
                          )}
                          {enc.status === "pendente" && (
                            <Button size="sm" variant="danger" loading={pendingTransitarExterno} onClick={() => void transitarEncaminhamentoExterno(enc.id, "cancelado")}>
                              Cancelar
                            </Button>
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
            )}

            {oferta(unidadeAtual, "centro_pop") && (
            <Card>
              <CardHeader>
                <CardTitle as="h2">Documentos (Centro POP)</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-4 pb-5">
                {ficha.data!.centro_pop_documentos.length > 0 && (
                  <ul aria-label="Documentos do Centro POP" className="flex flex-col gap-1">
                    {ficha.data!.centro_pop_documentos.map((doc) => (
                      <li key={doc.id} className="flex items-center justify-between gap-2 text-sm">
                        <span>
                          {doc.nome_original} · {TIPOS_DOCUMENTO_CENTRO_POP[doc.tipo]}
                          {doc.status === "pending" && <Badge tone="warning">Enviando</Badge>}
                        </span>
                        <span className="flex gap-1">
                          {doc.status === "ready" && (
                            <Button size="sm" variant="ghost" aria-label={`Baixar documento ${doc.nome_original}`} onClick={() => void baixarDocumentoCp(doc.id)}>
                              Baixar
                            </Button>
                          )}
                          <Button size="sm" variant="ghost" aria-label={`Remover documento ${doc.nome_original}`} onClick={() => void removerDocumentoCp(doc.id)}>
                            Remover
                          </Button>
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
                <div className="mt-2 grid gap-2 sm:grid-cols-[1fr_auto] sm:items-end">
                  <Select id="centro-pop-documento-tipo" label="Tipo de documento" value={tipoDocumentoCp}
                    onChange={(e) => setTipoDocumentoCp(e.target.value as TipoDocumentoCentroPop)}
                    options={Object.entries(TIPOS_DOCUMENTO_CENTRO_POP).map(([value, label]) => ({ value, label }))} />
                  <input ref={fileInputCp} type="file" accept="application/pdf" className="sr-only" id="centro-pop-documento-arquivo"
                    onChange={(e) => { const f = e.target.files?.[0]; if (f) void enviarDocumentoCp(f); e.target.value = ""; }} />
                  <Button size="sm" loading={enviandoCp} disabled={!unidadeId} onClick={() => fileInputCp.current?.click()}>
                    {enviandoCp ? "Enviando…" : "Enviar PDF"}
                  </Button>
                </div>
              </CardContent>
            </Card>
            )}

            <Dialog open={encerrandoPaefi !== null} onClose={() => setEncerrandoPaefi(null)} title="Encerrar caso do PAEFI">
              <form onSubmit={confirmarEncerramentoPaefi} className="flex flex-col gap-4">
                <Select id="paefi-motivo-encerramento" label="Motivo do encerramento" value={motivoEncerramentoPaefi}
                  onChange={(e) => setMotivoEncerramentoPaefi(e.target.value as Exclude<MotivoEncerramentoPaefi, "">)}
                  options={Object.entries(MOTIVOS_ENCERRAMENTO_PAEFI).map(([value, label]) => ({ value, label }))} />
                <div>
                  <Button type="submit" variant="danger" loading={pendingEncerrarPaefi}>
                    Confirmar
                  </Button>
                </div>
              </form>
            </Dialog>

            <Dialog open={concluindoMse !== null} onClose={() => setConcluindoMse(null)} title="Concluir medida socioeducativa">
              <form onSubmit={confirmarConclusaoMse} className="flex flex-col gap-4">
                <Select id="mse-resultado" label="Resultado" value={resultadoMse}
                  onChange={(e) => setResultadoMse(e.target.value as "cumprida" | "descumprida" | "extinta")}
                  options={[{ value: "cumprida", label: "Cumprida" }, { value: "descumprida", label: "Descumprida" }, { value: "extinta", label: "Extinta" }]} />
                <div>
                  <Button type="submit" loading={pendingConcluirMse}>
                    Confirmar
                  </Button>
                </div>
              </form>
            </Dialog>

            <Dialog open={respondendoNotificacao !== null} onClose={() => setRespondendoNotificacao(null)} title="Responder notificação">
              <form onSubmit={confirmarRespostaNotificacao} className="flex flex-col gap-4">
                <Textarea id="notificacao-resposta" label="Resposta *" rows={3} maxLength={4000}
                  value={respostaNotificacao} onChange={(e) => setRespostaNotificacao(e.target.value)} required />
                <div>
                  <Button type="submit" loading={pendingTransitarNotificacao} disabled={respostaNotificacao.trim().length < 3}>
                    Confirmar
                  </Button>
                </div>
              </form>
            </Dialog>

            <Dialog open={respondendoOficio !== null} onClose={() => setRespondendoOficio(null)} title="Responder ofício">
              <form onSubmit={confirmarRespostaOficio} className="flex flex-col gap-4">
                <Input id="oficio-numero-resposta" label="Nº da resposta *" maxLength={100}
                  value={numeroRespostaOficio} onChange={(e) => setNumeroRespostaOficio(e.target.value)} autoComplete="off" required />
                <div>
                  <Button type="submit" loading={pendingResponderOficio} disabled={numeroRespostaOficio.trim().length < 1}>
                    Confirmar
                  </Button>
                </div>
              </form>
            </Dialog>

            <Dialog open={excluindoPaif !== null} onClose={() => setExcluindoPaif(null)} title="Excluir vínculo PAIF">
              <form onSubmit={confirmarExclusaoPaif} className="flex flex-col gap-4">
                <Textarea id="paif-motivo-exclusao" label="Motivo da exclusão *" rows={3} maxLength={1000}
                  value={motivoExclusaoPaif} onChange={(e) => setMotivoExclusaoPaif(e.target.value)} required />
                <div>
                  <Button type="submit" variant="danger" loading={pendingExcluirPaif} disabled={motivoExclusaoPaif.trim().length < 3}>
                    Confirmar
                  </Button>
                </div>
              </form>
            </Dialog>

            <Dialog open={decisaoBeneficio !== null} onClose={() => setDecisaoBeneficio(null)}
              title={decisaoBeneficio?.acao === "negar" ? "Negar benefício" : "Suspender benefício"}>
              <form onSubmit={confirmarDecisaoBeneficio} className="flex flex-col gap-4">
                <Textarea id="beneficio-motivo" label="Motivo *" rows={3} maxLength={1000}
                  value={motivoBeneficio} onChange={(e) => setMotivoBeneficio(e.target.value)} required />
                <div>
                  <Button type="submit" variant="danger" loading={pendingDecisaoBeneficio} disabled={motivoBeneficio.trim().length < 3}>
                    Confirmar
                  </Button>
                </div>
              </form>
            </Dialog>

            <Dialog open={editando} onClose={() => setEditando(false)} title="Editar cadastro" size="lg"
              description="Informe CPF, NIS ou, na falta deles, data de nascimento e nome da mãe.">
              <form onSubmit={salvarEdicao} className="flex flex-col gap-4">
                <div className="grid gap-3 sm:grid-cols-2">
                  {CAMPOS_CIDADAO.map((campo) => (
                    <Input key={campo.name} id={`edit-${campo.name}`} name={campo.name} label={campo.label} type={campo.type}
                      maxLength={campo.max} defaultValue={c[campo.name]} required={campo.name === "nome"} autoComplete="off" />
                  ))}
                  <Select id="edit-sexo" name="sexo" label="Sexo" options={SEXOS} defaultValue={c.sexo ?? ""} />
                </div>
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" name="anonimo" defaultChecked={c.anonimo} />
                  Cadastro anônimo (Centro POP — sem documento, nascimento ou nome da mãe confiáveis)
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={pedidoCorrecao} onChange={(e) => setPedidoCorrecao(e.target.checked)} />
                  A pedido do titular (LGPD art. 18) — registra o pedido de correção
                </label>
                {pedidoCorrecao && (
                  <Textarea id="edit-justificativa" label="Justificativa *" rows={3} maxLength={1000}
                    value={justificativaCorrecao} onChange={(e) => setJustificativaCorrecao(e.target.value)} required />
                )}
                <div>
                  <Button type="submit" loading={pendingEditar} disabled={pedidoCorrecao && justificativaCorrecao.trim().length < 3}>
                    Salvar
                  </Button>
                </div>
              </form>
            </Dialog>

            <Dialog open={exportando} onClose={() => setExportando(false)} title="Exportar dados do munícipe"
              description="Direito de acesso (LGPD art. 18, II e V): baixa o cadastro e o histórico das suas unidades em JSON, para entregar ao titular.">
              <form onSubmit={(e) => e.preventDefault()} className="flex flex-col gap-4">
                <Textarea id="export-justificativa" label="Justificativa *" rows={3} maxLength={1000}
                  value={justificativaAcesso} onChange={(e) => setJustificativaAcesso(e.target.value)} required />
                <div>
                  <a
                    href={`/api/backend/v1/sempras/cidadaos/${id}/pedidos/acesso?justificativa=${encodeURIComponent(justificativaAcesso.trim())}`}
                    download
                    aria-disabled={justificativaAcesso.trim().length < 3}
                    onClick={(e) => {
                      if (justificativaAcesso.trim().length < 3) e.preventDefault();
                      else setExportando(false);
                    }}
                    className={buttonClass("primary", "md", justificativaAcesso.trim().length < 3 ? "pointer-events-none opacity-50" : "")}
                  >
                    <Download size={16} aria-hidden="true" className="mr-1" /> Baixar dados
                  </a>
                </div>
              </form>
            </Dialog>

            <Dialog open={anonimizando} onClose={() => setAnonimizando(false)} title="Solicitar anonimização"
              description="Direito de eliminação (LGPD art. 18, VI). O cadastro também serve política pública — fica pendente até a gestão (Superintendente) aprovar ou negar.">
              <form onSubmit={solicitarAnonimizacao} className="flex flex-col gap-4">
                <Textarea id="anon-justificativa" label="Justificativa *" rows={3} maxLength={1000}
                  value={justificativaAnonimizacao} onChange={(e) => setJustificativaAnonimizacao(e.target.value)} required />
                <div>
                  <Button type="submit" variant="danger" loading={pendingAnonimizar} disabled={justificativaAnonimizacao.trim().length < 3}>
                    Registrar pedido
                  </Button>
                </div>
              </form>
            </Dialog>
          </>
        )}
      </DataState>
    </div>
  );
}
