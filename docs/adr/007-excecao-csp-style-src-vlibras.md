# 007 — Exceção formal de CSP: `style-src 'unsafe-inline'` (VLibras)

- **Status:** Aceito (com revisão trimestral)
- **Data:** 2026-09-09
- **Autores:** Engenharia Projeto Nexus

## Contexto

Gap G-16 da auditoria de conformidade. A CSP do frontend
(`frontend/src/proxy.ts`) usa `nonce` + `strict-dynamic` no `script-src`
(sem `'unsafe-inline'` em produção), mas mantém `'unsafe-inline'` no
`style-src`:

```
style-src 'self' 'unsafe-inline' https://vlibras.gov.br https://*.vlibras.gov.br https://cdn.jsdelivr.net;
```

O widget oficial VLibras (Governo Federal — obrigatório pela política de
acessibilidade e-MAG) injeta múltiplos elementos `<style>` sem nonce em
tempo de execução, a partir do player em WebAssembly. Não há hook para
carimbar esses estilos.

## Decisão

Aceitar `style-src 'unsafe-inline'` como **exceção documentada**, com as
seguintes condições:

1. **Justificativa:** o VLibras é requisito de acessibilidade e não
   oferece alternativa com nonce/hash; removê-lo quebra a conformidade
   e-MAG.
2. **Compensação:** o vetor mitigado é apenas XSS de *estilo* (exfiltração
   via seletores de atributo, defacement) — substancialmente menos grave
   que XSS de *script*, que a CSP continua bloqueando (`script-src` sem
   `'unsafe-inline'`, com `strict-dynamic`).
3. **Escopo:** a exceção vale só para `style-src`. Nenhuma outra diretiva
   afrouxa.
4. **Revisão:** reavaliar a cada trimestre e a cada atualização do
   VLibras — investigar `style-src` com lista de `sha256-...` para os
   `<style>` conhecidos do widget, ou isolar o VLibras num `iframe` de
   origem própria com CSP separada.
5. **Registro de risco:** item permanente na pauta de revisão de risco
   de segurança, referenciando este ADR.

## Consequências

- A CSP de produção não é "pura" em `style-src` — aceito conscientemente.
- Qualquer PR que proponha novas fontes em `style-src` além das já
  listadas exige atualização deste ADR.
