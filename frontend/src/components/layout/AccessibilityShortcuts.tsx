"use client";

import { useEffect } from "react";

import { SKIP_TARGETS } from "@/components/layout/AccessibilityBar";

/** Move o foco para o alvo de um atalho e-MAG. Campos de formulário
 * recebem foco direto; regiões (main/nav/footer) ganham tabIndex=-1. */
export function focusSkipTarget(id: string): boolean {
  const el = document.getElementById(id);
  if (!el) return false;
  if (!/^(INPUT|TEXTAREA|SELECT|BUTTON|A)$/.test(el.tagName) && !el.hasAttribute("tabindex")) {
    el.setAttribute("tabindex", "-1");
  }
  el.focus();
  el.scrollIntoView({ block: "start" });
  return true;
}

/**
 * Listener global dos atalhos Alt+1 (conteúdo), Alt+2 (menu), Alt+3
 * (busca) e Alt+4 (rodapé) — skill §3. Complementa o accessKey dos links
 * da barra (cujo comportamento varia entre navegadores: no Firefox é
 * Alt+Shift) garantindo o mesmo atalho em todos.
 */
export function AccessibilityShortcuts() {
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (!e.altKey || e.ctrlKey || e.metaKey) return;
      const target = SKIP_TARGETS.find((t) => t.key === e.key || e.code === `Digit${t.key}`);
      if (!target) return;
      if (focusSkipTarget(target.id)) e.preventDefault();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);
  return null;
}
