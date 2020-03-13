package containerd

import (
	"github.com/opencontainers/selinux/go-selinux"
)

const (
	SELinuxContextType = "container_runtime_t"
)

func selinuxStatus() (bool, bool, error) {
	if !selinux.GetEnabled() {
		return false, false, nil
	}

	label, err := selinux.CurrentLabel()
	if err != nil {
		return true, false, err
	}

	ctx, err := selinux.NewContext(label)
	if err != nil {
		return true, false, err
	}

	return true, ctx["type"] == SELinuxContextType, nil
}
