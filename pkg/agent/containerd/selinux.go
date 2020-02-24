package containerd

import (
	"github.com/opencontainers/selinux/go-selinux"
)

const (
	SELinuxContextType = "container_runtime_t"
)

func selinuxEnabled() (bool, error) {
	if !selinux.GetEnabled() {
		return false, nil
	}

	label, err := selinux.CurrentLabel()
	if err != nil {
		return false, err
	}

	ctx, err := selinux.NewContext(label)
	if err != nil {
		return false, err
	}

	return ctx["type"] == SELinuxContextType, nil
}
