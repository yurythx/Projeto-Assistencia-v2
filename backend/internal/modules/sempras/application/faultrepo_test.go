package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
)

type innerRepo = domain.Repository

var errBoom = errors.New("falha simulada no repositório")

// faultRepo envolve o repositório real: na chamada failAt devolve erro; depois
// da chamada poisonAt "envenena" a transação (a instrução SQL seguinte — do
// repositório, do outbox ou da auditoria — falha). noTx marca veneno em
// leitura fora de transação, onde não há instrução seguinte na mesma conexão.
type faultRepo struct {
	inner                   innerRepo
	calls, failAt, poisonAt int
	trace                   []string
	noTx                    bool
}

func (fr *faultRepo) hook(name string) error {
	fr.calls++
	fr.trace = append(fr.trace, name)
	if fr.calls == fr.failAt {
		return errBoom
	}
	return nil
}

func (fr *faultRepo) post(ctx context.Context, db database.DBTX) {
	if fr.calls != fr.poisonAt {
		return
	}
	if _, inTx := db.(pgx.Tx); !inTx {
		fr.noTx = true
		return
	}
	_, _ = db.Exec(ctx, `SELECT 1/0`)
}

func (fr *faultRepo) GetUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Unidade, error) {
	if err := fr.hook("GetUnidade"); err != nil {
		var z0 domain.Unidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetUnidade(ctx, db, id)
}

func (fr *faultRepo) ListUnidades(ctx context.Context, db database.DBTX, area domain.Area, entidadeSlug string) ([]domain.Unidade, error) {
	if err := fr.hook("ListUnidades"); err != nil {
		var z0 []domain.Unidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListUnidades(ctx, db, area, entidadeSlug)
}

func (fr *faultRepo) InsertCidadao(ctx context.Context, db database.DBTX, c domain.Cidadao, autor uuid.UUID) error {
	if err := fr.hook("InsertCidadao"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertCidadao(ctx, db, c, autor)
}

func (fr *faultRepo) UpdateCidadao(ctx context.Context, db database.DBTX, c domain.Cidadao) error {
	if err := fr.hook("UpdateCidadao"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateCidadao(ctx, db, c)
}

func (fr *faultRepo) GetCidadao(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Cidadao, error) {
	if err := fr.hook("GetCidadao"); err != nil {
		var z0 domain.Cidadao
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetCidadao(ctx, db, id, forUpdate)
}

func (fr *faultRepo) BuscarPorDocumento(ctx context.Context, db database.DBTX, digitos string) ([]domain.Cidadao, error) {
	if err := fr.hook("BuscarPorDocumento"); err != nil {
		var z0 []domain.Cidadao
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.BuscarPorDocumento(ctx, db, digitos)
}

func (fr *faultRepo) BuscarPorNome(ctx context.Context, db database.DBTX, nome string, limit int) ([]domain.Cidadao, error) {
	if err := fr.hook("BuscarPorNome"); err != nil {
		var z0 []domain.Cidadao
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.BuscarPorNome(ctx, db, nome, limit)
}

func (fr *faultRepo) HistoricoNaArea(ctx context.Context, db database.DBTX, ids []uuid.UUID, area domain.Area) (map[uuid.UUID]int, error) {
	if err := fr.hook("HistoricoNaArea"); err != nil {
		var z0 map[uuid.UUID]int
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.HistoricoNaArea(ctx, db, ids, area)
}

func (fr *faultRepo) ProximaSenha(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string) (int, error) {
	if err := fr.hook("ProximaSenha"); err != nil {
		var z0 int
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ProximaSenha(ctx, db, unidade, dia)
}

func (fr *faultRepo) InsertAtendimento(ctx context.Context, db database.DBTX, a domain.Atendimento) error {
	if err := fr.hook("InsertAtendimento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertAtendimento(ctx, db, a)
}

func (fr *faultRepo) GetAtendimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Atendimento, error) {
	if err := fr.hook("GetAtendimento"); err != nil {
		var z0 domain.Atendimento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetAtendimento(ctx, db, id, forUpdate)
}

func (fr *faultRepo) ProximoAguardando(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string, tipos []string) (domain.Atendimento, error) {
	if err := fr.hook("ProximoAguardando"); err != nil {
		var z0 domain.Atendimento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ProximoAguardando(ctx, db, unidade, dia, tipos)
}

func (fr *faultRepo) UpdateAtendimento(ctx context.Context, db database.DBTX, a domain.Atendimento) error {
	if err := fr.hook("UpdateAtendimento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateAtendimento(ctx, db, a)
}

func (fr *faultRepo) Fila(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string) ([]domain.FilaItem, error) {
	if err := fr.hook("Fila"); err != nil {
		var z0 []domain.FilaItem
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Fila(ctx, db, unidade, dia)
}

func (fr *faultRepo) AtendimentosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Atendimento, error) {
	if err := fr.hook("AtendimentosNaArea"); err != nil {
		var z0 []domain.Atendimento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.AtendimentosNaArea(ctx, db, cidadao, area)
}

func (fr *faultRepo) Painel(ctx context.Context, db database.DBTX, unidades []domain.Unidade, dia string) ([]domain.PainelUnidade, error) {
	if err := fr.hook("Painel"); err != nil {
		var z0 []domain.PainelUnidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Painel(ctx, db, unidades, dia)
}

func (fr *faultRepo) InsertRegistro(ctx context.Context, db database.DBTX, r domain.Registro) error {
	if err := fr.hook("InsertRegistro"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertRegistro(ctx, db, r)
}

func (fr *faultRepo) RegistrosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Registro, error) {
	if err := fr.hook("RegistrosNaArea"); err != nil {
		var z0 []domain.Registro
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.RegistrosNaArea(ctx, db, cidadao, area)
}
