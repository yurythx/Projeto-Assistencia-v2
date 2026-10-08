"use client";

import { Network, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";

import { ScopePicker } from "@/components/iam/ScopePicker";
import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/Table";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import type { ADMapping, Perfil, ScopeRef } from "@/lib/nexus/types";

/**
 * Correlação grupo do AD → Perfil + escopo organizacional (skill §2.3).
 * Na autenticação, os grupos que chegam no token (federação LDAP do
 * Keycloak) são resolvidos para lotações de origem "ad" pelo IAM.
 */
export default function MapeamentoADPage() {
  const mappings = useApiQuery<ADMapping[]>("v1/iam/ad-mappings");
  const perfis = useApiQuery<Perfil[]>("v1/iam/perfis");
  const { run, pending } = useAction();
  const [scope, setScope] = useState<ScopeRef>({});
  const [formKey, setFormKey] = useState(0);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const ok = await run(
      () =>
        apiClient.post("v1/iam/ad-mappings", {
          ad_group: String(fd.get("ad_group") ?? "").trim(),
          perfil_id: String(fd.get("perfil_id") ?? ""),
          descricao: String(fd.get("descricao") ?? "").trim(),
          ...scope,
        }),
      "Mapeamento criado",
    );
    if (ok) {
      setScope({});
      setFormKey((k) => k + 1);
      void mappings.mutate();
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle as="h2" className="flex items-center gap-2">
            <Network size={18} aria-hidden="true" /> Novo mapeamento
          </CardTitle>
        </CardHeader>
        <CardContent className="pb-5">
          <form key={formKey} onSubmit={submit} className="mt-3 flex flex-col gap-3">
            <div className="grid gap-3 sm:grid-cols-2">
              <Input id="ad-group" name="ad_group" label="Grupo do AD *" required maxLength={200} placeholder="GRP_RH ou CN=GRP_RH,OU=Grupos,DC=org" />
              <Select
                id="ad-perfil"
                name="perfil_id"
                label="Perfil *"
                required
                placeholder="Selecione…"
                options={(perfis.data ?? []).filter((p) => p.ativo).map((p) => ({ value: p.id, label: p.nome }))}
              />
            </div>
            <ScopePicker idPrefix="ad-scope" value={scope} onChange={setScope} />
            <Input id="ad-descricao" name="descricao" label="Descrição" maxLength={300} />
            <div className="flex justify-end">
              <Button type="submit" loading={pending}>
                Adicionar mapeamento
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <DataState
        loading={mappings.isLoading}
        error={mappings.error}
        onRetry={() => void mappings.mutate()}
        empty={(mappings.data ?? []).length === 0}
        emptyTitle="Nenhum grupo mapeado"
        emptyDescription="Sem mapeamentos, usuários federados só têm as lotações manuais."
      >
        <Table caption="Mapeamentos de grupos do Active Directory">
          <TableHead>
            <TableRow>
              <TableHeaderCell>Grupo do AD</TableHeaderCell>
              <TableHeaderCell>Perfil</TableHeaderCell>
              <TableHeaderCell>Escopo</TableHeaderCell>
              <TableHeaderCell>Criado</TableHeaderCell>
              <TableHeaderCell>
                <span className="sr-only">Ações</span>
              </TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(mappings.data ?? []).map((m) => (
              <TableRow key={m.id}>
                <TableCell>
                  <code className="font-mono text-xs">{m.ad_group}</code>
                  {m.descricao && <span className="block text-xs text-muted">{m.descricao}</span>}
                </TableCell>
                <TableCell>{m.perfil_nome}</TableCell>
                <TableCell className="text-sm">{m.scope_label || "Global"}</TableCell>
                <TableCell className="text-xs text-muted">
                  {fmtDateTime(m.created_at)}
                  {m.created_by && <span className="block">por {m.created_by}</span>}
                </TableCell>
                <TableCell className="text-right">
                  <ConfirmButton
                    aria-label={`Remover mapeamento ${m.ad_group}`}
                    title="Remover mapeamento?"
                    description={`Membros de ${m.ad_group} perdem o perfil ${m.perfil_nome} no próximo login.`}
                    confirmLabel="Remover"
                    onConfirm={() => run(() => apiClient.delete(`v1/iam/ad-mappings/${m.id}`), "Mapeamento removido").then(() => mappings.mutate())}
                  >
                    <Trash2 size={14} aria-hidden="true" />
                  </ConfirmButton>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </DataState>
    </div>
  );
}
