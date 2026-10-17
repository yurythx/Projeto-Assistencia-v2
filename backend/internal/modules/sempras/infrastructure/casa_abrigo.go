package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
)

// --------------------------------------------------------------- Casa Abrigo

// dadosCols são as colunas de domain.AcolhimentoDados, na ordem de
// dadosPtrs e dadosVals (um lugar só para o assistente de 6 etapas).
var dadosCols = []string{
	"sexo", "situacao_especial", "pcd", "apelido", "identidade_genero", "orientacao_sexual", "raca_cor", "filiacao_1", "filiacao_2",
	"responsavel_nao_possui", "responsavel_nome", "responsavel_nome_social", "responsavel_apelido", "responsavel_cpf",
	"responsavel_nis", "responsavel_nascimento", "responsavel_sexo", "responsavel_vinculo", "responsavel_cep",
	"responsavel_logradouro", "responsavel_numero", "responsavel_complemento", "responsavel_bairro", "responsavel_cidade",
	"responsavel_uf", "responsavel_telefone", "responsavel_celular", "responsavel_email",
	"processo_judicial", "processo_judicial_2", "guia_acolhimento", "data_guia", "historico_acolhido_anteriormente",
	"historico_irmaos_nesta_unidade", "historico_irmaos_outra_unidade", "motivos", "outro_motivo",
	"situacao_juridica", "data_situacao_juridica", "conselheiro_nome", "conselheiro_telefone", "cna", "cncad",
	"laudo_medico", "laudo_medico_descricao", "medicacao_continua", "dependencia_quimica", "possui_transtorno", "transtorno_descricao",
	"estuda", "ameaca_morte", "recebe_visita", "beneficio_governo", "beneficio_qual", "programa_social", "relacao_trabalho",
	"observacoes_gerais",
}

func dadosPtrs(d *domain.AcolhimentoDados) []any {
	return []any{
		&d.Sexo, &d.SituacaoEspecial, &d.PCD, &d.Apelido, &d.IdentidadeGenero, &d.OrientacaoSexual, &d.RacaCor, &d.Filiacao1, &d.Filiacao2,
		&d.ResponsavelNaoPossui, &d.ResponsavelNome, &d.ResponsavelNomeSocial, &d.ResponsavelApelido, &d.ResponsavelCPF,
		&d.ResponsavelNIS, &d.ResponsavelNascimento, &d.ResponsavelSexo, &d.ResponsavelVinculo, &d.ResponsavelCEP,
		&d.ResponsavelLogradouro, &d.ResponsavelNumero, &d.ResponsavelComplemento, &d.ResponsavelBairro, &d.ResponsavelCidade,
		&d.ResponsavelUF, &d.ResponsavelTelefone, &d.ResponsavelCelular, &d.ResponsavelEmail,
		&d.ProcessoJudicial, &d.ProcessoJudicial2, &d.GuiaAcolhimento, &d.DataGuia, &d.HistoricoAcolhidoAnteriormente,
		&d.HistoricoIrmaosNestaUnidade, &d.HistoricoIrmaosOutraUnidade, &d.Motivos, &d.OutroMotivo,
		&d.SituacaoJuridica, &d.DataSituacaoJuridica, &d.ConselheiroNome, &d.ConselheiroTelefone, &d.CNA, &d.CNCAD,
		&d.LaudoMedico, &d.LaudoMedicoDescricao, &d.MedicacaoContinua, &d.DependenciaQuimica, &d.PossuiTranstorno, &d.TranstornoDescricao,
		&d.Estuda, &d.AmeacaMorte, &d.RecebeVisita, &d.BeneficioGoverno, &d.BeneficioQual, &d.ProgramaSocial, &d.RelacaoTrabalho,
		&d.Observacoes,
	}
}

func dadosVals(d domain.AcolhimentoDados) []any {
	return []any{
		d.Sexo, d.SituacaoEspecial, d.PCD, d.Apelido, d.IdentidadeGenero, d.OrientacaoSexual, d.RacaCor, d.Filiacao1, d.Filiacao2,
		d.ResponsavelNaoPossui, d.ResponsavelNome, d.ResponsavelNomeSocial, d.ResponsavelApelido, d.ResponsavelCPF,
		d.ResponsavelNIS, d.ResponsavelNascimento, d.ResponsavelSexo, d.ResponsavelVinculo, d.ResponsavelCEP,
		d.ResponsavelLogradouro, d.ResponsavelNumero, d.ResponsavelComplemento, d.ResponsavelBairro, d.ResponsavelCidade,
		d.ResponsavelUF, d.ResponsavelTelefone, d.ResponsavelCelular, d.ResponsavelEmail,
		d.ProcessoJudicial, d.ProcessoJudicial2, d.GuiaAcolhimento, d.DataGuia, d.HistoricoAcolhidoAnteriormente,
		d.HistoricoIrmaosNestaUnidade, d.HistoricoIrmaosOutraUnidade, d.Motivos, d.OutroMotivo,
		d.SituacaoJuridica, d.DataSituacaoJuridica, d.ConselheiroNome, d.ConselheiroTelefone, d.CNA, d.CNCAD,
		d.LaudoMedico, d.LaudoMedicoDescricao, d.MedicacaoContinua, d.DependenciaQuimica, d.PossuiTranstorno, d.TranstornoDescricao,
		d.Estuda, d.AmeacaMorte, d.RecebeVisita, d.BeneficioGoverno, d.BeneficioQual, d.ProgramaSocial, d.RelacaoTrabalho,
		d.Observacoes,
	}
}

// acolhimentoFixas são as colunas fora do assistente, na ordem de scanAcolhimento.
var acolhimentoFixas = []string{"id", "cidadao_id", "unidade_id", "data_entrada", "status", "desligamento_tipo",
	"motivo_desligamento", "guia_desligamento", "data_guia_desligamento", "registrado_por", "desligado_por", "data_desligamento"}

var acolhimentoCols = "a." + strings.Join(slicesConcat(acolhimentoFixas, dadosCols), ", a.")

func slicesConcat(a, b []string) []string { return append(append([]string{}, a...), b...) }

func scanAcolhimento(row pgx.Row) (domain.Acolhimento, error) {
	var a domain.Acolhimento
	dst := append([]any{&a.ID, &a.CidadaoID, &a.UnidadeID, &a.DataEntrada, &a.Status, &a.DesligamentoTipo, &a.MotivoDesligamento,
		&a.GuiaDesligamento, &a.DataGuiaDesligamento, &a.RegistradoPor, &a.DesligadoPor, &a.DataDesligamento},
		dadosPtrs(&a.AcolhimentoDados)...)
	err := row.Scan(dst...)
	return a, err
}

func placeholders(de, n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = fmt.Sprintf("$%d", de+i)
	}
	return strings.Join(p, ", ")
}

// InsertAcolhimento grava um acolhimento recém-registrado (nasce 'acolhido').
// O índice único parcial garante um acolhimento ativo por criança (R3).
func (r *Repository) InsertAcolhimento(ctx context.Context, db database.DBTX, a domain.Acolhimento, autor uuid.UUID) error {
	cols := slicesConcat([]string{"id", "cidadao_id", "unidade_id", "data_entrada", "status", "registrado_por"}, dadosCols)
	args := append([]any{a.ID, a.CidadaoID, a.UnidadeID, a.DataEntrada, a.Status, autor}, dadosVals(a.AcolhimentoDados)...)
	_, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_acolhimentos (`+strings.Join(cols, ", ")+`)
		VALUES (`+placeholders(1, len(cols))+`)`, args...)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "uq_sempras_casa_abrigo_ativo" {
		return domain.ErrAcolhimentoAtivo
	}
	if err != nil {
		return wrap("InsertAcolhimento", err)
	}
	return nil
}

// GetAcolhimento busca o acolhimento (forUpdate trava a linha na edição e no desligamento).
func (r *Repository) GetAcolhimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Acolhimento, error) {
	q := `SELECT ` + acolhimentoCols + ` FROM sempras_casa_abrigo_acolhimentos a WHERE a.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	a, err := scanAcolhimento(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Acolhimento{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Acolhimento{}, wrap("GetAcolhimento", err)
	}
	return a, nil
}

// UpdateAcolhimento grava a edição do assistente ou o desligamento.
func (r *Repository) UpdateAcolhimento(ctx context.Context, db database.DBTX, a domain.Acolhimento) error {
	cols := slicesConcat([]string{"data_entrada", "status", "desligamento_tipo", "motivo_desligamento", "guia_desligamento",
		"data_guia_desligamento", "desligado_por", "data_desligamento"}, dadosCols)
	sets := make([]string, len(cols))
	for i, c := range cols {
		sets[i] = fmt.Sprintf("%s = $%d", c, i+2)
	}
	args := append([]any{a.ID, a.DataEntrada, a.Status, a.DesligamentoTipo, a.MotivoDesligamento, a.GuiaDesligamento,
		a.DataGuiaDesligamento, a.DesligadoPor, a.DataDesligamento}, dadosVals(a.AcolhimentoDados)...)
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_acolhimentos SET `+strings.Join(sets, ", ")+` WHERE id = $1`, args...); err != nil {
		return wrap("UpdateAcolhimento", err)
	}
	return nil
}

// AcolhimentosNaArea lista os acolhimentos do munícipe nas unidades da área de quem consulta.
func (r *Repository) AcolhimentosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Acolhimento, error) {
	rows, err := db.Query(ctx, `SELECT `+acolhimentoCols+` FROM sempras_casa_abrigo_acolhimentos a JOIN unidades u ON u.id = a.unidade_id
		WHERE a.cidadao_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY a.data_entrada DESC LIMIT 200`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("AcolhimentosNaArea", err)
	}
	return collect(rows, "AcolhimentosNaArea", scanAcolhimento)
}

// AcolhidosDaUnidade lista os acolhimentos da unidade (ativos e
// desligados) com o que o relatório de institucionalizados, o painel e as
// estatísticas precisam — sem o assistente inteiro.
func (r *Repository) AcolhidosDaUnidade(ctx context.Context, db database.DBTX, unidade uuid.UUID) ([]domain.AcolhidoResumo, error) {
	rows, err := db.Query(ctx, `SELECT a.id, a.cidadao_id, c.nome, c.nome_social, COALESCE(to_char(c.data_nascimento, 'YYYY-MM-DD'), ''),
			a.sexo, a.data_entrada, a.processo_judicial, a.guia_acolhimento, a.situacao_juridica, a.motivos, a.status,
			a.desligamento_tipo, a.data_desligamento, p.atualizado_em,
			EXISTS (SELECT 1 FROM sempras_casa_abrigo_documentos d WHERE d.acolhimento_id = a.id AND d.tipo = 'acolhido' AND d.status = 'ready'),
			EXISTS (SELECT 1 FROM sempras_casa_abrigo_documentos d WHERE d.acolhimento_id = a.id AND d.tipo = 'responsavel' AND d.status = 'ready')
		FROM sempras_casa_abrigo_acolhimentos a
		JOIN sempras_cidadaos c ON c.id = a.cidadao_id
		LEFT JOIN sempras_casa_abrigo_pia p ON p.acolhimento_id = a.id
		WHERE a.unidade_id = $1
		ORDER BY (a.status = 'acolhido') DESC, lower(c.nome), a.data_entrada DESC LIMIT 2000`, unidade)
	if err != nil {
		return nil, wrap("AcolhidosDaUnidade", err)
	}
	return collect(rows, "AcolhidosDaUnidade", func(row pgx.Row) (domain.AcolhidoResumo, error) {
		var a domain.AcolhidoResumo
		err := row.Scan(&a.AcolhimentoID, &a.CidadaoID, &a.Nome, &a.NomeSocial, &a.DataNascimento, &a.Sexo, &a.DataEntrada,
			&a.ProcessoJudicial, &a.GuiaAcolhimento, &a.SituacaoJuridica, &a.Motivos, &a.Status, &a.DesligamentoTipo,
			&a.DataDesligamento, &a.PiaAtualizadoEm, &a.DocumentoAcolhido, &a.DocumentoResponsavel)
		return a, err
	})
}

const irmaoCols = `i.id, i.acolhimento_id, i.nome, i.data_nascimento, i.esta_acolhido, i.unidade_descricao, i.observacao, i.created_at`

func scanIrmao(row pgx.Row) (domain.Irmao, error) {
	var i domain.Irmao
	err := row.Scan(&i.ID, &i.AcolhimentoID, &i.Nome, &i.DataNascimento, &i.EstaAcolhido, &i.UnidadeDescricao, &i.Observacao, &i.CreatedAt)
	return i, err
}

// InsertIrmao grava um irmão do acolhido.
func (r *Repository) InsertIrmao(ctx context.Context, db database.DBTX, i domain.Irmao) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_irmaos
		(id, acolhimento_id, nome, data_nascimento, esta_acolhido, unidade_descricao, observacao, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		i.ID, i.AcolhimentoID, i.Nome, i.DataNascimento, i.EstaAcolhido, i.UnidadeDescricao, i.Observacao, i.CreatedAt); err != nil {
		return wrap("InsertIrmao", err)
	}
	return nil
}

// ListIrmaos lista os irmãos registrados de um acolhimento.
func (r *Repository) ListIrmaos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Irmao, error) {
	rows, err := db.Query(ctx, `SELECT `+irmaoCols+` FROM sempras_casa_abrigo_irmaos i
		WHERE i.acolhimento_id = $1 ORDER BY i.created_at`, acolhimento)
	if err != nil {
		return nil, wrap("ListIrmaos", err)
	}
	return collect(rows, "ListIrmaos", scanIrmao)
}

// GetIrmao busca um irmão (confere a quem pertence antes de remover).
func (r *Repository) GetIrmao(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Irmao, error) {
	i, err := scanIrmao(db.QueryRow(ctx, `SELECT `+irmaoCols+` FROM sempras_casa_abrigo_irmaos i WHERE i.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Irmao{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Irmao{}, wrap("GetIrmao", err)
	}
	return i, nil
}

// DeleteIrmao remove um irmão (cadastrado errado, por exemplo).
func (r *Repository) DeleteIrmao(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_irmaos WHERE id = $1`, id); err != nil {
		return wrap("DeleteIrmao", err)
	}
	return nil
}

const diagnosticoCols = `d.id, d.acolhimento_id, d.categoria, d.diagnostico, d.descricao, d.created_at`

func scanDiagnostico(row pgx.Row) (domain.Diagnostico, error) {
	var d domain.Diagnostico
	err := row.Scan(&d.ID, &d.AcolhimentoID, &d.Categoria, &d.Diagnostico, &d.Descricao, &d.CreatedAt)
	return d, err
}

// InsertDiagnostico grava um diagnóstico do acolhido.
func (r *Repository) InsertDiagnostico(ctx context.Context, db database.DBTX, d domain.Diagnostico) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_diagnosticos
		(id, acolhimento_id, categoria, diagnostico, descricao, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		d.ID, d.AcolhimentoID, d.Categoria, d.Diagnostico, d.Descricao, d.CreatedAt); err != nil {
		return wrap("InsertDiagnostico", err)
	}
	return nil
}

// ListDiagnosticos lista os diagnósticos registrados de um acolhimento.
func (r *Repository) ListDiagnosticos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Diagnostico, error) {
	rows, err := db.Query(ctx, `SELECT `+diagnosticoCols+` FROM sempras_casa_abrigo_diagnosticos d
		WHERE d.acolhimento_id = $1 ORDER BY d.created_at`, acolhimento)
	if err != nil {
		return nil, wrap("ListDiagnosticos", err)
	}
	return collect(rows, "ListDiagnosticos", scanDiagnostico)
}

// GetDiagnostico busca um diagnóstico (confere a quem pertence antes de remover).
func (r *Repository) GetDiagnostico(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Diagnostico, error) {
	d, err := scanDiagnostico(db.QueryRow(ctx, `SELECT `+diagnosticoCols+` FROM sempras_casa_abrigo_diagnosticos d WHERE d.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Diagnostico{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Diagnostico{}, wrap("GetDiagnostico", err)
	}
	return d, nil
}

// DeleteDiagnostico remove um diagnóstico (cadastrado errado, por exemplo).
func (r *Repository) DeleteDiagnostico(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_diagnosticos WHERE id = $1`, id); err != nil {
		return wrap("DeleteDiagnostico", err)
	}
	return nil
}

// --------------------------------------------- Casa Abrigo: visitantes

const visitanteCols = `v.id, v.unidade_id, v.acolhimento_id, v.tipo_visita, v.nome_visitante, v.documento_identidade,
	v.parentesco_vinculo, v.data_visita, COALESCE(to_char(v.hora_entrada, 'HH24:MI'), ''),
	COALESCE(to_char(v.hora_saida, 'HH24:MI'), ''), v.observacoes, v.created_at`

func scanVisitante(row pgx.Row) (domain.Visitante, error) {
	var v domain.Visitante
	err := row.Scan(&v.ID, &v.UnidadeID, &v.AcolhimentoID, &v.TipoVisita, &v.NomeVisitante, &v.DocumentoIdentidade,
		&v.ParentescoVinculo, &v.DataVisita, &v.HoraEntrada, &v.HoraSaida, &v.Observacoes, &v.CreatedAt)
	return v, err
}

// InsertVisitante grava uma visita recebida pela unidade.
func (r *Repository) InsertVisitante(ctx context.Context, db database.DBTX, v domain.Visitante, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_visitantes
		(id, unidade_id, acolhimento_id, tipo_visita, nome_visitante, documento_identidade, parentesco_vinculo, data_visita,
		 hora_entrada, hora_saida, observacoes, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::time, NULLIF($10, '')::time, $11, $12, $13)`,
		v.ID, v.UnidadeID, v.AcolhimentoID, v.TipoVisita, v.NomeVisitante, v.DocumentoIdentidade, v.ParentescoVinculo,
		v.DataVisita, v.HoraEntrada, v.HoraSaida, v.Observacoes, autor, v.CreatedAt); err != nil {
		return wrap("InsertVisitante", err)
	}
	return nil
}

// UpdateVisitante grava a saída do visitante.
func (r *Repository) UpdateVisitante(ctx context.Context, db database.DBTX, v domain.Visitante) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_visitantes SET hora_saida = NULLIF($2, '')::time WHERE id = $1`,
		v.ID, v.HoraSaida); err != nil {
		return wrap("UpdateVisitante", err)
	}
	return nil
}

// ListVisitantes lista as visitas registradas de um acolhimento.
func (r *Repository) ListVisitantes(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Visitante, error) {
	rows, err := db.Query(ctx, `SELECT `+visitanteCols+` FROM sempras_casa_abrigo_visitantes v
		WHERE v.acolhimento_id = $1 ORDER BY v.data_visita DESC, v.created_at DESC`, acolhimento)
	if err != nil {
		return nil, wrap("ListVisitantes", err)
	}
	return collect(rows, "ListVisitantes", scanVisitante)
}

// ListVisitantesDaUnidade lista as visitas da unidade (as mais recentes), com o nome do acolhido.
func (r *Repository) ListVisitantesDaUnidade(ctx context.Context, db database.DBTX, unidade uuid.UUID) ([]domain.Visitante, error) {
	rows, err := db.Query(ctx, `SELECT `+visitanteCols+`, COALESCE(NULLIF(c.nome_social, ''), c.nome, '')
		FROM sempras_casa_abrigo_visitantes v
		LEFT JOIN sempras_casa_abrigo_acolhimentos a ON a.id = v.acolhimento_id
		LEFT JOIN sempras_cidadaos c ON c.id = a.cidadao_id
		WHERE v.unidade_id = $1 ORDER BY v.data_visita DESC, v.created_at DESC LIMIT 300`, unidade)
	if err != nil {
		return nil, wrap("ListVisitantesDaUnidade", err)
	}
	return collect(rows, "ListVisitantesDaUnidade", func(row pgx.Row) (domain.Visitante, error) {
		var v domain.Visitante
		err := row.Scan(&v.ID, &v.UnidadeID, &v.AcolhimentoID, &v.TipoVisita, &v.NomeVisitante, &v.DocumentoIdentidade,
			&v.ParentescoVinculo, &v.DataVisita, &v.HoraEntrada, &v.HoraSaida, &v.Observacoes, &v.CreatedAt, &v.AcolhidoNome)
		return v, err
	})
}

// GetVisitante busca uma visita (confere a quem pertence antes de alterar).
func (r *Repository) GetVisitante(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Visitante, error) {
	v, err := scanVisitante(db.QueryRow(ctx, `SELECT `+visitanteCols+` FROM sempras_casa_abrigo_visitantes v WHERE v.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Visitante{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Visitante{}, wrap("GetVisitante", err)
	}
	return v, nil
}

// DeleteVisitante remove uma visita (cadastrada errada, por exemplo).
func (r *Repository) DeleteVisitante(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_visitantes WHERE id = $1`, id); err != nil {
		return wrap("DeleteVisitante", err)
	}
	return nil
}

// -------------------------------------------------- Casa Abrigo: termos

const termoCols = `t.id, t.acolhimento_id, t.tipo_termo, t.responsavel_retirada, t.parentesco_responsavel,
	t.motivo_observacao, t.data_termo, t.created_at`

func scanTermo(row pgx.Row) (domain.Termo, error) {
	var t domain.Termo
	err := row.Scan(&t.ID, &t.AcolhimentoID, &t.TipoTermo, &t.ResponsavelRetirada, &t.ParentescoResponsavel,
		&t.MotivoObservacao, &t.DataTermo, &t.CreatedAt)
	return t, err
}

// InsertTermo grava um termo de advertência, passeio ou saída.
func (r *Repository) InsertTermo(ctx context.Context, db database.DBTX, t domain.Termo, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_termos
		(id, acolhimento_id, tipo_termo, responsavel_retirada, parentesco_responsavel, motivo_observacao, data_termo, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.AcolhimentoID, t.TipoTermo, t.ResponsavelRetirada, t.ParentescoResponsavel, t.MotivoObservacao,
		t.DataTermo, autor, t.CreatedAt); err != nil {
		return wrap("InsertTermo", err)
	}
	return nil
}

// ListTermos lista os termos registrados de um acolhimento.
func (r *Repository) ListTermos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Termo, error) {
	rows, err := db.Query(ctx, `SELECT `+termoCols+` FROM sempras_casa_abrigo_termos t
		WHERE t.acolhimento_id = $1 ORDER BY t.data_termo DESC, t.created_at DESC`, acolhimento)
	if err != nil {
		return nil, wrap("ListTermos", err)
	}
	return collect(rows, "ListTermos", scanTermo)
}

// ListTermosDaUnidade lista os termos da unidade (os mais recentes), com o nome do acolhido.
func (r *Repository) ListTermosDaUnidade(ctx context.Context, db database.DBTX, unidade uuid.UUID) ([]domain.Termo, error) {
	rows, err := db.Query(ctx, `SELECT `+termoCols+`, COALESCE(NULLIF(c.nome_social, ''), c.nome)
		FROM sempras_casa_abrigo_termos t
		JOIN sempras_casa_abrigo_acolhimentos a ON a.id = t.acolhimento_id
		JOIN sempras_cidadaos c ON c.id = a.cidadao_id
		WHERE a.unidade_id = $1 ORDER BY t.data_termo DESC, t.created_at DESC LIMIT 300`, unidade)
	if err != nil {
		return nil, wrap("ListTermosDaUnidade", err)
	}
	return collect(rows, "ListTermosDaUnidade", func(row pgx.Row) (domain.Termo, error) {
		var t domain.Termo
		err := row.Scan(&t.ID, &t.AcolhimentoID, &t.TipoTermo, &t.ResponsavelRetirada, &t.ParentescoResponsavel,
			&t.MotivoObservacao, &t.DataTermo, &t.CreatedAt, &t.AcolhidoNome)
		return t, err
	})
}

// GetTermo busca um termo (confere a quem pertence antes de remover).
func (r *Repository) GetTermo(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Termo, error) {
	t, err := scanTermo(db.QueryRow(ctx, `SELECT `+termoCols+` FROM sempras_casa_abrigo_termos t WHERE t.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Termo{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Termo{}, wrap("GetTermo", err)
	}
	return t, nil
}

// DeleteTermo remove um termo (cadastrado errado, por exemplo).
func (r *Repository) DeleteTermo(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_termos WHERE id = $1`, id); err != nil {
		return wrap("DeleteTermo", err)
	}
	return nil
}

// ------------------------------------------------------ Casa Abrigo: PIA

var piaCols = []string{"naturalidade", "nacionalidade", "religiao", "doc_carteira_vacinacao", "doc_certidao_nascimento",
	"doc_rg_cpf", "doc_cartao_sus", "doc_nis", "sinais_violencia_fisica", "medicamentos", "alergias", "vacinacao_atualizada",
	"cartao_sus", "tratamento_odontologico", "tratamento_medico_psicologico", "deficiencias_restricoes", "escola", "serie_escolar",
	"matricula_escolar", "periodo_escolar", "frequencia_escolar", "aula_reforco", "historico_familiar", "possui_beneficios",
	"moradia_anterior", "contato_familiar", "visitas_autorizadas", "observacoes_familia", "atividades_manha", "atividades_tarde",
	"atividades_noite", "atividades_fim_semana", "observacoes_comportamentais", "intervencao_objetivos",
	"intervencao_estrategias", "intervencao_metas_prazos"}

func piaPtrs(d *domain.PiaFichaDados) []any {
	return []any{&d.Naturalidade, &d.Nacionalidade, &d.Religiao, &d.DocCarteiraVacinacao, &d.DocCertidaoNascimento,
		&d.DocRgCpf, &d.DocCartaoSUS, &d.DocNIS, &d.SinaisViolenciaFisica, &d.Medicamentos, &d.Alergias, &d.VacinacaoAtualizada,
		&d.CartaoSUS, &d.TratamentoOdontologico, &d.TratamentoMedicoPsicologico, &d.DeficienciasRestricoes, &d.Escola, &d.SerieEscolar,
		&d.MatriculaEscolar, &d.PeriodoEscolar, &d.FrequenciaEscolar, &d.AulaReforco, &d.HistoricoFamiliar, &d.PossuiBeneficios,
		&d.MoradiaAnterior, &d.ContatoFamiliar, &d.VisitasAutorizadas, &d.ObservacoesFamilia, &d.AtividadesManha, &d.AtividadesTarde,
		&d.AtividadesNoite, &d.AtividadesFimSemana, &d.ObservacoesComportamentais, &d.IntervencaoObjetivos,
		&d.IntervencaoEstrategias, &d.IntervencaoMetasPrazos}
}

func piaVals(d domain.PiaFichaDados) []any {
	return []any{d.Naturalidade, d.Nacionalidade, d.Religiao, d.DocCarteiraVacinacao, d.DocCertidaoNascimento,
		d.DocRgCpf, d.DocCartaoSUS, d.DocNIS, d.SinaisViolenciaFisica, d.Medicamentos, d.Alergias, d.VacinacaoAtualizada,
		d.CartaoSUS, d.TratamentoOdontologico, d.TratamentoMedicoPsicologico, d.DeficienciasRestricoes, d.Escola, d.SerieEscolar,
		d.MatriculaEscolar, d.PeriodoEscolar, d.FrequenciaEscolar, d.AulaReforco, d.HistoricoFamiliar, d.PossuiBeneficios,
		d.MoradiaAnterior, d.ContatoFamiliar, d.VisitasAutorizadas, d.ObservacoesFamilia, d.AtividadesManha, d.AtividadesTarde,
		d.AtividadesNoite, d.AtividadesFimSemana, d.ObservacoesComportamentais, d.IntervencaoObjetivos,
		d.IntervencaoEstrategias, d.IntervencaoMetasPrazos}
}

// UpsertPiaFicha grava ou atualiza a ficha 1:1 do PIA (as 6 abas).
func (r *Repository) UpsertPiaFicha(ctx context.Context, db database.DBTX, p domain.PiaFicha, autor uuid.UUID) error {
	cols := slicesConcat([]string{"acolhimento_id", "atualizado_por", "atualizado_em"}, piaCols)
	sets := make([]string, 0, len(cols)-1)
	for i, c := range cols[1:] {
		sets = append(sets, fmt.Sprintf("%s = $%d", c, i+2))
	}
	args := append([]any{p.AcolhimentoID, autor, p.AtualizadoEm}, piaVals(p.PiaFichaDados)...)
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_pia (`+strings.Join(cols, ", ")+`)
		VALUES (`+placeholders(1, len(cols))+`)
		ON CONFLICT (acolhimento_id) DO UPDATE SET `+strings.Join(sets, ", "), args...); err != nil {
		return wrap("UpsertPiaFicha", err)
	}
	return nil
}

// GetPiaFicha busca a ficha do PIA de um acolhimento (ErrNotFound se ainda não foi preenchida).
func (r *Repository) GetPiaFicha(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) (domain.PiaFicha, error) {
	var p domain.PiaFicha
	dst := append([]any{&p.AcolhimentoID, &p.AtualizadoPor, &p.AtualizadoEm}, piaPtrs(&p.PiaFichaDados)...)
	err := db.QueryRow(ctx, `SELECT acolhimento_id, atualizado_por, atualizado_em, `+strings.Join(piaCols, ", ")+`
		FROM sempras_casa_abrigo_pia WHERE acolhimento_id = $1`, acolhimento).Scan(dst...)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PiaFicha{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PiaFicha{}, wrap("GetPiaFicha", err)
	}
	return p, nil
}

const piaMetaCols = `m.id, m.acolhimento_id, m.tipo, m.descricao, m.providencia, m.responsavel, m.prazo, m.esportes, m.cultura,
	m.lazer, m.concluida, m.created_at`

func scanPiaMeta(row pgx.Row) (domain.PiaMeta, error) {
	var m domain.PiaMeta
	err := row.Scan(&m.ID, &m.AcolhimentoID, &m.Tipo, &m.Descricao, &m.Providencia, &m.Responsavel, &m.Prazo, &m.Esportes,
		&m.Cultura, &m.Lazer, &m.Concluida, &m.CreatedAt)
	return m, err
}

// InsertPiaMeta grava uma meta do plano ou da intervenção do PIA.
func (r *Repository) InsertPiaMeta(ctx context.Context, db database.DBTX, m domain.PiaMeta, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_pia_metas
		(id, acolhimento_id, tipo, descricao, providencia, responsavel, prazo, esportes, cultura, lazer, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		m.ID, m.AcolhimentoID, m.Tipo, m.Descricao, m.Providencia, m.Responsavel, m.Prazo, m.Esportes, m.Cultura, m.Lazer,
		autor, m.CreatedAt); err != nil {
		return wrap("InsertPiaMeta", err)
	}
	return nil
}

// ListPiaMetas lista as metas do PIA de um acolhimento.
func (r *Repository) ListPiaMetas(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.PiaMeta, error) {
	rows, err := db.Query(ctx, `SELECT `+piaMetaCols+` FROM sempras_casa_abrigo_pia_metas m
		WHERE m.acolhimento_id = $1 ORDER BY m.created_at`, acolhimento)
	if err != nil {
		return nil, wrap("ListPiaMetas", err)
	}
	return collect(rows, "ListPiaMetas", scanPiaMeta)
}

// MetasPendentesDaUnidade lista as metas abertas, com prazo, dos acolhidos ativos (monitor).
func (r *Repository) MetasPendentesDaUnidade(ctx context.Context, db database.DBTX, unidade uuid.UUID) ([]domain.MetaPendente, error) {
	rows, err := db.Query(ctx, `SELECT `+piaMetaCols+`, COALESCE(NULLIF(c.nome_social, ''), c.nome)
		FROM sempras_casa_abrigo_pia_metas m
		JOIN sempras_casa_abrigo_acolhimentos a ON a.id = m.acolhimento_id
		JOIN sempras_cidadaos c ON c.id = a.cidadao_id
		WHERE a.unidade_id = $1 AND a.status = 'acolhido' AND NOT m.concluida AND m.prazo IS NOT NULL
		ORDER BY m.prazo LIMIT 500`, unidade)
	if err != nil {
		return nil, wrap("MetasPendentesDaUnidade", err)
	}
	return collect(rows, "MetasPendentesDaUnidade", func(row pgx.Row) (domain.MetaPendente, error) {
		var m domain.MetaPendente
		err := row.Scan(&m.ID, &m.AcolhimentoID, &m.Tipo, &m.Descricao, &m.Providencia, &m.Responsavel, &m.Prazo, &m.Esportes,
			&m.Cultura, &m.Lazer, &m.Concluida, &m.CreatedAt, &m.AcolhidoNome)
		return m, err
	})
}

// GetPiaMeta busca uma meta (confere a quem pertence antes de alterar).
func (r *Repository) GetPiaMeta(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.PiaMeta, error) {
	m, err := scanPiaMeta(db.QueryRow(ctx, `SELECT `+piaMetaCols+` FROM sempras_casa_abrigo_pia_metas m WHERE m.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PiaMeta{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PiaMeta{}, wrap("GetPiaMeta", err)
	}
	return m, nil
}

// UpdatePiaMeta grava a edição ou a conclusão de uma meta.
func (r *Repository) UpdatePiaMeta(ctx context.Context, db database.DBTX, m domain.PiaMeta) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_pia_metas SET tipo = $2, descricao = $3, providencia = $4,
		responsavel = $5, prazo = $6, esportes = $7, cultura = $8, lazer = $9, concluida = $10 WHERE id = $1`,
		m.ID, m.Tipo, m.Descricao, m.Providencia, m.Responsavel, m.Prazo, m.Esportes, m.Cultura, m.Lazer, m.Concluida); err != nil {
		return wrap("UpdatePiaMeta", err)
	}
	return nil
}

// DeletePiaMeta remove uma meta (cadastrada errada, por exemplo).
func (r *Repository) DeletePiaMeta(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_pia_metas WHERE id = $1`, id); err != nil {
		return wrap("DeletePiaMeta", err)
	}
	return nil
}

// ------------------------------------------------- Casa Abrigo: documentos

const documentoCols = `d.id, d.acolhimento_id, d.tipo, d.nome_original, d.object_key, d.content_type, d.size_bytes,
	d.status, d.created_at`

func scanDocumentoCasaAbrigo(row pgx.Row) (domain.DocumentoCasaAbrigo, error) {
	var d domain.DocumentoCasaAbrigo
	err := row.Scan(&d.ID, &d.AcolhimentoID, &d.Tipo, &d.NomeOriginal, &d.ObjectKey, &d.ContentType, &d.SizeBytes,
		&d.Status, &d.CreatedAt)
	return d, err
}

// InsertDocumento grava o metadado pendente (o navegador ainda está
// enviando o conteúdo ao object storage).
func (r *Repository) InsertDocumento(ctx context.Context, db database.DBTX, d domain.DocumentoCasaAbrigo, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_documentos
		(id, acolhimento_id, tipo, nome_original, object_key, content_type, size_bytes, status, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		d.ID, d.AcolhimentoID, d.Tipo, d.NomeOriginal, d.ObjectKey, d.ContentType, d.SizeBytes, d.Status, autor, d.CreatedAt); err != nil {
		return wrap("InsertDocumento", err)
	}
	return nil
}

// GetDocumento busca o documento (confere o dono/tamanho antes de confirmar ou remover).
func (r *Repository) GetDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.DocumentoCasaAbrigo, error) {
	d, err := scanDocumentoCasaAbrigo(db.QueryRow(ctx, `SELECT `+documentoCols+` FROM sempras_casa_abrigo_documentos d WHERE d.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DocumentoCasaAbrigo{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.DocumentoCasaAbrigo{}, wrap("GetDocumento", err)
	}
	return d, nil
}

// UpdateDocumentoStatus grava a confirmação do upload (tamanho e tipo
// reais, conferidos no object storage — o que o navegador declarou não é confiável).
func (r *Repository) UpdateDocumentoStatus(ctx context.Context, db database.DBTX, id uuid.UUID, status string, sizeBytes int64, contentType string) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_documentos SET status = $2, size_bytes = $3, content_type = $4 WHERE id = $1`,
		id, status, sizeBytes, contentType); err != nil {
		return wrap("UpdateDocumentoStatus", err)
	}
	return nil
}

// ListDocumentos lista os documentos de um acolhimento.
func (r *Repository) ListDocumentos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.DocumentoCasaAbrigo, error) {
	rows, err := db.Query(ctx, `SELECT `+documentoCols+` FROM sempras_casa_abrigo_documentos d
		WHERE d.acolhimento_id = $1 ORDER BY d.created_at DESC`, acolhimento)
	if err != nil {
		return nil, wrap("ListDocumentos", err)
	}
	return collect(rows, "ListDocumentos", scanDocumentoCasaAbrigo)
}

// DeleteDocumento remove o metadado (a remoção do objeto no storage é
// feita pela aplicação, fora da transação, best-effort).
func (r *Repository) DeleteDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_documentos WHERE id = $1`, id); err != nil {
		return wrap("DeleteDocumento", err)
	}
	return nil
}

// ---------------------------------------- Casa Abrigo: evoluções da unidade

// EvolucoesDaUnidade lista as evoluções registradas na unidade para os
// seus acolhidos (a tela de evolução técnica), com o nome de cada um.
func (r *Repository) EvolucoesDaUnidade(ctx context.Context, db database.DBTX, unidade uuid.UUID) ([]domain.EvolucaoAcolhido, error) {
	rows, err := db.Query(ctx, `SELECT r.id, r.cidadao_id, r.unidade_id, r.atendimento_id, r.tipo, r.texto, r.autor_id,
			COALESCE(au.display_name, ''), r.created_at, COALESCE(NULLIF(c.nome_social, ''), c.nome)
		FROM sempras_registros r
		JOIN sempras_cidadaos c ON c.id = r.cidadao_id
		LEFT JOIN users au ON au.id = r.autor_id
		WHERE r.unidade_id = $1
		  AND EXISTS (SELECT 1 FROM sempras_casa_abrigo_acolhimentos a WHERE a.cidadao_id = r.cidadao_id AND a.unidade_id = $1)
		ORDER BY r.created_at DESC LIMIT 500`, unidade)
	if err != nil {
		return nil, wrap("EvolucoesDaUnidade", err)
	}
	return collect(rows, "EvolucoesDaUnidade", func(row pgx.Row) (domain.EvolucaoAcolhido, error) {
		var e domain.EvolucaoAcolhido
		err := row.Scan(&e.ID, &e.CidadaoID, &e.UnidadeID, &e.AtendimentoID, &e.Tipo, &e.Texto, &e.AutorID, &e.AutorNome,
			&e.CreatedAt, &e.AcolhidoNome)
		return e, err
	})
}

// --------------------------------------------------- Casa Abrigo: diário

// InsertAnotacaoDiario grava uma anotação do diário de bordo (só de inclusão).
func (r *Repository) InsertAnotacaoDiario(ctx context.Context, db database.DBTX, a domain.AnotacaoDiario, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_diario
		(id, unidade_id, acolhimento_id, data_referencia, texto, autor_nome, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.UnidadeID, a.AcolhimentoID, a.DataReferencia, a.Texto, a.AutorNome, autor, a.CreatedAt); err != nil {
		return wrap("InsertAnotacaoDiario", err)
	}
	return nil
}

// ListDiario lista as anotações da unidade com data em [inicio, fim).
func (r *Repository) ListDiario(ctx context.Context, db database.DBTX, unidade uuid.UUID, inicio, fim time.Time) ([]domain.AnotacaoDiario, error) {
	rows, err := db.Query(ctx, `SELECT d.id, d.unidade_id, d.acolhimento_id, d.data_referencia, d.texto, d.autor_nome, d.created_at,
			COALESCE(NULLIF(c.nome_social, ''), c.nome, '')
		FROM sempras_casa_abrigo_diario d
		LEFT JOIN sempras_casa_abrigo_acolhimentos a ON a.id = d.acolhimento_id
		LEFT JOIN sempras_cidadaos c ON c.id = a.cidadao_id
		WHERE d.unidade_id = $1 AND d.data_referencia >= $2::date AND d.data_referencia < $3::date
		ORDER BY d.data_referencia DESC, d.created_at DESC LIMIT 1000`, unidade, inicio, fim)
	if err != nil {
		return nil, wrap("ListDiario", err)
	}
	return collect(rows, "ListDiario", func(row pgx.Row) (domain.AnotacaoDiario, error) {
		var a domain.AnotacaoDiario
		err := row.Scan(&a.ID, &a.UnidadeID, &a.AcolhimentoID, &a.DataReferencia, &a.Texto, &a.AutorNome, &a.CreatedAt, &a.AcolhidoNome)
		return a, err
	})
}

// ----------------------------------------------- Casa Abrigo: relatórios

const relatorioCols = `r.id, r.unidade_id, r.acolhimento_id, r.tipo, r.tipo_outro, r.status, r.data_entrega, r.conteudo,
	r.created_at, r.atualizado_em, COALESCE(NULLIF(c.nome_social, ''), c.nome, '')`

const relatorioFrom = ` FROM sempras_casa_abrigo_relatorios r
	LEFT JOIN sempras_casa_abrigo_acolhimentos a ON a.id = r.acolhimento_id
	LEFT JOIN sempras_cidadaos c ON c.id = a.cidadao_id`

func scanRelatorio(row pgx.Row) (domain.RelatorioCasaAbrigo, error) {
	var r domain.RelatorioCasaAbrigo
	err := row.Scan(&r.ID, &r.UnidadeID, &r.AcolhimentoID, &r.Tipo, &r.TipoOutro, &r.Status, &r.DataEntrega, &r.Conteudo,
		&r.CreatedAt, &r.AtualizadoEm, &r.AcolhidoNome)
	return r, err
}

// InsertRelatorio grava um relatório escrito.
func (r *Repository) InsertRelatorio(ctx context.Context, db database.DBTX, rel domain.RelatorioCasaAbrigo, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_relatorios
		(id, unidade_id, acolhimento_id, tipo, tipo_outro, status, data_entrega, conteudo, registrado_por, created_at, atualizado_em)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		rel.ID, rel.UnidadeID, rel.AcolhimentoID, rel.Tipo, rel.TipoOutro, rel.Status, rel.DataEntrega, rel.Conteudo, autor,
		rel.CreatedAt, rel.AtualizadoEm); err != nil {
		return wrap("InsertRelatorio", err)
	}
	return nil
}

// GetRelatorio busca um relatório (forUpdate trava a linha na edição).
func (r *Repository) GetRelatorio(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.RelatorioCasaAbrigo, error) {
	q := `SELECT ` + relatorioCols + relatorioFrom + ` WHERE r.id = $1`
	if forUpdate {
		q += ` FOR UPDATE OF r`
	}
	rel, err := scanRelatorio(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RelatorioCasaAbrigo{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.RelatorioCasaAbrigo{}, wrap("GetRelatorio", err)
	}
	return rel, nil
}

// UpdateRelatorio grava a edição de um relatório.
func (r *Repository) UpdateRelatorio(ctx context.Context, db database.DBTX, rel domain.RelatorioCasaAbrigo) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_relatorios SET acolhimento_id = $2, tipo = $3, tipo_outro = $4,
		status = $5, data_entrega = $6, conteudo = $7, atualizado_em = $8 WHERE id = $1`,
		rel.ID, rel.AcolhimentoID, rel.Tipo, rel.TipoOutro, rel.Status, rel.DataEntrega, rel.Conteudo, rel.AtualizadoEm); err != nil {
		return wrap("UpdateRelatorio", err)
	}
	return nil
}

// DeleteRelatorio remove um relatório (o entregue é barrado antes, R8).
func (r *Repository) DeleteRelatorio(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_relatorios WHERE id = $1`, id); err != nil {
		return wrap("DeleteRelatorio", err)
	}
	return nil
}

// ListRelatorios lista os relatórios da unidade, os de prazo mais próximo primeiro.
func (r *Repository) ListRelatorios(ctx context.Context, db database.DBTX, unidade uuid.UUID) ([]domain.RelatorioCasaAbrigo, error) {
	rows, err := db.Query(ctx, `SELECT `+relatorioCols+relatorioFrom+` WHERE r.unidade_id = $1
		ORDER BY (r.status = 'entregue'), r.data_entrega, r.created_at LIMIT 500`, unidade)
	if err != nil {
		return nil, wrap("ListRelatorios", err)
	}
	return collect(rows, "ListRelatorios", scanRelatorio)
}

// -------------------------------------------------- Casa Abrigo: tarefas

const tarefaCols = `t.id, t.unidade_id, t.titulo, t.descricao, t.status, t.prioridade, t.responsavel, t.sprint, t.prazo, t.ordem,
	t.created_at, t.atualizado_em`

func scanTarefa(row pgx.Row) (domain.TarefaCasaAbrigo, error) {
	var t domain.TarefaCasaAbrigo
	err := row.Scan(&t.ID, &t.UnidadeID, &t.Titulo, &t.Descricao, &t.Status, &t.Prioridade, &t.Responsavel, &t.Sprint, &t.Prazo,
		&t.Ordem, &t.CreatedAt, &t.AtualizadoEm)
	return t, err
}

// InsertTarefa grava uma tarefa do Kanban.
func (r *Repository) InsertTarefa(ctx context.Context, db database.DBTX, t domain.TarefaCasaAbrigo, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_tarefas
		(id, unidade_id, titulo, descricao, status, prioridade, responsavel, sprint, prazo, ordem, registrado_por, created_at, atualizado_em)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		t.ID, t.UnidadeID, t.Titulo, t.Descricao, t.Status, t.Prioridade, t.Responsavel, t.Sprint, t.Prazo, t.Ordem, autor,
		t.CreatedAt, t.AtualizadoEm); err != nil {
		return wrap("InsertTarefa", err)
	}
	return nil
}

// GetTarefa busca uma tarefa (forUpdate trava a linha na edição).
func (r *Repository) GetTarefa(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.TarefaCasaAbrigo, error) {
	q := `SELECT ` + tarefaCols + ` FROM sempras_casa_abrigo_tarefas t WHERE t.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	t, err := scanTarefa(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TarefaCasaAbrigo{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.TarefaCasaAbrigo{}, wrap("GetTarefa", err)
	}
	return t, nil
}

// UpdateTarefa grava a edição (ou o arraste de coluna) de uma tarefa.
func (r *Repository) UpdateTarefa(ctx context.Context, db database.DBTX, t domain.TarefaCasaAbrigo) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_tarefas SET titulo = $2, descricao = $3, status = $4, prioridade = $5,
		responsavel = $6, sprint = $7, prazo = $8, ordem = $9, atualizado_em = $10 WHERE id = $1`,
		t.ID, t.Titulo, t.Descricao, t.Status, t.Prioridade, t.Responsavel, t.Sprint, t.Prazo, t.Ordem, t.AtualizadoEm); err != nil {
		return wrap("UpdateTarefa", err)
	}
	return nil
}

// DeleteTarefa remove uma tarefa.
func (r *Repository) DeleteTarefa(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_tarefas WHERE id = $1`, id); err != nil {
		return wrap("DeleteTarefa", err)
	}
	return nil
}

// ListTarefas lista as tarefas da unidade na ordem do quadro.
func (r *Repository) ListTarefas(ctx context.Context, db database.DBTX, unidade uuid.UUID) ([]domain.TarefaCasaAbrigo, error) {
	rows, err := db.Query(ctx, `SELECT `+tarefaCols+` FROM sempras_casa_abrigo_tarefas t WHERE t.unidade_id = $1
		ORDER BY t.ordem, t.created_at DESC LIMIT 1000`, unidade)
	if err != nil {
		return nil, wrap("ListTarefas", err)
	}
	return collect(rows, "ListTarefas", scanTarefa)
}
