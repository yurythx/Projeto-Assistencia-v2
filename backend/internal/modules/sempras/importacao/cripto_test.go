package importacao

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"testing"
)

// cifrar faz o que o CryptoJS.AES.encrypt do 4.0 faz (para os testes; sal
// fixo, para o resultado ser determinístico).
func cifrar(senha, texto string) string {
	sal := []byte("salfixo1")
	chave, iv := evpBytesToKey([]byte(senha), sal)
	n := aes.BlockSize - len(texto)%aes.BlockSize
	claro := append([]byte(texto), make([]byte, n)...)
	for i := len(texto); i < len(claro); i++ {
		claro[i] = byte(n)
	}
	bloco, _ := aes.NewCipher(chave)
	cifrado := make([]byte, len(claro))
	cipher.NewCBCEncrypter(bloco, iv).CryptBlocks(cifrado, claro)
	return prefixoCifrado + base64.StdEncoding.EncodeToString(append(append([]byte("Salted__"), sal...), cifrado...))
}

// cifrarBruto cifra bytes já com o preenchimento escolhido (casos inválidos).
func cifrarBruto(senha string, claro []byte) string {
	sal := []byte("12345678")
	chave, iv := evpBytesToKey([]byte(senha), sal)
	bloco, _ := aes.NewCipher(chave)
	cifrado := make([]byte, len(claro))
	cipher.NewCBCEncrypter(bloco, iv).CryptBlocks(cifrado, claro)
	return prefixoCifrado + base64.StdEncoding.EncodeToString(append(append([]byte("Salted__"), sal...), cifrado...))
}

func TestDecifrador(t *testing.T) {
	d := NovoDecifrador("chave-falsa-de-teste")
	// gerado pelo crypto-js do próprio 4.0 com a mesma chave (falsa)
	if v, err := d.Texto("ENC_U2FsdGVkX18nREqpjFXDUKJue6LDM2Bty6s8LVSWKSY4cFbw2PVZD4m9kn+WAqid"); err != nil || v != "Ana Júlia da Silva" {
		t.Fatalf("vetor do crypto-js: %q %v", v, err)
	}
	if v, err := d.Texto(cifrar("chave-falsa-de-teste", "Negligência, Outro")); err != nil || v != "Negligência, Outro" {
		t.Fatalf("ida e volta: %q %v", v, err)
	}
	if v, err := d.Texto("em claro"); err != nil || v != "em claro" {
		t.Errorf("campo em claro: %q %v", v, err)
	}
	invalidos := map[string]string{
		"base64 ruim":     "ENC_***",
		"curto":           "ENC_" + base64.StdEncoding.EncodeToString([]byte("Salted__1234")),
		"sem Salted__":    "ENC_" + base64.StdEncoding.EncodeToString(make([]byte, 32)),
		"bloco quebrado":  "ENC_" + base64.StdEncoding.EncodeToString(append([]byte("Salted__12345678"), make([]byte, 17)...)),
		"chave errada":    cifrar("outra-chave", "texto qualquer de teste"),
		"preenchimento 0": cifrarBruto("chave-falsa-de-teste", make([]byte, 16)),
		"utf-8 inválido":  cifrarBruto("chave-falsa-de-teste", append([]byte{0xff, 0xfe}, []byte{14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14, 14}...)),
	}
	for nome, v := range invalidos {
		if _, err := d.Texto(v); !errors.Is(err, ErrDecifrar) {
			t.Errorf("%s: %v", nome, err)
		}
	}
	if _, err := NovoDecifrador("").Texto(cifrar("x", "y")); !errors.Is(err, ErrDecifrar) {
		t.Errorf("sem chave: %v", err)
	}
}
