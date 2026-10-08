package domain

import (
	"encoding/json"
	"sort"

	"github.com/google/uuid"
)

// Importação de procedimentos em lote (ADR 025): cada item do arquivo vira
// um procedimento novo, uma nova versão de um existente ou fica como está.

// Situação de um item da importação.
const (
	ImportNovo       = "NOVO"        // código processual ainda não cadastrado
	ImportNovaVersao = "NOVA_VERSAO" // conteúdo diferente da versão em vigor
	ImportInalterado = "INALTERADO"  // igual à versão em vigor: nada a fazer
	ImportErro       = "ERRO"        // não passa nas regras de cadastro
)

// ItemImportacao é o resultado de um procedimento do arquivo.
type ItemImportacao struct {
	Linha            int    `json:"linha"` // posição no arquivo (1, 2, …)
	CodigoProcessual string `json:"codigo_processual"`
	Titulo           string `json:"titulo"`
	Situacao         string `json:"situacao"`
	Versao           int    `json:"versao"` // a versão que fica (ou ficaria) em vigor
	Erro             string `json:"erro,omitempty"`
}

// ImportacaoProcedimentos é o resultado da simulação ou da aplicação.
type ImportacaoProcedimentos struct {
	Hash     string           `json:"hash"`
	Aplicada bool             `json:"aplicada"`
	Totais   map[string]int   `json:"totais"`
	Itens    []ItemImportacao `json:"itens"`
}

// Assinatura resume o conteúdo do procedimento (sem ids, versão nem datas;
// peças e transições em ordem estável): duas versões com a mesma assinatura
// descrevem o mesmo fluxo. Chame com o procedimento normalizado.
func Assinatura(w Workflow) string {
	type doc struct {
		Nome, Formato, Assinatura, URL string
		Obrigatorio, Conferencia       bool
		Modelo                         *uuid.UUID
	}
	type trans struct {
		Destino              int
		Condicao, Diligencia string
		Devolucao            bool
	}
	type etapa struct {
		Ordem, Prazo                int
		Unidade, Setor, Atribuicoes string
		Aberto                      bool
		Docs                        []doc
		Trans                       []trans
	}
	v := struct {
		Titulo, Objetivo, Publico, Nivel, Hipotese, Serie string
		Etapas                                            []etapa
	}{w.Titulo, w.Objetivo, w.PublicoAlvo, string(w.NivelAcesso), w.HipoteseLegal, w.CodigoTTDD, nil}
	for _, e := range w.Etapas {
		x := etapa{Ordem: e.Ordem, Prazo: e.PrazoSLAEmDias, Unidade: e.UnidadeAdministrativa, Setor: e.NomeSetor,
			Atribuicoes: e.AtribuicoesSetor, Aberto: e.ManterAbertoAposRemessa}
		for _, d := range e.Documentos {
			x.Docs = append(x.Docs, doc{d.NomeDocumento, string(d.Formato), string(d.TipoAssinatura), d.ModeloMinutaPadraoURL,
				d.Obrigatorio, d.ExigeConferenciaCopia, d.ModeloID})
		}
		sort.Slice(x.Docs, func(i, j int) bool { return x.Docs[i].Nome < x.Docs[j].Nome })
		for _, t := range e.Transicoes {
			x.Trans = append(x.Trans, trans{t.DestinoOrdem, t.CondicaoTransicao, t.DescricaoDiligencia, t.IsDevolucaoDiligencia})
		}
		sort.Slice(x.Trans, func(i, j int) bool {
			if x.Trans[i].Destino != x.Trans[j].Destino {
				return x.Trans[i].Destino < x.Trans[j].Destino
			}
			return x.Trans[i].Condicao < x.Trans[j].Condicao
		})
		v.Etapas = append(v.Etapas, x)
	}
	sort.Slice(v.Etapas, func(i, j int) bool { return v.Etapas[i].Ordem < v.Etapas[j].Ordem })
	b, _ := json.Marshal(v) // tipos simples: não falha
	return string(b)
}
