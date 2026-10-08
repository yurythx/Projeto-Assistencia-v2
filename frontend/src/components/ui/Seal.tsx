// Selo circular — o elemento de assinatura do Projeto Nexus (§ redesenho
// 2026-08). Um carimbo de repartição: anéis concêntricos, borda pontilhada,
// dizeres curvos e um miolo com o monograma da marca sobre os dizeres de
// documento oficial. Aparece como marca d'água discreta no painel de login
// e na home pública — nunca como enfeite repetido pela interface toda.
//
// Usa `currentColor` em todos os traços/textos: quem chama controla a cor
// (branco sobre o painel institucional, tinta de carimbo sobre fundo claro)
// e a opacidade.
export function Seal({
  size = 220,
  className = "",
  topText = "PROJETO NEXUS ✦ PLATAFORMA",
  bottomText = "DOCUMENTO OFICIAL",
  decorative = false,
}: {
  size?: number;
  className?: string;
  topText?: string;
  bottomText?: string;
  /** Marca d'água puramente visual — sai da árvore de acessibilidade. */
  decorative?: boolean;
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 200 200"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      role={decorative ? undefined : "img"}
      aria-label={decorative ? undefined : "Selo do Projeto Nexus"}
      aria-hidden={decorative || undefined}
      focusable="false"
      className={`shrink-0 ${className}`}
      style={{ color: "currentColor" }}
    >
      <defs>
        <path id="nexus-seal-top" d="M 26 100 A 74 74 0 0 1 174 100" />
        <path id="nexus-seal-bottom" d="M 32 100 A 68 68 0 0 0 168 100" />
      </defs>

      <circle cx="100" cy="100" r="96" stroke="currentColor" strokeWidth="2" />
      <circle cx="100" cy="100" r="88" stroke="currentColor" strokeWidth="1" strokeOpacity="0.7" />
      <circle
        cx="100"
        cy="100"
        r="82"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeDasharray="1.5 6"
        strokeOpacity="0.85"
      />
      <circle cx="100" cy="100" r="58" stroke="currentColor" strokeWidth="1" strokeOpacity="0.55" />

      <text
        fontSize="12.5"
        fontWeight="700"
        letterSpacing="3.2"
        fill="currentColor"
        style={{ fontFamily: "var(--font-mono, monospace)" }}
      >
        <textPath href="#nexus-seal-top" startOffset="50%" textAnchor="middle">
          {topText}
        </textPath>
      </text>
      <text
        fontSize="12.5"
        fontWeight="700"
        letterSpacing="3.2"
        fill="currentColor"
        style={{ fontFamily: "var(--font-mono, monospace)" }}
      >
        <textPath href="#nexus-seal-bottom" startOffset="50%" textAnchor="middle">
          {bottomText}
        </textPath>
      </text>

      {/* Miolo: monograma "A" de Nexus (mesmo desenho de Logo.tsx/icon.svg,
          escalado pro selo) sobre os dizeres de documento oficial. */}
      <g transform="translate(100 92)">
        <polygon points="0,-15 -9,15 -15,15" fill="currentColor" />
        <polygon points="0,-15 15,15 9,15" fill="currentColor" />
        <rect x="-5.25" y="1.5" width="10.5" height="2.4" fill="currentColor" />
      </g>
      <path d="M 74 118 H 126" stroke="currentColor" strokeWidth="1.5" />
      <text
        x="100"
        y="132"
        textAnchor="middle"
        fontSize="9"
        fontWeight="600"
        letterSpacing="2"
        fill="currentColor"
        style={{ fontFamily: "var(--font-mono, monospace)" }}
      >
        DOCUMENTO OFICIAL
      </text>
    </svg>
  );
}
