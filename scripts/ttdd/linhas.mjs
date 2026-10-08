// Agrupa fragmentos do pdf.js em linhas (mesma página, y próximo) e junta os
// pedaços colados — códigos quebrados em "1", "2", ".0.01..." viram um token.
// Agrupa os itens do pdf.js em linhas (mesma página, y próximo) e junta os
// pedaços colados (códigos quebrados em "1","2",".0.01...").
export function linhas(items) {
  const porPag = new Map();
  for (const i of items) (porPag.get(i.p) ?? porPag.set(i.p, []).get(i.p)).push(i);
  const out = [];
  for (const [p, its] of [...porPag].sort((a, b) => a[0] - b[0])) {
    its.sort((a, b) => a.y - b.y || a.x - b.x);
    const grupos = [];
    for (const it of its) {
      const g = grupos.find((g) => Math.abs(g.y - it.y) <= 2);
      g ? g.its.push(it) : grupos.push({ y: it.y, its: [it] });
    }
    for (const g of grupos.sort((a, b) => a.y - b.y)) {
      g.its.sort((a, b) => a.x - b.x);
      // Pedaços colados (gap < 2pt) viram um token só; senão, tokens separados.
      const toks = [];
      for (const it of g.its) {
        const last = toks.at(-1);
        if (last && it.x - (last.x + last.w) < 2) { last.s += it.s; last.w = it.x + it.w - last.x; }
        else toks.push({ ...it });
      }
      out.push({ p, y: g.y, toks });
    }
  }
  return out;
}
