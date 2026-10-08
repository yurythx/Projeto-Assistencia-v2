package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
)

var hoje = time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)

func TestCPFValido(t *testing.T) {
	for cpf, want := range map[string]bool{
		"52998224725": true,
		"11111111111": false, // todos iguais
		"52998224715": false, // 1º dígito verificador
		"52998224726": false, // 2º dígito verificador
		"5299822472":  false, // tamanho
	} {
		if got := CPFValido(cpf); got != want {
			t.Errorf("CPFValido(%s) = %v", cpf, got)
		}
	}
	if MascararCPF("") != "" || MascararCPF("52998224725") != "***.982.247-**" {
		t.Errorf("máscara do CPF: %q", MascararCPF("52998224725"))
	}
	if MascararNIS("12") != "" || MascararNIS("12345678901") != "*******8901" {
		t.Error("máscara do NIS")
	}
	if Digitos("529.982.247-25") != "52998224725" {
		t.Error("Digitos")
	}
}

func TestNormalizarCadastro(t *testing.T) {
	ok := CidadaoInput{Nome: "  Maria   da Silva ", CPF: "529.982.247-25", Telefone: "(66) 9999"}
	c, err := ok.Normalizar(hoje)
	if err != nil || c.Nome != "Maria da Silva" || c.CPF != "52998224725" {
		t.Fatalf("cadastro válido: %+v %v", c, err)
	}
	if c, err := (CidadaoInput{Nome: "João", DataNascimento: "1990-05-01", NomeMae: "Ana"}).Normalizar(hoje); err != nil || c.DataNascimento != "1990-05-01" {
		t.Fatalf("sem documento, com nascimento e mãe: %+v %v", c, err)
	}
	if _, err := (CidadaoInput{Nome: "Zé", NIS: "123.45678.90-1"}).Normalizar(hoje); err != nil {
		t.Fatalf("NIS válido: %v", err)
	}
	for nome, in := range map[string]CidadaoInput{
		"nome curto":        {Nome: "A", CPF: "52998224725"},
		"nome social longo": {Nome: "Maria", NomeSocial: strings.Repeat("x", 201), CPF: "52998224725"},
		"CPF inválido":      {Nome: "Maria", CPF: "123"},
		"NIS curto":         {Nome: "Maria", NIS: "123"},
		"data ilegível":     {Nome: "Maria", CPF: "52998224725", DataNascimento: "01/05/1990"},
		"data futura":       {Nome: "Maria", CPF: "52998224725", DataNascimento: "2027-01-01"},
		"data antiga":       {Nome: "Maria", CPF: "52998224725", DataNascimento: "1899-12-31"},
		"sem identificação": {Nome: "Maria", DataNascimento: "1990-05-01"},
	} {
		var v *ValidationError
		if _, err := in.Normalizar(hoje); !errors.As(err, &v) || v.Error() == "" {
			t.Errorf("%s: esperado erro de validação, veio %v", nome, err)
		}
	}
}

func TestResumoENomeSocial(t *testing.T) {
	c := Cidadao{ID: uuid.New(), Nome: "Mariana", CPF: "52998224725", NIS: "12345678901"}
	if c.NomeExibicao() != "Mariana" {
		t.Error("sem nome social, o nome civil")
	}
	c.NomeSocial = "Mari"
	if c.NomeExibicao() != "Mari" {
		t.Error("nome social tem precedência")
	}
	r := c.Resumir(3)
	if r.CPF != "***.982.247-**" || r.NIS != "*******8901" || r.AtendimentosNaArea != 3 {
		t.Errorf("resumo: %+v", r)
	}
}

func TestFilaEstados(t *testing.T) {
	autor := uuid.New()
	if _, err := NovaEntrada(uuid.New(), uuid.New(), "juridico", "", autor, "2026-10-08"); err == nil {
		t.Error("tipo inválido")
	}
	if _, err := NovaEntrada(uuid.New(), uuid.New(), TipoSocial, "vip", autor, "2026-10-08"); err == nil {
		t.Error("prioridade inválida")
	}
	a, err := NovaEntrada(uuid.New(), uuid.New(), TipoSocial, "", autor, "2026-10-08")
	if err != nil || a.Status != StatusAguardando {
		t.Fatal(err)
	}
	a.Senha = 7
	if a.SenhaExibida() != "N007" {
		t.Errorf("senha: %s", a.SenhaExibida())
	}
	a.Prioridade = "gestante"
	if a.SenhaExibida() != "P007" {
		t.Errorf("senha prioritária: %s", a.SenhaExibida())
	}

	if err := a.Transitar("iniciar", autor, "", hoje); !errors.Is(err, ErrInvalidState) {
		t.Error("não se inicia quem não foi chamado")
	}
	if err := a.Transitar("chamar", autor, " Sala 1 ", hoje); err != nil || a.Local != "Sala 1" || a.ChamadoEm == nil {
		t.Fatalf("chamar: %+v %v", a, err)
	}
	if err := a.Transitar("chamar", autor, "", hoje); err != nil || a.Local != "Sala 1" {
		t.Fatal("rechamar mantém o local")
	}
	if err := a.Transitar("iniciar", autor, "", hoje); err != nil || a.IniciadoEm == nil || a.Status != StatusEmAtendimento {
		t.Fatal("iniciar")
	}
	if err := a.Transitar("cancelar", autor, "", hoje); !errors.Is(err, ErrInvalidState) {
		t.Error("em atendimento não se cancela")
	}
	if err := a.Transitar("concluir", autor, "", hoje); err != nil || a.EncerradoEm == nil || a.Status != StatusConcluido {
		t.Fatal("concluir")
	}
	if err := a.Transitar("desconhecida", autor, "", hoje); !errors.Is(err, ErrInvalidState) {
		t.Error("ação desconhecida")
	}
}

func TestRegistroESigilo(t *testing.T) {
	autor := uuid.New()
	if _, err := NovoRegistro(uuid.New(), uuid.New(), nil, "outro", "texto", autor); err == nil {
		t.Error("tipo inválido")
	}
	if _, err := NovoRegistro(uuid.New(), uuid.New(), nil, TipoSocial, "   ", autor); err == nil {
		t.Error("texto vazio")
	}
	r, err := NovoRegistro(uuid.New(), uuid.New(), nil, TipoPsicologico, " escuta ", autor)
	if err != nil || r.Texto != "escuta" {
		t.Fatal(err)
	}
	if o := r.Ocultar(); o.Texto != "" || !o.Sigiloso || r.Texto != "escuta" {
		t.Error("ocultar não altera o original e esconde o texto")
	}
}

func TestEscopoPorUnidade(t *testing.T) {
	ent, un, outra := uuid.New(), uuid.New(), uuid.New()
	u := Unidade{ID: un, EntidadeID: ent}
	tecnico := auth.Identity{Scopes: []auth.Scope{{UnidadeID: &un, Permissions: []string{"sempras:recepcao", "sempras:atendimento"}}}}
	psicologo := auth.Identity{Scopes: []auth.Scope{{UnidadeID: &un, Permissions: []string{"sempras:*"}}}}
	superint := auth.Identity{Scopes: []auth.Scope{{EntidadeID: &ent, Permissions: []string{"sempras:atendimento", "sempras:gestao"}}}}
	admin := auth.Identity{Roles: []string{auth.RoleAdmin}}
	alheio := auth.Identity{Scopes: []auth.Scope{{UnidadeID: &outra, Permissions: []string{"sempras:*"}}}}

	if !u.Pode(tecnico, auth.PermSemprasAtendimento) || u.Pode(alheio, auth.PermSemprasAtendimento) {
		t.Error("técnico na unidade sim, de outra unidade não")
	}
	if !u.Pode(superint, auth.PermSemprasAtendimento) || !u.PodeAlguma(superint, TodasAsPermissoes...) {
		t.Error("concessão na entidade cobre a unidade sem a árvore expandida")
	}
	if !u.PodeTipo(tecnico, TipoSocial) || u.PodeTipo(tecnico, TipoPsicologico) || !u.PodeTipo(psicologo, TipoPsicologico) {
		t.Error("psicológico só com o sigilo")
	}
	if u.PodePsicologia(admin) || !u.Pode(admin, auth.PermSemprasAtendimento) {
		t.Error("administrador passa no atendimento, mas não no sigilo psicológico")
	}
	if a := AreaDe(tecnico, TodasAsPermissoes...); a.Vazia() || a.All || len(a.Unidades) == 0 {
		t.Errorf("área do técnico: %+v", a)
	}
	if a := AreaDe(superint, auth.PermSemprasGestao); len(a.Entidades) != 1 {
		t.Errorf("área do superintendente: %+v", a)
	}
	if !AreaDe(admin, auth.PermSemprasGestao).All || !AreaDe(auth.Identity{}, TodasAsPermissoes...).Vazia() {
		t.Error("admin alcança tudo; sem concessão, nada")
	}
}
