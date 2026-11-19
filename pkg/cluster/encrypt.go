package cluster

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/fips140"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"

	"github.com/k3s-io/k3s/pkg/util"
	legacypbkdf2 "golang.org/x/crypto/pbkdf2"
)

// v2Prefix marks bootstrap data whose key was derived with PBKDF2-HMAC-SHA256.
// Legacy data starts with a hex salt, so the prefix cannot be mistaken for it.
const v2Prefix = "sha256:"

var fipsEnabled, fipsEnforced = fips140.Enabled, fips140.Enforced

// useV2 reports whether new bootstrap data is written in the v2 format: always in FIPS mode,
// and wherever the existing value is already v2, so that a mixed cluster does not flip formats.
func useV2(existing []byte) bool {
	return fipsEnabled() || bytes.HasPrefix(existing, []byte(v2Prefix))
}

// storageKey returns the etcd key for storing bootstrap data for a given passphrase.
// The key is derived from the sha256 hash of the passphrase.
func storageKey(passphrase string) string {
	return "/bootstrap/" + util.ShortHash(passphrase, 12)
}

// encrypt encrypts a byte slice using aes+gcm with a pbkdf2 key derived from the passphrase and a random salt.
// It returns a byte slice containing the salt and base64-encoded ciphertext.
// When v2 is set the key is derived with SHA-256 and the output carries the v2 prefix.
func encrypt(passphrase string, plaintext []byte, v2 bool) ([]byte, error) {
	if v2 {
		return encryptV2(passphrase, plaintext)
	}

	salt, err := util.Random(8)
	if err != nil {
		return nil, err
	}

	clearKey := legacypbkdf2.Key([]byte(passphrase), []byte(salt), 4096, 32, sha1.New)
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

func encryptV2(passphrase string, plaintext []byte) ([]byte, error) {
	salt, err := util.Random(16)
	if err != nil {
		return nil, err
	}

	gcm, err := v2GCM(passphrase, salt)
	if err != nil {
		return nil, err
	}

	var sealed []byte
	if fipsEnabled() {
		sealed = gcm.Seal(nil, nil, plaintext, nil)
	} else {
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return nil, err
		}
		sealed = gcm.Seal(nonce, nonce, plaintext, nil)
	}
	return []byte(v2Prefix + salt + ":" + base64.StdEncoding.EncodeToString(sealed)), nil
}

// v2GCM returns a GCM that generates its own nonce under FIPS mode, as FIPS 140-3 requires,
// and a plain GCM otherwise. Both read and write nonce || ciphertext || tag.
func v2GCM(passphrase, salt string) (cipher.AEAD, error) {
	clearKey, err := pbkdf2.Key(sha256.New, passphrase, []byte(salt), 4096, 32)
	if err != nil {
		return nil, err
	}
	key, err := aes.NewCipher(clearKey)
	if err != nil {
		return nil, err
	}
	if fipsEnabled() {
		return cipher.NewGCMWithRandomNonce(key)
	}
	return cipher.NewGCM(key)
}

// decrypt attempts to decrypt the byte slice using the supplied passphrase.
// The input byte slice should be the ciphertext output from the encrypt function.
func decrypt(passphrase string, ciphertext []byte) ([]byte, error) {
	if bytes.HasPrefix(ciphertext, []byte(v2Prefix)) {
		return decryptV2(passphrase, ciphertext[len(v2Prefix):])
	}
	if fipsEnforced() {
		return nil, errors.New("bootstrap data was written with SHA-1 and fips140=only forbids reading it, start once with GODEBUG=fips140=on to migrate it")
	}

	parts := strings.SplitN(string(ciphertext), ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid cipher text, not : delimited")
	}

	clearKey := legacypbkdf2.Key([]byte(passphrase), []byte(parts[0]), 4096, 32, sha1.New)
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

	if len(data) < gcm.NonceSize() {
		return nil, errors.New("invalid cipher text, too short")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}

func decryptV2(passphrase string, ciphertext []byte) ([]byte, error) {
	parts := strings.SplitN(string(ciphertext), ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid cipher text, not : delimited")
	}

	gcm, err := v2GCM(passphrase, parts[0])
	if err != nil {
		return nil, err
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	if fipsEnabled() {
		return gcm.Open(nil, nil, data, nil)
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("invalid cipher text, too short")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}
