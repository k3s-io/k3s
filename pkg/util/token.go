package util

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
)

func Random(size int) (string, error) {
	token := make([]byte, size, size)
	_, err := cryptorand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

// ReadTokenFromFile will attempt to get the token from <data-dir>/token.
// If the file is not found or is empty (in case of fresh installation) it will
// try to use provided serverToken value, instead.
func ReadTokenFromFile(serverToken, certs, dataDir string) (string, error) {
	tokenFile := filepath.Join(dataDir, "token")

	b, err := os.ReadFile(tokenFile)
	b = bytes.TrimSpace(b)

	if os.IsNotExist(err) || len(b) == 0 {
		return clientaccess.FormatToken(serverToken, certs)
	}

	return string(b), err
}

// NormalizeToken will normalize the token read from file or passed as a cli flag
func NormalizeToken(token string) (string, error) {
	_, password, ok := clientaccess.ParseUsernamePassword(token)
	if !ok {
		return password, errors.New("failed to normalize server token; must be in format K10<CA-HASH>::<USERNAME>:<PASSWORD> or <PASSWORD>")
	}

	return password, nil
}

func GetTokenHash(config *config.Control) (string, error) {
	token := config.Token
	if token == "" {
		tokenFromFile, err := ReadTokenFromFile(config.Runtime.ServerToken, config.Runtime.ServerCA, config.DataDir)
		if err != nil {
			return "", err
		}
		token = tokenFromFile
	}
	normalizedToken, err := NormalizeToken(token)
	if err != nil {
		return "", err
	}
	return ShortHash(normalizedToken, 12), nil
}

func ShortHash(s string, i int) string {
	digest := sha256.Sum256([]byte(s))
	return hex.EncodeToString(digest[:])[:i]
}
