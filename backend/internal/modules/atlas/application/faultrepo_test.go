package application_test

// Código no formato de scripts/genfault.py a partir da interface Repository
// do domínio. Regenere se a interface mudar.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
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

func (fr *faultRepo) ListTTDD(ctx context.Context, db database.DBTX, f domain.FiltroTTDD, p pagination.Params) ([]domain.ClassificacaoTTDD, int64, error) {
	if err := fr.hook("ListTTDD"); err != nil {
		var z0 []domain.ClassificacaoTTDD
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListTTDD(ctx, db, f, p)
}

func (fr *faultRepo) GetTTDD(ctx context.Context, db database.DBTX, codigo string) (domain.ClassificacaoTTDD, error) {
	if err := fr.hook("GetTTDD"); err != nil {
		var z0 domain.ClassificacaoTTDD
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetTTDD(ctx, db, codigo)
}

func (fr *faultRepo) LockTTDD(ctx context.Context, db database.DBTX, codigo string) (domain.SituacaoTTDD, error) {
	if err := fr.hook("LockTTDD"); err != nil {
		var z0 domain.SituacaoTTDD
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.LockTTDD(ctx, db, codigo)
}

func (fr *faultRepo) HistoricoTTDD(ctx context.Context, db database.DBTX, codigo string) ([]domain.HistoricoTTDD, error) {
	if err := fr.hook("HistoricoTTDD"); err != nil {
		var z0 []domain.HistoricoTTDD
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.HistoricoTTDD(ctx, db, codigo)
}

func (fr *faultRepo) MaxVersao(ctx context.Context, db database.DBTX, codigo string) (int, error) {
	if err := fr.hook("MaxVersao"); err != nil {
		var z0 int
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.MaxVersao(ctx, db, codigo)
}

func (fr *faultRepo) DesativarVersoes(ctx context.Context, db database.DBTX, codigo string, exceto uuid.UUID) ([]uuid.UUID, error) {
	if err := fr.hook("DesativarVersoes"); err != nil {
		var z0 []uuid.UUID
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.DesativarVersoes(ctx, db, codigo, exceto)
}

func (fr *faultRepo) List(ctx context.Context, db database.DBTX, f domain.Filter, p pagination.Params) ([]domain.Workflow, int64, error) {
	if err := fr.hook("List"); err != nil {
		var z0 []domain.Workflow
		var z1 int64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.List(ctx, db, f, p)
}

func (fr *faultRepo) Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Workflow, error) {
	if err := fr.hook("Get"); err != nil {
		var z0 domain.Workflow
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Get(ctx, db, id, forUpdate)
}

func (fr *faultRepo) Insert(ctx context.Context, db database.DBTX, w domain.Workflow) error {
	if err := fr.hook("Insert"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Insert(ctx, db, w)
}

func (fr *faultRepo) SetAtivo(ctx context.Context, db database.DBTX, id uuid.UUID, ativo bool) error {
	if err := fr.hook("SetAtivo"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.SetAtivo(ctx, db, id, ativo)
}

func (fr *faultRepo) Search(ctx context.Context, db database.DBTX, query string, limit int) ([]domain.Workflow, []float64, error) {
	if err := fr.hook("Search"); err != nil {
		var z0 []domain.Workflow
		var z1 []float64
		return z0, z1, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Search(ctx, db, query, limit)
}

func (fr *faultRepo) EstruturaTTDD(ctx context.Context, db database.DBTX) ([]domain.EstruturaTTDD, error) {
	if err := fr.hook("EstruturaTTDD"); err != nil {
		var z0 []domain.EstruturaTTDD
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.EstruturaTTDD(ctx, db)
}

func (fr *faultRepo) CandidatosTTDD(ctx context.Context, db database.DBTX, pergunta string, limit int) ([]domain.ClassificacaoTTDD, error) {
	if err := fr.hook("CandidatosTTDD"); err != nil {
		var z0 []domain.ClassificacaoTTDD
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.CandidatosTTDD(ctx, db, pergunta, limit)
}

func (fr *faultRepo) CargaTTDD(ctx context.Context, db database.DBTX, c domain.CargaTTDD, aplicar bool) (domain.ImpactoCarga, error) {
	if err := fr.hook("CargaTTDD"); err != nil {
		var z0 domain.ImpactoCarga
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.CargaTTDD(ctx, db, c, aplicar)
}

func (fr *faultRepo) Candidatos(ctx context.Context, db database.DBTX, pergunta string, limit int) ([]domain.Workflow, error) {
	if err := fr.hook("Candidatos"); err != nil {
		var z0 []domain.Workflow
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Candidatos(ctx, db, pergunta, limit)
}

func (fr *faultRepo) ListModelos(ctx context.Context, db database.DBTX, incluirInativos bool) ([]domain.Modelo, error) {
	if err := fr.hook("ListModelos"); err != nil {
		return nil, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListModelos(ctx, db, incluirInativos)
}

func (fr *faultRepo) GetModelo(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Modelo, error) {
	if err := fr.hook("GetModelo"); err != nil {
		return domain.Modelo{}, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetModelo(ctx, db, id)
}

func (fr *faultRepo) InsertModelo(ctx context.Context, db database.DBTX, m domain.Modelo, v domain.ModeloVersao, por string) error {
	if err := fr.hook("InsertModelo"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertModelo(ctx, db, m, v, por)
}

func (fr *faultRepo) InsertModeloVersao(ctx context.Context, db database.DBTX, id uuid.UUID, v domain.ModeloVersao) (int, error) {
	if err := fr.hook("InsertModeloVersao"); err != nil {
		return 0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertModeloVersao(ctx, db, id, v)
}

func (fr *faultRepo) UpdateModelo(ctx context.Context, db database.DBTX, m domain.Modelo, por string) error {
	if err := fr.hook("UpdateModelo"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateModelo(ctx, db, m, por)
}

func (fr *faultRepo) VersaoModelo(ctx context.Context, db database.DBTX, id uuid.UUID, versao int) (domain.ModeloVersao, error) {
	if err := fr.hook("VersaoModelo"); err != nil {
		return domain.ModeloVersao{}, err
	}
	defer fr.post(ctx, db)
	return fr.inner.VersaoModelo(ctx, db, id, versao)
}

func (fr *faultRepo) ModelosAtivos(ctx context.Context, db database.DBTX, ids []uuid.UUID) (int, error) {
	if err := fr.hook("ModelosAtivos"); err != nil {
		return 0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ModelosAtivos(ctx, db, ids)
}

func (fr *faultRepo) ModeloDaPeca(ctx context.Context, db database.DBTX, workflowID, docID uuid.UUID) (*uuid.UUID, error) {
	if err := fr.hook("ModeloDaPeca"); err != nil {
		return nil, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ModeloDaPeca(ctx, db, workflowID, docID)
}

func (fr *faultRepo) SetModeloPeca(ctx context.Context, db database.DBTX, docID uuid.UUID, modeloID *uuid.UUID) error {
	if err := fr.hook("SetModeloPeca"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.SetModeloPeca(ctx, db, docID, modeloID)
}

func (fr *faultRepo) ModelosDaSerie(ctx context.Context, db database.DBTX, codigo string) ([]domain.Modelo, error) {
	if err := fr.hook("ModelosDaSerie"); err != nil {
		return nil, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ModelosDaSerie(ctx, db, codigo)
}

func (fr *faultRepo) LigarModeloSerie(ctx context.Context, db database.DBTX, codigo string, modeloID uuid.UUID, por string) error {
	if err := fr.hook("LigarModeloSerie"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.LigarModeloSerie(ctx, db, codigo, modeloID, por)
}

func (fr *faultRepo) DesligarModeloSerie(ctx context.Context, db database.DBTX, codigo string, modeloID uuid.UUID) error {
	if err := fr.hook("DesligarModeloSerie"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DesligarModeloSerie(ctx, db, codigo, modeloID)
}

func (fr *faultRepo) ProcedimentosPorOrgao(ctx context.Context, db database.DBTX) ([]domain.ProcedimentosOrgao, error) {
	if err := fr.hook("ProcedimentosPorOrgao"); err != nil {
		return nil, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ProcedimentosPorOrgao(ctx, db)
}

func (fr *faultRepo) Organograma(ctx context.Context, db database.DBTX) ([]domain.OrgaoOrganograma, error) {
	if err := fr.hook("Organograma"); err != nil {
		return nil, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Organograma(ctx, db)
}

func (fr *faultRepo) Cobertura(ctx context.Context, db database.DBTX) (domain.Cobertura, error) {
	if err := fr.hook("Cobertura"); err != nil {
		return domain.Cobertura{}, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Cobertura(ctx, db)
}

func (fr *faultRepo) Seguir(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) error {
	if err := fr.hook("Seguir"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.Seguir(ctx, db, usuario, codigo)
}

func (fr *faultRepo) DeixarDeSeguir(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) error {
	if err := fr.hook("DeixarDeSeguir"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeixarDeSeguir(ctx, db, usuario, codigo)
}

func (fr *faultRepo) Seguindo(ctx context.Context, db database.DBTX, usuario uuid.UUID, codigo string) (bool, error) {
	if err := fr.hook("Seguindo"); err != nil {
		return false, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Seguindo(ctx, db, usuario, codigo)
}

func (fr *faultRepo) Interessados(ctx context.Context, db database.DBTX, codigo string, siglas []string) ([]uuid.UUID, error) {
	if err := fr.hook("Interessados"); err != nil {
		return nil, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Interessados(ctx, db, codigo, siglas)
}

func (fr *faultRepo) InteressadosModelo(ctx context.Context, db database.DBTX, modeloID uuid.UUID) ([]uuid.UUID, error) {
	if err := fr.hook("InteressadosModelo"); err != nil {
		return nil, err
	}
	defer fr.post(ctx, db)
	return fr.inner.InteressadosModelo(ctx, db, modeloID)
}

func (fr *faultRepo) SetSituacao(ctx context.Context, db database.DBTX, id uuid.UUID, situacao string, ativo bool) error {
	if err := fr.hook("SetSituacao"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.SetSituacao(ctx, db, id, situacao, ativo)
}

func (fr *faultRepo) Validacoes(ctx context.Context, db database.DBTX, codigo string) ([]domain.Validacao, error) {
	if err := fr.hook("Validacoes"); err != nil {
		return nil, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Validacoes(ctx, db, codigo)
}

func (fr *faultRepo) InsertValidacao(ctx context.Context, db database.DBTX, v domain.Validacao) error {
	if err := fr.hook("InsertValidacao"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertValidacao(ctx, db, v)
}
