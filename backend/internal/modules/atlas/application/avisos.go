package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
)

// Avisos de nova versão (ADR 027). O aviso é gravado na mesma transação da
// mudança (só existe se ela existir) e entregue em tempo real depois do
// commit. Quem fez a mudança não recebe o próprio aviso.

// WithNotificacoes liga a entrega em tempo real dos avisos.
func (s *Service) WithNotificacoes(n *notificacoes.Service) *Service {
	s.avisos = n
	return s
}

// entregar publica os avisos gravados (depois do commit).
func (s *Service) entregar(ctx context.Context, enviadas []notificacoes.Enviada) {
	if s.avisos != nil {
		s.avisos.Entregar(ctx, enviadas)
	}
}

// siglasDoFluxo: as siglas das unidades das etapas (e o primeiro segmento
// de "SEMAD/LIC"), em caixa alta, sem repetição.
func siglasDoFluxo(ws ...domain.Workflow) []string {
	vistas := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s != "" && !vistas[s] {
			vistas[s] = true
			out = append(out, s)
		}
	}
	for _, w := range ws {
		for _, e := range w.Etapas {
			add(e.UnidadeAdministrativa)
			add(strings.SplitN(e.UnidadeAdministrativa, "/", 2)[0])
		}
	}
	return out
}

// semAutor tira quem fez a mudança da lista.
func semAutor(usuarios []uuid.UUID, autor uuid.UUID) []uuid.UUID {
	out := usuarios[:0:0]
	for _, u := range usuarios {
		if u != autor {
			out = append(out, u)
		}
	}
	return out
}

// avisarNovaVersao grava o aviso da versão nova para quem segue o
// procedimento e para as unidades do fluxo (antigo e novo).
func (s *Service) avisarNovaVersao(ctx context.Context, tx pgx.Tx, identity auth.Identity, antiga, nova domain.Workflow) ([]notificacoes.Enviada, error) {
	usuarios, err := s.repo.Interessados(ctx, tx, nova.CodigoProcessual, siglasDoFluxo(antiga, nova))
	if err != nil {
		return nil, err
	}
	return notificacoes.Registrar(ctx, tx, semAutor(usuarios, identity.UserID), notificacoes.Nova{
		Modulo: "atlas", Titulo: fmt.Sprintf("Procedimento atualizado: %s", nova.CodigoProcessual),
		Mensagem: fmt.Sprintf("%s — publicada a versão %d (substitui a %d).", nova.Titulo, nova.Versao, antiga.Versao),
		Link:     "/atlas/procedimentos/" + nova.ID.String(), Chave: "atlas:procedimento:" + nova.ID.String(),
	})
}

// avisarNovaVersaoModelo grava o aviso para quem segue procedimentos que
// usam o modelo.
func (s *Service) avisarNovaVersaoModelo(ctx context.Context, tx pgx.Tx, identity auth.Identity, m domain.Modelo) ([]notificacoes.Enviada, error) {
	usuarios, err := s.repo.InteressadosModelo(ctx, tx, m.ID)
	if err != nil {
		return nil, err
	}
	return notificacoes.Registrar(ctx, tx, semAutor(usuarios, identity.UserID), notificacoes.Nova{
		Modulo: "atlas", Titulo: fmt.Sprintf("Modelo atualizado: %s", m.Nome),
		Mensagem: fmt.Sprintf("Publicada a versão %d (%s).", m.Atual.Versao, m.Atual.ArquivoNome),
		Link:     "/atlas/modelos?q=" + url.QueryEscape(m.Nome),
		Chave:    fmt.Sprintf("atlas:modelo:%s:%d", m.ID, m.Atual.Versao),
	})
}

// Seguimento é a resposta de seguir/deixar de seguir.
type Seguimento struct {
	CodigoProcessual string `json:"codigo_processual"`
	Seguindo         bool   `json:"seguindo"`
}

// codigoDe resolve o procedimento em vigor e exige o usuário local.
func (s *Service) codigoDe(ctx context.Context, identity auth.Identity, id uuid.UUID) (string, error) {
	if identity.UserID == uuid.Nil {
		return "", domain.InvalidError{Msg: "usuário sem cadastro local não pode seguir procedimentos"}
	}
	w, err := s.Get(ctx, id, false)
	return w.CodigoProcessual, err
}

// Seguindo diz se o usuário segue o procedimento (pelo código).
func (s *Service) Seguindo(ctx context.Context, identity auth.Identity, id uuid.UUID) (Seguimento, error) {
	codigo, err := s.codigoDe(ctx, identity, id)
	if err != nil {
		return Seguimento{}, MapError(err)
	}
	ok, err := s.repo.Seguindo(ctx, s.pool, identity.UserID, codigo)
	return Seguimento{codigo, ok}, MapError(err)
}

// Seguir passa a avisar o usuário das novas versões do procedimento.
func (s *Service) Seguir(ctx context.Context, identity auth.Identity, id uuid.UUID, seguir bool) (Seguimento, error) {
	codigo, err := s.codigoDe(ctx, identity, id)
	if err != nil {
		return Seguimento{}, MapError(err)
	}
	if seguir {
		err = s.repo.Seguir(ctx, s.pool, identity.UserID, codigo)
	} else {
		err = s.repo.DeixarDeSeguir(ctx, s.pool, identity.UserID, codigo)
	}
	return Seguimento{codigo, seguir}, MapError(err)
}
