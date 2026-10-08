package domain

import "testing"

func TestMontarOrganograma(t *testing.T) {
	novo := func() []OrgaoOrganograma {
		return []OrgaoOrganograma{{Prefixo: "2.0", Nome: "Administração", Funcoes: []FuncaoOrganograma{
			{Codigo: "2.0.02", Nome: "Compras", Subfuncoes: []SubfuncaoOrganograma{
				{Codigo: "2.0.02.00", ContagemOrganograma: ContagemOrganograma{Series: 10, Publicados: 1, Rascunhos: 2}, SeriesDeProcesso: 3, Procedimentos: 3},
				// Só rascunho: lacuna para o público, não para a gestão.
				{Codigo: "2.0.02.01", ContagemOrganograma: ContagemOrganograma{Series: 4, Rascunhos: 1, EmValidacao: 1}, SeriesDeProcesso: 1, Procedimentos: 2},
			}},
			{Codigo: "2.0.06", Nome: "Folha", Subfuncoes: []SubfuncaoOrganograma{
				{Codigo: "2.0.06.04", ContagemOrganograma: ContagemOrganograma{Series: 6}, SeriesDeProcesso: 2},
				// Sem série de processo: nunca é lacuna.
				{Codigo: "2.0.06.05", ContagemOrganograma: ContagemOrganograma{Series: 2}},
			}},
		}}}
	}

	g := MontarOrganograma(novo(), true)[0]
	if want := (ContagemOrganograma{Series: 22, Publicados: 1, EmValidacao: 1, Rascunhos: 3, Lacunas: 1}); g.ContagemOrganograma != want {
		t.Fatalf("gestão, órgão: %+v", g.ContagemOrganograma)
	}
	if f := g.Funcoes[0].ContagemOrganograma; f.Series != 14 || f.Rascunhos != 3 || f.Lacunas != 0 {
		t.Fatalf("gestão, função: %+v", f)
	}

	p := MontarOrganograma(novo(), false)[0]
	if want := (ContagemOrganograma{Series: 22, Publicados: 1, Lacunas: 2}); p.ContagemOrganograma != want {
		t.Fatalf("público, órgão: %+v", p.ContagemOrganograma)
	}
	if s := p.Funcoes[0].Subfuncoes[1]; s.Lacunas != 1 || s.Rascunhos != 0 || s.EmValidacao != 0 {
		t.Fatalf("público, subfunção só com rascunho: %+v", s)
	}
}
