package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
	"github.com/yurythx/projeto-nexus/internal/platform/storage"
)

// Biblioteca de modelos de documento (ADR 024). O arquivo de cada versão
// vai para o armazenamento de objetos ANTES da transação que o registra;
// se a transação falha, o objeto é apagado (sem órfão no bucket).

// Eventos de auditoria da biblioteca.
const (
	EventModeloCriado   = "atlas.modelo.criado"
	EventModeloVersao   = "atlas.modelo.versao"
	EventModeloAlterado = "atlas.modelo.alterado"
	EventModeloPeca     = "atlas.workflow.modelo_peca"
)

// WithStorage liga o armazenamento dos arquivos de modelo.
func (s *Service) WithStorage(store storage.Provider, bucket string) *Service {
	s.store, s.bucket = store, bucket
	return s
}

// ArquivoEnviado é o arquivo de uma versão de modelo vindo do formulário.
type ArquivoEnviado struct {
	Nome     string
	Conteudo []byte
	Nota     string
}

// autor identifica quem alterou (nome de usuário ou, sem ele, o subject).
func autor(identity auth.Identity) string {
	if identity.Username != "" {
		return identity.Username
	}
	return identity.Subject
}

func mapModeloError(err error) error {
	switch {
	case errors.Is(err, domain.ErrModeloNaoEncontrado):
		return apperrors.NotFound("modelo de documento não encontrado")
	case errors.Is(err, domain.ErrModeloRepetido):
		return apperrors.Conflict("já existe modelo com este nome")
	case errors.Is(err, domain.ErrPecaNaoEncontrada):
		return apperrors.NotFound("peça não encontrada no procedimento")
	}
	return MapError(err)
}

// ListModelos lista a biblioteca (desativados só para a gestão).
func (s *Service) ListModelos(ctx context.Context, incluirInativos bool) ([]domain.Modelo, error) {
	m, err := s.repo.ListModelos(ctx, s.pool, incluirInativos)
	return m, mapModeloError(err)
}

// GetModelo devolve o modelo com o histórico de versões. Desativado
// continua visível: peças já ligadas a ele seguem apontando para cá.
func (s *Service) GetModelo(ctx context.Context, id uuid.UUID) (domain.Modelo, error) {
	m, err := s.repo.GetModelo(ctx, s.pool, id)
	return m, mapModeloError(err)
}

// ArquivoModelo abre o arquivo de uma versão (0 = a atual).
func (s *Service) ArquivoModelo(ctx context.Context, id uuid.UUID, versao int) (domain.ModeloVersao, io.ReadCloser, error) {
	v, err := s.repo.VersaoModelo(ctx, s.pool, id, versao)
	if err != nil {
		return v, nil, mapModeloError(err)
	}
	rc, err := s.store.Get(ctx, s.bucket, v.Objeto)
	return v, rc, err
}

// versaoDoArquivo confere o arquivo, grava no armazenamento e devolve a
// versão a registrar.
func (s *Service) versaoDoArquivo(ctx context.Context, modeloID uuid.UUID, a ArquivoEnviado, por string) (domain.ModeloVersao, error) {
	nome, tipo, err := domain.ValidarArquivoModelo(a.Nome, a.Conteudo)
	if err != nil {
		return domain.ModeloVersao{}, err
	}
	nota := strings.TrimSpace(a.Nota)
	if len([]rune(nota)) > 500 {
		return domain.ModeloVersao{}, domain.InvalidError{Msg: "nota da versão acima de 500 caracteres"}
	}
	soma := sha256.Sum256(a.Conteudo)
	v := domain.ModeloVersao{ArquivoNome: nome, ContentType: tipo, Tamanho: int64(len(a.Conteudo)), SHA256: hex.EncodeToString(soma[:]),
		Nota: nota, CreatedBy: por,
		Objeto: "atlas/modelos/" + modeloID.String() + "/" + uuid.NewString() + strings.ToLower(filepath.Ext(nome))}
	if err := s.store.Put(ctx, s.bucket, v.Objeto, bytes.NewReader(a.Conteudo), v.Tamanho, tipo); err != nil {
		return v, err
	}
	return v, nil
}

// registrar roda a transação que registra o arquivo já gravado; se ela
// falha, apaga o objeto.
func (s *Service) registrar(ctx context.Context, v domain.ModeloVersao, fn func(ctx context.Context, tx pgx.Tx) error) error {
	err := database.WithTx(ctx, s.pool, fn)
	if err != nil {
		if derr := s.store.Delete(ctx, s.bucket, v.Objeto); derr != nil {
			s.logger.WarnContext(ctx, "atlas: arquivo de modelo órfão no armazenamento", "objeto", v.Objeto, "error", derr)
		}
	}
	return err
}

func resumoModelo(m domain.Modelo) map[string]any {
	return map[string]any{"nome": m.Nome, "descricao": m.Descricao, "ativo": m.Ativo, "versao": m.Atual.Versao,
		"arquivo": m.Atual.ArquivoNome, "sha256": m.Atual.SHA256}
}

// CriarModelo cadastra um modelo com a versão 1 (atlas:manage na rota).
func (s *Service) CriarModelo(ctx context.Context, identity auth.Identity, m domain.Modelo, a ArquivoEnviado) (domain.Modelo, error) {
	m.Normalizar()
	if err := m.Validar(); err != nil {
		return domain.Modelo{}, MapError(err)
	}
	m.ID = uuid.New()
	por := autor(identity)
	v, err := s.versaoDoArquivo(ctx, m.ID, a, por)
	if err != nil {
		return domain.Modelo{}, mapModeloError(err)
	}
	var out domain.Modelo
	err = s.registrar(ctx, v, func(ctx context.Context, tx pgx.Tx) (err error) {
		if err := s.repo.InsertModelo(ctx, tx, m, v, por); err != nil {
			return err
		}
		if out, err = s.repo.GetModelo(ctx, tx, m.ID); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventModeloCriado, "atlas_modelo", m.ID.String(), nil, resumoModelo(out)))
	})
	return out, mapModeloError(err)
}

// NovaVersaoModelo publica um novo arquivo do modelo: todas as peças
// ligadas passam a oferecer esta versão (as anteriores ficam no histórico).
func (s *Service) NovaVersaoModelo(ctx context.Context, identity auth.Identity, id uuid.UUID, a ArquivoEnviado) (domain.Modelo, error) {
	antes, err := s.repo.GetModelo(ctx, s.pool, id)
	if err != nil {
		return domain.Modelo{}, mapModeloError(err)
	}
	v, err := s.versaoDoArquivo(ctx, id, a, autor(identity))
	if err != nil {
		return domain.Modelo{}, mapModeloError(err)
	}
	var (
		out      domain.Modelo
		enviadas []notificacoes.Enviada
	)
	err = s.registrar(ctx, v, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.InsertModeloVersao(ctx, tx, id, v); err != nil {
			return err
		}
		if out, err = s.repo.GetModelo(ctx, tx, id); err != nil {
			return err
		}
		if enviadas, err = s.avisarNovaVersaoModelo(ctx, tx, identity, out); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventModeloVersao, "atlas_modelo", id.String(),
			resumoModelo(antes), resumoModelo(out)))
	})
	if err == nil {
		s.entregar(ctx, enviadas)
	}
	return out, mapModeloError(err)
}

// AlterarModelo muda nome, descrição e situação. Desativado, o modelo sai
// da escolha de novas peças, mas segue baixável onde já está ligado.
func (s *Service) AlterarModelo(ctx context.Context, identity auth.Identity, m domain.Modelo) (domain.Modelo, error) {
	m.Normalizar()
	if err := m.Validar(); err != nil {
		return domain.Modelo{}, MapError(err)
	}
	var out domain.Modelo
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		antes, err := s.repo.GetModelo(ctx, tx, m.ID)
		if err != nil {
			return err
		}
		if err := s.repo.UpdateModelo(ctx, tx, m, autor(identity)); err != nil {
			return err
		}
		if out, err = s.repo.GetModelo(ctx, tx, m.ID); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventModeloAlterado, "atlas_modelo", m.ID.String(),
			resumoModelo(antes), resumoModelo(out)))
	})
	return out, mapModeloError(err)
}

// conferirModelos exige que as peças apontem para modelos ativos — salvo
// os que já estavam ligados na versão anterior do procedimento (permitidos):
// desativar um modelo não impede a nova versão de quem já o usava.
func (s *Service) conferirModelos(ctx context.Context, tx pgx.Tx, w domain.Workflow, permitidos map[uuid.UUID]bool) error {
	vistos := map[uuid.UUID]bool{}
	var ids []uuid.UUID
	for _, e := range w.Etapas {
		for _, d := range e.Documentos {
			if d.ModeloID != nil && !permitidos[*d.ModeloID] && !vistos[*d.ModeloID] {
				vistos[*d.ModeloID] = true
				ids = append(ids, *d.ModeloID)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	n, err := s.repo.ModelosAtivos(ctx, tx, ids)
	if err != nil {
		return err
	}
	if n != len(ids) {
		return domain.InvalidError{Msg: "modelo de documento inexistente ou desativado na peça: escolha um modelo ativo da biblioteca"}
	}
	return nil
}

// modelosDe devolve os modelos ligados às peças do procedimento.
func modelosDe(w domain.Workflow) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, e := range w.Etapas {
		for _, d := range e.Documentos {
			if d.ModeloID != nil {
				out[*d.ModeloID] = true
			}
		}
	}
	return out
}

// LigarModelo liga a peça de um procedimento a um modelo ativo da
// biblioteca (nil desliga), direto na versão em vigor: o modelo é material
// de apoio, não muda o fluxo — não exige nova versão do procedimento.
// Auditado com o modelo anterior e o novo.
func (s *Service) LigarModelo(ctx context.Context, workflowID, docID uuid.UUID, modeloID *uuid.UUID) (domain.Workflow, error) {
	var out domain.Workflow
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		antes, err := s.repo.ModeloDaPeca(ctx, tx, workflowID, docID)
		if err != nil {
			return err
		}
		if modeloID != nil {
			n, err := s.repo.ModelosAtivos(ctx, tx, []uuid.UUID{*modeloID})
			if err != nil {
				return err
			}
			if n == 0 {
				return domain.InvalidError{Msg: "modelo de documento inexistente ou desativado: escolha um modelo ativo da biblioteca"}
			}
		}
		if err := s.repo.SetModeloPeca(ctx, tx, docID, modeloID); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, workflowID, false); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventModeloPeca, "atlas_workflow", workflowID.String(),
			map[string]any{"peca": docID.String(), "modelo": idOuNada(antes)}, map[string]any{"peca": docID.String(), "modelo": idOuNada(modeloID)}))
	})
	return out, mapModeloError(err)
}

func idOuNada(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
