package clientaccess

import (
	"os"

	"github.com/k3s-io/k3s/pkg/util/errors"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// WriteClientKubeConfig generates a kubeconfig at destFile that can be used to connect to a server at url with the given certs and keys
func WriteClientKubeConfig(destFile, name, url, serverCAFile, clientCertFile, clientKeyFile string) error {
	serverCA, err := os.ReadFile(serverCAFile)
	if err != nil {
		return errors.WithMessagef(err, "failed to read %s", serverCAFile)
	}

	clientCert, err := os.ReadFile(clientCertFile)
	if err != nil {
		return errors.WithMessagef(err, "failed to read %s", clientCertFile)
	}

	clientKey, err := os.ReadFile(clientKeyFile)
	if err != nil {
		return errors.WithMessagef(err, "failed to read %s", clientKeyFile)
	}

	config := clientcmdapi.NewConfig()

	cluster := clientcmdapi.NewCluster()
	cluster.CertificateAuthorityData = serverCA
	cluster.Server = url

	authInfo := clientcmdapi.NewAuthInfo()
	authInfo.ClientCertificateData = clientCert
	authInfo.ClientKeyData = clientKey

	context := clientcmdapi.NewContext()
	context.AuthInfo = name
	context.Cluster = name

	config.Clusters[name] = cluster
	config.AuthInfos[name] = authInfo
	config.Contexts[name] = context
	config.CurrentContext = name

	return clientcmd.WriteToFile(*config, destFile)
}
