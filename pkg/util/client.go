package util

import (
	"github.com/k3s-io/k3s/pkg/datadir"
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
