package token

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func Random(size int) (string, error) {
	token := make([]byte, size, size)
	_, err := cryptorand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

func ReadFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	for {
		tokenBytes, err := ioutil.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(tokenBytes)), nil
		} else if os.IsNotExist(err) {
			logrus.Infof("Waiting for %s to be available\n", path)
			time.Sleep(2 * time.Second)
		} else {
			return "", err
		}
	}
}
