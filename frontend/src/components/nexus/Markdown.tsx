import { Fragment, type ReactNode } from "react";

import { safeResourceUrl } from "@/lib/security/safe-url";

// Renderizador Markdown mínimo e SEGURO: produz elementos React (nunca
// dangerouslySetInnerHTML), então HTML embutido no texto aparece como
// texto literal — sem vetor de XSS. Suporta títulos, parágrafos, listas,
// citações, blocos de código, **negrito**, *itálico*, `código` e links
// (só http/https/relativos, via safeResourceUrl).

/** Link permitido no Markdown: caminho interno ("/wiki/x", nunca
 * "//host") ou URL absoluta http/https. javascript:, data: etc. viram
 * texto. */
export function safeMarkdownHref(href: string): string | null {
  if (/^\/(?![\/\\])/.test(href)) return href;
  return safeResourceUrl(href, { allowHttp: true });
}

function inline(text: string, keyBase: string): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`|\[[^\]]+\]\([^)\s]+\))/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let i = 0;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index));
    const tok = m[0];
    const key = `${keyBase}-${i++}`;
    if (tok.startsWith("**")) out.push(<strong key={key}>{tok.slice(2, -2)}</strong>);
    else if (tok.startsWith("`")) out.push(<code key={key} className="rounded bg-surface-hover px-1 py-0.5 font-mono text-[0.9em]">{tok.slice(1, -1)}</code>);
    else if (tok.startsWith("[")) {
      const [, label, href] = /^\[([^\]]+)\]\(([^)\s]+)\)$/.exec(tok) ?? [];
      const safe = href ? safeMarkdownHref(href) : null;
      out.push(
        safe ? (
          <a key={key} href={safe} className="text-primary underline underline-offset-2" rel="noopener noreferrer" target={safe.startsWith("/") ? undefined : "_blank"}>
            {label}
          </a>
        ) : (
          <Fragment key={key}>{label}</Fragment>
        ),
      );
    } else out.push(<em key={key}>{tok.slice(1, -1)}</em>);
    last = m.index + tok.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

export function Markdown({ source, className = "" }: { source: string; className?: string }) {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const blocks: ReactNode[] = [];
  let i = 0;
  let k = 0;
  while (i < lines.length) {
    const line = lines[i] ?? "";
    if (line.startsWith("```")) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !(lines[i] ?? "").startsWith("```")) code.push(lines[i++] ?? "");
      i++;
      blocks.push(<pre key={k++} className="overflow-x-auto rounded-lg bg-surface-hover p-3 font-mono text-sm"><code>{code.join("\n")}</code></pre>);
      continue;
    }
    const h = /^(#{1,4})\s+(.*)$/.exec(line);
    if (h) {
      const level = h[1]!.length;
      const cls = ["", "text-2xl", "text-xl", "text-lg", "text-base"][level];
      const content = inline(h[2]!, `h${k}`);
      const Tag = (`h${Math.min(level + 1, 5)}` as "h2" | "h3" | "h4" | "h5");
      blocks.push(<Tag key={k++} className={`mt-5 font-semibold ${cls}`}>{content}</Tag>);
      i++;
      continue;
    }
    if (/^\s*([-*]|\d+\.)\s+/.test(line)) {
      const ordered = /^\s*\d+\./.test(line);
      const items: string[] = [];
      while (i < lines.length && /^\s*([-*]|\d+\.)\s+/.test(lines[i] ?? "")) items.push((lines[i++] ?? "").replace(/^\s*([-*]|\d+\.)\s+/, ""));
      const List = ordered ? "ol" : "ul";
      blocks.push(
        <List key={k++} className={`my-3 ml-6 flex flex-col gap-1 ${ordered ? "list-decimal" : "list-disc"}`}>
          {items.map((it, j) => (
            <li key={j}>{inline(it, `l${k}-${j}`)}</li>
          ))}
        </List>,
      );
      continue;
    }
    if (line.startsWith(">")) {
      const quote: string[] = [];
      while (i < lines.length && (lines[i] ?? "").startsWith(">")) quote.push((lines[i++] ?? "").replace(/^>\s?/, ""));
      blocks.push(<blockquote key={k++} className="my-3 border-l-4 border-primary/40 pl-4 text-muted">{inline(quote.join(" "), `q${k}`)}</blockquote>);
      continue;
    }
    if (line.trim() === "") {
      i++;
      continue;
    }
    const para: string[] = [];
    while (i < lines.length && (lines[i] ?? "").trim() !== "" && !/^(#{1,4}\s|```|>|\s*([-*]|\d+\.)\s)/.test(lines[i] ?? "")) para.push(lines[i++] ?? "");
    blocks.push(<p key={k++} className="my-3 leading-relaxed">{inline(para.join(" "), `p${k}`)}</p>);
  }
  return <div className={`text-foreground ${className}`}>{blocks}</div>;
}
