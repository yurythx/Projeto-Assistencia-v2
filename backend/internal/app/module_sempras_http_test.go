package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/kernel"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/ws"
)

// semprasCenario monta a secretaria (entidade de slug "sempras"), duas
// unidades e as pessoas de cada perfil (ADR 030).
type semprasCenario struct {
	h                               *apiHarness
	admin, recepA, tecA, psiA, tecB string
	sup, semPerfil                  string
	entidade, unA, unB              string
	psiAID, tecAID                  uuid.UUID
}

func (h *apiHarness) perfilID(slug string) string {
	h.t.Helper()
	var id string
	if err := h.d.DB.QueryRow(context.Background(), `SELECT id FROM perfis WHERE slug = $1`, slug).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	return id
}

func (h *apiHarness) lotar(admin string, user uuid.UUID, perfil, campo, escopo string) {
	h.t.Helper()
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/users/"+user.String()+"/lotacoes", admin,
		`{"perfil_id":"`+h.perfilID(perfil)+`","`+campo+`":"`+escopo+`"}`)
}

func newSemprasCenario(t *testing.T) *semprasCenario {
	t.Helper()
	h := newHarness(t)
	c := &semprasCenario{h: h, admin: h.admin()}
	if err := h.d.DB.QueryRow(context.Background(), `INSERT INTO entidades (nome, sigla, slug)
		VALUES ('Secretaria Municipal de Promoção e Assistência Social', 'SEMPRAS', 'sempras')
		ON CONFLICT (slug) DO UPDATE SET ativo = true RETURNING id`).Scan(&c.entidade); err != nil {
		t.Fatal(err)
	}
	sfx := uuid.NewString()[:6]
	c.unA = data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", c.admin,
		`{"entidade_id":"`+c.entidade+`","nome":"CRAS A `+sfx+`"}`)).ID
	c.unB = data[idResp](t, h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/unidades", c.admin,
		`{"entidade_id":"`+c.entidade+`","nome":"CRAS B `+sfx+`"}`)).ID

	var id uuid.UUID
	id, c.recepA = h.user("nexus-user")
	h.lotar(c.admin, id, "sempras-recepcionista", "unidade_id", c.unA)
	c.tecAID, c.tecA = h.user("nexus-user")
	h.lotar(c.admin, c.tecAID, "sempras-tecnico", "unidade_id", c.unA)
	id, c.tecB = h.user("nexus-user")
	h.lotar(c.admin, id, "sempras-tecnico", "unidade_id", c.unB)
	id, c.sup = h.user("nexus-user")
	h.lotar(c.admin, id, "sempras-superintendente", "entidade_id", c.entidade)
	_, c.semPerfil = h.user("nexus-user")

	// O psicólogo chega pelo grupo do AD (SEMPRAS_<UNIDADE>_PSICOLOGO).
	grupo := "SEMPRAS_CRAS_A_" + strings.ToUpper(sfx) + "_PSICOLOGO"
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/iam/ad-mappings", c.admin,
		`{"ad_group":"`+grupo+`","perfil_id":"`+h.perfilID("sempras-psicologo")+`","unidade_id":"`+c.unA+`"}`)
	c.psiAID, _ = h.user("nexus-user")
	if _, err := h.d.DB.Exec(context.Background(), `UPDATE users SET groups = $2 WHERE id = $1`, c.psiAID, []string{grupo}); err != nil {
		t.Fatal(err)
	}
	c.psiA = h.relogin(c.psiAID)
	return c
}

type semprasItem struct {
	ID           string `json:"id"`
	CidadaoID    string `json:"cidadao_id"`
	Senha        int    `json:"senha"`
	SenhaExibida string `json:"senha_exibida"`
	Status       string `json:"status"`
	Tipo         string `json:"tipo"`
	Nome         string `json:"nome"`
	Local        string `json:"local"`
}

type semprasResumo struct {
	ID                 string `json:"id"`
	CPF                string `json:"cpf"`
	NIS                string `json:"nis"`
	AtendimentosNaArea int    `json:"atendimentos_na_area"`
}

type semprasProntuario struct {
	Cidadao struct {
		ID  string `json:"id"`
		CPF string `json:"cpf"`
	} `json:"cidadao"`
	Atendimentos []semprasItem `json:"atendimentos"`
	Registros    []struct {
		Tipo     string `json:"tipo"`
		Texto    string `json:"texto"`
		Sigiloso bool   `json:"sigiloso"`
	} `json:"registros"`
}

func (c *semprasCenario) cadastrar(tok, body string) string {
	c.h.t.Helper()
	return data[idResp](c.h.t, c.h.expect(http.StatusCreated, http.MethodPost, "/api/v1/sempras/cidadaos", tok, body)).ID
}

func (c *semprasCenario) incluir(tok, unidade, cidadao, tipo, prioridade string) semprasItem {
	c.h.t.Helper()
	return data[semprasItem](c.h.t, c.h.expect(http.StatusCreated, http.MethodPost, "/api/v1/sempras/unidades/"+unidade+"/fila", tok,
		`{"cidadao_id":"`+cidadao+`","tipo":"`+tipo+`","prioridade":"`+prioridade+`"}`))
}

func (c *semprasCenario) acao(want int, tok, id, acao string) {
	c.h.t.Helper()
	c.h.expect(want, http.MethodPost, "/api/v1/sempras/atendimentos/"+id+"/"+acao, tok, "")
}

func (c *semprasCenario) auditCount(action, resource string) int {
	c.h.t.Helper()
	var n int
	if err := c.h.d.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE action = $1 AND resource_id = $2`, action, resource).Scan(&n); err != nil {
		c.h.t.Fatal(err)
	}
	return n
}

// Jornada do munícipe: busca, cadastro, fila, chamada, atendimento e
// prontuário — cada perfil só onde foi concedido (ADR 030).
func TestSemprasJornadaHTTP(t *testing.T) {
	c := newSemprasCenario(t)
	h := c.h

	// Nasce desativado: tudo 404 até a gestão ligar.
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/sempras/unidades", c.recepA, "")
	h.setModule(c.admin, "sempras", true)

	// Unidades: cada um vê a sua; o superintendente, as da secretaria.
	unidades := func(tok string) []string {
		var out []string
		for _, u := range data[[]idResp](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/unidades", tok, "")) {
			out = append(out, u.ID)
		}
		return out
	}
	if got := unidades(c.recepA); len(got) != 1 || got[0] != c.unA {
		t.Fatalf("recepcionista da A deveria ver só a A: %v", got)
	}
	for _, tok := range []string{c.sup, c.admin} {
		if got := strings.Join(unidades(tok), ","); !strings.Contains(got, c.unA) || !strings.Contains(got, c.unB) {
			t.Fatalf("superintendente/admin deveria ver A e B: %v", got)
		}
	}
	if got := unidades(c.semPerfil); len(got) != 0 {
		t.Fatalf("sem perfil não vê unidade: %v", got)
	}

	// Cadastro: CPF conferido, documento único, identificação alternativa.
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/sempras/cidadaos", c.recepA, `{"nome":"Fulano","cpf":"123.456.789-00"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/sempras/cidadaos", c.recepA, `{"nome":"Sem Documento"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/sempras/cidadaos", c.semPerfil, `{"nome":"Maria","cpf":"52998224725"}`)
	cpf := cpfTeste(t)
	maria := c.cadastrar(c.recepA, `{"nome":"Maria da Silva","nome_social":"","cpf":"`+cpf+`","telefone":"(66) 99999-0000"}`)
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/sempras/cidadaos", c.tecB, `{"nome":"Outra Maria","cpf":"`+cpf+`"}`)
	joao := c.cadastrar(c.recepA, `{"nome":"João Sem Documento `+uuid.NewString()[:4]+`","data_nascimento":"1990-05-01","nome_mae":"Ana"}`)
	jose := c.cadastrar(c.tecA, `{"nome":"José Idoso","nis":"`+cpfTeste(t)+`"}`)
	if c.auditCount("sempras.cidadao.criado", maria) != 1 {
		t.Fatal("cadastro sem auditoria")
	}
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/sempras/cidadaos/"+maria, c.recepA,
		`{"nome":"Maria da Silva","nome_social":"Mari","cpf":"`+cpf+`","telefone":"(66) 99999-0000"}`)
	if c.auditCount("sempras.cidadao.atualizado", maria) != 1 {
		t.Fatal("edição sem auditoria")
	}
	// Editar para um CPF que já é de outra pessoa: conflito.
	h.expect(http.StatusConflict, http.MethodPut, "/api/v1/sempras/cidadaos/"+jose, c.recepA, `{"nome":"José Idoso","cpf":"`+cpf+`"}`)
	h.expect(http.StatusNotFound, http.MethodPut, "/api/v1/sempras/cidadaos/"+uuid.NewString(), c.recepA, `{"nome":"Xis","cpf":"`+cpf+`"}`)
	h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/cidadaos/"+maria, c.recepA, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/sempras/cidadaos/"+uuid.NewString(), c.recepA, "")

	// Busca por CPF (formatado) e por nome; documentos mascarados.
	achados := data[[]semprasResumo](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/cidadaos?q="+cpf[:3]+"."+cpf[3:6]+"."+cpf[6:9]+"-"+cpf[9:], c.recepA, ""))
	if len(achados) != 1 || achados[0].ID != maria || strings.Contains(achados[0].CPF, cpf[:3]) {
		t.Fatalf("busca por CPF: %+v", achados)
	}
	if got := data[[]semprasResumo](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/cidadaos?q=mari", c.recepA, "")); len(got) == 0 {
		t.Fatal("busca por nome social/nome deveria achar")
	}
	if got := data[[]semprasResumo](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/cidadaos?q=00000000000", c.recepA, "")); len(got) != 0 {
		t.Fatalf("CPF inexistente: %+v", got)
	}
	h.expect(http.StatusUnprocessableEntity, http.MethodGet, "/api/v1/sempras/cidadaos?q=ab", c.recepA, "")

	// Fila da A: senha do dia, prioridade legal, um por dia.
	m := c.incluir(c.recepA, c.unA, maria, "social", "")
	if m.SenhaExibida != fmt.Sprintf("N%03d", m.Senha) || m.Status != "aguardando" {
		t.Fatalf("entrada: %+v", m)
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/sempras/unidades/"+c.unA+"/fila", c.recepA, `{"cidadao_id":"`+maria+`","tipo":"social"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/sempras/unidades/"+c.unA+"/fila", c.recepA, `{"cidadao_id":"`+maria+`","tipo":"social","prioridade":"vip"}`)
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/sempras/unidades/"+c.unA+"/fila", c.recepA, `{"cidadao_id":"`+uuid.NewString()+`","tipo":"social"}`)
	jo := c.incluir(c.tecA, c.unA, jose, "social", "idoso_80")
	if !strings.HasPrefix(jo.SenhaExibida, "P") || jo.Senha != m.Senha+1 {
		t.Fatalf("prioridade: %+v", jo)
	}
	ps := c.incluir(c.recepA, c.unA, joao, "psicologico", "")

	// Escopo: B não vê nem mexe na A; a gestão vê.
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/sempras/unidades/"+c.unA+"/fila", c.tecB, "")
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/sempras/unidades/"+c.unA+"/fila", c.tecB, `{"cidadao_id":"`+maria+`","tipo":"social"}`)
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/sempras/unidades/"+c.unB+"/fila", c.recepA, `{"cidadao_id":"`+maria+`","tipo":"social"}`)
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/sempras/unidades/"+uuid.NewString()+"/fila", c.sup, "")
	c.acao(http.StatusNotFound, c.tecB, m.ID, "chamar")
	fila := data[[]semprasItem](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/unidades/"+c.unA+"/fila", c.sup, ""))
	if len(fila) != 3 || fila[0].ID != jo.ID || fila[0].Nome != "José Idoso" {
		t.Fatalf("fila ordenada por prioridade: %+v", fila)
	}

	// Chamar: só quem atende; o próximo é o prioritário; sem corrida.
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/sempras/unidades/"+c.unA+"/fila/proximo", c.recepA, "")
	prox := data[semprasItem](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/sempras/unidades/"+c.unA+"/fila/proximo", c.tecA, `{"local":"Sala 2"}`))
	if prox.ID != jo.ID || prox.Status != "chamado" || prox.Local != "Sala 2" {
		t.Fatalf("próximo deveria ser o idoso 80+: %+v", prox)
	}
	c.acao(http.StatusOK, c.tecA, jo.ID, "iniciar")
	c.acao(http.StatusOK, c.tecA, jo.ID, "concluir")
	c.acao(http.StatusConflict, c.tecA, jo.ID, "concluir")
	if c.auditCount("sempras.fila.concluido", jo.ID) != 1 || c.auditCount("sempras.fila.incluido", jo.ID) != 1 {
		t.Fatal("mudanças de estado sem auditoria")
	}
	var eventos int
	_ = h.d.DB.QueryRow(context.Background(), `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1`, jo.ID).Scan(&eventos)
	if eventos != 4 {
		t.Fatalf("esperado 1 evento por mudança (4), veio %d", eventos)
	}

	// O técnico pula o psicológico; só a psicologia o chama.
	prox = data[semprasItem](t, h.expect(http.StatusOK, http.MethodPost, "/api/v1/sempras/unidades/"+c.unA+"/fila/proximo", c.tecA, ""))
	if prox.ID != m.ID {
		t.Fatalf("técnico deveria chamar a social: %+v", prox)
	}
	h.expect(http.StatusConflict, http.MethodPost, "/api/v1/sempras/unidades/"+c.unA+"/fila/proximo", c.tecA, "")
	c.acao(http.StatusForbidden, c.tecA, ps.ID, "chamar")
	c.acao(http.StatusForbidden, c.admin, ps.ID, "chamar")
	c.acao(http.StatusOK, c.psiA, ps.ID, "chamar")
	c.acao(http.StatusOK, c.psiA, ps.ID, "ausente")
	c.acao(http.StatusOK, c.tecA, m.ID, "iniciar")
	c.acao(http.StatusConflict, c.recepA, m.ID, "cancelar")
	c.acao(http.StatusForbidden, c.recepA, m.ID, "concluir")
	c.acao(http.StatusNotFound, c.tecA, uuid.NewString(), "iniciar")
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/sempras/atendimentos/xyz/iniciar", c.tecA, "")
	h.expect(http.StatusBadRequest, http.MethodPost, "/api/v1/sempras/atendimentos/"+m.ID+"/concluir", c.tecA, `{"local":`)

	// Cancelar: a recepção cancela quem aguarda.
	c2 := c.cadastrar(c.recepA, `{"nome":"Cancelada","data_nascimento":"2000-01-01","nome_mae":"Mãe"}`)
	can := c.incluir(c.recepA, c.unA, c2, "social", "")
	c.acao(http.StatusOK, c.recepA, can.ID, "cancelar")

	// Prontuário: evolução social e psicológica; sigilo aplicado.
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/sempras/cidadaos/"+joao+"/registros", c.tecA,
		`{"unidade_id":"`+c.unA+`","tipo":"social","texto":"Orientação sobre benefícios."}`)
	h.expect(http.StatusCreated, http.MethodPost, "/api/v1/sempras/cidadaos/"+joao+"/registros", c.psiA,
		`{"unidade_id":"`+c.unA+`","atendimento_id":"`+ps.ID+`","tipo":"psicologico","texto":"Escuta qualificada."}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/sempras/cidadaos/"+joao+"/registros", c.tecA,
		`{"unidade_id":"`+c.unA+`","tipo":"psicologico","texto":"x"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/sempras/cidadaos/"+maria+"/registros", c.psiA,
		`{"unidade_id":"`+c.unA+`","atendimento_id":"`+ps.ID+`","tipo":"social","texto":"atendimento de outra pessoa"}`)
	h.expect(http.StatusNotFound, http.MethodPost, "/api/v1/sempras/cidadaos/"+joao+"/registros", c.tecB,
		`{"unidade_id":"`+c.unA+`","tipo":"social","texto":"x"}`)
	h.expect(http.StatusForbidden, http.MethodPost, "/api/v1/sempras/cidadaos/"+joao+"/registros", c.recepA,
		`{"unidade_id":"`+c.unA+`","tipo":"social","texto":"x"}`)

	ficha := func(tok string) semprasProntuario {
		t.Helper()
		return data[semprasProntuario](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/cidadaos/"+joao+"/prontuario", tok, ""))
	}
	antes := c.auditCount("sempras.prontuario.visualizado", joao)
	textoPsi := func(p semprasProntuario) (string, bool) {
		for _, r := range p.Registros {
			if r.Tipo == "psicologico" {
				return r.Texto, r.Sigiloso
			}
		}
		t.Fatalf("registro psicológico ausente: %+v", p.Registros)
		return "", false
	}
	if txt, sig := textoPsi(ficha(c.psiA)); txt != "Escuta qualificada." || sig {
		t.Fatalf("psicólogo vê o texto: %q %v", txt, sig)
	}
	for _, tok := range []string{c.tecA, c.sup, c.admin} {
		p := ficha(tok)
		if txt, sig := textoPsi(p); txt != "" || !sig {
			t.Fatalf("sigilo psicológico vazou: %q", txt)
		}
		if len(p.Registros) != 2 || len(p.Atendimentos) != 1 {
			t.Fatalf("ficha incompleta: %+v", p)
		}
	}
	if p := ficha(c.tecB); len(p.Registros) != 0 || len(p.Atendimentos) != 0 || p.Cidadao.ID != joao {
		t.Fatalf("a unidade B não vê o histórico da A: %+v", p)
	}
	if got := c.auditCount("sempras.prontuario.visualizado", joao) - antes; got != 5 {
		t.Fatalf("toda abertura da ficha é auditada: %d", got)
	}
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/sempras/cidadaos/"+joao+"/prontuario", c.recepA, "")
	h.expect(http.StatusNotFound, http.MethodGet, "/api/v1/sempras/cidadaos/"+uuid.NewString()+"/prontuario", c.tecA, "")

	// Histórico na busca: só o da área.
	hist := func(tok string) int {
		got := data[[]semprasResumo](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/cidadaos?q="+cpf, tok, ""))
		return got[0].AtendimentosNaArea
	}
	if hist(c.recepA) != 1 || hist(c.tecB) != 0 {
		t.Fatalf("histórico fora da área: A=%d B=%d", hist(c.recepA), hist(c.tecB))
	}

	// Painel da gestão.
	h.expect(http.StatusForbidden, http.MethodGet, "/api/v1/sempras/painel", c.tecA, "")
	painel := data[[]struct {
		Unidade struct {
			ID string `json:"id"`
		} `json:"unidade"`
		Concluidos      int `json:"concluidos"`
		EmAtendimento   int `json:"em_atendimento"`
		NaoCompareceram int `json:"nao_compareceram"`
	}](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/sempras/painel", c.sup, ""))
	var okA, okB bool
	for _, p := range painel {
		okA = okA || (p.Unidade.ID == c.unA && p.Concluidos == 1 && p.EmAtendimento == 1 && p.NaoCompareceram == 1)
		okB = okB || (p.Unidade.ID == c.unB && p.Concluidos == 0)
	}
	if !okA || !okB {
		t.Fatalf("painel: %+v", painel)
	}

	// Unidade desativada não recebe fila nem evolução (ADR 012).
	h.expect(http.StatusOK, http.MethodPut, "/api/v1/iam/unidades/"+c.unB, c.admin,
		`{"entidade_id":"`+c.entidade+`","nome":"CRAS B desativado `+uuid.NewString()[:4]+`","ativo":false}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/sempras/unidades/"+c.unB+"/fila", c.admin, `{"cidadao_id":"`+maria+`","tipo":"social"}`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/sempras/cidadaos/"+maria+"/registros", c.admin,
		`{"unidade_id":"`+c.unB+`","tipo":"social","texto":"x"}`)
}

// O tópico da fila no WebSocket segue a mesma regra — inclusive para a
// concessão na entidade sem a árvore expandida (o ticket não a serializa).
func TestSemprasWebSocketAuthorizer(t *testing.T) {
	c := newSemprasCenario(t)
	var authz ws.Authorizer
	for _, p := range c.h.d.Kernel.Plugins() {
		if p.Manifest().Key == "sempras" {
			authz = p.(kernel.WSProvider).WSAuthorizer()
			if err := p.(kernel.WSProvider).WSInbound()(context.Background(), ws.ClientInfo{}, ws.Frame{Type: "sempras.x"}); err == nil {
				t.Fatal("frames do navegador são recusados")
			}
		}
	}
	ent, unA := uuid.MustParse(c.entidade), uuid.MustParse(c.unA)
	perms := []string{"sempras:recepcao", "sempras:atendimento"}
	naUnidade := ws.ClientInfo{Scopes: []auth.Scope{{EntidadeID: &ent, UnidadeID: &unA, Permissions: perms}}}
	naEntidade := ws.ClientInfo{Scopes: []auth.Scope{{EntidadeID: &ent, Permissions: perms}}}
	outra := uuid.New()
	alheio := ws.ClientInfo{Scopes: []auth.Scope{{UnidadeID: &outra, Permissions: perms}}}
	ctx := context.Background()
	for _, tc := range []struct {
		client ws.ClientInfo
		topic  string
		ok     bool
	}{
		{naUnidade, "sempras:fila:" + c.unA, true},
		{naEntidade, "sempras:fila:" + c.unB, true},
		{alheio, "sempras:fila:" + c.unA, false},
		{naUnidade, "sempras:fila:" + c.unB, false},
		{naUnidade, "sempras:fila:nao-e-uuid", false},
		{naUnidade, "sempras:outro:" + c.unA, false},
		{naUnidade, "sempras:fila:" + uuid.NewString(), false},
	} {
		if err := authz(ctx, tc.client, tc.topic); (err == nil) != tc.ok {
			t.Errorf("%s: ok=%v, erro=%v", tc.topic, tc.ok, err)
		}
	}
}

// cpfTeste gera um CPF válido e único.
func cpfTeste(t *testing.T) string {
	t.Helper()
	base := strings.NewReplacer("-", "").Replace(uuid.NewString())
	d := make([]byte, 0, 11)
	for i := 0; len(d) < 9; i++ {
		d = append(d, '0'+base[i]%10)
	}
	for _, n := range []int{9, 10} {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(d[i]-'0') * (n + 1 - i)
		}
		d = append(d, byte('0'+soma*10%11%10))
	}
	return string(d)
}
