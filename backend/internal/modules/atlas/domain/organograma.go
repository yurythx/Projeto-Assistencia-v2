package domain

// Organograma da TTDD: secretaria (órgão) → função → subfunção, com as
// séries e os procedimentos de cada nível. É a base da aba Organograma do
// Atlas (visualizações em caixas, tópicos, blocos e fichas).

// ContagemOrganograma: séries vigentes e procedimentos de um nó. Rascunhos
// e em validação contam só a versão mais recente de cada código e só
// aparecem para a gestão.
type ContagemOrganograma struct {
	Series      int `json:"series"`
	Publicados  int `json:"publicados"`
	EmValidacao int `json:"em_validacao"`
	Rascunhos   int `json:"rascunhos"`
	// Lacunas: subfunções com série de processo e nenhum procedimento (para
	// o público: nenhum publicado).
	Lacunas int `json:"lacunas"`
}

func (c *ContagemOrganograma) somar(o ContagemOrganograma) {
	c.Series += o.Series
	c.Publicados += o.Publicados
	c.EmValidacao += o.EmValidacao
	c.Rascunhos += o.Rascunhos
	c.Lacunas += o.Lacunas
}

// SubfuncaoOrganograma é a folha do organograma.
type SubfuncaoOrganograma struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
	ContagemOrganograma
	// SeriesDeProcesso: séries que descrevem um processo ou pedido.
	SeriesDeProcesso int `json:"series_de_processo"`
	// Procedimentos: todos os códigos com alguma versão (base da lacuna da
	// gestão); não vai para a resposta.
	Procedimentos int `json:"-"`
}

// FuncaoOrganograma agrupa subfunções.
type FuncaoOrganograma struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
	ContagemOrganograma
	Subfuncoes []SubfuncaoOrganograma `json:"subfuncoes"`
}

// OrgaoOrganograma é a secretaria.
type OrgaoOrganograma struct {
	Prefixo string `json:"prefixo"`
	Nome    string `json:"nome"`
	ContagemOrganograma
	Funcoes []FuncaoOrganograma `json:"funcoes"`
}

// MontarOrganograma marca as lacunas das subfunções e soma as contagens nas
// funções e nos órgãos. Para o público (gestao = false), rascunhos e em
// validação são zerados e a lacuna é "nenhum procedimento publicado".
func MontarOrganograma(orgaos []OrgaoOrganograma, gestao bool) []OrgaoOrganograma {
	for i := range orgaos {
		o := &orgaos[i]
		o.ContagemOrganograma = ContagemOrganograma{}
		for j := range o.Funcoes {
			f := &o.Funcoes[j]
			f.ContagemOrganograma = ContagemOrganograma{}
			for k := range f.Subfuncoes {
				s := &f.Subfuncoes[k]
				com := s.Procedimentos
				if !gestao {
					s.EmValidacao, s.Rascunhos = 0, 0
					com = s.Publicados
				}
				s.Lacunas = 0
				if s.SeriesDeProcesso > 0 && com == 0 {
					s.Lacunas = 1
				}
				f.somar(s.ContagemOrganograma)
			}
			o.somar(f.ContagemOrganograma)
		}
	}
	return orgaos
}
