"use server";

import { redirect } from "next/navigation";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Server action do formulário de verificação: código válido abre a
 * página do envelope; qualquer outra coisa volta com o aviso de erro. */
export async function verificar(formData: FormData) {
  const codigo = String(formData.get("codigo") ?? "").trim();
  if (UUID_RE.test(codigo)) redirect(`/verificar/${codigo.toLowerCase()}`);
  redirect("/verificar?erro=1");
}
