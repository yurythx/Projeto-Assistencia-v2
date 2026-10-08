// Extrai o texto do PDF com a posição (x, y) de cada fragmento, por página.
// Saída: lista JSON { p, x, y, w, s } usada por analisar.mjs.
import { getDocument } from "pdfjs-dist/legacy/build/pdf.mjs";
import fs from "node:fs";
const doc = await getDocument({ data: new Uint8Array(fs.readFileSync(process.argv[2])), verbosity: 0 }).promise;
const out = [];
for (let p = 1; p <= doc.numPages; p++) {
  const page = await doc.getPage(p);
  const vp = page.getViewport({ scale: 1 });
  const tc = await page.getTextContent();
  for (const it of tc.items) {
    if (!it.str.trim()) continue;
    out.push({ p, x: Math.round(it.transform[4]), y: Math.round(vp.height - it.transform[5]), w: Math.round(it.width), s: it.str });
  }
}
fs.writeFileSync(process.argv[3], JSON.stringify(out));
console.log("páginas", doc.numPages, "itens", out.length);
