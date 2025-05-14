package https

import (
	"context"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/server/auth"
	"github.com/k3s-io/k3s/pkg/util"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/options"
)

// RouterFunc provides a hook for components to register additional routes to a request router
type RouterFunc func(ctx context.Context, nodeConfig *config.Node) (*mux.Router, error)

var once sync.Once
var router *mux.Router
var err error

// Start returns a router with authn/authz filters applied.
// The first time it is called, the router is created and a new HTTPS listener is started if the handler is nil.
// Subsequent calls will return the same router.
func Start(ctx context.Context, nodeConfig *config.Node, runtime *config.ControlRuntime) (*mux.Router, error) {
	once.Do(func() {
		router = mux.NewRouter().SkipClean(true)
		config := &server.Config{}

		if runtime == nil {
			// If we do not have an existing handler, set up a new listener
			tcp, lerr := util.ListenWithLoopback(ctx, nodeConfig.AgentConfig.ListenAddress, strconv.Itoa(nodeConfig.SupervisorPort))
			if lerr != nil {
				err = lerr
				return
			}

			serving := options.NewSecureServingOptions()
			serving.Listener = tcp
			serving.CipherSuites = nodeConfig.AgentConfig.CipherSuites
			serving.MinTLSVersion = nodeConfig.AgentConfig.MinTLSVersion
			serving.ServerCert = options.GeneratableKeyCert{
				CertKey: options.CertKey{
					CertFile: nodeConfig.AgentConfig.ServingKubeletCert,
					KeyFile:  nodeConfig.AgentConfig.ServingKubeletKey,
				},
			}
			if aerr := serving.ApplyTo(&config.SecureServing); aerr != nil {
				err = aerr
				return
			}
		} else {
			// If we have an existing handler, wrap it
			router.NotFoundHandler = runtime.Handler
			runtime.Handler = router
		}

		router.Use(auth.RequestInfo(), auth.Delegated(nodeConfig.AgentConfig.ClientCA, nodeConfig.AgentConfig.KubeConfigKubelet, config))

		if config.SecureServing != nil {
			_, _, err = config.SecureServing.Serve(router, 0, ctx.Done())
		}
	})

	return router, err
}
