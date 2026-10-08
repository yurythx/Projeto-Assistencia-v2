package domain

import (
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

func TestFormatNumero(t *testing.T) {
	if got := FormatNumero(42, 2026); got != "000042/2026" {
		t.Fatalf("got %q", got)
	}
}

func TestSigiloRules(t *testing.T) {
	unidadeA, unidadeB := uuid.New(), uuid.New()
	servidorA := auth.Identity{UserID: uuid.New(), Scopes: []auth.Scope{{Perfil: "servidor", UnidadeID: &unidadeA}}}
	// Concessões (ADR 013): o gestor global alcança tudo; o gestor de B,
	// só os processos que passam por B.
	gestor := auth.Identity{UserID: uuid.New(), Scopes: []auth.Scope{{Perfil: "gestor", Permissions: []string{"tramite:manage"}}}}
	unidadeC := uuid.New()
	gestorC := auth.Identity{UserID: uuid.New(), Scopes: []auth.Scope{{Perfil: "gestor", UnidadeID: &unidadeC, Permissions: []string{"tramite:manage"}}}}
	gestorB := auth.Identity{UserID: uuid.New(), Scopes: []auth.Scope{{Perfil: "gestor", UnidadeID: &unidadeB, Permissions: []string{"tramite:manage"}}}}
	estranho := auth.Identity{UserID: uuid.New()}

	base := AccessInfo{CreatedBy: uuid.New(), UnidadeOrigemID: unidadeB, UnidadeAtualID: unidadeA}

	pub := base
	pub.Sigilo = SigiloPublico
	if !CanRead(estranho, pub) {
		t.Error("processo público é legível por qualquer autenticado")
	}
	if CanAct(estranho, pub) {
		t.Error("ler não implica poder movimentar: só a unidade atual age")
	}
	if !CanAct(servidorA, pub) {
		t.Error("lotado na unidade atual deve poder movimentar")
	}

	res := base
	res.Sigilo = SigiloRestrito
	if CanRead(estranho, res) {
		t.Error("restrito não é legível por quem não está nas unidades")
	}
	if !CanRead(servidorA, res) || !CanRead(gestor, res) {
		t.Error("restrito é legível pela unidade atual e por tramite:manage")
	}
	if CanRead(gestorC, res) || !CanRead(gestorB, res) {
		t.Error("tramite:manage com escopo: só vale se cobre a unidade de origem ou a atual")
	}
	if CanAct(gestorB, res) || !CanAct(gestor, res) {
		t.Error("o gestor age onde o processo está (unidade atual); o global, em qualquer uma")
	}
	protoA := auth.Identity{UserID: uuid.New(), Scopes: []auth.Scope{{Perfil: "protocolo", UnidadeID: &unidadeA, Permissions: []string{"tramite:route"}}}}
	if PodeTramitar(servidorA, unidadeA) || !PodeTramitar(protoA, unidadeA) || PodeTramitar(protoA, unidadeB) || !PodeTramitar(gestor, unidadeB) {
		t.Error("tramitar exige tramite:route (ou manage) cobrindo a unidade atual")
	}

	sig := base
	sig.Sigilo = SigiloSigiloso
	if CanRead(servidorA, sig) || CanRead(gestor, sig) {
		t.Error("sigiloso exige credencial explícita, mesmo para a unidade atual e tramite:manage")
	}
	sig.ExplicitGrant = true
	if !CanRead(estranho, sig) {
		t.Error("credencial explícita dá acesso ao sigiloso")
	}
	autor := base
	autor.Sigilo = SigiloSigiloso
	if !CanRead(auth.Identity{UserID: base.CreatedBy}, autor) {
		t.Error("o autor sempre acessa o próprio processo")
	}
}

func TestCanActRequiresReadAndStates(t *testing.T) {
	unidade := uuid.New()
	lotado := auth.Identity{UserID: uuid.New(), Scopes: []auth.Scope{{UnidadeID: &unidade}, {Perfil: "sem-unidade"}}}
	sig := AccessInfo{Sigilo: SigiloSigiloso, CreatedBy: uuid.New(), UnidadeOrigemID: unidade, UnidadeAtualID: unidade}
	if CanAct(lotado, sig) {
		t.Error("estar na unidade atual não basta para agir em sigiloso sem credencial")
	}
	for status, want := range map[string][2]bool{
		StatusAberto: {true, false}, StatusEmTramitacao: {true, false}, StatusConcluido: {false, true}, StatusArquivado: {false, true},
	} {
		if Aberto(status) != want[0] || Encerrado(status) != want[1] {
			t.Errorf("%s: aberto=%v encerrado=%v", status, Aberto(status), Encerrado(status))
		}
	}
}

func TestValidarClassificacao(t *testing.T) {
	for _, ok := range []string{"", "2.0.02.00.07", "12.0.03.01.02"} {
		if ValidarClassificacao(ok) != nil {
			t.Errorf("%q é válido", ok)
		}
	}
	for _, ruim := range []string{"2.0", "2.0.02.00", "x", "2.1.02.00.07"} {
		if ValidarClassificacao(ruim) == nil {
			t.Errorf("%q é inválido", ruim)
		}
	}
}
