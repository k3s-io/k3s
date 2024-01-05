package util

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeConfigPath can be used to search for a kubeconfig in standard
// locations if an empty string is passed. If a non-empty string is passed,
// that path is used.
func GetKubeConfigPath(file string) string {
	if file != "" {
		return file
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.Precedence = append([]string{datadir.GlobalConfig}, rules.Precedence...)
	return rules.GetDefaultFilename()
}

// GetClientSet creates a Kubernetes client from the kubeconfig at the provided path.
func GetClientSet(file string) (clientset.Interface, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		return nil, err
	}

	return clientset.NewForConfig(restConfig)
}

// GetUserAgent builds a complete UserAgent string for a given controller, including the node name if possible.
func GetUserAgent(controllerName string) string {
	nodeName := os.Getenv("NODE_NAME")
	managerName := controllerName + "@" + nodeName
	if nodeName == "" || len(managerName) > validation.FieldManagerMaxLength {
		logrus.Warnf("%s controller node name is empty or too long, and will not be tracked via server side apply field management", controllerName)
		managerName = controllerName
	}
	return fmt.Sprintf("%s/%s (%s/%s) %s/%s", managerName, version.Version, runtime.GOOS, runtime.GOARCH, version.Program, version.GitCommit)
}

// SplitStringSlice is a helper function to handle StringSliceFlag containing multiple values
// By default, StringSliceFlag only supports repeated values, not multiple values
// e.g. --foo="bar,car" --foo=baz will result in []string{"bar", "car". "baz"}
func SplitStringSlice(ss []string) []string {
	result := []string{}
	for _, s := range ss {
		result = append(result, strings.Split(s, ",")...)
	}
	return result
}
