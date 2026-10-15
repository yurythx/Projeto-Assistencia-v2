"use client";

import Link from "next/link";
import { ListPlus, Search, UserPlus } from "lucide-react";
import { useState, type FormEvent } from "react";

import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { apiClient } from "@/lib/api/client";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { CAMPOS_CIDADAO, P, PRIORIDADES, TIPOS, type Cidadao, type Resumo } from "@/lib/sempras/types";

/** Recepção (ADR 030 §4): busca por CPF, NIS ou nome; cadastro simples
 * quando a pessoa ainda não existe; inclusão na fila da unidade. */
export function Recepcao({ unidadeId, onIncluido }: { unidadeId: string; onIncluido: () => void }) {
  const { can } = useNexus();
  const { run, pending } = useAction();
  const [q, setQ] = useState("");
  const [achados, setAchados] = useState<Resumo[] | null>(null);
  const [cadastro, setCadastro] = useState(false);
  const [tipo, setTipo] = useState("social");
  const [prioridade, setPrioridade] = useState("");

  async function buscar(e: FormEvent) {
    e.preventDefault();
    const out = await run(() => apiClient.get<Resumo[]>(`v1/sempras/cidadaos?q=${encodeURIComponent(q.trim())}`));
    if (out) setAchados(out.data);
  }

  async function cadastrar(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const body = {
      ...Object.fromEntries(CAMPOS_CIDADAO.map((c) => [c.name, String(fd.get(c.name) ?? "").trim()])),
      anonimo: fd.get("anonimo") === "on",
    };
    const out = await run(() => apiClient.post<Cidadao>("v1/sempras/cidadaos", body), "Munícipe cadastrado");
    if (out) {
      const c = out.data;
      setAchados([{ ...c, atendimentos_na_area: 0 }]);
      setCadastro(false);
    }
  }

  async function incluir(cidadao: Resumo) {
    const out = await run(
      () => apiClient.post(`v1/sempras/unidades/${unidadeId}/fila`, { cidadao_id: cidadao.id, tipo, prioridade }),
      "Incluído na fila",
    );
    if (out) onIncluido();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Recepção</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pb-5">
        <form onSubmit={buscar} className="mt-3 grid gap-3 sm:grid-cols-[1fr_auto_auto] sm:items-end">
          <Input
            id="sempras-busca"
            label="CPF, NIS ou nome"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            maxLength={200}
            autoComplete="off"
            required
          />
          <Button type="submit" loading={pending}>
            <Search size={16} aria-hidden="true" className="mr-1" /> Buscar
          </Button>
          <Button type="button" variant="secondary" onClick={() => setCadastro(true)}>
            <UserPlus size={16} aria-hidden="true" className="mr-1" /> Cadastrar
          </Button>
        </form>

        <div className="grid gap-3 sm:grid-cols-2">
          <Select id="sempras-tipo" label="Tipo de atendimento" options={TIPOS} value={tipo} onChange={(e) => setTipo(e.target.value)} />
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
            <p className="text-sm text-muted">Ninguém encontrado. Confira o documento ou cadastre o munícipe.</p>
          ) : (
            <ul aria-label="Munícipes encontrados" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
              {achados.map((c) => (
                <li key={c.id} className="flex flex-wrap items-center justify-between gap-3 p-3">
                  <div>
                    <p className="font-medium">{c.nome_social || c.nome}</p>
                    <p className="text-xs text-muted">
                      {[c.cpf && `CPF ${c.cpf}`, c.nis && `NIS ${c.nis}`, c.data_nascimento].filter(Boolean).join(" · ")}
                    </p>
                  </div>
                  <span className="flex items-center gap-2">
                    <Badge tone={c.atendimentos_na_area > 0 ? "info" : "neutral"}>
                      {c.atendimentos_na_area > 0 ? `${c.atendimentos_na_area} atendimento(s) na área` : "Sem histórico na área"}
                    </Badge>
                    {can(P.atendimento) && (
                      <Link href={`/sempras/cidadaos/${c.id}`} className="text-sm text-primary underline">
                        Ficha
                      </Link>
                    )}
                    <Button size="sm" onClick={() => void incluir(c)} disabled={pending} aria-label={`Incluir ${c.nome_social || c.nome} na fila`}>
                      <ListPlus size={14} aria-hidden="true" className="mr-1" /> Incluir na fila
                    </Button>
                  </span>
                </li>
              ))}
            </ul>
          ))}
      </CardContent>

      <Dialog open={cadastro} onClose={() => setCadastro(false)} title="Cadastrar munícipe" size="lg"
        description="Informe CPF, NIS ou, na falta deles, data de nascimento e nome da mãe.">
        <form onSubmit={cadastrar} className="grid gap-3 sm:grid-cols-2">
          {CAMPOS_CIDADAO.map((c) => (
            <Input key={c.name} id={`cad-${c.name}`} name={c.name} label={c.label} type={c.type} maxLength={c.max}
              required={c.name === "nome"} autoComplete="off" />
          ))}
          <label className="flex items-center gap-2 text-sm sm:col-span-2">
            <input type="checkbox" name="anonimo" />
            Não sabe o CPF/documento (Centro POP — cadastro anônimo)
          </label>
          <div className="sm:col-span-2">
            <Button type="submit" loading={pending}>
              Salvar cadastro
            </Button>
          </div>
        </form>
      </Dialog>
    </Card>
  );
}
