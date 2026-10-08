// Tipos da API do Projeto Nexus (espelho dos DTOs do backend Go).

export type UUID = string;

export interface PageMeta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

// ---------------------------------------------------------------- núcleo

export interface Scope {
  perfil: string;
  entidade_id?: UUID;
  unidade_id?: UUID;
  departamento_id?: UUID;
  origem: "manual" | "ad";
  /** Permissões do perfil desta concessão — valem no escopo dela (ADR 013). */
  permissions: string[];
}

export interface Me {
  id: UUID;
  subject: string;
  username: string;
  email: string;
  name: string;
  source: "keycloak" | "local";
  roles: string[];
  groups: string[];
  permissions: string[];
  scopes: Scope[];
}

export interface PermissionInfo {
  key: string;
  description: string;
}

export interface ModuleStatus {
  key: string;
  name: string;
  description: string;
  core: boolean;
  default_enabled: boolean;
  depends_on: string[];
  permissions: PermissionInfo[];
  public: boolean;
  icon: string;
  route: string;
  /** Estado efetivo: configurado ativo e com todas as dependências ativas. */
  enabled: boolean;
  /** Estado desejado gravado pelo administrador. */
  configured: boolean;
  /** Módulos que dependem deste. */
  dependents: string[];
  /** Dependências diretas inativas agora. */
  blocked_by: string[];
}

export interface PublicModule {
  key: string;
  enabled: boolean;
}

export interface Branding {
  app_name: string;
  app_description: string;
  org_name: string;
  logo_url: string;
  favicon_url: string;
  support_email: string;
  support_phone: string;
  support_hours: string;
  tokens: Record<string, string>;
  updated_at?: string;
}

// ------------------------------------------------------------------- IAM

export interface Entidade {
  id: UUID;
  nome: string;
  sigla: string;
  slug: string;
  documento: string;
  ativo: boolean;
}

export interface Unidade {
  id: UUID;
  entidade_id: UUID;
  parent_id?: UUID;
  nome: string;
  sigla: string;
  slug: string;
  ad_group: string;
  email: string;
  telefone: string;
  endereco: string;
  ativo: boolean;
}

export interface Departamento {
  id: UUID;
  unidade_id: UUID;
  nome: string;
  sigla: string;
  slug: string;
  ad_group: string;
  email: string;
  telefone: string;
  ativo: boolean;
}

export interface OrgTree extends Entidade {
  unidades: (Unidade & { departamentos: Departamento[] })[];
}

export interface Perfil {
  id: UUID;
  slug: string;
  nome: string;
  descricao: string;
  permissoes: string[];
  sistema: boolean;
  ativo: boolean;
}

export interface ScopeRef {
  entidade_id?: UUID;
  unidade_id?: UUID;
  departamento_id?: UUID;
}

export interface ADMapping extends ScopeRef {
  id: UUID;
  ad_group: string;
  perfil_id: UUID;
  perfil_nome: string;
  scope_label: string;
  descricao: string;
  created_at: string;
  created_by: string;
}

export interface Lotacao extends ScopeRef {
  id: UUID;
  user_id: UUID;
  perfil_id: UUID;
  perfil_nome: string;
  scope_label: string;
  principal: boolean;
  created_at: string;
}

export interface UserAdmin {
  id: UUID;
  username: string;
  email: string;
  display_name: string;
  active: boolean;
  federated: boolean;
  local_login: boolean;
  roles: string[];
  groups: string[];
  locked_until?: string;
  created_at: string;
  last_seen_at?: string;
  ad_synced_at?: string;
  lotacoes?: Lotacao[];
}

export interface PermissionGroup {
  module: string;
  name: string;
  permissions: PermissionInfo[];
}

// -------------------------------------------------------------- auditoria

export interface AuditRecord {
  id: UUID;
  chain_pos: number;
  actor_id?: UUID;
  actor_subject?: string;
  actor_name?: string;
  actor_roles: string[];
  ip_address?: string;
  user_agent?: string;
  /** JSON livre gravado com a entrada (json.RawMessage no backend; null se ausente). */
  entity_context: unknown;
  action: string;
  resource_type?: string;
  resource_id?: string;
  diff_before?: unknown;
  diff_after?: unknown;
  /** JSON livre gravado com a entrada (json.RawMessage no backend). */
  metadata: unknown;
  correlation_id?: UUID;
  timestamp_utc: string;
  prev_hash: string;
  hash: string;
}

export interface VerifyResult {
  checked: number;
  valid: boolean;
  first_invalid_pos?: number;
  reason?: string;
  verified_at: string;
}

// ------------------------------------------------------------------- blog

/** Público-alvo (ADR 014): secretarias e/ou unidades; vazio = todos. */
export interface Publico {
  entidades: UUID[];
  unidades: UUID[];
}

export interface Post {
  id: UUID;
  slug: string;
  title: string;
  summary: string;
  body?: string;
  cover_object_key?: string;
  cover_url?: string;
  kind: "noticia" | "comunicado";
  status: "draft" | "published" | "archived";
  pinned: boolean;
  author_name?: string;
  /** Unidade dona (ADR 013); ausente = institucional. */
  unidade_id?: UUID;
  publico?: Publico;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface UploadTicket {
  object_key: string;
  upload_url: string;
  method: "PUT";
  headers: Record<string, string>;
  expires_at: string;
}

// --------------------------------------------------------------- catálogo

export interface ServiceChannel {
  type: "online" | "presencial" | "telefone" | "email";
  label: string;
  value: string;
}

export interface CatalogService {
  id: UUID;
  slug: string;
  title: string;
  summary: string;
  description: string;
  category: string;
  audience: string;
  requirements: string[];
  steps: string[];
  channels: ServiceChannel[];
  sla: string;
  cost: string;
  icon: string;
  responsible_unidade_id?: UUID;
  responsible_unidade?: string;
  status: "draft" | "published" | "archived";
  position: number;
  published_at?: string;
  updated_at: string;
}

export interface CatalogCategory {
  name: string;
  count: number;
}

// ---------------------------------------------------------------- contato

export interface ContactSummary {
  id: UUID;
  protocol: string;
  name: string;
  email: string;
  subject: string;
  category: string;
  status: "new" | "in_progress" | "answered" | "archived";
  /** Setor encaminhado (ADR 013); ausente = caixa geral. */
  unidade_id?: UUID;
  created_at: string;
}

export interface ContactMessage extends ContactSummary {
  phone: string;
  message: string;
  consent_at: string;
  ip_address?: string;
  assigned_to?: UUID;
  notes: string;
}

// -------------------------------------------------------------- diretório

export interface Person {
  user_id: UUID;
  name: string;
  username: string;
  email: string;
  job_title: string;
  phone: string;
  extension: string;
  bio: string;
  visible: boolean;
  unidade_id?: UUID;
  unidade: string;
  departamento_id?: UUID;
  departamento: string;
  from_ad: boolean;
}

export interface Sector {
  id: UUID;
  kind: "unidade" | "departamento";
  nome: string;
  sigla: string;
  email: string;
  telefone: string;
  endereco?: string;
  unidade_id?: UUID;
  unidade?: string;
  entidade: string;
}

// ------------------------------------------------------------------ agenda

export interface Room {
  id: UUID;
  name: string;
  location: string;
  capacity: number;
  resources: string[];
  active: boolean;
  /** Unidade dona (ADR 013); ausente = institucional. */
  unidade_id?: UUID;
}

export interface CalendarEvent {
  id: UUID;
  title: string;
  description: string;
  location: string;
  room_id?: UUID;
  room_name?: string;
  starts_at: string;
  ends_at: string;
  all_day: boolean;
  visibility: "public" | "internal" | "private";
  status: "confirmed" | "cancelled";
  /** Público-alvo (ADR 014); vazio = todos. */
  publico?: Publico;
  organizer_id: UUID;
  organizer_name: string;
}

// ---------------------------------------------------------------- arquivos

export interface Folder {
  id: UUID;
  parent_id?: UUID;
  name: string;
  owner_id: UUID;
  owner_name: string;
  /** Unidade dona (ADR 013); ausente = sem dona (vale a da pasta acima). */
  unidade_id?: UUID;
  created_at: string;
  updated_at: string;
}

export interface FileItem {
  id: UUID;
  folder_id: UUID;
  name: string;
  size_bytes: number;
  content_type: string;
  status: "pending" | "ready";
  owner_id: UUID;
  owner_name: string;
  updated_at: string;
}

export interface FolderAccess {
  read: boolean;
  write: boolean;
  manage: boolean;
}

export interface Listing {
  folder?: Folder;
  breadcrumbs: Folder[];
  folders: Folder[];
  files: FileItem[];
  access: FolderAccess;
}

export interface ACLEntry {
  subject_type: "everyone" | "user" | "perfil" | "ad_group" | "unidade" | "departamento";
  subject: string;
  can_write: boolean;
}

// -------------------------------------------------------------------- wiki

export interface WikiPage {
  id: UUID;
  parent_id?: UUID;
  /** Público-alvo próprio (ADR 014); vale com o das páginas acima. */
  publico?: Publico;
  /** Unidade dona (ADR 013); ausente = institucional. */
  unidade_id?: UUID;
  slug: string;
  title: string;
  body?: string;
  position: number;
  version: number;
  updated_by_name: string;
  updated_at: string;
  breadcrumbs?: WikiPage[];
}

export interface WikiRevision {
  id: UUID;
  version: number;
  title: string;
  body?: string;
  summary: string;
  edited_by_name: string;
  edited_at: string;
}

// ------------------------------------------------------------------- busca

export interface SearchResult {
  module: string;
  type: string;
  id: string;
  title: string;
  snippet?: string;
  url: string;
  score: number;
  updated_at?: string;
}

export interface SearchResponse {
  query: string;
  results: SearchResult[];
  modules: string[];
  degraded: string[];
  took_ms: number;
}

// ------------------------------------------------------------------ signum

export interface Signer {
  id: UUID;
  user_id: UUID;
  name: string;
  position: number;
  status: "pending" | "signed" | "refused";
  signed_at?: string;
  signature_hash?: string;
  method?: string;
  reason?: string;
}

export interface Envelope {
  id: UUID;
  title: string;
  description: string;
  document_sha256: string;
  source_module: string;
  source_ref: string;
  sequential: boolean;
  status: "pending" | "completed" | "refused" | "cancelled";
  created_by: UUID;
  created_by_name: string;
  created_at: string;
  completed_at?: string;
  signers: Signer[];
}

export interface Challenge {
  challenge_id: UUID;
  nonce: string;
  expires_at: string;
  document_sha256: string;
}

export interface Verification {
  envelope_id: UUID;
  title: string;
  document_sha256: string;
  status: Envelope["status"];
  document_match?: boolean;
  signatures: {
    name: string;
    status: string;
    signed_at?: string;
    method?: string;
    valid: boolean;
  }[];
  checked_at: string;
}

// ----------------------------------------------------------------- trâmite

export interface TramiteTipo {
  id: UUID;
  slug: string;
  nome: string;
  descricao: string;
}

export interface Processo {
  id: UUID;
  numero: string;
  tipo_id: UUID;
  tipo: string;
  assunto: string;
  interessado: string;
  descricao: string;
  sigilo: "publico" | "restrito" | "sigiloso";
  status: "aberto" | "em_tramitacao" | "concluido" | "arquivado";
  unidade_origem_id: UUID;
  unidade_origem: string;
  unidade_atual_id: UUID;
  unidade_atual: string;
  created_by_name: string;
  created_at: string;
  updated_at: string;
  concluido_at?: string;
  /** Classificação pelo Atlas (ADR 026). */
  atlas_procedimento_id?: UUID | null;
  codigo_ttdd?: string;
}

export interface Documento {
  id: UUID;
  processo_id: UUID;
  tipo: string;
  titulo: string;
  origem: "redigido" | "anexo";
  conteudo?: string;
  content_type?: string;
  size_bytes: number;
  sha256?: string;
  status: "rascunho" | "aguardando_assinatura" | "assinado" | "cancelado";
  envelope_id?: UUID;
  created_at: string;
  download_url?: string;
}

export interface Movimento {
  id: UUID;
  acao: string;
  de_unidade?: string;
  para_unidade?: string;
  despacho: string;
  actor_name?: string;
  created_at: string;
}

export interface ProcessoView extends Processo {
  documentos: Documento[];
  movimentos: Movimento[];
  acessos: { user_id: UUID; name: string; granted_at: string }[];
  can_act: boolean;
  can_route: boolean;
}

// ---------------------------------------------------------------- mercúrio

export interface ChatRoom {
  id: UUID;
  kind: "global" | "department" | "direct";
  name: string;
  description: string;
  ad_group?: string;
  departamento_id?: UUID;
  archived: boolean;
  members?: UUID[];
  unread: number;
  last_message_at?: string;
}

export interface ChatMessage {
  id: UUID;
  room_id: UUID;
  author_id: UUID;
  author_name: string;
  body: string;
  created_at: string;
  edited_at?: string;
  deleted: boolean;
}

// ------------------------------------------------------------------ egress

export interface EgressTarget {
  id: UUID;
  name: string;
  kind: "webhook" | "n8n" | "zabbix" | "grafana";
  url: string;
  has_secret: boolean;
  event_patterns: string[];
  active: boolean;
  created_at: string;
}

export interface EgressDelivery {
  id: UUID;
  target_id: UUID;
  target_name?: string;
  event_id: UUID;
  event_type: string;
  status: "pending" | "delivered" | "failed" | "dead";
  attempts: number;
  next_attempt_at: string;
  last_status_code?: number;
  last_error?: string;
  created_at: string;
  delivered_at?: string;
}

// ------------------------------------------------------------------- atlas

export type DestinacaoFinal = "GUARDA_PERMANENTE" | "ELIMINACAO";
export type NivelAcesso = "PUBLICO" | "RESTRITO" | "SIGILOSO";
export type FormatoDocumento = "NATO_DIGITAL" | "EXTERNO_DIGITALIZADO";
export type TipoAssinatura = "INDIVIDUAL" | "CONJUNTA_MULTINIVEL" | "EM_BLOCO";

/** Órgão dono de uma TTDD, com a publicação oficial (Diário Oficial). */
export interface OrgaoTTDD {
  prefixo: string;
  nome: string;
  edicao_diario: string;
  data_publicacao: string | null;
  versao: string;
}

export interface SubfuncaoTTDD {
  codigo: string;
  nome: string;
  recomendacao: string;
  funcao: { codigo: string; nome: string; orgao: OrgaoTTDD };
}

/** Série documental da TTDD. Cada fase tem prazo em anos OU condição; sem
 * os dois, a fase intermediária não existe (a corrente: não informado).
 * destinacao_final null = a TTDD não define. */
export interface ClassificacaoTTDD {
  codigo: string;
  descritor: string;
  fase_corrente_anos: number | null;
  fase_corrente_condicao: string;
  fase_interm_anos: number | null;
  fase_interm_condicao: string;
  destinacao_final: DestinacaoFinal | null;
  observacoes: string;
  subfuncao?: SubfuncaoTTDD;
  /** Data em que a série saiu da TTDD em vigor (null = vigente) e a edição
   * do Diário Oficial que a retirou. */
  revogada_em: string | null;
  revogada_edicao: string;
  created_at: string;
}

/** Mudança de uma série: os valores ANTERIORES e a publicação que a trouxe. */
export interface HistoricoTTDD {
  evento: "ALTERADA" | "REVOGADA" | "RESTABELECIDA";
  anterior: Pick<
    ClassificacaoTTDD,
    | "descritor"
    | "fase_corrente_anos"
    | "fase_corrente_condicao"
    | "fase_interm_anos"
    | "fase_interm_condicao"
    | "destinacao_final"
    | "observacoes"
  >;
  edicao_diario: string;
  registrado_em: string;
}

export interface EstruturaTTDD extends OrgaoTTDD {
  total: number;
  funcoes: {
    codigo: string;
    nome: string;
    total: number;
    subfuncoes: { codigo: string; nome: string; total: number }[];
  }[];
}

/** Procedimentos de uma secretaria (órgão da TTDD). Rascunhos e em
 * validação só vêm para a gestão (no público, zero). */
export interface ProcedimentosOrgao {
  prefixo: string;
  nome: string;
  publicados: number;
  em_validacao: number;
  rascunhos: number;
}

/** Contagens de um nó do organograma da TTDD. Rascunhos e em validação só
 * vêm para a gestão; lacunas são subfunções com série de processo e nenhum
 * procedimento (para o público: nenhum publicado). */
export interface ContagemOrganograma {
  series: number;
  publicados: number;
  em_validacao: number;
  rascunhos: number;
  lacunas: number;
}

export interface SubfuncaoOrganograma extends ContagemOrganograma {
  codigo: string;
  nome: string;
  series_de_processo: number;
}

export interface FuncaoOrganograma extends ContagemOrganograma {
  codigo: string;
  nome: string;
  subfuncoes: SubfuncaoOrganograma[];
}

export interface OrgaoOrganograma extends ContagemOrganograma {
  prefixo: string;
  nome: string;
  funcoes: FuncaoOrganograma[];
}

export interface EtapaDocumento {
  id: UUID;
  nome_documento: string;
  obrigatorio: boolean;
  formato: FormatoDocumento;
  tipo_assinatura: TipoAssinatura;
  exige_conferencia_copia: boolean;
  modelo_minuta_padrao_url: string;
  /** Modelo da biblioteca ligado à peça (ADR 024), com a versão atual. */
  modelo_id: UUID | null;
  modelo?: ModeloResumo;
}

// ------------------------------------------- biblioteca de modelos (ADR 024)

export interface ModeloResumo {
  id: UUID;
  nome: string;
  versao: number;
  arquivo_nome: string;
}

export interface ModeloVersao {
  versao: number;
  arquivo_nome: string;
  content_type: string;
  tamanho: number;
  sha256: string;
  nota: string;
  created_at: string;
  created_by: string;
}

/** Modelo de documento: a versão atual e, no detalhe, o histórico. */
// ------------------------------------------ validação (ADR 028)

export type SituacaoValidacao = "RASCUNHO" | "EM_VALIDACAO" | "HOMOLOGADO";

export interface ValidacaoAtlas {
  id: UUID;
  codigo_processual: string;
  realizada_em: string;
  unidade: string;
  participantes: string;
  registro: string;
  pendencias: string;
  created_at: string;
  created_by: string;
}

// ------------------------------------- importação e cobertura (ADR 025)

export type SituacaoImportacao = "NOVO" | "NOVA_VERSAO" | "INALTERADO" | "ERRO";

export interface ItemImportacao {
  linha: number;
  codigo_processual: string;
  titulo: string;
  situacao: SituacaoImportacao;
  versao: number;
  erro?: string;
}

export interface ImportacaoProcedimentos {
  hash: string;
  aplicada: boolean;
  totais: Partial<Record<SituacaoImportacao, number>>;
  itens: ItemImportacao[];
}

export interface CoberturaAtlas {
  totais: {
    series: number;
    series_com_procedimento: number;
    series_com_modelo: number;
    procedimentos: number;
    pecas: number;
    pecas_com_modelo: number;
    modelos: number;
    modelos_sem_uso: number;
    rascunhos: number;
    em_validacao: number;
  };
  orgaos: {
    prefixo: string;
    nome: string;
    series: number;
    series_com_procedimento: number;
    series_com_modelo: number;
  }[];
  pecas_sem_modelo: {
    workflow_id: UUID;
    codigo_processual: string;
    titulo: string;
    etapa: number;
    peca: string;
  }[];
  modelos_sem_uso: { id: UUID; nome: string }[];
  /** Subfunções da TTDD com séries de processo e nenhum procedimento (ADR 028). */
  lacunas: { codigo: string; nome: string; orgao: string; series: number; exemplos: string[] }[];
}

export interface ModeloDocumento {
  id: UUID;
  nome: string;
  descricao: string;
  ativo: boolean;
  atual: ModeloVersao;
  versoes?: ModeloVersao[];
  updated_at: string;
  updated_by: string;
  pecas_ligadas: number;
}

export interface EtapaTransicao {
  id: UUID;
  destino_ordem: number;
  condicao_transicao: string;
  is_devolucao_diligencia: boolean;
  descricao_diligencia: string;
}

export interface Etapa {
  id: UUID;
  ordem: number;
  unidade_administrativa: string;
  nome_setor: string;
  atribuicoes_setor: string;
  prazo_sla_em_dias: number;
  manter_aberto_apos_remessa: boolean;
  documentos: EtapaDocumento[];
  transicoes: EtapaTransicao[];
}

/** Procedimento canônico. Na listagem `etapas` vem vazio (use total_etapas);
 * o detalhe traz o percurso completo. */
export interface Workflow {
  id: UUID;
  codigo_processual: string;
  titulo: string;
  objetivo: string;
  publico_alvo: string;
  versao: number;
  ativo: boolean;
  /** Validação (ADR 028): só o homologado é publicado. */
  situacao: SituacaoValidacao;
  nivel_acesso: NivelAcesso;
  hipotese_legal_restricao: string;
  codigo_ttdd: string;
  classificacao?: ClassificacaoTTDD;
  total_etapas: number;
  etapas: Etapa[];
  created_by?: UUID | null;
  created_at: string;
  updated_at: string;
}

/** Fonte da resposta: um fluxo homologado (procedimento, com id) ou uma
 * série da TTDD — o assistente só trata do acervo do Atlas (ADRs 021/023). */
export interface AtlasFonte {
  tipo: "procedimento" | "ttdd";
  id?: UUID;
  codigo: string;
  titulo: string;
  relevancia: number;
}

export interface AtlasResposta {
  answer: string;
  score: number;
  refused: boolean;
  mode: "ia" | "sintese" | "recusada";
  sources: AtlasFonte[];
  generated_at: string;
}

// -------------------------------------------------------------- ia (ADR 020)

export type ProvedorIA =
  | "ollama"
  | "openai"
  | "azure"
  | "gemini"
  | "anthropic"
  | "groq"
  | "openrouter"
  | "mistral"
  | "compativel";

/** Fornecedor do catálogo (pré-preenche a conexão). */
export interface ModeloProvedorIA {
  provedor: ProvedorIA;
  nome: string;
  endpoint: string;
  modelo_sugerido: string;
  externo: boolean;
  exige_chave: boolean;
}

export interface TesteIA {
  ok: boolean;
  em: string;
  latencia_ms: number;
  erro: string;
}

/** Conexão de IA como a API devolve: a chave nunca vem, só o final dela. */
export interface ConexaoIA {
  id: UUID;
  nome: string;
  provedor: ProvedorIA;
  endpoint: string;
  modelo: string;
  externo: boolean;
  timeout_segundos: number;
  tem_chave: boolean;
  chave_final: string;
  ultimo_teste: TesteIA | null;
  updated_at: string;
  updated_by: string;
}

/** Uso de uma função (ex.: assistente do Atlas). configurado=false: vale o
 * ambiente do servidor; principal_id nulo com configurado: IA desligada. */
export interface UsoIA {
  funcao: string;
  configurado: boolean;
  principal_id: UUID | null;
  reserva_id: UUID | null;
  mascarar_dados_pessoais: boolean;
  externo_autorizado_por: string;
  externo_autorizado_em: string | null;
  updated_at: string | null;
  updated_by: string;
  ambiente: ConexaoIA | null;
}

/** Valores de uma série num lado da comparação (antes/depois da carga). */
export type PrazosTTDD = HistoricoTTDD["anterior"];

export type SituacaoCarga = "NOVA" | "ALTERADA" | "REVOGADA" | "RESTABELECIDA" | "INALTERADA";

/** Impacto de uma nova TTDD (ADR 022): o que muda e quem é afetado. */
export interface ImpactoCargaTTDD {
  hash: string;
  aplicada: boolean;
  orgaos: OrgaoTTDD[];
  totais: Partial<Record<SituacaoCarga, number>>;
  series: {
    codigo: string;
    descritor: string;
    situacao: SituacaoCarga;
    antes: PrazosTTDD | null;
    depois: PrazosTTDD | null;
  }[];
  procedimentos: {
    id: UUID;
    codigo_processual: string;
    versao: number;
    ativo: boolean;
    codigo_ttdd: string;
    situacao: SituacaoCarga;
  }[];
}
