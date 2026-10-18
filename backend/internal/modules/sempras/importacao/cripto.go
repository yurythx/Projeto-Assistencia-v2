// Package importacao traz o histórico do Casa Abrigo 4.0 (o sistema que a
// unidade usava, ADR 032) para o SEMPRAS (ADR 033): lê o banco do 4.0,
// decifra os campos que ele cifrava, mapeia o texto livre e os rótulos das
// telas para as listas fechadas daqui e grava pelo domínio — com as mesmas
// regras de um cadastro feito na tela. Sempre simula antes: a simulação
// roda a importação inteira numa transação desfeita no fim e devolve o
// relatório do que entraria, do que precisa de revisão e do que ficaria de
// fora (sem nome de criança no relatório, só os ids do 4.0).
package importacao

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" // #nosec G501 -- o 4.0 deriva a chave assim (CryptoJS/OpenSSL); aqui só se decifra o legado
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf8"
)

// prefixoCifrado marca o campo cifrado pelo 4.0 (security/encryption.js).
const prefixoCifrado = "ENC_"

// ErrDecifrar: o campo cifrado não abriu com a chave dada (chave errada ou
// dado corrompido).
var ErrDecifrar = errors.New("campo cifrado do 4.0 não decifrou (confira a ENCRYPTION_KEY)")

// Decifrador abre os campos que o 4.0 cifrava com CryptoJS.AES.encrypt(texto,
// senha): o formato do OpenSSL ("Salted__" + sal de 8 bytes), chave e IV
// derivados da senha por EVP_BytesToKey com MD5, AES-256-CBC e PKCS#7.
type Decifrador struct{ senha []byte }

// NovoDecifrador guarda a ENCRYPTION_KEY do 4.0 (só em memória).
func NovoDecifrador(senha string) Decifrador { return Decifrador{senha: []byte(senha)} }

// Texto devolve o campo em claro: sem o prefixo, o valor já estava em claro.
func (d Decifrador) Texto(v string) (string, error) {
	if !strings.HasPrefix(v, prefixoCifrado) {
		return v, nil
	}
	bruto, err := base64.StdEncoding.DecodeString(v[len(prefixoCifrado):])
	if err != nil || len(bruto) < 16+aes.BlockSize || !bytes.HasPrefix(bruto, []byte("Salted__")) ||
		(len(bruto)-16)%aes.BlockSize != 0 || len(d.senha) == 0 {
		return "", ErrDecifrar
	}
	chave, iv := evpBytesToKey(d.senha, bruto[8:16])
	bloco, _ := aes.NewCipher(chave) // a chave tem sempre 32 bytes
	claro := make([]byte, len(bruto)-16)
	cipher.NewCBCDecrypter(bloco, iv).CryptBlocks(claro, bruto[16:])
	n := int(claro[len(claro)-1])
	if n == 0 || n > aes.BlockSize || !bytes.Equal(claro[len(claro)-n:], bytes.Repeat([]byte{byte(n)}, n)) {
		return "", ErrDecifrar
	}
	claro = claro[:len(claro)-n]
	if !utf8.Valid(claro) {
		return "", ErrDecifrar
	}
	return string(claro), nil
}

// evpBytesToKey é a derivação do OpenSSL (1 iteração, MD5): 32 bytes de
// chave e 16 de IV.
func evpBytesToKey(senha, sal []byte) (chave, iv []byte) {
	var out, ant []byte
	for len(out) < 48 {
		h := md5.New() // #nosec G401 -- derivação do OpenSSL usada pelo 4.0 (ver o import)
		h.Write(ant)
		h.Write(senha)
		h.Write(sal)
		ant = h.Sum(nil)
		out = append(out, ant...)
	}
	return out[:32], out[32:48]
}
