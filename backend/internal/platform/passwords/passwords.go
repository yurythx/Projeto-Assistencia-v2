// Package passwords implementa o armazenamento de senhas locais (A02 —
// Cryptographic Failures): Argon2id como algoritmo padrão, no formato PHC
// ("$argon2id$v=19$m=...,t=...,p=...$salt$hash"), com verificação de
// hashes bcrypt legados (fallback corporativo, cost 12). Um hash bcrypt
// válido é aceito e sinalizado por NeedsRehash, para que o login regrave a
// senha em Argon2id de forma transparente.
package passwords

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Parâmetros Argon2id — perfil recomendado pela OWASP (Password Storage
// Cheat Sheet): 64 MiB de memória, 3 iterações, paralelismo 2.
const (
	argonMemoryKiB  uint32 = 64 * 1024
	argonIterations uint32 = 3
	argonThreads    uint8  = 2
	argonSaltLen           = 16
	argonKeyLen     uint32 = 32

	// BcryptCost é o custo mínimo aceito para o fallback bcrypt.
	BcryptCost = 12
)

var (
	// ErrMismatch indica senha incorreta.
	ErrMismatch = errors.New("passwords: senha não confere")
	// ErrUnsupportedHash indica um hash em formato desconhecido (ex.: o
	// tombstone "ANONYMIZED" de uma conta anonimizada pela LGPD).
	ErrUnsupportedHash = errors.New("passwords: formato de hash não suportado")
)

// Hash gera o hash Argon2id de password no formato PHC.
func Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	_, _ = rand.Read(salt) // não falha (Go 1.24+: aborta o processo sem entropia)
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonIterations, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify confere password contra encoded (Argon2id ou bcrypt). Retorna
// nil quando confere, ErrMismatch quando não, ou ErrUnsupportedHash.
func Verify(encoded, password string) error {
	switch {
	case strings.HasPrefix(encoded, "$argon2id$"):
		return verifyArgon2id(encoded, password)
	case isBcrypt(encoded):
		if err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password)); err != nil {
			return ErrMismatch
		}
		return nil
	default:
		return ErrUnsupportedHash
	}
}

// NeedsRehash reporta se encoded deve ser regravado com os parâmetros
// atuais: todo hash bcrypt, e todo Argon2id com parâmetros mais fracos que
// os atuais.
func NeedsRehash(encoded string) bool {
	if isBcrypt(encoded) {
		return true
	}
	p, _, _, err := decodeArgon2id(encoded)
	if err != nil {
		return false
	}
	return p.memory < argonMemoryKiB || p.iterations < argonIterations || p.keyLen < argonKeyLen
}

// DummyVerify executa uma verificação Argon2id de custo equivalente contra
// um hash fixo — usado quando o usuário não existe, para que o tempo de
// resposta não revele quais usernames são válidos (enumeração de contas).
func DummyVerify(password string) {
	_ = verifyArgon2id(dummyHash, password)
}

// dummyHash é um Argon2id válido de uma senha aleatória descartada.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHRzb21lc2FsdA$Hj3SJzY2h0fQ2i1vM1p3Q2Q2l0ZkQ3BhZ0xqV2pUZ0U"

func isBcrypt(encoded string) bool {
	return strings.HasPrefix(encoded, "$2a$") || strings.HasPrefix(encoded, "$2b$") || strings.HasPrefix(encoded, "$2y$")
}

type argonParams struct {
	memory     uint32
	iterations uint32
	threads    uint8
	keyLen     uint32
}

func decodeArgon2id(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, ErrUnsupportedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argonParams{}, nil, nil, ErrUnsupportedHash
	}
	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.threads); err != nil {
		return argonParams{}, nil, nil, ErrUnsupportedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, ErrUnsupportedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return argonParams{}, nil, nil, ErrUnsupportedHash
	}
	// Limites defensivos: um hash adulterado no banco não pode forçar o
	// processo a alocar memória absurda (DoS).
	if p.memory == 0 || p.memory > 1024*1024 || p.iterations == 0 || p.iterations > 20 || p.threads == 0 {
		return argonParams{}, nil, nil, ErrUnsupportedHash
	}
	p.keyLen = uint32(len(key)) // #nosec G115 -- len(key) é pequeno (hash decodificado)
	return p, salt, key, nil
}

func verifyArgon2id(encoded, password string) error {
	p, salt, key, err := decodeArgon2id(encoded)
	if err != nil {
		return err
	}
	candidate := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.threads, p.keyLen)
	if subtle.ConstantTimeCompare(candidate, key) != 1 {
		return ErrMismatch
	}
	return nil
}
