import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fail, identityRoutes, mockBackend, renderApp } from "@/test/backend";
import { resetNavigation, router } from "@/test/navigation";

vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);

import ArquivosPage from "./page";

const folder = (id: string, extra: Record<string, unknown> = {}) => ({
  id, name: `Pasta ${id}`, owner_id: "u1", owner_name: "Maria", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z", ...extra,
});
const file = (id: string, extra: Record<string, unknown> = {}) => ({
  id, folder_id: "f1", name: `${id}.pdf`, size_bytes: 2048, content_type: "application/pdf", status: "ready",
  owner_id: "u1", owner_name: "Maria", updated_at: "2026-01-01T00:00:00Z", ...extra,
});
const access = (write: boolean, manage = false) => ({ read: true, write, manage });

const ORG = [{ id: "e1", nome: "Órgão", unidades: [{ id: "u1", nome: "Unidade A", sigla: "", departamentos: [{ id: "d1", nome: "TI" }] }] }];

describe("Arquivos — raiz", () => {
  beforeEach(() => resetNavigation({}, "/arquivos"));

  it("pasta com unidade dona (ADR 013): oferece as unidades em que a pessoa está lotada", async () => {
    const api = mockBackend({
      ...identityRoutes({ permissions: [], scopes: [{ perfil: "p", origem: "manual", entidade_id: "e1", unidade_id: "u1", permissions: [] }] }),
      "GET v1/iam/org-tree": { data: ORG },
      "GET v1/files/browse": { data: { breadcrumbs: [], folders: [folder("f1", { unidade_id: "u1" })], files: [], access: access(false) } },
      "POST v1/files/folders": { data: folder("nova") },
      "PATCH v1/files/folders/f1": { data: folder("f1") },
    });
    renderApp(<ArquivosPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Nova pasta/ }));
    let dialog = await screen.findByRole("dialog", { name: "Nova pasta" });
    await userEvent.type(within(dialog).getByLabelText("Nome"), "Da unidade");
    const dona = within(dialog).getByLabelText("Unidade dona");
    await waitFor(() => expect(within(dona).getByRole("option", { name: "Unidade A" })).toBeInTheDocument());
    await userEvent.selectOptions(dona, "u1");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("POST v1/files/folders")[0]?.body).toMatchObject({ unidade_id: "u1" }));

    await userEvent.click(screen.getByRole("button", { name: "Renomear Pasta f1" }));
    dialog = await screen.findByRole("dialog", { name: "Editar pasta" });
    await waitFor(() => expect(within(dialog).getByLabelText("Unidade dona")).toHaveValue("u1"));
  });

  it("lista as pastas; cria pasta e abre; renomeia e exclui", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/files/browse": { data: { breadcrumbs: [], folders: [folder("f1", { parent_id: "p0" })], files: [], access: access(false) } },
      "POST v1/files/folders": { data: folder("novo") },
      "PATCH v1/files/folders/f1": { data: folder("f1") },
      "DELETE v1/files/folders/f1": { data: null },
    });
    renderApp(<ArquivosPage />);
    expect(await screen.findByRole("link", { name: /Pasta f1/ })).toHaveAttribute("href", "/arquivos?pasta=f1");
    expect(screen.getByRole("heading", { name: "Pastas" })).toBeInTheDocument();
    // raiz: sem envio de arquivos
    expect(screen.queryByRole("button", { name: /Enviar arquivos/ })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Nova pasta/ }));
    let dialog = await screen.findByRole("dialog", { name: "Nova pasta" });
    await userEvent.type(within(dialog).getByLabelText("Nome"), "  Contratos  ");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/arquivos?pasta=novo"));
    expect(api.to("POST v1/files/folders")[0]!.body).toEqual({ name: "Contratos", parent_id: null, unidade_id: null });

    await userEvent.click(screen.getByRole("button", { name: "Renomear Pasta f1" }));
    dialog = await screen.findByRole("dialog", { name: "Editar pasta" });
    const input = within(dialog).getByLabelText("Nome");
    expect(input).toHaveValue("Pasta f1");
    await userEvent.clear(input);
    await userEvent.type(input, "Renomeada");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PATCH v1/files/folders/f1")).toHaveLength(1));
    expect(api.to("PATCH v1/files/folders/f1")[0]!.body).toEqual({ name: "Renomeada", parent_id: "p0", move: false, unidade_id: null, set_unidade: true });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Editar pasta" })).not.toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "Excluir Pasta f1" }));
    dialog = await screen.findByRole("dialog", { name: 'Excluir a pasta "Pasta f1"?' });
    await userEvent.click(within(dialog).getByRole("button", { name: "Excluir tudo" }));
    await waitFor(() => expect(api.to("DELETE v1/files/folders/f1")).toHaveLength(1));
    expect(api.to("DELETE v1/files/folders/f1")[0]!.query.get("recursive")).toBe("true");
  });

  it("nome em branco não envia; falha mantém o diálogo aberto", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/files/browse": { data: { breadcrumbs: [], folders: [], files: [], access: access(false) } },
      "POST v1/files/folders": fail(409, "já existe uma pasta com esse nome"),
    });
    renderApp(<ArquivosPage />);
    expect(await screen.findByText("Pasta vazia")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Nova pasta/ }));
    const dialog = await screen.findByRole("dialog", { name: "Nova pasta" });
    await userEvent.type(within(dialog).getByLabelText("Nome"), "   ");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    expect(api.to("POST v1/files/folders")).toHaveLength(0);
    await userEvent.type(within(dialog).getByLabelText("Nome"), "X");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    expect(await screen.findByText("já existe uma pasta com esse nome")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Nova pasta" })).toBeInTheDocument();
  });

  it("erro ao listar", async () => {
    mockBackend({ ...identityRoutes(), "GET v1/files/browse": fail(500, "armazenamento fora") });
    renderApp(<ArquivosPage />);
    expect(await screen.findByText("armazenamento fora")).toBeInTheDocument();
  });
});

describe("Arquivos — dentro de uma pasta", () => {
  beforeEach(() => resetNavigation({}, "/arquivos", "pasta=f1"));

  const listing = (write: boolean, manage = false) => ({
    data: {
      folder: folder("f1"),
      breadcrumbs: [folder("p0")],
      folders: [folder("sub")],
      files: [file("ata"), file("rascunho", { status: "pending" })],
      access: access(write, manage),
    },
  });

  it("somente leitura: baixa, mas não envia, renomeia nem exclui arquivo", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/files/browse": listing(false),
      "GET v1/files/objects/ata/download": { data: { url: "https://minio/ata.pdf?sig" } },
    });
    renderApp(<ArquivosPage />);
    expect(await screen.findByRole("heading", { name: "Pasta f1" })).toBeInTheDocument();
    const trail = screen.getByRole("navigation", { name: "Caminho" });
    expect(within(trail).getByRole("link", { name: "Pasta p0" })).toHaveAttribute("href", "/arquivos?pasta=p0");
    expect(screen.getByText("envio pendente")).toBeInTheDocument();
    expect(screen.getAllByText("2.0 KB")).toHaveLength(2);
    expect(screen.queryByRole("button", { name: /Nova pasta/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Renomear ata.pdf" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Baixar rascunho.pdf" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Permissões/ })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Baixar ata.pdf" }));
    await waitFor(() => expect(api.to("GET v1/files/objects/ata/download")).toHaveLength(1));
  });

  it("escrita: envia vários arquivos (ticket, PUT, confirmação), renomeia e exclui", async () => {
    let n = 0;
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/files/browse": listing(true),
      "POST v1/files/uploads": () => {
        n++;
        return { data: { file: file(`up${n}`), upload: { object_key: `k${n}`, upload_url: `http://minio.test/b/k${n}`, method: "PUT", headers: {}, expires_at: "" } } };
      },
      "PUT minio.test/*": (req) => (req.path.endsWith("k2") ? { status: 500 } : { status: 200 }),
      "POST v1/files/objects/up1/confirm": { data: null },
      "PATCH v1/files/objects/ata": { data: null },
      "DELETE v1/files/objects/ata": { data: null },
      "POST v1/files/folders": { data: folder("nova") },
    });
    renderApp(<ArquivosPage />);
    await screen.findByRole("heading", { name: "Pasta f1" });

    await userEvent.upload(document.getElementById("file-upload") as HTMLInputElement, [
      new File(["a"], "a.txt", { type: "text/plain" }),
      new File(["bb"], "b.bin", { type: "" }),
    ]);
    await waitFor(() => expect(api.to("POST v1/files/uploads")).toHaveLength(2));
    expect(api.to("POST v1/files/uploads")[0]!.body).toEqual({ folder_id: "f1", filename: "a.txt", content_type: "text/plain", size: 1 });
    expect(api.to("POST v1/files/uploads")[1]!.body).toMatchObject({ content_type: "application/octet-stream", size: 2 });
    // o segundo PUT falhou: só o primeiro é confirmado
    await waitFor(() => expect(api.to("POST v1/files/objects/up1/confirm")).toHaveLength(1));
    expect(api.to("POST v1/files/objects/up2/confirm")).toHaveLength(0);
    expect(await screen.findByText("a.txt enviado")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Renomear ata.pdf" }));
    const dialog = await screen.findByRole("dialog", { name: "Renomear arquivo" });
    await userEvent.type(within(dialog).getByLabelText("Nome"), "-v2");
    await userEvent.click(within(dialog).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("PATCH v1/files/objects/ata")[0]?.body).toEqual({ name: "ata.pdf-v2", folder_id: "f1" }));

    await userEvent.click(screen.getByRole("button", { name: "Excluir ata.pdf" }));
    const confirm = await screen.findByRole("dialog", { name: 'Excluir "ata.pdf"?' });
    await userEvent.click(within(confirm).getByRole("button", { name: "Excluir" }));
    await waitFor(() => expect(api.to("DELETE v1/files/objects/ata")).toHaveLength(1));

    // subpasta: cria e só recarrega (não navega)
    await userEvent.click(screen.getByRole("button", { name: /Nova pasta/ }));
    const nd = await screen.findByRole("dialog", { name: "Nova pasta" });
    await userEvent.type(within(nd).getByLabelText("Nome"), "Sub");
    await userEvent.click(within(nd).getByRole("button", { name: "Salvar" }));
    await waitFor(() => expect(api.to("POST v1/files/folders")[0]?.body).toEqual({ name: "Sub", parent_id: "f1", unidade_id: null }));
    expect(router.push).not.toHaveBeenCalled();
  });

  it("dono edita as permissões (herdadas pelas subpastas)", async () => {
    const api = mockBackend({
      ...identityRoutes(),
      "GET v1/files/browse": listing(true, true),
      "GET v1/files/folders/f1/acl": { data: [{ subject_type: "perfil", subject: "gestor", can_write: true }, { subject_type: "ad_group", subject: "GRP_X", can_write: false }] },
      "GET v1/iam/perfis": { data: [{ slug: "gestor", nome: "Gestor" }] },
      "GET v1/iam/org-tree": { data: ORG },
      "PUT v1/files/folders/f1/acl": { data: null },
    });
    renderApp(<ArquivosPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Permissões/ }));
    const dialog = await screen.findByRole("dialog", { name: "Permissões — Pasta f1" });
    expect(await within(dialog).findByText(/Gestor · leitura e escrita/)).toBeInTheDocument();
    expect(within(dialog).getByText(/GRP_X · somente leitura/)).toBeInTheDocument();
    const save = within(dialog).getByRole("button", { name: "Salvar permissões" });
    expect(save).toBeDisabled(); // nada mudou

    // remove o grupo do AD
    await userEvent.click(within(dialog).getAllByRole("button", { name: "Remover entrada" })[1]!);
    // adiciona um departamento com escrita
    await userEvent.selectOptions(within(dialog).getByLabelText("Sujeito"), "departamento");
    const add = within(dialog).getByRole("button", { name: /Adicionar/ });
    expect(add).toBeDisabled();
    await userEvent.selectOptions(within(dialog).getByLabelText("Qual"), "d1");
    await userEvent.click(within(dialog).getByLabelText("Escrita"));
    await userEvent.click(add);
    expect(within(dialog).getByText(/TI \(Unidade A\) · leitura e escrita/)).toBeInTheDocument();
    // "todos os autenticados" dispensa sujeito
    await userEvent.selectOptions(within(dialog).getByLabelText("Sujeito"), "everyone");
    expect(within(dialog).getByLabelText("Qual")).toBeDisabled();
    await userEvent.click(within(dialog).getByRole("button", { name: /Adicionar/ }));
    // usuário por UUID
    await userEvent.selectOptions(within(dialog).getByLabelText("Sujeito"), "user");
    expect(within(dialog).getByLabelText("Qual")).toHaveAttribute("placeholder", "UUID do usuário");
    await userEvent.selectOptions(within(dialog).getByLabelText("Sujeito"), "ad_group");
    expect(within(dialog).getByLabelText("Qual")).toHaveAttribute("placeholder", "GRP_EQUIPE");
    await userEvent.type(within(dialog).getByLabelText("Qual"), " GRP_Y ");
    await userEvent.click(within(dialog).getByRole("button", { name: /Adicionar/ }));
    await userEvent.selectOptions(within(dialog).getByLabelText("Sujeito"), "unidade");
    await userEvent.selectOptions(within(dialog).getByLabelText("Qual"), "u1");
    await userEvent.click(within(dialog).getByRole("button", { name: /Adicionar/ }));

    await userEvent.click(save);
    await waitFor(() => expect(api.to("PUT v1/files/folders/f1/acl")).toHaveLength(1));
    expect(api.to("PUT v1/files/folders/f1/acl")[0]!.body).toEqual({
      entries: [
        { subject_type: "perfil", subject: "gestor", can_write: true },
        { subject_type: "departamento", subject: "d1", can_write: true },
        { subject_type: "everyone", subject: "", can_write: true },
        { subject_type: "ad_group", subject: "GRP_Y", can_write: true },
        { subject_type: "unidade", subject: "u1", can_write: true },
      ],
    });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: /Permissões/ })).not.toBeInTheDocument());
  });

  it("pasta sem entradas é privada", async () => {
    mockBackend({
      ...identityRoutes(),
      "GET v1/files/browse": listing(true, true),
      "GET v1/files/folders/f1/acl": { data: [] },
      "GET v1/iam/perfis": { data: [] },
      "GET v1/iam/org-tree": { data: [] },
    });
    renderApp(<ArquivosPage />);
    await userEvent.click(await screen.findByRole("button", { name: /Permissões/ }));
    expect(await screen.findByText("Sem entradas — pasta privada.")).toBeInTheDocument();
  });
});
