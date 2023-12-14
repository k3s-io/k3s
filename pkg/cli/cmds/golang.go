package cmds

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

func ValidateGolang() error {
	k8sVersion, _, _ := strings.Cut(version.Version, "+")
	if version.UpstreamGolang == "" {
		return fmt.Errorf("kubernetes golang build version not set - see 'golang: upstream version' in https://github.com/kubernetes/kubernetes/blob/%s/build/dependencies.yaml", k8sVersion)
	}
	if v, _, _ := strings.Cut(runtime.Version(), " "); version.UpstreamGolang != v {
		return fmt.Errorf("incorrect golang build version - kubernetes %s should be built with %s, runtime version is %s", k8sVersion, version.UpstreamGolang, v)
	}
	return nil
}

func MustValidateGolang() {
	if err := ValidateGolang(); err != nil {
		logrus.Fatalf("Failed to validate golang version: %v", err)
	}
}
