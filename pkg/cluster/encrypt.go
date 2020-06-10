package cluster

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/rancher/k3s/pkg/token"
	"golang.org/x/crypto/pbkdf2"
)

func storageKey(passphrase string) string {
	d := sha256.New()
	d.Write([]byte(passphrase))
	return "/bootstrap/" + hex.EncodeToString(d.Sum(nil)[:])[:12]
}

func keyHash(passphrase string) string {
	d := sha256.New()
	d.Write([]byte(passphrase))
	return hex.EncodeToString(d.Sum(nil)[:])[:12]
}

func encrypt(passphrase string, plaintext []byte) ([]byte, error) {
	salt, err := token.Random(8)
	if err != nil {
		return nil, err
	}

	clearKey := pbkdf2.Key([]byte(passphrase), []byte(salt), 4096, 32, sha1.New)
	key, err := aes.NewCipher(clearKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return []byte(salt + ":" + base64.StdEncoding.EncodeToString(sealed)), nil
}

func decrypt(passphrase string, ciphertext []byte) ([]byte, error) {
	parts := strings.SplitN(string(ciphertext), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cipher text, not : delimited")
	}

	clearKey := pbkdf2.Key([]byte(passphrase), []byte(parts[0]), 4096, 32, sha1.New)
	key, err := aes.NewCipher(clearKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(key)
	if err != nil {
		return nil, err
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}
