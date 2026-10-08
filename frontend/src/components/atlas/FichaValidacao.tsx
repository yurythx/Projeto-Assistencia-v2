import { useId } from "react";

import { ASSINATURA, FORMATO, NIVEL_ACESSO } from "@/components/atlas/labels";
import { SITUACAO } from "@/components/atlas/ValidacaoProcedimento";
import type { Workflow } from "@/lib/nexus/types";

/** Linha em branco para preencher à mão. */
function Linha({ rotulo }: { rotulo: string }) {
  return (
    <p className="flex items-end gap-2 text-sm">
      <span className="shrink-0">{rotulo}</span>
      <span
        className="mb-1 flex-1 border-b border-dotted border-foreground/60"
        aria-hidden="true"
      />
    </p>
  );
}

/** Caixa "confere / corrigir" ao lado de um item proposto. */
function Confere() {
  return (
    <span className="whitespace-nowrap text-xs" aria-hidden="true">
      ☐ confere&nbsp;&nbsp;☐ corrigir
    </span>
  );
}

const celula = "border border-foreground/30 px-2 py-1 align-top";

/** Ficha de validação de um procedimento (ADR 028): o fluxo proposto, item a
 * item, com espaço para confirmar ou corrigir e para padronizar cada
 * documento. Usada sozinha e no caderno da secretaria. */
export function FichaValidacao({ wf, aninhada = false }: { wf: Workflow; aninhada?: boolean }) {
  // No caderno, várias fichas na mesma página: ids únicos e um nível abaixo
  // do título do caderno.
  const uid = useId();
  const H = aninhada ? "h2" : "h1";
  const Hs = aninhada ? "h3" : "h2";
  return (
    <article className="flex flex-col gap-5">
      <header className="border-b-2 border-foreground pb-2">
        <p className="text-xs uppercase tracking-wide">Ficha de validação do procedimento</p>
        <H className="text-xl font-bold">
          {wf.codigo_processual} — {wf.titulo}
        </H>
        <p className="text-xs">
          Versão {wf.versao} · situação: {SITUACAO[wf.situacao ?? "HOMOLOGADO"].label.toLowerCase()}{" "}
          · série TTDD {wf.codigo_ttdd}
          {wf.classificacao && ` (${wf.classificacao.descritor})`}
        </p>
      </header>

      <section aria-labelledby={`${uid}-entrevista`} className="flex flex-col gap-2">
        <Hs id={`${uid}-entrevista`} className="font-semibold">
          Entrevista
        </Hs>
        <div className="grid gap-x-6 gap-y-2 sm:grid-cols-2">
          <Linha rotulo="Departamento:" />
          <Linha rotulo="Data:" />
          <Linha rotulo="Participantes (funções):" />
          <Linha rotulo="Entrevistador:" />
        </div>
      </section>

      <section aria-labelledby={`${uid}-processo`} className="flex flex-col gap-2">
        <Hs id={`${uid}-processo`} className="font-semibold">
          1. O processo
        </Hs>
        <p className="text-sm">
          <strong>Objetivo proposto:</strong> {wf.objetivo} <Confere />
        </p>
        <p className="text-sm">
          <strong>Público:</strong> {wf.publico_alvo} <Confere />
        </p>
        <p className="text-sm">
          <strong>Nível de acesso:</strong> {NIVEL_ACESSO[wf.nivel_acesso].label}
          {wf.hipotese_legal_restricao && ` (${wf.hipotese_legal_restricao})`} <Confere />
        </p>
        <Linha rotulo="Base legal (lei, decreto, IN):" />
        <Linha rotulo="Canal de entrada (balcão, sistema, e-mail):" />
      </section>

      {wf.etapas.map((e) => {
        const segue = e.transicoes.filter((t) => !t.is_devolucao_diligencia);
        const volta = e.transicoes.filter((t) => t.is_devolucao_diligencia);
        return (
          <section
            key={e.id}
            aria-labelledby={`${uid}-etapa-${e.ordem}`}
            className="flex flex-col gap-2 break-inside-avoid"
          >
            <Hs id={`${uid}-etapa-${e.ordem}`} className="font-semibold">
              {e.ordem + 1}. Etapa {e.ordem}: {e.nome_setor}
            </Hs>
            <table className="w-full border-collapse text-sm">
              <tbody>
                <tr>
                  <th scope="row" className={`${celula} w-40 text-left`}>
                    Setor / sigla
                  </th>
                  <td className={celula}>
                    {e.nome_setor} ({e.unidade_administrativa}) <Confere />
                    <br />
                    <span className="text-xs">Sigla oficial: ________________</span>
                  </td>
                </tr>
                <tr>
                  <th scope="row" className={`${celula} text-left`}>
                    O que faz
                  </th>
                  <td className={celula}>
                    {e.atribuicoes_setor} <Confere />
                  </td>
                </tr>
                <tr>
                  <th scope="row" className={`${celula} text-left`}>
                    Prazo
                  </th>
                  <td className={celula}>
                    {e.prazo_sla_em_dias} {e.prazo_sla_em_dias === 1 ? "dia" : "dias"} <Confere /> ·
                    prazo legal: ________
                  </td>
                </tr>
                <tr>
                  <th scope="row" className={`${celula} text-left`}>
                    Para seguir
                  </th>
                  <td className={celula}>
                    {segue.length
                      ? segue
                          .map((t) => `${t.condicao_transicao} → etapa ${t.destino_ordem}`)
                          .join("; ")
                      : "Fim do fluxo"}{" "}
                    <Confere />
                  </td>
                </tr>
                <tr>
                  <th scope="row" className={`${celula} text-left`}>
                    Quando volta
                  </th>
                  <td className={celula}>
                    {volta.length
                      ? volta
                          .map(
                            (t) =>
                              `${t.condicao_transicao} → etapa ${t.destino_ordem} (${t.descricao_diligencia})`,
                          )
                          .join("; ")
                      : "—"}{" "}
                    <Confere />
                  </td>
                </tr>
              </tbody>
            </table>
            {e.documentos.length > 0 && (
              <table className="w-full border-collapse text-xs">
                <caption className="py-1 text-left text-sm font-medium">
                  Documentos da etapa {e.ordem}
                </caption>
                <thead>
                  <tr>
                    <th scope="col" className={celula}>
                      Documento proposto
                    </th>
                    <th scope="col" className={celula}>
                      Obrig.
                    </th>
                    <th scope="col" className={celula}>
                      Formato
                    </th>
                    <th scope="col" className={celula}>
                      Assinatura
                    </th>
                    <th scope="col" className={celula}>
                      Nome oficial
                    </th>
                    <th scope="col" className={celula}>
                      Tem modelo?
                    </th>
                    <th scope="col" className={celula}>
                      Campos mínimos
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {e.documentos.map((d) => (
                    <tr key={d.id}>
                      <td className={celula}>{d.nome_documento}</td>
                      <td className={celula}>{d.obrigatorio ? "sim" : "não"}</td>
                      <td className={celula}>{FORMATO[d.formato]}</td>
                      <td className={celula}>{ASSINATURA[d.tipo_assinatura]}</td>
                      <td className={`${celula} w-32`} />
                      <td className={celula}>
                        {d.modelo ? `sim (${d.modelo.nome})` : "☐ sim ☐ não"}
                      </td>
                      <td className={`${celula} w-40`} />
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            <Linha rotulo="Correções da etapa:" />
          </section>
        );
      })}

      <section aria-labelledby={`${uid}-fim`} className="flex flex-col gap-2 break-inside-avoid">
        <Hs id={`${uid}-fim`} className="font-semibold">
          {wf.etapas.length + 2}. Fechamento
        </Hs>
        <Linha rotulo="Documentos que faltam no fluxo:" />
        <Linha rotulo="Documentos que podem sair:" />
        <Linha rotulo="Pendências:" />
        <Linha rotulo="Quem valida a versão final:" />
        <div className="mt-6 grid gap-8 sm:grid-cols-2">
          <Linha rotulo="Chefia do departamento:" />
          <Linha rotulo="Gestão documental:" />
        </div>
      </section>
    </article>
  );
}
