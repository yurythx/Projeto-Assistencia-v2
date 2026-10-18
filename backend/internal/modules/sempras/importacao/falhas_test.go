package importacao

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
)

func contagens(r Relatorio) map[string]Contagem {
	out := map[string]Contagem{}
	for k, v := range r.Contagens {
		out[k] = *v
	}
	return out
}

// Cada chamada ao repositório que falha encerra a importação com erro (nada
// gravado pela metade); a transação envenenada também — ou, quando o veneno
// cai num registro recusado (o savepoint dele é desfeito), o resultado é o
// mesmo de sem veneno: nunca um resultado diferente em silêncio.
func TestImportarFalhas(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	c := povoar(t, pool)
	leg, err := Ler(ctx, pool, c.lt.schema)
	if err != nil {
		t.Fatal(err)
	}
	dec, autor := NovoDecifrador(chaveTeste), dbtest.User(t, pool)
	real := infrastructure.NewRepository()

	if _, err := Importar(ctx, pool, real, leg, dec, Opcoes{Unidade: dbtest.Unidade(t, pool), Autor: autor, Agora: time.Now()}); !errors.Is(err, ErrUnidade) {
		t.Errorf("destino que não é Casa Abrigo: %v", err)
	}
	if _, err := Importar(ctx, pool, real, leg, dec, Opcoes{Unidade: uuid.New(), Autor: autor, Agora: time.Now()}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("destino inexistente: %v", err)
	}
	if _, err := Importar(ctx, pool, real, leg, NovoDecifrador("errada"), Opcoes{Unidade: casaAbrigo(t, pool), Autor: autor, Agora: time.Now()}); !errors.Is(err, ErrDecifrar) {
		t.Errorf("chave errada: %v", err)
	}
	// sem nada a importar, o único passo depois da unidade é a auditoria
	vazio := &faultRepo{inner: real, poisonAt: 1}
	if _, err := Importar(ctx, pool, vazio, Legado{}, dec, Opcoes{Unidade: casaAbrigo(t, pool), Autor: autor, Agora: time.Now()}); err == nil {
		t.Error("auditoria que falha: a importação deveria falhar")
	}

	// a primeira importação (simulada) e, depois de aplicada, a repetida
	aplicada := casaAbrigo(t, pool)
	for _, rodada := range []struct {
		nome    string
		unidade uuid.UUID
	}{{"primeira importação", aplicada}, {"importação repetida", aplicada}} {
		if rodada.nome == "importação repetida" {
			if _, err := Importar(ctx, pool, real, leg, dec, Opcoes{Unidade: aplicada, Autor: autor, Aplicar: true, Agora: time.Now()}); err != nil {
				t.Fatal(err)
			}
		}
		op := Opcoes{Unidade: rodada.unidade, Autor: autor, Agora: time.Now()}
		probe := &faultRepo{inner: real}
		base, err := Importar(ctx, pool, probe, leg, dec, op)
		if err != nil {
			t.Fatalf("%s sem falha: %v", rodada.nome, err)
		}
		for i := range probe.trace {
			if _, err := Importar(ctx, pool, &faultRepo{inner: real, failAt: i + 1}, leg, dec, op); err == nil {
				t.Errorf("%s: falha na chamada %d (%s) foi engolida", rodada.nome, i+1, probe.trace[i])
			}
			r, err := Importar(ctx, pool, &faultRepo{inner: real, poisonAt: i + 1}, leg, dec, op)
			if err == nil && !maps.Equal(contagens(r), contagens(base)) {
				t.Errorf("%s: veneno depois da chamada %d (%s) mudou o resultado: %+v", rodada.nome, i+1, probe.trace[i], contagens(r))
			}
		}
	}
}

// txFalso deixa falhar o savepoint (abrir ou liberar).
type txFalso struct {
	pgx.Tx
	falharBegin, falharCommit bool
}

func (t txFalso) Begin(ctx context.Context) (pgx.Tx, error) {
	if t.falharBegin {
		return nil, dbtest.ErrInjected
	}
	sp, err := t.Tx.Begin(ctx)
	return spFalso{Tx: sp, falharCommit: t.falharCommit}, err
}

type spFalso struct {
	pgx.Tx
	falharCommit bool
}

func (s spFalso) Commit(ctx context.Context) error {
	if s.falharCommit {
		_ = s.Tx.Rollback(ctx)
		return dbtest.ErrInjected
	}
	return s.Tx.Commit(ctx)
}

func TestItemSavepointFalha(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	un, autor := casaAbrigo(t, pool), dbtest.User(t, pool)
	for _, f := range []txFalso{{falharBegin: true}, {falharCommit: true}} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		f.Tx = tx
		im := &importador{tx: f, op: Opcoes{Unidade: un, Autor: autor}, rel: &Relatorio{Contagens: map[string]*Contagem{"acolhidos": {}}}}
		_, ok, err := im.item(ctx, "acolhidos", 1, func(context.Context, pgx.Tx) (uuid.UUID, error) { return uuid.New(), nil })
		if ok || !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("savepoint %+v: %v %v", f, ok, err)
		}
		_ = tx.Rollback(ctx)
	}
}

// dbFalso responde às consultas do Ler: as `existe` dizem "sim" até a
// chamada limite (depois falham); cada Query tem a sua resposta.
type dbFalso struct {
	existeAte int
	consultas []dbtest.QueryResult
	qr, q     int
}

type linhaBool struct{ err error }

func (l linhaBool) Scan(dst ...any) error {
	if l.err != nil {
		return l.err
	}
	*dst[0].(*bool) = true
	return nil
}

func (d *dbFalso) QueryRow(context.Context, string, ...any) pgx.Row {
	d.qr++
	if d.qr > d.existeAte {
		return linhaBool{err: dbtest.ErrInjected}
	}
	return linhaBool{}
}

func (d *dbFalso) Query(context.Context, string, ...any) (pgx.Rows, error) {
	d.q++
	if d.q > len(d.consultas) {
		return dbtest.NoRows(), nil
	}
	r := d.consultas[d.q-1]
	return r.Rows, r.Err
}

func (d *dbFalso) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, dbtest.ErrInjected
}

// jsonRuim é uma linha cujo texto não é JSON.
type jsonRuim struct {
	pgx.Rows
	lida bool
}

func (r *jsonRuim) Next() bool { ok := !r.lida; r.lida = true; return ok }
func (r *jsonRuim) Scan(dst ...any) error {
	*dst[0].(*string) = "{"
	return nil
}
func (r *jsonRuim) Close()     {}
func (r *jsonRuim) Err() error { return nil }

func TestLerFalhas(t *testing.T) {
	ctx := context.Background()
	casos := map[string]*dbFalso{
		"existe falha":           {},
		"consulta falha":         {existeAte: 99, consultas: []dbtest.QueryResult{{Err: dbtest.ErrInjected}}},
		"linha ilegível":         {existeAte: 99, consultas: []dbtest.QueryResult{{Rows: dbtest.BadRows()}}},
		"JSON ruim":              {existeAte: 99, consultas: []dbtest.QueryResult{{Rows: &jsonRuim{}}}},
		"conexão cai na leitura": {existeAte: 99, consultas: []dbtest.QueryResult{{Rows: dbtest.ErrRows()}}},
		"usuarios: existe falha": {existeAte: 13},
	}
	usuariosFalha := &dbFalso{existeAte: 99}
	for range 13 {
		usuariosFalha.consultas = append(usuariosFalha.consultas, dbtest.QueryResult{Rows: dbtest.NoRows()})
	}
	usuariosFalha.consultas = append(usuariosFalha.consultas, dbtest.QueryResult{Err: dbtest.ErrInjected})
	casos["usuarios: consulta falha"] = usuariosFalha
	for nome, db := range casos {
		if _, err := Ler(ctx, db, SchemaLegado); err == nil {
			t.Errorf("%s: deveria falhar", nome)
		}
	}

	// um 4.0 com só a tabela de acolhidos (sem PIA, Kanban nem usuários)
	pool := dbtest.Pool(t)
	lt := novoLegado(t, pool)
	lt.tabela("acolhidos", "nome")
	lt.linha("acolhidos", 7, map[string]any{"nome": "Só"})
	leg, err := Ler(ctx, pool, lt.schema)
	if err != nil || len(leg.Acolhidos) != 1 || leg.Acolhidos[0].ID() != 7 || len(leg.Usuarios) != 0 || leg.Tarefas != nil {
		t.Errorf("4.0 mínimo: %+v %v", leg, err)
	}
}

func TestLinha(t *testing.T) {
	var l Linha
	if err := json.Unmarshal([]byte(`{"n": 12, "b": true, "s": " x ", "d": "2026-13-45", "curta": "2026", "ts": "2026-10-01T10:00:00-04:00", "sim": "Sim"}`), &l); err != nil {
		t.Fatal(err)
	}
	if l.Texto("n") != "12" || l.Inteiro("n") != 12 || !l.Logico("b") || !l.Logico("sim") || l.Logico("s") || l.Texto("s") != "x" ||
		l.Texto("nada") != "" || l.Dia("d") != nil || l.Dia("curta") != nil || l.Instante("nada") != nil || l.Inteiro("s") != 0 {
		t.Errorf("leitura de campos: %+v", l)
	}
	if ts := l.Instante("ts"); ts == nil || !ts.Equal(time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC)) {
		t.Errorf("timestamp com fuso: %v", ts)
	}
}

func TestMapeamento(t *testing.T) {
	for motivo, tipo := range map[string]string{
		"Adotada pela família X": "adocao", "transferida para Cuiabá": "transferencia", "completou 18 anos": "maioridade",
		"maioridade": "maioridade", "evadiu da casa": "evasao", "fuga": "evasao", "Reintegração à família extensa (avó)": "reintegracao_familia_extensa",
		"reintegrada": "reintegracao_familia_natural", "retorno à família": "reintegracao_familia_natural",
		"guarda concedida ao tio": "guarda", "decisão do juiz": "outro",
	} {
		if got, inferido := tipoDesligamento(motivo); got != tipo || inferido != (tipo != "outro") {
			t.Errorf("tipoDesligamento(%q) = %s %v", motivo, got, inferido)
		}
	}
	if l, o := motivos("Outro, Briga, N/I", ""); strings.Join(l, ",") != "outro" || o != "Briga" {
		t.Errorf("motivos com detalhe solto: %v %q", l, o)
	}
	if l, o := motivos("", ""); l != nil || o != "" {
		t.Errorf("sem motivos: %v %q", l, o)
	}
	for rot, want := range map[string]string{"Avó": "avo_feminino", "AVÔ": "avo_masculino", "Mãe": "mae", "Família Extensa (Outros)": "familia_extensa", "": ""} {
		if v, ok := vinculo(rot); !ok || v != want {
			t.Errorf("vinculo(%q) = %q %v", rot, v, ok)
		}
	}
	if _, ok := vinculo("Vizinho"); ok {
		t.Error("vínculo fora da lista")
	}
	if chave("  Não-binário!! ") != "nao binario" || !naoInformado("Selecione...") || naoInformado("Sim") {
		t.Error("normalização de rótulo")
	}
	if tipoEvolucao("Psicológica") != domain.TipoPsicologico || tipoEvolucao("Geral") != domain.TipoSocial {
		t.Error("tipo de evolução")
	}
	if hhmm("9") != "9" || hhmm("09:30:00") != "09:30" {
		t.Error("hora")
	}
	if NovoDecifrador(chaveTeste).textoOu(cifrar("outra-chave", "x")) != "" {
		t.Error("textoOu com chave errada")
	}
	if !recusado(recusa("x")) || !recusado(&domain.ValidationError{Msg: "x"}) || !recusado(domain.ErrAcolhimentoAtivo) || recusado(dbtest.ErrInjected) {
		t.Error("recusado")
	}
}
