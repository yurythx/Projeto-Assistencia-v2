import { apiClient } from "@/lib/api/client";
import { mimeOf, putToTicket } from "@/lib/nexus/upload";
import type { UploadTicket } from "@/lib/nexus/types";
import type { DocumentoCasaAbrigo, TipoDocumentoCasaAbrigo } from "@/lib/sempras/types";

/** Envia um PDF do acolhido ou do responsável: pede o ticket, sobe direto
 * ao armazenamento e confirma (o conteúdo nunca passa pela API). */
export async function enviarDocumento(acolhimentoId: string, tipo: TipoDocumentoCasaAbrigo, file: File): Promise<DocumentoCasaAbrigo> {
  const ticket = await apiClient.post<{ upload: UploadTicket; documento: DocumentoCasaAbrigo }>(
    `v1/sempras/casa-abrigo/${acolhimentoId}/documentos/uploads`,
    { tipo, filename: file.name, content_type: mimeOf(file), size: file.size },
  );
  await putToTicket(ticket.data.upload, file);
  const confirmado = await apiClient.post<DocumentoCasaAbrigo>(`v1/sempras/casa-abrigo-documentos/${ticket.data.documento.id}/confirm`);
  return confirmado.data;
}

/** Abre o PDF numa aba nova pela URL pré-assinada (curta). */
export async function abrirDocumento(id: string): Promise<void> {
  const res = await apiClient.get<{ url: string }>(`v1/sempras/casa-abrigo-documentos/${id}/download`);
  window.open(res.data.url, "_blank", "noopener,noreferrer");
}
