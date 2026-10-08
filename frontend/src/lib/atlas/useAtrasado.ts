import { useEffect, useState } from "react";

/** Devolve `valor` só depois de `ms` sem mudanças (espera o usuário parar de
 * digitar antes de consultar a API). */
export function useAtrasado(valor: string, ms = 250) {
  const [v, setV] = useState(valor);
  useEffect(() => {
    const t = setTimeout(() => setV(valor), ms);
    return () => clearTimeout(t);
  }, [valor, ms]);
  return v;
}
