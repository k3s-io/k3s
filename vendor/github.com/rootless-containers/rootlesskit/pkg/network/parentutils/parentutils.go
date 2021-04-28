package parentutils

import (
	"github.com/pkg/errors"
	"os"
	"strconv"

	"github.com/rootless-containers/rootlesskit/pkg/common"
)

func PrepareTap(pid int, tap string) error {
	cmds := [][]string{
		nsenter(pid, []string{"ip", "tuntap", "add", "name", tap, "mode", "tap"}),
		nsenter(pid, []string{"ip", "link", "set", tap, "up"}),
	}
	if err := common.Execs(os.Stderr, os.Environ(), cmds); err != nil {
		return errors.Wrapf(err, "executing %v", cmds)
	}
	return nil
}

func nsenter(pid int, cmd []string) []string {
	return append([]string{"nsenter", "-t", strconv.Itoa(pid), "-n", "-m", "-U", "--preserve-credentials"}, cmd...)
}
