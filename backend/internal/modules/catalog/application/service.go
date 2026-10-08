// Package application contém os casos de uso do Catálogo.
package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// EventServicePublished é emitido ao publicar um serviço.
const EventServicePublished = "catalog.service.published"

// Service implementa os casos de uso.
type Service struct {
	pool   *pgxpool.Pool
	repo   domain.Repository
	outbox *outbox.Writer
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, ob *outbox.Writer) *Service {
	return &Service{pool: pool, repo: repo, outbox: ob}
}

// MapError traduz erros de domínio.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("serviço não encontrado")
	case errors.Is(err, domain.ErrSlugTaken):
		return apperrors.Conflict("já existe um serviço com este endereço (slug)")
	case errors.Is(err, domain.ErrUnidade), errors.Is(err, domain.ErrIncomplete):
		return apperrors.Validation(err.Error())
	case errors.Is(err, domain.ErrPublished):
		return apperrors.Conflict(err.Error())
	case errors.Is(err, domain.ErrOutOfScope):
		return apperrors.Forbidden(err.Error())
	}
	return err
}

// ListPublic lista só publicados.
func (s *Service) ListPublic(ctx context.Context, category, query string, p pagination.Params) ([]domain.Service, int64, error) {
	return s.repo.List(ctx, s.pool, domain.Filter{Status: domain.StatusPublished, Category: category, Query: query}, p)
}

// gere reporta se identity tem catalog:manage sobre um serviço com essa
// unidade responsável (nil = institucional: só a gestão global — ADR 013).
func gere(identity auth.Identity, unidade *uuid.UUID) bool {
	alvo := auth.Target{}
	if unidade != nil {
		alvo = auth.InUnidade(*unidade)
	}
	return auth.Can(identity, auth.PermCatalogManage, alvo)
}

// ListAll lista para gestão (qualquer estado) o que identity gerencia.
func (s *Service) ListAll(ctx context.Context, identity auth.Identity, status, category, query string, p pagination.Params) ([]domain.Service, int64, error) {
	if status == "" {
		status = "all"
	}
	gestao := auth.CoverageOf(identity, auth.PermCatalogManage)
	return s.repo.List(ctx, s.pool, domain.Filter{Status: status, Category: category, Query: query,
		Restrito: !gestao.All, Unidades: gestao.Unidades}, p)
}

// GetPublic devolve um serviço publicado pelo slug.
func (s *Service) GetPublic(ctx context.Context, slug string) (domain.Service, error) {
	svc, err := s.repo.GetBySlug(ctx, s.pool, slug)
	if err != nil {
		return svc, MapError(err)
	}
	if svc.Status != domain.StatusPublished {
		return domain.Service{}, MapError(domain.ErrNotFound)
	}
	return svc, nil
}

// Get devolve um serviço por id para quem o gerencia.
func (s *Service) Get(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.Service, error) {
	svc, err := s.repo.Get(ctx, s.pool, id)
	if err == nil && !gere(identity, svc.ResponsibleUnidadeID) {
		err = domain.ErrOutOfScope
	}
	return svc, MapError(err)
}

// Categories lista categorias com serviços publicados.
func (s *Service) Categories(ctx context.Context) ([]domain.Category, error) {
	return s.repo.Categories(ctx, s.pool)
}

// Save cria (id nil) ou atualiza um serviço, preservando status/publicação.
func (s *Service) Save(ctx context.Context, identity auth.Identity, id uuid.UUID, in domain.Service) (domain.Service, error) {
	// A unidade responsável (nova) precisa estar no escopo de quem grava.
	if !gere(identity, in.ResponsibleUnidadeID) {
		return domain.Service{}, MapError(domain.ErrOutOfScope)
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Slug == "" {
		in.Slug = modkit.Slugify(in.Title)
	} else {
		in.Slug = modkit.Slugify(in.Slug)
	}
	if in.Slug == "" {
		return domain.Service{}, apperrors.Validation("o título (ou o slug) precisa ter letras ou números para formar o endereço")
	}
	if in.Category == "" {
		in.Category = "Geral"
	}
	if in.Requirements == nil {
		in.Requirements = []string{}
	}
	if in.Steps == nil {
		in.Steps = []string{}
	}
	if in.Channels == nil {
		in.Channels = []domain.Channel{}
	}
	var out domain.Service
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var before any
		if id == uuid.Nil {
			in.ID = uuid.New()
			in.Status = domain.StatusDraft
		} else {
			prev, err := s.repo.Get(ctx, tx, id)
			if err != nil {
				return err
			}
			if !gere(identity, prev.ResponsibleUnidadeID) {
				return domain.ErrOutOfScope
			}
			in.ID, in.Status, in.PublishedAt = id, prev.Status, prev.PublishedAt
			before = prev
			// Publicado continua precisando do conteúdo mínimo.
			if in.Status == domain.StatusPublished {
				if err := in.ReadyToPublish(); err != nil {
					return err
				}
			}
		}
		var err error
		if out, err = s.repo.Save(ctx, tx, in); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "catalog.service.saved", "catalog_service", out.ID.String(), before, out))
	})
	return out, MapError(err)
}

// SetStatus publica/arquiva/volta a rascunho; publicar emite o evento.
func (s *Service) SetStatus(ctx context.Context, identity auth.Identity, id uuid.UUID, status string) (domain.Service, error) {
	var out domain.Service
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if !gere(identity, prev.ResponsibleUnidadeID) {
			return domain.ErrOutOfScope
		}
		out = prev
		if prev.Status == status {
			return nil // já está neste estado: nada a gravar nem auditar
		}
		if status == domain.StatusPublished {
			if err := prev.ReadyToPublish(); err != nil {
				return err
			}
		}
		next := prev
		next.Status = status
		if status == domain.StatusPublished && next.PublishedAt == nil {
			now := time.Now().UTC()
			next.PublishedAt = &now
		}
		if out, err = s.repo.Save(ctx, tx, next); err != nil {
			return err
		}
		if status == domain.StatusPublished && prev.Status != domain.StatusPublished {
			if err := s.outbox.Write(ctx, tx, EventServicePublished, "catalog_service", id.String(), uuid.Nil, map[string]any{
				"id": id.String(), "slug": out.Slug, "title": out.Title, "category": out.Category,
			}); err != nil {
				return err
			}
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "catalog.service."+status, "catalog_service", id.String(),
			map[string]string{"status": prev.Status}, map[string]string{"status": status}))
	})
	return out, MapError(err)
}

// Delete remove um serviço que não esteja publicado.
func (s *Service) Delete(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	return MapError(database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		prev, err := s.repo.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if !gere(identity, prev.ResponsibleUnidadeID) {
			return domain.ErrOutOfScope
		}
		if prev.Status == domain.StatusPublished {
			return domain.ErrPublished
		}
		if err := s.repo.Delete(ctx, tx, id); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "catalog.service.deleted", "catalog_service", id.String(), prev, nil))
	}))
}

// Search alimenta a Busca Global.
func (s *Service) Search(ctx context.Context, q string, limit int) ([]domain.Service, []float64, error) {
	return s.repo.Search(ctx, s.pool, q, limit)
}
