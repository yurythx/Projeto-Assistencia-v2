#!/usr/bin/env node
// Converte o export do Active Directory (OUs por secretaria, TSV em
// ISO-8859-1 "Nome<TAB>Tipo<TAB>Descrição") na estrutura organizacional do
// Nexus: Entidade (órgão) > Unidade (local de trabalho, com unidade-mãe) >
// Departamento (setor da unidade) — docs/REGRAS_DE_NEGOCIO.md §6.
//
//   node scripts/estrutura/ad-para-estrutura.mjs <pasta-do-export>
//
// Gera deploy/estrutura/estrutura.json (revisão) e estrutura.sql (carga
// idempotente, IDs determinísticos). Contas de usuário do export são
// IGNORADAS (pessoas vêm do login pelo AD/Keycloak, não da carga) e os
// grupos de segurança só são listados — viram mapeamentos depois.
//
// Decisões aplicadas (revisão de 2026-10-02):
//   - "Ação Social" e "Sempras" = Promoção e Assistência Social; "SECITI" =
//     Ciência, Tecnologia e Inovação;
//   - turnos estendidos ("(3º Turno)") viram departamento da unidade-base;
//   - órgão sem detalhe no AD ganha uma unidade "Sede";
//   - a sede de cada órgão leva a sigla do órgão (não "SEDE"): os avisos do
//     Atlas chegam à unidade pela sigla da etapa ("SEGEP/RH" → SEGEP). As
//     siglas abaixo são provisórias até a confirmação nas entrevistas
//     (docs/atlas/PLANO_DE_VALIDACAO.md §7).
//   - repetições dentro da Saúde ficam uma vez só (unidade, não setor).
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SRC = process.argv[2];
if (!SRC) {
  console.error("uso: ad-para-estrutura.mjs <pasta-do-export-do-AD>");
  process.exit(2);
}
const OUT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../deploy/estrutura");

// ------------------------------------------------------------ leitura
const pessoasIgnoradas = { total: 0 };
const gruposAD = [];
function ler(rel, entidade) {
  const txt = new TextDecoder("latin1").decode(fs.readFileSync(path.join(SRC, rel))).replace(/\r/g, "");
  const ous = [];
  for (const linha of txt.split("\n")) {
    const [nome = "", tipo = ""] = linha.split("\t").map((s) => s.trim());
    if (!nome || nome === "Nome") continue;
    if (tipo === "Unidade organizacional") ous.push(nome);
    else if (tipo.startsWith("Grupo de segurança")) gruposAD.push({ entidade, grupo: nome });
    else if (tipo === "Usuário") pessoasIgnoradas.total++;
  }
  return ous;
}

// ------------------------------------------------------- normalização
const SIGLAS = new Set(("CMEI CMEITI EMEB EMEBTI EMEF EMEI UMEI EMCEB EIMEB ESF UPA SAMU CAPS AD SAE UVZ CAISM CEADAS CERARO " +
  "CRAS CREAS SCFV CEU DTI SEMED SMS MAMED CAIC COHAB PA UTI CPAC SUS RH IPPUR SINFRA SECITI PROCON CEO VIVA POP N.S. II I SEMPRAS").split(" "));
const MINUS = new Set("de da do dos das e em a o à às ao".split(" "));
const CORRIGE = { "Sáude": "Saúde", Juridico: "Jurídico", Juridica: "Jurídica", Manutencao: "Manutenção", Basico: "Básica",
  Unico: "Único", Assistencia: "Assistência", Familia: "Família", Olimpica: "Olímpica", Nucleo: "Núcleo", Gestao: "Gestão",
  Publica: "Pública", Administracao: "Administração", Policlinica: "Policlínica", Farmacia: "Farmácia", Musica: "Música" };

function titulo(raw) {
  return raw.replace(/\s+/g, " ").trim().split(" ").map((w, i) => {
    const up = w.toUpperCase();
    if (SIGLAS.has(up)) return up;
    if (/^\(?\d/.test(w)) return w.replace("ª", "º");
    const base = w.toLowerCase();
    if (i > 0 && MINUS.has(base)) return base;
    const t = (base.charAt(0).toUpperCase() + base.slice(1)).replace(/^Prof$/, "Prof.").replace(/'(\p{L})/u, (_, c) => "'" + c.toUpperCase());
    return CORRIGE[t] ?? t;
  }).join(" ").replace(/\(3º turno\)/i, "(3º Turno)");
}
const sem = (s) => s.normalize("NFD").replace(/\p{Mn}/gu, "");
const slug = (s) => sem(s).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "").slice(0, 80);
const TURNO = /\s*\(3º Turno\)$/;

// UUID v5 (determinístico): rodar de novo atualiza em vez de duplicar.
const NS = Buffer.from("6c2a1e2e5b9d4f0a9a8f3d1c7e4b2a90", "hex");
function uuid5(nome) {
  const h = crypto.createHash("sha1").update(Buffer.concat([NS, Buffer.from(nome, "utf8")])).digest();
  h[6] = (h[6] & 0x0f) | 0x50;
  h[8] = (h[8] & 0x3f) | 0x80;
  const x = h.subarray(0, 16).toString("hex");
  return `${x.slice(0, 8)}-${x.slice(8, 12)}-${x.slice(12, 16)}-${x.slice(16, 20)}-${x.slice(20)}`;
}

// ------------------------------------------------------- construção
const entidades = [];
const departamentos = (nomes) => nomes.map((n) => ({ nome: titulo(n) }));

// Folhas de um grupo: prefixo opcional (evita "Itamaraty" repetido entre CEO,
// ESF...), e "(3º Turno)" vira departamento da unidade-base.
function folhas(nomes, prefixo = "") {
  const out = [];
  for (const raw of nomes) {
    const n = titulo(raw);
    const base = n.replace(TURNO, "");
    const nome = prefixo && !base.toUpperCase().startsWith(prefixo.toUpperCase()) ? `${prefixo} ${base}` : base;
    let u = out.find((x) => x.nome === nome);
    if (!u) out.push((u = { nome, departamentos: [] }));
    if (TURNO.test(n)) u.departamentos.push({ nome: "3º Turno" });
  }
  return out;
}

// Prefeitura: cada OU é um órgão (entidade).
const DETALHADO = { "Educação": true, "Saúde": true, "Promoção e Assistencia Social": true };
const MESMO_QUE = { "Ação Social": "Promoção e Assistencia Social", Sempras: "Promoção e Assistencia Social", SECITI: "Ciência Tecnologia e Inovação" };
const SIGLA_ORGAO = {
  "Ciência Tecnologia e Inovação": "SECITI", Sinfra: "SINFRA", "IPPUR - SINFRA": "IPPUR", Procon: "PROCON",
  "Administração": "SEMAD", "Agricultura e Pecuária": "SEMAGRI", Cultura: "SECULT", "Desenvolvimento Econômico": "SEDEC",
  "Esporte e Lazer": "SEMEL", "Finanças": "SEFIN", "Gabinete Comunicação": "GAB", "Gestao de Pessoas": "SEGEP",
  Governo: "SEGOV", "Habitação": "SEHAB", "Meio Ambiente": "SEMMA", "Pesquisa e Planejamento Urbano": "SEPPU",
  Planejamento: "SEPLAN", "Procuradoria Geral": "PGM", Receita: "SEREC", "Segurança Publica": "SESP",
  "Transportes e Trânsito": "SETRAT", "Unidade Central de Controle Interno": "UCCI",
};
for (const o of ler("Prefeitura/prefeitura.txt", "PREF")) {
  if (MESMO_QUE[o] || DETALHADO[o]) continue;
  const nome = o === "Ciência Tecnologia e Inovação" ? "Ciência, Tecnologia e Inovação" : titulo(o);
  const sigla = SIGLA_ORGAO[o] ?? "";
  // slug pelo nome, como antes de o órgão ganhar sigla.
  entidades.push({ nome, sigla, slug: slug(nome), unidades: [{ nome: "Sede", sigla, departamentos: [] }] });
}

// Educação
const E = "Secretaria de Educação";
ler(`${E}/educação.txt`, "SEMED");
entidades.push({
  nome: "Secretaria Municipal de Educação", sigla: "SEMED",
  unidades: [
    { nome: "Secretaria Executiva", sigla: "SEMED", departamentos: departamentos(ler(`${E}/secretaria executiva educação.txt`, "SEMED")) },
    { nome: "Escola de Música", departamentos: [] },
    { nome: "Educação Infantil", agrupadora: true, subunidades: folhas(ler(`${E}/unidades educação infantil.txt`, "SEMED")) },
    { nome: "Ensino Fundamental", agrupadora: true, subunidades: folhas(ler(`${E}/unidades ensino fundamental.txt`, "SEMED")) },
    { nome: "Escolas Rurais", agrupadora: true, subunidades: folhas(ler(`${E}/escolas rurais.txt`, "SEMED")) },
  ],
});

// Saúde
const S = "Secretaria de Saude";
ler(`${S}/unidades de saude.txt`, "SMS");
const grupos = [
  ["Centros de Especialidades Odontológicas", "CEO", "C.E.O..txt", "CEO"],
  ["Centros de Saúde", "CS", "C.S", "CS"],
  ["Estratégia Saúde da Família", "ESF", "ESF.txt", "ESF"],
  ["Especialidades", "", "especialidades.txt", ""],
  ["Farmácias", "", "Farmacias.txt", ""],
  ["Hospitais e Pronto Atendimento", "", "hospitais.txt", ""],
  ["Policlínicas", "", "policlinicas.txt", ""],
  ["Zona Rural", "", "zona rural.txt", ""],
].map(([nome, sigla, f, prefixo]) => ({ nome, sigla, agrupadora: true, subunidades: folhas(ler(`${S}/${f}`, "SMS"), prefixo) }));
const nasFolhas = new Set(grupos.flatMap((g) => g.subunidades.map((u) => slug(u.nome))));
const raiz = ler(`${S}/saude.txt`, "SMS").filter((o) => !["SMS", "UNIDADES DE SAÚDE"].includes(o)).map(titulo);
// Setores da sede que são, na verdade, unidades (UPA, Policlínica Central) ou
// que repetem uma unidade da raiz (Conselho Municipal, Laboratório Central)
// ficam só como unidade.
const raizSlugs = new Set(raiz.map(slug));
const setoresSaude = ler(`${S}/SMS.txt`, "SMS").map(titulo)
  .filter((n) => !nasFolhas.has(slug(n)) && !["upa"].includes(slug(n)) && !raizSlugs.has(slug(n)));
entidades.push({
  nome: "Secretaria Municipal de Saúde", sigla: "SMS",
  unidades: [
    { nome: "Sede da SMS", sigla: "SMS", departamentos: setoresSaude.map((nome) => ({ nome })) },
    ...raiz.map((nome) => ({ nome, departamentos: [] })),
    ...grupos,
  ],
});

// Promoção e Assistência Social
const SETORES_SEMPRAS = new Set(["Administrativo", "Cadastro Unico", "Compras", "Engenharia", "Financeiro", "Juridico",
  "Nucleo de Conselhos", "Proteção Basico", "Proteção Especial"]);
const ousA = ler("Secretaria de promoção e Assistencia social/promoção e assistencia social.txt", "SEMPRAS");
entidades.push({
  nome: "Secretaria Municipal de Promoção e Assistência Social", sigla: "SEMPRAS",
  unidades: [
    { nome: "Sede da SEMPRAS", sigla: "SEMPRAS", departamentos: departamentos(ousA.filter((o) => SETORES_SEMPRAS.has(o)))
      .map((d) => ({ nome: d.nome.replace("Proteção Básica", "Proteção Social Básica").replace("Proteção Especial", "Proteção Social Especial") })) },
    ...folhas(ousA.filter((o) => !SETORES_SEMPRAS.has(o))),
  ],
});

// ------------------------------------------------------------ saída
const q = (s) => `'${String(s).replace(/'/g, "''")}'`;
const sql = ["-- GERADO por scripts/estrutura/ad-para-estrutura.mjs a partir do export do AD — não edite à mão.",
  "-- Estrutura organizacional real (entidades, unidades, departamentos). Idempotente.",
  "-- Aplicar: make estrutura-aplicar", "BEGIN;", ""];
let nU = 0, nD = 0;
const vistos = new Set();
function unidade(ent, entId, u, parentId) {
  const s = slug(u.nome);
  if (vistos.has(`${entId}/${s}`)) throw new Error(`slug repetido na entidade ${ent.nome}: ${s}`);
  vistos.add(`${entId}/${s}`);
  const id = uuid5(`unidade:${ent.nome}:${s}`);
  nU++;
  sql.push(`INSERT INTO unidades (id, entidade_id, parent_id, nome, sigla, slug) VALUES (${q(id)}, ${q(entId)}, ${parentId ? q(parentId) : "NULL"}, ${q(u.nome)}, ${q(u.sigla ?? "")}, ${q(s)})`,
    "  ON CONFLICT (id) DO UPDATE SET parent_id = EXCLUDED.parent_id, nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;");
  for (const d of u.departamentos ?? []) {
    nD++;
    sql.push(`INSERT INTO departamentos (id, unidade_id, nome, slug) VALUES (${q(uuid5(`departamento:${id}:${slug(d.nome)}`))}, ${q(id)}, ${q(d.nome)}, ${q(slug(d.nome))})`,
      "  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, slug = EXCLUDED.slug;");
  }
  for (const f of u.subunidades ?? []) unidade(ent, entId, f, id);
}
for (const e of entidades) {
  const id = uuid5(`entidade:${slug(e.nome)}`);
  sql.push(`-- ${e.nome}`, `INSERT INTO entidades (id, nome, sigla, slug) VALUES (${q(id)}, ${q(e.nome)}, ${q(e.sigla)}, ${q(e.slug ?? slug(e.sigla || e.nome))})`,
    "  ON CONFLICT (id) DO UPDATE SET nome = EXCLUDED.nome, sigla = EXCLUDED.sigla, slug = EXCLUDED.slug;");
  for (const u of e.unidades) unidade(e, id, u, null);
  sql.push("");
}
sql.push("COMMIT;", "");

fs.mkdirSync(OUT, { recursive: true });
fs.writeFileSync(path.join(OUT, "estrutura.json"), JSON.stringify({ gerado_por: "scripts/estrutura/ad-para-estrutura.mjs", entidades, grupos_ad: gruposAD }, null, 2) + "\n");
fs.writeFileSync(path.join(OUT, "estrutura.sql"), sql.join("\n"));
console.log(`entidades=${entidades.length} unidades=${nU} departamentos=${nD} grupos_ad=${gruposAD.length} contas_ignoradas=${pessoasIgnoradas.total}`);
