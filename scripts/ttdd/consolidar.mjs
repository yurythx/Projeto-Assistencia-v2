#!/usr/bin/env node
// Consolida a TTDD oficial (docs/ttdd.pdf) em deploy/ttdd/ttdd.json e
// deploy/ttdd/ttdd.sql (dados para deploy/ttdd/carga.sql: make ttdd-impacto e
// make ttdd-aplicar).
//
// Fontes:
//   - dados/extraida.json — páginas com texto (analisar.mjs);
//   - transcricao/        — páginas 51-73 do PDF, que são texto vetorizado
//     (sem camada de texto): listagem por OCR e prazos conferidos à mão
//     contra as páginas renderizadas;
//   - CORRECOES (abaixo)  — divergências do próprio documento, cada uma
//     registrada em `divergencias` no JSON gerado.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const AQUI = path.dirname(fileURLToPath(import.meta.url));
const OUT = path.resolve(AQUI, "../../deploy/ttdd");
const extraida = JSON.parse(fs.readFileSync(path.join(AQUI, "dados/extraida.json"), "utf8"));
const listagemOCR = JSON.parse(fs.readFileSync(path.join(AQUI, "transcricao/listagem-ocr.json"), "utf8"));

const divergencias = [];

// ---------------------------------------------------------------- órgãos
// Publicação de cada TTDD (Diário Oficial Eletrônico de Rondonópolis).
const ORGAOS = {
  "2.0": { nome: "Secretaria Municipal de Administração, Gestão de Pessoas e Inovação", edicao: "6.017", data: "2025-08-25", versao: "II" },
  "3.0": { nome: "Secretaria Municipal de Fazenda", edicao: "6.275", data: "2026-09-11", versao: "II" },
  "4.0": { nome: "Secretaria Municipal de Transparência Pública e Controle Interno", edicao: "6.275", data: "2026-09-11", versao: "II" },
  "6.0": { nome: "Secretaria Municipal de Habitação e Urbanismo", edicao: "6.275", data: "2026-09-11", versao: "II" },
  "7.0": { nome: "Secretaria Municipal de Infraestrutura", edicao: "6.275", data: "2026-09-11", versao: "II" },
  "8.0": { nome: "Secretaria Municipal de Meio Ambiente, Agricultura e Pecuária", edicao: "6.275", data: "2026-09-11", versao: "II" },
  "9.0": { nome: "Secretaria Municipal de Desenvolvimento Econômico", edicao: "6.055", data: "2025-10-16", versao: "II" },
  "10.0": { nome: "Secretaria Municipal de Promoção e Assistência Social", edicao: "6.275", data: "2026-09-11", versao: "II" },
  "12.0": { nome: "Secretaria Municipal de Saúde", edicao: "6.111", data: "2026-01-13", versao: "II" },
};
divergencias.push("Administração: o plano de classificação lista as subfunções 00 a 05 da Folha de Pagamento, mas a tabela traz itens 2.0.06.06.xx sob o título \"Encargos Sociais\" (o mesmo da 2.0.06.04) — mantidos como estão no documento.");
divergencias.push("Saúde: os cabeçalhos da tabela dizem \"19.0\", mas todos os códigos usam o prefixo 12.0 — adotado 12.0.");
divergencias.push("Pesquisa e Planejamento Urbano (13.0, edição 6.281): o PDF termina no primeiro item, sem os prazos — seção não importada.");

// ------------------------------------------------------ nomes legíveis
const SIGLAS = new Set(("SIC SADT SUS CMAS CRAS CREAS POP CONSEMMA DESOPEM PDI TCE MT GPE IPTU SMS CODIP SAMU UPA CAPS TI RH SCFV " +
  "PAEFI CADÚNICO SUAS OSC'S OSC CMDCA DSEI SIM SISVAN ESF UBS PSF CEO NASF IST HIV AIDS TB DST LGPD E-SOCIAL ISS ISSQN ITBI").split(" "));
const MINUS = new Set("de da do dos das e em a o à às ao aos no na nos nas com para por".split(" "));
function legivel(s) {
  if (s !== s.toUpperCase()) return s.replace(/\s+/g, " ").trim();
  return s.replace(/\s+/g, " ").trim().split(" ").map((w, i) => {
    const limpa = w.replace(/[(),]/g, "");
    if (SIGLAS.has(limpa) || /\d/.test(w)) return w;
    const b = w.toLowerCase();
    if (i > 0 && MINUS.has(b)) return b;
    return b.replace(/^(\(?)(\p{L})/u, (_, p, c) => p + c.toUpperCase());
  }).join(" ").replace(/ - /g, " – ");
}

// -------------------------------------------- funções e subfunções
const funcoes = {};
const subfuncoes = {};
// Só órgãos importados (o 5.0 é erro de digitação; o 13.0 está truncado no PDF).
const importado = (c) => Boolean(ORGAOS[c.split(".").slice(0, 2).join(".")]);
for (const [c, n] of Object.entries(extraida.funcoes)) if (importado(c)) funcoes[c] = legivel(n);
for (const [c, v] of Object.entries(extraida.subfuncoes)) if (importado(c)) subfuncoes[c] = { nome: legivel(v.nome), recomendacao: "" };
subfuncoes["3.0.01.00"].recomendacao =
  "A transferência para o Arquivo Permanente deve acontecer após a aprovação dos documentos pelo Tribunal de Contas do Mato Grosso.";

// Seções escaneadas (págs. 51-73).
Object.assign(funcoes, {
  "8.0.01": "Meio Ambiente", "8.0.02": "Agricultura e Pecuária",
  "9.0.01": "Desenvolvimento Econômico",
  "10.0.01": "Gestão em Promoção e Assistência Social", "10.0.02": "Conselho Municipal de Assistência Social – CMAS",
  "10.0.03": "Gestão de Conselhos Municipais (Idoso, Igualdade Racial, Criança e Adolescente, Mulher, Pessoa com Deficiência)",
  "10.0.04": "Conselho Tutelar", "10.0.05": "Casa Abrigo", "10.0.06": "CRAS", "10.0.07": "CREAS", "10.0.08": "Centro POP",
});
for (const [c, nome] of Object.entries({
  "8.0.01.00": "Fiscalização", "8.0.01.01": "Licenciamento Ambiental", "8.0.01.02": "Educação Ambiental e Urbanismo",
  "8.0.01.03": "Conselho Municipal do Meio Ambiente – CONSEMMA", "8.0.02.00": "Gestão Técnica e Social", "8.0.02.01": "Produção Animal",
  "8.0.02.02": "Produção Agrícola", "8.0.02.03": "Extensão Rural", "8.0.02.04": "Manutenção de Estradas",
  "9.0.01.00": "Gestão Administrativa", "9.0.01.01": "Comércio", "9.0.01.02": "Indústria",
  "10.0.01.00": "Legislação em Assistência Social", "10.0.02.00": "Gestão de OSC's – Organizações da Sociedade Civil",
  "10.0.03.00": "Coordenação Geral", "10.0.04.00": "Coordenação e Atendimento ao Público", "10.0.05.00": "Gestão Administrativa",
  "10.0.06.00": "Coordenação e Atendimento ao Público", "10.0.07.00": "Coordenação e Atendimento ao Público",
  "10.0.08.00": "Coordenação e Atendimento ao Público",
})) subfuncoes[c] = { nome, recomendacao: "" };

// ------------------------------------------------- itens (texto)
const itens = new Map();
for (const i of extraida.itens) {
  if (i.codigo.startsWith("13.")) continue;
  itens.set(i.codigo, { ...i });
}
function mover(de, para, motivo) {
  const it = itens.get(de);
  itens.delete(de);
  itens.set(para, { ...it, codigo: para });
  divergencias.push(motivo);
}
// Administração: a tabela de prazos pula o 2.0.01.00.11 (Manutenção Predial
// aparece como .12 e Ar Condicionado como .13); vale o plano de classificação.
{
  const predial = itens.get("2.0.01.00.12"), ar = itens.get("2.0.01.00.13");
  itens.set("2.0.01.00.11", { ...predial, codigo: "2.0.01.00.11", descritor: "Manutenção Predial" });
  itens.set("2.0.01.00.12", { ...ar, codigo: "2.0.01.00.12", descritor: "Manutenção de Ar Condicionado" });
  itens.delete("2.0.01.00.13");
  divergencias.push("Administração: a tabela de prazos numera Manutenção Predial/Ar Condicionado como 2.0.01.00.12/.13; o plano de classificação, como .11/.12 — adotado o plano.");
}
mover("5.0.05.02.03", "2.0.05.02.03", "Administração: \"5.0.05.02.03 Processo de Sindicância\" tem prefixo de outro órgão — corrigido para 2.0.05.02.03.");
for (const c of ["12.0.04.06.01", "12.0.07.04.13"])
  divergencias.push(`Saúde: o código ${c} é usado por duas séries (mensal e anual) — a segunda foi importada como ${c}-2.`);
{
  const it = itens.get("12.0.01.00.00");
  it.intermediaria = { anos: null, texto: "" };
}
{
  const it = itens.get("12.0.04.01.00"); // "Permanente" sem prazos de fase
  it.corrente = { anos: null, texto: "" };
  it.intermediaria = { anos: null, texto: "" };
  it.destinacao = "GUARDA_PERMANENTE";
  it.observacoes = "Guarda permanente desde a produção (a TTDD não fixa prazos de fase).";
}
for (const it of itens.values()) {
  if (it.destinacao === "" && /^X\b/.test(it.observacoes)) {
    it.observacoes = it.observacoes.replace(/^X\s*/, "");
    it.destinacao = null; // "X": a TTDD não define destinação
  }
  it.observacoes = it.observacoes.replace(/^(Eliminação|Guarda permanente)$/i, "");
  const rep = it.observacoes.match(/^(.+?) \1$/);
  if (rep) it.observacoes = rep[1];
}
divergencias.push("Saúde: 38 séries de Relatórios da Atenção Básica (12.0.02.01.xx) estão com a fase corrente em branco no documento — importadas como \"não informado\".");
divergencias.push("Saúde: 6 séries de Análises Clínicas/Imagem marcam a destinação com \"X\" (\"entregue ao paciente após transcrição no prontuário\") — importadas sem destinação definida.");

// ---------------------------------------- itens (páginas escaneadas)
const DESC_OCR_FIX = {
  "8.0.02.01.03": "Auto/Termo do SIM – Serviço de Inspeção Municipal",
  "10.0.04.00.06": "Convite de Comparecimento Individual – Reiteração",
  "10.0.04.00.11": "Denúncias de crimes encaminhamento à Delegacia Local Especializada de Segurança Pública",
  "10.0.04.00.40": "Requisição de Serviço Público – Reiteração",
  "10.0.06.00.16": "Ficha de Inscrição para o Grupo dos Idosos/intergeracional",
  // Linhas de continuação que o OCR perdeu (conferidas nas páginas 59-73).
  "10.0.02.00.08": "Processo de aprovação do Plano de Ação da Secretaria Municipal de Promoção e Assistência Social – SUASWEB",
  "10.0.04.00.51": "Termo de Responsabilidade de Adolescente que se auto-exclui do Ensino Médio – Reiteração",
  "10.0.06.00.19": "Ficha de inscrição de crianças de 7 a 9 anos de idade para o SCFVC – Serviço de Convivência e Fortalecimento de Vínculos",
  "10.0.06.00.20": "Ficha de inscrição de crianças de 10 a 12 anos de idade para o SCFVC – Serviço de Convivência e Fortalecimento de Vínculos",
  "10.0.06.00.21": "Ficha de inscrição de crianças de 13 a 17 anos de idade para o SCFVC – Serviço de Convivência e Fortalecimento de Vínculos",
  "10.0.06.00.22": "Ficha de inscrição de adultos de 18 a 59 anos para o SCFVC – Serviço de Convivência e Fortalecimento de Vínculos",
  "10.0.06.00.23": "Ficha de inscrição de idosos para o SCFVC – Serviço de Convivência e Fortalecimento de Vínculos",
  "10.0.07.00.09": "Termo de Denúncia ao Ministério Público Estadual/Promotoria de Justiça/Vara da Infância e Juventude",
};
const limpaOCR = (s) => s.replace(/\s+[lI|(]$/g, "").replace(/\s+—\s+/g, " – ").replace(/\s+/g, " ").trim();
const descritorOCR = (c) => DESC_OCR_FIX[c] ?? limpaOCR(listagemOCR[c] ?? "");
const DEST = { G: "GUARDA_PERMANENTE", E: "ELIMINACAO" };
const fase = (v) => (v === "-" ? { anos: null, texto: "" } : v === "EV" ? { anos: null, texto: "Enquanto estiver vigorando" } : { anos: Number(v), texto: "" });
const linhas = (f) => fs.readFileSync(path.join(AQUI, "transcricao", f), "utf8").split("\n").filter((l) => l.trim() && !l.startsWith("#"));

for (const f of ["prazos-8.tsv", "prazos-9.tsv"]) {
  for (const l of linhas(f)) {
    const [codigo, c, i, d] = l.split("\t");
    itens.set(codigo, { codigo, descritor: descritorOCR(codigo), corrente: fase(c), intermediaria: fase(i), destinacao: DEST[d], observacoes: "" });
  }
}
// SEMPRAS: o nº do item segue a ordem do plano de classificação.
const codigos10 = [...new Set([...Object.keys(listagemOCR).filter((c) => c.startsWith("10.")), ...Object.keys(DESC_OCR_FIX).filter((c) => c.startsWith("10."))])]
  .sort((a, b) => a.split(".").map(Number).join(".").localeCompare(b.split(".").map(Number).join("."), undefined, { numeric: true }));
const prazos10 = linhas("prazos-10.txt");
if (codigos10.length !== prazos10.length) throw new Error(`SEMPRAS: ${codigos10.length} códigos x ${prazos10.length} linhas de prazo`);
prazos10.forEach((l, k) => {
  const [n, c, i, d] = l.split(/\s+/);
  if (Number(n) !== k + 1) throw new Error(`SEMPRAS: item ${n} fora de ordem`);
  const codigo = codigos10[k];
  itens.set(codigo, { codigo, descritor: descritorOCR(codigo), corrente: fase(c), intermediaria: fase(i), destinacao: DEST[d], observacoes: "" });
});

// ------------------------------------------------------- verificação
const saida = [...itens.values()].map((i) => {
  const base = i.codigo.replace(/-2$/, "");
  const sub = base.split(".").slice(0, 4).join(".");
  if (!subfuncoes[sub]) throw new Error(`subfunção sem nome: ${sub} (${i.codigo})`);
  if (!i.descritor) throw new Error(`sem descritor: ${i.codigo}`);
  return {
    codigo: i.codigo, subfuncao: sub, descritor: i.descritor.replace(/\s+/g, " ").trim(),
    corrente_anos: i.corrente.anos, corrente_condicao: i.corrente.texto,
    intermediaria_anos: i.intermediaria.anos, intermediaria_condicao: i.intermediaria.texto,
    destinacao: i.destinacao || null, observacoes: i.observacoes || "",
  };
}).sort((a, b) => a.codigo.localeCompare(b.codigo, undefined, { numeric: true }));
for (const s of Object.keys(subfuncoes)) if (!funcoes[s.split(".").slice(0, 3).join(".")]) throw new Error(`função sem nome: ${s}`);

// ------------------------------------------------------------- saída
const q = (v) => (v === null || v === undefined ? "NULL" : typeof v === "number" ? String(v) : `'${String(v).replace(/'/g, "''")}'`);
// Só os dados, em tabelas temporárias da sessão: quem compara com o banco e
// grava é deploy/ttdd/carga.sql (make ttdd-impacto / make ttdd-aplicar).
const sql = [
  "-- GERADO por scripts/ttdd/consolidar.mjs a partir de docs/ttdd.pdf — não edite à mão.",
  "-- TTDD oficial (CCPAD / Diário Oficial de Rondonópolis) em tabelas temporárias;",
  "-- deploy/ttdd/carga.sql mostra o impacto e grava (make ttdd-impacto / make ttdd-aplicar).",
  "CREATE TEMP TABLE carga_orgaos (prefixo TEXT PRIMARY KEY, nome TEXT NOT NULL, edicao_diario TEXT NOT NULL, data_publicacao DATE, versao TEXT NOT NULL);",
  "CREATE TEMP TABLE carga_funcoes (codigo TEXT PRIMARY KEY, orgao_prefixo TEXT NOT NULL, nome TEXT NOT NULL);",
  "CREATE TEMP TABLE carga_subfuncoes (codigo TEXT PRIMARY KEY, funcao_codigo TEXT NOT NULL, nome TEXT NOT NULL, recomendacao TEXT NOT NULL);",
  "CREATE TEMP TABLE carga_series (codigo TEXT PRIMARY KEY, subfuncao_codigo TEXT NOT NULL, descritor TEXT NOT NULL, fase_corrente_anos INT,",
  "  fase_corrente_condicao TEXT NOT NULL, fase_interm_anos INT, fase_interm_condicao TEXT NOT NULL, destinacao_final TEXT, observacoes TEXT NOT NULL);",
  "",
];
const valores = (tabela, linhas) => {
  for (let i = 0; i < linhas.length; i += 200)
    sql.push(`INSERT INTO ${tabela} VALUES\n  ${linhas.slice(i, i + 200).map((l) => `(${l.map(q).join(", ")})`).join(",\n  ")};`);
};
valores("carga_orgaos", Object.entries(ORGAOS).map(([p, o]) => [p, o.nome, o.edicao, o.data, o.versao]));
valores("carga_funcoes", Object.entries(funcoes).map(([c, n]) => [c, c.split(".").slice(0, 2).join("."), n]));
valores("carga_subfuncoes", Object.entries(subfuncoes).map(([c, s]) => [c, c.split(".").slice(0, 3).join("."), s.nome, s.recomendacao]));
valores("carga_series", saida.map((i) => [i.codigo, i.subfuncao, i.descritor, i.corrente_anos, i.corrente_condicao,
  i.intermediaria_anos, i.intermediaria_condicao, i.destinacao, i.observacoes]));
sql.push("");

fs.mkdirSync(OUT, { recursive: true });
fs.writeFileSync(path.join(OUT, "ttdd.json"), JSON.stringify({ fonte: "docs/ttdd.pdf", orgaos: ORGAOS, funcoes, subfuncoes, itens: saida, divergencias }, null, 1) + "\n");
fs.writeFileSync(path.join(OUT, "ttdd.sql"), sql.join("\n"));
const por = {};
for (const i of saida) { const o = i.codigo.split(".").slice(0, 2).join("."); por[o] = (por[o] ?? 0) + 1; }
console.log(`itens=${saida.length} funções=${Object.keys(funcoes).length} subfunções=${Object.keys(subfuncoes).length}`, JSON.stringify(por));
console.log(`sem destinação=${saida.filter((i) => !i.destinacao).length} corrente não informada=${saida.filter((i) => i.corrente_anos === null && !i.corrente_condicao).length}`);
console.log(divergencias.map((d) => "  - " + d).join("\n"));
