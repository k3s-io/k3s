package home

import (
	"os"
	"strings"

	"github.com/k3s-io/k3s/pkg/util/errors"
)

var (
	homes = []string{"$HOME", "${HOME}", "~"}
)

func Resolve(s string) (string, error) {
	for _, home := range homes {
		if strings.Contains(s, home) {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", errors.WithMessage(err, "determining current user")
			}
			s = strings.Replace(s, home, homeDir, -1)
		}
	}

	return s, nil
}
