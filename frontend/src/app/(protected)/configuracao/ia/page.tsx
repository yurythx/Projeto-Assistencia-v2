"use client";

import {
  Bot,
  CheckCircle2,
  Globe,
  Pencil,
  Plus,
  Server,
  Send,
  ShieldAlert,
  Trash2,
  XCircle,
} from "lucide-react";
import { useState, type FormEvent } from "react";

import { useToast } from "@/components/notifications/ToastProvider";
import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import type { ConexaoIA, ModeloProvedorIA, ProvedorIA, TesteIA, UsoIA } from "@/lib/nexus/types";

const USO = "v1/ia/uso/atlas.assistente";

/** Último teste da conexão: resultado, tempo de resposta e quando. */
function UltimoTeste({ t }: { t: TesteIA | null }) {
  if (!t) return <span className="text-xs text-muted">Nunca testada</span>;
  return (
    <span className={`flex items-start gap-1 text-xs ${t.ok ? "text-success" : "text-danger"}`}>
      {t.ok ? (
        <CheckCircle2 size={14} aria-hidden="true" className="mt-0.5 shrink-0" />
      ) : (
        <XCircle size={14} aria-hidden="true" className="mt-0.5 shrink-0" />
      )}
      <span>
        {t.ok ? `Respondeu em ${(t.latencia_ms / 1000).toFixed(1)} s` : `Falhou: ${t.erro}`}
        <span className="text-muted"> · {fmtDateTime(t.em)}</span>
      </span>
    </span>
  );
}

function ConexaoForm({
  conexao,
  provedores,
  onDone,
}: {
  conexao?: ConexaoIA;
  provedores: ModeloProvedorIA[];
  onDone: () => void;
}) {
  const { run, pending } = useAction();
  const [provedor, setProvedor] = useState<ProvedorIA>(conexao?.provedor ?? "ollama");
  const modelo = provedores.find((p) => p.provedor === provedor);
  const [endpoint, setEndpoint] = useState(conexao?.endpoint ?? modelo?.endpoint ?? "");
  const [remover, setRemover] = useState(false);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const body = {
      nome: String(fd.get("nome") ?? "").trim(),
      provedor,
      endpoint: endpoint.trim(),
      modelo: String(fd.get("modelo") ?? "").trim(),
      // Em branco = manter a chave salva (a tela nunca a recebe de volta).
      chave: String(fd.get("chave") ?? ""),
      externo: provedor === "compativel" ? fd.get("externo") === "on" : (modelo?.externo ?? false),
      timeout_segundos: Number(fd.get("timeout") || 30),
      remover_chave: remover,
    };
    const ok = await run(
      () =>
        conexao
          ? apiClient.put(`v1/ia/conexoes/${conexao.id}`, body)
          : apiClient.post("v1/ia/conexoes", body),
      "Conexão testada e salva",
    );
    if (ok) onDone();
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <p className="text-sm text-muted">
        Ao salvar, a conexão é testada com uma conversa real; se o fornecedor não responder, nada é
        gravado.
      </p>
      <div className="grid gap-3 sm:grid-cols-2">
        <Select
          id="ia-provedor"
          label="Fornecedor *"
          value={provedor}
          onChange={(e) => {
            const p = e.target.value as ProvedorIA;
            setProvedor(p);
            setEndpoint(provedores.find((x) => x.provedor === p)?.endpoint ?? "");
          }}
          options={provedores.map((p) => ({ value: p.provedor, label: p.nome }))}
        />
        <Input
          id="ia-nome"
          name="nome"
          label="Nome *"
          required
          maxLength={80}
          defaultValue={conexao?.nome ?? modelo?.nome}
        />
      </div>
      <Input
        id="ia-endpoint"
        label="Endereço da API *"
        required
        maxLength={500}
        value={endpoint}
        onChange={(e) => setEndpoint(e.target.value)}
        placeholder="https://"
      />
      <div className="grid gap-3 sm:grid-cols-[1fr_9rem]">
        <Input
          id="ia-modelo"
          name="modelo"
          label="Modelo *"
          required
          maxLength={120}
          defaultValue={conexao?.modelo}
          placeholder={modelo?.modelo_sugerido ? `Ex.: ${modelo.modelo_sugerido}` : undefined}
        />
        <Input
          id="ia-timeout"
          name="timeout"
          type="number"
          label="Tempo limite (s)"
          min={1}
          max={300}
          defaultValue={conexao?.timeout_segundos ?? (provedor === "ollama" ? 120 : 30)}
        />
      </div>
      <Input
        id="ia-chave"
        name="chave"
        type="password"
        autoComplete="new-password"
        maxLength={4096}
        label={
          conexao?.tem_chave
            ? `Chave de API (salva: ••••${conexao.chave_final}; deixe em branco para manter)`
            : `Chave de API${modelo?.exige_chave ? " *" : " (se o serviço exigir)"}`
        }
        required={!conexao?.tem_chave && !!modelo?.exige_chave}
        disabled={remover}
      />
      {conexao?.tem_chave && !modelo?.exige_chave && (
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={remover}
            onChange={(e) => setRemover(e.target.checked)}
            className="h-4 w-4 accent-primary"
          />
          Remover a chave salva
        </label>
      )}
      {provedor === "compativel" ? (
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            name="externo"
            defaultChecked={conexao?.externo ?? true}
            className="h-4 w-4 accent-primary"
          />
          Serviço fora da rede da instituição (os dados saem da rede)
        </label>
      ) : (
        modelo?.externo && (
          <p className="flex items-start gap-2 rounded-md bg-warning/10 px-3 py-2 text-xs text-foreground">
            <Globe size={14} aria-hidden="true" className="mt-0.5 shrink-0 text-warning" />
            Fornecedor externo: as perguntas saem da rede da instituição. Prefira contratos sem
            retenção de dados.
          </p>
        )
      )}
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Testar e salvar
        </Button>
      </div>
    </form>
  );
}

/** Qual conexão o assistente do Atlas usa: principal, reserva, LGPD. */
function UsoAtlas({
  uso,
  conexoes,
  onSaved,
}: {
  uso: UsoIA;
  conexoes: ConexaoIA[];
  onSaved: () => void;
}) {
  const { run, pending } = useAction();
  const [principal, setPrincipal] = useState(uso.configurado ? (uso.principal_id ?? "") : "");
  const [reserva, setReserva] = useState(uso.reserva_id ?? "");
  const [mascarar, setMascarar] = useState(uso.mascarar_dados_pessoais);
  const [autorizo, setAutorizo] = useState(uso.externo_autorizado_por !== "");
  const externas = conexoes.filter((c) => c.externo && (c.id === principal || c.id === reserva));

  async function salvar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(
      () =>
        apiClient.put(USO, {
          principal_id: principal || null,
          reserva_id: principal && reserva ? reserva : null,
          mascarar_dados_pessoais: mascarar,
          autorizo_envio_externo: autorizo,
        }),
      "Assistente do Atlas atualizado",
    );
    if (ok) onSaved();
  }

  return (
    <form onSubmit={salvar} className="flex flex-col gap-4">
      <p className="text-sm text-muted">
        {!uso.configurado
          ? uso.ambiente
            ? `Hoje: definido no servidor (${uso.ambiente.modelo} em ${uso.ambiente.endpoint}). Salvar aqui passa a valer no lugar dele.`
            : "Hoje: sem IA — o assistente responde com a síntese dos procedimentos e da TTDD."
          : uso.principal_id
            ? `Configurado por ${uso.updated_by || "—"}${uso.updated_at ? ` em ${fmtDateTime(uso.updated_at)}` : ""}.`
            : "IA desligada: o assistente responde com a síntese dos procedimentos e da TTDD."}{" "}
        Se a IA falhar, a resposta cai na reserva e, por fim, na síntese — nunca fica sem resposta.
      </p>
      <div className="grid gap-3 sm:grid-cols-2">
        <Select
          id="ia-principal"
          label="Conexão principal"
          value={principal}
          onChange={(e) => setPrincipal(e.target.value)}
          options={[
            { value: "", label: "Desligada (só síntese)" },
            ...conexoes.map((c) => ({ value: c.id, label: `${c.nome} — ${c.modelo}` })),
          ]}
        />
        <Select
          id="ia-reserva"
          label="Reserva (se a principal falhar)"
          value={principal ? reserva : ""}
          disabled={!principal}
          onChange={(e) => setReserva(e.target.value)}
          options={[
            { value: "", label: "Sem reserva" },
            ...conexoes
              .filter((c) => c.id !== principal)
              .map((c) => ({ value: c.id, label: `${c.nome} — ${c.modelo}` })),
          ]}
        />
      </div>
      <label className="flex items-start gap-2 text-sm">
        <input
          type="checkbox"
          checked={mascarar}
          onChange={(e) => setMascarar(e.target.checked)}
          className="mt-0.5 h-4 w-4 accent-primary"
        />
        <span>
          Mascarar CPF, CNPJ, e-mail e telefone da pergunta antes de enviá-la a um fornecedor
          externo
          <span className="block text-xs text-muted">
            O contexto enviado (procedimentos e TTDD) é público.
          </span>
        </span>
      </label>
      {externas.length > 0 && (
        <div className="rounded-md border border-warning/40 bg-warning/10 p-3 text-sm">
          <p className="flex items-start gap-2 text-foreground">
            <ShieldAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0 text-warning" />
            <span>
              <strong>{externas.map((c) => c.nome).join(" e ")}</strong>{" "}
              {externas.length > 1 ? "são fornecedores externos" : "é um fornecedor externo"}: as
              perguntas dos usuários saem da rede da instituição (pode haver transferência
              internacional de dados — LGPD, art. 33).
            </span>
          </p>
          <label className="mt-2 flex items-center gap-2">
            <input
              type="checkbox"
              checked={autorizo}
              onChange={(e) => setAutorizo(e.target.checked)}
              className="h-4 w-4 accent-primary"
            />
            Autorizo o envio das perguntas a este fornecedor
          </label>
          {uso.externo_autorizado_por && (
            <p className="mt-1 text-xs text-muted">
              Autorizado por {uso.externo_autorizado_por}
              {uso.externo_autorizado_em && ` em ${fmtDateTime(uso.externo_autorizado_em)}`}.
            </p>
          )}
        </div>
      )}
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Salvar
        </Button>
      </div>
    </form>
  );
}

/** Configurações > Inteligência artificial (ia:manage — ADR 020). */
export default function IAPage() {
  const conexoes = useApiQuery<ConexaoIA[]>("v1/ia/conexoes");
  const provedores = useApiQuery<ModeloProvedorIA[]>("v1/ia/provedores");
  const uso = useApiQuery<UsoIA>(USO);
  const [editando, setEditando] = useState<ConexaoIA | "nova" | null>(null);
  const { run } = useAction();
  const { showToast } = useToast();
  const lista = conexoes.data ?? [];
  const nomeProvedor = (p: ProvedorIA) => provedores.data?.find((x) => x.provedor === p)?.nome ?? p;

  async function testar(c: ConexaoIA) {
    const res = await run(() => apiClient.post<TesteIA>(`v1/ia/conexoes/${c.id}/testar`));
    if (!res) return;
    showToast(
      res.data.ok
        ? {
            title: `${c.nome} respondeu`,
            description: `em ${(res.data.latencia_ms / 1000).toFixed(1)} s`,
            tone: "success",
          }
        : { title: `${c.nome} não respondeu`, description: res.data.erro, tone: "danger" },
    );
    void conexoes.mutate();
  }

  async function excluir(c: ConexaoIA) {
    if (await run(() => apiClient.delete(`v1/ia/conexoes/${c.id}`), "Conexão excluída"))
      void conexoes.mutate();
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle as="h2" className="flex items-center gap-2">
            <Bot size={18} aria-hidden="true" className="text-primary" /> Assistente do Atlas
          </CardTitle>
        </CardHeader>
        <CardContent className="pb-5 pt-3">
          <DataState
            loading={uso.isLoading || conexoes.isLoading}
            error={uso.error ?? conexoes.error}
            empty={!uso.data}
          >
            {uso.data && (
              <UsoAtlas
                key={`${uso.data.updated_at}-${lista.length}`}
                uso={uso.data}
                conexoes={lista}
                onSaved={() => void uso.mutate()}
              />
            )}
          </DataState>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-3">
          <CardTitle as="h2">Conexões</CardTitle>
          <Button size="sm" onClick={() => setEditando("nova")} disabled={!provedores.data}>
            <Plus size={14} aria-hidden="true" className="mr-1" /> Nova conexão
          </Button>
        </CardHeader>
        <CardContent className="pb-5 pt-3">
          <DataState
            loading={conexoes.isLoading}
            error={conexoes.error}
            empty={lista.length === 0}
            emptyTitle="Nenhuma conexão cadastrada"
            emptyDescription="Cadastre a IA local (Ollama) ou um fornecedor (OpenAI, Azure, Gemini, Claude, Groq…) com a chave de API."
          >
            <ul className="flex flex-col divide-y divide-surface-border">
              {lista.map((c) => (
                <li key={c.id} className="flex flex-wrap items-start justify-between gap-3 py-3">
                  <div className="min-w-0">
                    <p className="flex flex-wrap items-center gap-2 font-medium text-foreground">
                      {c.nome}
                      <Badge tone="neutral">{nomeProvedor(c.provedor)}</Badge>
                      {c.externo ? (
                        <Badge tone="warning" className="gap-1">
                          <Globe size={12} aria-hidden="true" /> Externo
                        </Badge>
                      ) : (
                        <Badge tone="success" className="gap-1">
                          <Server size={12} aria-hidden="true" /> Na rede
                        </Badge>
                      )}
                    </p>
                    <p className="mt-0.5 text-xs text-muted">
                      <span className="font-mono">{c.modelo}</span> ·{" "}
                      <span className="font-mono">{c.endpoint}</span>
                      {c.tem_chave && <> · chave ••••{c.chave_final}</>}
                    </p>
                    <div className="mt-1">
                      <UltimoTeste t={c.ultimo_teste} />
                    </div>
                  </div>
                  <div className="flex gap-1">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => void testar(c)}
                      aria-label={`Testar ${c.nome}`}
                    >
                      <Send size={14} aria-hidden="true" /> Testar
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setEditando(c)}
                      aria-label={`Editar ${c.nome}`}
                    >
                      <Pencil size={14} aria-hidden="true" />
                    </Button>
                    <ConfirmButton
                      title={`Excluir a conexão "${c.nome}"?`}
                      description="A chave salva é apagada. Uma conexão em uso precisa ser trocada antes."
                      confirmLabel="Excluir"
                      onConfirm={() => excluir(c)}
                      aria-label={`Excluir ${c.nome}`}
                    >
                      <Trash2 size={14} aria-hidden="true" />
                    </ConfirmButton>
                  </div>
                </li>
              ))}
            </ul>
          </DataState>
        </CardContent>
      </Card>

      <Dialog
        open={editando !== null}
        onClose={() => setEditando(null)}
        title={editando === "nova" ? "Nova conexão de IA" : "Editar conexão de IA"}
        size="lg"
      >
        {editando !== null && provedores.data && (
          <ConexaoForm
            key={editando === "nova" ? "nova" : editando.id}
            conexao={editando === "nova" ? undefined : editando}
            provedores={provedores.data}
            onDone={() => {
              setEditando(null);
              void conexoes.mutate();
            }}
          />
        )}
      </Dialog>
    </div>
  );
}
