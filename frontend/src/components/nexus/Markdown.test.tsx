import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Markdown, safeMarkdownHref } from "./Markdown";

describe("Markdown seguro", () => {
  it("renderiza títulos, listas, ênfase e código", () => {
    const { container } = render(<Markdown source={"# Título\n\n- um\n- **dois**\n\n1. primeiro\n\n`codigo` e *itálico*\n\n```\nbloco\n```"} />);
    expect(screen.getByRole("heading", { name: "Título" })).toBeInTheDocument();
    expect(container.querySelectorAll("ul li")).toHaveLength(2);
    expect(container.querySelector("ol li")).toHaveTextContent("primeiro");
    expect(container.querySelector("strong")).toHaveTextContent("dois");
    expect(container.querySelector("pre code")).toHaveTextContent("bloco");
  });

  it("HTML embutido aparece como texto (sem XSS)", () => {
    const { container } = render(<Markdown source={'<img src=x onerror="alert(1)"><script>alert(2)</script>'} />);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
    expect(container).toHaveTextContent("<script>alert(2)</script>");
  });

  it("links: internos e http(s) viram <a>; javascript:/data:/protocol-relative viram texto", () => {
    render(<Markdown source={"[interno](/wiki/manual) [site](https://gov.br) [mau](javascript:alert(1)) [dados](data:text/html,x) [outro](//evil.com)"} />);
    expect(screen.getByRole("link", { name: "interno" })).toHaveAttribute("href", "/wiki/manual");
    expect(screen.getByRole("link", { name: "interno" })).not.toHaveAttribute("target");
    expect(screen.getByRole("link", { name: "site" })).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.queryByRole("link", { name: "mau" })).toBeNull();
    expect(screen.queryByRole("link", { name: "dados" })).toBeNull();
    expect(screen.queryByRole("link", { name: "outro" })).toBeNull();
  });

  it("safeMarkdownHref", () => {
    expect(safeMarkdownHref("/a/b")).toBe("/a/b");
    expect(safeMarkdownHref("//evil.com")).toBeNull();
    expect(safeMarkdownHref("/\\evil.com")).toBeNull();
    expect(safeMarkdownHref("JAVASCRIPT:alert(1)")).toBeNull();
    expect(safeMarkdownHref("http://x.gov.br/")).toBe("http://x.gov.br/");
  });
});
