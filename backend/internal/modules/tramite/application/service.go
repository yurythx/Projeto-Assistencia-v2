// Package application contém os casos de uso do Trâmite.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/storage"
)

// Eventos emitidos.
const (
	EventAberto      = "tramite.processo.aberto"
	EventTramitado   = "tramite.processo.tramitado"
	EventConcluido   = "tramite.processo.concluido"
	EventArquivado   = "tramite.processo.arquivado"
	attachmentPrefix = "tramite"
)

// Service implementa os casos de uso.
type Service struct {
	pool     *pgxpool.Pool
	repo     domain.Repository
	sign     domain.SignaturePort
	outbox   *outbox.Writer
	store    storage.Provider
	bucket   string
	maxBytes int64
	expiry   time.Duration
	logger   *slog.Logger
	avisos   *notificacoes.Service // avisos do Atlas (WithNotificacoes)
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, sign domain.SignaturePort, ob *outbox.Writer, store storage.Provider,
	bucket string, maxBytes int64, expiry time.Duration, logger *slog.Logger) *Service {
	return &Service{pool: pool, repo: repo, sign: sign, outbox: ob, store: store, bucket: bucket, maxBytes: maxBytes, expiry: expiry, logger: logger}
}

// MapError traduz erros de domínio.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("processo ou documento não encontrado")
	case errors.Is(err, domain.ErrForbidden):
		return apperrors.Forbidden("sem acesso a este processo (sigilo ou unidade)")
	case errors.Is(err, domain.ErrInvalidState), errors.Is(err, domain.ErrDocumentState),
		errors.Is(err, domain.ErrClosed), errors.Is(err, domain.ErrPendingSignature):
		return apperrors.Conflict(err.Error())
	case errors.Is(err, domain.ErrPublicGrant), errors.Is(err, domain.ErrInactiveTipo), errors.Is(err, domain.ErrInactiveUnidade),
		errors.Is(err, domain.ErrSerieInvalida):
		return apperrors.Validation(err.Error())
	case errors.Is(err, domain.ErrSignatureOff):
		return apperrors.FeatureDisabled(err.Error())
	}
	return err
}

func (s *Service) accessInfo(ctx context.Context, db database.DBTX, identity auth.Identity, p domain.Processo) (domain.AccessInfo, error) {
	grant, err := s.repo.HasGrant(ctx, db, p.ID, identity.UserID)
	if err != nil {
		return domain.AccessInfo{}, err
	}
	return domain.AccessInfo{Sigilo: p.Sigilo, CreatedBy: p.CreatedBy, UnidadeOrigemID: p.UnidadeOrigemID,
		UnidadeAtualID: p.UnidadeAtualID, ExplicitGrant: grant}, nil
}

// load carrega o processo exigindo leitura (act=false) ou movimentação.
func (s *Service) load(ctx context.Context, db database.DBTX, identity auth.Identity, id uuid.UUID, act, forUpdate bool) (domain.Processo, error) {
	p, err := s.repo.Get(ctx, db, id, forUpdate)
	if err != nil {
		return p, err
	}
	info, err := s.accessInfo(ctx, db, identity, p)
	if err != nil {
		return p, err
	}
	if !domain.CanRead(identity, info) {
		// Não revela a existência de um processo sigiloso.
		return domain.Processo{}, domain.ErrNotFound
	}
	if act && !domain.CanAct(identity, info) {
		return domain.Processo{}, domain.ErrForbidden
	}
	return p, nil
}

func (s *Service) event(ctx context.Context, tx pgx.Tx, eventType string, p domain.Processo, extra map[string]any) error {
	payload := map[string]any{"id": p.ID.String(), "numero": p.Numero, "status": p.Status,
		"unidade_atual_id": p.UnidadeAtualID.String(), "sigilo": p.Sigilo}
	// Assunto só sai em evento de processo público (sigilo protege o conteúdo).
	if p.Sigilo == domain.SigiloPublico {
		payload["assunto"] = p.Assunto
	}
	for k, v := range extra {
		payload[k] = v
	}
	return s.outbox.Write(ctx, tx, eventType, "tramite_processo", p.ID.String(), uuid.Nil, payload)
}

// Tipos lista os tipos de processo.
func (s *Service) Tipos(ctx context.Context) ([]domain.Tipo, error) { return s.repo.Tipos(ctx, s.pool) }

// AbrirInput abre um processo.
type AbrirInput struct {
	TipoID          uuid.UUID
	Assunto         string
	Interessado     string
	Descricao       string
	Sigilo          string
	UnidadeOrigemID uuid.UUID
	// Classificação pelo Atlas (opcional — ADR 026).
	AtlasProcedimentoID *uuid.UUID
	CodigoTTDD          string
}

// Abrir numera e abre um processo (tramite:create exigido na rota).
func (s *Service) Abrir(ctx context.Context, identity auth.Identity, in AbrirInput) (domain.Processo, error) {
	// ADR 013: tramite:create vale onde foi concedido (o Protocolo abre na
	// unidade dele e nas subunidades); tramite:manage idem.
	if origem := auth.InUnidade(in.UnidadeOrigemID); !auth.Can(identity, auth.PermTramiteCreate, origem) && !auth.Can(identity, auth.PermTramiteManage, origem) {
		return domain.Processo{}, apperrors.Forbidden("você só pode abrir processos nas unidades em que tem o perfil de protocolo")
	}
	in.CodigoTTDD = strings.TrimSpace(in.CodigoTTDD)
	if err := domain.ValidarClassificacao(in.CodigoTTDD); err != nil {
		return domain.Processo{}, MapError(err)
	}
	var out domain.Processo
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		ativo, err := s.repo.TipoAtivo(ctx, tx, in.TipoID)
		if err != nil {
			return err
		}
		if !ativo {
			return domain.ErrInactiveTipo
		}
		if ativo, err = s.repo.UnidadeAtiva(ctx, tx, in.UnidadeOrigemID); err != nil {
			return err
		}
		if !ativo {
			return domain.ErrInactiveUnidade
		}
		ano := time.Now().Year()
		seq, err := s.repo.NextNumero(ctx, tx, ano)
		if err != nil {
			return err
		}
		p := domain.Processo{ID: uuid.New(), Numero: domain.FormatNumero(seq, ano), TipoID: in.TipoID,
			Assunto: strings.TrimSpace(in.Assunto), Interessado: in.Interessado, Descricao: in.Descricao, Sigilo: in.Sigilo,
			Status: domain.StatusAberto, UnidadeOrigemID: in.UnidadeOrigemID, CreatedBy: identity.UserID,
			AtlasProcedimentoID: in.AtlasProcedimentoID, CodigoTTDD: in.CodigoTTDD}
		if err := s.repo.Insert(ctx, tx, p); err != nil {
			return err
		}
		uid := identity.UserID
		if err := s.repo.AddMovimento(ctx, tx, domain.Movimento{ProcessoID: p.ID, Acao: "abertura",
			ParaUnidadeID: &in.UnidadeOrigemID, Despacho: "Abertura do processo", ActorID: &uid}); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, p.ID, false); err != nil {
			return err
		}
		if err := s.event(ctx, tx, EventAberto, out, nil); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "tramite.processo.aberto", "tramite_processo", out.ID.String(), nil,
			map[string]any{"numero": out.Numero, "sigilo": out.Sigilo, "unidade_origem_id": out.UnidadeOrigemID}))
	})
	return out, MapError(err)
}

// List lista processos visíveis.
func (s *Service) List(ctx context.Context, identity auth.Identity, f domain.Filter, p pagination.Params) ([]domain.Processo, int64, error) {
	return s.repo.ListVisible(ctx, s.pool, identity, f, p)
}

// View é o processo completo.
type View struct {
	domain.Processo
	Documentos []domain.Documento `json:"documentos"`
	Movimentos []domain.Movimento `json:"movimentos"`
	Acessos    []domain.Grant     `json:"acessos"`
	CanAct     bool               `json:"can_act"`
	CanRoute   bool               `json:"can_route"`
}

// Get devolve o processo com documentos e histórico (acesso auditado).
func (s *Service) Get(ctx context.Context, identity auth.Identity, id uuid.UUID) (View, error) {
	p, err := s.load(ctx, s.pool, identity, id, false, false)
	if err != nil {
		return View{}, MapError(err)
	}
	info, err := s.accessInfo(ctx, s.pool, identity, p)
	if err != nil {
		return View{}, err
	}
	v := View{Processo: p, CanAct: domain.CanAct(identity, info)}
	v.CanRoute = v.CanAct && domain.PodeTramitar(identity, p.UnidadeAtualID)
	if v.Documentos, err = s.repo.Documentos(ctx, s.pool, id); err != nil {
		return View{}, err
	}
	if v.Movimentos, err = s.repo.Movimentos(ctx, s.pool, id); err != nil {
		return View{}, err
	}
	v.Acessos = []domain.Grant{}
	if p.Sigilo != domain.SigiloPublico {
		if v.Acessos, err = s.repo.Grants(ctx, s.pool, id); err != nil {
			return View{}, err
		}
		_ = audit.NewWriter(s.pool).Record(ctx, audit.Meta(ctx, "tramite.processo.acessado", "tramite_processo", id.String(), nil,
			map[string]string{"sigilo": p.Sigilo}))
	}
	return v, nil
}

// transition aplica uma mudança de estado/unidade com movimento, evento e
// auditoria na mesma transação.
func (s *Service) transition(ctx context.Context, identity auth.Identity, id uuid.UUID, acao, eventType, despacho string,
	mutate func(ctx context.Context, tx pgx.Tx, p *domain.Processo) error) (domain.Processo, error) {
	var out domain.Processo
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		p, err := s.load(ctx, tx, identity, id, true, true)
		if err != nil {
			return err
		}
		prev := p
		if err := mutate(ctx, tx, &p); err != nil {
			return err
		}
		// A unidade de destino já foi conferida e travada (UnidadeAtiva):
		// não há violação de FK possível aqui.
		if err := s.repo.Update(ctx, tx, p); err != nil {
			return err
		}
		uid := identity.UserID
		de, para := prev.UnidadeAtualID, p.UnidadeAtualID
		if err := s.repo.AddMovimento(ctx, tx, domain.Movimento{ProcessoID: id, Acao: acao, DeUnidadeID: &de,
			ParaUnidadeID: &para, Despacho: despacho, ActorID: &uid}); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, id, false); err != nil {
			return err
		}
		if eventType != "" {
			if err := s.event(ctx, tx, eventType, out, map[string]any{"de_unidade_id": de.String()}); err != nil {
				return err
			}
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "tramite.processo."+acao, "tramite_processo", id.String(),
			map[string]any{"status": prev.Status, "unidade_atual_id": prev.UnidadeAtualID},
			map[string]any{"status": out.Status, "unidade_atual_id": out.UnidadeAtualID, "despacho": despacho}))
	})
	return out, MapError(err)
}

// Tramitar encaminha o processo a outra unidade (tramite:route na rota).
func (s *Service) Tramitar(ctx context.Context, identity auth.Identity, id, para uuid.UUID, despacho string) (domain.Processo, error) {
	return s.transition(ctx, identity, id, "tramitacao", EventTramitado, despacho, func(ctx context.Context, tx pgx.Tx, p *domain.Processo) error {
		if domain.Encerrado(p.Status) {
			return domain.ErrInvalidState
		}
		if !domain.PodeTramitar(identity, p.UnidadeAtualID) {
			return domain.ErrForbidden
		}
		if p.UnidadeAtualID == para {
			return apperrors.Validation("o processo já está nesta unidade")
		}
		ativa, err := s.repo.UnidadeAtiva(ctx, tx, para)
		if err != nil {
			return err
		}
		if !ativa {
			return domain.ErrInactiveUnidade
		}
		p.UnidadeAtualID, p.Status = para, domain.StatusEmTramitacao
		return nil
	})
}

// Concluir encerra o processo.
func (s *Service) Concluir(ctx context.Context, identity auth.Identity, id uuid.UUID, despacho string) (domain.Processo, error) {
	return s.transition(ctx, identity, id, "conclusao", EventConcluido, despacho, func(ctx context.Context, tx pgx.Tx, p *domain.Processo) error {
		if domain.Encerrado(p.Status) {
			return domain.ErrInvalidState
		}
		pending, err := s.repo.PendingSignatures(ctx, tx, p.ID)
		if err != nil {
			return err
		}
		if pending > 0 {
			return domain.ErrPendingSignature
		}
		now := time.Now().UTC()
		p.Status, p.ConcluidoAt = domain.StatusConcluido, &now
		return nil
	})
}

// Arquivar arquiva um processo concluído.
func (s *Service) Arquivar(ctx context.Context, identity auth.Identity, id uuid.UUID, despacho string) (domain.Processo, error) {
	return s.transition(ctx, identity, id, "arquivamento", EventArquivado, despacho, func(_ context.Context, _ pgx.Tx, p *domain.Processo) error {
		if p.Status != domain.StatusConcluido {
			return domain.ErrInvalidState
		}
		p.Status = domain.StatusArquivado
		return nil
	})
}

// Reabrir reabre um processo concluído/arquivado (tramite:manage na rota).
func (s *Service) Reabrir(ctx context.Context, identity auth.Identity, id uuid.UUID, despacho string) (domain.Processo, error) {
	return s.transition(ctx, identity, id, "reabertura", "", despacho, func(_ context.Context, _ pgx.Tx, p *domain.Processo) error {
		// tramite:manage cobrindo a unidade em que o processo está (ADR 013).
		if !auth.Can(identity, auth.PermTramiteManage, auth.InUnidade(p.UnidadeAtualID)) {
			return domain.ErrForbidden
		}
		if !domain.Encerrado(p.Status) {
			return domain.ErrInvalidState
		}
		p.Status, p.ConcluidoAt = domain.StatusEmTramitacao, nil
		return nil
	})
}

// ConcederAcesso credencia um usuário a um processo restrito/sigiloso.
func (s *Service) ConcederAcesso(ctx context.Context, identity auth.Identity, id, userID uuid.UUID) error {
	return MapError(database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		p, err := s.load(ctx, tx, identity, id, true, true)
		if err != nil {
			return err
		}
		if p.Sigilo == domain.SigiloPublico {
			return domain.ErrPublicGrant
		}
		if err := s.repo.Grant(ctx, tx, id, userID, identity.UserID); err != nil {
			if database.IsForeignKeyViolation(err) {
				return apperrors.Validation("usuário inexistente")
			}
			return err
		}
		uid := identity.UserID
		if err := s.repo.AddMovimento(ctx, tx, domain.Movimento{ProcessoID: id, Acao: "acesso_concedido",
			Despacho: "Credencial de acesso concedida", ActorID: &uid}); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "tramite.processo.acesso_concedido", "tramite_processo", id.String(), nil,
			map[string]string{"user_id": userID.String(), "sigilo": p.Sigilo}))
	}))
}

// RevogarAcesso retira a credencial de um usuário (quem pode movimentar o
// processo decide quem mais o lê). O autor não perde o acesso: a regra de
// sigilo sempre o inclui.
func (s *Service) RevogarAcesso(ctx context.Context, identity auth.Identity, id, userID uuid.UUID) error {
	return MapError(database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		p, err := s.load(ctx, tx, identity, id, true, true)
		if err != nil {
			return err
		}
		existed, err := s.repo.Revoke(ctx, tx, id, userID)
		if err != nil {
			return err
		}
		if !existed {
			return domain.ErrNotFound
		}
		uid := identity.UserID
		if err := s.repo.AddMovimento(ctx, tx, domain.Movimento{ProcessoID: id, Acao: "acesso_revogado",
			Despacho: "Credencial de acesso revogada", ActorID: &uid}); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "tramite.processo.acesso_revogado", "tramite_processo", id.String(),
			map[string]string{"user_id": userID.String()}, map[string]string{"sigilo": p.Sigilo}))
	}))
}

// loadAberto carrega o processo para movimentação e exige que ainda esteja
// em curso (documentos e assinaturas só em processo aberto).
func (s *Service) loadAberto(ctx context.Context, db database.DBTX, identity auth.Identity, id uuid.UUID, forUpdate bool) (domain.Processo, error) {
	p, err := s.load(ctx, db, identity, id, true, forUpdate)
	if err != nil {
		return p, err
	}
	if !domain.Aberto(p.Status) {
		return domain.Processo{}, domain.ErrClosed
	}
	return p, nil
}

// ------------------------------------------------------------- documentos

// NovoDocumentoInput cria um documento redigido ou anexado.
type NovoDocumentoInput struct {
	Tipo      string
	Titulo    string
	Conteudo  string // redigido
	ObjectKey string // anexo (upload já feito)
}

// UploadAnexo emite a URL de upload direto de um anexo.
func (s *Service) UploadAnexo(ctx context.Context, identity auth.Identity, id uuid.UUID, filename, contentType string) (modkit.UploadTicket, error) {
	if _, err := s.loadAberto(ctx, s.pool, identity, id, false); err != nil {
		return modkit.UploadTicket{}, MapError(err)
	}
	return modkit.NewUpload(ctx, s.store, s.bucket, attachmentPrefix+"/"+id.String(), filename, contentType, modkit.DocumentTypes, s.expiry)
}

func (s *Service) hashObject(ctx context.Context, key string) (string, error) {
	rc, err := s.store.Get(ctx, s.bucket, key)
	if err != nil {
		return "", apperrors.DependencyUnavailable("armazenamento indisponível").WithCause(err)
	}
	defer rc.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(rc, s.maxBytes+1)); err != nil {
		return "", apperrors.DependencyUnavailable("falha ao ler o anexo").WithCause(err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// AdicionarDocumento junta um documento ao processo.
func (s *Service) AdicionarDocumento(ctx context.Context, identity auth.Identity, id uuid.UUID, in NovoDocumentoInput) (domain.Documento, error) {
	d := domain.Documento{ID: uuid.New(), ProcessoID: id, Tipo: in.Tipo, Titulo: strings.TrimSpace(in.Titulo),
		Status: "rascunho", CreatedBy: identity.UserID}
	if d.Tipo == "" {
		d.Tipo = "despacho"
	}
	// Autoriza antes de tocar no armazenamento (a checagem se repete sob
	// lock na transação).
	if _, err := s.loadAberto(ctx, s.pool, identity, id, false); err != nil {
		return domain.Documento{}, MapError(err)
	}
	if in.ObjectKey != "" {
		info, err := modkit.ConfirmUpload(ctx, s.store, s.bucket, in.ObjectKey, attachmentPrefix+"/"+id.String(), s.maxBytes, modkit.DocumentTypes)
		if err != nil {
			return domain.Documento{}, err
		}
		sum, err := s.hashObject(ctx, in.ObjectKey)
		if err != nil {
			return domain.Documento{}, err
		}
		d.Origem, d.ObjectKey, d.ContentType, d.SizeBytes, d.SHA256 = "anexo", in.ObjectKey, info.ContentType, info.Size, &sum
	} else {
		d.Origem, d.Conteudo = "redigido", in.Conteudo
	}
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.loadAberto(ctx, tx, identity, id, true); err != nil {
			return err
		}
		if err := s.repo.InsertDocumento(ctx, tx, d); err != nil {
			return err
		}
		uid := identity.UserID
		if err := s.repo.AddMovimento(ctx, tx, domain.Movimento{ProcessoID: id, Acao: "documento",
			Despacho: "Documento juntado: " + d.Titulo, ActorID: &uid}); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "tramite.documento.adicionado", "tramite_documento", d.ID.String(), nil,
			map[string]any{"processo_id": id, "origem": d.Origem, "titulo": d.Titulo, "sha256": d.SHA256}))
	})
	if err != nil {
		return domain.Documento{}, MapError(err)
	}
	out, err := s.repo.GetDocumento(ctx, s.pool, d.ID)
	return out, MapError(err)
}

// EditarDocumento altera um documento redigido em rascunho.
func (s *Service) EditarDocumento(ctx context.Context, identity auth.Identity, docID uuid.UUID, titulo, conteudo string) (domain.Documento, error) {
	var out domain.Documento
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		d, err := s.repo.GetDocumento(ctx, tx, docID)
		if err != nil {
			return err
		}
		if _, err := s.loadAberto(ctx, tx, identity, d.ProcessoID, true); err != nil {
			return err
		}
		if d.Status != "rascunho" || d.Origem != "redigido" {
			return domain.ErrDocumentState
		}
		prev := d
		d.Titulo, d.Conteudo = strings.TrimSpace(titulo), conteudo
		if err := s.repo.UpdateDocumento(ctx, tx, d); err != nil {
			return err
		}
		out = d
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "tramite.documento.editado", "tramite_documento", docID.String(),
			map[string]any{"titulo": prev.Titulo, "len": len(prev.Conteudo)}, map[string]any{"titulo": d.Titulo, "len": len(d.Conteudo)}))
	})
	return out, MapError(err)
}

// DocumentoView é o documento com o conteúdo (ou link) para leitura.
type DocumentoView struct {
	domain.Documento
	DownloadURL string `json:"download_url,omitempty"`
}

// GetDocumento lê um documento respeitando o sigilo do processo.
func (s *Service) GetDocumento(ctx context.Context, identity auth.Identity, docID uuid.UUID) (DocumentoView, error) {
	d, err := s.repo.GetDocumento(ctx, s.pool, docID)
	if err != nil {
		return DocumentoView{}, MapError(err)
	}
	if _, err := s.load(ctx, s.pool, identity, d.ProcessoID, false, false); err != nil {
		return DocumentoView{}, MapError(err)
	}
	v := DocumentoView{Documento: d}
	if d.Origem == "anexo" {
		if v.DownloadURL, err = s.store.PresignedGetURL(ctx, s.bucket, d.ObjectKey, 5*time.Minute); err != nil {
			return DocumentoView{}, apperrors.DependencyUnavailable("armazenamento indisponível").WithCause(err)
		}
	}
	return v, nil
}

// SolicitarAssinatura congela o documento (hash SHA-256) e abre o
// envelope no Signum na MESMA transação.
func (s *Service) SolicitarAssinatura(ctx context.Context, identity auth.Identity, docID uuid.UUID, signers []uuid.UUID, sequential bool) (domain.Documento, error) {
	if s.sign == nil || !s.sign.Available() {
		return domain.Documento{}, MapError(domain.ErrSignatureOff)
	}
	var out domain.Documento
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		d, err := s.repo.GetDocumento(ctx, tx, docID)
		if err != nil {
			return err
		}
		p, err := s.loadAberto(ctx, tx, identity, d.ProcessoID, true)
		if err != nil {
			return err
		}
		if d.Status != "rascunho" {
			return domain.ErrDocumentState
		}
		if d.Origem == "redigido" {
			sum := sha256.Sum256([]byte(d.Conteudo))
			h := hex.EncodeToString(sum[:])
			d.SHA256 = &h
		}
		envID, err := s.sign.OpenEnvelope(ctx, tx, domain.SignatureRequest{
			Title: p.Numero + " — " + d.Titulo, Description: "Documento do processo " + p.Numero,
			DocumentSHA256: *d.SHA256, SourceRef: d.ID.String(), SignerIDs: signers, Sequential: sequential,
		})
		if err != nil {
			return err
		}
		d.Status, d.EnvelopeID = "aguardando_assinatura", &envID
		if err := s.repo.UpdateDocumento(ctx, tx, d); err != nil {
			return err
		}
		uid := identity.UserID
		if err := s.repo.AddMovimento(ctx, tx, domain.Movimento{ProcessoID: p.ID, Acao: "assinatura_solicitada",
			Despacho: "Assinatura solicitada: " + d.Titulo, ActorID: &uid}); err != nil {
			return err
		}
		out = d
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "tramite.documento.assinatura_solicitada", "tramite_documento", d.ID.String(), nil,
			map[string]any{"envelope_id": envID, "sha256": *d.SHA256, "signers": signers}))
	})
	return out, MapError(err)
}

type envelopeEvent struct {
	EnvelopeID   uuid.UUID `json:"envelope_id"`
	Status       string    `json:"status"`
	SourceModule string    `json:"source_module"`
}

// HandleSignatureEvent reage aos eventos do Signum (consumidor do worker,
// idempotente): concluído -> documento assinado; recusado/cancelado ->
// documento volta a rascunho.
func (s *Service) HandleSignatureEvent(ctx context.Context, ev events.Event) error {
	var p envelopeEvent
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return nil // payload inválido: não há o que reprocessar
	}
	if p.SourceModule != "tramite" {
		return nil
	}
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		d, err := s.repo.DocumentoByEnvelope(ctx, tx, p.EnvelopeID)
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.Status != "aguardando_assinatura" {
			return nil // já tratado
		}
		acao, despacho := "assinatura_concluida", "Documento assinado: "+d.Titulo
		switch ev.Type {
		case "signum.envelope.completed":
			d.Status = "assinado"
		case "signum.envelope.refused", "signum.envelope.cancelled":
			d.Status, d.EnvelopeID = "rascunho", nil
			acao, despacho = "documento", "Assinatura não concluída ("+p.Status+"): "+d.Titulo
		default:
			return nil
		}
		if err := s.repo.UpdateDocumento(ctx, tx, d); err != nil {
			return err
		}
		if err := s.repo.AddMovimento(ctx, tx, domain.Movimento{ProcessoID: d.ProcessoID, Acao: acao, Despacho: despacho}); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Entry{Action: "tramite.documento." + d.Status, ResourceType: "tramite_documento",
			ResourceID: d.ID.String(), Metadata: map[string]any{"envelope_id": p.EnvelopeID, "event": ev.Type}})
	})
}

// Search alimenta a Busca Global (respeita sigilo).
func (s *Service) Search(ctx context.Context, identity auth.Identity, q string, limit int) ([]domain.Processo, error) {
	items, _, err := s.repo.ListVisible(ctx, s.pool, identity, domain.Filter{Query: q}, pagination.New(1, limit, limit))
	return items, err
}

// Classificar define (ou troca) o procedimento do Atlas e a série da TTDD
// do processo — também depois de encerrado, para a guarda (ADR 026). Exige
// poder movimentar o processo; auditado com a classificação anterior.
func (s *Service) Classificar(ctx context.Context, identity auth.Identity, id uuid.UUID, procedimento *uuid.UUID, codigo string) (domain.Processo, error) {
	codigo = strings.TrimSpace(codigo)
	if err := domain.ValidarClassificacao(codigo); err != nil {
		return domain.Processo{}, MapError(err)
	}
	var out domain.Processo
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		p, err := s.load(ctx, tx, identity, id, true, true)
		if err != nil {
			return err
		}
		antes := map[string]any{"atlas_procedimento_id": p.AtlasProcedimentoID, "codigo_ttdd": p.CodigoTTDD}
		p.AtlasProcedimentoID, p.CodigoTTDD = procedimento, codigo
		if err := s.repo.Update(ctx, tx, p); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, id, false); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "tramite.processo.classificado", "tramite_processo", id.String(), antes,
			map[string]any{"atlas_procedimento_id": out.AtlasProcedimentoID, "codigo_ttdd": out.CodigoTTDD}))
	})
	return out, MapError(err)
}
