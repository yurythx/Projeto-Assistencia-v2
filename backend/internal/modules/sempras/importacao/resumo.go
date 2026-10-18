package importacao

import (
	"fmt"
	"strings"
)

// Resumo é o relatório para ler no terminal: as contagens por tabela, o
// que entrou com ajuste a revisar e o que ficou de fora.
func (r Relatorio) Resumo() string {
	var b strings.Builder
	if r.Aplicada {
		b.WriteString("IMPORTAÇÃO APLICADA\n\n")
	} else {
		b.WriteString("SIMULAÇÃO — nada foi gravado (rode com -aplicar para gravar)\n\n")
	}
	fmt.Fprintf(&b, "%-24s %6s %10s %9s %10s %9s\n", "tabela do 4.0", "lidos", "importados", "já antes", "rejeitados", "ignorados")
	for _, o := range Origens {
		c := r.Contagens[o]
		fmt.Fprintf(&b, "%-24s %6d %10d %9d %10d %9d\n", o, c.Lidos, c.Importados, c.JaImportados, c.Rejeitados, c.Ignorados)
	}
	for _, g := range []struct {
		titulo string
		itens  []Aviso
	}{{"A revisar (entraram com ajuste)", r.Avisos}, {"Ficaram de fora (corrija no 4.0 e simule de novo)", r.Rejeitados}} {
		fmt.Fprintf(&b, "\n%s: %d\n", g.titulo, len(g.itens))
		for _, a := range g.itens {
			fmt.Fprintf(&b, "  %s nº %d: %s\n", a.Origem, a.ID, a.Mensagem)
		}
	}
	return b.String()
}
