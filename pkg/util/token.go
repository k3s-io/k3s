package util

import (
	cryptorand "crypto/rand"
	"encoding/hex"
)

func Random(size int) (string, error) {
	token := make([]byte, size, size)
	_, err := cryptorand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}
