package lgpd

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// PersonalData é implementado por plugins que guardam dados pessoais
// próprios do titular (LGPD art. 18). O pacote de exportação (acesso e
// portabilidade, II e V) inclui o que ExportPersonalData devolve, sob a
// chave do módulo; a eliminação (VI) chama ErasePersonalData NA MESMA
// transação que anonimiza a conta — ou tudo é eliminado, ou nada.
//
// Registros com valor legal (tramitações, assinaturas, trilha de
// auditoria) não são dados "do titular" neste sentido e ficam de fora:
// a conta anonimizada continua sendo a referência deles.
type PersonalData interface {
	// ExportPersonalData devolve os dados do titular (nil = nada guardado).
	ExportPersonalData(ctx context.Context, db database.DBTX, userID uuid.UUID) (any, error)
	// ErasePersonalData elimina ou anonimiza os dados do titular em tx.
	ErasePersonalData(ctx context.Context, tx database.DBTX, userID uuid.UUID) error
}

// PersonalDataSource devolve os provedores por chave de módulo — TODOS os
// registrados, ativos ou não: desativar um módulo não suspende os direitos
// do titular sobre os dados que ele já guarda.
type PersonalDataSource func() map[string]PersonalData

// SetPersonalDataSource liga o Kernel (quem conhece os plugins) ao serviço.
func (s *Service) SetPersonalDataSource(src PersonalDataSource) { s.personal = src }

// providers devolve as chaves ordenadas (ordem estável no pacote e na eliminação).
func (s *Service) providers() ([]string, map[string]PersonalData) {
	if s.personal == nil {
		return nil, nil
	}
	m := s.personal()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, m
}
