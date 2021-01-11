package etcdctl

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/configfilearg"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/etcdctl/ctlv2"
	"go.etcd.io/etcd/etcdctl/ctlv3"
)

const (
	apiEnv = "ETCDCTL_API"
	cacert = "ETCDCTL_CACERT"
	cert   = "ETCDCTL_CERT"
	key    = "ETCDCTL_KEY"
)

func Main() {
	dataDir := findDataDir()
	etcdCtlAPI := os.Getenv(apiEnv)
	if etcdCtlAPI == "" {
		os.Setenv(apiEnv, "3")
	}
	etcdCtlCaCert := os.Getenv(cacert)
	if etcdCtlCaCert == "" {
		os.Setenv(cacert, filepath.Join(dataDir, "server", "tls", "etcd", "server-ca.crt"))
	}
	etcdCtlCert := os.Getenv(cert)
	if etcdCtlCert == "" {
		os.Setenv(cert, filepath.Join(dataDir, "server", "tls", "etcd", "server-client.crt"))
	}
	etcdCtlKey := os.Getenv(key)
	if etcdCtlKey == "" {
		os.Setenv(key, filepath.Join(dataDir, "server", "tls", "etcd", "server-client.key"))
	}
	main()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	apiv := os.Getenv(apiEnv)

	// unset apiEnv to avoid side-effect for future env and flag parsing.
	os.Unsetenv(apiEnv)
	if len(apiv) == 0 || apiv == "3" {
		ctlv3.Start()
		return
	}

	if apiv == "2" {
		ctlv2.Start()
		return
	}

	fmt.Fprintf(os.Stderr, "unsupported API version: %v\n", apiv)
	os.Exit(1)
}

func findDataDir() string {
	for i, arg := range os.Args {
		for _, flagName := range []string{"--data-dir", "-d"} {
			if flagName == arg {
				if len(os.Args) > i+1 {
					return os.Args[i+1]
				}
			} else if strings.HasPrefix(arg, flagName+"=") {
				return arg[len(flagName)+1:]
			}
		}
	}
	dataDir := configfilearg.MustFindString(os.Args, "data-dir")
	if dataDir == "" {
		dataDir = datadir.DefaultDataDir
		logrus.Debug("Using default data dir in self-extracting wrapper")
	}
	return dataDir
}
