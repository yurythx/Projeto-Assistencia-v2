// Contrato frontend × backend verificado pelo compilador (tsc --noEmit, no
// lint do CI). Cada tipo escrito à mão em types.ts precisa aceitar o que o
// backend de fato envia: os schemas de src/types/openapi.gen.ts saem do
// OpenAPI, que por sua vez sai dos tipos Go (ADR 010 §10.5). Se o backend
// deixar de enviar um campo que a tela lê, mudar um tipo ou passar a omitir
// um valor (omitempty), a linha correspondente deixa de compilar.
//
// Regenerar depois de mudar o OpenAPI: npm run gen:api
import type { components } from "@/types/openapi.gen";

import type * as T from "./types";

type S = components["schemas"];

/**
 * A tela costuma estreitar um string do backend para os valores que conhece
 * ("noticia" | "comunicado"); quem garante esses valores é o domínio no
 * backend, não o tipo Go. A comparação é de forma: literais viram string.
 */
type Widen<T> = T extends string
  ? string
  : T extends number
    ? number
    : T extends boolean
      ? boolean
      : T extends (infer U)[]
        ? Widen<U>[]
        : T extends object
          ? { [K in keyof T]: Widen<T[K]> }
          : T;

/** Compila só se o que o backend envia (Back) cabe no tipo da tela (Front). */
type Accepts<Front, Back extends Widen<Front>> = [Front, Back];

export type Contract = [
  Accepts<T.PageMeta, S["PaginationMeta"]>,
  Accepts<T.Me, S["IamMeResponse"]>,
  Accepts<T.PermissionInfo, S["KernelPermissionInfo"]>,
  Accepts<T.ModuleStatus, S["KernelModuleStatus"]>,
  Accepts<T.PublicModule, S["KernelPublicModule"]>,
  Accepts<T.Branding, S["BrandingSettings"]>,
  Accepts<T.Entidade, S["IamEntidade"]>,
  Accepts<T.Unidade, S["IamUnidade"]>,
  Accepts<T.Departamento, S["IamDepartamento"]>,
  Accepts<T.OrgTree, S["IamOrgTree"]>,
  Accepts<T.Perfil, S["IamPerfil"]>,
  Accepts<T.ADMapping, S["IamADMapping"]>,
  Accepts<T.Lotacao, S["IamLotacao"]>,
  Accepts<T.UserAdmin, S["IamUser"]>,
  Accepts<T.UserAdmin, S["IamUserDetail"]>,
  Accepts<T.PermissionGroup, S["IamPermissionGroup"]>,
  Accepts<T.AuditRecord, S["AuditRecord"]>,
  Accepts<T.VerifyResult, S["AuditVerifyResult"]>,
  Accepts<T.Post, S["BlogPost"]>,
  Accepts<T.UploadTicket, S["ModkitUploadTicket"]>,
  Accepts<T.ServiceChannel, S["CatalogChannel"]>,
  Accepts<T.CatalogService, S["CatalogService"]>,
  Accepts<T.CatalogCategory, S["CatalogCategory"]>,
  Accepts<T.ContactSummary, S["ContactSummary"]>,
  Accepts<T.ContactMessage, S["ContactMessage"]>,
  Accepts<T.Person, S["DirectoryPerson"]>,
  Accepts<T.Sector, S["DirectorySector"]>,
  Accepts<T.Room, S["CalendarRoom"]>,
  Accepts<T.CalendarEvent, S["CalendarEvent"]>,
  Accepts<T.Folder, S["FilesFolder"]>,
  Accepts<T.FileItem, S["FilesFile"]>,
  Accepts<T.FolderAccess, S["FilesAccess"]>,
  Accepts<T.Listing, S["FilesListing"]>,
  Accepts<T.ACLEntry, S["FilesACLEntry"]>,
  Accepts<T.WikiPage, S["WikiPage"]>,
  Accepts<T.WikiRevision, S["WikiRevision"]>,
  Accepts<T.SearchResult, S["SearchResult"]>,
  Accepts<T.SearchResponse, S["BuscaResponse"]>,
  Accepts<T.Signer, S["SignumSigner"]>,
  Accepts<T.Envelope, S["SignumEnvelope"]>,
  Accepts<T.Challenge, S["SignumChallengeResponse"]>,
  Accepts<T.Verification, S["SignumVerification"]>,
  Accepts<T.TramiteTipo, S["TramiteTipo"]>,
  Accepts<T.Processo, S["TramiteProcesso"]>,
  Accepts<T.Documento, S["TramiteDocumento"]>,
  Accepts<T.Movimento, S["TramiteMovimento"]>,
  Accepts<T.ProcessoView, S["TramiteView"]>,
  Accepts<T.ChatRoom, S["MercurioRoom"]>,
  Accepts<T.ChatMessage, S["MercurioMessage"]>,
  Accepts<T.EgressTarget, S["EgressTarget"]>,
  Accepts<T.EgressDelivery, S["EgressDelivery"]>,
];
