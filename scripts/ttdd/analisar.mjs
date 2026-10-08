// Extrai a TTDD do Diário Oficial (ttdd.pdf) para dados estruturados:
// órgão > função > subfunção > série documental, com prazos, destinação,
// observações e recomendações. Entrada: itens do pdf.js com coordenadas.
import fs from "node:fs";
import { linhas } from "./linhas.mjs";

const items = JSON.parse(fs.readFileSync(process.argv[2] ?? "items.json", "utf8"));
const LS = linhas(items);

const CODE = /^(\d{1,2}\.0\.\d{2}\.\d{2}\.\d{2})\s*(.*)$/;
const norm = (s) => s.replace(/\s+/g, " ").replace(/\s*[–—]\s*/g, " – ").trim();
const ruido = /DIÁRIO OFICIAL|EDIÇÃO Nº|Av\. Duque de Caxias|^Código de Classificação|^Item documental|^Prazo de Arquivamento|^Nº( do)?$|^do$|^Item$|^Arquivo|^Corrente|^Intermedi|^Destinação|^final$|^Observaç/i;

// ---------------------------------------------------------- estado global
const orgaos = new Map(); // prefixo -> {nome, fonte}
const funcoes = new Map(); // "12.0.01" -> nome
const subfuncoes = new Map(); // "12.0.01.00" -> {nome, recomendacao}
const listagem = new Map(); // código -> descritor (séries documentais)
const tabela = new Map(); // código -> {descritor, corrente, intermediaria, destinacao, observacoes, pagina}
const anomalias = [];

let modo = "capa";
let secao = { nome: "", edicao: "", data: "", versao: "" };
let funcaoNome = "", subNome = "", subNum = "";
let cols = null; // limites de coluna da tabela vigente
let recomendacao = null;
let orgaoLinha = "";
let funcaoCod = "";
const planoFuncoes = new Map(), planoSub = new Map();

const edicaoRe = /EDIÇÃO Nº\s*([\d.]+),\s*(\d{1,2}) DE ([A-ZÇ]+) DE (\d{4})/;
const meses = { JANEIRO: 1, FEVEREIRO: 2, MARÇO: 3, ABRIL: 4, MAIO: 5, JUNHO: 6, JULHO: 7, AGOSTO: 8, SETEMBRO: 9, OUTUBRO: 10, NOVEMBRO: 11, DEZEMBRO: 12 };

// Uma página por vez: primeiro os cabeçalhos/estrutura, depois as linhas da
// tabela (atribuição por posição).
const paginas = [...new Set(LS.map((l) => l.p))];
let ultimoItem = null;

for (const p of paginas) {
  const ls = LS.filter((l) => l.p === p);
  const texto = (l) => norm(l.toks.map((t) => t.s).join(" "));

  // Edição da página.
  for (const l of ls) {
    const m = texto(l).match(edicaoRe);
    if (m) secao.edicao = m[1], secao.data = `${m[4]}-${String(meses[m[3]] ?? 0).padStart(2, "0")}-${m[2].padStart(2, "0")}`;
  }

  // Cabeçalho de colunas da tabela (se houver nesta página).
  const achaX = (re) => { for (const l of ls) for (const t of l.toks) if (re.test(t.s.trim())) return t.x; return null; };
  const xI = achaX(/^Intermedi/), xD = achaX(/^Destina/), xO = achaX(/^Observa/), xC = achaX(/Corrente/);
  if (xI && xD) cols = { C: xC ?? xI - 60, I: xI, D: xD, O: xO ?? xD + 60 };

  const linhasTabela = [];
  for (let k = 0; k < ls.length; k++) {
    const l = ls[k];
    const t = texto(l);
    if (/^TABELA DE TEMPORALIDADE/.test(t)) { if (modo === "series") modo = "tabela"; else modo = "capa"; continue; }
    if (/^CÓDIGO DE CLASSIFICAÇÃO DE DOCUMENTOS/.test(t)) { modo = "classificacao"; continue; }
    if (/^SÉRIES DOCUMENTAIS/.test(t)) { modo = "series"; continue; }
    if (/Prazo de Arquivamento/.test(t)) { modo = "tabela"; }
    if (modo === "capa") {
      const s = t.match(/^(?:DA\s+)?(SECRETARIA MUNICIPAL DE .+)$/);
      if (s && !/ARQUIVO|ADMINISTRAÇÃO, GESTÃO DE PESSOAS E$/.test(t)) secao.nome = s[1];
      const v = t.match(/^VERSÃO\s+([IVX]+)/); if (v) secao.versao = v[1];
      continue;
    }
    // Órgão: "2.0 SECRETARIA ..." / "19.0 Secretaria ..."
    const o = t.match(/^(\d{1,2})\.0\s+(Secretaria .+|SECRETARIA .+)$/i);
    if (o) { orgaoLinha = norm(o[2]); continue; }
    const f = t.match(/^FUN[ÇC][ÃA]O:\s*(\d{1,2}\.0\.\d{2})\.?\s*[–-]?\s*(.+)$/i);
    if (f) { funcaoNome = norm(f[2]); funcaoCod = f[1]; if (modo === "classificacao" && !planoFuncoes.has(f[1])) planoFuncoes.set(f[1], funcaoNome); continue; }
    const sf = t.match(/^SUBFUN[ÇC][ÃA]O:\s*(\d{2})\s*[–-]?\s*(.+)$/i);
    if (sf) { subNum = sf[1]; subNome = norm(sf[2]); recomendacao = null; if (modo === "classificacao" && funcaoCod) planoSub.set(`${funcaoCod}.${sf[1]}`, subNome); continue; }
    const rec = t.match(/recomendação:\s*(.*)$/i);
    if (rec) { recomendacao = { texto: rec[1], sub: subNum }; continue; }
    if (recomendacao && modo === "tabela" && !CODE.test(t) && !ruido.test(t) && /^[A-ZÁÉ]/.test(l.toks[0].s) && l.toks.length === 1 && l.toks[0].x < 120) {
      recomendacao.texto += " " + t; continue;
    }

    if (modo === "series") {
      // "  01 2.0.01.00.00 Descritor" (+ linhas de continuação)
      const toks = l.toks.filter((x) => !/^\d{1,3}$/.test(x.s.trim()) || x.x > 100);
      const lt = norm(toks.map((x) => x.s).join(" "));
      const m = lt.match(CODE);
      // Código repetido no próprio documento: o segundo vira "<código>-2".
      if (m) { const k = listagem.has(m[1]) ? `${m[1]}-2` : m[1]; listagem.set(k, m[2]); ultimoItem = k; }
      else if (ultimoItem && lt && !/^d+$/.test(lt) && !ruido.test(lt) && !/^(FUNÇÕES|SUBFUNÇÕES|Função|Subfunção)/i.test(lt) && l.toks[0].x > 100)
        listagem.set(ultimoItem, `${listagem.get(ultimoItem)} ${lt}`);
      continue;
    }
    if (modo === "tabela" && cols && !/^(DIÁRIO OFICIAL|EDIÇÃO Nº|Av. Duque)/.test(t)) linhasTabela.push(l);
  }

  // ------------------------------------------------ tabela desta página
  if (!linhasTabela.length) continue;
  const { C, I, D, O } = cols;
  const pareceValor = (t) => /^(Enquanto|d+s*(anos?|dias?|m[eê]s)|[-–]$|vigorando|vigente|estiver)/i.test(t.trim());
  const col = (x, t = "") => (x < (pareceValor(t) ? C - 24 : C - 4) ? "desc" : x < I - 6 ? "corr" : x < D - 12 ? "int" : x < O - 8 ? "dest" : "obs");
  const linhasItem = []; // {codigo, y0, linhas desc, centro}
  const frag = { corr: [], int: [], dest: [], obs: [] };
  const numeros = [];
  for (const l of linhasTabela) {
    const porCol = { desc: [], corr: [], int: [], dest: [], obs: [] };
    for (const t of l.toks) {
      if (/^\d{1,3}$/.test(t.s.trim()) && t.x < 80) { numeros.push(l.y); continue; }
      porCol[col(t.x, t.s)].push(t.s);
    }
    const d = norm(porCol.desc.join(" "));
    const m = d.match(CODE);
    if (m) linhasItem.push({ codigo: m[1], desc: [m[2]], ys: [l.y] });
    else if (d && linhasItem.length && !ruido.test(d) && !/^(\d{1,2}\.0\s|Função|Subfunção|FUNÇÃO|SUBFUNÇÃO)/i.test(d)) {
      linhasItem.at(-1).desc.push(d); linhasItem.at(-1).ys.push(l.y);
    } else if (d && !linhasItem.length && ultimoItem && tabela.has(ultimoItem) && !ruido.test(d) && !/^(\d{1,2}\.0\s|Função|Subfunção)/i.test(d)) {
      tabela.get(ultimoItem).descritor += " " + d; // descritor que quebrou de página
    }
    for (const c of ["corr", "int", "dest", "obs"]) porCol[c] = porCol[c].filter((x) => !/^(Destinação|final|Arquivo|Corrente|Intermedi|Observaç|Prazo de)/i.test(x.trim()));
    for (const c of ["corr", "int", "dest", "obs"]) if (porCol[c].length) frag[c].push({ y: l.y, s: norm(porCol[c].join(" ")) });
  }
  // Centro de cada linha da tabela: o nº do item (centralizado) mais próximo
  // da faixa do descritor; sem nº, o meio do descritor.
  for (let k = 0; k < linhasItem.length; k++) {
    const it = linhasItem[k];
    const lo = it.ys[0] - 6, hi = (linhasItem[k + 1]?.ys[0] ?? Infinity) - 1;
    const n = numeros.filter((y) => y >= lo && y <= hi);
    it.centro = n.length ? n[0] : (it.ys[0] + it.ys.at(-1)) / 2;
  }
  // Valores: gramática curta para prazos e destinação; observação livre.
  const inicioPrazo = /^(\d+\s*(anos?|dias?|m[eê]s(es)?)|Enquanto|-|–|Permanente|At[ée]\s|Após|Ap[oó]s|Indeterminado|Vide|Conforme)/i;
  function valores(fr, novo) {
    const out = [];
    for (const f of fr) {
      const prev = out.at(-1);
      if (!prev || novo(f.s) || f.y - prev.y1 > 14) out.push({ s: f.s, y0: f.y, y1: f.y });
      else { prev.s += " " + f.s; prev.y1 = f.y; }
    }
    return out.map((v) => ({ ...v, c: (v.y0 + v.y1) / 2 }));
  }
  const vals = {
    corr: valores(frag.corr, (s) => inicioPrazo.test(s)),
    int: valores(frag.int, (s) => inicioPrazo.test(s)),
    dest: valores(frag.dest, (s) => /^(Elimina|Guarda)/i.test(s)),
    obs: valores(frag.obs, () => false),
  };
  const regs = linhasItem.map((it) => {
    const r = { codigo: it.codigo, descritor: norm(it.desc.join(" ")), corrente: [], intermediaria: [], destinacao: [], observacoes: [], pagina: p,
      funcao: funcaoNome, subfuncao: subNome, centro: it.centro };
    return r;
  });
  const alvo = { corr: "corrente", int: "intermediaria", dest: "destinacao", obs: "observacoes" };
  for (const c of Object.keys(vals)) for (const v of vals[c]) {
    if (!regs.length) { // continuação de item da página anterior
      if (ultimoItem && tabela.has(ultimoItem)) tabela.get(ultimoItem)[alvo[c]].push(v.s);
      continue;
    }
    const r = regs.reduce((a, b) => (Math.abs(b.centro - v.c) < Math.abs(a.centro - v.c) ? b : a));
    r[alvo[c]].push(v.s);
  }
  for (const r of regs) {
    if (tabela.has(r.codigo)) { anomalias.push(`código repetido no documento: ${r.codigo} (pág. ${p})`); r.codigo = `${r.codigo}-2`; }
    tabela.set(r.codigo, r);
    ultimoItem = r.codigo;
    const prefixo = r.codigo.split(".")[0];
    if (!orgaos.has(prefixo)) orgaos.set(prefixo, { nome: orgaoLinha || secao.nome, edicao: secao.edicao, data: secao.data, versao: secao.versao });
    const fcod = r.codigo.split(".").slice(0, 3).join(".");
    if (r.funcao && !funcoes.has(fcod)) funcoes.set(fcod, r.funcao);
    const scod = r.codigo.split(".").slice(0, 4).join(".");
    if (r.subfuncao && !subfuncoes.has(scod)) subfuncoes.set(scod, { nome: r.subfuncao, recomendacao: "" });
    if (recomendacao && subfuncoes.has(scod) && recomendacao.sub === r.codigo.split(".")[3]) subfuncoes.get(scod).recomendacao = norm(recomendacao.texto);
  }
}

// ------------------------------------------------------ normalização final
function prazo(arr, codigo, fase) {
  let s = norm(arr.join(" "));
  if (!s) return { anos: null, texto: "", vazio: true };
  // Resíduos de descritor ("– MT", "– SMS") e travessões soltos.
  s = s.replace(/^[-–]\s*/, "").replace(/\s*[-–]\s*[A-Z]{2,}$/, "").replace(/\s*[-–]$/, "");
  s = s.replace(/Enquanto\s+estiver\s*[-–]?\s*vigorando/gi, "Enquanto estiver vigorando").replace(/^Enquanto (estiver|vigora)$/i, "Enquanto estiver vigorando");
  const rep = s.match(/^(.+?) \1$/);
  if (rep) s = rep[1];
  if (!s) return { anos: null, texto: "" };
  const m = s.match(/^(\d+)\s*anos?$/i);
  if (m) return { anos: Number(m[1]), texto: "" };
  return { anos: null, texto: s };
}
const chaveD = (x) => (x ?? "").replace(/s+/g, "").toLowerCase();
function melhorDescritor(lista, tabela) {
  if (!lista) return tabela;
  return chaveD(tabela).startsWith(chaveD(lista)) && chaveD(tabela).length > chaveD(lista).length ? tabela : lista;
}
const saida = [];
for (const [codigo, r] of tabela) {
  r.observacoes = r.observacoes.filter((o) => !/^\d{1,3}( \d{1,3})*$/.test(o.trim()));
  const dest = norm(r.destinacao.join(" ")).replace(/^[-–]\s*/, "");
  const extra = dest.replace(/^(Eliminação|Guarda\s+permanente)\s*[–-]?\s*/i, "");
  const destinacao = /^Elimina/i.test(dest) ? "ELIMINACAO" : /^Guarda/i.test(dest) ? "GUARDA_PERMANENTE" : "";
  const corrente = prazo(r.corrente, codigo, "corrente");
  const intermediaria = prazo(r.intermediaria, codigo, "intermediaria");
  if (!destinacao) anomalias.push(`sem destinação: ${codigo} "${dest}" (pág. ${r.pagina})`);
  if (corrente.vazio) anomalias.push(`sem prazo corrente: ${codigo} (pág. ${r.pagina})`);
  
  saida.push({ codigo, descritor: melhorDescritor(listagem.get(codigo), r.descritor), descritor_tabela: r.descritor, corrente, intermediaria,
    destinacao, observacoes: norm([extra, ...r.observacoes].filter(Boolean).join(" ")), pagina: r.pagina });
}
for (const c of listagem.keys()) if (!tabela.has(c)) anomalias.push(`na listagem mas fora da tabela: ${c} ${listagem.get(c)}`);

fs.writeFileSync(process.argv[3] ?? "ttdd-extraida.json", JSON.stringify({
  orgaos: Object.fromEntries(orgaos), funcoes: Object.fromEntries([...funcoes].map(([c, n]) => [c, planoFuncoes.get(c) ?? n])),
  subfuncoes: Object.fromEntries([...subfuncoes].map(([c, v]) => [c, { ...v, nome: planoSub.get(c) ?? v.nome }])),
  itens: saida, anomalias }, null, 2));
const porOrgao = {};
for (const i of saida) porOrgao[i.codigo.split(".")[0]] = (porOrgao[i.codigo.split(".")[0]] ?? 0) + 1;
console.log("itens", saida.length, "listagem", listagem.size, "por órgão", JSON.stringify(porOrgao), "anomalias", anomalias.length);
console.log(JSON.stringify(Object.fromEntries(orgaos), null, 1));
