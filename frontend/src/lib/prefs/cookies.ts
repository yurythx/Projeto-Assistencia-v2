// Helpers de cookie no cliente para preferências de dispositivo que
// precisam chegar ao servidor — e, portanto, ao PRIMEIRO paint. É o mesmo
// motivo pelo qual o tema usa cookie e não localStorage: um Server
// Component (os layouts) lê o cookie e já renderiza o HTML no estado certo,
// sem o flash de "valor padrão → valor salvo" que localStorage + hidratação
// sempre causam. Ver components/ui/ThemeToggle.tsx.
//
// Todos os nomes de cookie usados aqui são constantes internas (`[\w-]+`),
// nunca entrada de usuário — daí o RegExp montado por interpolação ser
// seguro.

const ONE_YEAR_SECONDS = 60 * 60 * 24 * 365;

export function readCookie(name: string): string | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
  return match ? decodeURIComponent(match[1] ?? "") : null;
}

export function writeCookie(name: string, value: string, maxAgeSeconds = ONE_YEAR_SECONDS): void {
  if (typeof document === "undefined") return;
  document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=${maxAgeSeconds}; samesite=lax`;
}

export function deleteCookie(name: string): void {
  if (typeof document === "undefined") return;
  document.cookie = `${name}=; path=/; max-age=0; samesite=lax`;
}
