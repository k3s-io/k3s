//go:build linux && cgo

package cmds

import (
	"os"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/pkg/errors"
	"github.com/rootless-containers/rootlesskit/pkg/parent/cgrouputil"
)

// EvacuateCgroup2 will handle evacuating the root cgroup in order to enable subtree_control,
// if running as pid 1 without rootless support.
func EvacuateCgroup2() error {
	if os.Getpid() == 1 && !userns.RunningInUserNS() {
		// The root cgroup has to be empty to enable subtree_control, so evacuate it by placing
		// ourselves in the init cgroup.
		if err := cgrouputil.EvacuateCgroup2("init"); err != nil {
			return errors.Wrap(err, "failed to evacuate root cgroup")
		}
	}
	return nil
}
