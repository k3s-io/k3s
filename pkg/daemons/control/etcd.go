package control

import (
	"context"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
	"k8s.io/kubernetes/pkg/proxy/util"
)

func ETCD(ctx context.Context, cfg *config.Control) error {
	rand.Seed(time.Now().UTC().UnixNano())

	runtime := &config.ControlRuntime{}
	cfg.Runtime = runtime

	if err := prepare(ctx, cfg, runtime); err != nil {
		return errors.Wrap(err, "preparing server")
	}

	// no need to setup a tunnel here for the etcd only node, its basically an agent with etcd running
	// cfg.Runtime.Tunnel = setupTunnel()
	util.DisableProxyHostnameCheck = true

	basicAuth, err := basicAuthenticator(runtime.PasswdFile)
	if err != nil {
		return err
	}

	runtime.Authenticator = combineAuthenticators(basicAuth)

	return nil
}
