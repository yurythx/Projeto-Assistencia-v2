import { dadosVazios, piaVazia } from "@/lib/sempras/casaAbrigo";

/** Item da fila no formato da API. */
export const item = (id: string, extra: Record<string, unknown> = {}) => ({
  id,
  cidadao_id: `c-${id}`,
  unidade_id: "u1",
  dia: "2026-10-08",
  senha: 1,
  senha_exibida: `N00${id}`,
  tipo: "social",
  prioridade: "",
  status: "aguardando",
  local: "",
  demanda: "atendimento_tecnico",
  demanda_detalhe: "",
  nome: `Pessoa ${id}`,
  created_at: "2026-10-08T10:00:00Z",
  chamado_em: null,
  iniciado_em: null,
  encerrado_em: null,
  ...extra,
});

/** Prontuário com um item de cada serviço, em todos os estados que
 * acendem botões na ficha. Compartilhado pelos testes da SEMPRAS. */
export const PRONTUARIO = {
  cidadao: { id: "c1", nome: "João", nome_social: "", cpf: "52998224725", nis: "", data_nascimento: "1990-05-01", nome_mae: "Ana", telefone: "", endereco: "", anonimo: false },
  atendimentos: [{ ...item("1", { status: "concluido", prioridade: "idoso" }) }],
  registros: [
    { id: "r1", unidade_id: "u1", atendimento_id: null, tipo: "psicologico", texto: "", sigiloso: true, autor_nome: "Psi", created_at: "2026-10-08T11:00:00Z" },
    { id: "r2", unidade_id: "u1", atendimento_id: null, tipo: "social", texto: "Orientação sobre benefícios", sigiloso: false, autor_nome: "", created_at: "2026-10-08T10:00:00Z" },
  ],
  rede_apoio: [{ id: "ra1", cidadao_id: "c1", nome: "Maria", parentesco: "Irmã", telefone: "66999990000", created_at: "2026-10-08T09:00:00Z" }],
  beneficios: [
    { id: "b1", beneficiario_id: "c1", solicitante_id: null, unidade_id: "u1", tipo: "cesta_basica", forma_acesso: "demanda_espontanea",
      quantidade: 1, status: "solicitado",
      observacao: "", motivo_decisao: "", tecnico_decisao_id: null, data_solicitacao: "2026-10-08T08:00:00Z", data_decisao: null },
  ],
  encaminhamentos: [
    { id: "enc1", cidadao_id: "c1", unidade_origem_id: "u1", unidade_destino_id: "u2", motivo: "precisa de acompanhamento especializado",
      status: "pendente", observacao_decisao: "", encaminhado_por: null, decidido_por: null, created_at: "2026-10-08T07:00:00Z", decidido_em: null },
  ],
  paif_vinculos: [
    { id: "paif1", cidadao_id: "c1", unidade_id: "u1", status: "ativo", tipo_atendimento: "particularizado",
      extrema_pobreza: true, bolsa_familia: false, bpc: false,
      scfv_participantes: 2, motivo_exclusao: "", incluido_por: null, excluido_por: null, data_inclusao: "2026-10-08T06:00:00Z", data_exclusao: null },
  ],
  conselho_atendimentos: [
    { id: "ct1", protocolo: "CT-ABCD1234", crianca_id: "c1", unidade_id: "u1", responsavel_nome: "", responsavel_cpf: "",
      responsavel_telefone: "", origem_demanda: "escola", motivo_violacao: "evasao_escolar", medida_protetiva: "",
      status: "em_andamento", observacoes: "", conselheiro_id: null, created_at: "2026-10-08T05:00:00Z", atualizado_em: null },
  ],
  paefi_casos: [
    { id: "paefi1", caso_codigo: "PAEFI-ABCD1234", cidadao_id: "c1", unidade_id: "u1", violacoes: ["violencia_fisica"],
      membros_vulneraveis: "", pia_elaborado: false, status: "em_acompanhamento", motivo_encerramento: "",
      tecnico_referencia_id: null, observacoes: "", data_abertura: "2026-10-08T04:00:00Z", data_encerramento: null },
  ],
  mse_medidas: [
    { id: "mse1", adolescente_id: "c1", unidade_id: "u1", numero_processo: "123/2026", vara_origem: "Vara da Infância",
      ato_infracional: "furto (art. 155 CP)", tipo_medida: "liberdade_assistida", instituicao_parceira: "",
      status: "em_cumprimento", tecnico_referencia_id: null, observacoes: "", data_inicio: "2026-10-08T03:00:00Z",
      prazo_previsto: null, data_conclusao: null },
  ],
  notificacoes_creas: [
    { id: "notif1", protocolo: "OF-ABCD1234", vitima_id: "c1", unidade_id: "u1", origem_entidade: "ministerio_publico",
      assunto: "requisição de informações sobre acompanhamento", status: "recebida", resposta: "", tecnico_referencia_id: null,
      data_entrada: "2026-10-08T02:00:00Z", prazo_resposta: null, data_resposta: null },
  ],
  oficios_cras: [
    { id: "oficio1", cidadao_id: "c1", unidade_id: "u1", orgao_emissor: "Conselho Tutelar", numero_oficio: "123/2026",
      data_oficio: null, data_recebimento: "2026-10-08T02:30:00Z", assunto: "acompanhamento da família",
      status: "pendente", numero_resposta: "", data_resposta: null, tecnico_responsavel_id: null },
  ],
  casa_abrigo_acolhimentos: [
    { ...dadosVazios(), id: "acolh1", cidadao_id: "c1", unidade_id: "u1", sexo: "masculino", motivos: ["negligencia", "outro"],
      outro_motivo: "pais em conflito com a lei", processo_judicial: "123/2026", conselheiro_nome: "Conselheira Ana",
      situacao_juridica: "processo_guarda", historico_acolhido_anteriormente: "nao_sei",
      status: "acolhido", desligamento_tipo: "", motivo_desligamento: "", guia_desligamento: "", data_guia_desligamento: null,
      registrado_por: null, desligado_por: null, data_entrada: "2026-10-08T01:00:00Z", data_desligamento: null,
      irmaos: [{ id: "irmao1", acolhimento_id: "acolh1", nome: "Irmão da Criança", data_nascimento: null,
        esta_acolhido: false, unidade_descricao: "", observacao: "", created_at: "2026-10-08T01:10:00Z" }],
      diagnosticos: [{ id: "diag1", acolhimento_id: "acolh1", categoria: "doenca", diagnostico: "asma",
        descricao: "", created_at: "2026-10-08T01:20:00Z" }],
      visitantes: [{ id: "visita1", unidade_id: "u1", acolhimento_id: "acolh1", tipo_visita: "acolhido", nome_visitante: "Tia Maria", documento_identidade: "",
        parentesco_vinculo: "Tia", data_visita: "2026-10-05T00:00:00Z", hora_entrada: "14:00", hora_saida: "16:00",
        observacoes: "", created_at: "2026-10-05T14:00:00Z" }],
      termos: [{ id: "termo1", acolhimento_id: "acolh1", tipo_termo: "passeio", responsavel_retirada: "Tia Maria",
        parentesco_responsavel: "Tia", motivo_observacao: "passeio no parque", data_termo: "2026-10-06T00:00:00Z",
        created_at: "2026-10-06T00:00:00Z" }],
      pia: { ...piaVazia(), acolhimento_id: "acolh1", escola: "Escola Municipal", vacinacao_atualizada: true,
        atualizado_por: null, atualizado_em: "2026-10-07T00:00:00Z" },
      pia_metas: [{ id: "meta1", acolhimento_id: "acolh1", tipo: "intervencao", descricao: "reduzir episódios de agressividade",
        providencia: "atendimento semanal", responsavel: "Psicóloga", prazo: null, esportes: "", cultura: "", lazer: "",
        concluida: false, created_at: "2026-10-07T00:00:00Z" }],
      documentos: [{ id: "doc1", acolhimento_id: "acolh1", tipo: "acolhido", nome_original: "certidao.pdf",
        object_key: "sempras/casa-abrigo/acolh1/2026/10/x.pdf", content_type: "application/pdf", size_bytes: 1024,
        status: "ready", created_at: "2026-10-07T00:00:00Z" }] },
  ],
  centro_pop_perfil: null,
  encaminhamentos_externos: [
    { id: "encext1", cidadao_id: "c1", unidade_id: "u1", destino: "caps_ad", prioridade: "alta",
      motivo: "uso abusivo de substâncias", status: "pendente", observacoes: "", tecnico_id: null,
      created_at: "2026-10-08T00:30:00Z", atualizado_em: "2026-10-08T00:30:00Z" },
  ],
  centro_pop_documentos: [],
};
