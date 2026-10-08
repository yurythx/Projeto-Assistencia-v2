// Package secretcrypto cifra/decifra segredos ANTES de gravá-los no
// Postgres (§ Configurações persistidas em runtime). Diferente de
// LOCAL_AUTH_PRIVATE_KEY ou KEYCLOAK_CLIENT_SECRET (que só existem como
// variável de ambiente/arquivo montado, nunca tocam o banco), o novo
// recurso de configuração dinâmica do Keycloak (ver
// internal/platform/keycloakconfig) grava o Client Secret numa tabela —
// um dump do banco (backup, réplica de leitura, um DBA curioso) não pode
// devolver o segredo em texto plano por isso.
//
// AES-256-GCM foi escolhido por ser autenticado (detecta um ciphertext
// adulterado em vez de decifrar silenciosamente lixo) e por já vir na
// biblioteca padrão do Go — sem dependência nova só para isto.
package secretcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// KeySize é o tamanho exigido, em bytes, da chave decodificada de
// CONFIG_ENCRYPTION_KEY — AES-256 exige exatamente 32 bytes.
const KeySize = 32

// ErrInvalidCiphertext é retornado por Decrypt quando o valor não é um
// ciphertext válido produzido por Encrypt com a MESMA chave — payload
// truncado, adulterado, ou cifrado com uma chave diferente (ex.:
// CONFIG_ENCRYPTION_KEY rotacionada sem reescrever os segredos já
// gravados).
var ErrInvalidCiphertext = errors.New("secretcrypto: ciphertext inválido ou chave incorreta")

// Cipher cifra e decifra segredos com uma única chave AES-256, fixada na
// construção (NewFromBase64Key) — nunca lida do banco, sempre de
// configuração de processo (env var ou arquivo via convenção _FILE, como
// qualquer outro segredo desta plataforma).
type Cipher struct {
	aead cipher.AEAD
}

// NewFromBase64Key decodifica keyBase64 (esperado em base64 padrão,
// ex.: gerado com `openssl rand -base64 32`) e constrói o AEAD
// AES-256-GCM. Retorna erro se a chave não decodificar ou não tiver
// exatamente 32 bytes — falha rápido no boot em vez de aceitar uma chave
// fraca/truncada silenciosamente.
func NewFromBase64Key(keyBase64 string) (*Cipher, error) {
	raw, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("secretcrypto: CONFIG_ENCRYPTION_KEY não é base64 válido: %w", err)
	}
	if len(raw) != KeySize {
		return nil, fmt.Errorf("secretcrypto: CONFIG_ENCRYPTION_KEY deve decodificar para %d bytes, tem %d", KeySize, len(raw))
	}

	// Não falham: a chave tem exatamente KeySize (32) bytes, e GCM sobre AES
	// é sempre suportado.
	block, _ := aes.NewCipher(raw)
	aead, _ := cipher.NewGCM(block)
	return &Cipher{aead: aead}, nil
}

// Encrypt cifra plaintext e devolve nonce+ciphertext+tag codificados em
// base64 (RawURLEncoding não é usado de propósito: o valor só trafega
// dentro de uma coluna TEXT do Postgres, nunca numa URL). Um nonce
// aleatório novo é gerado a cada chamada — GCM nunca deve reusar
// nonce com a mesma chave.
func (c *Cipher) Encrypt(plaintext string) string {
	if plaintext == "" {
		// String vazia é o sentinel de "sem segredo" usado por todo o
		// resto do pacote (Store.Set trata "" como "manter o valor
		// atual" — ver keycloakconfig/store.go) — cifrar e decifrar uma
		// string vazia deixaria essa checagem ambígua.
		return ""
	}

	nonce := make([]byte, c.aead.NonceSize())
	_, _ = rand.Read(nonce) // não falha (Go 1.24+)

	ciphertext := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext)
}

// Decrypt reverte Encrypt. Uma string vazia decifra para uma string
// vazia (ver o comentário em Encrypt); qualquer outro valor que falhe
// autenticação (adulterado, truncado, ou cifrado com outra chave) retorna
// ErrInvalidCiphertext.
func (c *Cipher) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	nonceSize := c.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", ErrInvalidCiphertext
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plaintext), nil
}
