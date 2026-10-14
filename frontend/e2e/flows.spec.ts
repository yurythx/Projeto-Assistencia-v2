import { createHash } from "node:crypto";

import { expect, test, type Page } from "@playwright/test";

import { ADMIN_PASSWORD, asAdmin, autoAcceptConsent, NEEDS_ADMIN, STORAGE_STATE, uniq } from "./helpers";

// Fluxos críticos de ponta a ponta contra a stack real (API Go, Postgres,
// RabbitMQ, Redis, MinIO) — nenhum mock: o que a tela faz é o que o
// backend grava, audita e publica.
test.describe("Fluxos críticos", () => {
  test.skip(!ADMIN_PASSWORD, NEEDS_ADMIN);
  test.use({ storageState: STORAGE_STATE });
  // Estado compartilhado (módulos do Kernel, trilha de auditoria): em série.
  test.describe.configure({ mode: "serial" });

  function moduleCard(page: Page, name: RegExp) {
    return page.locator("div.rounded-xl").filter({ has: page.getByRole("heading", { name }) }).first();
  }

  test("Kernel: desativar um plug-in tira do menu e bloqueia a tela; reativar devolve", async ({ page }) => {
    await asAdmin(page);
    const menu = page.getByRole("navigation", { name: "Menu principal" });
    await expect(menu.getByRole("link", { name: /Blog/ })).toBeVisible();

    await page.goto("/configuracao/modulos");
    const card = moduleCard(page, /^Blog/);
    const toggle = card.getByRole("switch");
    await expect(toggle).toBeChecked();
    await toggle.click();
    await expect(toggle).not.toBeChecked();
    await expect(menu.getByRole("link", { name: /Blog/ })).toHaveCount(0);

    await page.goto("/blog");
    await expect(page.getByRole("heading", { name: /está desativado/ })).toBeVisible();

    await page.goto("/configuracao/modulos");
    await moduleCard(page, /^Blog/).getByRole("switch").click();
    await expect(moduleCard(page, /^Blog/).getByRole("switch")).toBeChecked();
    await page.goto("/blog");
    await expect(page.getByRole("heading", { name: "Blog & comunicados" })).toBeVisible();
  });

  test("Blog: rascunho, publicação e listagem", async ({ page }) => {
    await asAdmin(page);
    const title = uniq("Comunicado E2E");
    await page.goto("/blog");
    await page.getByRole("button", { name: /Nova publicação/ }).click();
    const dialog = page.getByRole("dialog", { name: "Nova publicação" });
    await dialog.getByLabel("Título *").fill(title);
    await dialog.getByLabel("Resumo").fill("Resumo do comunicado");
    await dialog.getByLabel("Conteúdo em Markdown").fill("## Aviso\n\nTexto **importante**.");
    await dialog.getByRole("button", { name: "Salvar" }).click();

    await expect(page).toHaveURL(/\/blog\/[a-z0-9-]+$/);
    await expect(page.getByRole("heading", { name: title, level: 1 })).toBeVisible();
    await expect(page.getByText("Rascunho")).toBeVisible();
    await page.getByRole("button", { name: /^Publicar$/ }).click();
    await expect(page.getByRole("button", { name: /Despublicar/ })).toBeVisible();
    await expect(page.getByText("importante")).toBeVisible();

    await page.goto("/blog");
    await expect(page.getByRole("link", { name: new RegExp(title) })).toBeVisible();
  });

  test("Contato: visitante envia pelo site, recebe protocolo e a gestão faz a triagem", async ({ page, browser }) => {
    const subject = uniq("Buraco na rua");
    const visitor = await browser.newPage();
    await autoAcceptConsent(visitor);
    await visitor.goto("/contato");
    await visitor.getByLabel("Nome *").fill("Cidadão E2E");
    await visitor.getByLabel("E-mail *").fill("cidadao@example.com");
    await visitor.getByLabel("Assunto *").fill(subject);
    await visitor.getByLabel("Mensagem *").fill("Há um buraco grande em frente ao número 10.");
    await visitor.getByRole("checkbox", { name: /Autorizo o tratamento/ }).check();
    await visitor.getByRole("button", { name: "Enviar mensagem" }).click();
    const protocol = (await visitor.locator("strong.font-mono").textContent())!.trim();
    expect(protocol).toMatch(/\S+/);
    await visitor.close();

    await asAdmin(page);
    await page.goto("/gestao/contato");
    const row = page.getByRole("row").filter({ hasText: protocol });
    await expect(row).toContainText(subject);
    await row.getByRole("button", { name: `Abrir ${protocol}` }).click();
    const dialog = page.getByRole("dialog", { name: `Mensagem ${protocol}` });
    await expect(dialog.getByText("Há um buraco grande")).toBeVisible();
    await dialog.getByLabel("Situação").selectOption("in_progress");
    await dialog.getByLabel("Anotações internas").fill("Encaminhado para obras");
    await dialog.getByRole("button", { name: "Salvar triagem" }).click();
    await expect(page.getByText("Mensagem atualizada")).toBeVisible();
  });

  test("Signum: envelope, cerimônia de assinatura com reautenticação e verificação pública", async ({ page }, info) => {
    const content = `contrato ${Date.now()}`;
    const sha = createHash("sha256").update(content).digest("hex");
    const file = { name: "contrato.txt", mimeType: "text/plain", buffer: Buffer.from(content) };
    const title = uniq("Contrato E2E");

    await asAdmin(page);
    await page.goto("/signum");
    await page.getByRole("button", { name: /Novo envelope/ }).click();
    const dialog = page.getByRole("dialog", { name: "Novo envelope de assinatura" });
    await dialog.getByLabel("Documento *").setInputFiles(file);
    await expect(dialog.getByText(`SHA-256: ${sha}`)).toBeVisible();
    await dialog.getByLabel("Título").fill(title);
    await dialog.getByLabel("Signatários *").fill("admin");
    await dialog.getByRole("option").first().getByRole("button").click();
    await dialog.getByRole("button", { name: "Abrir envelope" }).click();
    await expect(page).toHaveURL(/\/signum\/[0-9a-f-]{36}$/);
    const envelopeId = page.url().split("/").pop()!;

    await page.getByRole("button", { name: /^Assinar$/ }).click();
    const ceremony = page.getByRole("dialog", { name: "Assinar documento" });
    await ceremony.getByLabel("Selecione o arquivo que você vai assinar").setInputFiles(file);
    await expect(ceremony.getByText("Documento íntegro — idêntico ao do envelope.")).toBeVisible();
    await ceremony.getByRole("button", { name: "Gerar desafio de assinatura" }).click();
    // senha errada: recusada sem derrubar a sessão
    await ceremony.getByLabel("Sua senha").fill("senha-errada");
    await ceremony.getByRole("button", { name: /Assinar/ }).click();
    await expect(page.getByText("senha incorreta — a assinatura não foi realizada")).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/signum/${envelopeId}$`));
    await ceremony.getByLabel("Sua senha").fill(ADMIN_PASSWORD!);
    await ceremony.getByRole("button", { name: /Assinar/ }).click();
    await expect(page.getByText(/concluído em/)).toBeVisible();

    // verificação pública (sem sessão)
    const pub = await page.context().browser()!.newPage();
    await autoAcceptConsent(pub);
    await pub.goto(`/verificar/${envelopeId}`);
    await expect(pub.getByRole("heading", { name: title })).toBeVisible();
    await expect(pub.getByText("Todas as assinaturas concluídas")).toBeVisible();
    await expect(pub.getByText("Válida")).toBeVisible();
    await pub.getByLabel("Conferir um arquivo").setInputFiles(file);
    await expect(pub.getByText("O arquivo é idêntico ao documento assinado.")).toBeVisible();
    await pub.getByLabel("Conferir um arquivo").setInputFiles({ ...file, buffer: Buffer.from("adulterado") });
    await expect(pub.getByText("O arquivo é diferente do documento assinado.")).toBeVisible();
    await pub.close();
    info.annotations.push({ type: "envelope", description: envelopeId });
  });

  test("Arquivos: pasta, envio direto ao MinIO (URL pré-assinada) e download", async ({ page }) => {
    await asAdmin(page);
    const folder = uniq("Pasta E2E");
    await page.goto("/arquivos");
    await page.getByRole("button", { name: /Nova pasta/ }).click();
    const dialog = page.getByRole("dialog", { name: "Nova pasta" });
    await dialog.getByLabel("Nome").fill(folder);
    await dialog.getByRole("button", { name: "Salvar" }).click();
    await expect(page).toHaveURL(/\/arquivos\?pasta=/);
    await expect(page.getByRole("heading", { name: folder })).toBeVisible();

    await page.locator("#file-upload").setInputFiles({ name: "ata.txt", mimeType: "text/plain", buffer: Buffer.from("ata da reunião") });
    await expect(page.getByText("ata.txt enviado")).toBeVisible();
    const row = page.getByRole("row").filter({ hasText: "ata.txt" });
    await expect(row.getByText("envio pendente")).toHaveCount(0);

    // o download sai direto do MinIO por URL pré-assinada (o binário não
    // passa pela API nem pelo BFF)
    await row.getByRole("button", { name: "Baixar ata.txt" }).click();
    await page.waitForURL(/\/files\/.*X-Amz-Signature=/); // host público do MinIO (MINIO_PUBLIC_URL)
    await expect(page.locator("body")).toContainText("ata da reuni");
  });

  test("SEMPRAS: atendimento, direitos do titular e decisão da gestão", async ({ page }) => {
    await asAdmin(page);
    const sufixo = uniq("e2e").toLowerCase().replace(/[^a-z0-9]/g, "").slice(0, 16);

    // Unidade mínima pela API, sob a entidade "sempras" — o serviço só
    // mostra pro admin/superintendente (cobertura total) as unidades da
    // entidade de slug EXATO "sempras" (EntidadeSlug, application/service.go),
    // nunca uma entidade qualquer; reaproveita a existente (deploy/estrutura
    // ou fixture de outro teste) em vez de assumir que não existe.
    const orgTree = await page.request.get("/api/backend/v1/iam/org-tree");
    expect(orgTree.ok(), await orgTree.text()).toBeTruthy();
    const arvore = (await orgTree.json()).data as { id: string; slug: string }[];
    let entidadeId = arvore.find((e) => e.slug === "sempras")?.id;
    if (!entidadeId) {
      const ent = await page.request.post("/api/backend/v1/iam/entidades", { data: { nome: "SEMPRAS", slug: "sempras", ativo: true } });
      expect(ent.ok(), await ent.text()).toBeTruthy();
      entidadeId = (await ent.json()).data.id;
    }
    const unidade = await page.request.post("/api/backend/v1/iam/unidades", {
      data: { nome: `CRAS E2E ${sufixo}`, slug: `cras-e2e-${sufixo}`, entidade_id: entidadeId, ativo: true },
    });
    expect(unidade.ok(), await unidade.text()).toBeTruthy();
    const unidadeNome = `CRAS E2E ${sufixo}`;

    // liga o módulo SEMPRAS, se ainda não estiver
    await page.goto("/configuracao/modulos");
    const card = moduleCard(page, /^SEMPRAS/);
    const toggle = card.getByRole("switch");
    if (!(await toggle.isChecked())) await toggle.click();
    await expect(toggle).toBeChecked();

    // recepção: busca (ninguém), cadastra e inclui na fila
    const nome = uniq("Munícipe E2E");
    await page.goto("/sempras");
    await page.getByLabel("Unidade").selectOption({ label: unidadeNome });
    await page.getByLabel("CPF, NIS ou nome").fill(nome);
    await page.getByRole("button", { name: /Buscar/ }).click();
    await expect(page.getByText(/Ninguém encontrado/)).toBeVisible();
    await page.getByRole("button", { name: /Cadastrar/ }).click();
    const cadastro = page.getByRole("dialog", { name: "Cadastrar munícipe" });
    await cadastro.getByLabel("Nome civil *").fill(nome);
    await cadastro.getByLabel("Nome da mãe").fill("Mãe E2E");
    await cadastro.getByLabel("Data de nascimento").fill("1990-01-01");
    await cadastro.getByRole("button", { name: "Salvar cadastro" }).click();
    await expect(page.getByText("Sem histórico na área")).toBeVisible();
    await page.getByRole("button", { name: new RegExp(`Incluir ${nome} na fila`) }).click();
    await expect(page.getByText(nome)).toBeVisible();

    // ficha: exerce os 3 direitos do titular (ADR 030 §8)
    await page.getByRole("link", { name: "Ficha" }).click();
    await expect(page).toHaveURL(/\/sempras\/cidadaos\/[0-9a-f-]{36}$/);
    await expect(page.getByRole("heading", { name: nome })).toBeVisible();

    await page.getByRole("button", { name: "Editar" }).click();
    const editar = page.getByRole("dialog", { name: "Editar cadastro" });
    await editar.getByLabel("Telefone").fill("66999990000");
    await editar.getByLabel(/A pedido do titular/).check();
    await editar.getByLabel("Justificativa *").fill("titular pediu para corrigir o telefone, e2e");
    await editar.getByRole("button", { name: "Salvar" }).click();
    await expect(editar).toBeHidden();
    await expect(page.getByText("66999990000")).toBeVisible();

    await page.getByRole("button", { name: "Solicitar anonimização" }).click();
    const anonimizar = page.getByRole("dialog", { name: "Solicitar anonimização" });
    await anonimizar.getByLabel("Justificativa *").fill("titular pediu para ser esquecido, e2e");
    await anonimizar.getByRole("button", { name: "Registrar pedido" }).click();
    await expect(page.getByText("Pedido registrado — aguardando decisão da gestão")).toBeVisible();

    // gestão: vê o pedido pendente e aprova
    await page.goto("/sempras/pedidos");
    const linha = page.getByRole("row").filter({ hasText: nome });
    await expect(linha).toContainText("Pendente");
    await linha.getByRole("button", { name: "Decidir" }).click();
    const decisao = page.getByRole("dialog", { name: `Decidir anonimização — ${nome}` });
    await expect(decisao.getByText("titular pediu para ser esquecido, e2e")).toBeVisible();
    await decisao.getByRole("button", { name: "Aprovar anonimização" }).click();
    await expect(page.getByText("Anonimização aprovada")).toBeVisible();
    await expect(page.getByRole("row").filter({ hasText: nome })).toHaveCount(0);

    // o cadastro perdeu os identificadores; o histórico da unidade continua.
    await page.goto("/sempras");
    await page.getByLabel("CPF, NIS ou nome").fill("Munícipe anonimizado");
    await page.getByRole("button", { name: /Buscar/ }).click();
    await expect(page.getByText("Munícipe anonimizado").first()).toBeVisible();
  });

  test("Auditoria: toda a movimentação acima fica na trilha e a cadeia de hash é íntegra", async ({ page }) => {
    await asAdmin(page);
    await page.goto("/auditoria");
    await expect(page.getByRole("table", { name: "Registros de auditoria" })).toBeVisible();
    await page.getByRole("button", { name: /Verificar integridade/ }).click();
    await expect(page.getByText("Cadeia íntegra")).toBeVisible();
  });
});
