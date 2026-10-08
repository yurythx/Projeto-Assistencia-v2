package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSignatureSealDetectsTampering(t *testing.T) {
	key := []byte("chave-do-servidor")
	env, user := uuid.New(), uuid.New()
	doc := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	at := time.Date(2026, 9, 25, 14, 30, 0, 123456789, time.UTC)
	seal := SignatureSeal(key, env, doc, user, at, "password_reauth:local")
	s := Signer{UserID: user, SignedAt: &at, SignatureHash: seal, Method: "password_reauth:local"}

	if !VerifySeal(key, env, doc, s) {
		t.Fatal("selo íntegro deveria validar")
	}
	if VerifySeal(key, env, "0"+doc[1:], s) {
		t.Fatal("documento alterado não pode validar")
	}
	other := at.Add(time.Second)
	if VerifySeal(key, env, doc, Signer{UserID: user, SignedAt: &other, SignatureHash: seal, Method: s.Method}) {
		t.Fatal("instante alterado não pode validar")
	}
	if VerifySeal([]byte("outra-chave"), env, doc, s) {
		t.Fatal("selo não pode validar com outra chave (forja por quem só tem o banco)")
	}
}

func TestCanSignSequentialAndParallel(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	e := Envelope{Status: StatusPending, Sequential: true, Signers: []Signer{
		{UserID: a, Position: 0, Status: StatusPending},
		{UserID: b, Position: 1, Status: StatusPending},
	}}
	if err := e.CanSign(a); err != nil {
		t.Fatalf("primeiro da fila deveria poder assinar: %v", err)
	}
	if err := e.CanSign(b); err != ErrNotYourTurn {
		t.Fatalf("segundo deve aguardar a vez, veio %v", err)
	}
	if err := e.CanSign(uuid.New()); err != ErrNotSigner {
		t.Fatalf("estranho não é signatário, veio %v", err)
	}
	e.Sequential = false
	if err := e.CanSign(b); err != nil {
		t.Fatalf("em paralelo qualquer pendente assina: %v", err)
	}
	e.Status = StatusRefused
	if err := e.CanSign(a); err != ErrClosed {
		t.Fatalf("envelope fechado, veio %v", err)
	}
}

func TestValidSHA256(t *testing.T) {
	if !ValidSHA256("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08") {
		t.Fatal("hash válido recusado")
	}
	for _, bad := range []string{"", "xyz", "9F86D081884C7D659A2FEAA0C55AD015A3BF4F1B2B0B822CD15D6C15B0F00A08"} {
		if ValidSHA256(bad) {
			t.Fatalf("%q não deveria ser aceito", bad)
		}
	}
}

func TestCanSignEdgeCasesAndSealWithoutSignature(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	e := Envelope{Status: StatusPending, Sequential: true, Signers: []Signer{
		{UserID: a, Status: "signed"}, {UserID: b, Status: StatusPending}, {UserID: c, Status: StatusPending},
	}}
	if err := e.CanSign(a); err != ErrNotSigner {
		t.Fatalf("quem já assinou não é mais signatário pendente: %v", err)
	}
	if err := e.CanSign(b); err != nil {
		t.Fatalf("o próximo pendente assina: %v", err)
	}
	if err := e.CanSign(c); err != ErrNotYourTurn {
		t.Fatalf("o terceiro aguarda: %v", err)
	}
	e.Sequential = false
	if err := e.CanSign(uuid.New()); err != ErrNotSigner {
		t.Fatalf("em paralelo, estranho também não assina: %v", err)
	}
	if e.AllSigned() || (Envelope{}).AllSigned() {
		t.Fatal("envelope com pendentes (ou sem signatários) não está concluído")
	}
	if VerifySeal([]byte("k"), uuid.New(), "x", Signer{UserID: a}) {
		t.Fatal("sem instante e selo, não há assinatura para validar")
	}
	if HashNonce("abc") == HashNonce("abd") || len(HashNonce("abc")) != 64 {
		t.Fatal("hash do nonce")
	}
}
