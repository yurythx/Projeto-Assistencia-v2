package application_test

// Código gerado por scripts/genfault.py a partir da interface Repository do
// domínio. Não edite à mão: regenere se a interface mudar.

import (
	"context"
	"errors"
	"time"

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

func (fr *faultRepo) Rma(ctx context.Context, db database.DBTX, unidades []domain.Unidade, inicio time.Time, fim time.Time) ([]domain.RmaUnidade, error) {
	if err := fr.hook("Rma"); err != nil {
		var z0 []domain.RmaUnidade
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.Rma(ctx, db, unidades, inicio, fim)
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

func (fr *faultRepo) InsertPedido(ctx context.Context, db database.DBTX, p domain.PedidoTitular) error {
	if err := fr.hook("InsertPedido"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertPedido(ctx, db, p)
}

func (fr *faultRepo) GetPedido(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.PedidoTitular, error) {
	if err := fr.hook("GetPedido"); err != nil {
		var z0 domain.PedidoTitular
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetPedido(ctx, db, id, forUpdate)
}

func (fr *faultRepo) ListPedidosPendentes(ctx context.Context, db database.DBTX) ([]domain.PedidoTitular, error) {
	if err := fr.hook("ListPedidosPendentes"); err != nil {
		var z0 []domain.PedidoTitular
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListPedidosPendentes(ctx, db)
}

func (fr *faultRepo) DecidirPedido(ctx context.Context, db database.DBTX, id uuid.UUID, status string, decididoPor uuid.UUID, justificativa string, decididoEm time.Time) error {
	if err := fr.hook("DecidirPedido"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DecidirPedido(ctx, db, id, status, decididoPor, justificativa, decididoEm)
}

func (fr *faultRepo) AnonimizarCidadao(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("AnonimizarCidadao"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.AnonimizarCidadao(ctx, db, id)
}

func (fr *faultRepo) InsertContatoRedeApoio(ctx context.Context, db database.DBTX, c domain.ContatoRedeApoio, autor uuid.UUID) error {
	if err := fr.hook("InsertContatoRedeApoio"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertContatoRedeApoio(ctx, db, c, autor)
}

func (fr *faultRepo) ListContatosRedeApoio(ctx context.Context, db database.DBTX, cidadao uuid.UUID) ([]domain.ContatoRedeApoio, error) {
	if err := fr.hook("ListContatosRedeApoio"); err != nil {
		var z0 []domain.ContatoRedeApoio
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListContatosRedeApoio(ctx, db, cidadao)
}

func (fr *faultRepo) GetContatoRedeApoio(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.ContatoRedeApoio, error) {
	if err := fr.hook("GetContatoRedeApoio"); err != nil {
		var z0 domain.ContatoRedeApoio
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetContatoRedeApoio(ctx, db, id)
}

func (fr *faultRepo) DeleteContatoRedeApoio(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteContatoRedeApoio"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteContatoRedeApoio(ctx, db, id)
}

func (fr *faultRepo) InsertBeneficio(ctx context.Context, db database.DBTX, b domain.Beneficio, autor uuid.UUID) error {
	if err := fr.hook("InsertBeneficio"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertBeneficio(ctx, db, b, autor)
}

func (fr *faultRepo) GetBeneficio(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Beneficio, error) {
	if err := fr.hook("GetBeneficio"); err != nil {
		var z0 domain.Beneficio
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetBeneficio(ctx, db, id, forUpdate)
}

func (fr *faultRepo) UpdateBeneficio(ctx context.Context, db database.DBTX, b domain.Beneficio) error {
	if err := fr.hook("UpdateBeneficio"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateBeneficio(ctx, db, b)
}

func (fr *faultRepo) BeneficiosNaArea(ctx context.Context, db database.DBTX, beneficiario uuid.UUID, area domain.Area) ([]domain.Beneficio, error) {
	if err := fr.hook("BeneficiosNaArea"); err != nil {
		var z0 []domain.Beneficio
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.BeneficiosNaArea(ctx, db, beneficiario, area)
}

func (fr *faultRepo) InsertEncaminhamento(ctx context.Context, db database.DBTX, e domain.Encaminhamento, autor uuid.UUID) error {
	if err := fr.hook("InsertEncaminhamento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertEncaminhamento(ctx, db, e, autor)
}

func (fr *faultRepo) GetEncaminhamento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Encaminhamento, error) {
	if err := fr.hook("GetEncaminhamento"); err != nil {
		var z0 domain.Encaminhamento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetEncaminhamento(ctx, db, id, forUpdate)
}

func (fr *faultRepo) UpdateEncaminhamento(ctx context.Context, db database.DBTX, e domain.Encaminhamento) error {
	if err := fr.hook("UpdateEncaminhamento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateEncaminhamento(ctx, db, e)
}

func (fr *faultRepo) EncaminhamentosDoCidadaoNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Encaminhamento, error) {
	if err := fr.hook("EncaminhamentosDoCidadaoNaArea"); err != nil {
		var z0 []domain.Encaminhamento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.EncaminhamentosDoCidadaoNaArea(ctx, db, cidadao, area)
}

func (fr *faultRepo) ListEncaminhamentosPendentes(ctx context.Context, db database.DBTX, unidadeDestino uuid.UUID) ([]domain.Encaminhamento, error) {
	if err := fr.hook("ListEncaminhamentosPendentes"); err != nil {
		var z0 []domain.Encaminhamento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListEncaminhamentosPendentes(ctx, db, unidadeDestino)
}

func (fr *faultRepo) InsertPaifVinculo(ctx context.Context, db database.DBTX, p domain.PaifVinculo, autor uuid.UUID) error {
	if err := fr.hook("InsertPaifVinculo"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertPaifVinculo(ctx, db, p, autor)
}

func (fr *faultRepo) GetPaifVinculo(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.PaifVinculo, error) {
	if err := fr.hook("GetPaifVinculo"); err != nil {
		var z0 domain.PaifVinculo
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetPaifVinculo(ctx, db, id, forUpdate)
}

func (fr *faultRepo) UpdatePaifVinculo(ctx context.Context, db database.DBTX, p domain.PaifVinculo) error {
	if err := fr.hook("UpdatePaifVinculo"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdatePaifVinculo(ctx, db, p)
}

func (fr *faultRepo) PaifVinculosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.PaifVinculo, error) {
	if err := fr.hook("PaifVinculosNaArea"); err != nil {
		var z0 []domain.PaifVinculo
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.PaifVinculosNaArea(ctx, db, cidadao, area)
}

func (fr *faultRepo) InsertConselhoAtendimento(ctx context.Context, db database.DBTX, c domain.ConselhoAtendimento, autor uuid.UUID) error {
	if err := fr.hook("InsertConselhoAtendimento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertConselhoAtendimento(ctx, db, c, autor)
}

func (fr *faultRepo) GetConselhoAtendimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.ConselhoAtendimento, error) {
	if err := fr.hook("GetConselhoAtendimento"); err != nil {
		var z0 domain.ConselhoAtendimento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetConselhoAtendimento(ctx, db, id, forUpdate)
}

func (fr *faultRepo) UpdateConselhoAtendimento(ctx context.Context, db database.DBTX, c domain.ConselhoAtendimento) error {
	if err := fr.hook("UpdateConselhoAtendimento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateConselhoAtendimento(ctx, db, c)
}

func (fr *faultRepo) ConselhoAtendimentosNaArea(ctx context.Context, db database.DBTX, crianca uuid.UUID, area domain.Area) ([]domain.ConselhoAtendimento, error) {
	if err := fr.hook("ConselhoAtendimentosNaArea"); err != nil {
		var z0 []domain.ConselhoAtendimento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ConselhoAtendimentosNaArea(ctx, db, crianca, area)
}

func (fr *faultRepo) InsertPaefiCaso(ctx context.Context, db database.DBTX, p domain.PaefiCaso, autor uuid.UUID) error {
	if err := fr.hook("InsertPaefiCaso"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertPaefiCaso(ctx, db, p, autor)
}

func (fr *faultRepo) GetPaefiCaso(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.PaefiCaso, error) {
	if err := fr.hook("GetPaefiCaso"); err != nil {
		var z0 domain.PaefiCaso
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetPaefiCaso(ctx, db, id, forUpdate)
}

func (fr *faultRepo) UpdatePaefiCaso(ctx context.Context, db database.DBTX, p domain.PaefiCaso) error {
	if err := fr.hook("UpdatePaefiCaso"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdatePaefiCaso(ctx, db, p)
}

func (fr *faultRepo) PaefiCasosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.PaefiCaso, error) {
	if err := fr.hook("PaefiCasosNaArea"); err != nil {
		var z0 []domain.PaefiCaso
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.PaefiCasosNaArea(ctx, db, cidadao, area)
}

func (fr *faultRepo) InsertMseMedida(ctx context.Context, db database.DBTX, m domain.MseMedida, autor uuid.UUID) error {
	if err := fr.hook("InsertMseMedida"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertMseMedida(ctx, db, m, autor)
}

func (fr *faultRepo) GetMseMedida(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.MseMedida, error) {
	if err := fr.hook("GetMseMedida"); err != nil {
		var z0 domain.MseMedida
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetMseMedida(ctx, db, id, forUpdate)
}

func (fr *faultRepo) UpdateMseMedida(ctx context.Context, db database.DBTX, m domain.MseMedida) error {
	if err := fr.hook("UpdateMseMedida"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateMseMedida(ctx, db, m)
}

func (fr *faultRepo) MseMedidasNaArea(ctx context.Context, db database.DBTX, adolescente uuid.UUID, area domain.Area) ([]domain.MseMedida, error) {
	if err := fr.hook("MseMedidasNaArea"); err != nil {
		var z0 []domain.MseMedida
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.MseMedidasNaArea(ctx, db, adolescente, area)
}

func (fr *faultRepo) InsertNotificacaoCreas(ctx context.Context, db database.DBTX, n domain.NotificacaoCreas, autor uuid.UUID) error {
	if err := fr.hook("InsertNotificacaoCreas"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertNotificacaoCreas(ctx, db, n, autor)
}

func (fr *faultRepo) GetNotificacaoCreas(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.NotificacaoCreas, error) {
	if err := fr.hook("GetNotificacaoCreas"); err != nil {
		var z0 domain.NotificacaoCreas
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetNotificacaoCreas(ctx, db, id, forUpdate)
}

func (fr *faultRepo) UpdateNotificacaoCreas(ctx context.Context, db database.DBTX, n domain.NotificacaoCreas) error {
	if err := fr.hook("UpdateNotificacaoCreas"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateNotificacaoCreas(ctx, db, n)
}

func (fr *faultRepo) NotificacoesCreasNaArea(ctx context.Context, db database.DBTX, vitima uuid.UUID, area domain.Area) ([]domain.NotificacaoCreas, error) {
	if err := fr.hook("NotificacoesCreasNaArea"); err != nil {
		var z0 []domain.NotificacaoCreas
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.NotificacoesCreasNaArea(ctx, db, vitima, area)
}

func (fr *faultRepo) InsertAcolhimento(ctx context.Context, db database.DBTX, a domain.Acolhimento, autor uuid.UUID) error {
	if err := fr.hook("InsertAcolhimento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertAcolhimento(ctx, db, a, autor)
}

func (fr *faultRepo) GetAcolhimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Acolhimento, error) {
	if err := fr.hook("GetAcolhimento"); err != nil {
		var z0 domain.Acolhimento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetAcolhimento(ctx, db, id, forUpdate)
}

func (fr *faultRepo) UpdateAcolhimento(ctx context.Context, db database.DBTX, a domain.Acolhimento) error {
	if err := fr.hook("UpdateAcolhimento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateAcolhimento(ctx, db, a)
}

func (fr *faultRepo) AcolhimentosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Acolhimento, error) {
	if err := fr.hook("AcolhimentosNaArea"); err != nil {
		var z0 []domain.Acolhimento
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.AcolhimentosNaArea(ctx, db, cidadao, area)
}

func (fr *faultRepo) InsertIrmao(ctx context.Context, db database.DBTX, i domain.Irmao) error {
	if err := fr.hook("InsertIrmao"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertIrmao(ctx, db, i)
}

func (fr *faultRepo) ListIrmaos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Irmao, error) {
	if err := fr.hook("ListIrmaos"); err != nil {
		var z0 []domain.Irmao
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListIrmaos(ctx, db, acolhimento)
}

func (fr *faultRepo) GetIrmao(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Irmao, error) {
	if err := fr.hook("GetIrmao"); err != nil {
		var z0 domain.Irmao
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetIrmao(ctx, db, id)
}

func (fr *faultRepo) DeleteIrmao(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteIrmao"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteIrmao(ctx, db, id)
}

func (fr *faultRepo) InsertDiagnostico(ctx context.Context, db database.DBTX, d domain.Diagnostico) error {
	if err := fr.hook("InsertDiagnostico"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertDiagnostico(ctx, db, d)
}

func (fr *faultRepo) ListDiagnosticos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Diagnostico, error) {
	if err := fr.hook("ListDiagnosticos"); err != nil {
		var z0 []domain.Diagnostico
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListDiagnosticos(ctx, db, acolhimento)
}

func (fr *faultRepo) GetDiagnostico(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Diagnostico, error) {
	if err := fr.hook("GetDiagnostico"); err != nil {
		var z0 domain.Diagnostico
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetDiagnostico(ctx, db, id)
}

func (fr *faultRepo) DeleteDiagnostico(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteDiagnostico"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteDiagnostico(ctx, db, id)
}

func (fr *faultRepo) InsertVisitante(ctx context.Context, db database.DBTX, v domain.Visitante, autor uuid.UUID) error {
	if err := fr.hook("InsertVisitante"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertVisitante(ctx, db, v, autor)
}

func (fr *faultRepo) ListVisitantes(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Visitante, error) {
	if err := fr.hook("ListVisitantes"); err != nil {
		var z0 []domain.Visitante
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListVisitantes(ctx, db, acolhimento)
}

func (fr *faultRepo) GetVisitante(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Visitante, error) {
	if err := fr.hook("GetVisitante"); err != nil {
		var z0 domain.Visitante
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetVisitante(ctx, db, id)
}

func (fr *faultRepo) DeleteVisitante(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteVisitante"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteVisitante(ctx, db, id)
}

func (fr *faultRepo) InsertTermo(ctx context.Context, db database.DBTX, t domain.Termo, autor uuid.UUID) error {
	if err := fr.hook("InsertTermo"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertTermo(ctx, db, t, autor)
}

func (fr *faultRepo) ListTermos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Termo, error) {
	if err := fr.hook("ListTermos"); err != nil {
		var z0 []domain.Termo
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListTermos(ctx, db, acolhimento)
}

func (fr *faultRepo) GetTermo(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Termo, error) {
	if err := fr.hook("GetTermo"); err != nil {
		var z0 domain.Termo
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetTermo(ctx, db, id)
}

func (fr *faultRepo) DeleteTermo(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteTermo"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteTermo(ctx, db, id)
}

func (fr *faultRepo) UpsertPiaFicha(ctx context.Context, db database.DBTX, p domain.PiaFicha, autor uuid.UUID) error {
	if err := fr.hook("UpsertPiaFicha"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpsertPiaFicha(ctx, db, p, autor)
}

func (fr *faultRepo) GetPiaFicha(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) (domain.PiaFicha, error) {
	if err := fr.hook("GetPiaFicha"); err != nil {
		var z0 domain.PiaFicha
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetPiaFicha(ctx, db, acolhimento)
}

func (fr *faultRepo) InsertPiaMeta(ctx context.Context, db database.DBTX, m domain.PiaMeta, autor uuid.UUID) error {
	if err := fr.hook("InsertPiaMeta"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertPiaMeta(ctx, db, m, autor)
}

func (fr *faultRepo) ListPiaMetas(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.PiaMeta, error) {
	if err := fr.hook("ListPiaMetas"); err != nil {
		var z0 []domain.PiaMeta
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListPiaMetas(ctx, db, acolhimento)
}

func (fr *faultRepo) GetPiaMeta(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.PiaMeta, error) {
	if err := fr.hook("GetPiaMeta"); err != nil {
		var z0 domain.PiaMeta
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetPiaMeta(ctx, db, id)
}

func (fr *faultRepo) UpdatePiaMeta(ctx context.Context, db database.DBTX, m domain.PiaMeta) error {
	if err := fr.hook("UpdatePiaMeta"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdatePiaMeta(ctx, db, m)
}

func (fr *faultRepo) DeletePiaMeta(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeletePiaMeta"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeletePiaMeta(ctx, db, id)
}

func (fr *faultRepo) InsertDocumento(ctx context.Context, db database.DBTX, d domain.DocumentoCasaAbrigo, autor uuid.UUID) error {
	if err := fr.hook("InsertDocumento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.InsertDocumento(ctx, db, d, autor)
}

func (fr *faultRepo) GetDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.DocumentoCasaAbrigo, error) {
	if err := fr.hook("GetDocumento"); err != nil {
		var z0 domain.DocumentoCasaAbrigo
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.GetDocumento(ctx, db, id)
}

func (fr *faultRepo) UpdateDocumentoStatus(ctx context.Context, db database.DBTX, id uuid.UUID, status string, sizeBytes int64, contentType string) error {
	if err := fr.hook("UpdateDocumentoStatus"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.UpdateDocumentoStatus(ctx, db, id, status, sizeBytes, contentType)
}

func (fr *faultRepo) ListDocumentos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.DocumentoCasaAbrigo, error) {
	if err := fr.hook("ListDocumentos"); err != nil {
		var z0 []domain.DocumentoCasaAbrigo
		return z0, err
	}
	defer fr.post(ctx, db)
	return fr.inner.ListDocumentos(ctx, db, acolhimento)
}

func (fr *faultRepo) DeleteDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if err := fr.hook("DeleteDocumento"); err != nil {
		return err
	}
	defer fr.post(ctx, db)
	return fr.inner.DeleteDocumento(ctx, db, id)
}
