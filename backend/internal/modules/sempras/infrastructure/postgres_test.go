package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
)

// Toda falha do banco sobe como erro (nunca como sucesso vazio), e
// "nenhuma linha" vira o erro de domínio.
func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	area := domain.Area{Unidades: []uuid.UUID{id}, Entidades: []uuid.UUID{id}}
	unidades := []domain.Unidade{{ID: id}}

	calls := map[string]func(db database.DBTX) error{
		"GetUnidade":   func(db database.DBTX) error { _, err := r.GetUnidade(ctx, db, id); return err },
		"ListUnidades": func(db database.DBTX) error { _, err := r.ListUnidades(ctx, db, area, "sempras"); return err },
		"InsertCidadao": func(db database.DBTX) error {
			return r.InsertCidadao(ctx, db, domain.Cidadao{ID: id}, id)
		},
		"UpdateCidadao":      func(db database.DBTX) error { return r.UpdateCidadao(ctx, db, domain.Cidadao{ID: id}) },
		"GetCidadao":         func(db database.DBTX) error { _, err := r.GetCidadao(ctx, db, id, true); return err },
		"BuscarPorDocumento": func(db database.DBTX) error { _, err := r.BuscarPorDocumento(ctx, db, "1"); return err },
		"BuscarPorNome":      func(db database.DBTX) error { _, err := r.BuscarPorNome(ctx, db, "ma_ria%", 5); return err },
		"HistoricoNaArea": func(db database.DBTX) error {
			_, err := r.HistoricoNaArea(ctx, db, []uuid.UUID{id}, area)
			return err
		},
		"ProximaSenha":      func(db database.DBTX) error { _, err := r.ProximaSenha(ctx, db, id, "2026-01-01"); return err },
		"InsertAtendimento": func(db database.DBTX) error { return r.InsertAtendimento(ctx, db, domain.Atendimento{ID: id}) },
		"GetAtendimento":    func(db database.DBTX) error { _, err := r.GetAtendimento(ctx, db, id, true); return err },
		"ProximoAguardando": func(db database.DBTX) error {
			_, err := r.ProximoAguardando(ctx, db, id, "2026-01-01", []string{"social"})
			return err
		},
		"UpdateAtendimento": func(db database.DBTX) error { return r.UpdateAtendimento(ctx, db, domain.Atendimento{ID: id}) },
		"Fila":              func(db database.DBTX) error { _, err := r.Fila(ctx, db, id, "2026-01-01"); return err },
		"AtendimentosNaArea": func(db database.DBTX) error {
			_, err := r.AtendimentosNaArea(ctx, db, id, area)
			return err
		},
		"Painel":          func(db database.DBTX) error { _, err := r.Painel(ctx, db, unidades, "2026-01-01"); return err },
		"InsertRegistro":  func(db database.DBTX) error { return r.InsertRegistro(ctx, db, domain.Registro{ID: id}) },
		"RegistrosNaArea": func(db database.DBTX) error { _, err := r.RegistrosNaArea(ctx, db, id, area); return err },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: esperado ErrInjected, veio %v", name, err)
		}
	}
	// Linhas ilegíveis e conexão perdida no meio da leitura.
	for _, name := range []string{"GetUnidade", "ListUnidades", "GetCidadao", "BuscarPorDocumento", "BuscarPorNome",
		"HistoricoNaArea", "ProximaSenha", "GetAtendimento", "ProximoAguardando", "Fila", "AtendimentosNaArea", "Painel",
		"RegistrosNaArea"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
	}
	for _, name := range []string{"ListUnidades", "BuscarPorDocumento", "BuscarPorNome", "HistoricoNaArea", "Fila",
		"AtendimentosNaArea", "Painel", "RegistrosNaArea"} {
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com a conexão perdida na leitura: %v", name, err)
		}
	}
}

// "Nenhuma linha" vira o erro de domínio (banco de verdade).
func TestRepositoryNotFound(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	if _, err := r.GetUnidade(ctx, pool, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetUnidade: %v", err)
	}
	if _, err := r.GetCidadao(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetCidadao: %v", err)
	}
	if _, err := r.GetAtendimento(ctx, pool, id, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetAtendimento: %v", err)
	}
	if _, err := r.ProximoAguardando(ctx, pool, id, "2026-01-01", []string{"social"}); !errors.Is(err, domain.ErrFilaVazia) {
		t.Errorf("ProximoAguardando: %v", err)
	}
}
