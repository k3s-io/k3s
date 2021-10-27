package clientaccess

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// WriteClientKubeConfig generates a kubeconfig at destFile that can be used to connect to a server at url with the given certs and keys
func WriteClientKubeConfig(destFile string, url string, serverCAFile string, clientCertFile string, clientKeyFile string) error {
	serverCA, err := ioutil.ReadFile(serverCAFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", serverCAFile)
	}

	clientCert, err := ioutil.ReadFile(clientCertFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", clientCertFile)
	}

	clientKey, err := ioutil.ReadFile(clientKeyFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", clientKeyFile)
	}

	config := clientcmdapi.NewConfig()

	cluster := clientcmdapi.NewCluster()
	cluster.CertificateAuthorityData = serverCA
	cluster.Server = url

	authInfo := clientcmdapi.NewAuthInfo()
	authInfo.ClientCertificateData = clientCert
	authInfo.ClientKeyData = clientKey

	context := clientcmdapi.NewContext()
	context.AuthInfo = "default"
	context.Cluster = "default"

	config.Clusters["default"] = cluster
	config.AuthInfos["default"] = authInfo
	config.Contexts["default"] = context
	config.CurrentContext = "default"

	return clientcmd.WriteToFile(*config, destFile)
}
