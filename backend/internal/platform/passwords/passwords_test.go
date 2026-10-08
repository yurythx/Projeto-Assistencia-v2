package passwords

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndVerifyArgon2id(t *testing.T) {
	h, err := Hash("S3nh@-Forte!")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("formato PHC inesperado: %s", h)
	}
	if err := Verify(h, "S3nh@-Forte!"); err != nil {
		t.Fatalf("senha correta rejeitada: %v", err)
	}
	if err := Verify(h, "errada"); !errors.Is(err, ErrMismatch) {
		t.Fatalf("esperado ErrMismatch, veio %v", err)
	}
	if NeedsRehash(h) {
		t.Fatal("hash com parâmetros atuais não deveria precisar de rehash")
	}
}

func TestHashUsesRandomSalt(t *testing.T) {
	a, _ := Hash("mesma")
	b, _ := Hash("mesma")
	if a == b {
		t.Fatal("dois hashes da mesma senha não podem ser iguais (salt aleatório)")
	}
}

func TestBcryptFallbackAndRehash(t *testing.T) {
	legacy, err := bcrypt.GenerateFromPassword([]byte("legado"), BcryptCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(string(legacy), "legado"); err != nil {
		t.Fatalf("bcrypt legado deveria ser aceito: %v", err)
	}
	if !NeedsRehash(string(legacy)) {
		t.Fatal("bcrypt deve ser sinalizado para rehash em Argon2id")
	}
}

func TestUnsupportedAndTamperedHashes(t *testing.T) {
	for _, h := range []string{
		"ANONYMIZED",
		"",
		"$argon2id$v=19$m=999999999,t=3,p=2$c2FsdA$aGFzaA", // memória absurda (DoS)
		"$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$aGFzaA",     // versão errada
	} {
		if err := Verify(h, "x"); !errors.Is(err, ErrUnsupportedHash) {
			t.Errorf("hash %q: esperado ErrUnsupportedHash, veio %v", h, err)
		}
	}
}

func TestMalformedHashesAndRehashRules(t *testing.T) {
	for _, h := range []string{
		"$argon2id$v=19$m=65536,t=3,p=2$c2FsdA",        // partes faltando
		"$argon2id$v=19$lixo$c2FsdA$aGFzaA",            // parâmetros ilegíveis
		"$argon2id$v=19$m=65536,t=3,p=2$%%%$aGFzaA",    // salt não é base64
		"$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$%%%",    // hash não é base64
		"$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$",       // hash vazio
		"$argon2id$v=19$m=65536,t=0,p=2$c2FsdA$aGFzaA", // 0 iterações
	} {
		if err := Verify(h, "x"); !errors.Is(err, ErrUnsupportedHash) {
			t.Errorf("%q: %v", h, err)
		}
		if NeedsRehash(h) {
			t.Errorf("%q: hash ilegível não é \"rehash\" — é recusado", h)
		}
	}
	bc, _ := bcrypt.GenerateFromPassword([]byte("certa"), bcrypt.MinCost)
	if err := Verify(string(bc), "errada"); !errors.Is(err, ErrMismatch) {
		t.Fatalf("bcrypt com senha errada: %v", err)
	}
	strong, _ := Hash("x")
	if NeedsRehash(strong) {
		t.Fatal("hash com os parâmetros atuais não precisa de rehash")
	}
	weak := strings.Replace(strong, "t=3", "t=1", 1)
	if !NeedsRehash(weak) {
		t.Fatal("Argon2id mais fraco que o atual precisa de rehash")
	}
}

func TestDummyVerifyCostsLikeARealCheck(t *testing.T) {
	if _, _, _, err := decodeArgon2id(dummyHash); err != nil {
		t.Fatalf("dummyHash precisa ser um Argon2id válido (senão responde instantâneo e revela usernames): %v", err)
	}
	DummyVerify("qualquer")
}
