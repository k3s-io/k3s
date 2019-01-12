package resolvehome

import (
	"os"
	"os/user"
	"strings"

	"github.com/pkg/errors"
)

var (
	homes = []string{"$HOME", "${HOME}", "~"}
)

func Resolve(s string) (string, error) {
	for _, home := range homes {
		if strings.Contains(s, home) {
			homeDir, err := getHomeDir()
			if err != nil {
				return "", errors.Wrap(err, "determining current user")
			}
			s = strings.Replace(s, home, homeDir, -1)
		}
	}

	return s, nil
}

func getHomeDir() (string, error) {
	if os.Getuid() == 0 {
		return "/root", nil
	}

	u, err := user.Current()
	if err != nil {
		return "", errors.Wrap(err, "determining current user, try set HOME and USER env vars")
	}
	return u.HomeDir, nil
}
