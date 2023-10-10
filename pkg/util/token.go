package util

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/pkg/errors"
)

func Random(size int) (string, error) {
	token := make([]byte, size, size)
	_, err := cryptorand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

// ReadTokenFromFile will attempt to get the token from <data-dir>/token if it the file not found
// in case of fresh installation it will try to use the runtime serverToken saved in memory
// after stripping it from any additional information like the username or cahash, if the file
// found then it will still strip the token from any additional info
func ReadTokenFromFile(serverToken, certs, dataDir string) (string, error) {
	tokenFile := filepath.Join(dataDir, "token")

	b, err := os.ReadFile(tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			token, err := clientaccess.FormatToken(serverToken, certs)
			if err != nil {
				return token, err
			}
			return token, nil
		}
		return "", err
	}

	// strip the token from any new line if its read from file
	return string(bytes.TrimRight(b, "\n")), nil
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
