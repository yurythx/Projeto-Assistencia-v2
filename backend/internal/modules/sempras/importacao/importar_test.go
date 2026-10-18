package importacao

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
)

const chaveTeste = "chave-falsa-de-teste"

func enc(s string) string { return cifrar(chaveTeste, s) }

// cpfValido gera um CPF válido ao acaso (o banco de teste é compartilhado).
func cpfValido() string {
	d := make([]int, 9, 11)
	for i := range d {
		d[i] = rand.IntN(10)
	}
	for _, n := range []int{9, 10} {
		soma := 0
		for i := 0; i < n; i++ {
			soma += d[i] * (n + 1 - i)
		}
		dv := soma * 10 % 11 % 10
		d = append(d, dv)
	}
	var b strings.Builder
	for _, x := range d {
		b.WriteByte(byte('0' + x))
	}
	return b.String()
}

// legadoTeste é um banco do 4.0 sintético num schema próprio.
type legadoTeste struct {
	t      *testing.T
	pool   *pgxpool.Pool
	schema string
}

// tabela cria a tabela do 4.0; colunas "nome:tipo" (o padrão é text).
func (lt legadoTeste) tabela(nome string, cols ...string) {
	lt.t.Helper()
	defs := []string{"id serial PRIMARY KEY"}
	for _, c := range cols {
		n, tipo, ok := strings.Cut(c, ":")
		if !ok {
			tipo = "text"
		}
		defs = append(defs, pgx.Identifier{n}.Sanitize()+" "+tipo)
	}
	if _, err := lt.pool.Exec(context.Background(), `CREATE TABLE `+pgx.Identifier{lt.schema, nome}.Sanitize()+` (`+strings.Join(defs, ", ")+`)`); err != nil {
		lt.t.Fatal(err)
	}
}

// linha insere com id explícito (os ids do 4.0 aparecem no relatório).
func (lt legadoTeste) linha(tabela string, id int, campos map[string]any) {
	lt.t.Helper()
	cols, args, ph := []string{"id"}, []any{id}, []string{"$1"}
	keys := make([]string, 0, len(campos))
	for k := range campos {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cols = append(cols, pgx.Identifier{k}.Sanitize())
		args = append(args, campos[k])
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	if _, err := lt.pool.Exec(context.Background(), `INSERT INTO `+pgx.Identifier{lt.schema, tabela}.Sanitize()+` (`+strings.Join(cols, ", ")+
		`) VALUES (`+strings.Join(ph, ", ")+`)`, args...); err != nil {
		lt.t.Fatalf("%s %d: %v", tabela, id, err)
	}
}

func novoLegado(t *testing.T, pool *pgxpool.Pool) legadoTeste {
	t.Helper()
	lt := legadoTeste{t: t, pool: pool, schema: "Casa Abrigo t" + uuid.NewString()[:8]}
	if _, err := pool.Exec(context.Background(), `CREATE SCHEMA `+pgx.Identifier{lt.schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP SCHEMA `+pgx.Identifier{lt.schema}.Sanitize()+` CASCADE`)
	})
	return lt
}

// casaAbrigo cria uma unidade do tipo Casa Abrigo.
func casaAbrigo(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	un := dbtest.Unidade(t, pool)
	if _, err := pool.Exec(context.Background(), `INSERT INTO sempras_unidades (unidade_id, tipo, painel_exibe_nome) VALUES ($1, 'casa_abrigo', false)`, un); err != nil {
		t.Fatal(err)
	}
	return un
}

type cenarioLegado struct {
	lt               legadoTeste
	cpfAna, cpfBruno string
	bruno            uuid.UUID // munícipe já cadastrado (mesmo CPF do 4.0)
	eva              uuid.UUID // munícipe já cadastrado (mesmo nome e nascimento)
	nomeEva          string
	muitoLongo       string
	relatorioGigante string
}

// povoar monta o 4.0 com um caso de cada regra do mapeamento.
func povoar(t *testing.T, pool *pgxpool.Pool) cenarioLegado {
	t.Helper()
	ctx := context.Background()
	repo := infrastructure.NewRepository()
	autor := dbtest.User(t, pool)
	sfx := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return 'a' + (r - '0')
		}
		return r
	}, uuid.NewString()[:6])
	c := cenarioLegado{lt: novoLegado(t, pool), cpfAna: cpfValido(), cpfBruno: cpfValido(), nomeEva: "Eva Teste " + sfx,
		muitoLongo: strings.Repeat("x", 2001), relatorioGigante: strings.Repeat("y", 100001)}
	for _, p := range []struct {
		id        *uuid.UUID
		nome, cpf string
		nasc, mae string
	}{{&c.bruno, "Bruno Já Cadastrado " + sfx, c.cpfBruno, "", ""}, {&c.eva, c.nomeEva, "", "2016-05-05", "Mãe Eva"}} {
		*p.id = uuid.New()
		if err := repo.InsertCidadao(ctx, pool, domain.Cidadao{ID: *p.id, Nome: p.nome, CPF: p.cpf, DataNascimento: p.nasc, NomeMae: p.mae,
			CreatedAt: time.Now(), UpdatedAt: time.Now()}, autor); err != nil {
			t.Fatal(err)
		}
	}
	lt := c.lt
	lt.tabela("acolhidos", "nome", "data_nascimento:date", "sexo", "data_entrada:date", "motivo_acolhimento", "situacao_juridica",
		"situacao_especial:boolean", "pcd:boolean", "nome_social", "apelido", "cpf_crianca", "nis_crianca", "identidade_genero",
		"orientacao_sexual", "raca_cor", "filiacao_1", "filiacao_2", "resp_nao_possui:boolean", "resp_nome", "cpf_responsavel", "resp_nis",
		"resp_nascimento:date", "resp_sexo", "resp_vinculo", "resp_cep", "resp_uf", "resp_email", "processo", "processo_2", "guia",
		"data_guia:date", "historico_acolhido_anteriormente", "outro_motivo", "cna", "laudo_medico", "laudo_medico_desc",
		"doenca_diabetes:boolean", "transtorno_down:boolean", "transtorno_desc", "social_relacao_trabalho", "desligamento_motivo",
		"desligamento_data_saida:date", "desligamento_guia", "desligamento_data_guia:date", "rg", "cpf", "processo_judicial",
		"observacoes_gerais", "motivo_desligamento", "data_saida:date", "criado_em:timestamp")
	lt.linha("acolhidos", 1, map[string]any{"nome": enc("Ana Júlia Teste " + sfx), "data_nascimento": "2015-03-01", "sexo": "Feminino",
		"data_entrada": "2026-01-10", "motivo_acolhimento": enc("Negligência, Abandono, Outro, briga na família"),
		"outro_motivo": "briga na família", "situacao_juridica": enc("Em processo de guarda"), "resp_vinculo": "Avó",
		"cpf_crianca": c.cpfAna[:3] + "." + c.cpfAna[3:6] + "." + c.cpfAna[6:9] + "-" + c.cpfAna[9:], "cpf_responsavel": "11144477735",
		"resp_nis": "12345678901", "resp_cep": "78700-000", "resp_uf": "mt", "resp_email": "tia@ex.com", "raca_cor": "Parda",
		"identidade_genero": "Cisgênero", "historico_acolhido_anteriormente": "Não sei", "cna": "Sim", "laudo_medico": "Sim",
		"laudo_medico_desc": "laudo", "doenca_diabetes": true, "transtorno_down": true, "rg": enc("1234567"), "processo": "N/I",
		"processo_judicial": "PJ-1", "guia": "G-1", "desligamento_motivo": "Adoção concluída", "desligamento_data_saida": "2026-09-01",
		"desligamento_guia": "GD-1", "desligamento_data_guia": "2026-09-02", "social_relacao_trabalho": "Trabalho Formal",
		"observacoes_gerais": enc("obs"), "criado_em": "2026-01-10 10:00:00"})
	lt.linha("acolhidos", 2, map[string]any{"nome": enc("Bruno Teste " + sfx), "data_nascimento": "2012-06-10", "sexo": "Masculino",
		"data_entrada": "2026-02-01", "motivo_acolhimento": "N/I", "cpf_crianca": c.cpfBruno, "cpf_responsavel": "123", "resp_nis": "99",
		"resp_cep": "123", "resp_uf": "Mato", "resp_email": "x@", "resp_vinculo": "Vizinho", "raca_cor": "Roxa", "nis_crianca": "123",
		"transtorno_desc": "TDAH"})
	lt.linha("acolhidos", 3, map[string]any{"nome": enc("Repetido"), "data_entrada": "2026-01-01", "sexo": "Feminino"})
	lt.linha("acolhidos", 4, map[string]any{"nome": enc("Repetido"), "data_nascimento": "2015-01-01", "sexo": "Feminino"})
	lt.linha("acolhidos", 5, map[string]any{"nome": cifrar("outra-chave", "Zé"), "data_nascimento": "2015-01-01", "data_entrada": "2026-01-01"})
	lt.linha("acolhidos", 6, map[string]any{"nome": enc("Carla Teste " + sfx), "data_nascimento": "2014-01-01", "sexo": "Feminino",
		"data_entrada": "2026-01-01", "motivo_acolhimento": enc("Orfandade"), "filiacao_1": "Mãe Carla", "motivo_desligamento": "fim do acolhimento"})
	lt.linha("acolhidos", 7, map[string]any{"nome": "Davi Teste " + sfx, "data_nascimento": "2013-01-01", "sexo": "Masculino",
		"data_entrada": "2026-01-05", "motivo_acolhimento": enc("Outro"), "outro_motivo": "", "filiacao_1": "Mãe Davi " + sfx,
		"motivo_desligamento": "ok", "data_saida": "2026-08-01", "resp_vinculo": "Avô"})
	lt.linha("acolhidos", 8, map[string]any{"nome": "Adulto Teste " + sfx, "data_nascimento": "1990-01-01", "sexo": "Masculino",
		"data_entrada": "2026-01-01", "motivo_acolhimento": "Abandono, Briga", "filiacao_1": "Mãe"})
	lt.linha("acolhidos", 9, map[string]any{"nome": c.nomeEva, "data_nascimento": "2016-05-05", "sexo": "Feminino",
		"data_entrada": "2026-03-01", "motivo_acolhimento": "Abandono, Abandono", "nis_crianca": "10987654321"})
	lt.linha("acolhidos", 10, map[string]any{"nome": "Fábio Teste " + sfx, "data_nascimento": "2016-01-01", "sexo": "Masculino",
		"data_entrada": "2026-03-01", "motivo_acolhimento": "Abandono", "cpf_crianca": "111"})
	lt.linha("acolhidos", 11, map[string]any{"nome": enc("Ana Reacolhida " + sfx), "cpf": enc(c.cpfAna), "data_nascimento": "2015-03-01",
		"sexo": "Feminino", "data_entrada": "2026-09-10", "motivo_acolhimento": "Abandono"})
	lt.linha("acolhidos", 12, map[string]any{"nome": "Ana Terceira " + sfx, "cpf_crianca": c.cpfAna, "data_nascimento": "2015-03-01",
		"sexo": "Feminino", "data_entrada": "2026-09-15", "motivo_acolhimento": "Abandono"})

	lt.linha("acolhidos", 13, map[string]any{"nome": "Gil Teste " + sfx, "data_nascimento": "2014-02-02", "sexo": "Masculino",
		"data_entrada": "2026-05-01", "motivo_acolhimento": "Abandono", "filiacao_1": "Mãe Gil", "desligamento_motivo": "transferido",
		"desligamento_data_saida": "2026-04-01"})
	lt.tabela("pia_saude_escola", "acolhido_id:int", "periodo_escola", "nome_escola", "cartao_sus", "carteira_vacina_atualizada:boolean", "criado_em:timestamp")
	lt.linha("pia_saude_escola", 1, map[string]any{"acolhido_id": 1, "periodo_escola": "Manhã", "nome_escola": enc("EMEB"), "criado_em": "2026-02-01 08:00:00"})
	lt.linha("pia_saude_escola", 2, map[string]any{"acolhido_id": 1, "nome_escola": "EMEB Nova", "carteira_vacina_atualizada": true, "criado_em": "2026-03-01 08:00:00"})
	lt.linha("pia_saude_escola", 3, map[string]any{"acolhido_id": 2, "periodo_escola": "Madrugada"})
	lt.linha("pia_saude_escola", 4, map[string]any{"acolhido_id": 9, "cartao_sus": strings.Repeat("9", 40)})
	lt.linha("pia_saude_escola", 5, map[string]any{"acolhido_id": 3})
	lt.tabela("pia_situacao_familiar", "acolhido_id:int", "nome_pai", "nome_mae", "contato_pais", "irmaos_acolhidos", "observacoes_familia")
	lt.linha("pia_situacao_familiar", 1, map[string]any{"acolhido_id": 1, "nome_pai": "João", "irmaos_acolhidos": "true", "observacoes_familia": "obs"})
	lt.linha("pia_situacao_familiar", 2, map[string]any{"acolhido_id": 2, "irmaos_acolhidos": "2 irmãos"})
	lt.tabela("pia_rotina", "acolhido_id:int", "atividades_manha", "atualizado_em:timestamp")
	lt.linha("pia_rotina", 1, map[string]any{"acolhido_id": 1, "atividades_manha": "escola", "atualizado_em": "2026-04-01 09:00:00"})
	lt.tabela("pia_intervencao_familiar", "acolhido_id:int", "objetivos", "atualizado_em:timestamp")
	lt.linha("pia_intervencao_familiar", 1, map[string]any{"acolhido_id": 7, "objetivos": "reintegrar", "atualizado_em": "2026-05-01 09:00:00"})
	lt.tabela("pia_plano_acao", "acolhido_id:int", "necessidade", "providencias", "prazo:date", "esportes", "criado_em:timestamp")
	lt.linha("pia_plano_acao", 1, map[string]any{"acolhido_id": 1, "necessidade": enc("matrícula"), "prazo": "2026-11-01", "criado_em": "2026-02-01 08:00:00"})
	lt.linha("pia_plano_acao", 2, map[string]any{"acolhido_id": 1, "esportes": "futebol"})
	lt.linha("pia_plano_acao", 3, map[string]any{"acolhido_id": 2})
	lt.linha("pia_plano_acao", 4, map[string]any{"acolhido_id": 4, "necessidade": "matrícula"})

	lt.tabela("usuarios", "nome", "senha")
	lt.linha("usuarios", 1, map[string]any{"nome": enc("Técnica Maria"), "senha": "hash"})
	lt.linha("usuarios", 2, map[string]any{"nome": cifrar("outra-chave", "X"), "senha": "hash"})
	lt.tabela("evolucoes_tecnicas", "acolhido_id:int", "usuario_id:int", "texto_evolucao", "tipo_evolucao", "data_hora:timestamp")
	lt.linha("evolucoes_tecnicas", 1, map[string]any{"acolhido_id": 1, "usuario_id": 1, "texto_evolucao": enc("evolução social"),
		"tipo_evolucao": enc("Geral"), "data_hora": "2026-02-02 10:00:00"})
	lt.linha("evolucoes_tecnicas", 2, map[string]any{"acolhido_id": 2, "usuario_id": 2, "texto_evolucao": "sessão", "tipo_evolucao": "Psicologia"})
	lt.linha("evolucoes_tecnicas", 3, map[string]any{"acolhido_id": 3, "texto_evolucao": "x"})
	lt.linha("evolucoes_tecnicas", 4, map[string]any{"acolhido_id": 9, "texto_evolucao": "sem técnico"})
	lt.linha("evolucoes_tecnicas", 5, map[string]any{"acolhido_id": 9, "texto_evolucao": strings.Repeat("z", 20001)})

	lt.tabela("visitantes_voluntarios", "acolhido_id:int", "nome_visitante", "documento_identidade", "parentesco_vinculo", "data_visita:date",
		"hora_entrada", "hora_saida", "observacoes_visita")
	lt.linha("visitantes_voluntarios", 1, map[string]any{"acolhido_id": 1, "nome_visitante": enc("Avó Maria"), "data_visita": "2026-02-03",
		"hora_entrada": "14:03:22", "observacoes_visita": enc("trouxe roupas (Tipo: Doação)")})
	lt.linha("visitantes_voluntarios", 2, map[string]any{"acolhido_id": 2, "nome_visitante": "Tio", "data_visita": "2026-02-04",
		"hora_entrada": "09:00", "hora_saida": "10:00:00", "observacoes_visita": "visita"})
	lt.linha("visitantes_voluntarios", 3, map[string]any{"acolhido_id": 2, "nome_visitante": "Prima", "data_visita": "2026-02-04",
		"hora_entrada": "9", "observacoes_visita": "x (Tipo: )"})
	lt.linha("visitantes_voluntarios", 4, map[string]any{"acolhido_id": 1, "nome_visitante": "Sem data"})
	lt.linha("visitantes_voluntarios", 5, map[string]any{"acolhido_id": 5, "nome_visitante": "De quem não entrou", "data_visita": "2026-02-04"})

	lt.tabela("documentos_termos", "acolhido_id:int", "tipo_termo", "data_evento:date", "responsavel_retirada", "motivo_observacao", "data_registro:timestamp")
	lt.linha("documentos_termos", 1, map[string]any{"acolhido_id": 1, "tipo_termo": "advertencia", "data_evento": "2026-02-04",
		"motivo_observacao": enc("atraso"), "data_registro": "2026-02-04 12:00:00"})
	lt.linha("documentos_termos", 2, map[string]any{"acolhido_id": 2, "tipo_termo": "multa", "data_registro": "2026-02-05 08:00:00"})
	lt.linha("documentos_termos", 3, map[string]any{"acolhido_id": 2, "tipo_termo": "passeio"})
	lt.linha("documentos_termos", 4, map[string]any{"acolhido_id": 1, "tipo_termo": "passeio", "data_evento": "2026-02-06", "motivo_observacao": c.muitoLongo})
	lt.linha("documentos_termos", 5, map[string]any{"acolhido_id": 6, "tipo_termo": "passeio", "data_evento": "2026-02-06"})

	lt.tabela("documentos_acolhido", "acolhido_id:int", "tipo", "caminho")
	lt.linha("documentos_acolhido", 1, map[string]any{"acolhido_id": 1, "tipo": "acolhido", "caminho": "uploads/1_acolhido.pdf"})

	lt.tabela("diario_logs", "modulo", "descricao", "usuario_id:int", "data_hora:timestamp")
	lt.linha("diario_logs", 1, map[string]any{"modulo": "CADASTRO", "descricao": "Acolhido cadastrado", "data_hora": "2026-01-10 10:00:00"})
	lt.linha("diario_logs", 2, map[string]any{"modulo": "DIÁRIO DE BORDO (GERAL)", "descricao": "Anotação referente ao dia 05/10/2026: rotina da casa",
		"usuario_id": 1, "data_hora": "2026-10-05 22:00:00"})
	lt.linha("diario_logs", 3, map[string]any{"modulo": enc("DIÁRIO - " + c.nomeEva), "descricao": enc("Anotação referente ao dia 06/10/2026: consulta"),
		"data_hora": "2026-10-06 15:00:00"})
	lt.linha("diario_logs", 4, map[string]any{"modulo": "DIÁRIO - Fulano", "descricao": "texto livre sem padrão", "data_hora": "2026-10-07 15:00:00"})
	lt.linha("diario_logs", 5, map[string]any{"modulo": "DIÁRIO DE BORDO (GERAL)", "descricao": "ok"})
	lt.linha("diario_logs", 6, map[string]any{"modulo": "DIÁRIO DE BORDO (GERAL)", "descricao": "Anotação referente ao dia 31/02/2026: data impossível",
		"data_hora": "2026-10-08 15:00:00"})
	lt.linha("diario_logs", 7, map[string]any{"modulo": cifrar("outra-chave", "DIÁRIO - X"), "descricao": "ilegível"})

	lt.tabela("relatorios_gerais", "tipo", "status", "data_entrega:date", "conteudo_texto", "criado_em:timestamptz")
	lt.linha("relatorios_gerais", 1, map[string]any{"tipo": "Relatório Mensal", "status": "revisao", "data_entrega": "2026-10-25",
		"conteudo_texto": "texto", "criado_em": "2026-10-01T10:00:00Z"})
	lt.linha("relatorios_gerais", 2, map[string]any{"tipo": "Outro", "status": "entregue", "data_entrega": "2026-09-01"})
	lt.linha("relatorios_gerais", 3, map[string]any{"tipo": "Relatório ao MP", "status": "pronto", "data_entrega": "2026-10-01"})
	lt.linha("relatorios_gerais", 4, map[string]any{"tipo": "", "status": "aprovado", "data_entrega": "2026-10-02"})
	lt.linha("relatorios_gerais", 5, map[string]any{"tipo": "Relatório Mensal", "status": "revisao"})
	lt.linha("relatorios_gerais", 6, map[string]any{"tipo": "Relatório Mensal", "status": "revisao", "data_entrega": "2026-10-02",
		"conteudo_texto": c.relatorioGigante})

	lt.tabela("scrum_tarefas", "titulo", "descricao", "status", "prioridade", "responsavel", "sprint", "prazo:date", "ordem:int",
		"criado_em:timestamp", "atualizado_em:timestamp")
	lt.linha("scrum_tarefas", 1, map[string]any{"titulo": enc("Comprar material"), "status": "doing", "prioridade": "alta",
		"prazo": "2026-10-20", "ordem": 2, "criado_em": "2026-10-01 10:00:00", "atualizado_em": "2026-10-02 10:00:00"})
	lt.linha("scrum_tarefas", 2, map[string]any{"titulo": "Ofício", "status": "blocked", "prioridade": "urgente", "ordem": -1})
	lt.linha("scrum_tarefas", 3, map[string]any{"titulo": ""})
	return c
}

func contagem(lidos, importados, ja, rejeitados, ignorados int) Contagem {
	return Contagem{Lidos: lidos, Importados: importados, JaImportados: ja, Rejeitados: rejeitados, Ignorados: ignorados}
}

// destino é o registro daqui que veio do id do 4.0.
func destino(t *testing.T, pool *pgxpool.Pool, un uuid.UUID, origem string, id int64) uuid.UUID {
	t.Helper()
	var d uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT destino_id FROM sempras_casa_abrigo_importacao WHERE unidade_id = $1 AND origem = $2 AND origem_id = $3`,
		un, origem, id).Scan(&d); err != nil {
		t.Fatalf("%s %d: %v", origem, id, err)
	}
	return d
}

func temAviso(avisos []Aviso, origem string, id int64, trecho string) bool {
	return slices.ContainsFunc(avisos, func(a Aviso) bool {
		return a.Origem == origem && a.ID == id && strings.Contains(a.Mensagem, trecho)
	})
}

// A importação inteira no banco de verdade: simula (nada fica), aplica,
// confere o mapeamento e roda de novo sem duplicar nada.
func TestImportarCasaAbrigo40(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	repo := infrastructure.NewRepository()
	c := povoar(t, pool)
	leg, err := Ler(ctx, pool, c.lt.schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(leg.Acolhidos) != 13 || len(leg.Usuarios) != 2 || leg.Usuarios[1] == "" {
		t.Fatalf("leitura do 4.0: %d acolhidos, usuários %v", len(leg.Acolhidos), leg.Usuarios)
	}
	un, autor := casaAbrigo(t, pool), dbtest.User(t, pool)
	op := Opcoes{Unidade: un, Autor: autor, Agora: time.Now()}
	dec := NovoDecifrador(chaveTeste)

	sim, err := Importar(ctx, pool, repo, leg, dec, op)
	if err != nil {
		t.Fatal(err)
	}
	var gravados int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sempras_casa_abrigo_importacao WHERE unidade_id = $1`, un).Scan(&gravados); err != nil || gravados != 0 {
		t.Fatalf("a simulação gravou: %d %v", gravados, err)
	}
	if !strings.Contains(sim.Resumo(), "SIMULAÇÃO") {
		t.Errorf("resumo da simulação: %s", sim.Resumo())
	}

	op.Aplicar = true
	rel, err := Importar(ctx, pool, repo, leg, dec, op)
	if err != nil {
		t.Fatal(err)
	}
	esperado := map[string]Contagem{
		"acolhidos": contagem(13, 5, 0, 8, 0), "pia": contagem(5, 3, 0, 2, 0), "pia_plano_acao": contagem(4, 2, 0, 2, 0),
		"evolucoes_tecnicas": contagem(5, 3, 0, 2, 0), "visitantes_voluntarios": contagem(5, 2, 0, 3, 0),
		"documentos_termos": contagem(5, 2, 0, 3, 0), "diario_logs": contagem(7, 4, 0, 1, 2), "relatorios_gerais": contagem(6, 4, 0, 2, 0),
		"scrum_tarefas": contagem(3, 2, 0, 1, 0), "documentos_acolhido": contagem(1, 0, 0, 0, 1),
	}
	for o, want := range esperado {
		if got := *rel.Contagens[o]; got != want || *sim.Contagens[o] != want {
			t.Errorf("%s: aplicado %+v, simulado %+v, esperado %+v", o, got, *sim.Contagens[o], want)
		}
	}
	if t.Failed() || testing.Verbose() {
		t.Log(rel.Resumo())
	}
	resumo := rel.Resumo()
	if !strings.Contains(resumo, "IMPORTAÇÃO APLICADA") || strings.Contains(resumo, "Ana Júlia") || strings.Contains(resumo, c.nomeEva) {
		t.Errorf("o resumo não leva nome de criança:\n%s", resumo)
	}
	for _, r := range []struct {
		origem string
		id     int64
		trecho string
	}{
		{"acolhidos", 3, "sem data de nascimento"}, {"acolhidos", 4, "sem data de entrada"}, {"acolhidos", 5, "sem nome legível"},
		{"acolhidos", 6, "sem data da saída"}, {"acolhidos", 8, "menores de 18"}, {"acolhidos", 10, "CPF, NIS"},
		{"acolhidos", 12, "ativo"}, {"acolhidos", 13, "anterior à entrada"}, {"evolucoes_tecnicas", 5, "20000"}, {"pia", 3, "não foi importado"}, {"visitantes_voluntarios", 4, "sem data"},
		{"documentos_termos", 3, "sem data"}, {"relatorios_gerais", 5, "sem data de entrega"},
	} {
		if !temAviso(rel.Rejeitados, r.origem, r.id, r.trecho) {
			t.Errorf("rejeição de %s %d (%q) não está no relatório: %+v", r.origem, r.id, r.trecho, rel.Rejeitados)
		}
	}
	for _, a := range []struct {
		origem string
		id     int64
		trecho string
	}{
		{"acolhidos", 1, "deduzido do motivo: adocao"}, {"acolhidos", 2, "mesmo CPF/NIS"}, {"acolhidos", 2, "CPF do responsável"},
		{"acolhidos", 2, "NIS do responsável"}, {"acolhidos", 2, "CEP"}, {"acolhidos", 2, "UF"}, {"acolhidos", 2, "e-mail"},
		{"acolhidos", 2, "vínculo"}, {"acolhidos", 2, "raça/cor"}, {"acolhidos", 2, "NIS da criança"}, {"acolhidos", 2, "sem motivo"},
		{"acolhidos", 7, "não deduzido"}, {"acolhidos", 7, "sem detalhe"}, {"acolhidos", 9, "mesmo nome e nascimento"},
		{"acolhidos", 11, "mesmo CPF/NIS"}, {"pia", 2, "período escolar"}, {"pia", 2, "sem data de gravação"},
		{"diario_logs", 4, "não identificado"}, {"documentos_termos", 2, "ficou \"outro\""}, {"relatorios_gerais", 3, "etapa"},
		{"scrum_tarefas", 2, "coluna"}, {"scrum_tarefas", 2, "prioridade"}, {"documentos_acolhido", 1, "anexe pela tela"},
	} {
		if !temAviso(rel.Avisos, a.origem, a.id, a.trecho) {
			t.Errorf("aviso de %s %d (%q) não está no relatório: %+v", a.origem, a.id, a.trecho, rel.Avisos)
		}
	}

	// o mapeamento, registro a registro
	ana, err := repo.GetAcolhimento(ctx, pool, destino(t, pool, un, "acolhidos", 1), false)
	if err != nil {
		t.Fatal(err)
	}
	if ana.Status != domain.AcolhimentoStatusDesligado || ana.DesligamentoTipo != "adocao" || ana.GuiaDesligamento != "GD-1" ||
		!slices.Equal(ana.Motivos, []string{"abandono", "negligencia", "outro"}) || ana.OutroMotivo != "briga na família" ||
		ana.ResponsavelVinculo != "avo_feminino" || ana.SituacaoJuridica != "processo_guarda" || ana.ProcessoJudicial != "PJ-1" ||
		ana.GuiaAcolhimento != "G-1" || ana.ResponsavelCPF != "11144477735" || ana.ResponsavelCEP != "78700000" || ana.ResponsavelUF != "MT" ||
		ana.RacaCor != "parda" || ana.HistoricoAcolhidoAnteriormente != "nao_sei" || ana.CNA != "sim" || ana.LaudoMedicoDescricao != "laudo" ||
		ana.RelacaoTrabalho != "formal" || ana.PossuiTranstorno != "sim" || !strings.Contains(ana.Observacoes, "RG: 1234567") ||
		!strings.HasPrefix(ana.Observacoes, "obs") {
		t.Errorf("acolhido 1: %+v", ana)
	}
	if ds, err := repo.ListDiagnosticos(ctx, pool, ana.ID); err != nil || len(ds) != 2 {
		t.Errorf("checklist de saúde do acolhido 1: %+v %v", ds, err)
	}
	bruno, _ := repo.GetAcolhimento(ctx, pool, destino(t, pool, un, "acolhidos", 2), false)
	if bruno.CidadaoID != c.bruno || bruno.ResponsavelVinculo != "" || bruno.ResponsavelCPF != "" || bruno.RacaCor != "" ||
		!strings.Contains(bruno.Observacoes, "Vizinho") || !strings.Contains(bruno.Observacoes, "Roxa") || bruno.PossuiTranstorno != "sim" ||
		!slices.Equal(bruno.Motivos, []string{"outro"}) {
		t.Errorf("acolhido 2: %+v", bruno)
	}
	davi, _ := repo.GetAcolhimento(ctx, pool, destino(t, pool, un, "acolhidos", 7), false)
	if davi.ResponsavelVinculo != "avo_masculino" || davi.DesligamentoTipo != "outro" || davi.OutroMotivo != SemDetalhe ||
		davi.MotivoDesligamento != "não informado no sistema anterior" || davi.DataDesligamento == nil {
		t.Errorf("acolhido 7: %+v", davi)
	}
	eva, _ := repo.GetAcolhimento(ctx, pool, destino(t, pool, un, "acolhidos", 9), false)
	if eva.CidadaoID != c.eva || !slices.Equal(eva.Motivos, []string{"abandono"}) {
		t.Errorf("acolhido 9: %+v", eva)
	}
	pia, err := repo.GetPiaFicha(ctx, pool, ana.ID)
	if err != nil || pia.Escola != "EMEB Nova" || !pia.VacinacaoAtualizada || pia.ContatoFamiliar != "Pai: João" ||
		!strings.Contains(pia.ObservacoesFamilia, "Irmãos acolhidos: sim") || pia.AtividadesManha != "escola" ||
		!pia.AtualizadoEm.Equal(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("P.I.A. do acolhido 1: %+v %v", pia, err)
	}
	if p, err := repo.GetPiaFicha(ctx, pool, bruno.ID); err != nil || !p.AtualizadoEm.Equal(bruno.DataEntrada) ||
		!strings.Contains(p.ObservacoesFamilia, "2 irmãos") {
		t.Errorf("P.I.A. do acolhido 2 (sem data: conta da entrada): %+v %v", p, err)
	}
	if ms, err := repo.ListPiaMetas(ctx, pool, ana.ID); err != nil || len(ms) != 2 {
		t.Errorf("metas do acolhido 1: %+v %v", ms, err)
	}
	regs, err := repo.RegistrosDoAcolhido(ctx, pool, bruno.CidadaoID, un)
	if err != nil || len(regs) != 1 || regs[0].Tipo != domain.TipoPsicologico ||
		!strings.HasPrefix(regs[0].Texto, "Registrado no sistema anterior (Casa Abrigo 4.0).") {
		t.Errorf("evolução do acolhido 2: %+v %v", regs, err)
	}
	if regs, _ := repo.RegistrosDoAcolhido(ctx, pool, ana.CidadaoID, un); len(regs) != 1 || !strings.Contains(regs[0].Texto, "por Técnica Maria") ||
		!regs[0].CreatedAt.Equal(time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("evolução do acolhido 1: %+v", regs)
	}
	vs, _ := repo.ListVisitantesDaUnidade(ctx, pool, un)
	if len(vs) != 2 || !slices.ContainsFunc(vs, func(v domain.Visitante) bool {
		return v.TipoVisita == "doacao" && v.Observacoes == "trouxe roupas" && v.HoraEntrada == "14:03"
	}) {
		t.Errorf("visitas: %+v", vs)
	}
	diario, _ := repo.ListDiario(ctx, pool, un, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if len(diario) != 4 || !slices.ContainsFunc(diario, func(d domain.AnotacaoDiario) bool {
		return d.AcolhimentoID != nil && *d.AcolhimentoID == eva.ID && d.Texto == "consulta" && d.DataReferencia.Day() == 6
	}) || !slices.ContainsFunc(diario, func(d domain.AnotacaoDiario) bool {
		return d.AutorNome == "Técnica Maria" && d.Texto == "rotina da casa"
	}) {
		t.Errorf("diário: %+v", diario)
	}
	rels, _ := repo.ListRelatorios(ctx, pool, un)
	if len(rels) != 4 || !slices.ContainsFunc(rels, func(r domain.RelatorioCasaAbrigo) bool { return r.Tipo == "outro" && r.TipoOutro == "Relatório ao MP" }) ||
		!slices.ContainsFunc(rels, func(r domain.RelatorioCasaAbrigo) bool { return r.TipoOutro == "Relatório do sistema anterior" }) {
		t.Errorf("relatórios: %+v", rels)
	}
	tfs, _ := repo.ListTarefas(ctx, pool, un)
	if len(tfs) != 2 || !slices.ContainsFunc(tfs, func(x domain.TarefaCasaAbrigo) bool {
		return x.Titulo == "Ofício" && x.Status == "backlog" && x.Prioridade == "media" && x.Ordem == 0
	}) {
		t.Errorf("tarefas: %+v", tfs)
	}

	// de novo: nada é gravado duas vezes
	again, err := Importar(ctx, pool, repo, leg, dec, op)
	if err != nil {
		t.Fatal(err)
	}
	for o, want := range esperado {
		if got := *again.Contagens[o]; got.JaImportados != want.Importados || got.Importados != 0 {
			t.Errorf("segunda rodada de %s: %+v", o, got)
		}
	}
}
