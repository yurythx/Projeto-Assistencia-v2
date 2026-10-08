import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import type { EstruturaTTDD, ProcedimentosOrgao, Workflow } from "@/lib/nexus/types";

/** Resumo dos procedimentos de uma secretaria: publicados e, para a gestão,
 * os que estão em validação ou em rascunho. */
export function resumoProcedimentos(o: ProcedimentosOrgao, gestao: boolean): string {
  const partes = [`${o.publicados} ${o.publicados === 1 ? "publicado" : "publicados"}`];
  if (gestao && o.em_validacao) partes.push(`${o.em_validacao} em validação`);
  if (gestao && o.rascunhos)
    partes.push(`${o.rascunhos} ${o.rascunhos === 1 ? "rascunho" : "rascunhos"}`);
  return partes.join(" · ");
}

/** Agrupa os procedimentos pela função da TTDD (o departamento, em geral),
 * na ordem dos códigos. Sem classificação, ficam em "Outros". */
export function porFuncao(
  items: Workflow[],
): { codigo: string; nome: string; itens: Workflow[] }[] {
  const grupos = new Map<string, { codigo: string; nome: string; itens: Workflow[] }>();
  for (const wf of items) {
    const f = wf.classificacao?.subfuncao?.funcao;
    const chave = f?.codigo ?? "";
    const g = grupos.get(chave) ?? { codigo: chave, nome: f?.nome ?? "Outros", itens: [] };
    g.itens.push(wf);
    grupos.set(chave, g);
  }
  const ordem = (c: string) =>
    c
      ? c
          .split(".")
          .map((n) => n.padStart(3, "0"))
          .join(".")
      : "~";
  // Comparação simples (não localeCompare): "~" fica depois dos dígitos.
  return [...grupos.values()].sort((a, b) => (ordem(a.codigo) < ordem(b.codigo) ? -1 : 1));
}

export const SITUACOES = [
  { value: "", label: "Todas as situações" },
  { value: "RASCUNHO", label: "Rascunho" },
  { value: "EM_VALIDACAO", label: "Em validação" },
  { value: "HOMOLOGADO", label: "Homologado" },
];

/** Até 100 por página: uma secretaria cabe numa página, agrupada por função. */
const POR_PAGINA = 100;

/** Lista dos procedimentos da secretaria. A gestão vê a versão mais recente
 * de cada código (onde está o rascunho em revisão) e pode filtrar pela
 * situação; o público vê só os publicados. */
export function useProcedimentosDaSecretaria(
  prefixo: string,
  gestao: boolean,
  situacao: string,
  funcao = "",
) {
  // Com a função (ex.: 2.0.06), a API filtra pelo prefixo dela.
  const alvo = funcao || prefixo;
  return useApiPage<Workflow>(
    gestao
      ? withQuery("v1/atlas/admin/workflows", {
          prefixo_ttdd: alvo,
          ultima: true,
          situacao,
          page_size: POR_PAGINA,
        })
      : withQuery("v1/atlas/workflows", { prefixo_ttdd: alvo, page_size: POR_PAGINA }),
  );
}

/** Funções da TTDD da secretaria (em geral, os departamentos): o filtro que
 * divide a secretaria em entrevistas menores. */
export function useFuncoes(prefixo: string): { codigo: string; nome: string }[] {
  const estrutura = useApiQuery<EstruturaTTDD[]>("v1/atlas/ttdd/estrutura");
  return (estrutura.data?.find((o) => o.prefixo === prefixo)?.funcoes ?? []).map((f) => ({
    codigo: f.codigo,
    nome: f.nome,
  }));
}

/** A função da URL só vale se for desta secretaria. */
export function funcaoDaSecretaria(prefixo: string, funcao: string | null): string {
  return funcao?.startsWith(`${prefixo}.`) ? funcao : "";
}
