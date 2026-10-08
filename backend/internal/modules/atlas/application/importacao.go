package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
)

// Importação e exportação de procedimentos em lote (ADR 025). O arquivo é
// o mesmo nos dois sentidos: exportar, revisar e importar de volta. A
// simulação roda o cadastro de verdade numa transação desfeita no fim — as
// regras são as mesmas do cadastro pela tela.

// MaxProcedimentosImportacao limita o arquivo de importação.
const MaxProcedimentosImportacao = 500

// ArquivoProcedimentos é o formato do arquivo (JSON).
type ArquivoProcedimentos struct {
	Procedimentos []ProcedimentoArquivo `json:"procedimentos"`
}

// ProcedimentoArquivo é um procedimento no arquivo; a peça aponta para o
// modelo da biblioteca pelo nome.
type ProcedimentoArquivo struct {
	CodigoProcessual string         `json:"codigo_processual"`
	Titulo           string         `json:"titulo"`
	Objetivo         string         `json:"objetivo"`
	PublicoAlvo      string         `json:"publico_alvo"`
	NivelAcesso      string         `json:"nivel_acesso"`
	HipoteseLegal    string         `json:"hipotese_legal_restricao,omitempty"`
	CodigoTTDD       string         `json:"codigo_ttdd"`
	Etapas           []EtapaArquivo `json:"etapas"`
}

// EtapaArquivo é uma etapa no arquivo.
type EtapaArquivo struct {
	Ordem                   int                `json:"ordem"`
	UnidadeAdministrativa   string             `json:"unidade_administrativa"`
	NomeSetor               string             `json:"nome_setor"`
	AtribuicoesSetor        string             `json:"atribuicoes_setor"`
	PrazoSLAEmDias          int                `json:"prazo_sla_em_dias"`
	ManterAbertoAposRemessa bool               `json:"manter_aberto_apos_remessa,omitempty"`
	Documentos              []DocumentoArquivo `json:"documentos,omitempty"`
	Transicoes              []TransicaoArquivo `json:"transicoes,omitempty"`
}

// DocumentoArquivo é uma peça no arquivo.
type DocumentoArquivo struct {
	NomeDocumento         string `json:"nome_documento"`
	Obrigatorio           bool   `json:"obrigatorio"`
	Formato               string `json:"formato"`
	TipoAssinatura        string `json:"tipo_assinatura"`
	ExigeConferenciaCopia bool   `json:"exige_conferencia_copia,omitempty"`
	ModeloMinutaPadraoURL string `json:"modelo_minuta_padrao_url,omitempty"`
	Modelo                string `json:"modelo,omitempty"` // nome do modelo na biblioteca
}

// TransicaoArquivo é uma transição no arquivo.
type TransicaoArquivo struct {
	DestinoOrdem          int    `json:"destino_ordem"`
	CondicaoTransicao     string `json:"condicao_transicao"`
	IsDevolucaoDiligencia bool   `json:"is_devolucao_diligencia,omitempty"`
	DescricaoDiligencia   string `json:"descricao_diligencia,omitempty"`
}

// LerArquivoProcedimentos interpreta o JSON (campos desconhecidos são erro:
// pegam digitação errada no nome de um campo).
func LerArquivoProcedimentos(conteudo string) (ArquivoProcedimentos, error) {
	var a ArquivoProcedimentos
	dec := json.NewDecoder(bytes.NewReader([]byte(conteudo)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return a, domain.InvalidError{Msg: "arquivo inválido: " + err.Error()}
	}
	switch n := len(a.Procedimentos); {
	case n == 0:
		return a, domain.InvalidError{Msg: "o arquivo não traz procedimentos (lista \"procedimentos\")"}
	case n > MaxProcedimentosImportacao:
		return a, domain.InvalidError{Msg: fmt.Sprintf("até %d procedimentos por arquivo", MaxProcedimentosImportacao)}
	}
	return a, nil
}

// paraWorkflow converte o item, resolvendo os modelos pelo nome.
func (p ProcedimentoArquivo) paraWorkflow(modelos map[string]uuid.UUID) (domain.Workflow, error) {
	w := domain.Workflow{CodigoProcessual: p.CodigoProcessual, Titulo: p.Titulo, Objetivo: p.Objetivo, PublicoAlvo: p.PublicoAlvo,
		NivelAcesso: domain.NivelAcesso(p.NivelAcesso), HipoteseLegal: p.HipoteseLegal, CodigoTTDD: p.CodigoTTDD, Versao: 1}
	for _, e := range p.Etapas {
		etapa := domain.Etapa{Ordem: e.Ordem, UnidadeAdministrativa: e.UnidadeAdministrativa, NomeSetor: e.NomeSetor,
			AtribuicoesSetor: e.AtribuicoesSetor, PrazoSLAEmDias: e.PrazoSLAEmDias, ManterAbertoAposRemessa: e.ManterAbertoAposRemessa}
		for _, d := range e.Documentos {
			doc := domain.EtapaDocumento{NomeDocumento: d.NomeDocumento, Obrigatorio: d.Obrigatorio,
				Formato: domain.FormatoDocumento(d.Formato), TipoAssinatura: domain.TipoAssinatura(d.TipoAssinatura),
				ExigeConferenciaCopia: d.ExigeConferenciaCopia, ModeloMinutaPadraoURL: d.ModeloMinutaPadraoURL}
			if d.Modelo != "" {
				id, ok := modelos[domain.Fold(d.Modelo)]
				if !ok {
					return w, domain.InvalidError{Msg: fmt.Sprintf("modelo %q não está ativo na biblioteca (peça %q)", d.Modelo, d.NomeDocumento)}
				}
				doc.ModeloID = &id
			}
			etapa.Documentos = append(etapa.Documentos, doc)
		}
		for _, t := range e.Transicoes {
			etapa.Transicoes = append(etapa.Transicoes, domain.EtapaTransicao{DestinoOrdem: t.DestinoOrdem,
				CondicaoTransicao: t.CondicaoTransicao, IsDevolucaoDiligencia: t.IsDevolucaoDiligencia, DescricaoDiligencia: t.DescricaoDiligencia})
		}
		w.Etapas = append(w.Etapas, etapa)
	}
	w.Normalize()
	return w, w.Validate()
}

// errItem é falha de regra num item: vira ERRO no relatório, a importação segue.
func errItem(err error) (string, bool) {
	var inv domain.InvalidError
	ok := errors.As(err, &inv)
	return inv.Msg, ok
}

// ImportarProcedimentos simula (aplicar=false) ou aplica a importação. Só
// aplica o arquivo simulado (hash) e sem nenhum item com erro.
func (s *Service) ImportarProcedimentos(ctx context.Context, identity auth.Identity, conteudo, hashConferido string, aplicar bool) (domain.ImportacaoProcedimentos, error) {
	return s.importar(ctx, identity, conteudo, hashConferido, aplicar, false)
}

// ImportarRascunhos importa como RASCUNHO (ADR 028): os procedimentos ficam
// inativos para a validação nos departamentos; a versão em vigor de um
// código existente continua publicada e ninguém é avisado.
func (s *Service) ImportarRascunhos(ctx context.Context, identity auth.Identity, conteudo, hashConferido string, aplicar bool) (domain.ImportacaoProcedimentos, error) {
	return s.importar(ctx, identity, conteudo, hashConferido, aplicar, true)
}

func (s *Service) importar(ctx context.Context, identity auth.Identity, conteudo, hashConferido string, aplicar, rascunho bool) (domain.ImportacaoProcedimentos, error) {
	hash := HashCarga(conteudo)
	if aplicar && hashConferido != hash {
		return domain.ImportacaoProcedimentos{}, MapError(domain.InvalidError{Msg: "o arquivo enviado não é o que foi simulado: simule de novo antes de aplicar"})
	}
	arq, err := LerArquivoProcedimentos(conteudo)
	if err != nil {
		return domain.ImportacaoProcedimentos{}, MapError(err)
	}
	out := domain.ImportacaoProcedimentos{Hash: hash, Totais: map[string]int{}}
	var enviadas []notificacoes.Enviada
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		biblioteca, err := s.repo.ListModelos(ctx, tx, false)
		if err != nil {
			return err
		}
		modelos := map[string]uuid.UUID{}
		for _, m := range biblioteca {
			modelos[domain.Fold(m.Nome)] = m.ID
		}
		vistos := map[string]bool{}
		for i, p := range arq.Procedimentos {
			item, avisos, err := s.importarItem(ctx, tx, identity, p, modelos, vistos, rascunho)
			if err != nil {
				return err
			}
			enviadas = append(enviadas, avisos...)
			item.Linha = i + 1
			out.Itens = append(out.Itens, item)
			out.Totais[item.Situacao]++
		}
		switch {
		case !aplicar:
			return errSimulacao
		case out.Totais[domain.ImportErro] > 0:
			return domain.InvalidError{Msg: "corrija os itens com erro e simule de novo antes de aplicar"}
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "atlas.procedimentos.importados", "atlas_workflow", hash, nil,
			map[string]any{"totais": out.Totais}))
	})
	if errors.Is(err, errSimulacao) {
		err = nil
	}
	out.Aplicada = aplicar && err == nil
	if out.Aplicada {
		s.entregar(ctx, enviadas)
	}
	return out, MapError(err)
}

// importarItem trata um procedimento num savepoint: falha de regra desfaz
// só o item (ERRO no relatório); falha de banco aborta a importação.
func (s *Service) importarItem(ctx context.Context, tx pgx.Tx, identity auth.Identity, p ProcedimentoArquivo,
	modelos map[string]uuid.UUID, vistos map[string]bool, rascunho bool) (domain.ItemImportacao, []notificacoes.Enviada, error) {
	item := domain.ItemImportacao{CodigoProcessual: p.CodigoProcessual, Titulo: p.Titulo}
	w, err := p.paraWorkflow(modelos)
	item.CodigoProcessual = w.CodigoProcessual
	if rascunho {
		w.Situacao = domain.SituacaoRascunho
	}
	if err == nil && vistos[w.CodigoProcessual] {
		err = domain.InvalidError{Msg: "código processual repetido no arquivo"}
	}
	vistos[w.CodigoProcessual] = true
	if err != nil {
		item.Situacao, item.Erro = domain.ImportErro, err.Error()
		return item, nil, nil
	}
	versoes, _, err := s.repo.List(ctx, tx, domain.Filter{CodigoProcessual: w.CodigoProcessual, IncluirInativos: true}, pagination.New(1, 100, 100))
	if err != nil {
		return item, nil, err
	}
	var base *domain.Workflow
	for i := range versoes {
		if base == nil || versoes[i].Ativo || (!base.Ativo && versoes[i].Versao > base.Versao) {
			base = &versoes[i]
		}
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return item, nil, err
	}
	var (
		salvo  domain.Workflow
		avisos []notificacoes.Enviada
	)
	if base == nil {
		item.Situacao = domain.ImportNovo
		salvo, err = s.inserir(ctx, sp, identity, w, nil)
	} else {
		atual, gerr := s.repo.Get(ctx, sp, base.ID, true)
		if gerr != nil {
			_ = sp.Rollback(ctx)
			return item, nil, gerr
		}
		if atual.Ativo && domain.Assinatura(atual) == domain.Assinatura(w) {
			item.Situacao, item.Versao = domain.ImportInalterado, atual.Versao
			return item, nil, sp.Commit(ctx)
		}
		item.Situacao = domain.ImportNovaVersao
		salvo, avisos, err = s.novaVersaoTx(ctx, sp, identity, atual, w)
	}
	if err != nil {
		_ = sp.Rollback(ctx)
		msg, regra := errItem(err)
		if !regra {
			return item, nil, err
		}
		item.Situacao, item.Erro = domain.ImportErro, msg
		return item, nil, nil
	}
	item.Versao = salvo.Versao
	return item, avisos, sp.Commit(ctx)
}

// ExportarProcedimentos devolve os procedimentos em vigor no formato da
// importação (a peça leva o nome do modelo da biblioteca).
func (s *Service) ExportarProcedimentos(ctx context.Context) (ArquivoProcedimentos, error) {
	out := ArquivoProcedimentos{Procedimentos: []ProcedimentoArquivo{}}
	for page := 1; ; page++ {
		p := pagination.New(page, pagination.AbsoluteMaxPageSize, pagination.AbsoluteMaxPageSize)
		items, total, err := s.repo.List(ctx, s.pool, domain.Filter{}, p)
		if err != nil {
			return out, MapError(err)
		}
		for _, resumo := range items {
			w, err := s.repo.Get(ctx, s.pool, resumo.ID, false)
			if err != nil {
				return out, MapError(err)
			}
			out.Procedimentos = append(out.Procedimentos, paraArquivo(w))
		}
		if int64(len(out.Procedimentos)) >= total || len(items) == 0 {
			return out, nil
		}
	}
}

func paraArquivo(w domain.Workflow) ProcedimentoArquivo {
	p := ProcedimentoArquivo{CodigoProcessual: w.CodigoProcessual, Titulo: w.Titulo, Objetivo: w.Objetivo, PublicoAlvo: w.PublicoAlvo,
		NivelAcesso: string(w.NivelAcesso), HipoteseLegal: w.HipoteseLegal, CodigoTTDD: w.CodigoTTDD, Etapas: []EtapaArquivo{}}
	for _, e := range w.Etapas {
		etapa := EtapaArquivo{Ordem: e.Ordem, UnidadeAdministrativa: e.UnidadeAdministrativa, NomeSetor: e.NomeSetor,
			AtribuicoesSetor: e.AtribuicoesSetor, PrazoSLAEmDias: e.PrazoSLAEmDias, ManterAbertoAposRemessa: e.ManterAbertoAposRemessa}
		for _, d := range e.Documentos {
			doc := DocumentoArquivo{NomeDocumento: d.NomeDocumento, Obrigatorio: d.Obrigatorio, Formato: string(d.Formato),
				TipoAssinatura: string(d.TipoAssinatura), ExigeConferenciaCopia: d.ExigeConferenciaCopia, ModeloMinutaPadraoURL: d.ModeloMinutaPadraoURL}
			if d.Modelo != nil {
				doc.Modelo = d.Modelo.Nome
			}
			etapa.Documentos = append(etapa.Documentos, doc)
		}
		for _, t := range e.Transicoes {
			etapa.Transicoes = append(etapa.Transicoes, TransicaoArquivo{t.DestinoOrdem, t.CondicaoTransicao, t.IsDevolucaoDiligencia, t.DescricaoDiligencia})
		}
		p.Etapas = append(p.Etapas, etapa)
	}
	return p
}
