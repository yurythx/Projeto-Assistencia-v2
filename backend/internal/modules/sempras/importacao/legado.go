package importacao

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
)

// SchemaLegado é o schema do banco do 4.0 (server.js: search_path "Casa Abrigo").
const SchemaLegado = "Casa Abrigo"

// Linha é um registro do 4.0 como o banco o devolve em JSON (row_to_json):
// a leitura não depende da lista exata de colunas de cada versão do 4.0.
type Linha map[string]any

// Legado é o conteúdo do banco do 4.0.
type Legado struct {
	Acolhidos      []Linha
	Evolucoes      []Linha
	Visitantes     []Linha
	Termos         []Linha
	Documentos     []Linha
	PiaSaude       []Linha
	PiaFamilia     []Linha
	PiaPlano       []Linha
	PiaRotina      []Linha
	PiaIntervencao []Linha
	Diario         []Linha
	Relatorios     []Linha
	Tarefas        []Linha
	// Usuarios: id → nome (só isso: a tabela tem as senhas do 4.0).
	Usuarios map[int64]string
}

// Ler lê todas as tabelas do 4.0 que existirem no schema (as do PIA e do
// Kanban o 4.0 cria só quando a tela é usada).
func Ler(ctx context.Context, db database.DBTX, schema string) (Legado, error) {
	l := Legado{Usuarios: map[int64]string{}}
	for _, t := range []struct {
		nome string
		dst  *[]Linha
	}{
		{"acolhidos", &l.Acolhidos}, {"evolucoes_tecnicas", &l.Evolucoes}, {"visitantes_voluntarios", &l.Visitantes},
		{"documentos_termos", &l.Termos}, {"documentos_acolhido", &l.Documentos}, {"pia_saude_escola", &l.PiaSaude},
		{"pia_situacao_familiar", &l.PiaFamilia}, {"pia_plano_acao", &l.PiaPlano}, {"pia_rotina", &l.PiaRotina},
		{"pia_intervencao_familiar", &l.PiaIntervencao}, {"diario_logs", &l.Diario}, {"relatorios_gerais", &l.Relatorios},
		{"scrum_tarefas", &l.Tarefas},
	} {
		ident := pgx.Identifier{schema, t.nome}.Sanitize()
		ok, err := existe(ctx, db, ident)
		if err != nil || !ok {
			if err != nil {
				return Legado{}, err
			}
			continue
		}
		if *t.dst, err = linhas(ctx, db, `SELECT row_to_json(t)::text FROM `+ident+` t ORDER BY t.id`); err != nil {
			return Legado{}, fmt.Errorf("lendo %s: %w", t.nome, err)
		}
	}
	ident := pgx.Identifier{schema, "usuarios"}.Sanitize()
	ok, err := existe(ctx, db, ident)
	if err != nil || !ok {
		return l, err
	}
	us, err := linhas(ctx, db, `SELECT json_build_object('id', id, 'nome', nome)::text FROM `+ident)
	if err != nil {
		return Legado{}, fmt.Errorf("lendo usuarios: %w", err)
	}
	for _, u := range us {
		l.Usuarios[u.ID()] = u.Texto("nome")
	}
	return l, nil
}

func existe(ctx context.Context, db database.DBTX, ident string) (bool, error) {
	var ok bool
	if err := db.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, ident).Scan(&ok); err != nil {
		return false, fmt.Errorf("conferindo %s: %w", ident, err)
	}
	return ok, nil
}

func linhas(ctx context.Context, db database.DBTX, sql string) ([]Linha, error) {
	rows, err := db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Linha
	for rows.Next() {
		var bruto string
		if err := rows.Scan(&bruto); err != nil {
			return nil, err
		}
		d := json.NewDecoder(strings.NewReader(bruto))
		d.UseNumber()
		var l Linha
		if err := d.Decode(&l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ID é a chave serial da linha (0 se não houver).
func (l Linha) ID() int64 { return l.Inteiro("id") }

// Inteiro lê um número inteiro (0 se vazio ou inválido).
func (l Linha) Inteiro(campo string) int64 {
	n, _ := strconv.ParseInt(l.Texto(campo), 10, 64)
	return n
}

// Texto lê o campo como texto, sem os espaços das pontas ("" se nulo).
func (l Linha) Texto(campo string) string {
	switch v := l[campo].(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		return fmt.Sprint(v)
	}
}

// Logico lê um campo booleano (o 4.0 grava true/false e às vezes "Sim").
func (l Linha) Logico(campo string) bool {
	switch chave(l.Texto(campo)) {
	case "true", "t", "sim", "1":
		return true
	}
	return false
}

// Dia lê uma data (AAAA-MM-DD, com ou sem hora) como meia-noite UTC — a
// data "pura" do SEMPRAS.
func (l Linha) Dia(campo string) *time.Time {
	v := l.Texto(campo)
	if len(v) < 10 {
		return nil
	}
	d, err := time.Parse(time.DateOnly, v[:10])
	if err != nil {
		return nil
	}
	return &d
}

// Instante lê um timestamp; sem fuso, é UTC (o servidor do 4.0 grava em UTC
// e subtrai 4 horas só para mostrar).
func (l Linha) Instante(campo string) *time.Time {
	v := l.Texto(campo)
	for _, f := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999"} {
		if t, err := time.Parse(f, v); err == nil {
			return &t
		}
	}
	return l.Dia(campo)
}
