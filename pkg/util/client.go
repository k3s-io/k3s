package util

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/wrangler/v3/pkg/ratelimit"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	restConfig, err := GetRESTConfig(file)
	if err != nil {
		return nil, err
	}

	return clientset.NewForConfig(restConfig)
}

// GetRESTConfig returns a REST config with default timeouts and ratelimitsi cribbed from wrangler defaults.
// ref: https://github.com/rancher/wrangler/blob/v3.0.0/pkg/clients/clients.go#L184-L190
func GetRESTConfig(file string) (*rest.Config, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		return nil, err
	}
	restConfig.Timeout = 15 * time.Minute
	restConfig.RateLimiter = ratelimit.None
	return restConfig, nil
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
// By default, StringSliceFlag supports repeated values, and multiple values, separated by a comma
// e.g. --foo="bar,car" --foo=baz will result in []string{"bar", "car". "baz"}
// However, we disable this with urfave/cli/v2, as controls are not granular enough. You can either have all flags
// support comma separated values, or no flags. We can't have all flags support comma separated values
// because our kube-XXX-arg flags need to pass the value "as is" to the kubelet/kube-apiserver etc.
func SplitStringSlice(ss []string) []string {
	result := []string{}
	for _, s := range ss {
		result = append(result, strings.Split(s, ",")...)
	}
	return result
}
