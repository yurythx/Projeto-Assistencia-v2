#!/usr/bin/env node
// Gera os dados FICTÍCIOS do ambiente de teste a partir de uma única
// definição da estrutura organizacional:
//
//   deploy/keycloak/realm-nexus.json  realm "nexus" (clients, grupos, usuários)
//   deploy/demo/seed-demo.sql         entidades, unidades, departamentos,
//                                     mapeamentos grupo -> perfil, usuários
//                                     pré-provisionados e perfis do Diretório
//   deploy/demo/usuarios.csv          lista dos usuários (sem senha)
//
// Determinístico (PRNG com semente fixa, UUIDs derivados de nomes): rodar
// de novo gera os mesmos arquivos, e o SQL é idempotente. Nada aqui é dado
// real — nomes sorteados de listas comuns, e-mails no TLD reservado .test.
//
//   node scripts/demo-data/generate.mjs
//
// Segredos NÃO entram nos arquivos: o realm usa placeholders que o
// Keycloak resolve do ambiente no import (${KEYCLOAK_FRONTEND_CLIENT_SECRET}, ${KEYCLOAK_CLIENT_SECRET},
// ${FRONTEND_URL}, ${DEMO_USER_PASSWORD}).
import { createHash } from "node:crypto";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const USERS_PER_UNIT = 50;
const EMAIL_DOMAIN = "nexus.test";

// --- Estrutura organizacional -------------------------------------------

// Departamentos padrão de toda unidade (exceto a sede da Prefeitura).
// cargos[0] é o chefe do departamento; os demais se repetem.
const DEPARTAMENTOS_PADRAO = [
  { code: "DIR", nome: "Direção", cargos: ["Coordenador(a) da Unidade", "Assessor(a) de Direção", "Secretário(a) Executivo(a)"] },
  { code: "ADM", nome: "Administração", cargos: ["Chefe Administrativo(a)", "Assistente Administrativo(a)", "Agente Administrativo(a)", "Auxiliar Administrativo(a)"] },
  { code: "ATD", nome: "Atendimento", cargos: ["Supervisor(a) de Atendimento", "Atendente", "Recepcionista", "Agente de Atendimento"] },
  { code: "RH", nome: "Recursos Humanos", cargos: ["Chefe de Recursos Humanos", "Analista de RH", "Assistente de RH"] },
  { code: "ALM", nome: "Almoxarifado", cargos: ["Chefe de Almoxarifado", "Almoxarife", "Auxiliar de Almoxarifado"] },
];

const DEPARTAMENTOS_PREFEITURA = [
  { code: "RH", nome: "Recursos Humanos", cargos: ["Diretor(a) de Recursos Humanos", "Analista de RH", "Assistente de RH", "Técnico(a) de Folha de Pagamento"] },
  { code: "TI", nome: "Tecnologia da Informação", cargos: ["Diretor(a) de TI", "Analista de Sistemas", "Técnico(a) de Suporte", "Administrador(a) de Redes"] },
  { code: "CONT", nome: "Contabilidade", cargos: ["Contador(a) Geral", "Contador(a)", "Técnico(a) em Contabilidade"] },
  { code: "COMP", nome: "Compras", cargos: ["Diretor(a) de Compras", "Comprador(a)", "Pregoeiro(a)", "Assistente de Compras"] },
  { code: "ADM", nome: "Administração", cargos: ["Diretor(a) Administrativo(a)", "Assistente Administrativo(a)", "Agente Administrativo(a)"] },
  { code: "GOV", nome: "Governo", cargos: ["Chefe de Gabinete", "Assessor(a) de Gabinete", "Assessor(a) de Comunicação", "Assessor(a) Jurídico(a)"] },
];

const ENTIDADES = [
  {
    code: "PREF", slug: "prefeitura", sigla: "PREF", nome: "Prefeitura Municipal",
    unidades: [{ slug: "sede", sigla: "SEDE", nome: "Sede Administrativa da Prefeitura", departamentos: DEPARTAMENTOS_PREFEITURA }],
  },
  {
    code: "SEMSA", slug: "saude", sigla: "SEMSA", nome: "Secretaria Municipal de Saúde",
    unidades: [
      { slug: "psf-sagrada-familia", sigla: "PSF-SF", nome: "PSF Sagrada Família" },
      { slug: "psf-conjunto", sigla: "PSF-CJ", nome: "PSF Conjunto" },
      { slug: "psf-marechal-rondon", sigla: "PSF-MR", nome: "PSF Marechal Rondon" },
      { slug: "upa", sigla: "UPA", nome: "UPA 24h" },
    ],
  },
  {
    code: "SEMED", slug: "educacao", sigla: "SEMED", nome: "Secretaria Municipal de Educação",
    unidades: [
      { slug: "escola-maria-elza", sigla: "EMME", nome: "Escola Maria Elza" },
      { slug: "escola-marechal-dutra", sigla: "EMMD", nome: "Escola Marechal Dutra" },
      { slug: "escola-silvestre", sigla: "EMS", nome: "Escola Silvestre" },
      { slug: "escola-elizabete", sigla: "EMEL", nome: "Escola Elizabete" },
    ],
  },
  {
    code: "SEMPRAS", slug: "assistencia-social", sigla: "SEMPRAS", nome: "Secretaria Municipal de Promoção e Assistência Social",
    unidades: [
      { slug: "sede", sigla: "SEDE", nome: "Sede da SEMPRAS" },
      { slug: "cras-conjunto", sigla: "CRAS-CJ", nome: "CRAS Conjunto" },
      { slug: "cras-ana-carla", sigla: "CRAS-AC", nome: "CRAS Ana Carla" },
      { slug: "cras-alfredo", sigla: "CRAS-AL", nome: "CRAS Alfredo" },
      { slug: "creas", sigla: "CREAS", nome: "CREAS" },
      { slug: "centro-pop", sigla: "POP", nome: "Centro POP" },
    ],
  },
];

// Grupos de plataforma -> perfil de sistema (escopo: plataforma toda).
const GRUPOS_PLATAFORMA = [
  { group: "NEXUS-ADMINS", perfil: "administrador", descricao: "Administradores da plataforma (teste)" },
  { group: "NEXUS-AUDITORES", perfil: "auditor", descricao: "Auditores (teste)" },
  { group: "NEXUS-GESTORES-IAM", perfil: "gestor-iam", descricao: "Gestores de identidade (teste)" },
  { group: "NEXUS-GESTORES-CONTEUDO", perfil: "gestor-conteudo", descricao: "Gestores de conteúdo (teste)" },
];

// Contas de teste por papel, lotadas na TI da Prefeitura.
const CONTAS_TESTE = [
  { username: "teste.admin", first: "Teste", last: "Administrador", groups: ["NEXUS-ADMINS"], roles: ["nexus-admin"], cargo: "Administrador(a) da Plataforma" },
  { username: "teste.auditor", first: "Teste", last: "Auditor", groups: ["NEXUS-AUDITORES"], roles: [], cargo: "Auditor(a)" },
  { username: "teste.iam", first: "Teste", last: "Gestor IAM", groups: ["NEXUS-GESTORES-IAM"], roles: [], cargo: "Gestor(a) de Identidade" },
  { username: "teste.conteudo", first: "Teste", last: "Gestor Conteúdo", groups: ["NEXUS-GESTORES-CONTEUDO"], roles: [], cargo: "Gestor(a) de Conteúdo" },
];

// --- Nomes fictícios ----------------------------------------------------

const PRIMEIROS = [
  "Ana", "Maria", "Juliana", "Fernanda", "Patrícia", "Aline", "Camila", "Amanda", "Bruna", "Letícia",
  "Mariana", "Gabriela", "Larissa", "Beatriz", "Vanessa", "Débora", "Cristina", "Renata", "Simone", "Luciana",
  "Tatiane", "Adriana", "Sandra", "Rosângela", "Eliane", "Priscila", "Carla", "Daniela", "Natália", "Raquel",
  "Francisca", "Antônia", "Márcia", "Cláudia", "Jéssica", "Kelly", "Sônia", "Vera", "Lúcia", "Helena",
  "José", "João", "Antônio", "Francisco", "Carlos", "Paulo", "Pedro", "Lucas", "Luiz", "Marcos",
  "Luís", "Gabriel", "Rafael", "Daniel", "Marcelo", "Bruno", "Eduardo", "Felipe", "Raimundo", "Rodrigo",
  "Manoel", "Mateus", "André", "Fernando", "Fábio", "Leonardo", "Gustavo", "Guilherme", "Leandro", "Tiago",
  "Anderson", "Ricardo", "Jorge", "Alexandre", "Roberto", "Diego", "Sérgio", "Vinícius", "Márcio", "Wesley",
];
const SOBRENOMES = [
  "Silva", "Santos", "Oliveira", "Souza", "Rodrigues", "Ferreira", "Alves", "Pereira", "Lima", "Gomes",
  "Costa", "Ribeiro", "Martins", "Carvalho", "Almeida", "Lopes", "Soares", "Fernandes", "Vieira", "Barbosa",
  "Rocha", "Dias", "Nascimento", "Andrade", "Moreira", "Nunes", "Marques", "Machado", "Mendes", "Freitas",
  "Cardoso", "Ramos", "Gonçalves", "Santana", "Teixeira", "Araújo", "Batista", "Campos", "Castro", "Correia",
  "Monteiro", "Moura", "Pinto", "Reis", "Cavalcante", "Borges", "Miranda", "Rezende", "Farias", "Sales",
  "Queiroz", "Tavares", "Pires", "Brito", "Xavier", "Coelho", "Siqueira", "Figueiredo", "Duarte", "Macedo",
];

// mulberry32: PRNG pequeno e determinístico.
function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rand = prng(20260926);
const pick = (arr) => arr[Math.floor(rand() * arr.length)];

// UUID determinístico (formato v5) a partir de um nome.
function uuidFrom(name) {
  const h = createHash("sha1").update(`projeto-nexus-demo:${name}`).digest();
  h[6] = (h[6] & 0x0f) | 0x50;
  h[8] = (h[8] & 0x3f) | 0x80;
  const x = h.subarray(0, 16).toString("hex");
  return `${x.slice(0, 8)}-${x.slice(8, 12)}-${x.slice(12, 16)}-${x.slice(16, 20)}-${x.slice(20)}`;
}

const ascii = (s) => s.normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase().replace(/[^a-z0-9]+/g, ".");
const sqlStr = (s) => `'${String(s).replaceAll("'", "''")}'`;
const sqlArr = (a) => (a.length ? `ARRAY[${a.map(sqlStr).join(", ")}]` : "ARRAY[]::TEXT[]");
const sqlUuid = (u) => (u ? `'${u}'` : "NULL");

// --- Montagem -----------------------------------------------------------

const usedUsernames = new Set(CONTAS_TESTE.map((c) => c.username));
function uniqueUsername(first, last) {
  const base = `${ascii(first)}.${ascii(last)}`;
  let u = base;
  for (let n = 2; usedUsernames.has(u); n++) u = `${base}${n}`;
  usedUsernames.add(u);
  return u;
}

const entidades = [];
const unidades = [];
const departamentos = [];
const users = [];
let matricula = 1;
let ramal = 2000;

for (const e of ENTIDADES) {
  const ent = { ...e, id: uuidFrom(`entidade:${e.slug}`) };
  entidades.push(ent);
  for (const u of e.unidades) {
    const code = `${e.code}-${u.slug.toUpperCase()}`;
    const uni = { ...u, id: uuidFrom(`unidade:${e.slug}/${u.slug}`), entidade: ent, code };
    unidades.push(uni);
    const deps = (u.departamentos ?? DEPARTAMENTOS_PADRAO).map((d) => ({
      ...d,
      id: uuidFrom(`departamento:${e.slug}/${u.slug}/${d.code}`),
      unidade: uni,
      group: `${code}-${d.code}`,
      slug: ascii(d.nome).replaceAll(".", "-"),
    }));
    departamentos.push(...deps);
    uni.departamentos = deps;

    // 50 por unidade, distribuídos igualmente entre os departamentos.
    for (let i = 0; i < USERS_PER_UNIT; i++) {
      const dep = deps[i % deps.length];
      const nth = Math.floor(i / deps.length); // 0 = chefe do departamento
      const first = pick(PRIMEIROS);
      const last = `${pick(SOBRENOMES)} ${pick(SOBRENOMES)}`;
      const lastShort = last.split(" ").at(-1);
      const username = uniqueUsername(first, lastShort);
      users.push({
        id: uuidFrom(`user:${username}`),
        username, first, last,
        email: `${username}@${EMAIL_DOMAIN}`,
        matricula: String(matricula++).padStart(6, "0"),
        ramal: String(ramal++),
        cargo: nth === 0 ? dep.cargos[0] : dep.cargos[1 + ((nth - 1) % (dep.cargos.length - 1))],
        dep, groups: [dep.group], roles: [],
      });
    }
  }
}

const ti = departamentos.find((d) => d.group === "PREF-SEDE-TI");
for (const c of CONTAS_TESTE) {
  users.push({
    id: uuidFrom(`user:${c.username}`),
    username: c.username, first: c.first, last: c.last,
    email: `${c.username}@${EMAIL_DOMAIN}`,
    matricula: String(matricula++).padStart(6, "0"),
    ramal: String(ramal++),
    cargo: c.cargo, dep: ti, groups: [ti.group, ...c.groups], roles: c.roles,
  });
}

// --- Keycloak realm -----------------------------------------------------

const groupPath = new Map();
const kcGroups = entidades.map((e) => ({
  name: e.code,
  attributes: { entidade: [e.nome] },
  subGroups: unidades
    .filter((u) => u.entidade === e)
    .map((u) => ({
      name: u.code,
      attributes: { unidade: [u.nome] },
      subGroups: u.departamentos.map((d) => {
        groupPath.set(d.group, `/${e.code}/${u.code}/${d.group}`);
        return { name: d.group, attributes: { departamento: [d.nome] } };
      }),
    })),
}));
kcGroups.push({
  name: "NEXUS",
  attributes: { descricao: ["Grupos de papéis da plataforma"] },
  subGroups: GRUPOS_PLATAFORMA.map((g) => {
    groupPath.set(g.group, `/NEXUS/${g.group}`);
    return { name: g.group, attributes: { descricao: [g.descricao] } };
  }),
});

const realm = {
  realm: "nexus",
  displayName: "Projeto Nexus (teste)",
  enabled: true,
  // Ambiente de TESTE servido por HTTP na rede interna.
  sslRequired: "none",
  loginWithEmailAllowed: true,
  duplicateEmailsAllowed: false,
  resetPasswordAllowed: false,
  rememberMe: true,
  internationalizationEnabled: true,
  supportedLocales: ["pt-BR"],
  defaultLocale: "pt-BR",
  accessTokenLifespan: 900,
  ssoSessionIdleTimeout: 3600,
  bruteForceProtected: true,
  roles: {
    realm: [
      { name: "nexus-admin", description: "Administrador da plataforma (equivale a *)" },
      { name: "nexus-user", description: "Usuário da plataforma" },
    ],
  },
  groups: kcGroups,
  clients: [
    {
      clientId: "nexus-frontend",
      name: "Projeto Nexus — frontend (NextAuth)",
      enabled: true,
      publicClient: false,
      clientAuthenticatorType: "client-secret",
      secret: "${KEYCLOAK_FRONTEND_CLIENT_SECRET}",
      standardFlowEnabled: true,
      directAccessGrantsEnabled: false,
      serviceAccountsEnabled: false,
      redirectUris: ["${FRONTEND_URL}/*"],
      webOrigins: ["${FRONTEND_URL}"],
      attributes: {
        "post.logout.redirect.uris": "${FRONTEND_URL}/*",
        "pkce.code.challenge.method": "S256",
      },
      protocolMappers: [
        {
          // A API valida aud = nexus-backend (KEYCLOAK_AUDIENCE).
          name: "audience-nexus-backend",
          protocol: "openid-connect",
          protocolMapper: "oidc-audience-mapper",
          config: { "included.client.audience": "nexus-backend", "access.token.claim": "true", "id.token.claim": "false" },
        },
        {
          // "groups" só com o nome do grupo (não o caminho): é o que o IAM
          // cruza com ad_group_mappings / departamentos.ad_group.
          name: "groups",
          protocol: "openid-connect",
          protocolMapper: "oidc-group-membership-mapper",
          config: { "claim.name": "groups", "full.path": "false", "access.token.claim": "true", "id.token.claim": "true", "userinfo.token.claim": "true" },
        },
      ],
    },
    {
      // Audience dos tokens E client da reautenticação do Signum: a API
      // confere a senha na hora da assinatura com o grant "password"
      // (KEYCLOAK_CLIENT_ID/SECRET) — por isso confidencial, com Direct
      // Access Grants e nenhum outro fluxo.
      clientId: "nexus-backend",
      name: "Projeto Nexus — API (audience + reautenticação do Signum)",
      enabled: true,
      publicClient: false,
      clientAuthenticatorType: "client-secret",
      secret: "${KEYCLOAK_CLIENT_SECRET}",
      standardFlowEnabled: false,
      implicitFlowEnabled: false,
      serviceAccountsEnabled: false,
      directAccessGrantsEnabled: true,
    },
  ],
  users: users.map((u) => ({
    id: u.id,
    username: u.username,
    email: u.email,
    emailVerified: true,
    firstName: u.first,
    lastName: u.last,
    enabled: true,
    attributes: { matricula: [u.matricula], cargo: [u.cargo], unidade: [u.dep.unidade.nome], departamento: [u.dep.nome] },
    credentials: [{ type: "password", value: "${DEMO_USER_PASSWORD}", temporary: false }],
    realmRoles: ["nexus-user", ...u.roles],
    groups: u.groups.map((g) => groupPath.get(g)),
  })),
};

// --- SQL do Nexus -------------------------------------------------------

const sql = [];
sql.push(`-- GERADO por scripts/demo-data/generate.mjs — não edite à mão.
-- Dados FICTÍCIOS do ambiente de teste. Idempotente: pode rodar de novo.
-- Aplicar: make demo-seed
BEGIN;
`);

sql.push("-- Entidades");
for (const e of entidades) {
  sql.push(`INSERT INTO entidades (id, nome, sigla, slug) VALUES (${sqlUuid(e.id)}, ${sqlStr(e.nome)}, ${sqlStr(e.sigla)}, ${sqlStr(e.slug)})
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;`);
}

sql.push("\n-- Unidades");
for (const u of unidades) {
  sql.push(`INSERT INTO unidades (id, entidade_id, nome, sigla, slug, ad_group, email) VALUES (${sqlUuid(u.id)}, ${sqlUuid(u.entidade.id)}, ${sqlStr(u.nome)}, ${sqlStr(u.sigla)}, ${sqlStr(u.slug)}, ${sqlStr(u.code)}, ${sqlStr(`${ascii(u.code)}@${EMAIL_DOMAIN}`)})
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug, ad_group = EXCLUDED.ad_group, email = EXCLUDED.email;`);
}

sql.push("\n-- Departamentos");
for (const d of departamentos) {
  sql.push(`INSERT INTO departamentos (id, unidade_id, nome, sigla, slug, ad_group) VALUES (${sqlUuid(d.id)}, ${sqlUuid(d.unidade.id)}, ${sqlStr(d.nome)}, ${sqlStr(d.code)}, ${sqlStr(d.slug)}, ${sqlStr(d.group)})
  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug, ad_group = EXCLUDED.ad_group;`);
}

const mapping = (group, perfil, e, u, d, descricao) =>
  `INSERT INTO ad_group_mappings (ad_group, perfil_id, entidade_id, unidade_id, departamento_id, descricao, created_by)
  SELECT ${sqlStr(group)}, p.id, ${sqlUuid(e)}, ${sqlUuid(u)}, ${sqlUuid(d)}, ${sqlStr(descricao)}, 'demo-seed' FROM perfis p WHERE p.slug = ${sqlStr(perfil)}
  ON CONFLICT DO NOTHING;`;

sql.push("\n-- Mapeamentos grupo do Keycloak -> perfil + escopo");
for (const g of GRUPOS_PLATAFORMA) sql.push(mapping(g.group, g.perfil, null, null, null, g.descricao));
for (const d of departamentos) {
  const u = d.unidade;
  sql.push(mapping(d.group, "servidor", u.entidade.id, u.id, d.id, `Servidores de ${d.nome} — ${u.nome}`));
  // A Administração de cada unidade protocola e tramita processos da unidade.
  if (d.code === "ADM") sql.push(mapping(d.group, "protocolo", u.entidade.id, u.id, null, `Protocolo — ${u.nome}`));
}

sql.push("\n-- Usuários pré-provisionados (keycloak_subject = id no realm)");
for (const u of users) {
  sql.push(`INSERT INTO users (id, keycloak_subject, username, email, display_name, groups, active) VALUES (${sqlUuid(u.id)}, ${sqlStr(u.id)}, ${sqlStr(u.username)}, ${sqlStr(u.email)}, ${sqlStr(`${u.first} ${u.last}`)}, ${sqlArr(u.groups)}, true)
  ON CONFLICT (keycloak_subject) DO UPDATE SET username = EXCLUDED.username, email = EXCLUDED.email, display_name = EXCLUDED.display_name, groups = EXCLUDED.groups;`);
}

sql.push("\n-- Diretório: cargo, ramal e lotação exibida");
for (const u of users) {
  sql.push(`INSERT INTO directory_profiles (user_id, job_title, extension, unidade_id, departamento_id) SELECT id, ${sqlStr(u.cargo)}, ${sqlStr(u.ramal)}, ${sqlUuid(u.dep.unidade.id)}, ${sqlUuid(u.dep.id)} FROM users WHERE keycloak_subject = ${sqlStr(u.id)}
  ON CONFLICT (user_id) DO UPDATE SET job_title = EXCLUDED.job_title, extension = EXCLUDED.extension, unidade_id = EXCLUDED.unidade_id, departamento_id = EXCLUDED.departamento_id;`);
}
sql.push("\nCOMMIT;\n");

// --- CSV ----------------------------------------------------------------

const csvCell = (s) => (/[",;\n]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s);
const csv = [
  ["usuario", "nome", "email", "entidade", "unidade", "departamento", "cargo", "matricula", "ramal", "grupos"].join(";"),
  ...users.map((u) =>
    [u.username, `${u.first} ${u.last}`, u.email, u.dep.unidade.entidade.sigla, u.dep.unidade.nome, u.dep.nome, u.cargo, u.matricula, u.ramal, u.groups.join(" ")]
      .map(csvCell)
      .join(";"),
  ),
].join("\n");

// --- Escrita ------------------------------------------------------------

const write = (rel, content) => {
  const p = join(ROOT, rel);
  mkdirSync(dirname(p), { recursive: true });
  writeFileSync(p, content);
  console.log(`${rel} (${Buffer.byteLength(content)} bytes)`);
};
write("deploy/keycloak/realm-nexus.json", JSON.stringify(realm, null, 2) + "\n");
write("deploy/demo/seed-demo.sql", sql.join("\n"));
write("deploy/demo/usuarios.csv", "﻿" + csv + "\n");
console.log(`${entidades.length} entidades, ${unidades.length} unidades, ${departamentos.length} departamentos, ${users.length} usuários`);
