package importacao

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/audit"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
)

// Origens: as tabelas do 4.0, na ordem em que são importadas (os
// dependentes vêm depois do acolhido).
var Origens = []string{"acolhidos", "pia", "pia_plano_acao", "evolucoes_tecnicas", "visitantes_voluntarios",
	"documentos_termos", "diario_logs", "relatorios_gerais", "scrum_tarefas", "documentos_acolhido"}

// Opcoes da importação.
type Opcoes struct {
	Unidade uuid.UUID // a Casa Abrigo de destino
	Autor   uuid.UUID // quem roda (users.id): registrado_por e auditoria
	Aplicar bool      // false = simulação (tudo desfeito no fim)
	Agora   time.Time
}

// Contagem de uma tabela do 4.0.
type Contagem struct {
	Lidos        int `json:"lidos"`
	Importados   int `json:"importados"`
	JaImportados int `json:"ja_importados"`
	Rejeitados   int `json:"rejeitados"`
	Ignorados    int `json:"ignorados"`
}

// Aviso aponta um registro do 4.0 pelo id (nunca pelo nome da criança).
type Aviso struct {
	Origem   string `json:"origem"`
	ID       int64  `json:"id"`
	Mensagem string `json:"mensagem"`
}

// Relatorio é o resultado: o que entrou (ou entraria), o que entrou com
// ajuste a revisar e o que ficou de fora.
type Relatorio struct {
	Aplicada   bool                 `json:"aplicada"`
	Contagens  map[string]*Contagem `json:"contagens"`
	Avisos     []Aviso              `json:"avisos"`
	Rejeitados []Aviso              `json:"rejeitados"`
}

// Erros que encerram a importação sem gravar nada.
var (
	ErrUnidade   = errors.New("a unidade de destino tem de ser uma Casa Abrigo")
	errSimulacao = errors.New("simulação: desfazer")
)

// recusa é um registro do 4.0 que não entra (vai para o relatório).
type recusa string

func (r recusa) Error() string { return string(r) }

// recusado diz se o erro é do registro (fica de fora) e não do banco.
func recusado(err error) bool {
	var v *domain.ValidationError
	var r recusa
	return errors.As(err, &v) || errors.As(err, &r) || errors.Is(err, domain.ErrAcolhimentoAtivo)
}

type importador struct {
	repo         domain.Repository
	dec          Decifrador
	op           Opcoes
	leg          Legado
	rel          *Relatorio
	tx           pgx.Tx
	acolhimentos map[int64]domain.Acolhimento // id do 4.0 → acolhimento daqui
	porNome      map[string]int64             // nome (chave) → id do 4.0, só os nomes sem repetição
	pendentes    []Aviso                      // avisos do registro em curso (só valem se ele entrar)
	origem       string
	id           int64
}

// Importar leva o legado para a Casa Abrigo de destino numa transação só.
// Na simulação a transação é desfeita no fim: o relatório mostra o que
// entraria, com as mesmas regras e restrições do banco.
func Importar(ctx context.Context, pool *pgxpool.Pool, repo domain.Repository, leg Legado, dec Decifrador, op Opcoes) (Relatorio, error) {
	rel := Relatorio{Aplicada: op.Aplicar, Contagens: map[string]*Contagem{}}
	for _, o := range Origens {
		rel.Contagens[o] = &Contagem{}
	}
	if err := conferirChave(leg, dec); err != nil {
		return rel, err
	}
	err := database.WithTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := repo.GetUnidade(ctx, tx, op.Unidade)
		if err != nil {
			return err
		}
		if u.Tipo != domain.UnidadeCasaAbrigo {
			return ErrUnidade
		}
		im := &importador{repo: repo, dec: dec, op: op, leg: leg, rel: &rel, tx: tx,
			acolhimentos: map[int64]domain.Acolhimento{}, porNome: map[string]int64{}}
		for _, etapa := range []func(context.Context) error{im.acolhidos, im.pias, im.metas, im.evolucoes, im.visitantes,
			im.termos, im.diario, im.relatorios, im.tarefas} {
			if err := etapa(ctx); err != nil {
				return err
			}
		}
		im.documentos()
		e := audit.Meta(ctx, "sempras.casa_abrigo.importacao_4_0", "unidade", op.Unidade.String(), nil, nil)
		e.ActorID = &op.Autor
		e.Metadata = map[string]any{"aplicada": op.Aplicar, "contagens": rel.Contagens}
		if err := audit.NewWriter(tx).Record(ctx, e); err != nil {
			return err
		}
		if !op.Aplicar {
			return errSimulacao
		}
		return nil
	})
	if errors.Is(err, errSimulacao) {
		err = nil
	}
	return rel, err
}

// conferirChave decifra o primeiro nome cifrado: chave errada para tudo
// antes de começar, em vez de rejeitar acolhido por acolhido.
func conferirChave(leg Legado, dec Decifrador) error {
	for _, l := range leg.Acolhidos {
		if v := l.Texto("nome"); strings.HasPrefix(v, prefixoCifrado) {
			_, err := dec.Texto(v)
			return err
		}
	}
	return nil
}

func (im *importador) aviso(msg string, args ...any) {
	im.pendentes = append(im.pendentes, Aviso{Origem: im.origem, ID: im.id, Mensagem: fmt.Sprintf(msg, args...)})
}

// txt lê e decifra um campo; o que não decifra fica vazio, com aviso.
func (im *importador) txt(l Linha, campo string) string {
	v, err := im.dec.Texto(l.Texto(campo))
	if err != nil {
		im.aviso("campo %s não decifrou: ficou em branco", campo)
		return ""
	}
	return strings.TrimSpace(v)
}

// item grava um registro do 4.0 num savepoint: o que o domínio recusa fica
// de fora (no relatório) sem desfazer o resto; erro do banco encerra tudo.
// Registro já importado (rastreio) não é gravado de novo.
func (im *importador) item(ctx context.Context, origem string, id int64, gravar func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error)) (uuid.UUID, bool, error) {
	c := im.rel.Contagens[origem]
	c.Lidos++
	var destino uuid.UUID
	err := im.tx.QueryRow(ctx, `SELECT destino_id FROM sempras_casa_abrigo_importacao WHERE unidade_id = $1 AND origem = $2 AND origem_id = $3`,
		im.op.Unidade, origem, id).Scan(&destino)
	if err == nil {
		c.JaImportados++
		return destino, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, err
	}
	im.origem, im.id, im.pendentes = origem, id, nil
	sp, err := im.tx.Begin(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	destino, err = gravar(ctx, sp)
	if err == nil {
		_, err = sp.Exec(ctx, `INSERT INTO sempras_casa_abrigo_importacao (unidade_id, origem, origem_id, destino_id, importado_por)
			VALUES ($1, $2, $3, $4, $5)`, im.op.Unidade, origem, id, destino, im.op.Autor)
	}
	if err != nil {
		_ = sp.Rollback(ctx)
		if !recusado(err) {
			return uuid.Nil, false, err
		}
		c.Rejeitados++
		im.rel.Rejeitados = append(im.rel.Rejeitados, Aviso{Origem: origem, ID: id, Mensagem: err.Error()})
		return uuid.Nil, false, nil
	}
	if err := sp.Commit(ctx); err != nil {
		return uuid.Nil, false, err
	}
	c.Importados++
	im.rel.Avisos = append(im.rel.Avisos, im.pendentes...)
	return destino, true, nil
}

// acolhimentoDe acha o acolhimento daqui do acolhido do 4.0.
func (im *importador) acolhimentoDe(l Linha) (domain.Acolhimento, error) {
	a, ok := im.acolhimentos[l.Inteiro("acolhido_id")]
	if !ok {
		return domain.Acolhimento{}, recusa(fmt.Sprintf("o acolhido nº %d do 4.0 não foi importado", l.Inteiro("acolhido_id")))
	}
	return a, nil
}

// quando é o instante do registro no 4.0 (o primeiro campo que houver) ou agora.
func (im *importador) quando(l Linha, campos ...string) time.Time {
	for _, c := range campos {
		if t := l.Instante(c); t != nil {
			return *t
		}
	}
	return im.op.Agora
}

// ------------------------------------------------------------- acolhidos

func (im *importador) acolhidos(ctx context.Context) error {
	contagem := map[string]int{}
	for _, l := range im.leg.Acolhidos {
		if v, err := im.dec.Texto(l.Texto("nome")); err == nil && chave(v) != "" {
			contagem[chave(v)]++
			im.porNome[chave(v)] = l.ID()
		}
	}
	for n, k := range contagem {
		if k > 1 {
			delete(im.porNome, n)
		}
	}
	for _, l := range im.leg.Acolhidos {
		var novo domain.Acolhimento
		destino, ok, err := im.item(ctx, "acolhidos", l.ID(), func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			a, err := im.acolhimento(ctx, tx, l)
			novo = a
			return a.ID, err
		})
		if err != nil {
			return err
		}
		switch {
		case ok && novo.ID == uuid.Nil: // importado antes
			a, err := im.repo.GetAcolhimento(ctx, im.tx, destino, false)
			if err != nil {
				return err
			}
			im.acolhimentos[l.ID()] = a
		case ok:
			im.acolhimentos[l.ID()] = novo
		}
	}
	return nil
}

func (im *importador) acolhimento(ctx context.Context, tx pgx.Tx, l Linha) (domain.Acolhimento, error) {
	nome := im.txt(l, "nome")
	if len([]rune(chave(nome))) < 2 {
		return domain.Acolhimento{}, recusa("sem nome legível")
	}
	nasc := l.Dia("data_nascimento")
	if nasc == nil {
		return domain.Acolhimento{}, recusa("sem data de nascimento")
	}
	entrada := l.Dia("data_entrada")
	if entrada == nil {
		return domain.Acolhimento{}, recusa("sem data de entrada na unidade")
	}
	dados := im.dadosAcolhimento(l)
	c, err := im.cidadao(ctx, tx, l, nome, *nasc, dados.Sexo)
	if err != nil {
		return domain.Acolhimento{}, err
	}
	nascimento := c.DataNascimento
	if nascimento == "" {
		nascimento = nasc.Format(time.DateOnly)
	}
	a, err := domain.NovoAcolhimento(c.ID, im.op.Unidade, nascimento, dados, *entrada, im.op.Agora)
	if err != nil {
		return domain.Acolhimento{}, err
	}
	if err := im.repo.InsertAcolhimento(ctx, tx, a, im.op.Autor); err != nil {
		return domain.Acolhimento{}, err
	}
	for _, d := range diagnosticosDoChecklist {
		if !l.Logico(d.coluna) {
			continue
		}
		diag, _ := domain.NovoDiagnostico(a.ID, d.categoria, d.nome, "") // nomes do catálogo: sempre válidos
		diag.CreatedAt = im.op.Agora
		if err := im.repo.InsertDiagnostico(ctx, tx, diag); err != nil {
			return domain.Acolhimento{}, err
		}
	}
	return a, im.desligamento(ctx, tx, l, &a)
}

// desligamento: o 4.0 tem só o texto do motivo e a data (em dois pares de
// colunas, conforme a versão); o tipo é deduzido do texto e vai para revisão.
func (im *importador) desligamento(ctx context.Context, tx pgx.Tx, l Linha, a *domain.Acolhimento) error {
	motivo := im.txt(l, "desligamento_motivo")
	if motivo == "" {
		motivo = im.txt(l, "motivo_desligamento")
	}
	saida := l.Dia("desligamento_data_saida")
	if saida == nil {
		saida = l.Dia("data_saida")
	}
	if motivo == "" && saida == nil {
		return nil
	}
	if saida == nil {
		return recusa("desligado sem data da saída: informe a data no 4.0 e simule de novo")
	}
	tipo, inferido := tipoDesligamento(motivo)
	if inferido {
		im.aviso("tipo de desligamento deduzido do motivo: %s — confira", tipo)
	} else {
		im.aviso("tipo de desligamento não deduzido do motivo: ficou \"outro\" — corrija na ficha")
	}
	if len([]rune(motivo)) < 3 {
		motivo = "não informado no sistema anterior"
	}
	if err := a.Desligar(domain.Desligamento{Tipo: tipo, Motivo: motivo, Guia: im.txt(l, "desligamento_guia"), DataSaida: *saida,
		DataGuia: l.Dia("desligamento_data_guia")}, im.op.Autor, im.op.Agora); err != nil {
		return err
	}
	return im.repo.UpdateAcolhimento(ctx, tx, *a)
}

// cidadao acha o munícipe no cadastro daqui (CPF, NIS ou nome e
// nascimento) ou o cadastra.
func (im *importador) cidadao(ctx context.Context, tx pgx.Tx, l Linha, nome string, nasc time.Time, sexo string) (domain.Cidadao, error) {
	cpf := domain.Digitos(im.txt(l, "cpf_crianca"))
	if cpf == "" {
		cpf = domain.Digitos(im.txt(l, "cpf"))
	}
	if cpf != "" && !domain.CPFValido(cpf) {
		im.aviso("CPF da criança inválido no 4.0: não foi importado")
		cpf = ""
	}
	nis := domain.Digitos(l.Texto("nis_crianca"))
	if nis != "" && len(nis) != 11 {
		im.aviso("NIS da criança inválido no 4.0: não foi importado")
		nis = ""
	}
	for _, doc := range []string{cpf, nis} {
		if doc == "" {
			continue
		}
		cs, err := im.repo.BuscarPorDocumento(ctx, tx, doc)
		if err != nil {
			return domain.Cidadao{}, err
		}
		if len(cs) == 1 {
			im.aviso("munícipe já cadastrado (mesmo CPF/NIS): mantido o cadastro daqui")
			return cs[0], nil
		}
	}
	cs, err := im.repo.BuscarPorNome(ctx, tx, nome, 50)
	if err != nil {
		return domain.Cidadao{}, err
	}
	for _, c := range cs {
		if chave(c.Nome) == chave(nome) && c.DataNascimento == nasc.Format(time.DateOnly) {
			im.aviso("munícipe já cadastrado (mesmo nome e nascimento): mantido o cadastro daqui")
			return c, nil
		}
	}
	in := domain.CidadaoInput{Nome: nome, NomeSocial: l.Texto("nome_social"), CPF: cpf, NIS: nis,
		DataNascimento: nasc.Format(time.DateOnly), Sexo: sexo, NomeMae: im.txt(l, "filiacao_1")}
	c, err := in.Normalizar(im.op.Agora)
	if err != nil {
		return domain.Cidadao{}, err
	}
	c.ID, c.CreatedAt, c.UpdatedAt = uuid.New(), im.quando(l, "criado_em"), im.op.Agora
	return c, im.repo.InsertCidadao(ctx, tx, c, im.op.Autor)
}

// enum traduz um rótulo do 4.0; o que não for de nenhuma opção vai para as
// observações (nada se perde) e para o relatório.
func (im *importador) enum(l Linha, campo, nome string, rot rotulos, extra *[]string) string {
	bruto := im.txt(l, campo)
	if v, ok := rot.valor(bruto); ok {
		return v
	}
	im.aviso("%s fora da lista (%q): ficou em branco, com o texto nas observações", nome, bruto)
	*extra = append(*extra, nome+" no sistema anterior: "+bruto)
	return ""
}

func (im *importador) dadosAcolhimento(l Linha) domain.AcolhimentoDados {
	var extra []string
	e := func(campo, nome string, rot rotulos) string { return im.enum(l, campo, nome, rot, &extra) }
	primeiro := func(campos ...string) string {
		for _, c := range campos {
			if v := im.txt(l, c); !naoInformado(v) {
				return v
			}
		}
		return ""
	}
	d := domain.AcolhimentoDados{
		Sexo: e("sexo", "sexo", rotSexo), SituacaoEspecial: l.Logico("situacao_especial"), PCD: l.Logico("pcd"),
		Apelido: im.txt(l, "apelido"), IdentidadeGenero: e("identidade_genero", "identidade de gênero", rotGenero),
		OrientacaoSexual: e("orientacao_sexual", "orientação sexual", rotOrientacao), RacaCor: e("raca_cor", "raça/cor", rotRacaCor),
		Filiacao1: im.txt(l, "filiacao_1"), Filiacao2: im.txt(l, "filiacao_2"),
		ResponsavelNaoPossui: l.Logico("resp_nao_possui"), ResponsavelNome: im.txt(l, "resp_nome"),
		ResponsavelNomeSocial: im.txt(l, "resp_nome_social"), ResponsavelApelido: im.txt(l, "resp_apelido"),
		ResponsavelNascimento: l.Dia("resp_nascimento"), ResponsavelSexo: e("resp_sexo", "sexo do responsável", rotSexo),
		ResponsavelLogradouro: im.txt(l, "resp_logradouro"), ResponsavelNumero: im.txt(l, "resp_numero"),
		ResponsavelComplemento: im.txt(l, "resp_complemento"), ResponsavelBairro: im.txt(l, "resp_bairro"),
		ResponsavelCidade: im.txt(l, "resp_cidade"), ResponsavelTelefone: im.txt(l, "resp_telefone"),
		ResponsavelCelular: im.txt(l, "resp_celular"),
		ProcessoJudicial:   primeiro("processo", "processo_judicial"), ProcessoJudicial2: primeiro("processo_2"),
		GuiaAcolhimento: primeiro("guia", "guia_acolhimento"), DataGuia: l.Dia("data_guia"),
		HistoricoAcolhidoAnteriormente: e("historico_acolhido_anteriormente", "acolhido anteriormente", rotSimNaoSei),
		HistoricoIrmaosNestaUnidade:    e("historico_irmaos_nesta", "irmãos nesta unidade", rotSimNaoSei),
		HistoricoIrmaosOutraUnidade:    e("historico_irmaos_outra", "irmãos em outra unidade", rotSimNaoSei),
		SituacaoJuridica:               e("situacao_juridica", "situação jurídica", rotSituacao),
		DataSituacaoJuridica:           l.Dia("data_situacao_juridica"), ConselheiroNome: im.txt(l, "conselheiro_nome"),
		ConselheiroTelefone: im.txt(l, "conselheiro_telefone"), CNA: e("cna", "CNA", rotSimNaoSei), CNCAD: e("cncad", "CNCAD", rotSimNaoSei),
		LaudoMedico: e("laudo_medico", "laudo médico", rotSimNao), LaudoMedicoDescricao: im.txt(l, "laudo_medico_desc"),
		MedicacaoContinua:   e("medicacao_continua", "medicação contínua", rotSimNao),
		DependenciaQuimica:  e("dependencia_quimica", "dependência química", rotSimNao),
		TranstornoDescricao: im.txt(l, "transtorno_desc"), Estuda: e("social_estuda", "estuda", rotSimNao),
		AmeacaMorte: e("social_ameaca_morte", "ameaça de morte", rotSimNao), RecebeVisita: e("social_recebe_visita", "recebe visita", rotSimNao),
		BeneficioGoverno: e("social_beneficio_governo", "benefício do governo", rotSimNao), BeneficioQual: im.txt(l, "social_qual_beneficio"),
		ProgramaSocial:  e("social_programa_social", "programa social", rotSimNao),
		RelacaoTrabalho: e("social_relacao_trabalho", "relação de trabalho", rotTrabalho),
	}
	if v, ok := vinculo(im.txt(l, "resp_vinculo")); ok {
		d.ResponsavelVinculo = v
	} else {
		im.aviso("vínculo do responsável fora da lista: ficou em branco, com o texto nas observações")
		extra = append(extra, "vínculo do responsável no sistema anterior: "+im.txt(l, "resp_vinculo"))
	}
	d.Motivos, d.OutroMotivo = motivos(im.txt(l, "motivo_acolhimento"), im.txt(l, "outro_motivo"))
	if len(d.Motivos) == 0 {
		d.Motivos, d.OutroMotivo = []string{"outro"}, "não informado no sistema anterior"
		im.aviso("sem motivo de acolhimento no 4.0: ficou \"outro\" — corrija na ficha")
	} else if d.OutroMotivo == SemDetalhe {
		im.aviso("motivo \"outro\" sem detalhe no 4.0 — complete na ficha")
	}
	// o 4.0 não grava "possui transtorno": deduzido do checklist e da descrição
	if d.TranstornoDescricao != "" {
		d.PossuiTranstorno = "sim"
	}
	for _, t := range diagnosticosDoChecklist {
		if t.categoria == "transtorno" && l.Logico(t.coluna) {
			d.PossuiTranstorno = "sim"
		}
	}
	im.sanearResponsavel(l, &d)
	for _, c := range []struct{ campo, nome string }{{"rg", "RG"}, {"certidao_nascimento", "certidão de nascimento"}} {
		if v := im.txt(l, c.campo); !naoInformado(v) {
			extra = append(extra, c.nome+": "+v)
		}
	}
	d.Observacoes = strings.Join(append([]string{im.txt(l, "observacoes_gerais")}, extra...), "\n")
	d.Observacoes = strings.TrimSpace(d.Observacoes)
	return d
}

// sanearResponsavel tira do responsável o que o domínio recusaria (CPF,
// NIS, CEP, UF e e-mail inválidos), com aviso: o acolhido entra assim mesmo.
func (im *importador) sanearResponsavel(l Linha, d *domain.AcolhimentoDados) {
	if v := domain.Digitos(im.txt(l, "cpf_responsavel")); v != "" && domain.CPFValido(v) {
		d.ResponsavelCPF = v
	} else if v != "" {
		im.aviso("CPF do responsável inválido no 4.0: não foi importado")
	}
	if v := domain.Digitos(im.txt(l, "resp_nis")); len(v) == 11 {
		d.ResponsavelNIS = v
	} else if v != "" {
		im.aviso("NIS do responsável inválido no 4.0: não foi importado")
	}
	if v := domain.Digitos(im.txt(l, "resp_cep")); len(v) == 8 {
		d.ResponsavelCEP = v
	} else if v != "" {
		im.aviso("CEP do responsável inválido no 4.0: não foi importado")
	}
	if v := strings.ToUpper(im.txt(l, "resp_uf")); ufValida.MatchString(v) {
		d.ResponsavelUF = v
	} else if v != "" {
		im.aviso("UF do responsável inválida no 4.0: não foi importada")
	}
	if v := im.txt(l, "resp_email"); emailValido.MatchString(v) {
		d.ResponsavelEmail = v
	} else if v != "" {
		im.aviso("e-mail do responsável inválido no 4.0: não foi importado")
	}
}

var (
	ufValida    = regexp.MustCompile(`^[A-Z]{2}$`)
	emailValido = regexp.MustCompile(`^[^\s@<>()",;:]+@[^\s@<>()",;:]+\.[^\s@<>()",;:]+$`)
)

// ------------------------------------------------------------------- PIA

// pias monta a ficha do PIA de cada acolhido com o que houver nas cinco
// tabelas do PIA do 4.0 (a mais recente de saúde e de família).
func (im *importador) pias(ctx context.Context) error {
	ficha := map[int64]map[string]Linha{}
	var ordem []int64
	for _, g := range []struct {
		nome   string
		linhas []Linha
	}{{"saude", im.leg.PiaSaude}, {"familia", im.leg.PiaFamilia}, {"rotina", im.leg.PiaRotina}, {"intervencao", im.leg.PiaIntervencao}} {
		for _, l := range g.linhas {
			id := l.Inteiro("acolhido_id")
			if ficha[id] == nil {
				ficha[id] = map[string]Linha{}
				ordem = append(ordem, id)
			}
			ficha[id][g.nome] = l // as linhas vêm por id: fica a mais recente
		}
	}
	for _, id := range ordem {
		partes := ficha[id]
		_, _, err := im.item(ctx, "pia", id, func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			a, err := im.acolhimentoDe(Linha{"acolhido_id": fmt.Sprint(id)})
			if err != nil {
				return uuid.Nil, err
			}
			return a.ID, im.pia(ctx, tx, a, partes)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (im *importador) pia(ctx context.Context, tx pgx.Tx, a domain.Acolhimento, p map[string]Linha) error {
	s, f, r, iv := p["saude"], p["familia"], p["rotina"], p["intervencao"]
	var periodo string
	if v, ok := rotPeriodo.valor(im.txt(s, "periodo_escola")); ok {
		periodo = v
	} else {
		im.aviso("período escolar fora da lista: ficou em branco")
	}
	contato := strings.Join(naoVazios("Pai: "+im.txt(f, "nome_pai"), "Mãe: "+im.txt(f, "nome_mae"), im.txt(f, "contato_pais")), "\n")
	obsFamilia := im.txt(f, "observacoes_familia")
	if v := f.Texto("irmaos_acolhidos"); v != "" && v != "false" {
		if v == "true" {
			v = "sim"
		}
		obsFamilia = strings.TrimSpace(obsFamilia + "\nIrmãos acolhidos: " + v)
	}
	d := domain.PiaFichaDados{
		Medicamentos: im.txt(s, "medicamentos_uso_continuo"), Alergias: im.txt(s, "alergias"),
		VacinacaoAtualizada: s.Logico("carteira_vacina_atualizada"), CartaoSUS: im.txt(s, "cartao_sus"),
		TratamentoMedicoPsicologico: s.Logico("tratamento_medico_psicologico"), DeficienciasRestricoes: im.txt(s, "deficiencias_restricoes"),
		Escola: im.txt(s, "nome_escola"), SerieEscolar: im.txt(s, "serie_escola"), MatriculaEscolar: im.txt(s, "matricula_escola"),
		PeriodoEscolar: periodo, FrequenciaEscolar: im.txt(s, "frequencia_escola"),
		HistoricoFamiliar: im.txt(f, "historico_familiar"), ContatoFamiliar: contato,
		VisitasAutorizadas: im.txt(f, "visitas_familiares_autorizadas"), ObservacoesFamilia: obsFamilia,
		AtividadesManha: im.txt(r, "atividades_manha"), AtividadesTarde: im.txt(r, "atividades_tarde"),
		AtividadesNoite: im.txt(r, "atividades_noite"), AtividadesFimSemana: im.txt(r, "finais_semana"),
		ObservacoesComportamentais: im.txt(r, "observacoes_comportamentais"),
		IntervencaoObjetivos:       im.txt(iv, "objetivos"), IntervencaoEstrategias: im.txt(iv, "estrategias"),
		IntervencaoMetasPrazos: im.txt(iv, "metas_prazos"),
	}
	ficha, err := domain.NovaPiaFicha(a.ID, d)
	if err != nil {
		return err
	}
	// a última gravação no 4.0 é a última reavaliação (R7); sem data, conta
	// da entrada — o monitor avisa antes, nunca depois
	ficha.AtualizadoEm = a.DataEntrada
	achou := false
	for _, l := range []Linha{s, f, r, iv} {
		for _, c := range []string{"atualizado_em", "criado_em", "data_registro"} {
			if t := l.Instante(c); t != nil && (!achou || t.After(ficha.AtualizadoEm)) {
				ficha.AtualizadoEm, achou = *t, true
			}
		}
	}
	if !achou {
		im.aviso("P.I.A. sem data de gravação no 4.0: a próxima reavaliação conta da entrada")
	}
	return im.repo.UpsertPiaFicha(ctx, tx, ficha, im.op.Autor)
}

func naoVazios(vs ...string) []string {
	var out []string
	for _, v := range vs {
		if v != "" && !strings.HasSuffix(v, ": ") {
			out = append(out, v)
		}
	}
	return out
}

// metas: o plano de ação do PIA do 4.0 (uma linha por necessidade).
func (im *importador) metas(ctx context.Context) error {
	for _, l := range im.leg.PiaPlano {
		_, _, err := im.item(ctx, "pia_plano_acao", l.ID(), func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			a, err := im.acolhimentoDe(l)
			if err != nil {
				return uuid.Nil, err
			}
			d := domain.PiaMetaDados{Tipo: domain.PiaMetaPlano, Descricao: im.txt(l, "necessidade"), Providencia: im.txt(l, "providencias"),
				Responsavel: im.txt(l, "responsavel"), Prazo: l.Dia("prazo"), Esportes: im.txt(l, "esportes"),
				Cultura: im.txt(l, "cultura"), Lazer: im.txt(l, "lazer")}
			if d.Descricao == "" && d.Esportes+d.Cultura+d.Lazer != "" {
				d.Descricao = "Esportes, cultura e lazer"
			}
			m, err := domain.NovaPiaMeta(a.ID, d)
			if err != nil {
				return uuid.Nil, err
			}
			m.CreatedAt = im.quando(l, "criado_em")
			return m.ID, im.repo.InsertPiaMeta(ctx, tx, m, im.op.Autor)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// ------------------------------------------------- evoluções, visitas, termos

func (im *importador) evolucoes(ctx context.Context) error {
	for _, l := range im.leg.Evolucoes {
		_, _, err := im.item(ctx, "evolucoes_tecnicas", l.ID(), func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			a, err := im.acolhimentoDe(l)
			if err != nil {
				return uuid.Nil, err
			}
			origem := "Registrado no sistema anterior (Casa Abrigo 4.0)"
			if t := im.usuario(l); t != "" {
				origem += " por " + t
			}
			r, err := domain.NovoRegistro(a.CidadaoID, im.op.Unidade, nil, tipoEvolucao(im.txt(l, "tipo_evolucao")),
				origem+".\n\n"+im.txt(l, "texto_evolucao"), im.op.Autor)
			if err != nil {
				return uuid.Nil, err
			}
			r.CreatedAt = im.quando(l, "data_hora")
			return r.ID, im.repo.InsertRegistro(ctx, tx, r)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// usuario é o nome do técnico do 4.0 que fez o registro ("" se não houver).
func (im *importador) usuario(l Linha) string {
	v, err := im.dec.Texto(im.leg.Usuarios[l.Inteiro("usuario_id")])
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

var sufixoTipoVisita = regexp.MustCompile(`\s*\(Tipo:\s*([^)]*)\)\s*$`)

func (im *importador) visitantes(ctx context.Context) error {
	for _, l := range im.leg.Visitantes {
		_, _, err := im.item(ctx, "visitantes_voluntarios", l.ID(), func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			a, err := im.acolhimentoDe(l)
			if err != nil {
				return uuid.Nil, err
			}
			dia := l.Dia("data_visita")
			if dia == nil {
				return uuid.Nil, recusa("visita sem data")
			}
			// o 4.0 não tem coluna do tipo: guarda "(Tipo: Doação)" no fim das observações
			obs, tipo := im.txt(l, "observacoes_visita"), "acolhido"
			if m := sufixoTipoVisita.FindStringSubmatch(obs); m != nil {
				obs = strings.TrimSpace(sufixoTipoVisita.ReplaceAllString(obs, ""))
				if v, ok := rotVisita.valor(m[1]); ok && v != "" {
					tipo = v
				}
			}
			v, err := domain.NovoVisitante(im.op.Unidade, domain.VisitanteDados{AcolhimentoID: &a.ID, TipoVisita: tipo,
				NomeVisitante: im.txt(l, "nome_visitante"), DocumentoIdentidade: im.txt(l, "documento_identidade"),
				ParentescoVinculo: im.txt(l, "parentesco_vinculo"), DataVisita: *dia, HoraEntrada: hhmm(l.Texto("hora_entrada")),
				HoraSaida: hhmm(l.Texto("hora_saida")), Observacoes: obs})
			if err != nil {
				return uuid.Nil, err
			}
			v.CreatedAt = im.quando(l, "criado_em", "data_visita")
			return v.ID, im.repo.InsertVisitante(ctx, tx, v, im.op.Autor)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// hhmm corta "14:03:22" (toLocaleTimeString) em "14:03".
func hhmm(v string) string {
	if len(v) >= 5 {
		return v[:5]
	}
	return v
}

func (im *importador) termos(ctx context.Context) error {
	for _, l := range im.leg.Termos {
		_, _, err := im.item(ctx, "documentos_termos", l.ID(), func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			a, err := im.acolhimentoDe(l)
			if err != nil {
				return uuid.Nil, err
			}
			dia := l.Dia("data_evento")
			if dia == nil {
				dia = l.Dia("data_registro")
			}
			if dia == nil {
				return uuid.Nil, recusa("termo sem data")
			}
			tipo, ok := rotTermo.valor(l.Texto("tipo_termo"))
			if !ok || tipo == "" {
				im.aviso("tipo de termo fora da lista: ficou \"outro\"")
				tipo = "outro"
			}
			t, err := domain.NovoTermo(a.ID, tipo, im.txt(l, "responsavel_retirada"), im.txt(l, "parentesco_responsavel"),
				im.txt(l, "motivo_observacao"), *dia)
			if err != nil {
				return uuid.Nil, err
			}
			t.CreatedAt = im.quando(l, "data_registro")
			return t.ID, im.repo.InsertTermo(ctx, tx, t, im.op.Autor)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------- diário

var anotacaoDoDia = regexp.MustCompile(`(?s)^Anotação referente ao dia (\d{2})/(\d{2})/(\d{4}):\s*(.*)$`)

// diario importa só as anotações da equipe ("DIÁRIO DE BORDO (GERAL)" e
// "DIÁRIO - <acolhido>"): os registros automáticos do 4.0 (cadastros,
// exclusões, logins) já têm equivalente na trilha de auditoria daqui.
func (im *importador) diario(ctx context.Context) error {
	for _, l := range im.leg.Diario {
		modulo := im.dec.textoOu(l.Texto("modulo"))
		if !strings.HasPrefix(chave(modulo), "diario") {
			c := im.rel.Contagens["diario_logs"]
			c.Lidos++
			c.Ignorados++
			continue
		}
		_, _, err := im.item(ctx, "diario_logs", l.ID(), func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			texto, dia := im.txt(l, "descricao"), im.quando(l, "data_hora")
			if m := anotacaoDoDia.FindStringSubmatch(texto); m != nil {
				if d, err := time.Parse("02/01/2006", m[1]+"/"+m[2]+"/"+m[3]); err == nil {
					dia = d
				}
				texto = m[4]
			}
			dia = time.Date(dia.Year(), dia.Month(), dia.Day(), 0, 0, 0, 0, time.UTC)
			var acolhimento *uuid.UUID
			if _, nome, ok := strings.Cut(modulo, " - "); ok {
				if a, achou := im.acolhimentos[im.porNome[chave(nome)]]; achou {
					acolhimento = &a.ID
				} else {
					im.aviso("acolhido da anotação não identificado pelo nome: ficou como anotação geral")
				}
			}
			autor := im.usuario(l)
			if autor == "" {
				autor = "Casa Abrigo 4.0"
			}
			an, err := domain.NovaAnotacaoDiario(im.op.Unidade, acolhimento, dia, texto, autor)
			if err != nil {
				return uuid.Nil, err
			}
			an.CreatedAt = im.quando(l, "data_hora")
			return an.ID, im.repo.InsertAnotacaoDiario(ctx, tx, an, im.op.Autor)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// textoOu decifra sem aviso (o módulo só decide o que importar).
func (d Decifrador) textoOu(v string) string {
	if t, err := d.Texto(v); err == nil {
		return t
	}
	return ""
}

// ------------------------------------------------------ relatórios, tarefas

func (im *importador) relatorios(ctx context.Context) error {
	for _, l := range im.leg.Relatorios {
		_, _, err := im.item(ctx, "relatorios_gerais", l.ID(), func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			dia := l.Dia("data_entrega")
			if dia == nil {
				return uuid.Nil, recusa("relatório sem data de entrega")
			}
			rotulo := im.txt(l, "tipo")
			tipo, outro := "outro", rotulo
			if v, ok := rotRelatorio.valor(rotulo); ok && v != "" {
				tipo, outro = v, ""
			} else if naoInformado(rotulo) || chave(rotulo) == "outro" {
				outro = "Relatório do sistema anterior"
			}
			status, ok := rotStatusRel.valor(l.Texto("status"))
			if !ok || status == "" {
				im.aviso("etapa do relatório fora da lista: ficou \"em elaboração\"")
				status = "em_elaboracao"
			}
			r, err := domain.NovoRelatorio(im.op.Unidade, domain.RelatorioDados{Tipo: tipo, TipoOutro: outro, Status: status,
				DataEntrega: *dia, Conteudo: im.txt(l, "conteudo_texto")})
			if err != nil {
				return uuid.Nil, err
			}
			r.CreatedAt = im.quando(l, "criado_em", "created_at")
			r.AtualizadoEm = im.quando(l, "atualizado_em", "criado_em", "created_at")
			return r.ID, im.repo.InsertRelatorio(ctx, tx, r, im.op.Autor)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (im *importador) tarefas(ctx context.Context) error {
	for _, l := range im.leg.Tarefas {
		_, _, err := im.item(ctx, "scrum_tarefas", l.ID(), func(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
			status, ok := rotStatusTaref.valor(l.Texto("status"))
			if !ok {
				im.aviso("coluna da tarefa fora da lista: ficou no backlog")
				status = ""
			}
			prioridade, ok := rotPrioridade.valor(l.Texto("prioridade"))
			if !ok {
				im.aviso("prioridade da tarefa fora da lista: ficou média")
				prioridade = ""
			}
			ordem := int(l.Inteiro("ordem"))
			if ordem < 0 {
				ordem = 0
			}
			t, err := domain.NovaTarefa(im.op.Unidade, domain.TarefaDados{Titulo: im.txt(l, "titulo"), Descricao: im.txt(l, "descricao"),
				Status: status, Prioridade: prioridade, Responsavel: im.txt(l, "responsavel"), Sprint: im.txt(l, "sprint"),
				Prazo: l.Dia("prazo"), Ordem: ordem})
			if err != nil {
				return uuid.Nil, err
			}
			t.CreatedAt = im.quando(l, "criado_em")
			t.AtualizadoEm = im.quando(l, "atualizado_em", "criado_em")
			return t.ID, im.repo.InsertTarefa(ctx, tx, t, im.op.Autor)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// documentos: os PDFs ficam em uploads/ no servidor do 4.0, fora do banco —
// a equipe os anexa pela tela do acolhido (o relatório diz quais).
func (im *importador) documentos() {
	c := im.rel.Contagens["documentos_acolhido"]
	for _, l := range im.leg.Documentos {
		c.Lidos++
		c.Ignorados++
		im.rel.Avisos = append(im.rel.Avisos, Aviso{Origem: "documentos_acolhido", ID: l.ID(),
			Mensagem: fmt.Sprintf("PDF (%s) do acolhido nº %d está em uploads/ no servidor do 4.0: anexe pela tela", l.Texto("tipo"), l.Inteiro("acolhido_id"))})
	}
}
